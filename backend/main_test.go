package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go-sse-server/internal/auth"
	"go-sse-server/internal/config"
	"go-sse-server/internal/database"
	"go-sse-server/internal/handlers"
	"go-sse-server/internal/middleware"
)

func setupTestServer(t *testing.T) (http.Handler, *pgxpool.Pool) {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("skipping test: database connection failed: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping test: postgres not reachable: %v", err)
	}

	queries := database.New(pool)
	apiHandler := handlers.NewAPIHandler(queries, pool, cfg)

	mux := http.NewServeMux()
	authMw := middleware.AuthMiddleware(cfg.JWTSecret)
	userGuard := middleware.RequireAccountType(auth.AccountTypeUser)
	providerGuard := middleware.RequireAccountType(auth.AccountTypeProvider)

	mux.HandleFunc("POST /api/auth/users/register", apiHandler.RegisterUser)
	mux.HandleFunc("POST /api/auth/users/login", apiHandler.LoginUser)
	mux.Handle("GET /api/auth/users/me", authMw(userGuard(http.HandlerFunc(apiHandler.GetUserMe))))

	mux.HandleFunc("POST /api/auth/providers/register", apiHandler.RegisterProvider)
	mux.HandleFunc("POST /api/auth/providers/login", apiHandler.LoginProvider)
	mux.Handle("GET /api/auth/providers/me", authMw(providerGuard(http.HandlerFunc(apiHandler.GetProviderMe))))

	mux.HandleFunc("GET /healthz", apiHandler.Healthz)

	return middleware.CORSMiddleware(mux), pool
}

func TestHealthzEndpoint(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got: %d", rec.Code)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode healthz response: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("expected status 'ok', got: %v", res["status"])
	}
}

func TestUserAuthFlow(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	uniquePhone := fmt.Sprintf("+91%d", time.Now().UnixNano()%10000000000)
	password := "SecretPass123!"

	// 1. Register User
	registerPayload := map[string]string{
		"phone":     uniquePhone,
		"password":  password,
		"full_name": "Test User",
		"role":      "VICTIM",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/auth/users/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on user register, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var authResp handlers.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to decode register response: %v", err)
	}

	if authResp.Token == "" {
		t.Fatal("expected JWT token in registration response, got empty")
	}
	if authResp.AccountType != "user" {
		t.Errorf("expected account_type 'user', got %s", authResp.AccountType)
	}

	userToken := authResp.Token

	// 2. Duplicate Registration Rejection
	recDup := httptest.NewRecorder()
	reqDup := httptest.NewRequest("POST", "/api/auth/users/register", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recDup, reqDup)
	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict on duplicate phone register, got: %d", recDup.Code)
	}

	// 3. Login User
	loginPayload := map[string]string{
		"phone":    uniquePhone,
		"password": password,
	}
	loginBody, _ := json.Marshal(loginPayload)
	loginReq := httptest.NewRequest("POST", "/api/auth/users/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()

	handler.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on user login, got: %d (%s)", loginRec.Code, loginRec.Body.String())
	}

	// 4. Access Protected User Me Endpoint
	meReq := httptest.NewRequest("GET", "/api/auth/users/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+userToken)
	meRec := httptest.NewRecorder()

	handler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on /api/auth/users/me, got: %d (%s)", meRec.Code, meRec.Body.String())
	}

	var userProfile handlers.UserResponse
	if err := json.NewDecoder(meRec.Body).Decode(&userProfile); err != nil {
		t.Fatalf("failed to decode user me response: %v", err)
	}

	if userProfile.Phone != uniquePhone {
		t.Errorf("expected user phone %s, got %s", uniquePhone, userProfile.Phone)
	}
	if userProfile.Role != "VICTIM" {
		t.Errorf("expected user role 'VICTIM', got %s", userProfile.Role)
	}

	// 5. Protected Endpoint Missing Token Rejection
	unauthReq := httptest.NewRequest("GET", "/api/auth/users/me", nil)
	unauthRec := httptest.NewRecorder()
	handler.ServeHTTP(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized for missing token, got: %d", unauthRec.Code)
	}
}

