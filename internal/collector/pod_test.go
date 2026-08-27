package collector

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
	"github.com/evanofslack/runpod-exporter/openapi"
)

type fixtureServer struct {
	mu     sync.Mutex
	status int
	body   string
}

func newFixtureServer() (*fixtureServer, *httptest.Server) {
	fs := &fixtureServer{status: http.StatusOK, body: `{"pods":[]}`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(fs.status)
		w.Write([]byte(fs.body))
	}))
	return fs, srv
}

func (fs *fixtureServer) set(status int, body string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.status = status
	fs.body = body
}

func testClient(t *testing.T, serverURL string) *openapi.ClientWithResponses {
	t.Helper()
	client, err := openapi.NewClientWithResponses(serverURL)
	if err != nil {
		t.Fatalf("NewClientWithResponses: %v", err)
	}
	return client
}

func resetPodMetrics() {
	metrics.PodUp.Reset()
	metrics.PodCPUUtilPercent.Reset()
	metrics.PodMemoryUtilPercent.Reset()
	metrics.PodGPUUtilPercent.Reset()
	metrics.PodGPUMemoryUtilPercent.Reset()
	metrics.PodUptimeSeconds.Reset()
	metrics.PodCostPerHourDollars.Reset()
	metrics.PodDiskGB.Reset()
	metrics.PodInfo.Reset()
}

const twoPodsFixture = `{
  "pods": [
    {
      "id": "pod-a",
      "name": "Pod A",
      "status": "RUNNING",
      "image": "runpod/pytorch:1.0",
      "disk": 50,
      "cost": 0.44,
      "cloud": "SECURE",
      "dataCenterId": "US-TX-3",
      "gpu": {"id": "NVIDIA GeForce RTX 4090", "count": 2},
      "runtime": {
        "uptime": 3600,
        "cpu": {"util": 55},
        "memory": {"util": 40},
        "gpus": [
          {"util": 90, "memoryUtil": 70},
          {"util": 80, "memoryUtil": 60}
        ]
      }
    },
    {
      "id": "pod-b",
      "name": "Pod B",
      "status": "EXITED",
      "image": "runpod/base:1.0",
      "disk": 20,
      "cost": 0,
      "cloud": "COMMUNITY",
      "dataCenterId": "US-KS-2",
      "runtime": null
    }
  ]
}`

const onePodFixture = `{
  "pods": [
    {
      "id": "pod-a",
      "name": "Pod A",
      "status": "RUNNING",
      "image": "runpod/pytorch:1.0",
      "disk": 50,
      "cost": 0.44,
      "cloud": "SECURE",
      "dataCenterId": "US-TX-3",
      "runtime": null
    }
  ]
}`

