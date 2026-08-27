package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ServerlessWorkers = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_serverless_workers",
		Help: "Worker count by state, from WorkerSummary.",
	}, []string{"endpoint_id", "state"})

	ServerlessWorkerStale = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_serverless_worker_stale",
		Help: "Count of workers running an older endpoint configuration (Worker.isStale).",
	}, []string{"endpoint_id"})

	ServerlessWorkersMin = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_serverless_workers_min",
		Help: "Configured minimum worker count.",
	}, []string{"endpoint_id"})

	ServerlessWorkersMax = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_serverless_workers_max",
		Help: "Configured maximum worker count.",
	}, []string{"endpoint_id"})

	ServerlessInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_serverless_info",
		Help: "Always 1. Endpoint inventory/info labels.",
	}, []string{"endpoint_id", "name", "type", "flashboot"})
)
