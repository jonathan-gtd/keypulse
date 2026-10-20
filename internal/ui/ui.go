// Package ui serves a small status page on the loopback interface, so you
// can see at a glance what keypulse is counting and where it sends it.
package ui

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"
)

//go:embed index.html
var assets embed.FS

// Status is what the page displays.
type Status struct {
	Version   string   `json:"version"`
	Host      string   `json:"host"`
	Output    string   `json:"output"`
	Interval  string   `json:"interval"`
	Since     string   `json:"since"`
	UptimeSec int64    `json:"uptime_s"`
	Tracking  []string `json:"tracking"`

	LastSend   string `json:"last_send"`
	LastError  string `json:"last_error"`
	SentPoints uint64 `json:"sent_points"`

	Keys            uint64  `json:"keys"`
	Clicks          uint64  `json:"clicks"`
	DoubleClicks    uint64  `json:"double_clicks"`
	WheelNotches    int64   `json:"wheel_notches"`
	MouseMeters     float64 `json:"mouse_m"`
	ActiveSeconds   uint64  `json:"active_s"`
	IdleSeconds     uint64  `json:"idle_s"`
	Shortcuts       uint64  `json:"shortcuts"`
	ClipboardCopies uint64  `json:"clipboard_copies"`
	AppSwitches     uint64  `json:"app_switches"`
	TypingPeak1s    uint64  `json:"typing_peak_1s"`

	Apps []AppRow `json:"apps"`
}

// AppRow is one line of the per-application table.
type AppRow struct {
	App           string `json:"app"`
	Keys          uint64 `json:"keys"`
	Clicks        uint64 `json:"clicks"`
	ActiveSeconds uint64 `json:"active_s"`
}

// Server is the status page.
type Server struct {
	srv  *http.Server
	ln   net.Listener
	addr string
}

// New returns a server that calls status for every request. It listens
// immediately so that a port conflict is reported at start-up.
func New(addr string, status func() Status) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	page, err := assets.ReadFile("index.html")
	if err != nil {
		ln.Close()
		return nil, err
	}
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(status())
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})
	return &Server{
		srv:  &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second},
		ln:   ln,
		addr: ln.Addr().String(),
	}, nil
}

// Serve runs until Close. It returns nil on a clean shutdown.
func (s *Server) Serve() error {
	if err := s.srv.Serve(s.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// URL is the address to open in a browser.
func (s *Server) URL() string { return "http://" + s.addr + "/" }

// Close stops the server.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
