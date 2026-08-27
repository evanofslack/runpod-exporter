package collector

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/evanofslack/runpod-exporter/internal/config"
	"github.com/evanofslack/runpod-exporter/openapi"
)

// captureTransport records the request URL instead of hitting the network.
type captureTransport struct {
	capturedURL string
	body        string
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.capturedURL = req.URL.String()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}, nil
}

// TestDefaultAPIURL_DoesNotDoubleVersionPath is a regression test. The
// generated client's operation paths already include /v2 (they match the
// openapi spec's own servers[0].url, which is the bare host, no /v2). If
// --api-url's default also includes /v2, every request resolves to
// /v2/v2/... and 404s against the real API — this shipped once already.
// It only surfaced against a valid key, because every other test builds its
// client from a bare httptest.Server URL with no path segment, which can't
// exercise the doubling behavior url.Parse's relative resolution produces
// when the base URL's path doesn't end in "/".
func TestDefaultAPIURL_DoesNotDoubleVersionPath(t *testing.T) {
	cfg, err := config.Parse(nil, func(k string) string {
		if k == "RUNPOD_API_KEY" {
			return "test-key"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("config.Parse: %v", err)
	}

	ct := &captureTransport{body: `{"pods":[]}`}
	auth := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		return nil
	}
	client, err := openapi.NewClientWithResponses(
		cfg.APIURL.String(),
		openapi.WithRequestEditorFn(auth),
		openapi.WithHTTPClient(&http.Client{Transport: ct}),
	)
	if err != nil {
		t.Fatalf("NewClientWithResponses: %v", err)
	}

	if _, err := client.ListPodsWithResponse(context.Background(), nil); err != nil {
		t.Fatalf("ListPodsWithResponse: %v", err)
	}

	const want = "https://api.runpod.io/v2/pods"
	if ct.capturedURL != want {
		t.Errorf("request URL = %q, want %q", ct.capturedURL, want)
	}
}
