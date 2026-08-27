package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

// fakeDomain lets domain_test.go exercise Run's shared error-counting and
// stale-serve mechanics without a real API — this is the behavior every
// real domain gets for free from the runner, not something each domain's
// own tests need to re-verify.
type fakeDomain struct {
	name   string
	pollFn func(ctx context.Context) error
}

func (f *fakeDomain) Name() string { return f.name }
func (f *fakeDomain) Tier() Tier   { return Fast }
func (f *fakeDomain) Poll(ctx context.Context) error {
	return f.pollFn(ctx)
}

func resetSelfMetrics(name string) {
	metrics.ScrapeErrorsTotal.DeleteLabelValues(name)
	metrics.LastSuccessTimestamp.DeleteLabelValues(name)
	metrics.ScrapeDuration.DeleteLabelValues(name)
}

// runOnce runs Run with a long tick interval so only the immediate,
// on-start poll fires, then cancels as soon as that poll has happened.
func runOnce(t *testing.T, d Domain) {
	t.Helper()
	done := make(chan struct{})
	fd := d.(*fakeDomain)
	inner := fd.pollFn
	fd.pollFn = func(ctx context.Context) error {
		err := inner(ctx)
		close(done)
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-done
		cancel()
	}()

	finished := make(chan struct{})
	go func() {
		Run(ctx, []Domain{d}, time.Hour, time.Hour)
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
}

func TestRun_SuccessUpdatesSelfObservability(t *testing.T) {
	resetSelfMetrics("fake")
	d := &fakeDomain{name: "fake", pollFn: func(ctx context.Context) error { return nil }}
	runOnce(t, d)

	if got := testutil.ToFloat64(metrics.ScrapeErrorsTotal.WithLabelValues("fake")); got != 0 {
		t.Errorf("ScrapeErrorsTotal = %v, want 0", got)
	}
	if got := testutil.ToFloat64(metrics.LastSuccessTimestamp.WithLabelValues("fake")); got == 0 {
		t.Error("LastSuccessTimestamp was not set")
	}
}

func TestRun_ErrorIncrementsCounterAndStaleServes(t *testing.T) {
	resetSelfMetrics("fake")

	// First poll succeeds, establishing a "last known good" timestamp.
	d := &fakeDomain{name: "fake", pollFn: func(ctx context.Context) error { return nil }}
	runOnce(t, d)
	firstSuccess := testutil.ToFloat64(metrics.LastSuccessTimestamp.WithLabelValues("fake"))

	// Second poll fails: error count increments, last-success stays put
	// (stale-serve — this is what makes the last good scrape keep serving).
	d.pollFn = func(ctx context.Context) error { return errors.New("boom") }
	runOnce(t, d)

	if got := testutil.ToFloat64(metrics.ScrapeErrorsTotal.WithLabelValues("fake")); got != 1 {
		t.Errorf("ScrapeErrorsTotal = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.LastSuccessTimestamp.WithLabelValues("fake")); got != firstSuccess {
		t.Errorf("LastSuccessTimestamp changed on a failed poll: got %v, want %v", got, firstSuccess)
	}
}

// withShortTimings shrinks pollTimeout/shutdownGrace for the duration of a
// test so shutdown-grace tests don't run at real-world (15s/5s) durations.
func withShortTimings(t *testing.T, timeout, grace time.Duration) {
	t.Helper()
	origTimeout, origGrace := pollTimeout, shutdownGrace
	pollTimeout, shutdownGrace = timeout, grace
	t.Cleanup(func() { pollTimeout, shutdownGrace = origTimeout, origGrace })
}

func TestPoll_ForceCancelsAfterShutdownGrace(t *testing.T) {
	withShortTimings(t, time.Second, 20*time.Millisecond)

	started := make(chan struct{})
	pollCtxErr := make(chan error, 1)
	d := &fakeDomain{name: "fake", pollFn: func(ctx context.Context) error {
		close(started)
		<-ctx.Done() // blocks until force-canceled
		pollCtxErr <- ctx.Err()
		return ctx.Err()
	}}

	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()
	go func() {
		<-started
		parentCancel() // simulate shutdown while the poll is still in flight
	}()

	done := make(chan struct{})
	go func() {
		poll(parentCtx, d)
		close(done)
	}()

	select {
	case err := <-pollCtxErr:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("ctx.Err() = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("poll's context was never force-canceled after shutdownGrace")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poll did not return after force-cancel")
	}
}

func TestPoll_FinishesWithinGraceWithoutForceCancel(t *testing.T) {
	withShortTimings(t, time.Second, 200*time.Millisecond)

	started := make(chan struct{})
	canceledEarly := make(chan bool, 1)
	d := &fakeDomain{name: "fake", pollFn: func(ctx context.Context) error {
		close(started)
		time.Sleep(20 * time.Millisecond) // finishes well within shutdownGrace
		canceledEarly <- ctx.Err() != nil
		return nil
	}}

	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()
	go func() {
		<-started
		parentCancel()
	}()

	poll(parentCtx, d) // pollFn runs synchronously inside this call

	if wasCanceled := <-canceledEarly; wasCanceled {
		t.Error("poll's context was canceled before it finished, even though it completed within shutdownGrace")
	}
}
