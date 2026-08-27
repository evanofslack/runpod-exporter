package collector

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/evanofslack/runpod-exporter/openapi"
)

// NewClient builds a Runpod v2 REST client authenticated with apiKey against
// baseURL.
func NewClient(baseURL *url.URL, apiKey string) (*openapi.ClientWithResponses, error) {
	auth := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}
	client, err := openapi.NewClientWithResponses(baseURL.String(), openapi.WithRequestEditorFn(auth))
	if err != nil {
		return nil, fmt.Errorf("build openapi client: %w", err)
	}
	return client, nil
}

// Build returns the Domain implementations for the given domain names,
// warning and skipping any name that has no collector implemented yet.
func Build(domainNames []string, client *openapi.ClientWithResponses) []Domain {
	var out []Domain
	for _, name := range domainNames {
		switch name {
		case "pod":
			out = append(out, NewPodDomain(client))
		default:
			slog.Warn("domain not yet implemented, skipping", "domain", name)
		}
	}
	return out
}
