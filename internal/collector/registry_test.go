package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

// registryServer fakes both GET /v2/registries and GET /v2/registries/delegations
// so tests can control each independently.
type registryServer struct {
	mu                sync.Mutex
	registriesStatus  int
	registriesBody    string
	delegationsStatus int
	delegationsBody   string
}

func newRegistryServer() (*registryServer, *httptest.Server) {
	s := &registryServer{
		registriesStatus:  http.StatusOK,
		registriesBody:    `{"registries":[]}`,
		delegationsStatus: http.StatusOK,
		delegationsBody:   `{"delegations":[]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/registries/delegations" {
			w.WriteHeader(s.delegationsStatus)
			w.Write([]byte(s.delegationsBody))
			return
		}
		w.WriteHeader(s.registriesStatus)
		w.Write([]byte(s.registriesBody))
	}))
	return s, srv
}

const registriesFixture = `{"registries":[{"id":"reg-a","name":"my-private-registry"},{"id":"reg-b","name":"another-registry"}]}`
const delegationsFixture = `{"delegations":[{"id":"del-a","awsRegion":"us-east-2","awsUser":"123456789"}]}`

func TestRegistryDomain_Poll_Success(t *testing.T) {
	metrics.RegistryInfo.Reset()
	s, srv := newRegistryServer()
	defer srv.Close()
	s.registriesBody = registriesFixture
	s.delegationsBody = delegationsFixture

	d := NewRegistryDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if got := testutil.ToFloat64(metrics.RegistryInfo.WithLabelValues("reg-a", "my-private-registry")); got != 1 {
		t.Errorf("reg-a info = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.RegistryDelegationCount); got != 1 {
		t.Errorf("RegistryDelegationCount = %v, want 1", got)
	}
}

func TestRegistryDomain_Poll_PartialFailureStaleServesEverything(t *testing.T) {
	metrics.RegistryInfo.Reset()
	s, srv := newRegistryServer()
	defer srv.Close()
	d := NewRegistryDomain(testClient(t, srv.URL))

	s.registriesBody = registriesFixture
	s.delegationsBody = delegationsFixture
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	beforeInfo := testutil.ToFloat64(metrics.RegistryInfo.WithLabelValues("reg-a", "my-private-registry"))
	beforeCount := testutil.ToFloat64(metrics.RegistryDelegationCount)

	s.delegationsStatus = http.StatusInternalServerError
	s.delegationsBody = `{"error":"database is on fire"}`
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error when delegations call fails, got nil")
	}

	if after := testutil.ToFloat64(metrics.RegistryInfo.WithLabelValues("reg-a", "my-private-registry")); after != beforeInfo {
		t.Errorf("reg-a info changed despite delegations failure: before=%v after=%v", beforeInfo, after)
	}
	if after := testutil.ToFloat64(metrics.RegistryDelegationCount); after != beforeCount {
		t.Errorf("RegistryDelegationCount changed: before=%v after=%v", beforeCount, after)
	}
}
