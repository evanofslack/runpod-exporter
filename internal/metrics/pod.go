package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PodUp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_up",
		Help: "1 if the pod exists, 0 otherwise. Labeled with its current status.",
	}, []string{"pod_id", "pod_name", "status"})

	PodCPUUtilPercent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_cpu_util_percent",
		Help: "CPU utilization percent. Absent while the pod's runtime is null.",
	}, []string{"pod_id"})

	PodMemoryUtilPercent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_memory_util_percent",
		Help: "Memory utilization percent. Absent while the pod's runtime is null.",
	}, []string{"pod_id"})

	PodGPUUtilPercent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_gpu_util_percent",
		Help: "Per-GPU utilization percent. Absent while the pod's runtime is null.",
	}, []string{"pod_id", "gpu_index"})

	PodGPUMemoryUtilPercent = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_gpu_memory_util_percent",
		Help: "Per-GPU memory utilization percent. Absent while the pod's runtime is null.",
	}, []string{"pod_id", "gpu_index"})

	PodUptimeSeconds = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_uptime_seconds",
		Help: "Seconds since the pod's container started. Absent while the pod's runtime is null.",
	}, []string{"pod_id"})

	PodCostPerHourDollars = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_cost_per_hour_dollars",
		Help: "Current pod cost in USD per hour.",
	}, []string{"pod_id"})

	PodDiskGB = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_disk_gb",
		Help: "Pod container disk size in GB.",
	}, []string{"pod_id"})

	PodInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_pod_info",
		Help: "Always 1. Pod inventory/info labels.",
	}, []string{"pod_id", "image", "data_center_id", "cloud", "gpu_id"})
)
