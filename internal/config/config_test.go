package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != time.Minute || cfg.Output.URL != "udp://127.0.0.1:8094" || !cfg.PerApp || cfg.PerKey || cfg.Host == "" {
		t.Errorf("defaults = %+v", cfg)
	}
}

func TestLoad(t *testing.T) {
	p := write(t, "\xef\xbb\xbfinterval: 30s\nhost: Mars\nper_key: true\nmouse_dpi: 1600\nheatmap:\n  enabled: false\noutput:\n  url: http://127.0.0.1:8186/write\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Interval != 30*time.Second || cfg.Host != "Mars" || !cfg.PerKey || cfg.MouseDPI != 1600 ||
		cfg.Heatmap.Enabled || cfg.Heatmap.CellSize != 40 || cfg.Output.URL != "http://127.0.0.1:8186/write" || !cfg.PerApp {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	for content, want := range map[string]string{
		"intervall: 30s\n":            "field intervall not found",
		"interval: 10ms\n":            "interval must be at least 1s",
		"output:\n  url: \"\"\n":      "output.url is required",
		"heatmap:\n  cell_size: -2\n": "cell_size must be positive",
	} {
		_, err := Load(write(t, content))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", content, err, want)
		}
	}
}
