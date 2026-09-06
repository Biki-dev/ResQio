package handlers

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"go-sse-server/internal/dispatch"
	"go-sse-server/internal/middleware"
)

type ActivePingResponse struct {
	PingID           string    `json:"ping_id"`
	RequestID        string    `json:"request_id"`
	TrackingCode     string    `json:"tracking_code"`
	Category         string    `json:"category"`
	QuantityNeeded   int32     `json:"quantity_needed"`
	Description      string    `json:"description"`
	AddressText      string    `json:"address_text"`
	DistanceMeters   int64     `json:"distance_meters"`
	DistanceKm       float64   `json:"distance_km"`
	ExpiresAt        time.Time `json:"expires_at"`
	RemainingSeconds int64     `json:"remaining_seconds"`
	CreatedAt        time.Time `json:"created_at"`
}

type MatchResponse struct {
	MatchID       string    `json:"match_id"`
	RequestID     string    `json:"request_id"`
	ProviderID    string    `json:"provider_id"`
	HandshakeCode string    `json:"handshake_code"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// GetActiveProviderPing returns the latest pending ping for the authenticated provider
func (h *APIHandler) GetActiveProviderPing(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	providerUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	ping, err := h.queries.GetActivePingForProvider(r.Context(), pgtype.UUID{Bytes: providerUUID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithJSON(w, http.StatusOK, map[string]interface{}{
				"ping": nil,
			})
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to check active pings")
		return
	}

	remaining := int64(time.Until(ping.ExpiresAt.Time).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	distKm := math.Round((float64(ping.DistanceMeters)/1000.0)*10) / 10

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"ping": ActivePingResponse{
			PingID:           uuid.UUID(ping.PingID.Bytes).String(),
			RequestID:        uuid.UUID(ping.RequestID.Bytes).String(),
			TrackingCode:     ping.TrackingCode,
			Category:         string(ping.Category),
			QuantityNeeded:   ping.QuantityNeeded,
			Description:      pgTextToString(ping.Description),
			AddressText:      pgTextToString(ping.AddressText),
			DistanceMeters:   ping.DistanceMeters,
			DistanceKm:       distKm,
			ExpiresAt:        ping.ExpiresAt.Time,
			RemainingSeconds: remaining,
			CreatedAt:        ping.CreatedAt.Time,
		},
	})
}

// AcceptPing handles provider acceptance of an emergency dispatch ping
func (h *APIHandler) AcceptPing(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	providerUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	pingIDStr := r.PathValue("id")
	pingUUID, err := uuid.Parse(pingIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ping ID format")
		return
	}

	match, err := h.coordinator.HandleProviderAccept(r.Context(), pingUUID, providerUUID)
	if err != nil {
		switch {
		case errors.Is(err, dispatch.ErrPingNotFound):
			respondWithError(w, http.StatusNotFound, "Dispatch ping not found")
		case errors.Is(err, dispatch.ErrUnauthorizedProvider):
			respondWithError(w, http.StatusForbidden, "You are not authorized to respond to this ping")
		case errors.Is(err, dispatch.ErrPingExpired):
			respondWithError(w, http.StatusGone, "This dispatch ping has expired and was offered to another provider")
		case errors.Is(err, dispatch.ErrPingNotPending):
			respondWithError(w, http.StatusConflict, "This ping is no longer pending")
		default:
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to accept ping: %v", err))
		}
		return
	}

	respondWithJSON(w, http.StatusOK, MatchResponse{
		MatchID:       uuid.UUID(match.ID.Bytes).String(),
		RequestID:     uuid.UUID(match.RequestID.Bytes).String(),
		ProviderID:    uuid.UUID(match.ProviderID.Bytes).String(),
		HandshakeCode: match.HandshakeCode,
		Status:        string(match.Status),
		CreatedAt:     match.CreatedAt.Time,
	})
}

// RejectPing handles provider rejection of a ping and triggers the cascade
func (h *APIHandler) RejectPing(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountID == "" {
		respondWithError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	providerUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID")
		return
	}

	pingIDStr := r.PathValue("id")
	pingUUID, err := uuid.Parse(pingIDStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ping ID format")
		return
	}

	if err := h.coordinator.HandleProviderReject(r.Context(), pingUUID, providerUUID); err != nil {
		switch {
		case errors.Is(err, dispatch.ErrPingNotFound):
			respondWithError(w, http.StatusNotFound, "Dispatch ping not found")
		case errors.Is(err, dispatch.ErrUnauthorizedProvider):
			respondWithError(w, http.StatusForbidden, "You are not authorized to respond to this ping")
		case errors.Is(err, dispatch.ErrPingNotPending):
			respondWithError(w, http.StatusConflict, "This ping is no longer pending")
		default:
			respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reject ping: %v", err))
		}
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Ping rejected. Cascade initiated to next candidate.",
	})
}

// GetExhaustedAlerts returns requests that could not be matched with any provider
func (h *APIHandler) GetExhaustedAlerts(w http.ResponseWriter, r *http.Request) {
	requests, err := h.queries.GetExhaustedRequests(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve exhausted requests")
		return
	}

	type ExhaustedAlert struct {
		ID             string    `json:"id"`
		TrackingCode   string    `json:"tracking_code"`
		Category       string    `json:"category"`
		QuantityNeeded int32     `json:"quantity_needed"`
		Description    string    `json:"description"`
		Priority       string    `json:"priority"`
		Status         string    `json:"status"`
		DispatchStatus string    `json:"dispatch_status"`
		AddressText    string    `json:"address_text"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}

	alerts := make([]ExhaustedAlert, 0, len(requests))
	for _, req := range requests {
		alerts = append(alerts, ExhaustedAlert{
			ID:             uuid.UUID(req.ID.Bytes).String(),
			TrackingCode:   req.TrackingCode,
			Category:       string(req.Category),
			QuantityNeeded: req.QuantityNeeded,
			Description:    pgTextToString(req.Description),
			Priority:       string(req.Priority),
			Status:         string(req.Status),
			DispatchStatus: string(req.DispatchStatus),
			AddressText:    pgTextToString(req.AddressText),
			CreatedAt:      req.CreatedAt.Time,
			UpdatedAt:      req.UpdatedAt.Time,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

type AdminOverviewResponse struct {
	Users             int64 `json:"users"`
	Providers         int64 `json:"providers"`
	OpenRequests      int64 `json:"open_requests"`
	CriticalRequests  int64 `json:"critical_requests"`
	ActiveHazards     int64 `json:"active_hazards"`
	ActiveCamps       int64 `json:"active_camps"`
	PendingDispatches int64 `json:"pending_dispatches"`
	ExhaustedRequests int64 `json:"exhausted_requests"`
}

func (h *APIHandler) GetAdminOverview(w http.ResponseWriter, r *http.Request) {
	var overview AdminOverviewResponse
	queries := []struct {
		value *int64
		sql   string
	}{
		{&overview.Users, `SELECT COUNT(*) FROM users`},
		{&overview.Providers, `SELECT COUNT(*) FROM providers`},
		{&overview.OpenRequests, `SELECT COUNT(*) FROM assistance_requests WHERE status NOT IN ('FULFILLED', 'CANCELLED')`},
		{&overview.CriticalRequests, `SELECT COUNT(*) FROM assistance_requests WHERE priority = 'CRITICAL' AND status NOT IN ('FULFILLED', 'CANCELLED')`},
		{&overview.ActiveHazards, `SELECT COUNT(*) FROM road_hazards WHERE is_verified = FALSE AND created_at >= NOW() - INTERVAL '30 days'`},
		{&overview.ActiveCamps, `SELECT COUNT(*) FROM distribution_camps WHERE is_active = TRUE`},
		{&overview.PendingDispatches, `SELECT COUNT(*) FROM dispatch_pings WHERE status = 'PENDING' AND expires_at > NOW()`},
		{&overview.ExhaustedRequests, `SELECT COUNT(*) FROM assistance_requests WHERE dispatch_status = 'EXHAUSTED'`},
	}
	for _, query := range queries {
		if err := h.pool.QueryRow(r.Context(), query.sql).Scan(query.value); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to load admin overview")
			return
		}
	}
	respondWithJSON(w, http.StatusOK, overview)
}
