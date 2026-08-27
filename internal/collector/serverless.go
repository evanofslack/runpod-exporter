package collector

import (
	"context"
	"fmt"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type ServerlessDomain struct {
	client *openapi.ClientWithResponses
}

func NewServerlessDomain(client *openapi.ClientWithResponses) *ServerlessDomain {
	return &ServerlessDomain{client: client}
}

func (d *ServerlessDomain) Name() string { return "serverless" }
func (d *ServerlessDomain) Tier() Tier   { return Fast }

// Poll fetches every endpoint's workers before touching any metric vec: a
// single failed sub-call fails the whole poll (stale-serve for everything),
// rather than mixing fresh and stale data within one metric family.
func (d *ServerlessDomain) Poll(ctx context.Context) error {
	listResp, err := d.client.ListEndpointsWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list endpoints: %w", err)
	}
	if listResp.JSON200 == nil {
		return httpError(listResp.StatusCode(), listResp.Body)
	}
	endpoints := listResp.JSON200.Endpoints

	workerResps := make([]*openapi.ListEndpointWorkersResponse, len(endpoints))
	for i, ep := range endpoints {
		wResp, err := d.client.ListEndpointWorkersWithResponse(ctx, ep.Id)
		if err != nil {
			return fmt.Errorf("list workers for endpoint %s: %w", ep.Id, err)
		}
		if wResp.JSON200 == nil {
			return fmt.Errorf("list workers for endpoint %s: %w", ep.Id, httpError(wResp.StatusCode(), wResp.Body))
		}
		workerResps[i] = wResp.JSON200
	}

	metrics.ServerlessWorkers.Reset()
	metrics.ServerlessWorkerStale.Reset()
	metrics.ServerlessWorkersMin.Reset()
	metrics.ServerlessWorkersMax.Reset()
	metrics.ServerlessInfo.Reset()

	for i, ep := range endpoints {
		epType := ""
		if ep.Type != nil {
			epType = string(*ep.Type)
		}
		metrics.ServerlessInfo.WithLabelValues(ep.Id, ep.Name, epType, string(ep.Flashboot)).Set(1)
		metrics.ServerlessWorkersMin.WithLabelValues(ep.Id).Set(float64(ep.Workers.Min))
		metrics.ServerlessWorkersMax.WithLabelValues(ep.Id).Set(float64(ep.Workers.Max))

		summary := workerResps[i].Summary
		metrics.ServerlessWorkers.WithLabelValues(ep.Id, "running").Set(float64(summary.Running))
		metrics.ServerlessWorkers.WithLabelValues(ep.Id, "idle").Set(float64(summary.Idle))
		metrics.ServerlessWorkers.WithLabelValues(ep.Id, "initializing").Set(float64(summary.Initializing))
		metrics.ServerlessWorkers.WithLabelValues(ep.Id, "throttled").Set(float64(summary.Throttled))
		metrics.ServerlessWorkers.WithLabelValues(ep.Id, "unhealthy").Set(float64(summary.Unhealthy))

		stale := 0
		for _, w := range workerResps[i].Workers {
			if w.IsStale {
				stale++
			}
		}
		metrics.ServerlessWorkerStale.WithLabelValues(ep.Id).Set(float64(stale))
	}

	return nil
}
