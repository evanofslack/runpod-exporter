package config

import (
	"log/slog"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestParse_EnvDefaults(t *testing.T) {
	cfg, err := Parse(nil, env(map[string]string{"RUNPOD_API_KEY": "secret"}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.APIKey != "secret" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "secret")
	}
	if cfg.APIURL.String() != "https://api.runpod.io/v2" {
		t.Errorf("APIURL = %q", cfg.APIURL.String())
	}
	if got, want := cfg.Domains, []string{"pod", "account", "billing"}; !equal(got, want) {
		t.Errorf("Domains = %v, want %v", got, want)
	}
	if cfg.ListenAddr != ":9836" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
	if cfg.ScrapeInterval != 30*time.Second {
		t.Errorf("ScrapeInterval = %v", cfg.ScrapeInterval)
	}
	if cfg.ScrapeIntervalSlow != 5*time.Minute {
		t.Errorf("ScrapeIntervalSlow = %v", cfg.ScrapeIntervalSlow)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
}

func TestParse_FlagOverridesEnv(t *testing.T) {
	getenv := env(map[string]string{
		"RUNPOD_API_KEY":   "env-key",
		"RUNPOD_DOMAINS":   "pod",
		"RUNPOD_LOG_LEVEL": "warn",
	})
	cfg, err := Parse([]string{"--api-key=flag-key", "--domains=cluster,catalog", "--log-level=debug"}, getenv)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("APIKey = %q, want flag-key", cfg.APIKey)
	}
	if got, want := cfg.Domains, []string{"cluster", "catalog"}; !equal(got, want) {
		t.Errorf("Domains = %v, want %v", got, want)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

func TestParse_AllDomainsExpansion(t *testing.T) {
	cfg, err := Parse([]string{"--domains=all"}, env(map[string]string{"RUNPOD_API_KEY": "k"}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !equal(cfg.Domains, AllDomains) {
		t.Errorf("Domains = %v, want %v", cfg.Domains, AllDomains)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"missing api key", nil, nil},
		{"unknown domain", []string{"--domains=nope"}, map[string]string{"RUNPOD_API_KEY": "k"}},
		{"interval below floor", []string{"--scrape-interval=1s"}, map[string]string{"RUNPOD_API_KEY": "k"}},
		{"unparseable interval", []string{"--scrape-interval=soon"}, map[string]string{"RUNPOD_API_KEY": "k"}},
		{"unparseable api url", []string{"--api-url=:::not a url"}, map[string]string{"RUNPOD_API_KEY": "k"}},
		{"api url missing host", []string{"--api-url=/just/a/path"}, map[string]string{"RUNPOD_API_KEY": "k"}},
		{"invalid log level", []string{"--log-level=verbose"}, map[string]string{"RUNPOD_API_KEY": "k"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.args, env(tt.env))
			if err == nil {
				t.Fatal("Parse: want error, got nil")
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
