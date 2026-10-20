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
			"my game.exe": {
				Keys: 12, Clicks: [stats.NumButtons]uint64{3, 1}, DoubleClicks: 1, WheelUp: 2.5,
				MouseDistance: 1234.56, CursorMeters: 0.123456, ActiveSeconds: 40, ForegroundSeconds: 60,
				KeyHoldMillis: 900.25, KeyHoldCount: 12, ClickHoldMillis: 340, ClickHoldCount: 4,
			},
		},
		Keys:            map[string]uint64{"SPACE": 5, "A": 7},
		Shortcuts:       map[string]uint64{"CTRL+C": 2},
		Heat:            map[stats.Cell]uint64{{Kind: stats.Click, X: 4, Y: 2}: 1, {Kind: stats.Move, X: 10, Y: 0}: 9},
		TypingPeak1s:    7,
		TypingPeak10s:   31,
		IdleSeconds:     12,
		HandLeft:        9,
		HandRight:       3,
		CapturedSeconds: 42,
		CursorPeak:      1.25,
		Categories:      [stats.NumCategories]uint64{stats.CatLetter: 7, stats.CatDigit: 2},
		Rhythm:          [stats.NumRhythm]uint64{0: 4, 5: 1},
		AppSwitches:     3,
	}
	got := string(Encode(Options{Host: "mars", MouseDPI: 1600}, s))
	want := strings.Join([]string{
		`keypulse,host=mars,app=my\ game.exe keys=12i,clicks=4i,clicks_left=3i,clicks_right=1i,clicks_middle=0i,clicks_x1=0i,clicks_x2=0i,double_clicks=1i,wheel_up=2.5,wheel_down=0,wheel_left=0,wheel_right=0,mouse_distance=1234.6,mouse_m=0.02,cursor_m=0.1235,active_s=40i,foreground_s=60i,key_hold_ms_sum=900.3,key_hold_n=12i,click_hold_ms_sum=340,click_hold_n=4i 1700000000000000000`,
		`keypulse_session,host=mars typing_peak_1s=7i,typing_peak_10s=31i,idle_s=12i,app_switches=3i,clipboard_copies=0i,locks=0i,unlocks=0i,hand_left=9i,hand_right=3i,captured_s=42i,cursor_peak_mps=1.25 1700000000000000000`,
		`keypulse_keycat,host=mars,category=letter count=7i 1700000000000000000`,
		`keypulse_keycat,host=mars,category=digit count=2i 1700000000000000000`,
		`keypulse_rhythm,host=mars,bucket=<50ms count=4i,rank=0i 1700000000000000000`,
		`keypulse_rhythm,host=mars,bucket=>1s count=1i,rank=5i 1700000000000000000`,
		`keypulse_keys,host=mars,key=A count=7i 1700000000000000000`,
		`keypulse_keys,host=mars,key=SPACE count=5i 1700000000000000000`,
		`keypulse_shortcuts,host=mars,combo=CTRL+C count=2i 1700000000000000000`,
		`keypulse_heatmap,host=mars,kind=move,x=10,y=0 count=9i 1700000000000000000`,
		`keypulse_heatmap,host=mars,kind=click,x=4,y=2 count=1i 1700000000000000000`,
		``,
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestEncodeNoHostNoApps(t *testing.T) {
	s := stats.Snapshot{End: time.Unix(1, 0), Keys: map[string]uint64{"A": 1}}
	got := string(Encode(Options{Measurement: "input"}, s))
	want := "input_session typing_peak_1s=0i,typing_peak_10s=0i,idle_s=0i,app_switches=0i,clipboard_copies=0i,locks=0i,unlocks=0i,hand_left=0i,hand_right=0i,captured_s=0i,cursor_peak_mps=0 1000000000\n" +
		"input_keys,key=A count=1i 1000000000\n"
	if got != want {
		t.Errorf("got %q", got)
	}
}
