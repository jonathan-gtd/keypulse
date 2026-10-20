// Package netstat turns kernel network events into bytes sent and received
// per application. Only byte counts are kept: no address, no port and no
// packet content.
package netstat

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Counts is one application's traffic over a period.
type Counts struct {
	BytesSent uint64
	BytesRecv uint64
	Events    uint64
}

// Collector accumulates traffic per process. It is safe for concurrent use.
type Collector struct {
	mu     sync.Mutex
	byApp  map[string]*Counts
	names  map[uint32]string
	resolv func(uint32) string
}

// New returns a collector. resolve maps a process id to an executable name;
// it is called once per process id and the answer is cached.
func New(resolve func(uint32) string) *Collector {
	return &Collector{byApp: map[string]*Counts{}, names: map[uint32]string{}, resolv: resolve}
}

// Add records size bytes for a process, sent or received.
func (c *Collector) Add(pid uint32, size uint32, sent bool) {
	if size == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	name, ok := c.names[pid]
	if !ok {
		name = c.resolv(pid)
		// Process ids are reused; the cache is dropped once it grows large
		// so a recycled id cannot keep an old name forever.
		if len(c.names) > 4096 {
			clear(c.names)
		}
		c.names[pid] = name
	}
	a := c.byApp[name]
	if a == nil {
		a = &Counts{}
		c.byApp[name] = a
	}
	if sent {
		a.BytesSent += uint64(size)
	} else {
		a.BytesRecv += uint64(size)
	}
	a.Events++
}

// Snapshot returns the traffic since the previous snapshot and starts a new
// period.
func (c *Collector) Snapshot() map[string]Counts {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]Counts, len(c.byApp))
	for name, a := range c.byApp {
		out[name] = *a
	}
	c.byApp = map[string]*Counts{}
	return out
}

// Encode writes the snapshot as InfluxDB line protocol.
func Encode(measurement, host string, at int64, snap map[string]Counts) []byte {
	if len(snap) == 0 {
		return nil
	}
	names := make([]string, 0, len(snap))
	for n := range snap {
		names = append(names, n)
	}
	sort.Strings(names)

	ts := " " + strconv.FormatInt(at, 10) + "\n"
	hostTag := ""
	if host != "" {
		hostTag = ",host=" + escape(host)
	}
	var b strings.Builder
	for _, n := range names {
		c := snap[n]
		b.WriteString(escape(measurement))
		b.WriteString(hostTag)
		b.WriteString(",app=")
		b.WriteString(escape(n))
		b.WriteString(" bytes_sent=")
		b.WriteString(strconv.FormatUint(c.BytesSent, 10))
		b.WriteString("i,bytes_recv=")
		b.WriteString(strconv.FormatUint(c.BytesRecv, 10))
		b.WriteString("i,bytes_total=")
		b.WriteString(strconv.FormatUint(c.BytesSent+c.BytesRecv, 10))
		b.WriteString("i,events=")
		b.WriteString(strconv.FormatUint(c.Events, 10))
		b.WriteString("i")
		b.WriteString(ts)
	}
	return []byte(b.String())
}

var escaper = strings.NewReplacer(`\`, `\\`, ",", `\,`, "=", `\=`, " ", `\ `, "\n", `\n`)

func escape(s string) string { return escaper.Replace(s) }
