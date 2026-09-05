package dispatch_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"

	"go-sse-server/internal/config"
	"go-sse-server/internal/database"
	"go-sse-server/internal/dispatch"
)

func setupTestDB(t *testing.T) (*database.Queries, *pgxpool.Pool) {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("skipping test: failed to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("skipping test: database unreachable: %v", err)
	}

	queries := database.New(pool)
	t.Cleanup(func() {
		pool.Close()
	})
	return queries, pool
}

func createTestProviderWithResource(t *testing.T, ctx context.Context, queries *database.Queries, pool *pgxpool.Pool, name string, phone string, email string, lon, lat float64, category database.ResourceCategory, capacity int32) (uuid.UUID, uuid.UUID) {
	locationStr := fmt.Sprintf("POINT(%f %f)", lon, lat)
	prov, err := queries.CreateProvider(ctx, database.CreateProviderParams{
		Type:             database.ProviderTypeORGANISATION,
		AuthorizedPerson: textToPgText("Manager"),
		Name:             name,
		PasswordHash:     "hashed_test_password",
		RegistrationNo:   textToPgText("REG-TEST"),
		GovtID:           "GOV-" + uuid.New().String()[:8],
		Cin:              textToPgText("CIN-TEST"),
		Email:            email,
		PhNo:             phone,
		Website:          textToPgText("https://test.org"),
		State:            "Bagmati",
		Dist:             "Kathmandu",
		Location:         locationStr,
	})
	if err != nil {
		t.Fatalf("failed to create provider %s: %v", name, err)
	}
	provID := uuid.UUID(prov.ID.Bytes)

	res, err := queries.CreateResource(ctx, database.CreateResourceParams{
		ProviderID:      prov.ID,
		Title:           name + " Inventory",
		Description:     textToPgText("Test resource"),
		Category:        category,
		TotalCapacity:   capacity,
		CurrentCapacity: capacity,
		Unit:            textToPgText("units"),
		Status:          database.VerificationStatusVERIFIED,
		Location:        locationStr,
		ContactPhone:    textToPgText(phone),
		ImageUrl:        textToPgText(""),
		Embedding:       pgvector.Vector{},
	})
	if err != nil {
		t.Fatalf("failed to create resource for %s: %v", name, err)
	}
	resID := uuid.UUID(res.ID.Bytes)

	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM resources WHERE id = $1", res.ID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM providers WHERE id = $1", prov.ID)
	})

	return provID, resID
}

func textToPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func TestAutomatedCascadeDispatchEngine(t *testing.T) {
	queries, pool := setupTestDB(t)

	ctx := context.Background()
	coordinator := dispatch.NewCoordinator(queries, pool, 2*time.Second) // 2s timeout for test speed

	ts := time.Now().UnixNano()
	// Isolated test coordinate space: (10.0000, 10.0000)
	reqOrigin := "POINT(10.0000 10.0000)"

	// 1. Create Provider A: closest (~500m) BUT 0 water
	_, _ = createTestProviderWithResource(t, ctx, queries, pool,
		fmt.Sprintf("ProvA_%d", ts),
		fmt.Sprintf("+97711%d", ts%1000000),
		fmt.Sprintf("prova_%d@test.com", ts),
		10.0050, 10.0050,
		database.ResourceCategoryWATER,
		0, // Zero capacity!
	)

	// 2. Create Provider B: middle (~2km) with 50 water
	provBID, _ := createTestProviderWithResource(t, ctx, queries, pool,
		fmt.Sprintf("ProvB_%d", ts),
		fmt.Sprintf("+97712%d", ts%1000000),
		fmt.Sprintf("provb_%d@test.com", ts),
		10.0200, 10.0200,
		database.ResourceCategoryWATER,
		50, // Has 50 units
	)

	// 3. Create Provider C: furthest (~5km) with 100 water
	provCID, resCID := createTestProviderWithResource(t, ctx, queries, pool,
		fmt.Sprintf("ProvC_%d", ts),
		fmt.Sprintf("+97713%d", ts%1000000),
		fmt.Sprintf("provc_%d@test.com", ts),
		10.0500, 10.0500,
		database.ResourceCategoryWATER,
		100, // Has 100 units
	)

	// 4. Create Assistance Request needing 15 WATER at origin
	trackingCode := fmt.Sprintf("REQ-%d", ts%1000000)
	req, err := queries.CreateAssistanceRequest(ctx, database.CreateAssistanceRequestParams{
		RequesterID:            pgtype.UUID{Valid: false},
		TrackingCode:           trackingCode,
		Category:               database.ResourceCategoryWATER,
		QuantityNeeded:         15,
		Description:            textToPgText("Urgent drinking water"),
		Priority:               database.RequestPriorityHIGH,
		Status:                 database.RequestStatusSUBMITTED,
		Location:               reqOrigin,
		AddressText:            textToPgText("Kathmandu Center"),
		ContactPhoneEncrypted:  "+9779800000001",
		RequesterNameEncrypted: "Victim Tester",
	})
	if err != nil {
		t.Fatalf("failed to create assistance request: %v", err)
	}
	reqID := uuid.UUID(req.ID.Bytes)

	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM dispatch_matches WHERE request_id = $1", req.ID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM dispatch_pings WHERE request_id = $1", req.ID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM assistance_requests WHERE id = $1", req.ID)
	})

	// STEP 1: Initial Trigger
	// Should select Provider B (Provider A is skipped due to 0 inventory)
	ping1, err := coordinator.TriggerDispatch(ctx, reqID)
	if err != nil {
		t.Fatalf("unexpected error on initial dispatch: %v", err)
	}
	if ping1 == nil {
		t.Fatalf("expected ping1 to be non-nil")
	}
	if uuid.UUID(ping1.ProviderID.Bytes) != provBID {
		t.Fatalf("expected ping1 provider to be Provider B (%s), got: %s", provBID, uuid.UUID(ping1.ProviderID.Bytes))
	}
	if ping1.PingOrder != 1 {
		t.Fatalf("expected ping order 1, got %d", ping1.PingOrder)
	}

	// STEP 2: Rejection & Cascade
	// Provider B rejects ping -> Engine cascades to Provider C
	err = coordinator.HandleProviderReject(ctx, uuid.UUID(ping1.ID.Bytes), provBID)
	if err != nil {
		t.Fatalf("failed to reject ping1: %v", err)
	}

	// Wait briefly for the cascade goroutine to dispatch ping2
	time.Sleep(300 * time.Millisecond)

	// Provider C should now have an active pending ping
	activePingC, err := queries.GetActivePingForProvider(ctx, pgtype.UUID{Bytes: provCID, Valid: true})
	if err != nil {
		t.Fatalf("expected Provider C to receive cascaded ping, err: %v", err)
	}
	if uuid.UUID(activePingC.RequestID.Bytes) != reqID {
		t.Fatalf("expected active ping for Provider C to be request %s, got: %s", reqID, uuid.UUID(activePingC.RequestID.Bytes))
	}
	if activePingC.PingOrder != 2 {
		t.Fatalf("expected ping order 2, got %d", activePingC.PingOrder)
	}

	// STEP 3: Acceptance State
	// Provider C accepts the ping
	match, err := coordinator.HandleProviderAccept(ctx, uuid.UUID(activePingC.PingID.Bytes), provCID)
	if err != nil {
		t.Fatalf("failed to accept ping by Provider C: %v", err)
	}
	if match == nil || match.HandshakeCode == "" {
		t.Fatalf("expected valid match with handshake code, got: %+v", match)
	}
	if len(match.HandshakeCode) != 6 {
		t.Fatalf("expected 6-char handshake code, got: %s", match.HandshakeCode)
	}

	// Verify request updated to MATCHED & IN_PROGRESS
	reqAfterMatch, err := queries.GetAssistanceRequestByID(ctx, pgtype.UUID{Bytes: reqID, Valid: true})
	if err != nil {
		t.Fatalf("failed to fetch updated request: %v", err)
	}
	if reqAfterMatch.DispatchStatus != database.DispatchStatusMATCHED {
		t.Fatalf("expected dispatch_status MATCHED, got: %s", reqAfterMatch.DispatchStatus)
	}
	if reqAfterMatch.Status != database.RequestStatusINPROGRESS {
		t.Fatalf("expected status IN_PROGRESS, got: %s", reqAfterMatch.Status)
	}
	if uuid.UUID(reqAfterMatch.MatchedProviderID.Bytes) != provCID {
		t.Fatalf("expected matched_provider_id %s, got: %s", provCID, uuid.UUID(reqAfterMatch.MatchedProviderID.Bytes))
	}

	// Verify Provider C resource capacity was decremented by 15 (100 - 15 = 85)
	resC, err := queries.GetResourceByID(ctx, pgtype.UUID{Bytes: resCID, Valid: true})
	if err != nil {
		t.Fatalf("failed to fetch resource C: %v", err)
	}
	if resC.CurrentCapacity != 85 {
		t.Fatalf("expected resource capacity to be 85, got: %d", resC.CurrentCapacity)
	}

	// STEP 4: Exhaustion State Test
	// Create a new request where NO provider has capacity (need 9999 units)
	reqExhausted, err := queries.CreateAssistanceRequest(ctx, database.CreateAssistanceRequestParams{
		RequesterID:            pgtype.UUID{Valid: false},
		TrackingCode:           fmt.Sprintf("EXH-%d", ts%1000000),
		Category:               database.ResourceCategoryFOOD,
		QuantityNeeded:         9999, // Impossible quantity
		Description:            textToPgText("Massive food supplies needed"),
		Priority:               database.RequestPriorityCRITICAL,
		Status:                 database.RequestStatusSUBMITTED,
		Location:               reqOrigin,
		AddressText:            textToPgText("Kathmandu"),
		ContactPhoneEncrypted:  "+9779800000002",
		RequesterNameEncrypted: "Exhaustion Tester",
	})
	if err != nil {
		t.Fatalf("failed to create request for exhaustion test: %v", err)
	}

	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM assistance_requests WHERE id = $1", reqExhausted.ID)
	})

	exhID := uuid.UUID(reqExhausted.ID.Bytes)
	_, exhErr := coordinator.TriggerDispatch(ctx, exhID)
	if exhErr != dispatch.ErrNoCandidates {
		t.Fatalf("expected ErrNoCandidates, got: %v", exhErr)
	}

	// Verify request was marked EXHAUSTED in DB
	reqExhAfter, err := queries.GetAssistanceRequestByID(ctx, pgtype.UUID{Bytes: exhID, Valid: true})
	if err != nil {
		t.Fatalf("failed to get request: %v", err)
	}
	if reqExhAfter.DispatchStatus != database.DispatchStatusEXHAUSTED {
		t.Fatalf("expected dispatch_status EXHAUSTED, got: %s", reqExhAfter.DispatchStatus)
	}

	// Verify it shows up in GetExhaustedRequests
	exhaustedList, err := queries.GetExhaustedRequests(ctx)
	if err != nil {
		t.Fatalf("failed to query exhausted requests: %v", err)
	}
	found := false
	for _, item := range exhaustedList {
		if uuid.UUID(item.ID.Bytes) == exhID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected request %s to appear in exhausted requests list", exhID)
	}
}

