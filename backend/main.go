package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go-sse-server/internal/auth"
	"go-sse-server/internal/config"
	"go-sse-server/internal/database"
	"go-sse-server/internal/handlers"
	"go-sse-server/internal/middleware"
)

func main() {
	// 1. Load configuration
	cfg := config.Load()
	log.Printf("[Config] Port: %s | Database configured\n", cfg.Port)

	// 2. Setup PostgreSQL connection pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[DB] Failed to parse database configuration: %v\n", err)
	}
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("[DB] Unable to create connection pool: %v\n", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Printf("[DB] Warning: initial DB ping failed (will retry on requests): %v\n", err)
	} else {
		log.Println("[DB] Connected to PostgreSQL successfully")
	}

	// 3. Initialize queries and API handlers
	queries := database.New(pool)
	apiHandler := handlers.NewAPIHandler(queries, pool, cfg)

	// 4. Setup routes
	mux := http.NewServeMux()

	authMw := middleware.AuthMiddleware(cfg.JWTSecret)
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

	// System & Health
	mux.HandleFunc("GET /healthz", apiHandler.Healthz)

	// 5. Wrap with Global Middleware (Logging -> CORS -> Mux)
	handler := middleware.LoggingMiddleware(middleware.CORSMiddleware(mux))

	// 6. Configure HTTP server
	addr := fmt.Sprintf(":%s", cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 7. Setup graceful shutdown handler
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 8. Start HTTP Server
	go func() {
		log.Printf("[Server] Starting Auth HTTP server on http://localhost%s\n", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[Server] HTTP server error: %v\n", err)
		}
	}()

	// 9. Block until shutdown signal
	<-shutdownCtx.Done()
	log.Println("[Server] Shutdown signal received, gracefully terminating...")

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStop()

	if err := server.Shutdown(stopCtx); err != nil {
		log.Printf("[Server] Forced server shutdown error: %v\n", err)
	} else {
		log.Println("[Server] HTTP server stopped cleanly")
	}

	log.Println("[Server] Server stopped")
}
