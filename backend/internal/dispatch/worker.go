package dispatch

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"go-sse-server/internal/database"
)

type Worker struct {
	coordinator *Coordinator
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func NewWorker(coordinator *Coordinator, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Worker{
		coordinator: coordinator,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		log.Printf("[Dispatch Worker] Started timeout monitor (interval: %v)\n", w.interval)

		for {
			select {
			case <-w.stopCh:
				log.Println("[Dispatch Worker] Timeout monitor stopped")
				return
			case <-ctx.Done():
				log.Println("[Dispatch Worker] Context cancelled, stopping monitor")
				return
			case <-ticker.C:
				w.CheckExpiredPings(ctx)
			}
		}
	}()
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// CheckExpiredPings finds all pending pings whose expires_at is in the past, marks them EXPIRED, and cascades
func (w *Worker) CheckExpiredPings(ctx context.Context) {
	expiredPings, err := w.coordinator.queries.GetExpiredPings(ctx)
	if err != nil {
		log.Printf("[Dispatch Worker] Error querying expired pings: %v\n", err)
		return
	}

	for _, ping := range expiredPings {
		_, err := w.coordinator.queries.UpdatePingStatus(ctx, database.UpdatePingStatusParams{
			ID:     ping.ID,
			Status: database.DispatchPingStatusEXPIRED,
		})
		if err != nil {
			log.Printf("[Dispatch Worker] Failed to mark ping %s as EXPIRED: %v\n", uuid.UUID(ping.ID.Bytes).String(), err)
			continue
		}

		reqUUID := uuid.UUID(ping.RequestID.Bytes)
		log.Printf("[Dispatch Worker] Ping %s expired for request %s. Cascading to next candidate...\n",
			uuid.UUID(ping.ID.Bytes).String(), reqUUID)

		if _, cascadeErr := w.coordinator.TriggerDispatch(ctx, reqUUID); cascadeErr != nil && !errors.Is(cascadeErr, ErrNoCandidates) {
			log.Printf("[Dispatch Worker] Error cascading request %s: %v\n", reqUUID, cascadeErr)
		}
	}
}
