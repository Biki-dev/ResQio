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
	"go-sse-server/internal/dispatch"
	"go-sse-server/internal/handlers"
	"go-sse-server/internal/middleware"
	"go-sse-server/internal/ml"
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

	// 3. Initialize queries, ML client, dispatch coordinator, worker, and API handlers
	queries := database.New(pool)
	mlClient := ml.NewClient(os.Getenv("EMBEDDING_SERVICE_URL"))
	coordinator := dispatch.NewCoordinator(queries, pool, 3*time.Minute)
	coordinator.SetMLClient(mlClient)
	dispatchWorker := dispatch.NewWorker(coordinator, 5*time.Second)
	apiHandler := handlers.NewAPIHandler(queries, pool, cfg, coordinator, mlClient)

	// 4. Setup routes
	mux := http.NewServeMux()

	authMw := middleware.AuthMiddleware(cfg.JWTSecret)
	optAuthMw := middleware.OptionalAuthMiddleware(cfg.JWTSecret)
	userGuard := middleware.RequireAccountType(auth.AccountTypeUser)
	providerGuard := middleware.RequireAccountType(auth.AccountTypeProvider)
	victimGuard := middleware.RequireUserRole(string(database.UserRoleVICTIM))

	// User Routes
	mux.HandleFunc("POST /api/auth/users/register", apiHandler.RegisterUser)
	mux.HandleFunc("POST /api/auth/users/login", apiHandler.LoginUser)
	mux.Handle("GET /api/auth/users/me", authMw(userGuard(http.HandlerFunc(apiHandler.GetUserMe))))

	// Provider Routes
	mux.HandleFunc("POST /api/auth/providers/register", apiHandler.RegisterProvider)
	mux.HandleFunc("POST /api/auth/providers/login", apiHandler.LoginProvider)
	mux.Handle("GET /api/auth/providers/me", authMw(providerGuard(http.HandlerFunc(apiHandler.GetProviderMe))))

	// Disaster Reporting Portal (Image Embedding & Location)
	mux.Handle("POST /api/disaster-reports", optAuthMw(http.HandlerFunc(apiHandler.CreateDisasterReport)))
	mux.HandleFunc("GET /api/disaster-reports", apiHandler.ListDisasterReports)
	mux.HandleFunc("GET /api/disaster-reports/{id}", apiHandler.GetDisasterReportByID)

	// Road Hazards / Issue Submission (Wireframe: Left Form & Previous Submissions Feed)
	mux.Handle("POST /api/hazards", optAuthMw(http.HandlerFunc(apiHandler.SubmitRoadHazard)))
	mux.HandleFunc("GET /api/hazards", apiHandler.ListRoadHazards)
	mux.HandleFunc("GET /api/hazards/{id}", apiHandler.GetRoadHazardByID)

	// Assistance Requests (Wireframe: Right Form & Previous Calls Feed & Tracking)
	mux.Handle("POST /api/requests", authMw(victimGuard(http.HandlerFunc(apiHandler.SubmitAssistanceRequest))))
	mux.HandleFunc("GET /api/requests", apiHandler.ListAssistanceRequests)
	mux.HandleFunc("GET /api/requests/{id}", apiHandler.GetAssistanceRequestByID)
	mux.HandleFunc("GET /api/requests/track/{code}", apiHandler.TrackAssistanceRequest)

	// Community Mutual Aid Items
	mux.Handle("POST /api/mutual-aid", authMw(userGuard(http.HandlerFunc(apiHandler.CreateMutualAidItem))))
	mux.HandleFunc("GET /api/mutual-aid", apiHandler.ListMutualAidItems)
	mux.Handle("POST /api/mutual-aid/{id}/claim", authMw(userGuard(http.HandlerFunc(apiHandler.ClaimMutualAidItem))))

	// Provider Resource Capacities & Inventory
	mux.HandleFunc("GET /api/distribution-camps", apiHandler.ListDistributionCamps)
	mux.Handle("POST /api/provider/distribution-camps", authMw(providerGuard(http.HandlerFunc(apiHandler.CreateDistributionCamp))))
	mux.Handle("PUT /api/provider/distribution-camps/{id}", authMw(providerGuard(http.HandlerFunc(apiHandler.UpdateDistributionCamp))))
	mux.Handle("DELETE /api/provider/distribution-camps/{id}", authMw(providerGuard(http.HandlerFunc(apiHandler.DeleteDistributionCamp))))
	mux.Handle("POST /api/resources", authMw(providerGuard(http.HandlerFunc(apiHandler.CreateResource))))
	mux.HandleFunc("GET /api/resources", apiHandler.ListResources)
	mux.HandleFunc("GET /api/resources/{id}", apiHandler.GetResourceByID)
	mux.Handle("PUT /api/resources/{id}", authMw(providerGuard(http.HandlerFunc(apiHandler.UpdateResource))))
	mux.Handle("DELETE /api/resources/{id}", authMw(providerGuard(http.HandlerFunc(apiHandler.DeleteResource))))

	// Provider Dispatch Pings (Real-Time Emergency Matching & Cascades)
	mux.Handle("GET /api/provider/requests", authMw(providerGuard(http.HandlerFunc(apiHandler.ListProviderAssistanceRequests))))
	mux.Handle("GET /api/provider/pings/active", authMw(providerGuard(http.HandlerFunc(apiHandler.GetActiveProviderPing))))
	mux.Handle("POST /api/provider/pings/{id}/accept", authMw(providerGuard(http.HandlerFunc(apiHandler.AcceptPing))))
	mux.Handle("POST /api/provider/pings/{id}/reject", authMw(providerGuard(http.HandlerFunc(apiHandler.RejectPing))))

	// Admin / System Escalation Alerts
	mux.HandleFunc("GET /api/admin/alerts", apiHandler.GetExhaustedAlerts)

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

	// Start background timeout worker for dispatch cascade
	dispatchWorker.Start(shutdownCtx)

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

	dispatchWorker.Stop()

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStop()

	if err := server.Shutdown(stopCtx); err != nil {
		log.Printf("[Server] Forced server shutdown error: %v\n", err)
	} else {
		log.Println("[Server] HTTP server stopped cleanly")
	}

	log.Println("[Server] Server stopped")
}
