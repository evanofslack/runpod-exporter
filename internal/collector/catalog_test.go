package collector

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

// catalogServer fakes GET /v2/catalog/gpus (dispatching on the `product`
// query param) and GET /v2/catalog/cpus, so tests can control each
// product-scoped GPU response and the CPU response independently.
type catalogServer struct {
	mu                 sync.Mutex
	gpuStatusByProduct map[string]int
	gpuBodyByProduct   map[string]string
	cpuStatus          int
	cpuBody            string
}

func newCatalogServer() (*catalogServer, *httptest.Server) {
	s := &catalogServer{
		gpuStatusByProduct: map[string]int{},
		gpuBodyByProduct:   map[string]string{},
		cpuStatus:          http.StatusOK,
		cpuBody:            `{"cpus":[]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/v2/catalog/cpus" {
			w.WriteHeader(s.cpuStatus)
			w.Write([]byte(s.cpuBody))
			return
		}

		product := r.URL.Query().Get("product")
		status := s.gpuStatusByProduct[product]
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		w.Write([]byte(s.gpuBodyByProduct[product]))
	}))
	return s, srv
}

func gpuFixture(availability string) string {
	return `{"gpus":[{"id":"gpu-a","name":"RTX 4090","price":{"secure":0.44,"community":0.31},` +
		`"dataCenters":[{"id":"US-TX-3","name":"US Texas 3","availability":"` + availability + `"}]}]}`
}

const cpuFixture = `{"cpus":[{"id":"cpu-a","name":"Compute-Optimized","price":{"securePerVcpu":0.04,"serverlessPerVcpu":0.03}}]}`

func resetCatalogMetrics() {
	metrics.CatalogGpuPriceDollarsPerHour.Reset()
	metrics.CatalogCpuPriceDollarsPerVcpuHour.Reset()
	metrics.CatalogGpuAvailability.Reset()
}

func TestCatalogDomain_Poll_Success(t *testing.T) {
	resetCatalogMetrics()
	s, srv := newCatalogServer()
	defer srv.Close()
	s.gpuBodyByProduct["POD"] = gpuFixture("HIGH")
	s.gpuBodyByProduct["SERVERLESS"] = gpuFixture("LOW")
	s.gpuBodyByProduct["CLUSTER"] = gpuFixture("NONE")
	s.cpuBody = cpuFixture

	d := NewCatalogDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	approxEqual := func(t *testing.T, got, want float64, what string) {
		t.Helper()
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("%s = %v, want ~%v", what, got, want)
		}
	}

	approxEqual(t, testutil.ToFloat64(metrics.CatalogGpuPriceDollarsPerHour.WithLabelValues("gpu-a", "SECURE")), 0.44, "gpu-a secure price")
	approxEqual(t, testutil.ToFloat64(metrics.CatalogGpuPriceDollarsPerHour.WithLabelValues("gpu-a", "COMMUNITY")), 0.31, "gpu-a community price")
	approxEqual(t, testutil.ToFloat64(metrics.CatalogCpuPriceDollarsPerVcpuHour.WithLabelValues("cpu-a", "SECURE")), 0.04, "cpu-a secure price")
	approxEqual(t, testutil.ToFloat64(metrics.CatalogCpuPriceDollarsPerVcpuHour.WithLabelValues("cpu-a", "SERVERLESS")), 0.03, "cpu-a serverless price")

	if got := testutil.ToFloat64(metrics.CatalogGpuAvailability.WithLabelValues("gpu-a", "US-TX-3", "POD")); got != 3 {
		t.Errorf("gpu-a POD availability = %v, want 3 (HIGH)", got)
	}
	if got := testutil.ToFloat64(metrics.CatalogGpuAvailability.WithLabelValues("gpu-a", "US-TX-3", "SERVERLESS")); got != 1 {
		t.Errorf("gpu-a SERVERLESS availability = %v, want 1 (LOW)", got)
	}
	if got := testutil.ToFloat64(metrics.CatalogGpuAvailability.WithLabelValues("gpu-a", "US-TX-3", "CLUSTER")); got != 0 {
		t.Errorf("gpu-a CLUSTER availability = %v, want 0 (NONE)", got)
	}
}

func TestCatalogDomain_Poll_PartialFailureStaleServesEverything(t *testing.T) {
	resetCatalogMetrics()
	s, srv := newCatalogServer()
	defer srv.Close()
	d := NewCatalogDomain(testClient(t, srv.URL))

	s.gpuBodyByProduct["POD"] = gpuFixture("HIGH")
	s.gpuBodyByProduct["SERVERLESS"] = gpuFixture("LOW")
	s.gpuBodyByProduct["CLUSTER"] = gpuFixture("NONE")
	s.cpuBody = cpuFixture
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.CatalogGpuAvailability.WithLabelValues("gpu-a", "US-TX-3", "POD"))

	// CLUSTER-scoped call fails this round; per the all-or-nothing design,
	// nothing should update, not even POD/SERVERLESS which were fine.
	s.gpuStatusByProduct["CLUSTER"] = http.StatusInternalServerError
	s.gpuBodyByProduct["CLUSTER"] = `{"error":"database is on fire"}`

	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error when one product-scoped call fails, got nil")
	}
	if after := testutil.ToFloat64(metrics.CatalogGpuAvailability.WithLabelValues("gpu-a", "US-TX-3", "POD")); after != before {
		t.Errorf("POD availability changed despite CLUSTER failure: before=%v after=%v", before, after)
	}
}
