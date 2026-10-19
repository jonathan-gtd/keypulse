//go:build !windows

package input

import (
	"context"
	"errors"

	"github.com/jonathan-gtd/keypulse/internal/stats"
)

// Run is only implemented on Windows.
func Run(ctx context.Context, c *stats.Counter, opt Options) error {
	return errors.New("keypulse captures input on Windows only")
}
