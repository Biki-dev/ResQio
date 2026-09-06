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

	"go-sse-server/internal/auth"
	"go-sse-server/internal/database"
	"go-sse-server/internal/middleware"
)

type CreateMutualAidRequest struct {
	ItemName    string   `json:"item_name"`
	Quantity    int32    `json:"quantity"`
	Description string   `json:"description,omitempty"`
	Location    string   `json:"location,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
}

type MutualAidResponse struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	ItemName        string    `json:"item_name"`
	Quantity        int32     `json:"quantity"`
	Description     string    `json:"description"`
	Location        string    `json:"location"`
	IsClaimed       bool      `json:"is_claimed"`
	ClaimedByUserID *string   `json:"claimed_by_user_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

func (h *APIHandler) CreateMutualAidItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" || claims.AccountType != auth.AccountTypeUser {
		respondWithError(w, http.StatusUnauthorized, "User authentication required to post mutual aid item")
		return
	}

	userUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID in token")
		return
	}

	var req CreateMutualAidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.ItemName = strings.TrimSpace(req.ItemName)
	if req.ItemName == "" {
		respondWithError(w, http.StatusBadRequest, "item_name is required")
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	location := strings.TrimSpace(req.Location)
	if location == "" && req.Latitude != nil && req.Longitude != nil {
		location = fmt.Sprintf("POINT(%f %f)", *req.Longitude, *req.Latitude)
	}
	if location == "" {
		location = "POINT(0 0)"
	}

	item, err := h.queries.CreateMutualAidItem(r.Context(), database.CreateMutualAidItemParams{
		UserID:      pgtype.UUID{Bytes: userUUID, Valid: true},
		ItemName:    req.ItemName,
		Quantity:    req.Quantity,
		Description: textToPgText(req.Description),
		Location:    location,
		IsClaimed:   false,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create mutual aid item: %v", err))
		return
	}

	respondWithJSON(w, http.StatusCreated, MutualAidResponse{
		ID:          uuid.UUID(item.ID.Bytes).String(),
		UserID:      uuid.UUID(item.UserID.Bytes).String(),
		ItemName:    item.ItemName,
		Quantity:    item.Quantity,
		Description: pgTextToString(item.Description),
		Location:    item.Location,
		IsClaimed:   item.IsClaimed,
		CreatedAt:   item.CreatedAt.Time,
	})
}

func (h *APIHandler) ListMutualAidItems(w http.ResponseWriter, r *http.Request) {
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

	onlyAvailable := r.URL.Query().Get("available") == "true"
	var items []database.MutualAidItem
	var err error

	if onlyAvailable {
		items, err = h.queries.ListAvailableMutualAidItems(r.Context(), database.ListAvailableMutualAidItemsParams{
			Limit:  limit,
			Offset: offset,
		})
	} else {
		items, err = h.queries.ListMutualAidItems(r.Context(), database.ListMutualAidItemsParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve mutual aid items")
		return
	}

	res := make([]MutualAidResponse, 0, len(items))
	for _, item := range items {
		var claimedBy *string
		if item.ClaimedByUserID.Valid {
			str := uuid.UUID(item.ClaimedByUserID.Bytes).String()
			claimedBy = &str
		}

		res = append(res, MutualAidResponse{
			ID:              uuid.UUID(item.ID.Bytes).String(),
			UserID:          uuid.UUID(item.UserID.Bytes).String(),
			ItemName:        item.ItemName,
			Quantity:        item.Quantity,
			Description:     pgTextToString(item.Description),
			Location:        item.Location,
			IsClaimed:       item.IsClaimed,
			ClaimedByUserID: claimedBy,
			CreatedAt:       item.CreatedAt.Time,
		})
	}

	respondWithJSON(w, http.StatusOK, res)
}

func (h *APIHandler) ListMyMutualAidItems(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountType != auth.AccountTypeUser || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "User authentication required")
		return
	}
	userID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	limit, offset := int32(20), int32(0)
	if value, parseErr := strconv.Atoi(r.URL.Query().Get("limit")); parseErr == nil && value > 0 && value <= 100 {
		limit = int32(value)
	}
	if value, parseErr := strconv.Atoi(r.URL.Query().Get("offset")); parseErr == nil && value >= 0 {
		offset = int32(value)
	}
	rows, err := h.pool.Query(r.Context(), `SELECT id, user_id, item_name, quantity, description, ST_AsText(location), is_claimed, claimed_by_user_id, created_at FROM mutual_aid_items WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		respondWithError(w, 500, "Failed to retrieve your community aid posts")
		return
	}
	defer rows.Close()
	items := make([]MutualAidResponse, 0)
	for rows.Next() {
		var item database.MutualAidItem
		if err := rows.Scan(&item.ID, &item.UserID, &item.ItemName, &item.Quantity, &item.Description, &item.Location, &item.IsClaimed, &item.ClaimedByUserID, &item.CreatedAt); err != nil {
			respondWithError(w, 500, "Failed to read your community aid posts")
			return
		}
		var claimedBy *string
		if item.ClaimedByUserID.Valid {
			value := uuid.UUID(item.ClaimedByUserID.Bytes).String()
			claimedBy = &value
		}
		items = append(items, MutualAidResponse{ID: uuid.UUID(item.ID.Bytes).String(), UserID: uuid.UUID(item.UserID.Bytes).String(), ItemName: item.ItemName, Quantity: item.Quantity, Description: pgTextToString(item.Description), Location: item.Location, IsClaimed: item.IsClaimed, ClaimedByUserID: claimedBy, CreatedAt: item.CreatedAt.Time})
	}
	respondWithJSON(w, 200, items)
}

func (h *APIHandler) ClaimMutualAidItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" || claims.AccountType != auth.AccountTypeUser {
		respondWithError(w, http.StatusUnauthorized, "User authentication required to claim mutual aid item")
		return
	}

	claimerUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID in token")
		return
	}

	idStr := r.PathValue("id")
	itemID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid mutual aid item ID")
		return
	}

	item, err := h.queries.ClaimMutualAidItem(r.Context(), database.ClaimMutualAidItemParams{
		ID:              pgtype.UUID{Bytes: itemID, Valid: true},
		ClaimedByUserID: pgtype.UUID{Bytes: claimerUUID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Item not found or already claimed")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to claim item")
		return
	}

	claimedByStr := uuid.UUID(item.ClaimedByUserID.Bytes).String()
	respondWithJSON(w, http.StatusOK, MutualAidResponse{
		ID:              uuid.UUID(item.ID.Bytes).String(),
		UserID:          uuid.UUID(item.UserID.Bytes).String(),
		ItemName:        item.ItemName,
		Quantity:        item.Quantity,
		Description:     pgTextToString(item.Description),
		Location:        item.Location,
		IsClaimed:       item.IsClaimed,
		ClaimedByUserID: &claimedByStr,
		CreatedAt:       item.CreatedAt.Time,
	})
}
