// Package sink sends line protocol to Telegraf or InfluxDB.
package sink

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Sink delivers encoded points.
type Sink interface {
	Send(payload []byte) error
	Close() error
}

// New returns a sink for rawURL:
//
//	udp://127.0.0.1:8094               Telegraf socket_listener
//	http://127.0.0.1:8186/write        Telegraf influxdb_listener or http_listener_v2
//	https://influx:8086/api/v2/write?org=o&bucket=b   InfluxDB 2, with token
//
// HTTP payloads that cannot be delivered are kept, up to maxBuffer of them,
// and sent again with the next payload.
func New(rawURL, token string, maxBuffer int) (Sink, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "udp":
		conn, err := net.Dial("udp", u.Host)
		if err != nil {
			return nil, err
		}
		return &udpSink{conn: conn}, nil
	case "http", "https":
		return &httpSink{url: rawURL, token: token, max: maxBuffer, client: &http.Client{Timeout: 10 * time.Second}}, nil
	}
	return nil, fmt.Errorf("unsupported output scheme %q (use udp, http or https)", u.Scheme)
}

// maxDatagram stays below the 64 KiB UDP limit and Telegraf's default read buffer.
const maxDatagram = 60000

type udpSink struct{ conn net.Conn }

// Send splits the payload on line boundaries into datagrams.
func (s *udpSink) Send(payload []byte) error {
	for len(payload) > 0 {
		n := len(payload)
		if n > maxDatagram {
			n = bytes.LastIndexByte(payload[:maxDatagram], '\n') + 1
			if n == 0 {
				return fmt.Errorf("line longer than %d bytes", maxDatagram)
			}
		}
		if _, err := s.conn.Write(payload[:n]); err != nil {
			return err
		}
		payload = payload[n:]
	}
	return nil
}

func (s *udpSink) Close() error { return s.conn.Close() }

type httpSink struct {
	url, token string
	max        int
	client     *http.Client
	pending    [][]byte
}

func (s *httpSink) Send(payload []byte) error {
	s.pending = append(s.pending, payload)
	if s.max > 0 && len(s.pending) > s.max {
		s.pending = s.pending[len(s.pending)-s.max:]
	}
	for len(s.pending) > 0 {
		if err := s.post(s.pending[0]); err != nil {
			return fmt.Errorf("%w (%d payloads kept for retry)", err, len(s.pending))
		}
		s.pending = s.pending[1:]
	}
	return nil
}

func (s *httpSink) post(payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, s.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	if s.token != "" {
		req.Header.Set("Authorization", "Token "+s.token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s", s.url, resp.Status)
	}
	return nil
}

func (s *httpSink) Close() error {
	s.client.CloseIdleConnections()
	return nil
}
