package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type NetworkVolumeDomain struct {
	client *openapi.ClientWithResponses
}

func NewNetworkVolumeDomain(client *openapi.ClientWithResponses) *NetworkVolumeDomain {
	return &NetworkVolumeDomain{client: client}
}

func (d *NetworkVolumeDomain) Name() string { return "network-volume" }
func (d *NetworkVolumeDomain) Tier() Tier   { return Slow }

func (d *NetworkVolumeDomain) Poll(ctx context.Context) error {
	resp, err := d.client.ListNetworkVolumesWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list network volumes: %w", err)
	}
	if resp.JSON200 == nil {
		return httpError(resp.StatusCode(), resp.Body)
	}

	metrics.NetworkVolumeSizeGB.Reset()

	for _, v := range resp.JSON200.NetworkVolumes {
		metrics.NetworkVolumeSizeGB.WithLabelValues(v.Id, v.DataCenter, string(v.Type)).Set(float64(v.Size))
	}

	return nil
}
