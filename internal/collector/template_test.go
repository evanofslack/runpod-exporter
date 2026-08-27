package collector

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

const templatesFixture = `{
  "templates": [
    {"id": "tpl-a", "name": "PyTorch", "serverless": false, "public": true, "category": "NVIDIA"},
    {"id": "tpl-b", "name": "CPU worker", "serverless": true, "public": false, "category": "CPU"}
  ]
}`

func TestTemplateDomain_Poll_Success(t *testing.T) {
	metrics.TemplateInfo.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	fs.set(http.StatusOK, templatesFixture)

	d := NewTemplateDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}

	if got := testutil.ToFloat64(metrics.TemplateInfo.WithLabelValues("tpl-a", "PyTorch", "false", "true", "NVIDIA")); got != 1 {
		t.Errorf("tpl-a info = %v, want 1", got)
	}
	if got := testutil.ToFloat64(metrics.TemplateInfo.WithLabelValues("tpl-b", "CPU worker", "true", "false", "CPU")); got != 1 {
		t.Errorf("tpl-b info = %v, want 1", got)
	}
}

func TestTemplateDomain_Poll_HTTPErrorStaleServes(t *testing.T) {
	metrics.TemplateInfo.Reset()
	fs, srv := newFixtureServer()
	defer srv.Close()
	d := NewTemplateDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, templatesFixture)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.TemplateInfo.WithLabelValues("tpl-a", "PyTorch", "false", "true", "NVIDIA"))

	fs.set(http.StatusInternalServerError, `{"error":"database is on fire"}`)
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error on 500, got nil")
	}
	if after := testutil.ToFloat64(metrics.TemplateInfo.WithLabelValues("tpl-a", "PyTorch", "false", "true", "NVIDIA")); after != before {
		t.Errorf("tpl-a info changed on a failed poll: before=%v after=%v", before, after)
	}
}
