// Package stats accumulates input events between two reports. It records
// counts and durations only — never which text is typed, nor the order of
// keys. Per-key counts are opt-in.
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

// Category groups keys by what they are for. It says nothing about which
// key was pressed beyond its kind.
type Category uint8

const (
	CatLetter Category = iota
	CatDigit
	CatPunctuation
	CatNavigation
	CatFunction
	CatModifier
	CatNumpad
	CatMedia
	CatOther
	NumCategories
)

// CategoryNames are the tag values used in reports, indexed by Category.
var CategoryNames = [NumCategories]string{
	"letter", "digit", "punctuation", "navigation",
	"function", "modifier", "numpad", "media", "other",
}

// Hand is the side of the keyboard a key sits on, from its physical
// position, so it does not depend on the keyboard layout.
type Hand uint8

const (
	HandNone Hand = iota // keys that belong to neither side
	HandLeft
	HandRight
)

// NumRhythm is the number of buckets in the typing-rhythm histogram.
const NumRhythm = 6

// RhythmNames label the time between two key presses. Only the gaps are
// counted, never which keys they were between.
var RhythmNames = [NumRhythm]string{"<50ms", "50-100ms", "100-200ms", "200-500ms", "500ms-1s", ">1s"}

// rhythmBucket places a gap between two key presses.
func rhythmBucket(d time.Duration) int {
	switch {
	case d < 50*time.Millisecond:
		return 0
	case d < 100*time.Millisecond:
		return 1
	case d < 200*time.Millisecond:
		return 2
	case d < 500*time.Millisecond:
		return 3
	case d < time.Second:
		return 4
	default:
		return 5
	}
}

// rhythmMaxGap is the longest gap still counted as typing: past it the
// person simply stopped, which says nothing about their rhythm.
const rhythmMaxGap = 10 * time.Second

// App holds what happened while one application was in the foreground.
type App struct {
	Keys   uint64
	Clicks [NumButtons]uint64

	DoubleClicks uint64

	// Wheel movement in notches. High-resolution wheels produce fractions.
	WheelUp, WheelDown, WheelLeft, WheelRight float64

	// MouseDistance is in sensor counts (pixels for absolute devices).
	MouseDistance float64

	// CursorMeters is the cursor travel on screen, in meters.
	CursorMeters float64

	// ActiveSeconds is the number of distinct seconds with at least one input.
	ActiveSeconds uint64

	// ForegroundSeconds is the time the app was in the foreground, input or not.
	ForegroundSeconds uint64

	// Key and click hold times: total duration and number of presses, so an
	// average can be computed without keeping each press.
	KeyHoldMillis   float64
	KeyHoldCount    uint64
	ClickHoldMillis float64
	ClickHoldCount  uint64
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
	Shortcuts  map[string]uint64 // by combo name, e.g. "CTRL+C"
	Heat       map[Cell]uint64   // nil unless the heatmap is on

	Categories [NumCategories]uint64 // key presses by kind of key
	Rhythm     [NumRhythm]uint64     // gaps between two key presses

	// Session-wide, not tied to one application.
	HandLeft        uint64  // keys struck on the left half of the keyboard
	HandRight       uint64  // ... and on the right half
	CursorPeak      float64 // fastest the cursor moved, in meters per second
	CapturedSeconds uint64  // seconds with the pointer confined, i.e. in a game
	TypingPeak1s    uint64  // most keys in any one-second window
	TypingPeak10s   uint64  // most keys in any ten-second window
	IdleSeconds     uint64  // seconds with no input at all
	AppSwitches     uint64  // foreground changes
	ClipboardCopies uint64  // clipboard updates (content never read)
	Locks           uint64  // session locks
	Unlocks         uint64  // session unlocks
}

// Empty reports whether nothing worth sending happened during the period.
func (s Snapshot) Empty() bool {
	return len(s.Apps) == 0 && len(s.Keys) == 0 && len(s.Shortcuts) == 0 && len(s.Heat) == 0 &&
		s.IdleSeconds == 0 && s.AppSwitches == 0 && s.ClipboardCopies == 0 && s.Locks == 0 && s.Unlocks == 0
}

