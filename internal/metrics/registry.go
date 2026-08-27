package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RegistryInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_registry_info",
		Help: "Always 1. Container registry inventory/info labels.",
	}, []string{"registry_id", "name"})

	RegistryDelegationCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "runpod_registry_delegation_count",
		Help: "Number of ECR delegations on the account.",
	})
)
