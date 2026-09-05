package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-sse-server/internal/auth"
)

func TestRequireUserRole(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	victimGuard := RequireUserRole("VICTIM")
	handler := victimGuard(dummyHandler)

	tests := []struct {
		name           string
		claims         *auth.AuthClaims
		setClaims      bool
		expectedStatus int
	}{
		{
			name:           "No claims in context",
			setClaims:      false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Provider account type with VICTIM role (rejected for account type)",
			claims: &auth.AuthClaims{
				AccountID:   "provider-uuid",
				AccountType: auth.AccountTypeProvider,
				Role:        "VICTIM",
			},
			setClaims:      true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "User account type with non-victim role",
			claims: &auth.AuthClaims{
				AccountID:   "user-uuid",
				AccountType: auth.AccountTypeUser,
				Role:        "PUBLIC",
			},
			setClaims:      true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "User account type with VICTIM role",
			claims: &auth.AuthClaims{
				AccountID:   "user-uuid",
				AccountType: auth.AccountTypeUser,
				Role:        "VICTIM",
			},
			setClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name: "User account type with lowercase victim role",
			claims: &auth.AuthClaims{
				AccountID:   "user-uuid",
				AccountType: auth.AccountTypeUser,
				Role:        "victim",
			},
			setClaims:      true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if tc.setClaims {
				ctx := context.WithValue(req.Context(), ClaimsContextKey, tc.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	roleGuard := RequireRole("ADMIN", "COORDINATOR")
	handler := roleGuard(dummyHandler)

	tests := []struct {
		name           string
		claims         *auth.AuthClaims
		setClaims      bool
		expectedStatus int
	}{
		{
			name:           "No claims",
			setClaims:      false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Unauthorized role",
			claims: &auth.AuthClaims{
				AccountID:   "user-uuid",
				AccountType: auth.AccountTypeUser,
				Role:        "VICTIM",
			},
			setClaims:      true,
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "Authorized role ADMIN",
			claims: &auth.AuthClaims{
				AccountID:   "user-uuid",
				AccountType: auth.AccountTypeUser,
				Role:        "ADMIN",
			},
			setClaims:      true,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Authorized role COORDINATOR",
			claims: &auth.AuthClaims{
				AccountID:   "provider-uuid",
				AccountType: auth.AccountTypeProvider,
				Role:        "COORDINATOR",
			},
			setClaims:      true,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.setClaims {
				ctx := context.WithValue(req.Context(), ClaimsContextKey, tc.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAuthMiddlewareIntegration(t *testing.T) {
	secret := "test-secret-key-12345"
	authMw := AuthMiddleware(secret)
	victimGuard := RequireUserRole("VICTIM")

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	securedEndpoint := authMw(victimGuard(dummyHandler))

	// Generate valid victim token
	victimToken, err := auth.GenerateToken(
		"user-1",
		auth.AccountTypeUser,
		"VICTIM",
		"+919876543210",
		"",
		secret,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to generate victim token: %v", err)
	}

	// Generate non-victim token
	publicToken, err := auth.GenerateToken(
		"user-2",
		auth.AccountTypeUser,
		"PUBLIC",
		"+919876543211",
		"",
		secret,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("failed to generate public token: %v", err)
	}

	// 1. Missing Authorization header
	req1 := httptest.NewRequest(http.MethodPost, "/requests", nil)
	rec1 := httptest.NewRecorder()
	securedEndpoint.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing auth header, got: %d", rec1.Code)
	}

	// 2. Token with non-victim role
	req2 := httptest.NewRequest(http.MethodPost, "/requests", nil)
	req2.Header.Set("Authorization", "Bearer "+publicToken)
	rec2 := httptest.NewRecorder()
	securedEndpoint.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-victim role, got: %d", rec2.Code)
	}

	// 3. Valid token with VICTIM role
	req3 := httptest.NewRequest(http.MethodPost, "/requests", nil)
	req3.Header.Set("Authorization", "Bearer "+victimToken)
	rec3 := httptest.NewRecorder()
	securedEndpoint.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("expected 200 OK for victim role, got: %d (%s)", rec3.Code, rec3.Body.String())
	}
}
