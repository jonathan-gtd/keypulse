package stats

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time { return f.t }

func TestCounterSnapshot(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	c := newCounter(true, true, clk.now)

	c.Key("A", CatLetter, HandLeft) // before any SetApp: unknown
	c.SetApp("game.exe")
	c.Key("A", CatLetter, HandLeft)
	c.Key("SPACE", CatPunctuation, HandNone)
	c.KeyHold(80)
	c.KeyHold(99999) // out of range, ignored
	c.Shortcut("CTRL+C")
	c.Shortcut("CTRL+C")
	c.Click(Left)
	c.DoubleClick()
	c.ClickHold(120)
	c.Click(Button(42)) // ignored
	clk.t = clk.t.Add(1500 * time.Millisecond)
	c.Key("B", CatLetter, HandRight) // a second later: same ten-second window, new one-second window
	c.Key("C", CatDigit, HandRight)
	c.Wheel(240, false)
	c.Wheel(-60, false)
	c.Wheel(120, true)
	c.Wheel(-120, true)
	c.Move(3)
	c.Move(4)
	c.Move(-1) // ignored
	c.Cursor(0.25)
	c.Cursor(0.5)
	c.CursorSpeed(1.0)
	c.CursorSpeed(2.5)
	c.CursorSpeed(0.8) // slower, so the peak stays 2.5
	c.Tick(false)      // input happened this second: no idle, +1 foreground
	c.Tick(true)       // nothing since: +1 idle
	c.SetApp("")
	c.Click(Right)
	c.Clipboard()
	c.Lock()
	c.Unlock()
	c.Heat(Cell{Move, 1, 2})
	c.Heat(Cell{Move, 1, 2})
	c.Heat(Cell{Click, 3, 4})

	end := time.Unix(1060, 0)
	s := c.Snapshot(end)

	if s.Keys["A"] != 2 || s.Keys["SPACE"] != 1 {
		t.Errorf("keys = %v", s.Keys)
	}
	if s.Shortcuts["CTRL+C"] != 2 {
		t.Errorf("shortcuts = %v", s.Shortcuts)
	}
	g := s.Apps["game.exe"]
	if g.Keys != 4 || g.Clicks[Left] != 1 || g.TotalClicks() != 1 || g.DoubleClicks != 1 {
		t.Errorf("game = %+v", g)
	}
	if g.KeyHoldMillis != 80 || g.KeyHoldCount != 1 || g.ClickHoldMillis != 120 || g.ClickHoldCount != 1 {
		t.Errorf("holds = %+v", g)
	}
	if g.WheelUp != 2 || g.WheelDown != 0.5 || g.WheelRight != 1 || g.WheelLeft != 1 {
		t.Errorf("wheel = %v %v %v %v", g.WheelUp, g.WheelDown, g.WheelRight, g.WheelLeft)
	}
	if g.MouseDistance != 7 || g.CursorMeters != 0.75 || g.ActiveSeconds != 2 || g.ForegroundSeconds != 2 {
		t.Errorf("game totals = %+v", g)
	}
	u := s.Apps[UnknownApp]
	if u.Keys != 1 || u.Clicks[Right] != 1 {
		t.Errorf("unknown = %+v", u)
	}
	// unknown -> game.exe -> unknown
	if s.AppSwitches != 2 || s.IdleSeconds != 1 || s.ClipboardCopies != 1 || s.Locks != 1 || s.Unlocks != 1 {
		t.Errorf("session = %+v", s)
	}
	if s.Categories[CatLetter] != 3 || s.Categories[CatDigit] != 1 || s.Categories[CatPunctuation] != 1 {
		t.Errorf("categories = %v", s.Categories)
	}
	if s.HandLeft != 2 || s.HandRight != 2 {
		t.Errorf("hands = %d / %d", s.HandLeft, s.HandRight)
	}
	// Five keys make four gaps: three within the same instant, one of 1.5 s.
	if s.Rhythm[0] != 3 || s.Rhythm[5] != 1 {
		t.Errorf("rhythm = %v", s.Rhythm)
	}
	if s.CursorPeak != 2.5 || s.CapturedSeconds != 1 {
		t.Errorf("peak = %v, captured = %d", s.CursorPeak, s.CapturedSeconds)
	}
	if s.TypingPeak1s != 3 || s.TypingPeak10s != 5 {
		t.Errorf("peaks = %d / %d", s.TypingPeak1s, s.TypingPeak10s)
	}
	if s.Heat[Cell{Move, 1, 2}] != 2 || s.Heat[Cell{Click, 3, 4}] != 1 {
		t.Errorf("heat = %v", s.Heat)
	}
	if !s.Start.Equal(time.Unix(1000, 0)) || !s.End.Equal(end) {
		t.Errorf("period = %v - %v", s.Start, s.End)
	}

	next := c.Snapshot(time.Unix(1120, 0))
	if !next.Empty() {
		t.Errorf("counter not reset: %+v", next)
	}
	if !next.Start.Equal(end) {
		t.Errorf("next start = %v, want %v", next.Start, end)
	}
}

func TestCounterOptionsOff(t *testing.T) {
	c := NewCounter(false, false)
	c.Key("A", CatLetter, HandLeft)
	c.Heat(Cell{Move, 0, 0})
	s := c.Snapshot(time.Now())
	if s.Keys != nil || s.Heat != nil || s.Apps[UnknownApp].Keys != 1 {
		t.Errorf("snapshot = %+v", s)
	}
}

func TestSessionAdd(t *testing.T) {
	var sess Session
	for i := 0; i < 2; i++ {
		sess.Add(Snapshot{
			Apps: map[string]App{
				"a.exe": {Keys: 10, Clicks: [NumButtons]uint64{2}, CursorMeters: 1.5, ActiveSeconds: 30},
				"b.exe": {Keys: 1, ActiveSeconds: 5},
			},
			Shortcuts:    map[string]uint64{"CTRL+C": 3},
			IdleSeconds:  20,
			TypingPeak1s: uint64(5 + i),
		})
	}
	if sess.Keys != 22 || sess.Clicks != 4 || sess.MouseMeters != 3 || sess.ActiveSeconds != 70 {
		t.Errorf("session = %+v", sess)
	}
	if sess.Shortcuts != 6 || sess.IdleSeconds != 40 || sess.TypingPeak1s != 6 {
		t.Errorf("session = %+v", sess)
	}
	if len(sess.TopApps) != 2 {
		t.Fatalf("apps = %+v", sess.TopApps)
	}
	for _, a := range sess.TopApps {
		if a.App == "a.exe" && (a.Keys != 20 || a.ActiveSeconds != 60) {
			t.Errorf("a.exe = %+v", a)
		}
	}
}