func TestWorkerTimeoutExpiration(t *testing.T) {
	queries, pool := setupTestDB(t)

	ctx := context.Background()
	// Set coordinator timeout to 1 second
	coordinator := dispatch.NewCoordinator(queries, pool, 1*time.Second)
	worker := dispatch.NewWorker(coordinator, 500*time.Millisecond)

	ts := time.Now().UnixNano()
	// Isolated coordinate space (20.0000, 20.0000) and category SHELTER
	reqOrigin := "POINT(20.0000 20.0000)"

	// Provider 1 (~1km)
	prov1ID, _ := createTestProviderWithResource(t, ctx, queries, pool,
		fmt.Sprintf("PTimeout1_%d", ts),
		fmt.Sprintf("+97714%d", ts%1000000),
		fmt.Sprintf("pto1_%d@test.com", ts),
		20.0100, 20.0100,
		database.ResourceCategorySHELTER,
		50,
	)

	// Provider 2 (~3km)
	prov2ID, _ := createTestProviderWithResource(t, ctx, queries, pool,
		fmt.Sprintf("PTimeout2_%d", ts),
		fmt.Sprintf("+97715%d", ts%1000000),
		fmt.Sprintf("pto2_%d@test.com", ts),
		20.0300, 20.0300,
		database.ResourceCategorySHELTER,
		50,
	)

	// Create request
	req, err := queries.CreateAssistanceRequest(ctx, database.CreateAssistanceRequestParams{
		RequesterID:            pgtype.UUID{Valid: false},
		TrackingCode:           fmt.Sprintf("TO-%d", ts%1000000),
		Category:               database.ResourceCategorySHELTER,
		QuantityNeeded:         5,
		Description:            textToPgText("Emergency Shelter"),
		Priority:               database.RequestPriorityHIGH,
		Status:                 database.RequestStatusSUBMITTED,
		Location:               reqOrigin,
		AddressText:            textToPgText("Shelter Site"),
		ContactPhoneEncrypted:  "+9779800000003",
		RequesterNameEncrypted: "Timeout Tester",
	})
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	reqID := uuid.UUID(req.ID.Bytes)

	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM dispatch_matches WHERE request_id = $1", req.ID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM dispatch_pings WHERE request_id = $1", req.ID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM assistance_requests WHERE id = $1", req.ID)
	})

	// Trigger initial dispatch -> Ping created for Provider 1
	ping1, err := coordinator.TriggerDispatch(ctx, reqID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uuid.UUID(ping1.ProviderID.Bytes) != prov1ID {
		t.Fatalf("expected ping for Provider 1, got %s", uuid.UUID(ping1.ProviderID.Bytes))
	}

	// Wait 1.2s so ping1 expires
	time.Sleep(1200 * time.Millisecond)

	// Run worker CheckExpiredPings
	worker.CheckExpiredPings(ctx)

	// Verify ping1 is now EXPIRED
	p1After, err := queries.GetPingByID(ctx, ping1.ID)
	if err != nil {
		t.Fatalf("failed to get ping1: %v", err)
	}
	if p1After.Status != database.DispatchPingStatusEXPIRED {
		t.Fatalf("expected ping1 to be EXPIRED, got %s", p1After.Status)
	}

	// Verify Provider 2 now has the cascaded ping
	activePing2, err := queries.GetActivePingForProvider(ctx, pgtype.UUID{Bytes: prov2ID, Valid: true})
	if err != nil {
		t.Fatalf("expected Provider 2 to have active ping after cascade, err: %v", err)
	}
	if uuid.UUID(activePing2.RequestID.Bytes) != reqID {
		t.Fatalf("expected ping for request %s, got: %s", reqID, uuid.UUID(activePing2.RequestID.Bytes))
	}
}
