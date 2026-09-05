package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-sse-server/internal/auth"
	"go-sse-server/internal/middleware"
)

type DistributionCampRequest struct {
	CampName          string  `json:"camp_name"`
	AddressText       string  `json:"address_text"`
	ItemsAvailable    string  `json:"items_available"`
	DistributionStart string  `json:"distribution_start"`
	DistributionEnd   string  `json:"distribution_end"`
	ContactPhone      string  `json:"contact_phone,omitempty"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
}

type DistributionCampResponse struct {
	ID                string    `json:"id"`
	ProviderID        string    `json:"provider_id"`
	ProviderName      string    `json:"provider_name"`
	CampName          string    `json:"camp_name"`
	AddressText       string    `json:"address_text"`
	ItemsAvailable    string    `json:"items_available"`
	DistributionStart string    `json:"distribution_start"`
	DistributionEnd   string    `json:"distribution_end"`
	ContactPhone      string    `json:"contact_phone"`
	Latitude          float64   `json:"latitude"`
	Longitude         float64   `json:"longitude"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (h *APIHandler) CreateDistributionCamp(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountType != auth.AccountTypeProvider || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Provider authentication required")
		return
	}
	providerID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	var input DistributionCampRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	input.CampName = strings.TrimSpace(input.CampName)
	input.AddressText = strings.TrimSpace(input.AddressText)
	input.ItemsAvailable = strings.TrimSpace(input.ItemsAvailable)
	input.DistributionStart = strings.TrimSpace(input.DistributionStart)
	input.DistributionEnd = strings.TrimSpace(input.DistributionEnd)
	if input.CampName == "" || input.AddressText == "" || input.ItemsAvailable == "" || input.DistributionStart == "" || input.DistributionEnd == "" {
		respondWithError(w, http.StatusBadRequest, "camp_name, address_text, items_available, and distribution hours are required")
		return
	}
	if input.Latitude < -90 || input.Latitude > 90 || input.Longitude < -180 || input.Longitude > 180 {
		respondWithError(w, http.StatusBadRequest, "valid latitude and longitude are required")
		return
	}
	if _, err := time.Parse("15:04", input.DistributionStart); err != nil {
		respondWithError(w, http.StatusBadRequest, "distribution_start must use HH:MM format")
		return
	}
	if _, err := time.Parse("15:04", input.DistributionEnd); err != nil {
		respondWithError(w, http.StatusBadRequest, "distribution_end must use HH:MM format")
		return
	}

	var response DistributionCampResponse
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO distribution_camps
			(provider_id, camp_name, address_text, location, items_available,
			 distribution_start, distribution_end, contact_phone)
		VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326), $6, $7, $8, $9)
		RETURNING id, provider_id, camp_name, address_text, items_available,
		          distribution_start::text, distribution_end::text, COALESCE(contact_phone, ''),
		          ST_Y(location::geometry), ST_X(location::geometry), is_active, created_at, updated_at`,
		providerID, input.CampName, input.AddressText, input.Longitude, input.Latitude,
		input.ItemsAvailable, input.DistributionStart, input.DistributionEnd, input.ContactPhone,
	).Scan(&response.ID, &response.ProviderID, &response.CampName, &response.AddressText,
		&response.ItemsAvailable, &response.DistributionStart, &response.DistributionEnd,
		&response.ContactPhone, &response.Latitude, &response.Longitude, &response.IsActive,
		&response.CreatedAt, &response.UpdatedAt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create distribution camp: %v", err))
		return
	}
	_ = h.pool.QueryRow(r.Context(), `SELECT name FROM providers WHERE id = $1`, providerID).Scan(&response.ProviderName)
	respondWithJSON(w, http.StatusCreated, response)
}

func (h *APIHandler) ListDistributionCamps(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if value := r.URL.Query().Get("limit"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, c.provider_id, p.name, c.camp_name, c.address_text, c.items_available,
		       c.distribution_start::text, c.distribution_end::text, COALESCE(c.contact_phone, ''),
		       ST_Y(c.location::geometry), ST_X(c.location::geometry), c.is_active, c.created_at, c.updated_at
		FROM distribution_camps c
		JOIN providers p ON p.id = c.provider_id
		WHERE c.is_active = TRUE
		ORDER BY c.updated_at DESC
		LIMIT $1`, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve distribution camps")
		return
	}
	defer rows.Close()

	camps := make([]DistributionCampResponse, 0)
	for rows.Next() {
		var camp DistributionCampResponse
		if err := rows.Scan(&camp.ID, &camp.ProviderID, &camp.ProviderName, &camp.CampName,
			&camp.AddressText, &camp.ItemsAvailable, &camp.DistributionStart, &camp.DistributionEnd,
			&camp.ContactPhone, &camp.Latitude, &camp.Longitude, &camp.IsActive,
			&camp.CreatedAt, &camp.UpdatedAt); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to read distribution camps")
			return
		}
		camps = append(camps, camp)
	}
	if err := rows.Err(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to read distribution camps")
		return
	}
	respondWithJSON(w, http.StatusOK, camps)
}

