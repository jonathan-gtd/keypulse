// Package config loads keypulse's YAML configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the content of config.yaml.
type Config struct {
	Interval    time.Duration `yaml:"interval"`
	Host        string        `yaml:"host"`
	Measurement string        `yaml:"measurement"`
	PerKey      bool          `yaml:"per_key"`
	PerApp      bool          `yaml:"per_app"`
	Shortcuts   bool          `yaml:"shortcuts"`
	Clipboard   bool          `yaml:"clipboard"`
	SessionLock bool          `yaml:"session_lock"`
	MouseDPI    float64       `yaml:"mouse_dpi"`
	Heatmap     Heatmap       `yaml:"heatmap"`
	Output      Output        `yaml:"output"`
	Tray        Tray          `yaml:"tray"`
	UI          UI            `yaml:"ui"`
	LogFile     string        `yaml:"log_file"`
}

// Tray is the notification-area icon.
type Tray struct {
	Enabled bool `yaml:"enabled"`
}

// UI is the small local status page.
type UI struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// Heatmap configures cursor and click positions.
type Heatmap struct {
	Enabled  bool `yaml:"enabled"`
	CellSize int  `yaml:"cell_size"`
}

// Output is where the points go.
type Output struct {
	URL       string `yaml:"url"`
	Token     string `yaml:"token"`
	MaxBuffer int    `yaml:"max_buffer"`
}

// Default returns the configuration used when no file exists: one report a
// minute to a local Telegraf socket_listener.
func Default() Config {
	return Config{
		Interval:    time.Minute,
		Measurement: "keypulse",
		PerApp:      true,
		Shortcuts:   true,
		Clipboard:   true,
		SessionLock: true,
		Heatmap:     Heatmap{Enabled: true, CellSize: 40},
		Output:      Output{URL: "udp://127.0.0.1:8094", MaxBuffer: 1440},
		Tray:        Tray{Enabled: true},
		UI:          UI{Enabled: true, Addr: "127.0.0.1:8099"},
	}
}

// DefaultPath is %APPDATA%\keypulse\config.yaml on Windows.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "keypulse", "config.yaml")
}

// DefaultLogPath is %LOCALAPPDATA%\keypulse\keypulse.log on Windows.
func DefaultLogPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "keypulse", "keypulse.log")
}

// Load reads the file at path. A missing file yields the defaults.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, cfg.finish()
	}
	if err != nil {
		return cfg, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // BOM written by some editors
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, cfg.finish()
}

func (c *Config) finish() error {
	if c.Host == "" {
		if h, err := os.Hostname(); err == nil {
			c.Host = strings.ToLower(h)
		}
	}
	if c.Measurement == "" {
		c.Measurement = "keypulse"
	}
	if c.Heatmap.CellSize == 0 {
		c.Heatmap.CellSize = 40
	}
	if c.UI.Addr == "" {
		c.UI.Addr = "127.0.0.1:8099"
	}
	switch {
	case c.Interval < time.Second:
		return fmt.Errorf("interval must be at least 1s, got %s", c.Interval)
	case c.Heatmap.CellSize < 1:
		return fmt.Errorf("heatmap.cell_size must be positive, got %d", c.Heatmap.CellSize)
	case c.MouseDPI < 0:
		return fmt.Errorf("mouse_dpi cannot be negative")
	case c.Output.URL == "":
		return fmt.Errorf("output.url is required")
	}
	return nil
}
