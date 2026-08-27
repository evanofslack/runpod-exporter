package collector

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/evanofslack/runpod-exporter/internal/metrics"
)

func TestAccountDomain_Poll_Success(t *testing.T) {
	fs, srv := newFixtureServer()
	defer srv.Close()
	fs.set(http.StatusOK, `{"keys":["ssh-ed25519 AAAA... a@b","ssh-ed25519 BBBB... c@d"]}`)

	d := NewAccountDomain(testClient(t, srv.URL))
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if got := testutil.ToFloat64(metrics.AccountSSHKeys); got != 2 {
		t.Errorf("AccountSSHKeys = %v, want 2", got)
	}
}

func TestAccountDomain_Poll_HTTPErrorStaleServes(t *testing.T) {
	fs, srv := newFixtureServer()
	defer srv.Close()
	d := NewAccountDomain(testClient(t, srv.URL))

	fs.set(http.StatusOK, `{"keys":["ssh-ed25519 AAAA... a@b"]}`)
	if err := d.Poll(context.Background()); err != nil {
		t.Fatalf("first Poll: %v", err)
	}
	before := testutil.ToFloat64(metrics.AccountSSHKeys)

	fs.set(http.StatusTooManyRequests, `{"error":"slow down"}`)
	if err := d.Poll(context.Background()); err == nil {
		t.Fatal("Poll: want error on 429, got nil")
	}

	if after := testutil.ToFloat64(metrics.AccountSSHKeys); after != before {
		t.Errorf("AccountSSHKeys changed on a failed poll: before=%v after=%v", before, after)
	}
}
