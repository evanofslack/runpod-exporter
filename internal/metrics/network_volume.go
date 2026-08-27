package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var NetworkVolumeSizeGB = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "runpod_network_volume_size_gb",
	Help: "Allocated network volume storage in GB.",
}, []string{"volume_id", "data_center_id", "type"})
