// Package config resolves runpod-exporter's flags and env vars into a
// validated Config.
package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

const minScrapeInterval = 5 * time.Second

var AllDomains = []string{
	"pod", "serverless", "billing", "account", "cluster",
	"template", "network-volume", "registry", "catalog",
}

type Config struct {
	APIKey             string
	APIURL             *url.URL
	Domains            []string
	ListenAddr         string
	ScrapeInterval     time.Duration
	ScrapeIntervalSlow time.Duration
	LogLevel           slog.Level
}

// Parse resolves flags and env vars (flag wins if both are set) into a
// validated Config. args excludes the program name, as in os.Args[1:].
//
// api-key is a deliberate exception to "default resolved from env before
// flag registration": its default is left blank so --help never echoes a
// secret pulled from RUNPOD_API_KEY, and the env value is applied after
// parsing instead.
func Parse(args []string, getenv func(string) string) (*Config, error) {
	fs := flag.NewFlagSet("runpod-exporter", flag.ContinueOnError)

	apiKey := fs.String("api-key", "", "Runpod API key (required; env RUNPOD_API_KEY)")
	apiURL := fs.String("api-url", envOrDefault(getenv, "RUNPOD_API_URL", "https://api.runpod.io/v2"), "Runpod API base URL")
	domains := fs.String("domains", envOrDefault(getenv, "RUNPOD_DOMAINS", "pod,account,billing"), `comma-separated domains to poll, or "all"`)
	listenAddr := fs.String("listen-addr", envOrDefault(getenv, "RUNPOD_LISTEN_ADDR", ":9836"), "address to serve /metrics and /healthz on")
	scrapeInterval := fs.String("scrape-interval", envOrDefault(getenv, "RUNPOD_SCRAPE_INTERVAL", "30s"), "poll interval for fast-tier domains")
	scrapeIntervalSlow := fs.String("scrape-interval-slow", envOrDefault(getenv, "RUNPOD_SCRAPE_INTERVAL_SLOW", "5m"), "poll interval for slow-tier domains")
	logLevel := fs.String("log-level", envOrDefault(getenv, "RUNPOD_LOG_LEVEL", "info"), "debug, info, warn, or error")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if *apiKey == "" {
		*apiKey = getenv("RUNPOD_API_KEY")
	}
	if *apiKey == "" {
		return nil, errors.New("api-key is required")
	}

	parsedURL, err := url.Parse(*apiURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("invalid api-url %q", *apiURL)
	}

	domainList, err := parseDomains(*domains)
	if err != nil {
		return nil, err
	}

	interval, err := parseInterval("scrape-interval", *scrapeInterval)
	if err != nil {
		return nil, err
	}
	intervalSlow, err := parseInterval("scrape-interval-slow", *scrapeIntervalSlow)
	if err != nil {
		return nil, err
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return nil, err
	}

	return &Config{
		APIKey:             *apiKey,
		APIURL:             parsedURL,
		Domains:            domainList,
		ListenAddr:         *listenAddr,
		ScrapeInterval:     interval,
		ScrapeIntervalSlow: intervalSlow,
		LogLevel:           level,
	}, nil
}

func envOrDefault(getenv func(string) string, key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDomains(s string) ([]string, error) {
	if s == "all" {
		out := make([]string, len(AllDomains))
		copy(out, AllDomains)
		return out, nil
	}

	known := make(map[string]bool, len(AllDomains))
	for _, d := range AllDomains {
		known[d] = true
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !known[p] {
			return nil, fmt.Errorf("unknown domain %q (known: %s)", p, strings.Join(AllDomains, ", "))
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, errors.New("domains must not be empty")
	}
	return out, nil
}

func parseInterval(flagName, s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", flagName, s, err)
	}
	if d < minScrapeInterval {
		return 0, fmt.Errorf("%s %q is below the minimum of %s", flagName, s, minScrapeInterval)
	}
	return d, nil
}

func parseLogLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log-level %q (want debug, info, warn, or error)", s)
	}
}
