package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"

	"go-sse-server/internal/database"
	"go-sse-server/internal/dispatch"
	"go-sse-server/internal/middleware"
)

type SubmitAssistanceRequest struct {
	Name         string   `json:"name"`
	Identity     string   `json:"identity,omitempty"`
	PhoneNumber  string   `json:"phone_number"`
	ThingsNeeded string   `json:"things_needed"` // Maps to ResourceCategory or description
	Category     string   `json:"category,omitempty"`
	Amount       int32    `json:"amount"` // Quantity needed
	Description  string   `json:"description,omitempty"`
	PhotoURL     string   `json:"photo_url,omitempty"`
	Priority     string   `json:"priority,omitempty"` // Deprecated: priority is calculated server-side.
	Location     string   `json:"location,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	AddressText  string   `json:"address_text,omitempty"`
}

type AssistanceRequestResponse struct {
	ID                    string    `json:"id"`
	RequesterID           *string   `json:"requester_id,omitempty"`
	TrackingCode          string    `json:"tracking_code"`
	Category              string    `json:"category"`
	QuantityNeeded        int32     `json:"quantity_needed"`
	Description           string    `json:"description"`
	Priority              string    `json:"priority"`
	Status                string    `json:"status"`
	DispatchStatus        string    `json:"dispatch_status"`
	MatchedProviderID     *string   `json:"matched_provider_id,omitempty"`
	MatchedProviderName   *string   `json:"matched_provider_name,omitempty"`
	MatchedProviderPhone  *string   `json:"matched_provider_phone,omitempty"`
	HandshakeCode         *string   `json:"handshake_code,omitempty"`
	AssignedCoordinatorID *string   `json:"assigned_coordinator_id,omitempty"`
	Location              string    `json:"location"`
	AddressText           string    `json:"address_text"`
	RequesterName         string    `json:"requester_name"`
	ContactPhone          string    `json:"contact_phone"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type ProviderAssistanceRequestResponse struct {
	AssistanceRequestResponse
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func assistanceRequestResponse(req database.AssistanceRequest) AssistanceRequestResponse {
	var requesterID, coordinatorID, providerID *string
	if req.RequesterID.Valid {
		value := uuid.UUID(req.RequesterID.Bytes).String()
		requesterID = &value
	}
	if req.AssignedCoordinatorID.Valid {
		value := uuid.UUID(req.AssignedCoordinatorID.Bytes).String()
		coordinatorID = &value
	}
	if req.MatchedProviderID.Valid {
		value := uuid.UUID(req.MatchedProviderID.Bytes).String()
		providerID = &value
	}
	return AssistanceRequestResponse{
		ID:                    uuid.UUID(req.ID.Bytes).String(),
		RequesterID:           requesterID,
		TrackingCode:          req.TrackingCode,
		Category:              string(req.Category),
		QuantityNeeded:        req.QuantityNeeded,
		Description:           pgTextToString(req.Description),
		Priority:              string(req.Priority),
		Status:                string(req.Status),
		DispatchStatus:        string(req.DispatchStatus),
		MatchedProviderID:     providerID,
		AssignedCoordinatorID: coordinatorID,
		Location:              req.Location,
		AddressText:           pgTextToString(req.AddressText),
		RequesterName:         req.RequesterNameEncrypted,
		ContactPhone:          req.ContactPhoneEncrypted,
		CreatedAt:             req.CreatedAt.Time,
		UpdatedAt:             req.UpdatedAt.Time,
	}
}

func generateTrackingCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("REQ-%s", strings.ToUpper(hex.EncodeToString(b)))
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func calculateAssistancePriority(category, thingsNeeded, description string, hazardNearby bool) database.RequestPriority {
	text := strings.ToLower(strings.Join([]string{category, thingsNeeded, description}, " "))
	categoryName := strings.ToUpper(strings.TrimSpace(category))

	baseScore := 30
	medical := categoryName == "MEDICINE" || containsAny(text, "medical", "medicine", "rescue", "ambulance", "oxygen", "blood", "insulin", "life-threatening")
	food := categoryName == "FOOD" || containsAny(text, "food", "blanket", "blankets")
	shelter := categoryName == "SHELTER" || containsAny(text, "shelter", "tent", "housing")
	water := categoryName == "WATER" || containsAny(text, "water", "drinking")
	lowUrgency := categoryName == "EQUIPMENT" || categoryName == "VOLUNTEER" || containsAny(text, "cleanup", "clean-up", "volunteer")

	switch {
	case medical:
		baseScore = 90
	case water || shelter:
		baseScore = 70
	case food:
		baseScore = 45
	case lowUrgency:
		baseScore = 20
	}

	vulnerable := containsAny(text,
		"infant", "baby", "newborn", "pregnant", "pregnancy", "elderly", "senior",
		"disabil", "wheelchair", "blind", "bedridden",
	)
	if vulnerable {
		if food {
			return database.RequestPriorityCRITICAL
		}
		baseScore += 30
	}
	if hazardNearby {
		baseScore += 35
	}

	switch {
	case baseScore >= 90:
		return database.RequestPriorityCRITICAL
	case baseScore >= 65:
		return database.RequestPriorityHIGH
	case baseScore >= 35:
		return database.RequestPriorityMEDIUM
	default:
		return database.RequestPriorityLOW
	}
}

func (h *APIHandler) hasNearbyActiveHazard(ctx context.Context, location string) bool {
	if location == "" {
		return false
	}

	var nearby bool
	err := h.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM road_hazards
			WHERE created_at >= NOW() - INTERVAL '30 days'
			  AND ST_DWithin(location::geography, ST_GeomFromText($1, 4326)::geography, 500)
			  AND (
				is_verified
				OR UPPER(severity) IN ('HIGH', 'CRITICAL')
				OR LOWER(hazard_type) ~ '(flood|collapse|fire|landslide)'
			  )
		)`, location).Scan(&nearby)
	if err != nil {
		log.Printf("[Priority] Nearby hazard lookup skipped: %v", err)
		return false
	}
	return nearby
}

func (h *APIHandler) SubmitAssistanceRequest(w http.ResponseWriter, r *http.Request) {
	var req SubmitAssistanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.PhoneNumber = strings.TrimSpace(req.PhoneNumber)
	req.ThingsNeeded = strings.TrimSpace(req.ThingsNeeded)
	req.Category = strings.TrimSpace(req.Category)
	req.Description = strings.TrimSpace(req.Description)
	req.AddressText = strings.TrimSpace(req.AddressText)

	claims, hasClaims := middleware.GetClaims(r.Context())
	if req.PhoneNumber == "" && hasClaims && claims.Phone != "" {
		req.PhoneNumber = claims.Phone
	}

	if req.Name == "" || req.PhoneNumber == "" {
		respondWithError(w, http.StatusBadRequest, "name and phone_number are required")
		return
	}

	if req.Amount <= 0 {
		req.Amount = 1
	}

	// Resolve Category
	category := database.ResourceCategoryOTHER
	catSource := req.Category
	if catSource == "" {
		catSource = req.ThingsNeeded
	}
	switch strings.ToUpper(catSource) {
	case "FOOD":
		category = database.ResourceCategoryFOOD
	case "WATER":
		category = database.ResourceCategoryWATER
	case "MEDICINE":
		category = database.ResourceCategoryMEDICINE
	case "SHELTER":
		category = database.ResourceCategorySHELTER
	case "EQUIPMENT":
		category = database.ResourceCategoryEQUIPMENT
	case "VOLUNTEER":
		category = database.ResourceCategoryVOLUNTEER
	default:
		category = database.ResourceCategoryOTHER
		if req.Description == "" && req.ThingsNeeded != "" {
			req.Description = req.ThingsNeeded
		}
	}

	// Resolve Location
	location := strings.TrimSpace(req.Location)
	if location == "" && req.Latitude != nil && req.Longitude != nil {
		location = fmt.Sprintf("POINT(%f %f)", *req.Longitude, *req.Latitude)
	}
	if location == "" {
		respondWithError(w, http.StatusBadRequest, "location or latitude/longitude is required")
		return
	}

	// Priority is calculated from the request content and nearby active hazards.
	priority := calculateAssistancePriority(
		string(category), req.ThingsNeeded, req.Description,
		h.hasNearbyActiveHazard(r.Context(), location),
	)

	// Resolve Requester ID from JWT claims if logged in
	var requesterID pgtype.UUID
	if claims, ok := middleware.GetClaims(r.Context()); ok && claims.AccountID != "" {
		if parsed, err := uuid.Parse(claims.AccountID); err == nil {
			requesterID = pgtype.UUID{Bytes: parsed, Valid: true}
		}
	}

	trackingCode := generateTrackingCode()

	fullDescription := req.Description
	if req.Identity != "" {
		fullDescription = fmt.Sprintf("[Identity: %s] %s", strings.TrimSpace(req.Identity), fullDescription)
	}

	var embVector pgvector.Vector
	if h.mlClient != nil {
		embedTarget := req.ThingsNeeded
		if embedTarget == "" {
			embedTarget = req.Description
		}
		if embedTarget != "" {
			if floats, embErr := h.mlClient.GenerateEmbedding(r.Context(), embedTarget); embErr == nil && len(floats) > 0 {
				embVector = pgvector.NewVector(floats)
			}
		}
	}

	createdReq, err := h.queries.CreateAssistanceRequest(r.Context(), database.CreateAssistanceRequestParams{
		RequesterID:            requesterID,
		TrackingCode:           trackingCode,
		Category:               category,
		QuantityNeeded:         req.Amount,
		Description:            textToPgText(fullDescription),
		Priority:               priority,
		Status:                 database.RequestStatusSUBMITTED,
		Location:               location,
		AddressText:            textToPgText(req.AddressText),
		ContactPhoneEncrypted:  req.PhoneNumber,
		RequesterNameEncrypted: req.Name,
		Embedding:              embVector,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create assistance request: %v", err))
		return
	}

	var requesterIDStr *string
	if createdReq.RequesterID.Valid {
		str := uuid.UUID(createdReq.RequesterID.Bytes).String()
		requesterIDStr = &str
	}

	reqUUID := uuid.UUID(createdReq.ID.Bytes)
	if h.coordinator != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if _, err := h.coordinator.TriggerDispatch(bgCtx, reqUUID); err != nil && !errors.Is(err, dispatch.ErrNoCandidates) {
				log.Printf("[Dispatch] Initial dispatch error for request %s: %v\n", reqUUID, err)
			}
		}()
	}

	respondWithJSON(w, http.StatusCreated, AssistanceRequestResponse{
		ID:             reqUUID.String(),
		RequesterID:    requesterIDStr,
		TrackingCode:   createdReq.TrackingCode,
		Category:       string(createdReq.Category),
		QuantityNeeded: createdReq.QuantityNeeded,
		Description:    pgTextToString(createdReq.Description),
		Priority:       string(createdReq.Priority),
		Status:         string(createdReq.Status),
		DispatchStatus: string(createdReq.DispatchStatus),
		Location:       createdReq.Location,
		AddressText:    pgTextToString(createdReq.AddressText),
		RequesterName:  createdReq.RequesterNameEncrypted,
		ContactPhone:   createdReq.ContactPhoneEncrypted,
		CreatedAt:      createdReq.CreatedAt.Time,
		UpdatedAt:      createdReq.UpdatedAt.Time,
	})
}

func (h *APIHandler) ListAssistanceRequests(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	offset := int32(0)

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = int32(parsed)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	var requests []database.AssistanceRequest
	var err error

	requesterIDStr := r.URL.Query().Get("requester_id")
	if requesterIDStr != "" {
		parsed, parseErr := uuid.Parse(requesterIDStr)
		if parseErr != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid requester_id format")
			return
		}
		requests, err = h.queries.ListAssistanceRequestsByRequester(r.Context(), database.ListAssistanceRequestsByRequesterParams{
			RequesterID: pgtype.UUID{Bytes: parsed, Valid: true},
			Limit:       limit,
			Offset:      offset,
		})
	} else {
		requests, err = h.queries.ListAssistanceRequests(r.Context(), database.ListAssistanceRequestsParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve assistance requests")
		return
	}

	res := make([]AssistanceRequestResponse, 0, len(requests))
	for _, req := range requests {
		var requesterIDStr *string
		if req.RequesterID.Valid {
			str := uuid.UUID(req.RequesterID.Bytes).String()
			requesterIDStr = &str
		}
		var coordIDStr *string
		if req.AssignedCoordinatorID.Valid {
			str := uuid.UUID(req.AssignedCoordinatorID.Bytes).String()
			coordIDStr = &str
		}
		var matchedProviderIDStr *string
		if req.MatchedProviderID.Valid {
			str := uuid.UUID(req.MatchedProviderID.Bytes).String()
			matchedProviderIDStr = &str
		}

		res = append(res, AssistanceRequestResponse{
			ID:                    uuid.UUID(req.ID.Bytes).String(),
			RequesterID:           requesterIDStr,
			TrackingCode:          req.TrackingCode,
			Category:              string(req.Category),
			QuantityNeeded:        req.QuantityNeeded,
			Description:           pgTextToString(req.Description),
			Priority:              string(req.Priority),
			Status:                string(req.Status),
			DispatchStatus:        string(req.DispatchStatus),
			MatchedProviderID:     matchedProviderIDStr,
			AssignedCoordinatorID: coordIDStr,
			Location:              req.Location,
			AddressText:           pgTextToString(req.AddressText),
			RequesterName:         req.RequesterNameEncrypted,
			ContactPhone:          req.ContactPhoneEncrypted,
			CreatedAt:             req.CreatedAt.Time,
			UpdatedAt:             req.UpdatedAt.Time,
		})
	}

	respondWithJSON(w, http.StatusOK, res)
}

func (h *APIHandler) ListMyAssistanceRequests(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	if _, err := uuid.Parse(claims.AccountID); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}
	clone := r.Clone(r.Context())
	query := clone.URL.Query()
	query.Set("requester_id", claims.AccountID)
	clone.URL.RawQuery = query.Encode()
	h.ListAssistanceRequests(w, clone)
}

func (h *APIHandler) ListProviderAssistanceRequests(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetClaims(r.Context()); !ok {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	limit := 100
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id, requester_id, tracking_code, category, quantity_needed, description,
		       priority, status, dispatch_status, matched_provider_id,
		       assigned_coordinator_id, location, address_text,
		       requester_name_encrypted, contact_phone_encrypted, created_at, updated_at,
		       ST_Y(location::geometry), ST_X(location::geometry)
		FROM assistance_requests
		ORDER BY CASE priority
		           WHEN 'CRITICAL' THEN 0
		           WHEN 'HIGH' THEN 1
		           WHEN 'MEDIUM' THEN 2
		           ELSE 3
		         END, created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve provider requests")
		return
	}
	defer rows.Close()

	requests := make([]ProviderAssistanceRequestResponse, 0)
	for rows.Next() {
		var req database.AssistanceRequest
		var latitude, longitude float64
		if err := rows.Scan(
			&req.ID, &req.RequesterID, &req.TrackingCode, &req.Category, &req.QuantityNeeded,
			&req.Description, &req.Priority, &req.Status, &req.DispatchStatus,
			&req.MatchedProviderID, &req.AssignedCoordinatorID, &req.Location,
			&req.AddressText, &req.RequesterNameEncrypted, &req.ContactPhoneEncrypted,
			&req.CreatedAt, &req.UpdatedAt, &latitude, &longitude,
		); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to read provider requests")
			return
		}
		requests = append(requests, ProviderAssistanceRequestResponse{
			AssistanceRequestResponse: assistanceRequestResponse(req),
			Latitude:                  latitude,
			Longitude:                 longitude,
		})
	}
	if err := rows.Err(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read provider requests")
		return
	}
	respondWithJSON(w, http.StatusOK, requests)
}

func (h *APIHandler) GetAssistanceRequestByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid assistance request ID format")
		return
	}

	req, err := h.queries.GetAssistanceRequestByID(r.Context(), pgtype.UUID{Bytes: parsedID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Assistance request not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve assistance request")
		return
	}

	var requesterIDStr *string
	if req.RequesterID.Valid {
		str := uuid.UUID(req.RequesterID.Bytes).String()
		requesterIDStr = &str
	}
	var coordIDStr *string
	if req.AssignedCoordinatorID.Valid {
		str := uuid.UUID(req.AssignedCoordinatorID.Bytes).String()
		coordIDStr = &str
	}
	var matchedProviderIDStr *string
	if req.MatchedProviderID.Valid {
		str := uuid.UUID(req.MatchedProviderID.Bytes).String()
		matchedProviderIDStr = &str
	}

	respondWithJSON(w, http.StatusOK, AssistanceRequestResponse{
		ID:                    uuid.UUID(req.ID.Bytes).String(),
		RequesterID:           requesterIDStr,
		TrackingCode:          req.TrackingCode,
		Category:              string(req.Category),
		QuantityNeeded:        req.QuantityNeeded,
		Description:           pgTextToString(req.Description),
		Priority:              string(req.Priority),
		Status:                string(req.Status),
		DispatchStatus:        string(req.DispatchStatus),
		MatchedProviderID:     matchedProviderIDStr,
		AssignedCoordinatorID: coordIDStr,
		Location:              req.Location,
		AddressText:           pgTextToString(req.AddressText),
		RequesterName:         req.RequesterNameEncrypted,
		ContactPhone:          req.ContactPhoneEncrypted,
		CreatedAt:             req.CreatedAt.Time,
		UpdatedAt:             req.UpdatedAt.Time,
	})
}

func (h *APIHandler) TrackAssistanceRequest(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		respondWithError(w, http.StatusBadRequest, "Tracking code is required")
		return
	}

	req, err := h.queries.GetAssistanceRequestByTrackingCode(r.Context(), code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Assistance request not found for this tracking code")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve tracking information")
		return
	}

	var requesterIDStr *string
	if req.RequesterID.Valid {
		str := uuid.UUID(req.RequesterID.Bytes).String()
		requesterIDStr = &str
	}
	var coordIDStr *string
	if req.AssignedCoordinatorID.Valid {
		str := uuid.UUID(req.AssignedCoordinatorID.Bytes).String()
		coordIDStr = &str
	}
	var matchedProviderIDStr *string
	if req.MatchedProviderID.Valid {
		str := uuid.UUID(req.MatchedProviderID.Bytes).String()
		matchedProviderIDStr = &str
	}

	var matchedProviderName *string
	var matchedProviderPhone *string
	var handshakeCode *string
	if req.DispatchStatus == database.DispatchStatusMATCHED {
		if match, matchErr := h.queries.GetMatchByRequestID(r.Context(), req.ID); matchErr == nil {
			handshakeCode = &match.HandshakeCode
			matchedProviderName = &match.ProviderName
			matchedProviderPhone = &match.ProviderPhone
		}
	}

	respondWithJSON(w, http.StatusOK, AssistanceRequestResponse{
		ID:                    uuid.UUID(req.ID.Bytes).String(),
		RequesterID:           requesterIDStr,
		TrackingCode:          req.TrackingCode,
		Category:              string(req.Category),
		QuantityNeeded:        req.QuantityNeeded,
		Description:           pgTextToString(req.Description),
		Priority:              string(req.Priority),
		Status:                string(req.Status),
		DispatchStatus:        string(req.DispatchStatus),
		MatchedProviderID:     matchedProviderIDStr,
		MatchedProviderName:   matchedProviderName,
		MatchedProviderPhone:  matchedProviderPhone,
		HandshakeCode:         handshakeCode,
		AssignedCoordinatorID: coordIDStr,
		Location:              req.Location,
		AddressText:           pgTextToString(req.AddressText),
		RequesterName:         req.RequesterNameEncrypted,
		ContactPhone:          req.ContactPhoneEncrypted,
		CreatedAt:             req.CreatedAt.Time,
		UpdatedAt:             req.UpdatedAt.Time,
	})
}
