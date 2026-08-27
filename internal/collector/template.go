package collector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type TemplateDomain struct {
	client *openapi.ClientWithResponses
}

func NewTemplateDomain(client *openapi.ClientWithResponses) *TemplateDomain {
	return &TemplateDomain{client: client}
}

func (d *TemplateDomain) Name() string { return "template" }
func (d *TemplateDomain) Tier() Tier   { return Slow }

func (d *TemplateDomain) Poll(ctx context.Context) error {
	resp, err := d.client.ListTemplatesWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list templates: %w", err)
	}
	if resp.JSON200 == nil {
		return httpError(resp.StatusCode(), resp.Body)
	}

	metrics.TemplateInfo.Reset()

	for _, t := range resp.JSON200.Templates {
		metrics.TemplateInfo.WithLabelValues(
			t.Id, t.Name, strconv.FormatBool(t.Serverless), strconv.FormatBool(t.Public), string(t.Category),
		).Set(1)
	}

	return nil
}
