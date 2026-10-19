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

	c.Key("A") // before any SetApp: unknown
	c.SetApp("game.exe")
	c.Key("A")
	c.Key("SPACE")
	c.Click(Left)
	c.Click(Button(42)) // ignored
	clk.t = clk.t.Add(1500 * time.Millisecond)
	c.Wheel(240, false)
	c.Wheel(-60, false)
	c.Wheel(120, true)
	c.Wheel(-120, true)
	c.Move(3)
	c.Move(4)
	c.Move(-1) // ignored
	c.SetApp("")
	c.Click(Right)
	c.Heat(Cell{Move, 1, 2})
	c.Heat(Cell{Move, 1, 2})
	c.Heat(Cell{Click, 3, 4})

	end := time.Unix(1060, 0)
	s := c.Snapshot(end)

	if s.Keys["A"] != 2 || s.Keys["SPACE"] != 1 {
		t.Errorf("keys = %v", s.Keys)
	}
	g := s.Apps["game.exe"]
	if g.Keys != 2 || g.Clicks[Left] != 1 || g.TotalClicks() != 1 {
		t.Errorf("game = %+v", g)
	}
	if g.WheelUp != 2 || g.WheelDown != 0.5 || g.WheelRight != 1 || g.WheelLeft != 1 {
		t.Errorf("wheel = %v %v %v %v", g.WheelUp, g.WheelDown, g.WheelRight, g.WheelLeft)
	}
	if g.MouseDistance != 7 || g.ActiveSeconds != 2 {
		t.Errorf("distance = %v, active = %d", g.MouseDistance, g.ActiveSeconds)
	}
	u := s.Apps[UnknownApp]
	if u.Keys != 1 || u.Clicks[Right] != 1 {
		t.Errorf("unknown = %+v", u)
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
	c.Key("A")
	c.Heat(Cell{Move, 0, 0})
	s := c.Snapshot(time.Now())
	if s.Keys != nil || s.Heat != nil || s.Apps[UnknownApp].Keys != 1 {
		t.Errorf("snapshot = %+v", s)
	}
}