func TestPodDomain_Poll_Success(t *testing.T) {
	resetPodMetrics()
	fs, srv := newFixtureServer()
	defer srv.Close()
	fs.set(http.StatusOK, twoPodsFixture)

	d := NewPodDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if got := testutil.ToFloat64(metrics.PodUp.WithLabelValues("pod-a", "Pod A", "RUNNING")); got != 1 {
		t.Errorf("pod-a up = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.PodUp.WithLabelValues("pod-b", "Pod B", "EXITED")); got != 0 {
		t.Errorf("pod-b up = %v, want 0", got)
	}
	// pod.Cost is float32 on the wire; comparing against a float64 literal
	// needs an epsilon for the widening rounding error.
	if got := testutil.ToFloat64(metrics.PodCostPerHourDollars.WithLabelValues("pod-a")); math.Abs(got-0.44) > 1e-6 {
		t.Errorf("pod-a cost = %v, want ~0.44", got)
	}
	if got := testutil.ToFloat64(metrics.PodDiskGB.WithLabelValues("pod-b")); got != 20 {
		t.Errorf("pod-b disk = %v, want 20", got)
	}
	if got := testutil.ToFloat64(metrics.PodInfo.WithLabelValues("pod-a", "runpod/pytorch:1.0", "US-TX-3", "SECURE", "NVIDIA GeForce RTX 4090")); got != 1 {
		t.Errorf("pod-a info = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.PodInfo.WithLabelValues("pod-b", "runpod/base:1.0", "US-KS-2", "COMMUNITY", "")); got != 1 {
		t.Errorf("pod-b info = %v, want 1", got)
	}

	// pod-a's runtime is present: utilization series exist with the right values.
	if got := testutil.ToFloat64(metrics.PodUptimeSeconds.WithLabelValues("pod-a")); got != 3600 {
		t.Errorf("pod-a uptime = %v, want 3600", got)
	}
	if got := testutil.ToFloat64(metrics.PodCPUUtilPercent.WithLabelValues("pod-a")); got != 55 {
		t.Errorf("pod-a cpu util = %v, want 55", got)
	}
	if got := testutil.ToFloat64(metrics.PodMemoryUtilPercent.WithLabelValues("pod-a")); got != 40 {
		t.Errorf("pod-a memory util = %v, want 40", got)
	}
	if got := testutil.ToFloat64(metrics.PodGPUUtilPercent.WithLabelValues("pod-a", "0")); got != 90 {
		t.Errorf("pod-a gpu0 util = %v, want 90", got)
	}
	if got := testutil.ToFloat64(metrics.PodGPUMemoryUtilPercent.WithLabelValues("pod-a", "1")); got != 60 {
		t.Errorf("pod-a gpu1 memory util = %v, want 60", got)
	}

	// pod-b's runtime is null: no utilization series for it at all.
	if n := testutil.CollectAndCount(metrics.PodUptimeSeconds); n != 1 {
		t.Errorf("PodUptimeSeconds series count = %d, want 1", n)
	}
	if n := testutil.CollectAndCount(metrics.PodCPUUtilPercent); n != 1 {
		t.Errorf("PodCPUUtilPercent series count = %d, want 1", n)
	}
	if n := testutil.CollectAndCount(metrics.PodGPUUtilPercent); n != 2 {
		t.Errorf("PodGPUUtilPercent series count = %d, want 2", n)
	}
}

func TestPodDomain_Poll_VanishedResource(t *testing.T) {
	resetPodMetrics()
	fs, srv := newFixtureServer()
	defer srv.Close()

	d := NewPodDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, twoPodsFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	if n := testutil.CollectAndCount(metrics.PodUp); n != 2 {
		t.Fatalf("PodUp series count after first poll = %d, want 2", n)
	}

	fs.set(http.StatusOK, onePodFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("second Poll: %v", err)
	}
	if n := testutil.CollectAndCount(metrics.PodUp); n != 1 {
		t.Errorf("PodUp series count after second poll = %d, want 1 (pod-b should be gone)", n)
	}
	if n := testutil.CollectAndCount(metrics.PodInfo); n != 1 {
		t.Errorf("PodInfo series count after second poll = %d, want 1", n)
	}
}

func TestPodDomain_Poll_HTTPErrorStaleServes(t *testing.T) {
	resetPodMetrics()
	fs, srv := newFixtureServer()
	defer srv.Close()

	d := NewPodDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, onePodFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.PodUp.WithLabelValues("pod-a", "Pod A", "RUNNING"))

	fs.set(http.StatusInternalServerError, `{"error":"database is on fire"}`)
	err := d.Poll(context.Background())
	if err == nil {
		t.Fatal("Poll: want error on 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "database is on fire") {
		t.Errorf("Poll error = %q, want it to mention status 500 and the body", err.Error())
	}

	after := testutil.ToFloat64(metrics.PodUp.WithLabelValues("pod-a", "Pod A", "RUNNING"))
	if after != before {
		t.Errorf("PodUp changed on a failed poll: before=%v after=%v (should stale-serve)", before, after)
	}
}
