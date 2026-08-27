package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ClusterPods = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_cluster_pods",
		Help: "Member pod count by status, from ClusterPodsSummary.byStatus.",
	}, []string{"cluster_id", "status"})

	ClusterInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_cluster_info",
		Help: "Always 1. Cluster inventory/info labels.",
	}, []string{"cluster_id", "type", "data_center_id"})
)
