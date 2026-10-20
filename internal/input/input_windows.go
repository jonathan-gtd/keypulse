//go:build windows

package input

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/jonathan-gtd/keypulse/internal/stats"
)

var (
	user32                      = windows.NewLazySystemDLL("user32.dll")
	kernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procGetMessageW             = user32.NewProc("GetMessageW")
	procDispatchMessageW        = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW      = user32.NewProc("PostThreadMessageW")
	procRegisterRawInputDevices = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData         = user32.NewProc("GetRawInputData")
	procGetSystemMetrics        = user32.NewProc("GetSystemMetrics")
	procGetCursorPos            = user32.NewProc("GetCursorPos")
	procSetTimer                = user32.NewProc("SetTimer")
	procSetWinEventHook         = user32.NewProc("SetWinEventHook")
	procUnhookWinEvent          = user32.NewProc("UnhookWinEvent")
	procGetClipCursor           = user32.NewProc("GetClipCursor")
	procMonitorFromPoint        = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW         = user32.NewProc("GetMonitorInfoW")
	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")

	procAddClipboardFormatListener    = user32.NewProc("AddClipboardFormatListener")
	procRemoveClipboardFormatListener = user32.NewProc("RemoveClipboardFormatListener")

	wtsapi32                             = windows.NewLazySystemDLL("wtsapi32.dll")
	procWTSRegisterSessionNotification   = wtsapi32.NewProc("WTSRegisterSessionNotification")
	procWTSUnRegisterSessionNotification = wtsapi32.NewProc("WTSUnRegisterSessionNotification")

	gdi32             = windows.NewLazySystemDLL("gdi32.dll")
	procCreateDCW     = gdi32.NewProc("CreateDCW")
	procGetDeviceCaps = gdi32.NewProc("GetDeviceCaps")
	procDeleteDC      = gdi32.NewProc("DeleteDC")
)

const (
	wmEndSession       = 0x0016
	wmQuit             = 0x0012
	wmDisplayChange    = 0x007E
	wmInput            = 0x00FF
	wmTimer            = 0x0113
	wmClipboardUpdate  = 0x031D
	wmWTSSessionChange = 0x02B1

	wtsSessionLock       = 0x7
	wtsSessionUnlock     = 0x8
	notifyForThisSession = 0

	wsPopup        = 0x80000000
	wsExToolWindow = 0x00000080

	ridInput        = 0x10000003
	rimTypeMouse    = 0
	rimTypeKeyboard = 1
	ridevInputSink  = 0x00000100

	riKeyBreak = 0x01
	riKeyE0    = 0x02

	mouseMoveAbsolute   = 0x01
	mouseVirtualDesktop = 0x02

	riMouseLeftDown   = 0x0001
	riMouseRightDown  = 0x0004
	riMouseMiddleDown = 0x0010
	riMouseX1Down     = 0x0040
	riMouseX2Down     = 0x0100
	riMouseWheel      = 0x0400
	riMouseHWheel     = 0x0800
	riMouseAnyDown    = riMouseLeftDown | riMouseRightDown | riMouseMiddleDown | riMouseX1Down | riMouseX2Down

	riMouseLeftUp   = 0x0002
	riMouseRightUp  = 0x0008
	riMouseMiddleUp = 0x0020
	riMouseX1Up     = 0x0080
	riMouseX2Up     = 0x0200
	riMouseAnyUp    = riMouseLeftUp | riMouseRightUp | riMouseMiddleUp | riMouseX1Up | riMouseX2Up

	smCxScreen        = 0
	smCyScreen        = 1
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCxVirtualScreen = 78
	smCyVirtualScreen = 79

	eventSystemForeground = 0x0003
	wineventOutOfContext  = 0x0000

	cursorTimerID = 1
	cursorTimerMs = 100
	tickTimerID   = 2
	tickTimerMs   = 1000

	// Two left clicks closer than this count as a double-click. Matches the
	// usual Windows default.
	doubleClickWindow = 400 * time.Millisecond

	// The cursor path is sampled at most this often: enough for its length,
	// and cheap with mice polling at several kHz.
	cursorTrackEvery = 8 * time.Millisecond

	monitorDefaultToNearest = 2
	horzSize                = 4 // GetDeviceCaps: width in millimeters
	horzRes                 = 8 // GetDeviceCaps: width in pixels
)

