//go:build windows

package main

// The network side of keypulse: how many bytes each application sends and
// receives. Windows only exposes that through Event Tracing, and opening a
// trace session is privileged — while Raw Input and the foreground window
// are only visible from inside the interactive session. One process cannot
// do both, so this half runs as a service, from the same executable.
//
// Only the process id and the byte count are read from each event: never an
// address, a port, or any packet content.

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/jonathan-gtd/keypulse/internal/config"
	"github.com/jonathan-gtd/keypulse/internal/etw"
	"github.com/jonathan-gtd/keypulse/internal/netstat"
	"github.com/jonathan-gtd/keypulse/internal/sink"
	"github.com/jonathan-gtd/keypulse/internal/winutil"
)

const (
	netServiceName = "keypulse-net"
	netSessionName = "keypulse-net"
)

// kernelNetwork is Microsoft-Windows-Kernel-Network.
var kernelNetwork = windows.GUID{
	Data1: 0x7dd42a49, Data2: 0x5329, Data3: 0x4832,
	Data4: [8]byte{0x8d, 0xfd, 0x43, 0xd9, 0x79, 0x15, 0x3a, 0x88},
}

// netOptions is what the service is registered with.
type netOptions struct {
	output      string
	host        string
	measurement string
	interval    time.Duration
}

func (o netOptions) args() []string {
	return []string{"net-service",
		"-output", o.output, "-host", o.host,
		"-measurement", o.measurement, "-interval", o.interval.String()}
}

// netOptionsFromConfig takes the agent's own settings, so the service sends
// to the same place without a second thing to configure.
func netOptionsFromConfig(cfg config.Config) netOptions {
	return netOptions{
		output:      cfg.Output.URL,
		host:        cfg.Host,
		measurement: cfg.Measurement + "_net",
		interval:    cfg.Interval,
	}
}

// netCheck verifies everything that can be verified without administrator
// rights: that every structure shared with Windows has the size and layout
// the C headers give. A mismatch there is what turns a trace into a crash.
func netCheck() int {
	bad := 0
	fmt.Printf("keypulse %s — structures partagées avec Windows (x64)\n\n", version)
	fmt.Printf("  %-46s %8s %8s\n", "STRUCTURE", "ATTENDU", "OBTENU")
	for _, l := range etw.Layouts() {
		mark := "ok"
		if !l.OK() {
			mark, bad = "ERREUR", bad+1
		}
		fmt.Printf("  %-46s %8d %8d  %s\n", l.Name, l.Want, l.Got, mark)
	}
	fmt.Printf("\n  %-46s %8s %8s\n", "CHAMP LU DANS CHAQUE ÉVÉNEMENT", "ATTENDU", "OBTENU")
	for _, l := range etw.Offsets() {
		mark := "ok"
		if !l.OK() {
			mark, bad = "ERREUR", bad+1
		}
		fmt.Printf("  %-46s %8d %8d  %s\n", l.Name, l.Want, l.Got, mark)
	}
	fmt.Println()
	if bad > 0 {
		fmt.Printf("%d problème(s) : ne pas lancer la collecte en l'état.\n", bad)
		return 1
	}
	fmt.Println("Tout est conforme. Reste la session de trace elle-même,")
	fmt.Println("qui demande une invite administrateur.")
	return 0
}

// netRun collects until stop is closed.
func netRun(opt netOptions, stop <-chan struct{}) error {
	out, err := sink.New(opt.output, "", 1440)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	defer out.Close()

	c := netstat.New(winutil.ProcessName)
	session := etw.NewSession(netSessionName, kernelNetwork, func(e etw.Event) {
		if pid, size, sent, ok := netstat.DecodeKernelNetwork(e.ID, e.Data); ok {
			c.Add(pid, size, sent)
		}
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
	log.Printf("keypulse %s network: tracing, host=%s output=%s every %s",
		version, opt.host, opt.output, opt.interval)

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

// netService adapts netRun to the service control manager.
type netService struct{ opt netOptions }

func (s *netService) Execute(args []string, r <-chan svc.ChangeRequest, st chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	st <- svc.Status{State: svc.StartPending}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- netRun(s.opt, stop) }()
	st <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				st <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				st <- svc.Status{State: svc.StopPending}
				close(stop)
				<-done
				return false, 0
			}
		case err := <-done:
			st <- svc.Status{State: svc.StopPending}
			if err != nil {
				log.Printf("network collector stopped: %v", err)
				return false, 1
			}
			return false, 0
		}
	}
}

