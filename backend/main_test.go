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
	optAuthMw := middleware.OptionalAuthMiddleware(cfg.JWTSecret)
	userGuard := middleware.RequireAccountType(auth.AccountTypeUser)
	providerGuard := middleware.RequireAccountType(auth.AccountTypeProvider)

	// User Routes
	mux.HandleFunc("POST /api/auth/users/register", apiHandler.RegisterUser)
	mux.HandleFunc("POST /api/auth/users/login", apiHandler.LoginUser)
	mux.Handle("GET /api/auth/users/me", authMw(userGuard(http.HandlerFunc(apiHandler.GetUserMe))))

	// Provider Routes
	mux.HandleFunc("POST /api/auth/providers/register", apiHandler.RegisterProvider)
	mux.HandleFunc("POST /api/auth/providers/login", apiHandler.LoginProvider)
	mux.Handle("GET /api/auth/providers/me", authMw(providerGuard(http.HandlerFunc(apiHandler.GetProviderMe))))

	// Disaster Reporting Portal
	mux.Handle("POST /api/disaster-reports", optAuthMw(http.HandlerFunc(apiHandler.CreateDisasterReport)))
	mux.HandleFunc("GET /api/disaster-reports", apiHandler.ListDisasterReports)
	mux.HandleFunc("GET /api/disaster-reports/{id}", apiHandler.GetDisasterReportByID)

	// Road Hazards / Issue Submission
	mux.Handle("POST /api/hazards", optAuthMw(http.HandlerFunc(apiHandler.SubmitRoadHazard)))
	mux.HandleFunc("GET /api/hazards", apiHandler.ListRoadHazards)
	mux.HandleFunc("GET /api/hazards/{id}", apiHandler.GetRoadHazardByID)

	// Assistance Requests
	victimGuard := middleware.RequireUserRole(string(database.UserRoleVICTIM))
	mux.Handle("POST /api/requests", authMw(victimGuard(http.HandlerFunc(apiHandler.SubmitAssistanceRequest))))
	mux.HandleFunc("GET /api/requests", apiHandler.ListAssistanceRequests)
	mux.HandleFunc("GET /api/requests/{id}", apiHandler.GetAssistanceRequestByID)
	mux.HandleFunc("GET /api/requests/track/{code}", apiHandler.TrackAssistanceRequest)

	// Community Mutual Aid Items
	mux.Handle("POST /api/mutual-aid", authMw(userGuard(http.HandlerFunc(apiHandler.CreateMutualAidItem))))
	mux.HandleFunc("GET /api/mutual-aid", apiHandler.ListMutualAidItems)
	mux.Handle("POST /api/mutual-aid/{id}/claim", authMw(userGuard(http.HandlerFunc(apiHandler.ClaimMutualAidItem))))

	// Provider Resources
	mux.Handle("POST /api/resources", authMw(providerGuard(http.HandlerFunc(apiHandler.CreateResource))))
	mux.HandleFunc("GET /api/resources", apiHandler.ListResources)
	mux.HandleFunc("GET /api/resources/{id}", apiHandler.GetResourceByID)

	// System & Health
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

