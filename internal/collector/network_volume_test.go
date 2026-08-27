package collector

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

const networkVolumesFixture = `{
  "networkVolumes": [
    {"id": "vol-a", "name": "dataset-a", "size": 50, "dataCenter": "EU-RO-1", "type": "STANDARD"},
    {"id": "vol-b", "name": "dataset-b", "size": 500, "dataCenter": "US-TX-3", "type": "HIGH_PERFORMANCE"}
  ]
}`

func TestNetworkVolumeDomain_Poll_Success(t *testing.T) {
	metrics.NetworkVolumeSizeGB.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	fs.set(http.StatusOK, networkVolumesFixture)

	d := NewNetworkVolumeDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if got := testutil.ToFloat64(metrics.NetworkVolumeSizeGB.WithLabelValues("vol-a", "EU-RO-1", "STANDARD")); got != 50 {
		t.Errorf("vol-a size = %v, want 50", got)
	}
	if got := testutil.ToFloat64(metrics.NetworkVolumeSizeGB.WithLabelValues("vol-b", "US-TX-3", "HIGH_PERFORMANCE")); got != 500 {
		t.Errorf("vol-b size = %v, want 500", got)
	}
}

func TestNetworkVolumeDomain_Poll_HTTPErrorStaleServes(t *testing.T) {
	metrics.NetworkVolumeSizeGB.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	d := NewNetworkVolumeDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, networkVolumesFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.NetworkVolumeSizeGB.WithLabelValues("vol-a", "EU-RO-1", "STANDARD"))

	fs.set(http.StatusTooManyRequests, `{"error":"slow down"}`)
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error on 429, got nil")
	}
	if after := testutil.ToFloat64(metrics.NetworkVolumeSizeGB.WithLabelValues("vol-a", "EU-RO-1", "STANDARD")); after != before {
		t.Errorf("vol-a size changed on a failed poll: before=%v after=%v", before, after)
	}
}