type rawInputDevice struct {
	usagePage uint16
	usage     uint16
	flags     uint32
	target    uintptr
}

type rawInputHeader struct {
	typ    uint32
	size   uint32
	device uintptr
	wParam uintptr
}

type rawMouse struct {
	flags       uint16
	_           uint16
	buttonFlags uint16
	buttonData  uint16
	rawButtons  uint32
	lastX       int32
	lastY       int32
	extra       uint32
}

type rawKeyboard struct {
	makeCode uint16
	flags    uint16
	reserved uint16
	vKey     uint16
	message  uint32
	extra    uint32
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type point struct{ x, y int32 }

type rect struct{ left, top, right, bottom int32 }

type monitorInfoEx struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
	device  [32]uint16
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

// Run reads keyboard and mouse input through the Raw Input API until ctx is
// done. Raw input is a copy of the events delivered to this process: unlike a
// low-level hook, it is not in the path between the devices and the
// foreground application, so it cannot add latency to games.
func Run(ctx context.Context, c *stats.Counter, opt Options) error {
	if opt.CellSize <= 0 {
		opt.CellSize = 40
	}
	l := &listener{c: c, opt: opt, apps: make(map[uint32]string), scales: make(map[uintptr]float64)}
	l.refreshScreen()
	errc := make(chan error, 1)
	tidc := make(chan uint32, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		errc <- l.loop(tidc)
	}()

	var tid uint32
	select {
	case tid = <-tidc:
	case err := <-errc:
		return err
	}
	select {
	case <-ctx.Done():
		procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
		<-errc
		return nil
	case err := <-errc:
		return err
	}
}

type listener struct {
	c   *stats.Counter
	opt Options

	down     [numpadEnter + 1]bool
	downAt   [numpadEnter + 1]time.Time
	btnDown  [stats.NumButtons]time.Time
	lastLeft time.Time

	absX, absY float64
	haveAbs    bool

	moved   bool
	lastPos point
	havePos bool

	curPos    point
	haveCur   bool
	lastTrack time.Time

	virtual rect                // virtual desktop, refreshed on WM_DISPLAYCHANGE
	scales  map[uintptr]float64 // monitor handle -> meters per pixel

	apps map[uint32]string // process id -> executable name
	buf  [16]uint64        // RAWINPUT buffer, 8-byte aligned
}

func (l *listener) loop(tidc chan<- uint32) error {
	inst, _, _ := procGetModuleHandleW.Call(0)
	className, err := windows.UTF16PtrFromString("keypulse")
	if err != nil {
		return err
	}
	wc := wndClassEx{wndProc: windows.NewCallback(l.wndProc), instance: windows.Handle(inst), className: className}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}
	// A hidden top-level window rather than a message-only one, so that it
	// also receives WM_ENDSESSION when the user signs out.
	hwnd, _, err := procCreateWindowExW.Call(wsExToolWindow, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)), wsPopup, 0, 0, 0, 0, 0, 0, inst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	defer procDestroyWindow.Call(hwnd)

	devices := [2]rawInputDevice{
		{usagePage: 0x01, usage: 0x02, flags: ridevInputSink, target: hwnd}, // mouse
		{usagePage: 0x01, usage: 0x06, flags: ridevInputSink, target: hwnd}, // keyboard
	}
	if r, _, err := procRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&devices[0])),
		uintptr(len(devices)), unsafe.Sizeof(devices[0])); r == 0 {
		return fmt.Errorf("RegisterRawInputDevices: %w", err)
	}

	if l.opt.PerApp {
		// Foreground changes are pushed by Windows: nothing is polled.
		hook, _, err := procSetWinEventHook.Call(eventSystemForeground, eventSystemForeground, 0,
			windows.NewCallback(l.winEvent), 0, 0, wineventOutOfContext)
		if hook == 0 {
			return fmt.Errorf("SetWinEventHook: %w", err)
		}
		defer procUnhookWinEvent.Call(hook)
		l.setForeground(windows.GetForegroundWindow())
	}
	if l.opt.Heatmap {
		if r, _, err := procSetTimer.Call(hwnd, cursorTimerID, cursorTimerMs, 0); r == 0 {
			return fmt.Errorf("SetTimer: %w", err)
		}
	}

	if r, _, err := procSetTimer.Call(hwnd, tickTimerID, tickTimerMs, 0); r == 0 {
		return fmt.Errorf("SetTimer: %w", err)
	}
	if l.opt.Clipboard {
		if r, _, err := procAddClipboardFormatListener.Call(hwnd); r == 0 {
			return fmt.Errorf("AddClipboardFormatListener: %w", err)
		}
		defer procRemoveClipboardFormatListener.Call(hwnd)
	}
	if l.opt.Session {
		if r, _, err := procWTSRegisterSessionNotification.Call(hwnd, notifyForThisSession); r == 0 {
			return fmt.Errorf("WTSRegisterSessionNotification: %w", err)
		}
		defer procWTSUnRegisterSessionNotification.Call(hwnd)
	}

	tidc <- windows.GetCurrentThreadId()
	var m msg
	for {
		r, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(r) {
		case -1:
			return fmt.Errorf("GetMessage: %w", err)
		case 0:
			return nil
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (l *listener) wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmInput:
		l.rawInput(lParam)
	case wmTimer:
		switch wParam {
		case cursorTimerID:
			l.sampleCursor()
		case tickTimerID:
			l.c.Tick(l.pointerLocked())
		}
		return 0
	case wmClipboardUpdate:
		// Only that the clipboard changed; its content is never read.
		l.c.Clipboard()
		return 0
	case wmWTSSessionChange:
		switch wParam {
		case wtsSessionLock:
			l.c.Lock()
		case wtsSessionUnlock:
			l.c.Unlock()
		}
		return 0
	case wmEndSession:
		if wParam != 0 && l.opt.OnEndSession != nil {
			l.opt.OnEndSession()
		}
		return 0
	case wmDisplayChange:
		l.refreshScreen()
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func (l *listener) rawInput(handle uintptr) {
	size := uint32(unsafe.Sizeof(l.buf))
	r, _, _ := procGetRawInputData.Call(handle, ridInput, uintptr(unsafe.Pointer(&l.buf[0])),
		uintptr(unsafe.Pointer(&size)), unsafe.Sizeof(rawInputHeader{}))
	if int32(r) <= 0 {
		return
	}
	hdr := (*rawInputHeader)(unsafe.Pointer(&l.buf[0]))
	data := unsafe.Add(unsafe.Pointer(&l.buf[0]), unsafe.Sizeof(rawInputHeader{}))
	switch hdr.typ {
	case rimTypeMouse:
		l.mouse((*rawMouse)(data))
	case rimTypeKeyboard:
		l.keyboard((*rawKeyboard)(data))
	}
}

func (l *listener) keyboard(k *rawKeyboard) {
	if k.vKey == 0 || k.vKey == 0xFF { // 0xFF: fake key, part of an escape sequence
		return
	}
	id := keyID(k.vKey, k.makeCode, k.flags&riKeyE0 != 0)
	if id < 0 || id >= len(l.down) {
		return
	}
	now := time.Now()
	if k.flags&riKeyBreak != 0 {
		if l.down[id] {
			l.c.KeyHold(float64(now.Sub(l.downAt[id]).Microseconds()) / 1000)
		}
		l.down[id] = false
		return
	}
	if l.down[id] { // auto-repeat while the key is held
		return
	}
	l.down[id], l.downAt[id] = true, now
	l.c.Key(keyNames[id], category(id), handFor(k.makeCode, k.flags&riKeyE0 != 0))
	if combo := shortcutName(id, func(k int) bool { return l.down[k] }); combo != "" {
		l.c.Shortcut(combo)
	}
}

// mouseButtons maps the raw input flags of each button.
var mouseButtons = []struct {
	down, up uint16
	button   stats.Button
}{
	{riMouseLeftDown, riMouseLeftUp, stats.Left},
	{riMouseRightDown, riMouseRightUp, stats.Right},
	{riMouseMiddleDown, riMouseMiddleUp, stats.Middle},
	{riMouseX1Down, riMouseX1Up, stats.X1},
	{riMouseX2Down, riMouseX2Up, stats.X2},
}

func (l *listener) mouse(m *rawMouse) {
	f := m.buttonFlags
	if f&(riMouseAnyDown|riMouseAnyUp) != 0 {
		now := time.Now()
		for _, b := range mouseButtons {
			switch {
			case f&b.down != 0:
				l.c.Click(b.button)
				l.btnDown[b.button] = now
				if b.button == stats.Left {
					if !l.lastLeft.IsZero() && now.Sub(l.lastLeft) <= doubleClickWindow {
						l.c.DoubleClick()
						l.lastLeft = time.Time{} // a triple click is not two doubles
					} else {
						l.lastLeft = now
					}
				}
			case f&b.up != 0:
				if t := l.btnDown[b.button]; !t.IsZero() {
					l.c.ClickHold(float64(now.Sub(t).Microseconds()) / 1000)
					l.btnDown[b.button] = time.Time{}
				}
			}
		}
	}
	if f&riMouseAnyDown != 0 {
		if l.opt.Heatmap {
			if p, ok := cursorPos(); ok {
				x, y := l.cell(p)
				l.c.Heat(stats.Cell{Kind: stats.Click, X: x, Y: y})
			}
		}
	}
	if f&riMouseWheel != 0 {
		l.c.Wheel(int(int16(m.buttonData)), false)
	}
	if f&riMouseHWheel != 0 {
		l.c.Wheel(int(int16(m.buttonData)), true)
	}

	if m.flags&mouseMoveAbsolute != 0 {
		// Tablets and remote desktop sessions report positions, normalized to 0..65535.
		var w, h float64
		if m.flags&mouseVirtualDesktop != 0 {
			w, h = float64(systemMetric(smCxVirtualScreen)), float64(systemMetric(smCyVirtualScreen))
		} else {
			w, h = float64(systemMetric(smCxScreen)), float64(systemMetric(smCyScreen))
		}
		x, y := float64(m.lastX)/65535*w, float64(m.lastY)/65535*h
		if l.haveAbs {
			l.c.Move(math.Hypot(x-l.absX, y-l.absY))
		}
		l.absX, l.absY, l.haveAbs = x, y, true
		l.moved = true
		l.trackCursor()
	} else if m.lastX != 0 || m.lastY != 0 {
		l.haveAbs = false
		l.c.Move(math.Hypot(float64(m.lastX), float64(m.lastY)))
		l.moved = true
		l.trackCursor()
	}
}

// trackCursor adds the on-screen distance covered by the cursor since the
// previous sample, converted to meters with the physical size of the monitor.
func (l *listener) trackCursor() {
	now := time.Now()
	since := now.Sub(l.lastTrack)
	if since < cursorTrackEvery {
		return
	}
	l.lastTrack = now
	p, ok := cursorPos()
	if !ok || l.pointerLocked() {
		l.haveCur = false
		return
	}
	if l.haveCur && p != l.curPos {
		d := math.Hypot(float64(p.x-l.curPos.x), float64(p.y-l.curPos.y))
		meters := d * l.metersPerPixel(p)
		l.c.Cursor(meters)
		// A gap longer than a second means the mouse was simply still in
		// between, so the average over it is not a speed worth keeping.
		if s := since.Seconds(); s > 0 && since <= time.Second {
			l.c.CursorSpeed(meters / s)
		}
	}
	l.curPos, l.haveCur = p, true
}

// pointerLocked reports whether the cursor is confined to part of the
// desktop, as games do when they capture the mouse. Its position then says
// nothing about where the user is pointing.
func (l *listener) pointerLocked() bool {
	var r rect
	if ok, _, _ := procGetClipCursor.Call(uintptr(unsafe.Pointer(&r))); ok == 0 {
		return false
	}
	return r.right-r.left < l.virtual.right-l.virtual.left || r.bottom-r.top < l.virtual.bottom-l.virtual.top
}

func (l *listener) refreshScreen() {
	x, y := systemMetric(smXVirtualScreen), systemMetric(smYVirtualScreen)
	l.virtual = rect{x, y, x + systemMetric(smCxVirtualScreen), y + systemMetric(smCyVirtualScreen)}
	clear(l.scales)
}

// metersPerPixel returns the pixel pitch of the monitor under p, from the
// physical size Windows reads in the monitor's EDID (96 DPI when unknown).
func (l *listener) metersPerPixel(p point) float64 {
	mon, _, _ := procMonitorFromPoint.Call(uintptr(uint32(p.x))|uintptr(uint32(p.y))<<32, monitorDefaultToNearest)
	if s, ok := l.scales[mon]; ok {
		return s
	}
	s := 0.0254 / 96
	info := monitorInfoEx{}
	info.size = uint32(unsafe.Sizeof(info))
	if r, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&info))); r != 0 {
		if dc, _, _ := procCreateDCW.Call(0, uintptr(unsafe.Pointer(&info.device[0])), 0, 0); dc != 0 {
			mm, _, _ := procGetDeviceCaps.Call(dc, horzSize)
			px, _, _ := procGetDeviceCaps.Call(dc, horzRes)
			procDeleteDC.Call(dc)
			if int32(mm) > 0 && int32(px) > 0 {
				s = float64(int32(mm)) / 1000 / float64(int32(px))
			}
		}
	}
	l.scales[mon] = s
	return s
}

