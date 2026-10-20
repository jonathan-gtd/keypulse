//go:build windows

// Package winutil holds process-level Windows helpers.
package winutil

import (
	"errors"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning is returned by SingleInstance when another instance holds the lock.
var ErrAlreadyRunning = errors.New("keypulse is already running")

var (
	procSetProcessInformation         = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetProcessInformation")
	procSetProcessDpiAwarenessContext = windows.NewLazySystemDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext")
)

// dpiAwarenessPerMonitorV2 is DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2, (HANDLE)-4.
const dpiAwarenessPerMonitorV2 = ^uintptr(3)

// SetDPIAware makes Windows report real pixels to the process. Without it,
// cursor positions are scaled on monitors set to 125 %, 150 %…, which skews
// the heatmap and the cursor distance.
func SetDPIAware() error {
	if r, _, err := procSetProcessDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2); r == 0 {
		return err
	}
	return nil
}

const (
	processPowerThrottling          = 4 // PROCESS_INFORMATION_CLASS
	processPowerThrottlingVersion   = 1
	processPowerThrottlingExecSpeed = 0x1
)

type powerThrottlingState struct {
	version, controlMask, stateMask uint32
}

// LowerPriority runs the process below normal priority and opts it into
// EcoQoS, so that Windows schedules it on efficiency cores when it can and
// it never competes with a game for CPU time.
func LowerPriority() error {
	h := windows.CurrentProcess()
	if err := windows.SetPriorityClass(h, windows.BELOW_NORMAL_PRIORITY_CLASS); err != nil {
		return err
	}
	s := powerThrottlingState{processPowerThrottlingVersion, processPowerThrottlingExecSpeed, processPowerThrottlingExecSpeed}
	if r, _, err := procSetProcessInformation.Call(uintptr(h), processPowerThrottling,
		uintptr(unsafe.Pointer(&s)), unsafe.Sizeof(s)); r == 0 {
		return err
	}
	return nil
}

// ProcessName returns the executable name of a process id, in lower case,
// or "unknown" when the process is gone or cannot be opened.
func ProcessName(pid uint32) string {
	if pid == 0 {
		return "system"
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "unknown"
	}
	defer windows.CloseHandle(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return "unknown"
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

// SingleInstance takes a session-wide lock. The returned function releases it.
func SingleInstance(name string) (func(), error) {
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, p)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		return nil, err
	}
	return func() { windows.CloseHandle(h) }, nil
}
