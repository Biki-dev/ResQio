package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go-sse-server/internal/database"
	"go-sse-server/internal/middleware"
)

type AdminAssignRequest struct {
	ProviderID string `json:"provider_id"`
}

type AdminUser struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	FullName  string    `json:"full_name"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type AdminProvider struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	State      string    `json:"state"`
	District   string    `json:"district"`
	IsActive   bool      `json:"is_active"`
	IsVerified bool      `json:"is_verified"`
	CreatedAt  time.Time `json:"created_at"`
}

type AdminAuditLog struct {
	ID          string          `json:"id"`
	AdminUserID *string         `json:"admin_user_id,omitempty"`
	Action      string          `json:"action"`
	TargetType  string          `json:"target_type"`
	TargetID    *string         `json:"target_id,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

func adminActorID(r *http.Request) (uuid.UUID, bool) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(claims.AccountID)
	return id, err == nil
}

func (h *APIHandler) recordAdminAudit(r *http.Request, action, targetType string, targetID *uuid.UUID, details map[string]string) {
	adminID, ok := adminActorID(r)
	if !ok {
		return
	}
	payload, _ := json.Marshal(details)
	_, _ = h.pool.Exec(r.Context(), `INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, details) VALUES ($1, $2, $3, $4, $5)`, adminID, action, targetType, targetID, payload)
}

func (h *APIHandler) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminPagination(r)
	rows, err := h.pool.Query(r.Context(), `SELECT id, phone, full_name, role, is_active, created_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve users")
		return
	}
	defer rows.Close()
	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Phone, &u.FullName, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
			respondWithError(w, 500, "Failed to read users")
			return
		}
		users = append(users, u)
	}
	respondWithJSON(w, http.StatusOK, users)
}

func (h *APIHandler) UpdateAdminUserRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, 400, "Invalid user ID")
		return
	}
	var input struct {
		Role string `json:"role"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		respondWithError(w, 400, "Invalid request payload")
		return
	}
	role := database.UserRole(strings.ToUpper(strings.TrimSpace(input.Role)))
	allowed := map[database.UserRole]bool{database.UserRoleGUEST: true, database.UserRoleVICTIM: true, database.UserRolePUBLIC: true, database.UserRolePROVIDER: true, database.UserRoleCOORDINATOR: true, database.UserRoleADMIN: true}
	if !allowed[role] {
		respondWithError(w, 400, "Invalid role")
		return
	}
	var user AdminUser
	err = h.pool.QueryRow(r.Context(), `UPDATE users SET role = $1 WHERE id = $2 RETURNING id, phone, full_name, role, is_active, created_at`, role, id).Scan(&user.ID, &user.Phone, &user.FullName, &user.Role, &user.IsActive, &user.CreatedAt)
	if err != nil {
		respondWithError(w, 404, "User not found")
		return
	}
	h.recordAdminAudit(r, "UPDATE_USER_ROLE", "user", &id, map[string]string{"role": string(role)})
	respondWithJSON(w, http.StatusOK, user)
}

func (h *APIHandler) ListAdminProviders(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminPagination(r)
	rows, err := h.pool.Query(r.Context(), `SELECT id, name, email, ph_no, state, dist, is_active, is_verified, created_at FROM providers ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		respondWithError(w, 500, "Failed to retrieve providers")
		return
	}
	defer rows.Close()
	providers := make([]AdminProvider, 0)
	for rows.Next() {
		var p AdminProvider
		if err := rows.Scan(&p.ID, &p.Name, &p.Email, &p.Phone, &p.State, &p.District, &p.IsActive, &p.IsVerified, &p.CreatedAt); err != nil {
			respondWithError(w, 500, "Failed to read providers")
			return
		}
		providers = append(providers, p)
	}
	respondWithJSON(w, 200, providers)
}

func (h *APIHandler) UpdateAdminProviderStatus(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, 400, "Invalid provider ID")
		return
	}
	var input struct {
		IsActive   *bool `json:"is_active"`
		IsVerified *bool `json:"is_verified"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		respondWithError(w, 400, "Invalid request payload")
		return
	}
	var provider AdminProvider
	err = h.pool.QueryRow(r.Context(), `UPDATE providers SET is_active = COALESCE($1, is_active), is_verified = COALESCE($2, is_verified), last_updated_at = NOW() WHERE id = $3 RETURNING id, name, email, ph_no, state, dist, is_active, is_verified, created_at`, input.IsActive, input.IsVerified, id).Scan(&provider.ID, &provider.Name, &provider.Email, &provider.Phone, &provider.State, &provider.District, &provider.IsActive, &provider.IsVerified, &provider.CreatedAt)
	if err != nil {
		respondWithError(w, 404, "Provider not found")
		return
	}
	h.recordAdminAudit(r, "UPDATE_PROVIDER_STATUS", "provider", &id, map[string]string{"is_active": strconv.FormatBool(provider.IsActive), "is_verified": strconv.FormatBool(provider.IsVerified)})
	respondWithJSON(w, 200, provider)
}

