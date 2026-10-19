package lineproto

import (
	"strings"
	"testing"
	"time"

	"github.com/jonathan-gtd/keypulse/internal/stats"
)

func TestEncode(t *testing.T) {
	s := stats.Snapshot{
		End: time.Unix(1700000000, 0),
		Apps: map[string]stats.App{
			"my game.exe": {Keys: 12, Clicks: [stats.NumButtons]uint64{3, 1}, WheelUp: 2.5, MouseDistance: 1234.56, ActiveSeconds: 40},
		},
		Keys: map[string]uint64{"SPACE": 5, "A": 7},
		Heat: map[stats.Cell]uint64{{Kind: stats.Click, X: 4, Y: 2}: 1, {Kind: stats.Move, X: 10, Y: 0}: 9},
	}
	got := string(Encode(Options{Host: "mars", MouseDPI: 1600}, s))
	want := strings.Join([]string{
		`keypulse,host=mars,app=my\ game.exe keys=12i,clicks=4i,clicks_left=3i,clicks_right=1i,clicks_middle=0i,clicks_x1=0i,clicks_x2=0i,wheel_up=2.5,wheel_down=0,wheel_left=0,wheel_right=0,mouse_distance=1234.6,mouse_m=0.02,active_s=40i 1700000000000000000`,
		`keypulse_keys,host=mars,key=A count=7i 1700000000000000000`,
		`keypulse_keys,host=mars,key=SPACE count=5i 1700000000000000000`,
		`keypulse_heatmap,host=mars,kind=move,x=10,y=0 count=9i 1700000000000000000`,
		`keypulse_heatmap,host=mars,kind=click,x=4,y=2 count=1i 1700000000000000000`,
		``,
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestEncodeEmptyAndNoHost(t *testing.T) {
	if got := Encode(Options{}, stats.Snapshot{End: time.Unix(1, 0)}); len(got) != 0 {
		t.Errorf("empty snapshot encoded as %q", got)
	}
	s := stats.Snapshot{End: time.Unix(1, 0), Keys: map[string]uint64{"A": 1}}
	if got := string(Encode(Options{Measurement: "input"}, s)); got != "input_keys,key=A count=1i 1000000000\n" {
		t.Errorf("got %q", got)
	}
}
