// Command keypulse counts keyboard and mouse activity and sends it to
// Telegraf or InfluxDB as line protocol. It counts only: no typed text and
// no key sequence is ever kept or sent.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jonathan-gtd/keypulse/internal/config"
	"github.com/jonathan-gtd/keypulse/internal/input"
	"github.com/jonathan-gtd/keypulse/internal/lineproto"
	"github.com/jonathan-gtd/keypulse/internal/sink"
	"github.com/jonathan-gtd/keypulse/internal/stats"
	"github.com/jonathan-gtd/keypulse/internal/tray"
	"github.com/jonathan-gtd/keypulse/internal/ui"
	"github.com/jonathan-gtd/keypulse/internal/winutil"
)

var (
	version = "dev"
	newline = []byte{'\n'}
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

// agent holds the running state shared with the local status page.
type agent struct {
	cfg     config.Config
	counter *stats.Counter
	out     sink.Sink
	enc     lineproto.Options

	mu      sync.Mutex
	session stats.Session
	sent    uint64
	last    time.Time
	lastErr string
}

// flush turns the period into points and sends them.
func (a *agent) flush(end time.Time) {
	s := a.counter.Snapshot(end)
	if s.Empty() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.session.Add(s)
	payload := lineproto.Encode(a.enc, s)
	if err := a.out.Send(payload); err != nil {
		a.lastErr = err.Error()
		log.Printf("send: %v", err)
		return
	}
	a.lastErr = ""
	a.last = end
	a.sent += uint64(bytes.Count(payload, newline))
}

func (a *agent) status() ui.Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := ui.Status{
		Version: version, Host: a.cfg.Host, Output: a.cfg.Output.URL,
		Interval: a.cfg.Interval.String(), Since: a.session.Since.Format(time.RFC3339),
		UptimeSec: int64(time.Since(a.session.Since).Seconds()), Tracking: a.tracking(),
		SentPoints: a.sent, LastError: a.lastErr,
		Keys: a.session.Keys, Clicks: a.session.Clicks, DoubleClicks: a.session.DoubleClicks,
		WheelNotches: int64(a.session.WheelNotches), MouseMeters: a.session.MouseMeters,
		ActiveSeconds: a.session.ActiveSeconds, IdleSeconds: a.session.IdleSeconds,
		Shortcuts: a.session.Shortcuts, ClipboardCopies: a.session.ClipboardCopies,
		AppSwitches: a.session.AppSwitches, TypingPeak1s: a.session.TypingPeak1s,
		HandLeft: a.session.HandLeft, HandRight: a.session.HandRight,
		CursorPeak: a.session.CursorPeak, CapturedSeconds: a.session.CapturedSeconds,
	}
	if !a.last.IsZero() {
		st.LastSend = a.last.Format("15:04:05")
	}
	for _, t := range a.session.TopApps {
		st.Apps = append(st.Apps, ui.AppRow{App: t.App, Keys: t.Keys, Clicks: t.Clicks, ActiveSeconds: t.ActiveSeconds})
	}
	return st
}

func (a *agent) tracking() []string {
	t := []string{"keys", "clicks", "wheel", "mouse"}
	for _, o := range []struct {
		on   bool
		name string
	}{
		{a.cfg.PerApp, "per application"},
		{a.cfg.PerKey, "per key"},
		{a.cfg.Shortcuts, "shortcuts"},
		{a.cfg.Heatmap.Enabled, "heatmap"},
		{a.cfg.Clipboard, "clipboard count"},
		{a.cfg.SessionLock, "lock / unlock"},
	} {
		if o.on {
			t = append(t, o.name)
		}
	}
	return t
}

func run() error {
	configPath := flag.String("config", config.DefaultPath(), "configuration file")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return nil
	}

	closeLog := setupLog(config.DefaultLogPath())
	defer func() { closeLog() }()
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if cfg.LogFile != "" {
		closeLog()
		closeLog = setupLog(cfg.LogFile)
	}

	release, err := winutil.SingleInstance(`Local\keypulse`)
	if errors.Is(err, winutil.ErrAlreadyRunning) {
		log.Print(err)
		return nil
	}
	if err != nil {
		return err
	}
	defer release()
	if err := winutil.LowerPriority(); err != nil {
		log.Printf("lowering priority: %v", err)
	}
	if err := winutil.SetDPIAware(); err != nil {
		log.Printf("DPI awareness: %v", err)
	}

	out, err := sink.New(cfg.Output.URL, cfg.Output.Token, cfg.Output.MaxBuffer)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	defer out.Close()

	a := &agent{
		cfg:     cfg,
		counter: stats.NewCounter(cfg.PerKey, cfg.Heatmap.Enabled),
		out:     out,
		enc:     lineproto.Options{Measurement: cfg.Measurement, Host: cfg.Host, MouseDPI: cfg.MouseDPI},
	}
	a.session.Since = time.Now()

	// The status page is a convenience: a busy port must not stop counting.
	var page *ui.Server
	if cfg.UI.Enabled {
		if page, err = ui.New(cfg.UI.Addr, a.status); err != nil {
			log.Printf("status page disabled: %v", err)
			page = nil
		} else {
			defer page.Close()
			go func() {
				if err := page.Serve(); err != nil {
					log.Printf("status page: %v", err)
				}
			}()
		}
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	go func() {
		err := input.Run(ctx, a.counter, input.Options{
			PerApp:       cfg.PerApp,
			Heatmap:      cfg.Heatmap.Enabled,
			CellSize:     cfg.Heatmap.CellSize,
			Clipboard:    cfg.Clipboard,
			Session:      cfg.SessionLock,
			OnEndSession: func() { a.flush(time.Now()) },
		})
		if err != nil {
			log.Printf("input: %v", err)
		}
		cancel()
	}()
	go a.loop(ctx)

	url := ""
	if page != nil {
		url = page.URL()
	}
	log.Printf("keypulse %s: host=%s output=%s interval=%s tracking=%s page=%s",
		version, cfg.Host, cfg.Output.URL, cfg.Interval, strings.Join(a.tracking(), ", "), url)

	if cfg.Tray.Enabled {
		go func() {
			<-ctx.Done()
			tray.Quit()
		}()
		tray.Run(tray.Options{
			Tooltip:  "keypulse — counting keyboard and mouse activity",
			StatsURL: url,
			OnQuit:   cancel,
		}, nil)
		cancel()
	} else {
		<-ctx.Done()
	}

	a.flush(time.Now())
	return nil
}

// loop sends a report at every interval, aligned on the clock.
func (a *agent) loop(ctx context.Context) {
	for {
		next := time.Now().Truncate(a.cfg.Interval).Add(a.cfg.Interval)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			a.flush(next)
		}
	}
}

// setupLog writes the log to path (and stderr), keeping one previous file
// once it grows past 5 MB. The returned function closes it.
func setupLog(path string) func() {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return func() {}
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > 5<<20 {
		os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return func() {}
	}
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	return func() {
		log.SetOutput(os.Stderr)
		f.Close()
	}
}
