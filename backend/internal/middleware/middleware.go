package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"go-sse-server/internal/auth"
)

type contextKey string

const ClaimsContextKey contextKey = "authClaims"

// statusRecorder wraps http.ResponseWriter to record the HTTP status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs request method, path, response status, and duration.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := newStatusRecorder(w)

		next.ServeHTTP(recorder, r)

		log.Printf("[HTTP] %s %s | Status: %d | Duration: %v\n",
			r.Method, r.URL.Path, recorder.statusCode, time.Since(start))
	})
}

// CORSMiddleware sets CORS headers and handles preflight OPTIONS requests.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates JWT Bearer token and attaches claims to the request context.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondJSONError(w, http.StatusUnauthorized, "Authorization header required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				respondJSONError(w, http.StatusUnauthorized, "Invalid authorization header format. Expected 'Bearer <token>'")
				return
			}

			tokenString := parts[1]
			claims, err := auth.ValidateToken(tokenString, secret)
			if err != nil {
				respondJSONError(w, http.StatusUnauthorized, "Invalid or expired authorization token")
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuthMiddleware validates JWT Bearer token if present and attaches claims to the context.
// If the Authorization header is absent, the request proceeds unauthenticated.
func OptionalAuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				if claims, err := auth.ValidateToken(parts[1], secret); err == nil {
					ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAccountType(expectedType auth.AccountType) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok || claims.AccountType != expectedType {
				respondJSONError(w, http.StatusForbidden, "Access forbidden for this account type")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireUserRole verifies that the request comes from an authenticated user account
// and that their role matches one of the allowed roles.
func RequireUserRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok {
				respondJSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}
			if claims.AccountType != auth.AccountTypeUser {
				respondJSONError(w, http.StatusForbidden, "Access forbidden for this account type")
				return
			}

			for _, role := range allowedRoles {
				if strings.EqualFold(claims.Role, role) {
					next.ServeHTTP(w, r)
					return
				}
			}

			respondJSONError(w, http.StatusForbidden, "Access forbidden: insufficient role permissions")
		})
	}
}

// RequireRole verifies that the authenticated entity has one of the allowed roles.
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok {
				respondJSONError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			for _, role := range allowedRoles {
				if strings.EqualFold(claims.Role, role) {
					next.ServeHTTP(w, r)
					return
				}
			}

			respondJSONError(w, http.StatusForbidden, "Access forbidden: insufficient role permissions")
		})
	}
}

// GetClaims retrieves auth claims from the context if present.
func GetClaims(ctx context.Context) (*auth.AuthClaims, bool) {
	claims, ok := ctx.Value(ClaimsContextKey).(*auth.AuthClaims)
	return claims, ok
}

func respondJSONError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