func (h *APIHandler) UpdateDistributionCamp(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountType != auth.AccountTypeProvider || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Provider authentication required")
		return
	}
	providerID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}
	campID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid camp ID")
		return
	}
	var input DistributionCampRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	input.CampName = strings.TrimSpace(input.CampName)
	input.AddressText = strings.TrimSpace(input.AddressText)
	input.ItemsAvailable = strings.TrimSpace(input.ItemsAvailable)
	if input.CampName == "" || input.AddressText == "" || input.ItemsAvailable == "" {
		respondWithError(w, http.StatusBadRequest, "camp_name, address_text, and items_available are required")
		return
	}
	if _, err := time.Parse("15:04", input.DistributionStart); err != nil {
		respondWithError(w, http.StatusBadRequest, "distribution_start must use HH:MM format")
		return
	}
	if _, err := time.Parse("15:04", input.DistributionEnd); err != nil {
		respondWithError(w, http.StatusBadRequest, "distribution_end must use HH:MM format")
		return
	}

	var response DistributionCampResponse
	err = h.pool.QueryRow(r.Context(), `
		UPDATE distribution_camps
		SET camp_name = $1, address_text = $2,
		    location = ST_SetSRID(ST_MakePoint($3, $4), 4326),
		    items_available = $5, distribution_start = $6, distribution_end = $7,
		    contact_phone = $8, updated_at = NOW()
		WHERE id = $9 AND provider_id = $10
		RETURNING id, provider_id, camp_name, address_text, items_available,
		          distribution_start::text, distribution_end::text, COALESCE(contact_phone, ''),
		          ST_Y(location::geometry), ST_X(location::geometry), is_active, created_at, updated_at`,
		input.CampName, input.AddressText, input.Longitude, input.Latitude, input.ItemsAvailable,
		input.DistributionStart, input.DistributionEnd, input.ContactPhone, campID, providerID,
	).Scan(&response.ID, &response.ProviderID, &response.CampName, &response.AddressText,
		&response.ItemsAvailable, &response.DistributionStart, &response.DistributionEnd,
		&response.ContactPhone, &response.Latitude, &response.Longitude, &response.IsActive,
		&response.CreatedAt, &response.UpdatedAt)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Distribution camp not found")
		return
	}
	_ = h.pool.QueryRow(r.Context(), `SELECT name FROM providers WHERE id = $1`, providerID).Scan(&response.ProviderName)
	respondWithJSON(w, http.StatusOK, response)
}

func (h *APIHandler) DeleteDistributionCamp(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountType != auth.AccountTypeProvider || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Provider authentication required")
		return
	}
	providerID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}
	campID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid camp ID")
		return
	}
	result, err := h.pool.Exec(r.Context(), `
		UPDATE distribution_camps SET is_active = FALSE, updated_at = NOW()
		WHERE id = $1 AND provider_id = $2 AND is_active = TRUE`, campID, providerID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete distribution camp")
		return
	}
	if result.RowsAffected() == 0 {
		respondWithError(w, http.StatusNotFound, "Distribution camp not found")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Distribution camp removed"})
}
