package collector

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

const clustersFixture = `{
  "clusters": [
    {"id": "cl-a", "name": "Cluster A", "type": "TRAINING", "dataCenterId": "US-TX-3",
     "pods": {"total": 4, "byStatus": {"RUNNING": 3, "PROVISIONING": 1}}},
    {"id": "cl-b", "name": "Cluster B", "type": "SLURM",
     "pods": {"total": 0, "byStatus": {}}}
  ]
}`

func TestClusterDomain_Poll_Success(t *testing.T) {
	metrics.ClusterPods.Reset()
	metrics.ClusterInfo.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	fs.set(http.StatusOK, clustersFixture)

	d := NewClusterDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if got := testutil.ToFloat64(metrics.ClusterInfo.WithLabelValues("cl-a", "TRAINING", "US-TX-3")); got != 1 {
		t.Errorf("cl-a info = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ClusterInfo.WithLabelValues("cl-b", "SLURM", "")); got != 1 {
		t.Errorf("cl-b info (no data center yet) = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.ClusterPods.WithLabelValues("cl-a", "RUNNING")); got != 3 {
		t.Errorf("cl-a RUNNING pods = %v, want 3", got)
	}
	if got := testutil.ToFloat64(metrics.ClusterPods.WithLabelValues("cl-a", "PROVISIONING")); got != 1 {
		t.Errorf("cl-a PROVISIONING pods = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(metrics.ClusterPods); n != 2 {
		t.Errorf("ClusterPods series count = %d, want 2 (cl-b has no statuses)", n)
	}
}

func TestClusterDomain_Poll_HTTPErrorStaleServes(t *testing.T) {
	metrics.ClusterPods.Reset()
	metrics.ClusterInfo.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	d := NewClusterDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, clustersFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.ClusterPods.WithLabelValues("cl-a", "RUNNING"))

	fs.set(http.StatusInternalServerError, `{"error":"database is on fire"}`)
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error on 500, got nil")
	}
	if after := testutil.ToFloat64(metrics.ClusterPods.WithLabelValues("cl-a", "RUNNING")); after != before {
		t.Errorf("cl-a RUNNING pods changed on a failed poll: before=%v after=%v", before, after)
	}
}
