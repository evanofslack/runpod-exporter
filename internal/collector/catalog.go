package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type CatalogDomain struct {
	client *openapi.ClientWithResponses
}

func NewCatalogDomain(client *openapi.ClientWithResponses) *CatalogDomain {
	return &CatalogDomain{client: client}
}

func (d *CatalogDomain) Name() string { return "catalog" }
func (d *CatalogDomain) Tier() Tier   { return Slow }

// gpuAvailabilityProducts are the contexts requested for
// runpod_catalog_gpu_availability's product label — see the spec's
// decisions log for why this isn't just one context.
var gpuAvailabilityProducts = []openapi.Product{
	openapi.ProductPOD,
	openapi.ProductSERVERLESS,
	openapi.ProductCLUSTER,
}

func availabilityToFloat(level openapi.AvailabilityLevel) float64 {
	switch level {
	case openapi.NONE:
		return 0
	case openapi.LOW:
		return 1
	case openapi.MEDIUM:
		return 2
	case openapi.HIGH:
		return 3
	default:
		return 0
	}
}

// Poll fetches every call before touching any metric vec — all-or-nothing,
// same as every other multi-call domain (see the serverless domain for why).
func (d *CatalogDomain) Poll(ctx context.Context) error {
	include := openapi.CatalogIncludeParam{openapi.AVAILABILITY}

	gpuResps := make([]*openapi.ListGpuTypesResponse, len(gpuAvailabilityProducts))
	for i, p := range gpuAvailabilityProducts {
		product := openapi.GpuProductFilter{p}
		resp, err := d.client.ListGpuTypesWithResponse(ctx, &openapi.ListGpuTypesParams{Include: &include, Product: &product})
		if err != nil {
			return fmt.Errorf("list gpu types (product=%s): %w", p, err)
		}
		if resp.JSON200 == nil {
			return fmt.Errorf("list gpu types (product=%s): %w", p, httpError(resp.StatusCode(), resp.Body))
		}
		gpuResps[i] = resp.JSON200
	}

	cpuResp, err := d.client.ListCpuTypesWithResponse(ctx, nil)
	if err != nil {
		return fmt.Errorf("list cpu types: %w", err)
	}
	if cpuResp.JSON200 == nil {
		return httpError(cpuResp.StatusCode(), cpuResp.Body)
	}

	metrics.CatalogGpuPriceDollarsPerHour.Reset()
	metrics.CatalogCpuPriceDollarsPerVcpuHour.Reset()
	metrics.CatalogGpuAvailability.Reset()

	// Price is context-independent (always present regardless of which
	// product was requested), so any one of the per-product responses has
	// it — the first (POD) is as good as any.
	for _, gpu := range gpuResps[0].Gpus {
		metrics.CatalogGpuPriceDollarsPerHour.WithLabelValues(gpu.Id, "SECURE").Set(float64(gpu.Price.Secure))
		metrics.CatalogGpuPriceDollarsPerHour.WithLabelValues(gpu.Id, "COMMUNITY").Set(float64(gpu.Price.Community))
	}

	for i, resp := range gpuResps {
		product := string(gpuAvailabilityProducts[i])
		for _, gpu := range resp.Gpus {
			if gpu.DataCenters == nil {
				continue
			}
			for _, dc := range *gpu.DataCenters {
				metrics.CatalogGpuAvailability.WithLabelValues(gpu.Id, dc.Id, product).Set(availabilityToFloat(dc.Availability))
			}
		}
	}

	for _, cpu := range cpuResp.JSON200.Cpus {
		// Casing matches the GPU price metric's cloud label (SECURE/COMMUNITY,
		// straight from the Cloud enum) for consistency, even though
		// "SERVERLESS" here isn't a Cloud enum member — CpuType.price has no
		// enum to source from, just field names (securePerVcpu/serverlessPerVcpu).
		metrics.CatalogCpuPriceDollarsPerVcpuHour.WithLabelValues(cpu.Id, "SECURE").Set(float64(cpu.Price.SecurePerVcpu))
		metrics.CatalogCpuPriceDollarsPerVcpuHour.WithLabelValues(cpu.Id, "SERVERLESS").Set(float64(cpu.Price.ServerlessPerVcpu))
	}

	return nil
}