func TestDisasterReportEndpoints(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	// 1. Submit Disaster Report with 1536d vector
	lat := 19.0760
	lng := 72.8777
	dummyEmbedding := make([]float32, 1536)
	for i := range dummyEmbedding {
		dummyEmbedding[i] = 0.05
	}

	payload := map[string]interface{}{
		"embedding": dummyEmbedding,
		"latitude":  lat,
		"longitude": lng,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/disaster-reports", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on disaster report submission, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var createdReport handlers.DisasterReportResponse
	if err := json.NewDecoder(rec.Body).Decode(&createdReport); err != nil {
		t.Fatalf("failed to decode created report: %v", err)
	}

	if createdReport.ID == "" {
		t.Fatal("expected non-empty ID for disaster report")
	}

	// 2. Fetch Disaster Reports List
	listReq := httptest.NewRequest("GET", "/api/disaster-reports?limit=10", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on listing disaster reports, got: %d", listRec.Code)
	}

	var reports []handlers.DisasterReportResponse
	if err := json.NewDecoder(listRec.Body).Decode(&reports); err != nil {
		t.Fatalf("failed to decode disaster reports list: %v", err)
	}
	if len(reports) == 0 {
		t.Fatal("expected at least 1 disaster report in list")
	}

	// 3. Get Disaster Report by ID
	getReq := httptest.NewRequest("GET", "/api/disaster-reports/"+createdReport.ID, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get disaster report by ID, got: %d", getRec.Code)
	}
}

func TestRoadHazardFormEndpoints(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	lat := 28.6139
	lng := 77.2090
	payload := handlers.SubmitRoadHazardRequest{
		Name:        "Rohan Gupta",
		PhoneNumber: "+919811223344",
		HazardType:  "FLOODED_ROAD",
		Severity:    "HIGH",
		Description: "Water logging up to 3 feet near Main Market underpass",
		Latitude:    &lat,
		Longitude:   &lng,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/hazards", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on hazard submission, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var hazardResp handlers.RoadHazardResponse
	if err := json.NewDecoder(rec.Body).Decode(&hazardResp); err != nil {
		t.Fatalf("failed to decode road hazard response: %v", err)
	}

	if hazardResp.Name != "Rohan Gupta" {
		t.Errorf("expected name Rohan Gupta, got: %s", hazardResp.Name)
	}
	if hazardResp.HazardType != "FLOODED_ROAD" {
		t.Errorf("expected hazard type FLOODED_ROAD, got: %s", hazardResp.HazardType)
	}

	// Fetch Feed: "Previous issue submittion"
	feedReq := httptest.NewRequest("GET", "/api/hazards?limit=5", nil)
	feedRec := httptest.NewRecorder()
	handler.ServeHTTP(feedRec, feedReq)

	if feedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on previous issue submissions feed, got: %d", feedRec.Code)
	}

	// Fetch Detail View
	viewReq := httptest.NewRequest("GET", "/api/hazards/"+hazardResp.ID, nil)
	viewRec := httptest.NewRecorder()
	handler.ServeHTTP(viewRec, viewReq)

	if viewRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get hazard view, got: %d", viewRec.Code)
	}
}

