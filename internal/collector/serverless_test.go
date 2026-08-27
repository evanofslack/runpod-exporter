package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

// serverlessServer fakes GET /v2/serverless plus a per-endpoint
// GET /v2/serverless/{id}/workers, so tests can control the list response and
// each endpoint's worker response (including failing just one) independently.
type serverlessServer struct {
	mu            sync.Mutex
	listStatus    int
	listBody      string
	workersStatus map[string]int
	workersBody   map[string]string
}

func newServerlessServer() (*serverlessServer, *httptest.Server) {
	s := &serverlessServer{
		listStatus:    http.StatusOK,
		listBody:      `{"endpoints":[]}`,
		workersStatus: map[string]int{},
		workersBody:   map[string]string{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/v2/serverless" {
			w.WriteHeader(s.listStatus)
			w.Write([]byte(s.listBody))
			return
		}

		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v2/serverless/"), "/workers")
		status := s.workersStatus[id]
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		w.Write([]byte(s.workersBody[id]))
	}))
	return s, srv
}

const twoEndpointsFixture = `{
  "endpoints": [
    {"id": "ep-a", "name": "Endpoint A", "type": "QUEUE", "flashboot": "FLASHBOOT", "workers": {"min": 0, "max": 5}},
    {"id": "ep-b", "name": "Endpoint B", "type": "LOAD_BALANCER", "flashboot": "OFF", "workers": {"min": 1, "max": 3}}
  ]
}`

const epAWorkersFixture = `{
  "summary": {"running": 2, "idle": 1, "initializing": 0, "throttled": 0, "unhealthy": 0, "total": 3},
  "workers": [
    {"id": "w1", "status": "RUNNING", "gpuCount": 1, "isStale": false},
    {"id": "w2", "status": "RUNNING", "gpuCount": 1, "isStale": true},
    {"id": "w3", "status": "IDLE", "gpuCount": 1, "isStale": false}
  ]
}`

const epBWorkersFixture = `{
  "summary": {"running": 0, "idle": 0, "initializing": 1, "throttled": 0, "unhealthy": 0, "total": 1},
  "workers": [
    {"id": "w4", "status": "INITIALIZING", "gpuCount": 1, "isStale": false}
  ]
}`

func resetServerlessMetrics() {
	metrics.ServerlessWorkers.Reset()
	metrics.ServerlessWorkerStale.Reset()
	metrics.ServerlessWorkersMin.Reset()
	metrics.ServerlessWorkersMax.Reset()
	metrics.ServerlessInfo.Reset()
}

func TestServerlessDomain_Poll_Success(t *testing.T) {
	resetServerlessMetrics()
	s, srv := newServerlessServer()
	defer srv.Close()
	s.listBody = twoEndpointsFixture
	s.workersBody["ep-a"] = epAWorkersFixture
	s.workersBody["ep-b"] = epBWorkersFixture

	d := NewServerlessDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if got := testutil.ToFloat64(metrics.ServerlessInfo.WithLabelValues("ep-a", "Endpoint A", "QUEUE", "FLASHBOOT")); got != 1 {
		t.Errorf("ep-a info = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessInfo.WithLabelValues("ep-b", "Endpoint B", "LOAD_BALANCER", "OFF")); got != 1 {
		t.Errorf("ep-b info = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkersMin.WithLabelValues("ep-a")); got != 0 {
		t.Errorf("ep-a workers min = %v, want 0", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkersMax.WithLabelValues("ep-a")); got != 5 {
		t.Errorf("ep-a workers max = %v, want 5", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-a", "running")); got != 2 {
		t.Errorf("ep-a running workers = %v, want 2", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-a", "idle")); got != 1 {
		t.Errorf("ep-a idle workers = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkerStale.WithLabelValues("ep-a")); got != 1 {
		t.Errorf("ep-a stale workers = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-b", "initializing")); got != 1 {
		t.Errorf("ep-b initializing workers = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ServerlessWorkerStale.WithLabelValues("ep-b")); got != 0 {
		t.Errorf("ep-b stale workers = %v, want 0", got)
	}
}

func TestServerlessDomain_Poll_PartialFailureStaleServesEverything(t *testing.T) {
	resetServerlessMetrics()
	s, srv := newServerlessServer()
	defer srv.Close()
	d := NewServerlessDomain(testClient(t, srv.URL))

	// First poll succeeds fully, establishing known-good values.
	s.listBody = twoEndpointsFixture
	s.workersBody["ep-a"] = epAWorkersFixture
	s.workersBody["ep-b"] = epBWorkersFixture
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	beforeA := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-a", "running"))
	beforeB := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-b", "initializing"))

	// Second poll: the list still succeeds, but ep-b's workers call fails.
	// Per the all-or-nothing design, NEITHER endpoint's data should update —
	// not even ep-a's, which was otherwise fine this round.
	s.workersStatus["ep-b"] = http.StatusInternalServerError
	s.workersBody["ep-b"] = `{"error":"database is on fire"}`

	err := d.Poll(context.Background())
	if err == nil {
		t.Fatal("Poll: want error when one endpoint's workers call fails, got nil")
	}

	if after := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-a", "running")); after != beforeA {
		t.Errorf("ep-a running workers changed despite ep-b's failure: before=%v after=%v", beforeA, after)
	}
	if after := testutil.ToFloat64(metrics.ServerlessWorkers.WithLabelValues("ep-b", "initializing")); after != beforeB {
		t.Errorf("ep-b initializing workers changed: before=%v after=%v", beforeB, after)
	}
}

func TestServerlessDomain_Poll_ListErrorStaleServes(t *testing.T) {
	resetServerlessMetrics()
	s, srv := newServerlessServer()
	defer srv.Close()
	d := NewServerlessDomain(testClient(t, srv.URL))

	s.listBody = twoEndpointsFixture
	s.workersBody["ep-a"] = epAWorkersFixture
	s.workersBody["ep-b"] = epBWorkersFixture
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.ServerlessInfo.WithLabelValues("ep-a", "Endpoint A", "QUEUE", "FLASHBOOT"))

	s.listStatus = http.StatusTooManyRequests
	s.listBody = `{"error":"slow down"}`
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error on 429, got nil")
	}

	if after := testutil.ToFloat64(metrics.ServerlessInfo.WithLabelValues("ep-a", "Endpoint A", "QUEUE", "FLASHBOOT")); after != before {
		t.Errorf("ep-a info changed on a failed poll: before=%v after=%v", before, after)
	}
}
