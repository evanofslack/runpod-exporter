package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type AccountDomain struct {
	client *openapi.ClientWithResponses
}

func NewAccountDomain(client *openapi.ClientWithResponses) *AccountDomain {
	return &AccountDomain{client: client}
}

func (d *AccountDomain) Name() string { return "account" }
func (d *AccountDomain) Tier() Tier   { return Fast }

func (d *AccountDomain) Poll(ctx context.Context) error {
	resp, err := d.client.GetSshKeysWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("get ssh keys: %w", err)
	}
	if resp.JSON200 == nil {
		return httpError(resp.StatusCode(), resp.Body)
	}

	metrics.AccountSSHKeys.Set(float64(len(resp.JSON200.Keys)))
	return nil
}