type appAcc struct {
	keys         uint64
	clicks       [NumButtons]uint64
	doubleClicks uint64
	wheel        [4]int64 // up, down, right, left, in raw delta units
	distance     float64
	cursor       float64
	active       uint64
	lastActive   int64
	foreground   uint64
	keyHoldMs    float64
	keyHoldN     uint64
	clickHoldMs  float64
	clickHoldN   uint64
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

	shortcuts map[string]uint64
	switches  uint64
	clipboard uint64
	locks     uint64
	unlocks   uint64

	categories [NumCategories]uint64
	rhythm     [NumRhythm]uint64
	handLeft   uint64
	handRight  uint64
	lastKeyAt  time.Time
	cursorPeak float64
	captured   uint64

	// Typing peaks, on tumbling windows.
	sec1Start, sec1Count   uint64
	sec10Start, sec10Count uint64
	peak1s, peak10s        uint64

	// Idle: a second with no input increments idle on the next tick.
	inputThisSecond bool
	idle            uint64
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
	c.shortcuts = make(map[string]uint64)
	c.switches, c.clipboard, c.locks, c.unlocks = 0, 0, 0, 0
	c.peak1s, c.peak10s, c.idle = 0, 0, 0
	c.sec1Count, c.sec10Count = 0, 0
	c.categories = [NumCategories]uint64{}
	c.rhythm = [NumRhythm]uint64{}
	c.handLeft, c.handRight = 0, 0
	c.cursorPeak = 0
	c.captured = 0
	if c.perKey {
		c.keys = make(map[string]uint64)
	}
	if c.heat {
		c.cells = make(map[Cell]uint64)
	}
}

// accRaw returns the current application's accumulator, creating it if needed,
// without marking activity. Callers hold c.mu.
func (c *Counter) accRaw() *appAcc {
	if c.cur == nil {
		c.cur = c.apps[c.app]
		if c.cur == nil {
			c.cur = &appAcc{}
			c.apps[c.app] = c.cur
		}
	}
	return c.cur
}

// acc returns the current accumulator and records that an input happened in
// the current second. Callers hold c.mu.
func (c *Counter) acc() *appAcc {
	a := c.accRaw()
	c.inputThisSecond = true
	if s := c.now().Unix(); s != a.lastActive {
		a.lastActive = s
		a.active++
	}
	return a
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
		c.switches++
	}
}

// Key records one key press: its kind, which side of the keyboard it sits
// on, and — only when per-key counting is on — its name. The gap since the
// previous press feeds the rhythm histogram; the gap itself is counted, not
// the pair of keys it separated.
func (c *Counter) Key(name string, cat Category, hand Hand) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acc().keys++
	if c.perKey {
		c.keys[name]++
	}
	if cat < NumCategories {
		c.categories[cat]++
	}
	switch hand {
	case HandLeft:
		c.handLeft++
	case HandRight:
		c.handRight++
	}
	now := c.now()
	if !c.lastKeyAt.IsZero() {
		if gap := now.Sub(c.lastKeyAt); gap >= 0 && gap <= rhythmMaxGap {
			c.rhythm[rhythmBucket(gap)]++
		}
	}
	c.lastKeyAt = now
	c.bumpPeaks()
}

// CursorSpeed records how fast the cursor just moved, in meters per second,
// keeping the fastest of the period.
func (c *Counter) CursorSpeed(mps float64) {
	if mps <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if mps > c.cursorPeak {
		c.cursorPeak = mps
	}
}

func (c *Counter) bumpPeaks() {
	u := uint64(c.now().Unix())
	if u != c.sec1Start {
		c.sec1Start, c.sec1Count = u, 0
	}
	c.sec1Count++
	if c.sec1Count > c.peak1s {
		c.peak1s = c.sec1Count
	}
	w := u - u%10
	if w != c.sec10Start {
		c.sec10Start, c.sec10Count = w, 0
	}
	c.sec10Count++
	if c.sec10Count > c.peak10s {
		c.peak10s = c.sec10Count
	}
}

// KeyHold records how long a key was held, in milliseconds.
func (c *Counter) KeyHold(ms float64) {
	if ms <= 0 || ms > 10000 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.accRaw()
	a.keyHoldMs += ms
	a.keyHoldN++
}

