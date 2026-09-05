package dispatch

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-sse-server/internal/database"
	"go-sse-server/internal/ml"
)

var (
	ErrNoCandidates          = errors.New("no matching candidate providers found")
	ErrRequestAlreadyHandled = errors.New("request is already matched, fulfilled, or cancelled")
	ErrPingNotFound          = errors.New("dispatch ping not found")
	ErrPingExpired           = errors.New("dispatch ping has expired")
	ErrPingNotPending        = errors.New("dispatch ping is no longer pending")
	ErrUnauthorizedProvider  = errors.New("provider is not authorized for this ping")
)

const (
	DefaultPingTimeout = 3 * time.Minute
	HandshakeCharset   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // omits easily confused chars (I, O, 0, 1)
)

type Coordinator struct {
	queries  *database.Queries
	pool     *pgxpool.Pool
	timeout  time.Duration
	mlClient *ml.Client
}

func NewCoordinator(queries *database.Queries, pool *pgxpool.Pool, timeout time.Duration) *Coordinator {
	if timeout <= 0 {
		timeout = DefaultPingTimeout
	}
	return &Coordinator{
		queries: queries,
		pool:    pool,
		timeout: timeout,
	}
}

func (c *Coordinator) SetMLClient(client *ml.Client) {
	c.mlClient = client
}

func (c *Coordinator) SetTimeout(d time.Duration) {
	c.timeout = d
}

// Generate a clean, human-readable 6-character handshake code
func GenerateHandshakeCode() string {
	b := make([]byte, 6)
	charsetLen := big.NewInt(int64(len(HandshakeCharset)))
	for i := range b {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			b[i] = HandshakeCharset[i%len(HandshakeCharset)]
		} else {
			b[i] = HandshakeCharset[idx.Int64()]
		}
	}
	return string(b)
}

