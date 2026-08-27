package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type BillingDomain struct {
	client *openapi.ClientWithResponses
}

func NewBillingDomain(client *openapi.ClientWithResponses) *BillingDomain {
	return &BillingDomain{client: client}
}

func (d *BillingDomain) Name() string { return "billing" }
func (d *BillingDomain) Tier() Tier   { return Slow }

func (d *BillingDomain) Poll(ctx context.Context) error {
	bucketSize := openapi.BillingBucketSizeHour
	lastN := 1
	params := &openapi.ListBillingParams{BucketSize: &bucketSize, LastN: &lastN}

	resp, err := d.client.ListBillingWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("list billing: %w", err)
	}
	if resp.JSON200 == nil {
		return httpError(resp.StatusCode(), resp.Body)
	}

	metrics.BillingCostDollars.Reset()

	records := resp.JSON200.Records
	if len(records) == 0 {
		return nil
	}
	latest := records[len(records)-1]

	metrics.BillingCostDollars.WithLabelValues("pod_gpu").Set(latest.PodGpuAmount)
	metrics.BillingCostDollars.WithLabelValues("pod_cpu").Set(latest.PodCpuAmount)
	metrics.BillingCostDollars.WithLabelValues("pod_disk").Set(latest.PodDiskAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_gpu").Set(latest.ServerlessGpuAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_cpu").Set(latest.ServerlessCpuAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_disk").Set(latest.ServerlessDiskAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_fee").Set(latest.ServerlessFeeAmount)
	metrics.BillingCostDollars.WithLabelValues("storage_standard").Set(latest.StorageStandardAmount)
	metrics.BillingCostDollars.WithLabelValues("storage_high_performance").Set(latest.StorageHighPerformanceAmount)
	metrics.BillingCostDollars.WithLabelValues("endpoint").Set(latest.EndpointAmount)
	metrics.BillingCostDollars.WithLabelValues("cluster_gpu").Set(latest.ClusterGpuAmount)
	metrics.BillingCostDollars.WithLabelValues("cluster_disk").Set(latest.ClusterDiskAmount)
	metrics.BillingCostDollars.WithLabelValues("cluster_networking").Set(latest.ClusterNetworkingAmount)

	return nil
}