func TestProviderAuthFlow(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano()%10000000000)
	email := fmt.Sprintf("provider%s@test.com", uniqueSuffix)
	phNo := fmt.Sprintf("+91%s", uniqueSuffix)
	password := "ProviderSecret123!"

	// 1. Register Provider
	registerPayload := map[string]string{
		"type":              "ORGANISATION",
		"name":              "Relief NGO",
		"authorized_person": "Jane Doe",
		"password":          password,
		"govt_id":           "GOVT-12345",
		"email":             email,
		"ph_no":             phNo,
		"state":             "Maharashtra",
		"dist":              "Mumbai",
		"location":          "POINT(72.8777 19.0760)",
	}
	body, _ := json.Marshal(registerPayload)
	req := httptest.NewRequest("POST", "/api/auth/providers/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created on provider register, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var authResp handlers.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to decode provider register response: %v", err)
	}

	if authResp.Token == "" {
		t.Fatal("expected token in provider register response, got empty")
	}
	if authResp.AccountType != "provider" {
		t.Errorf("expected account_type 'provider', got %s", authResp.AccountType)
	}

	providerToken := authResp.Token

	// 2. Login Provider with Email
	loginPayloadEmail := map[string]string{
		"email":    email,
		"password": password,
	}
	loginBodyEmail, _ := json.Marshal(loginPayloadEmail)
	loginReqEmail := httptest.NewRequest("POST", "/api/auth/providers/login", bytes.NewReader(loginBodyEmail))
	loginReqEmail.Header.Set("Content-Type", "application/json")
	loginRecEmail := httptest.NewRecorder()

	handler.ServeHTTP(loginRecEmail, loginReqEmail)

	if loginRecEmail.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on provider login with email, got: %d (%s)", loginRecEmail.Code, loginRecEmail.Body.String())
	}

	// 3. Login Provider with Phone Number
	loginPayloadPhone := map[string]string{
		"ph_no":    phNo,
		"password": password,
	}
	loginBodyPhone, _ := json.Marshal(loginPayloadPhone)
	loginReqPhone := httptest.NewRequest("POST", "/api/auth/providers/login", bytes.NewReader(loginBodyPhone))
	loginReqPhone.Header.Set("Content-Type", "application/json")
	loginRecPhone := httptest.NewRecorder()

	handler.ServeHTTP(loginRecPhone, loginReqPhone)

	if loginRecPhone.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on provider login with phone, got: %d (%s)", loginRecPhone.Code, loginRecPhone.Body.String())
	}

	// 4. Access Protected Provider Me Endpoint
	meReq := httptest.NewRequest("GET", "/api/auth/providers/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+providerToken)
	meRec := httptest.NewRecorder()

	handler.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on /api/auth/providers/me, got: %d (%s)", meRec.Code, meRec.Body.String())
	}

	var providerProfile handlers.ProviderResponse
	if err := json.NewDecoder(meRec.Body).Decode(&providerProfile); err != nil {
		t.Fatalf("failed to decode provider profile: %v", err)
	}

	if providerProfile.Email != email {
		t.Errorf("expected provider email %s, got %s", email, providerProfile.Email)
	}
	if providerProfile.PhNo != phNo {
		t.Errorf("expected provider ph_no %s, got %s", phNo, providerProfile.PhNo)
	}

	// 5. Account Type Isolation Guard: Provider Token cannot access User /me endpoint
	crossReq := httptest.NewRequest("GET", "/api/auth/users/me", nil)
	crossReq.Header.Set("Authorization", "Bearer "+providerToken)
	crossRec := httptest.NewRecorder()

	handler.ServeHTTP(crossRec, crossReq)

	if crossRec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden when provider accesses user /me, got: %d", crossRec.Code)
	}
}