// TriggerDispatch attempts to find the nearest matching provider and dispatch a ping.
// If no candidates remain, it transitions the request to EXHAUSTED.
func (c *Coordinator) TriggerDispatch(ctx context.Context, requestID uuid.UUID) (*database.DispatchPing, error) {
	pgReqID := pgtype.UUID{Bytes: requestID, Valid: true}

	// 1. Verify request state
	req, err := c.queries.GetAssistanceRequestByID(ctx, pgReqID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPingNotFound
		}
		return nil, fmt.Errorf("failed to fetch assistance request: %w", err)
	}

	if req.DispatchStatus == database.DispatchStatusMATCHED ||
		req.Status == database.RequestStatusFULFILLED ||
		req.Status == database.RequestStatusCANCELLED {
		return nil, ErrRequestAlreadyHandled
	}

	// 2. Check if an active pending ping is already out for this request
	hasActive, err := c.queries.HasActivePingForRequest(ctx, pgReqID)
	if err == nil && hasActive {
		log.Printf("[Dispatch] Request %s already has an active pending ping. Skipping duplicate ping.\n", requestID)
		return nil, nil
	}

	// 3. Query closest candidates with matching inventory who haven't been pinged yet
	candidates, err := c.queries.FindCandidateProvidersForRequest(ctx, database.FindCandidateProvidersForRequestParams{
		ID:    pgReqID,
		Limit: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query candidate providers: %w", err)
	}

	// 4. Exhaustion State: No candidate providers found or all have rejected/expired
	if len(candidates) == 0 {
		log.Printf("[Dispatch] Request %s: No candidate providers found or all candidates exhausted. Marking EXHAUSTED.\n", requestID)
		if markErr := c.queries.MarkRequestExhausted(ctx, pgReqID); markErr != nil {
			log.Printf("[Dispatch] Failed to mark request %s as EXHAUSTED: %v\n", requestID, markErr)
		}
		return nil, ErrNoCandidates
	}

	selectedCandidate := candidates[0]

	// 4b. If ML client is available and we have multiple candidates, rank semantically
	if c.mlClient != nil && len(candidates) > 1 {
		reqDetails, err := c.queries.GetAssistanceRequestByID(ctx, pgReqID)
		queryText := ""
		if err == nil {
			if reqDetails.Description.Valid && reqDetails.Description.String != "" {
				queryText = reqDetails.Description.String
			} else {
				queryText = string(reqDetails.Category)
			}
		}

		if queryText != "" {
			var mlCandidates []ml.Candidate
			for _, cand := range candidates {
				mlCandidates = append(mlCandidates, ml.Candidate{
					ID:   uuid.UUID(cand.ResourceID.Bytes).String(),
					Text: cand.ResourceTitle,
				})
			}

			matches, _, matchErr := c.mlClient.Match(ctx, queryText, len(mlCandidates), mlCandidates)
			if matchErr == nil && len(matches) > 0 {
				scoreMap := make(map[string]float64)
				for _, m := range matches {
					scoreMap[m.ID] = m.Score
				}

				bestIdx := 0
				bestScore := -999.0
				for idx, cand := range candidates {
					resIDStr := uuid.UUID(cand.ResourceID.Bytes).String()
					semScore := scoreMap[resIDStr]
					distKm := float64(cand.DistanceMeters) / 1000.0
					combinedScore := semScore - (distKm * 0.002)

					if semScore >= 0.20 && combinedScore > bestScore {
						bestScore = combinedScore
						bestIdx = idx
					}
				}
				if bestScore > -999.0 {
					selectedCandidate = candidates[bestIdx]
				}
			}
		}
	}

	candidate := selectedCandidate

	// 5. Calculate ping order and expiration time
	nextOrder, err := c.queries.GetNextPingOrderForRequest(ctx, pgReqID)
	if err != nil {
		nextOrder = 1
	}

	expiresAt := time.Now().Add(c.timeout)

	// 6. Create dispatch ping
	ping, err := c.queries.CreateDispatchPing(ctx, database.CreateDispatchPingParams{
		RequestID:  pgReqID,
		ProviderID: candidate.ProviderID,
		PingOrder:  nextOrder,
		ExpiresAt:  pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create dispatch ping: %w", err)
	}

	// 7. Update request dispatch status to DISPATCHING
	if err := c.queries.MarkRequestDispatching(ctx, pgReqID); err != nil {
		log.Printf("[Dispatch] Warning: failed to mark request %s as DISPATCHING: %v\n", requestID, err)
	}

	log.Printf("[Dispatch] Ping #%d created for request %s -> Provider '%s' (%s) | Distance: %dm | Expires: %s\n",
		nextOrder, requestID, candidate.ProviderName, uuid.UUID(candidate.ProviderID.Bytes).String(), candidate.DistanceMeters, expiresAt.Format(time.RFC3339))

	return &ping, nil
}

// HandleProviderAccept executes the acceptance transaction:
// - Verifies ping status and validity
// - Marks ping ACCEPTED
// - Generates a 6-character handshake code and creates dispatch_matches
// - Sets assistance_request to MATCHED and IN_PROGRESS
// - Deducts inventory from provider's resources
// - Increments provider's current active tasks
func (c *Coordinator) HandleProviderAccept(ctx context.Context, pingID uuid.UUID, providerID uuid.UUID) (*database.DispatchMatch, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start database transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := c.queries.WithTx(tx)

	// 1. Fetch ping
	ping, err := qtx.GetPingByID(ctx, pgtype.UUID{Bytes: pingID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPingNotFound
		}
		return nil, fmt.Errorf("failed to get ping: %w", err)
	}

	if ping.ProviderID.Bytes != providerID {
		return nil, ErrUnauthorizedProvider
	}

	if ping.Status != database.DispatchPingStatusPENDING {
		return nil, ErrPingNotPending
	}

	if ping.ExpiresAt.Time.Before(time.Now()) {
		_, _ = qtx.UpdatePingStatus(ctx, database.UpdatePingStatusParams{
			ID:     ping.ID,
			Status: database.DispatchPingStatusEXPIRED,
		})
		_ = tx.Commit(ctx)
		return nil, ErrPingExpired
	}

	// 2. Fetch assistance request details for resource category and quantity
	req, err := qtx.GetAssistanceRequestByID(ctx, ping.RequestID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch assistance request: %w", err)
	}

	// 3. Mark ping as ACCEPTED
	acceptedPing, err := qtx.UpdatePingStatus(ctx, database.UpdatePingStatusParams{
		ID:     ping.ID,
		Status: database.DispatchPingStatusACCEPTED,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update ping status to ACCEPTED: %w", err)
	}

	// 4. Generate handshake code and create dispatch match
	handshakeCode := GenerateHandshakeCode()
	match, err := qtx.CreateDispatchMatch(ctx, database.CreateDispatchMatchParams{
		RequestID:     ping.RequestID,
		ProviderID:    ping.ProviderID,
		HandshakeCode: handshakeCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create dispatch match: %w", err)
	}

	// 5. Update assistance request to MATCHED and IN_PROGRESS
	if err := qtx.MarkRequestMatched(ctx, database.MarkRequestMatchedParams{
		ID:                ping.RequestID,
		MatchedProviderID: ping.ProviderID,
	}); err != nil {
		return nil, fmt.Errorf("failed to mark request as matched: %w", err)
	}

	// 6. Deduct resource capacity from provider's inventory
	if err := qtx.DeductProviderResourceCapacityByCategory(ctx, database.DeductProviderResourceCapacityByCategoryParams{
		ProviderID:      ping.ProviderID,
		Category:        req.Category,
		CurrentCapacity: req.QuantityNeeded,
	}); err != nil {
		log.Printf("[Dispatch] Warning: Deduct resource capacity query returned: %v\n", err)
	}

	// 7. Increment provider's active task count
	if err := qtx.IncrementProviderActiveTasks(ctx, ping.ProviderID); err != nil {
		log.Printf("[Dispatch] Warning: Increment active tasks returned: %v\n", err)
	}

	// 8. Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit match transaction: %w", err)
	}

	log.Printf("[Dispatch] MATCH CONFIRMED: Request %s -> Provider %s | Ping #%d accepted | Handshake Code: %s\n",
		uuid.UUID(acceptedPing.RequestID.Bytes).String(),
		uuid.UUID(acceptedPing.ProviderID.Bytes).String(),
		acceptedPing.PingOrder,
		handshakeCode,
	)

	return &match, nil
}

// HandleProviderReject sets the ping to REJECTED and immediately triggers cascade to next nearest provider
func (c *Coordinator) HandleProviderReject(ctx context.Context, pingID uuid.UUID, providerID uuid.UUID) error {
	ping, err := c.queries.GetPingByID(ctx, pgtype.UUID{Bytes: pingID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPingNotFound
		}
		return fmt.Errorf("failed to get ping: %w", err)
	}

	if ping.ProviderID.Bytes != providerID {
		return ErrUnauthorizedProvider
	}

	if ping.Status != database.DispatchPingStatusPENDING {
		return ErrPingNotPending
	}

	// Mark ping REJECTED
	if _, err := c.queries.UpdatePingStatus(ctx, database.UpdatePingStatusParams{
		ID:     ping.ID,
		Status: database.DispatchPingStatusREJECTED,
	}); err != nil {
		return fmt.Errorf("failed to mark ping as REJECTED: %w", err)
	}

	log.Printf("[Dispatch] Provider %s REJECTED ping %s for request %s. Cascading to next candidate...\n",
		providerID, pingID, uuid.UUID(ping.RequestID.Bytes).String())

	// Trigger cascade immediately in a background goroutine
	reqUUID := uuid.UUID(ping.RequestID.Bytes)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, cascadeErr := c.TriggerDispatch(bgCtx, reqUUID); cascadeErr != nil && !errors.Is(cascadeErr, ErrNoCandidates) {
			log.Printf("[Dispatch] Error cascading request %s after rejection: %v\n", reqUUID, cascadeErr)
		}
	}()

	return nil
}
