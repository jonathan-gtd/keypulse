//go:build !windows

package tray

// Options configures the icon and its menu.
type Options struct {
	Tooltip     string
	StatsURL    string
	OnQuit      func()
	NetEnabled  func() bool
	OnToggleNet func(on bool) error
}

// Run is a no-op outside Windows: it returns at once, so the caller falls
// back to waiting for a signal.
func Run(opt Options, ready func()) {
	if ready != nil {
		ready()
	}
}

// Quit is a no-op outside Windows.
func Quit() {}

// OpenURL is a no-op outside Windows.
func OpenURL(url string) {}
