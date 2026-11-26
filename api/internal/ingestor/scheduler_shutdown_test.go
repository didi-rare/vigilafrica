package ingestor

import (
	"context"
	"testing"
	"time"
)

// lockingRepo is a Repository with the scheduler lock (mockRepo + fakeLock).
type lockingRepo struct {
	*mockRepo
	*fakeLock
}

// blockingLoader blocks until the run's context is cancelled, like a boundary
// download interrupted by SIGTERM.
type blockingLoader struct{ started chan struct{} }

func (l *blockingLoader) Run(ctx context.Context, _ func(context.Context) error) error {
	close(l.started)
	<-ctx.Done()
	return ctx.Err()
}

// Regression for the independent review of feat/boundary-loader (P1): main
// closed the pool right after srv.Shutdown, before the scheduler goroutine had
// released the lease, so a stopped container kept the lease for a whole TTL.
// StartScheduler's done channel must close only AFTER the release.
func TestStartSchedulerDoneClosesAfterLeaseRelease(t *testing.T) {
	lock := &fakeLock{answers: []bool{true}}
	repo := lockingRepo{mockRepo: &mockRepo{}, fakeLock: lock}
	loader := &blockingLoader{started: make(chan struct{})}

	ctx, cancel := context.WithCancel(context.Background())
	done := StartScheduler(ctx, repo, nil, loader)

	select {
	case <-loader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the startup run never reached the boundary loader")
	}
	cancel() // SIGTERM mid-run

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StartScheduler's done channel did not close after cancellation")
	}
	lock.mu.Lock()
	defer lock.mu.Unlock()
	if lock.releases != 1 {
		t.Fatalf("lease released %d times by the time done closed; want 1 (main closes the pool next)", lock.releases)
	}
	if !lock.releaseCtxAlive[0] {
		t.Error("lease released on the cancelled shutdown context")
	}
}

func TestStartSchedulerDisabledReturnsClosedChannel(t *testing.T) {
	t.Setenv("INGEST_INTERVAL_MIN", "0")
	done := StartScheduler(context.Background(), lockingRepo{mockRepo: &mockRepo{}, fakeLock: &fakeLock{answers: []bool{true}}}, nil, nil)
	select {
	case <-done:
	default:
		t.Fatal("a disabled scheduler must return an already-closed channel")
	}
}
