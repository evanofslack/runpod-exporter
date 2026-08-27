// Package metrics holds the Prometheus metric descriptors for every domain,
// plus the exporter's own self-observability metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ScrapeErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "runpod_exporter_scrape_errors_total",
		Help: "Total number of failed polls, by domain.",
	}, []string{"domain"})

	LastSuccessTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_exporter_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful poll, by domain.",
	}, []string{"domain"})

	ScrapeDuration = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_exporter_scrape_duration_seconds",
		Help: "Wall-clock duration of the last poll, by domain.",
	}, []string{"domain"})
)
