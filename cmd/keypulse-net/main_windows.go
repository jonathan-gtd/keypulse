//go:build windows

// Command keypulse-net reports how many bytes each application sends and
// receives, from the Windows kernel network provider.
//
// Windows only exposes per-process network traffic through Event Tracing,
// and opening a trace session is privileged, so this runs as a service
// under LocalSystem — separately from keypulse itself, which stays a plain
// user program. Only byte counts per process are read: no address, no port
// and no packet content.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/jonathan-gtd/keypulse/internal/etw"
	"github.com/jonathan-gtd/keypulse/internal/netstat"
	"github.com/jonathan-gtd/keypulse/internal/sink"
	"github.com/jonathan-gtd/keypulse/internal/winutil"
)

var version = "dev"

const (
	serviceName = "keypulse-net"
	sessionName = "keypulse-net"
)

// kernelNetwork is Microsoft-Windows-Kernel-Network.
var kernelNetwork = windows.GUID{
	Data1: 0x7dd42a49, Data2: 0x5329, Data3: 0x4832,
	Data4: [8]byte{0x8d, 0xfd, 0x43, 0xd9, 0x79, 0x15, 0x3a, 0x88},
}

// Event ids of the kernel network provider. The payload of every one of
// them starts with the process id and the number of bytes.
var direction = map[uint16]bool{
	10: true, 26: true, 42: true, 58: true, // sent:     TCP v4/v6, UDP v4/v6
	11: false, 27: false, 43: false, 59: false, // received: idem
}

type options struct {
	output      string
	host        string
	measurement string
	interval    time.Duration
	logFile     string
}

func main() {
	opt := options{}
	flag.StringVar(&opt.output, "output", "udp://127.0.0.1:8094", "where to send the measurements")
	flag.StringVar(&opt.host, "host", "", "value of the host tag (default: computer name)")
	flag.StringVar(&opt.measurement, "measurement", "keypulse_net", "measurement name")
	flag.DurationVar(&opt.interval, "interval", time.Minute, "how often to report")
	flag.StringVar(&opt.logFile, "log", defaultLog(), "log file")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	switch flag.Arg(0) {
	case "install":
		if err := install(opt); err != nil {
			fmt.Fprintln(os.Stderr, "install:", err)
			os.Exit(1)
		}
		fmt.Printf("service %q installed and started\n", serviceName)
		return
	case "uninstall":
		if err := uninstall(); err != nil {
			fmt.Fprintln(os.Stderr, "uninstall:", err)
			os.Exit(1)
		}
		fmt.Printf("service %q removed\n", serviceName)
		return
	}

	setupLog(opt.logFile)
	if opt.host == "" {
		if h, err := os.Hostname(); err == nil {
			opt.host = lower(h)
		}
	}

	isService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("service check: %v", err)
	}
	if isService {
		if err := svc.Run(serviceName, &handler{opt: opt}); err != nil {
			log.Fatalf("service: %v", err)
		}
		return
	}

	// Console mode, for trying it out: Ctrl-C stops it cleanly, so the
	// trace session is always closed behind us.
	log.SetOutput(os.Stderr)
	log.Printf("keypulse-net %s (console): output=%s interval=%s", version, opt.output, opt.interval)
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; close(stop) }()
	if err := collect(opt, stop); err != nil {
		log.Fatalf("%v", err)
	}
}

// handler adapts collect to the service control manager.
type handler struct{ opt options }

func (h *handler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	s <- svc.Status{State: svc.StartPending}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- collect(h.opt, stop) }()
	s <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				close(stop)
				<-done
				return false, 0
			}
		case err := <-done:
			if err != nil {
				log.Printf("collector stopped: %v", err)
				s <- svc.Status{State: svc.StopPending}
				return false, 1
			}
			s <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
}

// collect runs the trace session until stop is closed.
func collect(opt options, stop <-chan struct{}) error {
	out, err := sink.New(opt.output, "", 1440)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	defer out.Close()

	c := netstat.New(winutil.ProcessName)
	session := etw.NewSession(sessionName, kernelNetwork, func(e etw.Event) {
		sent, ok := direction[e.ID]
		if !ok || len(e.Data) < 8 {
			return
		}
		pid := uint32(e.Data[0]) | uint32(e.Data[1])<<8 | uint32(e.Data[2])<<16 | uint32(e.Data[3])<<24
		size := uint32(e.Data[4]) | uint32(e.Data[5])<<8 | uint32(e.Data[6])<<16 | uint32(e.Data[7])<<24
		c.Add(pid, size, sent)
	})
	if err := session.Start(); err != nil {
		return err
	}
	defer session.Stop()

	traced := make(chan error, 1)
	go func() { traced <- session.Process() }()

	flush := func(at time.Time) {
		payload := netstat.Encode(opt.measurement, opt.host, at.UnixNano(), c.Snapshot())
		if payload == nil {
			return
		}
		if err := out.Send(payload); err != nil {
			log.Printf("send: %v", err)
		}
	}
	log.Printf("keypulse-net %s: tracing, host=%s output=%s every %s", version, opt.host, opt.output, opt.interval)

	for {
		next := time.Now().Truncate(opt.interval).Add(opt.interval)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-stop:
			timer.Stop()
			flush(time.Now())
			session.Stop()
			<-traced
			return nil
		case err := <-traced:
			timer.Stop()
			flush(time.Now())
			return err
		case <-timer.C:
			flush(next)
		}
	}
}

func install(opt options) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("the service manager needs administrator rights: %w", err)
	}
	defer m.Disconnect()
	if s, err := m.OpenService(serviceName); err == nil {
		s.Close()
		return fmt.Errorf("service %q already exists — run `%s uninstall` first", serviceName, filepath.Base(exe))
	}
	s, err := m.CreateService(serviceName, exe, mgr.Config{
		DisplayName:  "keypulse network collector",
		Description:  "Counts bytes sent and received per application, for keypulse. Reads only byte counts, never packet content.",
		StartType:    mgr.StartAutomatic,
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		Dependencies: []string{"Tcpip"},
	},
		"-output", opt.output, "-host", opt.host,
		"-measurement", opt.measurement, "-interval", opt.interval.String(), "-log", opt.logFile)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

func uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("the service manager needs administrator rights: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %q is not installed", serviceName)
	}
	defer s.Close()
	if st, err := s.Control(svc.Stop); err == nil {
		for i := 0; i < 20 && st.State != svc.Stopped; i++ {
			time.Sleep(250 * time.Millisecond)
			if st, err = s.Query(); err != nil {
				break
			}
		}
	}
	return s.Delete()
}

func defaultLog() string {
	dir := os.Getenv("ProgramData")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "keypulse", "keypulse-net.log")
}

func setupLog(path string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > 5<<20 {
		os.Rename(path, path+".1")
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		log.SetOutput(f)
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
