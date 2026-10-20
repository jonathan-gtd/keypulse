// Package lineproto encodes snapshots in InfluxDB line protocol, the format
// Telegraf and InfluxDB read.
package lineproto

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/jonathan-gtd/keypulse/internal/stats"
)

// Options controls the encoding.
type Options struct {
	Measurement string  // prefix of the measurement names, "keypulse" by default
	Host        string  // host tag, omitted when empty
	MouseDPI    float64 // when set, mouse distance is also reported in meters
}

// Encode returns the snapshot as line protocol, one line per point:
//
//	keypulse,host=mars,app=game.exe keys=12i,clicks=3i,... <ts>
//	keypulse_session,host=mars idle_s=12i,typing_peak_1s=7i,... <ts>
//	keypulse_keys,host=mars,key=A count=4i <ts>
//	keypulse_shortcuts,host=mars,combo=CTRL+C count=2i <ts>
//	keypulse_heatmap,host=mars,kind=move,x=12,y=7 count=9i <ts>
func Encode(o Options, s stats.Snapshot) []byte {
	m := o.Measurement
	if m == "" {
		m = "keypulse"
	}
	ts := " " + strconv.FormatInt(s.End.UnixNano(), 10) + "\n"
	host := ""
	if o.Host != "" {
		host = ",host=" + escapeTag(o.Host)
	}
	var b strings.Builder

	for _, name := range sortedKeys(s.Apps) {
		a := s.Apps[name]
		b.WriteString(escapeMeasurement(m))
		b.WriteString(host)
		b.WriteString(",app=")
		b.WriteString(escapeTag(name))
		b.WriteByte(' ')
		writeInt(&b, "keys", a.Keys, true)
		writeInt(&b, "clicks", a.TotalClicks(), false)
		for i, n := range a.Clicks {
			writeInt(&b, "clicks_"+stats.ButtonNames[i], n, false)
		}
		writeInt(&b, "double_clicks", a.DoubleClicks, false)
		writeFloat(&b, "wheel_up", a.WheelUp)
		writeFloat(&b, "wheel_down", a.WheelDown)
		writeFloat(&b, "wheel_left", a.WheelLeft)
		writeFloat(&b, "wheel_right", a.WheelRight)
		writeFloat(&b, "mouse_distance", round(a.MouseDistance, 1))
		if o.MouseDPI > 0 {
			writeFloat(&b, "mouse_m", round(a.MouseDistance/o.MouseDPI*0.0254, 3))
		}
		writeFloat(&b, "cursor_m", round(a.CursorMeters, 4))
		writeInt(&b, "active_s", a.ActiveSeconds, false)
		writeInt(&b, "foreground_s", a.ForegroundSeconds, false)
		writeFloat(&b, "key_hold_ms_sum", round(a.KeyHoldMillis, 1))
		writeInt(&b, "key_hold_n", a.KeyHoldCount, false)
		writeFloat(&b, "click_hold_ms_sum", round(a.ClickHoldMillis, 1))
		writeInt(&b, "click_hold_n", a.ClickHoldCount, false)
		b.WriteString(ts)
	}

	b.WriteString(escapeMeasurement(m + "_session"))
	b.WriteString(host)
	b.WriteByte(' ')
	writeInt(&b, "typing_peak_1s", s.TypingPeak1s, true)
	writeInt(&b, "typing_peak_10s", s.TypingPeak10s, false)
	writeInt(&b, "idle_s", s.IdleSeconds, false)
	writeInt(&b, "app_switches", s.AppSwitches, false)
	writeInt(&b, "clipboard_copies", s.ClipboardCopies, false)
	writeInt(&b, "locks", s.Locks, false)
	writeInt(&b, "unlocks", s.Unlocks, false)
	writeInt(&b, "hand_left", s.HandLeft, false)
	writeInt(&b, "hand_right", s.HandRight, false)
	writeInt(&b, "captured_s", s.CapturedSeconds, false)
	writeFloat(&b, "cursor_peak_mps", round(s.CursorPeak, 3))
	b.WriteString(ts)

	for i, n := range s.Categories {
		if n == 0 {
			continue
		}
		b.WriteString(escapeMeasurement(m + "_keycat"))
		b.WriteString(host)
		b.WriteString(",category=")
		b.WriteString(stats.CategoryNames[i])
		b.WriteByte(' ')
		writeInt(&b, "count", n, true)
		b.WriteString(ts)
	}

	for i, n := range s.Rhythm {
		if n == 0 {
			continue
		}
		b.WriteString(escapeMeasurement(m + "_rhythm"))
		b.WriteString(host)
		b.WriteString(",bucket=")
		b.WriteString(escapeTag(stats.RhythmNames[i]))
		b.WriteByte(' ')
		writeInt(&b, "count", n, true)
		writeInt(&b, "rank", uint64(i), false)
		b.WriteString(ts)
	}

	for _, key := range sortedKeys(s.Keys) {
		b.WriteString(escapeMeasurement(m + "_keys"))
		b.WriteString(host)
		b.WriteString(",key=")
		b.WriteString(escapeTag(key))
		b.WriteByte(' ')
		writeInt(&b, "count", s.Keys[key], true)
		b.WriteString(ts)
	}

	for _, combo := range sortedKeys(s.Shortcuts) {
		b.WriteString(escapeMeasurement(m + "_shortcuts"))
		b.WriteString(host)
		b.WriteString(",combo=")
		b.WriteString(escapeTag(combo))
		b.WriteByte(' ')
		writeInt(&b, "count", s.Shortcuts[combo], true)
		b.WriteString(ts)
	}

	cells := make([]stats.Cell, 0, len(s.Heat))
	for c := range s.Heat {
		cells = append(cells, c)
	}
	sort.Slice(cells, func(i, j int) bool {
		a, c := cells[i], cells[j]
		if a.Kind != c.Kind {
			return a.Kind < c.Kind
		}
		if a.Y != c.Y {
			return a.Y < c.Y
		}
		return a.X < c.X
	})
	for _, c := range cells {
		b.WriteString(escapeMeasurement(m + "_heatmap"))
		b.WriteString(host)
		b.WriteString(",kind=")
		b.WriteString(c.Kind.String())
		b.WriteString(",x=")
		b.WriteString(strconv.Itoa(int(c.X)))
		b.WriteString(",y=")
		b.WriteString(strconv.Itoa(int(c.Y)))
		b.WriteByte(' ')
		writeInt(&b, "count", s.Heat[c], true)
		b.WriteString(ts)
	}
	return []byte(b.String())
}

func writeInt(b *strings.Builder, name string, v uint64, first bool) {
	if !first {
		b.WriteByte(',')
	}
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(strconv.FormatUint(v, 10))
	b.WriteByte('i')
}

func writeFloat(b *strings.Builder, name string, v float64) {
	b.WriteByte(',')
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
}

func round(v float64, decimals int) float64 {
	p := math.Pow10(decimals)
	return math.Round(v*p) / p
}

var (
	tagEscaper         = strings.NewReplacer(`\`, `\\`, ",", `\,`, "=", `\=`, " ", `\ `, "\n", `\n`)
	measurementEscaper = strings.NewReplacer(",", `\,`, " ", `\ `)
)

func escapeTag(s string) string         { return tagEscaper.Replace(s) }
func escapeMeasurement(s string) string { return measurementEscaper.Replace(s) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
