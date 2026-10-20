package netstat

import (
	"strings"
	"testing"
)

func TestCollector(t *testing.T) {
	names := map[uint32]string{1: "chrome.exe", 2: "game.exe"}
	calls := 0
	c := New(func(pid uint32) string {
		calls++
		if n, ok := names[pid]; ok {
			return n
		}
		return "unknown"
	})

	c.Add(1, 1000, true)
	c.Add(1, 500, false)
	c.Add(1, 0, true) // ignored
	c.Add(2, 42, false)
	c.Add(9, 7, true)

	if calls != 3 {
		t.Errorf("resolver called %d times, want 3 (one per pid)", calls)
	}
	snap := c.Snapshot()
	if got := snap["chrome.exe"]; got.BytesSent != 1000 || got.BytesRecv != 500 || got.Events != 2 {
		t.Errorf("chrome = %+v", got)
	}
	if got := snap["game.exe"]; got.BytesRecv != 42 {
		t.Errorf("game = %+v", got)
	}
	if got := snap["unknown"]; got.BytesSent != 7 {
		t.Errorf("unknown = %+v", got)
	}
	if len(c.Snapshot()) != 0 {
		t.Error("collector not reset")
	}

	c.Add(1, 10, true)
	if calls != 3 {
		t.Errorf("resolver called again after snapshot: %d", calls)
	}
}

func TestEncode(t *testing.T) {
	snap := map[string]Counts{
		"my app.exe": {BytesSent: 1200, BytesRecv: 34, Events: 5},
		"a.exe":      {BytesSent: 1},
	}
	got := string(Encode("keypulse_net", "mars", 1700000000000000000, snap))
	want := strings.Join([]string{
		`keypulse_net,host=mars,app=a.exe bytes_sent=1i,bytes_recv=0i,bytes_total=1i,events=0i 1700000000000000000`,
		`keypulse_net,host=mars,app=my\ app.exe bytes_sent=1200i,bytes_recv=34i,bytes_total=1234i,events=5i 1700000000000000000`,
		``,
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if Encode("keypulse_net", "mars", 1, nil) != nil {
		t.Error("empty snapshot should encode to nothing")
	}
}
