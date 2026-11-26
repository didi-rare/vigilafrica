package ingestor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeLock is a scheduler_locks double: acquire answers come from a queue
// (the last answer repeats), and every call records whether its context was
// already cancelled.
type fakeLock struct {
	mu              sync.Mutex
	answers         []bool
	acquireCalls    int
	releases        int
	releaseCtxAlive []bool
}

func (f *fakeLock) TryAcquireSchedulerLock(context.Context, string, string, time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.acquireCalls
	f.acquireCalls++
	if i >= len(f.answers) {
		i = len(f.answers) - 1
	}
	return f.answers[i], nil
}

func (f *fakeLock) ReleaseSchedulerLock(ctx context.Context, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releases++
	f.releaseCtxAlive = append(f.releaseCtxAlive, ctx.Err() == nil)
	return nil
}

// fakeLoader records calls and can renew the lease or fail.
type fakeLoader struct {
	calls   int
	renew   bool // call renewLease once inside Run
	err     error
	onRun   func()
	renewEr error
}

func (l *fakeLoader) Run(ctx context.Context, renewLease func(context.Context) error) error {
	l.calls++
	if l.onRun != nil {
		l.onRun()
	}
	if l.renew {
		if err := renewLease(ctx); err != nil {
			l.renewEr = err
			return fmt.Errorf("boundary loader: %w", err)
		}
	}
	return l.err
}

func TestLockedRunLoadsBoundariesBeforeIngestion(t *testing.T) {
	lock := &fakeLock{answers: []bool{true}}
	var order []string
	loader := &fakeLoader{onRun: func() { order = append(order, "boundaries") }}

	runLocked(context.Background(), lock, "h:1", loader, func(context.Context) { order = append(order, "ingest") })

	if fmt.Sprint(order) != "[boundaries ingest]" {
		t.Errorf("order = %v, want boundaries before ingestion", order)
	}
	if lock.releases != 1 {
		t.Errorf("lease released %d times, want 1", lock.releases)
	}
}

// Spec §2 / acceptance: with the lease held at boot, the loader runs on the
// next tick (the loader runs in every locked run, not only at startup).
func TestLeaseHeldAtBootLoaderRunsOnNextTick(t *testing.T) {
	lock := &fakeLock{answers: []bool{false, true}}
	loader := &fakeLoader{}
	ingests := 0
	ingest := func(context.Context) { ingests++ }

	runLocked(context.Background(), lock, "h:1", loader, ingest) // startup: lease held elsewhere
	if loader.calls != 0 || ingests != 0 || lock.releases != 0 {
		t.Fatalf("startup with the lease held: loader=%d ingest=%d releases=%d; want nothing run or released", loader.calls, ingests, lock.releases)
	}
	runLocked(context.Background(), lock, "h:1", loader, ingest) // first tick
	if loader.calls != 1 || ingests != 1 {
		t.Errorf("next tick: loader=%d ingest=%d, want both once", loader.calls, ingests)
	}
}

func TestLockedRunStopsWhenRenewalFails(t *testing.T) {
	lock := &fakeLock{answers: []bool{true, false}} // acquire ok, renewal refused
	loader := &fakeLoader{renew: true}
	ingested := false

	runLocked(context.Background(), lock, "h:1", loader, func(context.Context) { ingested = true })

	if !errors.Is(loader.renewEr, ErrLeaseLost) {
		t.Errorf("renew error = %v, want ErrLeaseLost", loader.renewEr)
	}
	if ingested {
		t.Error("ingestion ran after the lease was lost")
	}
}

func TestLockedRunIngestsAfterANonLeaseLoaderError(t *testing.T) {
	lock := &fakeLock{answers: []bool{true}}
	loader := &fakeLoader{err: errors.New("boundary_sources unreadable")}
	ingested := false

	runLocked(context.Background(), lock, "h:1", loader, func(context.Context) { ingested = true })

	if !ingested {
		t.Error("a boundary load error must not stop ingestion on the boundaries already stored")
	}
}

// Acceptance: a stopped container frees the lease. The run's context is
// cancelled by SIGTERM; the release must still run on a live context.
func TestLockReleasedOnFreshContextAfterShutdown(t *testing.T) {
	lock := &fakeLock{answers: []bool{true}}
	ctx, cancel := context.WithCancel(context.Background())
	loader := &fakeLoader{onRun: cancel} // SIGTERM arrives during the run

	runLocked(ctx, lock, "h:1", loader, func(context.Context) {})

	if lock.releases != 1 || !lock.releaseCtxAlive[0] {
		t.Errorf("releases=%d live-context=%v; want one release on a non-cancelled context", lock.releases, lock.releaseCtxAlive)
	}
}

func TestRunWithoutLockStillLoadsBoundaries(t *testing.T) {
	loader := &fakeLoader{renew: true}
	ingested := false
	runLocked(context.Background(), nil, "", loader, func(context.Context) { ingested = true })
	if loader.calls != 1 || loader.renewEr != nil || !ingested {
		t.Errorf("no lock repo: loader=%d renewErr=%v ingested=%v", loader.calls, loader.renewEr, ingested)
	}
}