func TestAssistanceRequestFormAndTracking(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	lat := 12.9716
	lng := 77.5946
	payload := handlers.SubmitAssistanceRequest{
		Name:         "Priya Nair",
		Identity:     "GOVT-ID-778899",
		PhoneNumber:  "+919876112233",
		ThingsNeeded: "Water & Dry Rations",
		Category:     "WATER",
		Amount:       50,
		Description:  "Bottled drinking water needed for 50 people at community hall",
		Latitude:     &lat,
		Longitude:    &lng,
		AddressText:  "Community Hall, Sector 4",
		Priority:     "CRITICAL",
	}

	body, _ := json.Marshal(payload)
	// 1. Unauthenticated request should fail with 401 Unauthorized
	reqUnauth := httptest.NewRequest("POST", "/api/requests", bytes.NewReader(body))
	reqUnauth.Header.Set("Content-Type", "application/json")
	recUnauth := httptest.NewRecorder()
	handler.ServeHTTP(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated assistance request, got: %d", recUnauth.Code)
	}

	// 2. Non-victim user (e.g., PUBLIC role) should fail with 403 Forbidden
	publicPhone := fmt.Sprintf("+91%010d", time.Now().UnixNano()%10000000000)
	publicUserPayload := map[string]string{
		"phone":     publicPhone,
		"password":  "PublicUserPass123!",
		"full_name": "Public Citizen",
		"role":      "PUBLIC",
	}
	publicBody, _ := json.Marshal(publicUserPayload)
	regReq := httptest.NewRequest("POST", "/api/auth/users/register", bytes.NewReader(publicBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	handler.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("failed to register public user: %d (%s)", regRec.Code, regRec.Body.String())
	}
	var publicAuthResp handlers.AuthResponse
	_ = json.NewDecoder(regRec.Body).Decode(&publicAuthResp)

	reqForbidden := httptest.NewRequest("POST", "/api/requests", bytes.NewReader(body))
	reqForbidden.Header.Set("Content-Type", "application/json")
	reqForbidden.Header.Set("Authorization", "Bearer "+publicAuthResp.Token)
	recForbidden := httptest.NewRecorder()
	handler.ServeHTTP(recForbidden, reqForbidden)
	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-victim user, got: %d", recForbidden.Code)
	}

	// 3. User with VICTIM role should succeed with 201 Created
	victimPhone := fmt.Sprintf("+91%010d", (time.Now().UnixNano()+1)%10000000000)
	victimUserPayload := map[string]string{
		"phone":     victimPhone,
		"password":  "VictimUserPass123!",
		"full_name": "Priya Nair",
		"role":      "VICTIM",
	}
	victimBody, _ := json.Marshal(victimUserPayload)
	vRegReq := httptest.NewRequest("POST", "/api/auth/users/register", bytes.NewReader(victimBody))
	vRegReq.Header.Set("Content-Type", "application/json")
	vRegRec := httptest.NewRecorder()
	handler.ServeHTTP(vRegRec, vRegReq)
	if vRegRec.Code != http.StatusCreated {
		t.Fatalf("failed to register victim user: %d (%s)", vRegRec.Code, vRegRec.Body.String())
	}
	var victimAuthResp handlers.AuthResponse
	_ = json.NewDecoder(vRegRec.Body).Decode(&victimAuthResp)

	req := httptest.NewRequest("POST", "/api/requests", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+victimAuthResp.Token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on assistance request submission, got: %d (%s)", rec.Code, rec.Body.String())
	}

	var reqResp handlers.AssistanceRequestResponse
	if err := json.NewDecoder(rec.Body).Decode(&reqResp); err != nil {
		t.Fatalf("failed to decode assistance request response: %v", err)
	}

	if reqResp.TrackingCode == "" {
		t.Fatal("expected non-empty tracking code")
	}
	if reqResp.Category != "WATER" {
		t.Errorf("expected category WATER, got: %s", reqResp.Category)
	}
	if reqResp.QuantityNeeded != 50 {
		t.Errorf("expected quantity 50, got: %d", reqResp.QuantityNeeded)
	}

	// Fetch Feed: "Previous calls"
	feedReq := httptest.NewRequest("GET", "/api/requests?limit=5", nil)
	feedRec := httptest.NewRecorder()
	handler.ServeHTTP(feedRec, feedReq)

	if feedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on previous calls feed, got: %d", feedRec.Code)
	}

	// Fetch View by ID
	viewReq := httptest.NewRequest("GET", "/api/requests/"+reqResp.ID, nil)
	viewRec := httptest.NewRecorder()
	handler.ServeHTTP(viewRec, viewReq)

	if viewRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on request view by ID, got: %d", viewRec.Code)
	}

	// Track by tracking code
	trackReq := httptest.NewRequest("GET", "/api/requests/track/"+reqResp.TrackingCode, nil)
	trackRec := httptest.NewRecorder()
	handler.ServeHTTP(trackRec, trackReq)

	if trackRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on tracking request, got: %d (%s)", trackRec.Code, trackRec.Body.String())
	}
}