// Shortcut records one modifier combination, e.g. "CTRL+C".
func (c *Counter) Shortcut(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputThisSecond = true
	if len(c.shortcuts) < 4096 {
		c.shortcuts[name]++
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

// DoubleClick records one double-click.
func (c *Counter) DoubleClick() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accRaw().doubleClicks++
}

// ClickHold records how long a mouse button was held, in milliseconds.
func (c *Counter) ClickHold(ms float64) {
	if ms <= 0 || ms > 60000 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	a := c.accRaw()
	a.clickHoldMs += ms
	a.clickHoldN++
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

// Move records a mouse movement of distance d, in sensor counts.
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
	c.accRaw().cursor += m
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

// Clipboard records one clipboard update. The content is never read.
func (c *Counter) Clipboard() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clipboard++
}

// Lock and Unlock record session lock and unlock.
func (c *Counter) Lock()   { c.mu.Lock(); c.locks++; c.mu.Unlock() }
func (c *Counter) Unlock() { c.mu.Lock(); c.unlocks++; c.mu.Unlock() }

// Tick is called once a second: it adds a foreground second to the current
// application and, if nothing was pressed in the past second, an idle
// second. captured says whether the pointer was confined to part of the
// screen, which is what a game does when it takes the mouse.
func (c *Counter) Tick(captured bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accRaw().foreground++
	if !c.inputThisSecond {
		c.idle++
	}
	if captured {
		c.captured++
	}
	c.inputThisSecond = false
}

// Snapshot returns what was counted since the previous snapshot and starts
// a new period at end.
func (c *Counter) Snapshot(end time.Time) Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := Snapshot{
		Start: c.start, End: end, Keys: c.keys, Shortcuts: c.shortcuts, Heat: c.cells,
		TypingPeak1s: c.peak1s, TypingPeak10s: c.peak10s, IdleSeconds: c.idle,
		AppSwitches: c.switches, ClipboardCopies: c.clipboard, Locks: c.locks, Unlocks: c.unlocks,
		Categories: c.categories, Rhythm: c.rhythm,
		HandLeft: c.handLeft, HandRight: c.handRight,
		CursorPeak: c.cursorPeak, CapturedSeconds: c.captured,
	}
	if len(c.apps) > 0 {
		s.Apps = make(map[string]App, len(c.apps))
	}
	for name, a := range c.apps {
		s.Apps[name] = App{
			Keys:              a.keys,
			Clicks:            a.clicks,
			DoubleClicks:      a.doubleClicks,
			WheelUp:           float64(a.wheel[0]) / wheelNotch,
			WheelDown:         float64(a.wheel[1]) / wheelNotch,
			WheelRight:        float64(a.wheel[2]) / wheelNotch,
			WheelLeft:         float64(a.wheel[3]) / wheelNotch,
			MouseDistance:     a.distance,
			CursorMeters:      a.cursor,
			ActiveSeconds:     a.active,
			ForegroundSeconds: a.foreground,
			KeyHoldMillis:     a.keyHoldMs,
			KeyHoldCount:      a.keyHoldN,
			ClickHoldMillis:   a.clickHoldMs,
			ClickHoldCount:    a.clickHoldN,
		}
	}
	c.reset(end)
	return s
}

// Session is a running total since keypulse started, for the local UI.
type Session struct {
	Since           time.Time
	Keys            uint64
	Clicks          uint64
	DoubleClicks    uint64
	WheelNotches    float64
	MouseMeters     float64
	ActiveSeconds   uint64
	IdleSeconds     uint64
	Shortcuts       uint64
	ClipboardCopies uint64
	AppSwitches     uint64
	TypingPeak1s    uint64
	HandLeft        uint64
	HandRight       uint64
	CursorPeak      float64
	CapturedSeconds uint64
	TopApps         []AppTotal
}

// AppTotal is one application's running total.
type AppTotal struct {
	App           string
	Keys          uint64
	Clicks        uint64
	ActiveSeconds uint64
}

// Add folds a snapshot into the session totals (for the local UI only).
func (s *Session) Add(snap Snapshot) {
	byApp := map[string]*AppTotal{}
	for _, a := range s.TopApps {
		t := a
		byApp[a.App] = &t
	}
	for name, a := range snap.Apps {
		s.Keys += a.Keys
		s.Clicks += a.TotalClicks()
		s.DoubleClicks += a.DoubleClicks
		s.WheelNotches += a.WheelUp + a.WheelDown + a.WheelLeft + a.WheelRight
		s.MouseMeters += a.CursorMeters
		s.ActiveSeconds += a.ActiveSeconds
		t := byApp[name]
		if t == nil {
			t = &AppTotal{App: name}
			byApp[name] = t
		}
		t.Keys += a.Keys
		t.Clicks += a.TotalClicks()
		t.ActiveSeconds += a.ActiveSeconds
	}
	for _, n := range snap.Shortcuts {
		s.Shortcuts += n
	}
	s.IdleSeconds += snap.IdleSeconds
	s.ClipboardCopies += snap.ClipboardCopies
	s.AppSwitches += snap.AppSwitches
	s.HandLeft += snap.HandLeft
	s.HandRight += snap.HandRight
	s.CapturedSeconds += snap.CapturedSeconds
	if snap.TypingPeak1s > s.TypingPeak1s {
		s.TypingPeak1s = snap.TypingPeak1s
	}
	if snap.CursorPeak > s.CursorPeak {
		s.CursorPeak = snap.CursorPeak
	}
	s.TopApps = s.TopApps[:0]
	for _, t := range byApp {
		s.TopApps = append(s.TopApps, *t)
	}
}
