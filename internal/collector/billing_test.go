package collector

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

const billingFixture = `{
  "records": [
    {
      "startTime": "2026-06-01T00:00:00Z",
      "endTime": "2026-06-01T01:00:00Z",
      "totalAmount": 42.34,
      "podGpuAmount": 18.2,
      "podCpuAmount": 0,
      "podDiskAmount": 1.1,
      "serverlessGpuAmount": 12.6,
      "serverlessCpuAmount": 0,
      "serverlessDiskAmount": 0.44,
      "serverlessFeeAmount": 1.25,
      "storageStandardAmount": 0,
      "storageHighPerformanceAmount": 2.5,
      "endpointAmount": 3.21,
      "clusterGpuAmount": 2.5,
      "clusterDiskAmount": 0.3,
      "clusterNetworkingAmount": 0.24
    }
  ],
  "metadata": {
    "query": {"startTime": "2026-06-01T00:00:00Z", "endTime": "2026-06-01T01:00:00Z", "bucketSize": "hour"},
    "recordCount": 1,
    "totals": {
      "totalAmount": 42.34, "podGpuAmount": 18.2, "podCpuAmount": 0, "podDiskAmount": 1.1,
      "serverlessGpuAmount": 12.6, "serverlessCpuAmount": 0, "serverlessDiskAmount": 0.44,
      "serverlessFeeAmount": 1.25, "storageStandardAmount": 0, "storageHighPerformanceAmount": 2.5,
      "endpointAmount": 3.21, "clusterGpuAmount": 2.5, "clusterDiskAmount": 0.3, "clusterNetworkingAmount": 0.24
    }
  }
}`

func TestBillingDomain_Poll_Success(t *testing.T) {
	metrics.BillingCostDollars.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	fs.set(http.StatusOK, billingFixture)

	d := NewBillingDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	want := map[string]float64{
		"pod_gpu":                  18.2,
		"pod_cpu":                  0,
		"pod_disk":                 1.1,
		"serverless_gpu":           12.6,
		"serverless_cpu":           0,
		"serverless_disk":          0.44,
		"serverless_fee":           1.25,
		"storage_standard":         0,
		"storage_high_performance": 2.5,
		"endpoint":                 3.21,
		"cluster_gpu":              2.5,
		"cluster_disk":             0.3,
		"cluster_networking":       0.24,
	}
	for resource, wantVal := range want {
		if got := testutil.ToFloat64(metrics.BillingCostDollars.WithLabelValues(resource)); got != wantVal {
			t.Errorf("resource %q = %v, want %v", resource, got, wantVal)
		}
	}
	if n := testutil.CollectAndCount(metrics.BillingCostDollars); n != len(want) {
		t.Errorf("BillingCostDollars series count = %d, want %d", n, len(want))
	}
}

func TestBillingDomain_Poll_HTTPErrorStaleServes(t *testing.T) {
	metrics.BillingCostDollars.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	d := NewBillingDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, billingFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.BillingCostDollars.WithLabelValues("pod_gpu"))

	fs.set(http.StatusInternalServerError, `{"error":"database is on fire"}`)
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error on 500, got nil")
	}

	if after := testutil.ToFloat64(metrics.BillingCostDollars.WithLabelValues("pod_gpu")); after != before {
		t.Errorf("pod_gpu changed on a failed poll: before=%v after=%v", before, after)
	}
}
