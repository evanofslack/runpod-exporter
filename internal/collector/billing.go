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

	// metadata.totals, not records[-1]: totals is the same BillingAmounts
	// shape and is unconditionally present, correctly reporting all-zero
	// spend when the current hour has zero records instead of reporting
	// nothing at all. When exactly one record exists (the normal lastN=1
	// case), totals is that record's own amounts (sum of one item), so this
	// loses nothing in the common case either.
	totals := resp.JSON200.Metadata.Totals

	metrics.BillingCostDollars.WithLabelValues("pod_gpu").Set(totals.PodGpuAmount)
	metrics.BillingCostDollars.WithLabelValues("pod_cpu").Set(totals.PodCpuAmount)
	metrics.BillingCostDollars.WithLabelValues("pod_disk").Set(totals.PodDiskAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_gpu").Set(totals.ServerlessGpuAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_cpu").Set(totals.ServerlessCpuAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_disk").Set(totals.ServerlessDiskAmount)
	metrics.BillingCostDollars.WithLabelValues("serverless_fee").Set(totals.ServerlessFeeAmount)
	metrics.BillingCostDollars.WithLabelValues("storage_standard").Set(totals.StorageStandardAmount)
	metrics.BillingCostDollars.WithLabelValues("storage_high_performance").Set(totals.StorageHighPerformanceAmount)
	metrics.BillingCostDollars.WithLabelValues("endpoint").Set(totals.EndpointAmount)
	metrics.BillingCostDollars.WithLabelValues("cluster_gpu").Set(totals.ClusterGpuAmount)
	metrics.BillingCostDollars.WithLabelValues("cluster_disk").Set(totals.ClusterDiskAmount)
	metrics.BillingCostDollars.WithLabelValues("cluster_networking").Set(totals.ClusterNetworkingAmount)

	return nil
}