// sampleCursor adds the cursor position to the heatmap, at most every
// cursorTimerMs and only after the mouse moved. A cursor that stays put
// despite movement (games that lock the pointer) is not counted.
func (l *listener) sampleCursor() {
	if !l.moved {
		return
	}
	l.moved = false
	p, ok := cursorPos()
	if !ok || (l.havePos && p == l.lastPos) || l.pointerLocked() {
		return
	}
	l.lastPos, l.havePos = p, true
	x, y := l.cell(p)
	l.c.Heat(stats.Cell{Kind: stats.Move, X: x, Y: y})
}

// cell converts screen coordinates to heatmap cells, counted from the
// top-left corner of the virtual desktop (monitors can sit at negative coordinates).
func (l *listener) cell(p point) (int32, int32) {
	cs := int32(l.opt.CellSize)
	return (p.x - l.virtual.left) / cs, (p.y - l.virtual.top) / cs
}

func (l *listener) winEvent(hook, event, hwnd, idObject, idChild, thread, eventTime uintptr) uintptr {
	l.setForeground(windows.HWND(hwnd))
	return 0
}

func (l *listener) setForeground(hwnd windows.HWND) {
	if hwnd == 0 {
		return
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil || pid == 0 {
		return
	}
	name, ok := l.apps[pid]
	if !ok {
		name = processName(pid)
		if len(l.apps) > 512 {
			clear(l.apps)
		}
		l.apps[pid] = name
	}
	l.c.SetApp(name)
}

func processName(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return stats.UnknownApp
	}
	defer windows.CloseHandle(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return stats.UnknownApp
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

func cursorPos() (point, bool) {
	var p point
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p, r != 0
}

func systemMetric(index uintptr) int32 {
	r, _, _ := procGetSystemMetrics.Call(index)
	return int32(r)
}
