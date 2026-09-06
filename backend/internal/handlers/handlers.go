package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-sse-server/internal/auth"
	"go-sse-server/internal/config"
	"go-sse-server/internal/database"
	"go-sse-server/internal/dispatch"
	"go-sse-server/internal/middleware"
	"go-sse-server/internal/ml"
)

type APIHandler struct {
	queries     *database.Queries
	pool        *pgxpool.Pool
	cfg         *config.Config
	coordinator *dispatch.Coordinator
	mlClient    *ml.Client
}

func NewAPIHandler(queries *database.Queries, pool *pgxpool.Pool, cfg *config.Config, coordinator *dispatch.Coordinator, mlClient *ml.Client) *APIHandler {
	return &APIHandler{
		queries:     queries,
		pool:        pool,
		cfg:         cfg,
		coordinator: coordinator,
		mlClient:    mlClient,
	}
}

// User DTOs
type UserRegisterRequest struct {
	Phone     string   `json:"phone"`
	Password  string   `json:"password"`
	Role      string   `json:"role,omitempty"`
	FullName  string   `json:"full_name"`
	Location  string   `json:"location,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type UserLoginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	Role      string    `json:"role"`
	FullName  string    `json:"full_name"`
	Location  string    `json:"location,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	Token       string      `json:"token"`
	AccountType string      `json:"account_type"`
	User        interface{} `json:"profile"`
}

// Provider DTOs
type ProviderRegisterRequest struct {
	Type             string `json:"type"` // "ORGANISATION" or "INDIVIDUAL"
	AuthorizedPerson string `json:"authorized_person,omitempty"`
	Name             string `json:"name"`
	Password         string `json:"password"`
	RegistrationNo   string `json:"registration_no,omitempty"`
	GovtID           string `json:"govt_id"`
	Cin              string `json:"cin,omitempty"`
	Email            string `json:"email"`
	PhNo             string `json:"ph_no"`
	Website          string `json:"website,omitempty"`
	State            string `json:"state"`
	Dist             string `json:"dist"`
	Location         string `json:"location"` // e.g. "POINT(77.2090 28.6139)" or GeoJSON
}

type ProviderLoginRequest struct {
	Email    string `json:"email,omitempty"`
	PhNo     string `json:"ph_no,omitempty"`
	Password string `json:"password"`
}

