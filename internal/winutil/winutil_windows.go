//go:build windows

// Package winutil holds process-level Windows helpers.
package winutil

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning is returned by SingleInstance when another instance holds the lock.
var ErrAlreadyRunning = errors.New("keypulse is already running")

var procSetProcessInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetProcessInformation")

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
