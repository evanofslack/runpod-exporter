package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type ClusterDomain struct {
	client *openapi.ClientWithResponses
}

func NewClusterDomain(client *openapi.ClientWithResponses) *ClusterDomain {
	return &ClusterDomain{client: client}
}

func (d *ClusterDomain) Name() string { return "cluster" }
func (d *ClusterDomain) Tier() Tier   { return Fast }

func (d *ClusterDomain) Poll(ctx context.Context) error {
	resp, err := d.client.ListClustersWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list clusters: %w", err)
	}
	if resp.JSON200 == nil {
		return httpError(resp.StatusCode(), resp.Body)
	}

	metrics.ClusterPods.Reset()
	metrics.ClusterInfo.Reset()

	for _, c := range resp.JSON200.Clusters {
		dataCenterID := ""
		if c.DataCenterId != nil {
			dataCenterID = *c.DataCenterId
		}
		metrics.ClusterInfo.WithLabelValues(c.Id, string(c.Type), dataCenterID).Set(1)

		for status, count := range c.Pods.ByStatus {
			metrics.ClusterPods.WithLabelValues(c.Id, status).Set(float64(count))
		}
	}

	return nil
}
