//go:build !windows

// Command keypulse-net reports per-application network traffic. It reads the
// Windows kernel network provider, so it only builds and runs on Windows.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "keypulse-net runs on Windows only")
	os.Exit(1)
}