// netConsole runs the collector in a console, for trying it out.
func netConsole(opt netOptions) error {
	log.SetOutput(os.Stderr)
	log.Printf("keypulse %s network (console): output=%s every %s", version, opt.output, opt.interval)
	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() { <-sig; close(stop) }()
	return netRun(opt, stop)
}

// NetState says whether the network collector is installed and running.
type NetState struct {
	Installed bool
	Running   bool
}

func netState() NetState {
	m, err := mgr.Connect()
	if err != nil {
		return NetState{}
	}
	defer m.Disconnect()
	s, err := m.OpenService(netServiceName)
	if err != nil {
		return NetState{}
	}
	defer s.Close()
	st, err := s.Query()
	return NetState{Installed: true, Running: err == nil && st.State == svc.Running}
}

func netInstall(opt netOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("the service manager needs administrator rights: %w", err)
	}
	defer m.Disconnect()
	if s, err := m.OpenService(netServiceName); err == nil {
		s.Close()
		return fmt.Errorf("the network collector is already installed")
	}
	s, err := m.CreateService(netServiceName, exe, mgr.Config{
		DisplayName:  "keypulse network statistics",
		Description:  "Counts bytes sent and received per application, for keypulse. Reads only byte counts, never packet content.",
		StartType:    mgr.StartAutomatic,
		ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
		Dependencies: []string{"Tcpip"},
	}, opt.args()...)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Start()
}

func netUninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("the service manager needs administrator rights: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(netServiceName)
	if err != nil {
		return fmt.Errorf("the network collector is not installed")
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

// netCommand handles the network sub-commands. It reports whether it
// handled one, and the exit code to use.
func netCommand(cmd string, argv []string) (handled bool, code int) {
	if cmd != "net-service" {
		// Run by hand from a terminal: make the output visible.
		winutil.AttachParentConsole()
	}
	if cmd == "net-check" {
		return true, netCheck()
	}
	if cmd == "net-uninstall" {
		if err := netUninstall(); err != nil {
			fmt.Fprintln(os.Stderr, "uninstall:", err)
			return true, 1
		}
		fmt.Println("network statistics disabled")
		return true, 0
	}
	if cmd != "net" && cmd != "net-install" && cmd != "net-service" {
		return false, 0
	}

	// The service runs as LocalSystem, where the user's configuration is
	// not reachable — so its settings are baked into the command line when
	// it is registered. Run by hand, the settings come from the config.
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	var o netOptions
	fs.StringVar(&o.output, "output", "", "where to send the measurements")
	fs.StringVar(&o.host, "host", "", "value of the host tag")
	fs.StringVar(&o.measurement, "measurement", "", "measurement name")
	fs.DurationVar(&o.interval, "interval", 0, "how often to report")
	path := fs.String("config", config.DefaultPath(), "configuration file")
	if err := fs.Parse(argv); err != nil {
		return true, 2
	}
	if o.output == "" || o.host == "" || o.measurement == "" || o.interval == 0 {
		cfg, err := config.Load(*path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true, 1
		}
		d := netOptionsFromConfig(cfg)
		if o.output == "" {
			o.output = d.output
		}
		if o.host == "" {
			o.host = d.host
		}
		if o.measurement == "" {
			o.measurement = d.measurement
		}
		if o.interval == 0 {
			o.interval = d.interval
		}
	}

	switch cmd {
	case "net-service":
		setupLog(netLogPath())
		if err := svc.Run(netServiceName, &netService{opt: o}); err != nil {
			log.Printf("service: %v", err)
			return true, 1
		}
	case "net":
		if err := netConsole(o); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return true, 1
		}
	case "net-install":
		if err := netInstall(o); err != nil {
			fmt.Fprintln(os.Stderr, "install:", err)
			return true, 1
		}
		fmt.Println("network statistics enabled")
	}
	return true, 0
}

// netToggle turns the network collector on or off, asking Windows for
// administrator rights: registering a service needs them, and the agent
// itself deliberately runs without.
func netToggle(on bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb := "net-install"
	if !on {
		verb = "net-uninstall"
	}
	return winutil.RunElevated(exe, verb)
}

func netLogPath() string {
	dir := os.Getenv("ProgramData")
	if dir == "" {
		dir = os.TempDir()
	}
	return dir + `\keypulse\keypulse-net.log`
}
