package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type RegistryDomain struct {
	client *openapi.ClientWithResponses
}

func NewRegistryDomain(client *openapi.ClientWithResponses) *RegistryDomain {
	return &RegistryDomain{client: client}
}

func (d *RegistryDomain) Name() string { return "registry" }
func (d *RegistryDomain) Tier() Tier   { return Slow }

// Poll fetches both calls before touching any metric vec — all-or-nothing,
// same as every other multi-call domain (see the serverless domain for why).
func (d *RegistryDomain) Poll(ctx context.Context) error {
	regResp, err := d.client.ListRegistriesWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list registries: %w", err)
	}
	if regResp.JSON200 == nil {
		return httpError(regResp.StatusCode(), regResp.Body)
	}

	delResp, err := d.client.ListDelegationsWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list delegations: %w", err)
	}
	if delResp.JSON200 == nil {
		return httpError(delResp.StatusCode(), delResp.Body)
	}

	metrics.RegistryInfo.Reset()
	for _, r := range regResp.JSON200.Registries {
		metrics.RegistryInfo.WithLabelValues(r.Id, r.Name).Set(1)
	}
	metrics.RegistryDelegationCount.Set(float64(len(delResp.JSON200.Delegations)))

	return nil
}