func TestMutualAidAndResources(t *testing.T) {
	handler, pool := setupTestServer(t)
	if pool != nil {
		defer pool.Close()
	}

	// 1. Register User to get token
	phone := fmt.Sprintf("+91%010d", time.Now().UnixNano()%10000000000)
	userRegPayload := map[string]string{
		"phone":     phone,
		"password":  "SecureUser123!",
		"full_name": "Mutual Aid Volunteer",
	}
	body, _ := json.Marshal(userRegPayload)
	regReq := httptest.NewRequest("POST", "/api/auth/users/register", bytes.NewReader(body))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	handler.ServeHTTP(regRec, regReq)

	var authResp handlers.AuthResponse
	_ = json.NewDecoder(regRec.Body).Decode(&authResp)
	userToken := authResp.Token

	// 2. Create Mutual Aid Item (Protected)
	aidPayload := handlers.CreateMutualAidRequest{
		ItemName:    "First Aid Kit",
		Quantity:    5,
		Description: "Unopened antiseptic bandages and basic medicines",
		Location:    "POINT(72.8777 19.0760)",
	}
	aidBody, _ := json.Marshal(aidPayload)
	aidReq := httptest.NewRequest("POST", "/api/mutual-aid", bytes.NewReader(aidBody))
	aidReq.Header.Set("Content-Type", "application/json")
	aidReq.Header.Set("Authorization", "Bearer "+userToken)
	aidRec := httptest.NewRecorder()
	handler.ServeHTTP(aidRec, aidReq)

	if aidRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on mutual aid creation, got: %d (%s)", aidRec.Code, aidRec.Body.String())
	}

	var aidResp handlers.MutualAidResponse
	_ = json.NewDecoder(aidRec.Body).Decode(&aidResp)

	// List Available Mutual Aid Items
	listAidReq := httptest.NewRequest("GET", "/api/mutual-aid?available=true", nil)
	listAidRec := httptest.NewRecorder()
	handler.ServeHTTP(listAidRec, listAidReq)

	if listAidRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list mutual aid items, got: %d", listAidRec.Code)
	}

	// 3. Register Provider to create resource
	email := fmt.Sprintf("ngo_%d@relief.org", time.Now().UnixNano()%1000000)
	provPhone := fmt.Sprintf("+91%010d", (time.Now().UnixNano()+1)%10000000000)
	provRegPayload := map[string]string{
		"type":              "ORGANISATION",
		"name":              "City Relief Foundation",
		"password":          "ProviderPass123!",
		"govt_id":           "GOVT-7788",
		"email":             email,
		"ph_no":             provPhone,
		"state":             "Delhi",
		"dist":              "New Delhi",
		"location":          "POINT(77.2090 28.6139)",
	}
	provBody, _ := json.Marshal(provRegPayload)
	provRegReq := httptest.NewRequest("POST", "/api/auth/providers/register", bytes.NewReader(provBody))
	provRegReq.Header.Set("Content-Type", "application/json")
	provRegRec := httptest.NewRecorder()
	handler.ServeHTTP(provRegRec, provRegReq)

	var provAuthResp handlers.AuthResponse
	_ = json.NewDecoder(provRegRec.Body).Decode(&provAuthResp)
	provToken := provAuthResp.Token

	// 4. Create Provider Resource
	resPayload := handlers.CreateResourceRequest{
		Title:           "Emergency Water Supply Tanker",
		Description:     "10,000 Liters of potable drinking water",
		Category:        "WATER",
		TotalCapacity:   10000,
		CurrentCapacity: 10000,
		Unit:            "Liters",
		Location:        "POINT(77.2090 28.6139)",
		ContactPhone:    provPhone,
	}
	resBody, _ := json.Marshal(resPayload)
	resReq := httptest.NewRequest("POST", "/api/resources", bytes.NewReader(resBody))
	resReq.Header.Set("Content-Type", "application/json")
	resReq.Header.Set("Authorization", "Bearer "+provToken)
	resRec := httptest.NewRecorder()
	handler.ServeHTTP(resRec, resReq)

	if resRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created on resource creation, got: %d (%s)", resRec.Code, resRec.Body.String())
	}

	var resCreated handlers.ResourceResponse
	_ = json.NewDecoder(resRec.Body).Decode(&resCreated)

	// 5. List Resources
	listResReq := httptest.NewRequest("GET", "/api/resources?category=WATER", nil)
	listResRec := httptest.NewRecorder()
	handler.ServeHTTP(listResRec, listResReq)

	if listResRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list resources, got: %d", listResRec.Code)
	}

	// 6. View Resource by ID
	viewResReq := httptest.NewRequest("GET", "/api/resources/"+resCreated.ID, nil)
	viewResRec := httptest.NewRecorder()
	handler.ServeHTTP(viewResRec, viewResReq)

	if viewResRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get resource by ID, got: %d", viewResRec.Code)
	}
}