type ProviderResponse struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	AuthorizedPerson string    `json:"authorized_person,omitempty"`
	Name             string    `json:"name"`
	RegistrationNo   string    `json:"registration_no,omitempty"`
	GovtID           string    `json:"govt_id"`
	Cin              string    `json:"cin,omitempty"`
	Email            string    `json:"email"`
	PhNo             string    `json:"ph_no"`
	Website          string    `json:"website,omitempty"`
	State            string    `json:"state"`
	Dist             string    `json:"dist"`
	Location         string    `json:"location"`
	LastUpdatedAt    time.Time `json:"last_updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// ==================== USER HANDLERS ====================

func (h *APIHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req UserRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	req.FullName = strings.TrimSpace(req.FullName)
	req.Password = strings.TrimSpace(req.Password)

	if req.Phone == "" || req.FullName == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "phone, full_name, and password are required")
		return
	}

	if len(req.Password) < 8 {
		respondWithError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	role := database.UserRolePUBLIC
	if req.Role != "" {
		upperRole := database.UserRole(strings.ToUpper(strings.TrimSpace(req.Role)))
		switch upperRole {
		case database.UserRoleGUEST, database.UserRoleVICTIM, database.UserRolePUBLIC,
			database.UserRolePROVIDER, database.UserRoleCOORDINATOR:
			role = upperRole
		case database.UserRoleADMIN:
			respondWithError(w, http.StatusForbidden, "ADMIN accounts can only be created by an administrator")
			return
		default:
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("invalid role '%s'", req.Role))
			return
		}
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	userLocation := strings.TrimSpace(req.Location)
	if userLocation == "" && req.Latitude != nil && req.Longitude != nil {
		userLocation = fmt.Sprintf("POINT(%.6f %.6f)", *req.Longitude, *req.Latitude)
	}

	userRow, err := h.queries.CreateUser(r.Context(), database.CreateUserParams{
		Phone:        req.Phone,
		PasswordHash: hashedPassword,
		Role:         role,
		FullName:     req.FullName,
		Location:     userLocation,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondWithError(w, http.StatusConflict, "A user with this phone number already exists")
			return
		}
		log.Printf("[UserRegister] Failed to create user: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	userIDStr := uuid.UUID(userRow.ID.Bytes).String()
	token, err := auth.GenerateToken(
		userIDStr,
		auth.AccountTypeUser,
		string(userRow.Role),
		userRow.Phone,
		"",
		h.cfg.JWTSecret,
		h.cfg.JWTExpiration,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate authorization token")
		return
	}

	profile := UserResponse{
		ID:        userIDStr,
		Phone:     userRow.Phone,
		Role:      string(userRow.Role),
		FullName:  userRow.FullName,
		Location:  userRow.Location,
		CreatedAt: userRow.CreatedAt.Time,
	}

	respondWithJSON(w, http.StatusCreated, AuthResponse{
		Token:       token,
		AccountType: string(auth.AccountTypeUser),
		User:        profile,
	})
}

func (h *APIHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	req.Password = strings.TrimSpace(req.Password)

	if req.Phone == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "phone and password are required")
		return
	}

	user, err := h.queries.GetUserByPhone(r.Context(), req.Phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusUnauthorized, "Invalid phone number or password")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to query user")
		return
	}
	var userActive bool
	if err := h.pool.QueryRow(r.Context(), `SELECT is_active FROM users WHERE id = $1`, user.ID).Scan(&userActive); err == nil && !userActive {
		respondWithError(w, http.StatusForbidden, "This user account is inactive")
		return
	}

	if err := auth.CheckPasswordHash(req.Password, user.PasswordHash); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid phone number or password")
		return
	}

	userIDStr := uuid.UUID(user.ID.Bytes).String()
	token, err := auth.GenerateToken(
		userIDStr,
		auth.AccountTypeUser,
		string(user.Role),
		user.Phone,
		"",
		h.cfg.JWTSecret,
		h.cfg.JWTExpiration,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate authorization token")
		return
	}

	profile := UserResponse{
		ID:        userIDStr,
		Phone:     user.Phone,
		Role:      string(user.Role),
		FullName:  user.FullName,
		Location:  user.Location,
		CreatedAt: user.CreatedAt.Time,
	}

	respondWithJSON(w, http.StatusOK, AuthResponse{
		Token:       token,
		AccountType: string(auth.AccountTypeUser),
		User:        profile,
	})
}

func (h *APIHandler) GetUserMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountType != auth.AccountTypeUser {
		respondWithError(w, http.StatusForbidden, "User claims not found in request context")
		return
	}

	parsedUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID format in token")
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), pgtype.UUID{Bytes: parsedUUID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve user profile")
		return
	}

	respondWithJSON(w, http.StatusOK, UserResponse{
		ID:        uuid.UUID(user.ID.Bytes).String(),
		Phone:     user.Phone,
		Role:      string(user.Role),
		FullName:  user.FullName,
		Location:  user.Location,
		CreatedAt: user.CreatedAt.Time,
	})
}

// ==================== PROVIDER HANDLERS ====================

func (h *APIHandler) RegisterProvider(w http.ResponseWriter, r *http.Request) {
	var req ProviderRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Password = strings.TrimSpace(req.Password)
	req.Email = strings.TrimSpace(req.Email)
	req.PhNo = strings.TrimSpace(req.PhNo)
	req.GovtID = strings.TrimSpace(req.GovtID)
	req.State = strings.TrimSpace(req.State)
	req.Dist = strings.TrimSpace(req.Dist)
	req.Location = strings.TrimSpace(req.Location)

	if req.Name == "" || req.Password == "" || req.Email == "" || req.PhNo == "" ||
		req.GovtID == "" || req.State == "" || req.Dist == "" || req.Location == "" {
		respondWithError(w, http.StatusBadRequest, "name, password, email, ph_no, govt_id, state, dist, and location are required")
		return
	}

	if len(req.Password) < 8 {
		respondWithError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	providerType := database.ProviderTypeORGANISATION
	if req.Type != "" {
		upperType := database.ProviderType(strings.ToUpper(strings.TrimSpace(req.Type)))
		if upperType == database.ProviderTypeORGANISATION || upperType == database.ProviderTypeINDIVIDUAL {
			providerType = upperType
		} else {
			respondWithError(w, http.StatusBadRequest, "type must be either 'ORGANISATION' or 'INDIVIDUAL'")
			return
		}
	}

	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	providerRow, err := h.queries.CreateProvider(r.Context(), database.CreateProviderParams{
		Type:             providerType,
		AuthorizedPerson: textToPgText(req.AuthorizedPerson),
		Name:             req.Name,
		PasswordHash:     hashedPassword,
		RegistrationNo:   textToPgText(req.RegistrationNo),
		GovtID:           req.GovtID,
		Cin:              textToPgText(req.Cin),
		Email:            req.Email,
		PhNo:             req.PhNo,
		Website:          textToPgText(req.Website),
		State:            req.State,
		Dist:             req.Dist,
		Location:         req.Location,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			respondWithError(w, http.StatusConflict, "A provider with this email or phone number already exists")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to create provider")
		return
	}

	providerIDStr := uuid.UUID(providerRow.ID.Bytes).String()
	token, err := auth.GenerateToken(
		providerIDStr,
		auth.AccountTypeProvider,
		string(providerRow.Type),
		providerRow.PhNo,
		providerRow.Email,
		h.cfg.JWTSecret,
		h.cfg.JWTExpiration,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate authorization token")
		return
	}

	profile := ProviderResponse{
		ID:               providerIDStr,
		Type:             string(providerRow.Type),
		AuthorizedPerson: pgTextToString(providerRow.AuthorizedPerson),
		Name:             providerRow.Name,
		RegistrationNo:   pgTextToString(providerRow.RegistrationNo),
		GovtID:           providerRow.GovtID,
		Cin:              pgTextToString(providerRow.Cin),
		Email:            providerRow.Email,
		PhNo:             providerRow.PhNo,
		Website:          pgTextToString(providerRow.Website),
		State:            providerRow.State,
		Dist:             providerRow.Dist,
		Location:         providerRow.Location,
		LastUpdatedAt:    providerRow.LastUpdatedAt.Time,
		CreatedAt:        providerRow.CreatedAt.Time,
	}

	respondWithJSON(w, http.StatusCreated, AuthResponse{
		Token:       token,
		AccountType: string(auth.AccountTypeProvider),
		User:        profile,
	})
}

func (h *APIHandler) LoginProvider(w http.ResponseWriter, r *http.Request) {
	var req ProviderLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	req.PhNo = strings.TrimSpace(req.PhNo)
	req.Password = strings.TrimSpace(req.Password)

	if req.Password == "" || (req.Email == "" && req.PhNo == "") {
		respondWithError(w, http.StatusBadRequest, "email or ph_no, and password are required")
		return
	}

	var provider database.Provider
	var err error

	if req.Email != "" {
		p, e := h.queries.GetProviderByEmail(r.Context(), req.Email)
		if e == nil {
			provider = database.Provider{
				ID:               p.ID,
				Type:             p.Type,
				AuthorizedPerson: p.AuthorizedPerson,
				Name:             p.Name,
				PasswordHash:     p.PasswordHash,
				RegistrationNo:   p.RegistrationNo,
				GovtID:           p.GovtID,
				Cin:              p.Cin,
				Email:            p.Email,
				PhNo:             p.PhNo,
				Website:          p.Website,
				State:            p.State,
				Dist:             p.Dist,
				Location:         p.Location,
				LastUpdatedAt:    p.LastUpdatedAt,
				CreatedAt:        p.CreatedAt,
			}
		}
		err = e
	} else {
		p, e := h.queries.GetProviderByPhone(r.Context(), req.PhNo)
		if e == nil {
			provider = database.Provider{
				ID:               p.ID,
				Type:             p.Type,
				AuthorizedPerson: p.AuthorizedPerson,
				Name:             p.Name,
				PasswordHash:     p.PasswordHash,
				RegistrationNo:   p.RegistrationNo,
				GovtID:           p.GovtID,
				Cin:              p.Cin,
				Email:            p.Email,
				PhNo:             p.PhNo,
				Website:          p.Website,
				State:            p.State,
				Dist:             p.Dist,
				Location:         p.Location,
				LastUpdatedAt:    p.LastUpdatedAt,
				CreatedAt:        p.CreatedAt,
			}
		}
		err = e
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to query provider")
		return
	}
	var providerActive bool
	if err := h.pool.QueryRow(r.Context(), `SELECT is_active FROM providers WHERE id = $1`, provider.ID).Scan(&providerActive); err == nil && !providerActive {
		respondWithError(w, http.StatusForbidden, "This provider account is inactive")
		return
	}

	if err := auth.CheckPasswordHash(req.Password, provider.PasswordHash); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	providerIDStr := uuid.UUID(provider.ID.Bytes).String()
	token, err := auth.GenerateToken(
		providerIDStr,
		auth.AccountTypeProvider,
		string(provider.Type),
		provider.PhNo,
		provider.Email,
		h.cfg.JWTSecret,
		h.cfg.JWTExpiration,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate authorization token")
		return
	}

	profile := ProviderResponse{
		ID:               providerIDStr,
		Type:             string(provider.Type),
		AuthorizedPerson: pgTextToString(provider.AuthorizedPerson),
		Name:             provider.Name,
		RegistrationNo:   pgTextToString(provider.RegistrationNo),
		GovtID:           provider.GovtID,
		Cin:              pgTextToString(provider.Cin),
		Email:            provider.Email,
		PhNo:             provider.PhNo,
		Website:          pgTextToString(provider.Website),
		State:            provider.State,
		Dist:             provider.Dist,
		Location:         provider.Location,
		LastUpdatedAt:    provider.LastUpdatedAt.Time,
		CreatedAt:        provider.CreatedAt.Time,
	}

	respondWithJSON(w, http.StatusOK, AuthResponse{
		Token:       token,
		AccountType: string(auth.AccountTypeProvider),
		User:        profile,
	})
}

func (h *APIHandler) GetProviderMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims.AccountType != auth.AccountTypeProvider {
		respondWithError(w, http.StatusForbidden, "Provider claims not found in request context")
		return
	}

	parsedUUID, err := uuid.Parse(claims.AccountID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid provider ID format in token")
		return
	}

	provider, err := h.queries.GetProviderByID(r.Context(), pgtype.UUID{Bytes: parsedUUID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Provider not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve provider profile")
		return
	}

	respondWithJSON(w, http.StatusOK, ProviderResponse{
		ID:               uuid.UUID(provider.ID.Bytes).String(),
		Type:             string(provider.Type),
		AuthorizedPerson: pgTextToString(provider.AuthorizedPerson),
		Name:             provider.Name,
		RegistrationNo:   pgTextToString(provider.RegistrationNo),
		GovtID:           provider.GovtID,
		Cin:              pgTextToString(provider.Cin),
		Email:            provider.Email,
		PhNo:             provider.PhNo,
		Website:          pgTextToString(provider.Website),
		State:            provider.State,
		Dist:             provider.Dist,
		Location:         provider.Location,
		LastUpdatedAt:    provider.LastUpdatedAt.Time,
		CreatedAt:        provider.CreatedAt.Time,
	})
}

// ==================== HEALTH HANDLER ====================

func (h *APIHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "connected"
	if h.pool != nil {
		if err := h.pool.Ping(ctx); err != nil {
			dbStatus = fmt.Sprintf("unreachable: %v", err)
		}
	} else {
		dbStatus = "no pool configured"
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"database": dbStatus,
		"time":     time.Now().UTC(),
	})
}

// ==================== UTILITY HELPERS ====================

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{
		"error": message,
	})
}

func textToPgText(s string) pgtype.Text {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: trimmed, Valid: true}
}

func pgTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
