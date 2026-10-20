// Package stats accumulates input events between two reports.
package stats

import (
	"sync"
	"time"
)

// Button is a mouse button.
type Button int

const (
	Left Button = iota
	Right
	Middle
	X1
	X2
	NumButtons
)

// ButtonNames are the names used in reports, indexed by Button.
var ButtonNames = [NumButtons]string{"left", "right", "middle", "x1", "x2"}

// wheelNotch is the wheel delta of one detent (WHEEL_DELTA).
const wheelNotch = 120

// UnknownApp is used when the foreground application cannot be determined.
const UnknownApp = "unknown"

// HeatKind tells what a heatmap cell counts.
type HeatKind uint8

const (
	Move  HeatKind = iota // cursor samples while the mouse moves
	Click                 // clicks, any button
)

func (k HeatKind) String() string {
	if k == Click {
		return "click"
	}
	return "move"
}

// Cell is a heatmap cell, in cell units from the top-left corner of the virtual desktop.
type Cell struct {
	Kind HeatKind
	X, Y int32
}

// App holds what happened while one application was in the foreground.
type App struct {
	Keys   uint64
	Clicks [NumButtons]uint64

	// Wheel movement in notches. High-resolution wheels produce fractions.
	WheelUp, WheelDown, WheelLeft, WheelRight float64

	// MouseDistance is in sensor counts (pixels for absolute devices such as
	// tablets or remote desktop sessions).
	MouseDistance float64

	// CursorMeters is the distance travelled by the cursor on screen, in
	// meters, from the physical size of the monitors.
	CursorMeters float64

	// ActiveSeconds is the number of distinct seconds with at least one input.
	ActiveSeconds uint64
}

// TotalClicks returns the number of clicks across all buttons.
func (a App) TotalClicks() uint64 {
	var n uint64
	for _, c := range a.Clicks {
		n += c
	}
	return n
}

// Snapshot holds what happened between Start and End.
type Snapshot struct {
	Start, End time.Time
	Apps       map[string]App    // by foreground application
	Keys       map[string]uint64 // by key name; nil unless per-key counting is on
	Heat       map[Cell]uint64   // nil unless the heatmap is on
}

// Empty reports whether nothing happened during the period.
func (s Snapshot) Empty() bool {
	return len(s.Apps) == 0 && len(s.Keys) == 0 && len(s.Heat) == 0
}

type appAcc struct {
	keys       uint64
	clicks     [NumButtons]uint64
	wheel      [4]int64 // up, down, right, left, in raw delta units
	distance   float64
	cursor     float64
	active     uint64
	lastActive int64
}

// Counter accumulates events. It is safe for concurrent use.
type Counter struct {
	mu     sync.Mutex
	now    func() time.Time
	perKey bool
	heat   bool

	app   string
	cur   *appAcc
	start time.Time
	apps  map[string]*appAcc
	keys  map[string]uint64
	cells map[Cell]uint64
}

// NewCounter returns a counter. perKey also counts every key by name;
// heat enables the heatmap.
func NewCounter(perKey, heat bool) *Counter {
	return newCounter(perKey, heat, time.Now)
}

func newCounter(perKey, heat bool, now func() time.Time) *Counter {
	c := &Counter{perKey: perKey, heat: heat, now: now, app: UnknownApp}
	c.reset(now())
	return c
}

func (c *Counter) reset(start time.Time) {
	c.start = start
	c.apps = make(map[string]*appAcc)
	c.cur = nil
	if c.perKey {
		c.keys = make(map[string]uint64)
	}
	if c.heat {
		c.cells = make(map[Cell]uint64)
	}
}

// SetApp sets the application that receives the following events.
func (c *Counter) SetApp(name string) {
	if name == "" {
		name = UnknownApp
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if name != c.app {
		c.app, c.cur = name, nil
	}
}

// acc returns the accumulator of the current application and records
// activity for the current second. Callers hold c.mu.
func (c *Counter) acc() *appAcc {
	if c.cur == nil {
		c.cur = c.apps[c.app]
		if c.cur == nil {
			c.cur = &appAcc{}
			c.apps[c.app] = c.cur
		}
	}
	if s := c.now().Unix(); s != c.cur.lastActive {
		c.cur.lastActive = s
		c.cur.active++
	}
	return c.cur
}

// Key records one key press.
func (c *Counter) Key(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acc().keys++
	if c.perKey {
		c.keys[name]++
	}
}

// Click records one press of button b.
func (c *Counter) Click(b Button) {
	if b < 0 || b >= NumButtons {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acc().clicks[b]++
}

// Wheel records a wheel movement. A positive delta is up (or right when horizontal).
func (c *Counter) Wheel(delta int, horizontal bool) {
	if delta == 0 {
		return
	}
	i := 0
	if delta < 0 {
		i, delta = 1, -delta
	}
	if horizontal {
		i += 2
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acc().wheel[i] += int64(delta)
}

// Move records a mouse movement of distance d.
func (c *Counter) Move(d float64) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acc().distance += d
}

// Cursor records an on-screen cursor movement of m meters.
func (c *Counter) Cursor(m float64) {
	if m <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acc().cursor += m
}

// Heat adds one sample to a heatmap cell.
func (c *Counter) Heat(cell Cell) {
	if !c.heat {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cells[cell]++
}

// Snapshot returns what was counted since the previous snapshot and starts
// a new period at end.
func (c *Counter) Snapshot(end time.Time) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := Snapshot{Start: c.start, End: end, Keys: c.keys, Heat: c.cells}
	if len(c.apps) > 0 {
		s.Apps = make(map[string]App, len(c.apps))
	}
	for name, a := range c.apps {
		s.Apps[name] = App{
			Keys:          a.keys,
			Clicks:        a.clicks,
			WheelUp:       float64(a.wheel[0]) / wheelNotch,
			WheelDown:     float64(a.wheel[1]) / wheelNotch,
			WheelRight:    float64(a.wheel[2]) / wheelNotch,
			WheelLeft:     float64(a.wheel[3]) / wheelNotch,
			MouseDistance: a.distance,
			CursorMeters:  a.cursor,
			ActiveSeconds: a.active,
		}
	}
	c.reset(end)
	return s
}
