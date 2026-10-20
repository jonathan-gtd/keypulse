//go:build windows

// Package tray shows keypulse in the notification area, so it is never
// running invisibly.
package tray

import (
	_ "embed"
	"os/exec"
	"syscall"

	"fyne.io/systray"
)

//go:embed keypulse.ico
var icon []byte

// Options configures the icon and its menu.
type Options struct {
	Tooltip  string
	StatsURL string // empty when the local page is disabled
	OnQuit   func() // called when the user picks Quit, before Run returns
}

// Run shows the icon and blocks until Quit. It must be called on the main
// goroutine. ready runs once the icon is up.
func Run(opt Options, ready func()) {
	systray.Run(func() {
		systray.SetIcon(icon)
		systray.SetTitle("keypulse")
		if opt.Tooltip != "" {
			systray.SetTooltip(opt.Tooltip)
		}
		if opt.StatsURL != "" {
			open := systray.AddMenuItem("Open statistics", opt.StatsURL)
			go func() {
				for range open.ClickedCh {
					OpenURL(opt.StatsURL)
				}
			}()
			systray.AddSeparator()
		}
		quit := systray.AddMenuItem("Quit keypulse", "Stop counting and exit")
		go func() {
			<-quit.ClickedCh
			systray.Quit()
		}()
		if ready != nil {
			ready()
		}
	}, opt.OnQuit)
}

// Quit removes the icon and makes Run return.
func Quit() { systray.Quit() }

// OpenURL opens a link in the default browser.
func OpenURL(url string) {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Start()
}