func (h *APIHandler) ListAdminAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit, offset := adminPagination(r)
	rows, err := h.pool.Query(r.Context(), `SELECT id, admin_user_id, action, target_type, target_id, details, created_at FROM admin_audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		respondWithError(w, 500, "Failed to retrieve audit logs")
		return
	}
	defer rows.Close()
	logs := make([]AdminAuditLog, 0)
	for rows.Next() {
		var log AdminAuditLog
		if err := rows.Scan(&log.ID, &log.AdminUserID, &log.Action, &log.TargetType, &log.TargetID, &log.Details, &log.CreatedAt); err != nil {
			respondWithError(w, 500, "Failed to read audit logs")
			return
		}
		logs = append(logs, log)
	}
	respondWithJSON(w, 200, logs)
}

func (h *APIHandler) ListAdminHazards(w http.ResponseWriter, r *http.Request) {
	h.ListRoadHazards(w, r)
}

func (h *APIHandler) ListAdminRequests(w http.ResponseWriter, r *http.Request) {
	h.ListAssistanceRequests(w, r)
}

func (h *APIHandler) ListAdminResources(w http.ResponseWriter, r *http.Request) {
	h.ListResources(w, r)
}

func (h *APIHandler) ListAdminCamps(w http.ResponseWriter, r *http.Request) {
	h.ListDistributionCamps(w, r)
}

func (h *APIHandler) VerifyAdminHazard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, 400, "Invalid hazard ID")
		return
	}
	var input struct {
		IsVerified bool `json:"is_verified"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		respondWithError(w, 400, "Invalid request payload")
		return
	}
	var verified bool
	if err := h.pool.QueryRow(r.Context(), `UPDATE road_hazards SET is_verified = $1 WHERE id = $2 RETURNING is_verified`, input.IsVerified, id).Scan(&verified); err != nil {
		respondWithError(w, 404, "Hazard not found")
		return
	}
	h.recordAdminAudit(r, "VERIFY_HAZARD", "hazard", &id, map[string]string{"is_verified": strconv.FormatBool(verified)})
	respondWithJSON(w, 200, map[string]interface{}{"id": id.String(), "is_verified": verified})
}

