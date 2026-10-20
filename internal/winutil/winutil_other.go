//go:build !windows

package winutil

import "errors"

// ErrAlreadyRunning is returned by SingleInstance when another instance holds the lock.
var ErrAlreadyRunning = errors.New("keypulse is already running")

// LowerPriority is a no-op outside Windows.
func LowerPriority() error { return nil }

// SetDPIAware is a no-op outside Windows.
func SetDPIAware() error { return nil }

// SingleInstance is a no-op outside Windows.
func SingleInstance(name string) (func(), error) { return func() {}, nil }
