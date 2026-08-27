// Package collector polls the Runpod v2 REST API and updates the metrics in
// internal/metrics. Poll-cache-serve: each Domain's Poll runs on its own
// ticker and updates its metric vecs in place; /metrics never blocks on a
// live API call.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

type Tier int

const (
	Fast Tier = iota
	Slow
)

// pollTimeout bounds a single Poll call so one hung request can't wedge a
// domain's loop forever.
const pollTimeout = 15 * time.Second

type Domain interface {
	Name() string
	Tier() Tier
	// Poll updates this domain's metric vecs in place.
	Poll(ctx context.Context) error
}

// Run starts one goroutine per domain, polling immediately and then on its
// own ticker (fastInterval for Tier Fast, slowInterval for Tier Slow), until
// ctx is canceled. It blocks until every domain's loop has stopped.
func Run(ctx context.Context, domains []Domain, fastInterval, slowInterval time.Duration) {
	var wg sync.WaitGroup
	for _, d := range domains {
		interval := fastInterval
		if d.Tier() == Slow {
			interval = slowInterval
		}
		wg.Add(1)
		go func(d Domain, interval time.Duration) {
			defer wg.Done()
			loop(ctx, d, interval)
		}(d, interval)
	}
	wg.Wait()
}

// loop only checks for shutdown between ticks, not mid-request: an in-flight
// poll is left to finish on its own (bounded by pollTimeout) rather than
// being aborted the instant ctx is canceled, per §8.
func loop(ctx context.Context, d Domain, interval time.Duration) {
	poll(d)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll(d)
		}
	}
}

func poll(d Domain) {
	callCtx, cancel := context.WithTimeout(context.Background(), pollTimeout)
	defer cancel()

	start := time.Now()
	err := d.Poll(callCtx)
	elapsed := time.Since(start)

	if err != nil {
		slog.Error("poll failed", "domain", d.Name(), "elapsed_ms", elapsed.Milliseconds(), "error", err)
		metrics.ScrapeErrorsTotal.WithLabelValues(d.Name()).Inc()
		return
	}

	metrics.LastSuccessTimestamp.WithLabelValues(d.Name()).Set(float64(time.Now().Unix()))
	metrics.ScrapeDuration.WithLabelValues(d.Name()).Set(elapsed.Seconds())
}
