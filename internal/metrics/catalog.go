package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	CatalogGpuPriceDollarsPerHour = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_catalog_gpu_price_dollars_per_hour",
		Help: "List price in USD/hour for a single GPU of this type, by cloud.",
	}, []string{"gpu_id", "cloud"})

	// CatalogCpuPriceDollarsPerVcpuHour deviates from the original spec name
	// (runpod_catalog_cpu_price_dollars_per_hour, no cloud label): the API's
	// CpuType.price is per-vCPU and split by context (secure/serverless), not
	// a single flat hourly rate. Renamed and given a cloud label to state
	// that honestly, matching the shape of the GPU price metric. Confirmed
	// with the user.
	CatalogCpuPriceDollarsPerVcpuHour = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_catalog_cpu_price_dollars_per_vcpu_hour",
		Help: "List price in USD/hour per vCPU for this CPU flavor, by cloud (secure|serverless).",
	}, []string{"cpu_id", "cloud"})

	// CatalogGpuAvailability adds a product label beyond the original spec
	// (gpu_id,data_center_id only): the API requires an explicit product
	// context (POD/SERVERLESS/CLUSTER) for availability data, since the same
	// GPU can be scarce for one and plentiful for another. Confirmed with
	// the user. AvailabilityLevel is mapped to an int: NONE=0, LOW=1,
	// MEDIUM=2, HIGH=3.
	CatalogGpuAvailability = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "runpod_catalog_gpu_availability",
		Help: "GPU stock availability by data center and product context. NONE=0, LOW=1, MEDIUM=2, HIGH=3.",
	}, []string{"gpu_id", "data_center_id", "product"})
)