func (h *APIHandler) RebuildAdminHazardClusters(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, predicted_class, ST_AsText(location), created_at
		FROM road_hazards
		WHERE predicted_class IS NOT NULL AND predicted_class <> ''
		ORDER BY created_at ASC`)
	if err != nil {
		respondWithError(w, 500, "Failed to load hazards for clustering")
		return
	}
	defer rows.Close()
	type hazard struct {
		id              uuid.UUID
		class, location string
		createdAt       time.Time
	}
	hazards := make([]hazard, 0)
	for rows.Next() {
		var item hazard
		if err := rows.Scan(&item.id, &item.class, &item.location, &item.createdAt); err != nil {
			respondWithError(w, 500, "Failed to read hazards for clustering")
			return
		}
		hazards = append(hazards, item)
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		respondWithError(w, 500, "Failed to start cluster rebuild")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), `UPDATE road_hazards SET cluster_id = gen_random_uuid(), cluster_size = 1`); err != nil {
		respondWithError(w, 500, "Failed to reset clusters")
		return
	}
	for _, item := range hazards {
		var clusterID uuid.UUID
		err := tx.QueryRow(r.Context(), `SELECT cluster_id FROM road_hazards WHERE predicted_class = $1 AND created_at < $2 AND ST_DWithin(location::geography, ST_GeomFromText($3, 4326)::geography, 100) ORDER BY created_at ASC LIMIT 1`, item.class, item.createdAt, item.location).Scan(&clusterID)
		if err != nil {
			continue
		}
		if _, err := tx.Exec(r.Context(), `UPDATE road_hazards SET cluster_id = $1 WHERE id = $2`, clusterID, item.id); err != nil {
			respondWithError(w, 500, "Failed to rebuild clusters")
			return
		}
	}
	if _, err := tx.Exec(r.Context(), `UPDATE road_hazards h SET cluster_size = c.size FROM (SELECT cluster_id, COUNT(*) AS size FROM road_hazards GROUP BY cluster_id) c WHERE h.cluster_id = c.cluster_id`); err != nil {
		respondWithError(w, 500, "Failed to update cluster sizes")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		respondWithError(w, 500, "Failed to commit cluster rebuild")
		return
	}
	h.recordAdminAudit(r, "REBUILD_HAZARD_CLUSTERS", "hazard", nil, map[string]string{"radius_meters": "100"})
	respondWithJSON(w, 200, map[string]interface{}{"message": "Hazard clusters rebuilt", "processed": len(hazards)})
}

func (h *APIHandler) AssignAdminRequest(w http.ResponseWriter, r *http.Request) {
	requestID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, 400, "Invalid request ID")
		return
	}
	var input AdminAssignRequest
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		respondWithError(w, 400, "Invalid request payload")
		return
	}
	providerID, err := uuid.Parse(input.ProviderID)
	if err != nil {
		respondWithError(w, 400, "Invalid provider ID")
		return
	}
	var pingID uuid.UUID
	var pingOrder int
	err = h.pool.QueryRow(r.Context(), `
		WITH request_row AS (
			SELECT id, category, quantity_needed FROM assistance_requests
			WHERE id = $1 AND status NOT IN ('FULFILLED', 'CANCELLED') AND dispatch_status <> 'MATCHED'
		), provider_row AS (
			SELECT p.id FROM providers p
			WHERE p.id = $2 AND p.is_active = TRUE AND p.is_verified = TRUE
		), capacity AS (
			SELECT r.provider_id FROM resources r, request_row q
			WHERE r.provider_id = $2 AND r.category = q.category AND r.current_capacity >= q.quantity_needed
			LIMIT 1
		), next_order AS (
			SELECT COALESCE(MAX(ping_order), 0) + 1 AS value FROM dispatch_pings WHERE request_id = $1
		)
		INSERT INTO dispatch_pings (request_id, provider_id, ping_order, expires_at)
		SELECT q.id, p.id, n.value, NOW() + INTERVAL '3 minutes'
		FROM request_row q, provider_row p, capacity c, next_order n
		RETURNING id, ping_order`, requestID, providerID).Scan(&pingID, &pingOrder)
	if err != nil {
		respondWithError(w, 409, "Provider is unavailable, unverified, lacks capacity, or request is already handled")
		return
	}
	if _, err := h.pool.Exec(r.Context(), `UPDATE assistance_requests SET dispatch_status = 'DISPATCHING', updated_at = NOW() WHERE id = $1`, requestID); err != nil {
		respondWithError(w, 500, "Failed to update request dispatch state")
		return
	}
	h.recordAdminAudit(r, "ASSIGN_REQUEST", "assistance_request", &requestID, map[string]string{"provider_id": providerID.String(), "ping_id": pingID.String()})
	respondWithJSON(w, 200, map[string]interface{}{"message": "Provider assignment sent", "ping_id": pingID.String(), "ping_order": pingOrder})
}

func adminPagination(r *http.Request) (int, int) {
	limit, offset := 50, 0
	if value, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && value > 0 && value <= 200 {
		limit = value
	}
	if value, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && value >= 0 {
		offset = value
	}
	return limit, offset
}
