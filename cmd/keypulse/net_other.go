//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
)

// NetState says whether the network collector is installed and running.
type NetState struct {
	Installed bool
	Running   bool
}

func netState() NetState { return NetState{} }

func netToggle(on bool) error { return errors.New("network statistics are Windows only") }

func netCommand(cmd string, argv []string) (bool, int) {
	switch cmd {
	case "net", "net-check", "net-service", "net-install", "net-uninstall":
		fmt.Fprintln(os.Stderr, "network statistics are Windows only")
		return true, 1
	}
	return false, 0
}
