// Command keypulse counts keyboard and mouse activity and sends it to
// Telegraf or InfluxDB as line protocol.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/jonathan-gtd/keypulse/internal/config"
	"github.com/jonathan-gtd/keypulse/internal/input"
	"github.com/jonathan-gtd/keypulse/internal/lineproto"
	"github.com/jonathan-gtd/keypulse/internal/sink"
	"github.com/jonathan-gtd/keypulse/internal/stats"
	"github.com/jonathan-gtd/keypulse/internal/winutil"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
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

	out, err := sink.New(cfg.Output.URL, cfg.Output.Token, cfg.Output.MaxBuffer)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	defer out.Close()

	counter := stats.NewCounter(cfg.PerKey, cfg.Heatmap.Enabled)
	enc := lineproto.Options{Measurement: cfg.Measurement, Host: cfg.Host, MouseDPI: cfg.MouseDPI}
	var sendMu sync.Mutex
	flush := func(end time.Time) {
		s := counter.Snapshot(end)
		if s.Empty() {
			return
		}
		sendMu.Lock()
		defer sendMu.Unlock()
		if err := out.Send(lineproto.Encode(enc, s)); err != nil {
			log.Printf("send: %v", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	inputErr := make(chan error, 1)
	go func() {
		inputErr <- input.Run(ctx, counter, input.Options{
			PerApp:       cfg.PerApp,
			Heatmap:      cfg.Heatmap.Enabled,
			CellSize:     cfg.Heatmap.CellSize,
			OnEndSession: func() { flush(time.Now()) },
		})
	}()

	log.Printf("keypulse %s: host=%s output=%s interval=%s per_key=%t per_app=%t heatmap=%t",
		version, cfg.Host, cfg.Output.URL, cfg.Interval, cfg.PerKey, cfg.PerApp, cfg.Heatmap.Enabled)

	for {
		// Reports are aligned on the clock: every minute at :00 with the default interval.
		next := time.Now().Truncate(cfg.Interval).Add(cfg.Interval)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			flush(time.Now())
			return nil
		case err := <-inputErr:
			timer.Stop()
			flush(time.Now())
			return fmt.Errorf("input: %w", err)
		case <-timer.C:
			flush(next)
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
