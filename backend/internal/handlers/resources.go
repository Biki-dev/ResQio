package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"

	"go-sse-server/internal/auth"
	"go-sse-server/internal/database"
	"go-sse-server/internal/middleware"
)

type CreateResourceRequest struct {
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Category        string   `json:"category"` // FOOD, WATER, MEDICINE, SHELTER, EQUIPMENT, VOLUNTEER, OTHER
	TotalCapacity   int32    `json:"total_capacity"`
	CurrentCapacity int32    `json:"current_capacity"`
	Unit            string   `json:"unit,omitempty"`
	Location        string   `json:"location,omitempty"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
	ContactPhone    string   `json:"contact_phone,omitempty"`
}

type UpdateResourceCapacityRequest struct {
	CurrentCapacity int32 `json:"current_capacity"`
}

type ResourceResponse struct {
	ID              string    `json:"id"`
	ProviderID      string    `json:"provider_id"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Category        string    `json:"category"`
	TotalCapacity   int32     `json:"total_capacity"`
	CurrentCapacity int32     `json:"current_capacity"`
	Unit            string    `json:"unit"`
	Status          string    `json:"status"`
	Location        string    `json:"location"`
	ContactPhone    string    `json:"contact_phone"`
	LastUpdatedAt   time.Time `json:"last_updated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *APIHandler) CreateResource(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" || claims.AccountType != auth.AccountTypeProvider {
		respondWithError(w, http.StatusUnauthorized, "Provider authentication required to create resource")
		return
	}

	providerUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID in token")
		return
	}

	var req CreateResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		respondWithError(w, http.StatusBadRequest, "title is required")
		return
	}

	category := database.ResourceCategoryOTHER
	switch strings.ToUpper(strings.TrimSpace(req.Category)) {
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
	}

	location := strings.TrimSpace(req.Location)
	if location == "" && req.Latitude != nil && req.Longitude != nil {
		location = fmt.Sprintf("POINT(%f %f)", *req.Longitude, *req.Latitude)
	}
	if location == "" {
		location = "POINT(0 0)"
	}

	if req.CurrentCapacity <= 0 && req.TotalCapacity > 0 {
		req.CurrentCapacity = req.TotalCapacity
	}

	resource, err := h.queries.CreateResource(r.Context(), database.CreateResourceParams{
		ProviderID:      pgtype.UUID{Bytes: providerUUID, Valid: true},
		Title:           req.Title,
		Description:     textToPgText(req.Description),
		Category:        category,
		TotalCapacity:   req.TotalCapacity,
		CurrentCapacity: req.CurrentCapacity,
		Unit:            textToPgText(req.Unit),
		Status:          database.VerificationStatusUNVERIFIED,
		Location:        location,
		ContactPhone:    textToPgText(req.ContactPhone),
		Embedding:       pgvector.Vector{},
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create resource: %v", err))
		return
	}

	respondWithJSON(w, http.StatusCreated, ResourceResponse{
		ID:              uuid.UUID(resource.ID.Bytes).String(),
		ProviderID:      uuid.UUID(resource.ProviderID.Bytes).String(),
		Title:           resource.Title,
		Description:     pgTextToString(resource.Description),
		Category:        string(resource.Category),
		TotalCapacity:   resource.TotalCapacity,
		CurrentCapacity: resource.CurrentCapacity,
		Unit:            pgTextToString(resource.Unit),
		Status:          string(resource.Status),
		Location:        resource.Location,
		ContactPhone:    pgTextToString(resource.ContactPhone),
		LastUpdatedAt:   resource.LastUpdatedAt.Time,
		CreatedAt:       resource.CreatedAt.Time,
	})
}

func (h *APIHandler) ListResources(w http.ResponseWriter, r *http.Request) {
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

	var resources []database.Resource
	var err error

	providerIDStr := r.URL.Query().Get("provider_id")
	categoryStr := r.URL.Query().Get("category")

	if providerIDStr != "" {
		parsed, parseErr := uuid.Parse(providerIDStr)
		if parseErr != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid provider_id")
			return
		}
		resources, err = h.queries.ListResourcesByProvider(r.Context(), database.ListResourcesByProviderParams{
			ProviderID: pgtype.UUID{Bytes: parsed, Valid: true},
			Limit:      limit,
			Offset:     offset,
		})
	} else if categoryStr != "" {
		resources, err = h.queries.ListResourcesByCategory(r.Context(), database.ListResourcesByCategoryParams{
			Category: database.ResourceCategory(strings.ToUpper(categoryStr)),
			Limit:    limit,
			Offset:   offset,
		})
	} else {
		resources, err = h.queries.ListResources(r.Context(), database.ListResourcesParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve resources")
		return
	}

	res := make([]ResourceResponse, 0, len(resources))
	for _, resource := range resources {
		res = append(res, ResourceResponse{
			ID:              uuid.UUID(resource.ID.Bytes).String(),
			ProviderID:      uuid.UUID(resource.ProviderID.Bytes).String(),
			Title:           resource.Title,
			Description:     pgTextToString(resource.Description),
			Category:        string(resource.Category),
			TotalCapacity:   resource.TotalCapacity,
			CurrentCapacity: resource.CurrentCapacity,
			Unit:            pgTextToString(resource.Unit),
			Status:          string(resource.Status),
			Location:        resource.Location,
			ContactPhone:    pgTextToString(resource.ContactPhone),
			LastUpdatedAt:   resource.LastUpdatedAt.Time,
			CreatedAt:       resource.CreatedAt.Time,
		})
	}

	respondWithJSON(w, http.StatusOK, res)
}

func (h *APIHandler) GetResourceByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	parsedID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid resource ID")
		return
	}

	resource, err := h.queries.GetResourceByID(r.Context(), pgtype.UUID{Bytes: parsedID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Resource not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve resource")
		return
	}

	respondWithJSON(w, http.StatusOK, ResourceResponse{
		ID:              uuid.UUID(resource.ID.Bytes).String(),
		ProviderID:      uuid.UUID(resource.ProviderID.Bytes).String(),
		Title:           resource.Title,
		Description:     pgTextToString(resource.Description),
		Category:        string(resource.Category),
		TotalCapacity:   resource.TotalCapacity,
		CurrentCapacity: resource.CurrentCapacity,
		Unit:            pgTextToString(resource.Unit),
		Status:          string(resource.Status),
		Location:        resource.Location,
		ContactPhone:    pgTextToString(resource.ContactPhone),
		LastUpdatedAt:   resource.LastUpdatedAt.Time,
		CreatedAt:       resource.CreatedAt.Time,
	})
}
