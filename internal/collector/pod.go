package collector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type PodDomain struct {
	client *openapi.ClientWithResponses
}

func NewPodDomain(client *openapi.ClientWithResponses) *PodDomain {
	return &PodDomain{client: client}
}

func (d *PodDomain) Name() string { return "pod" }
func (d *PodDomain) Tier() Tier   { return Fast }

func (d *PodDomain) Poll(ctx context.Context) error {
	resp, err := d.client.ListPodsWithResponse(ctx, nil)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}
	if resp.JSON200 == nil {
		return httpError(resp.StatusCode(), resp.Body)
	}

	metrics.PodUp.Reset()
	metrics.PodCPUUtilPercent.Reset()
	metrics.PodMemoryUtilPercent.Reset()
	metrics.PodGPUUtilPercent.Reset()
	metrics.PodGPUMemoryUtilPercent.Reset()
	metrics.PodUptimeSeconds.Reset()
	metrics.PodCostPerHourDollars.Reset()
	metrics.PodDiskGB.Reset()
	metrics.PodInfo.Reset()

	for _, pod := range resp.JSON200.Pods {
		up := 0.0
		if pod.Status == openapi.RUNNING {
			up = 1.0
		}
		metrics.PodUp.WithLabelValues(pod.Id, pod.Name, string(pod.Status)).Set(up)
		metrics.PodCostPerHourDollars.WithLabelValues(pod.Id).Set(float64(pod.Cost))
		metrics.PodDiskGB.WithLabelValues(pod.Id).Set(float64(pod.Disk))

		dataCenterID := ""
		if pod.DataCenterId != nil {
			dataCenterID = *pod.DataCenterId
		}
		gpuID := ""
		if pod.Gpu != nil {
			gpuID = pod.Gpu.Id
		}
		metrics.PodInfo.WithLabelValues(pod.Id, pod.Image, dataCenterID, string(pod.Cloud), gpuID).Set(1)

		if pod.Runtime == nil {
			continue
		}
		rt := pod.Runtime
		if rt.Uptime != nil {
			metrics.PodUptimeSeconds.WithLabelValues(pod.Id).Set(float64(*rt.Uptime))
		}
		if rt.Cpu != nil && rt.Cpu.Util != nil {
			metrics.PodCPUUtilPercent.WithLabelValues(pod.Id).Set(float64(*rt.Cpu.Util))
		}
		if rt.Memory != nil && rt.Memory.Util != nil {
			metrics.PodMemoryUtilPercent.WithLabelValues(pod.Id).Set(float64(*rt.Memory.Util))
		}
		if rt.Gpus != nil {
			for i, gpu := range *rt.Gpus {
				idx := strconv.Itoa(i)
				if gpu.Util != nil {
					metrics.PodGPUUtilPercent.WithLabelValues(pod.Id, idx).Set(float64(*gpu.Util))
				}
				if gpu.MemoryUtil != nil {
					metrics.PodGPUMemoryUtilPercent.WithLabelValues(pod.Id, idx).Set(float64(*gpu.MemoryUtil))
				}
			}
		}
	}

	return nil
}
