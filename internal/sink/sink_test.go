package sink

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUDPSplitsOnLines(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	s, err := New("udp://"+pc.LocalAddr().String(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	line := strings.Repeat("x", 999) + "\n"
	payload := []byte(strings.Repeat(line, 150)) // 150 kB
	if err := s.Send(payload); err != nil {
		t.Fatal(err)
	}
	var got []byte
	buf := make([]byte, 65536)
	for len(got) < len(payload) {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		if n > maxDatagram || buf[n-1] != '\n' {
			t.Fatalf("datagram of %d bytes not split on a line", n)
		}
		got = append(got, buf[:n]...)
	}
	if !bytes.Equal(got, payload) {
		t.Error("payload altered")
	}
}

func TestHTTPRetriesPending(t *testing.T) {
	var bodies []string
	fail := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token secret" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if fail {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	s, _ := New(srv.URL+"/write", "secret", 2)
	for _, p := range []string{"a", "b", "c"} {
		if err := s.Send([]byte(p)); err == nil {
			t.Fatal("expected an error while the server fails")
		}
	}
	fail = false
	if err := s.Send([]byte("d")); err != nil {
		t.Fatal(err)
	}
	if strings.Join(bodies, "") != "cd" { // max 2 kept: "a" and "b" were dropped
		t.Errorf("bodies = %v", bodies)
	}
}
