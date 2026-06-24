package app

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"
)

// fakeConn is a no-op net.Conn for the fake dialer.
type fakeConn struct{ net.Conn }

func (fakeConn) Close() error { return nil }

func TestPingerMeasureAndBest(t *testing.T) {
	p := newPinger()
	// Fake dialer: nodeA fast, nodeB slow, nodeC always fails.
	delays := map[string]time.Duration{
		"a:1": 5 * time.Millisecond,
		"b:1": 40 * time.Millisecond,
	}
	p.dial = func(ctx context.Context, _ /*network*/, addr string) (net.Conn, error) {
		if d, ok := delays[addr]; ok {
			select {
			case <-time.After(d):
				return fakeConn{}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return nil, errors.New("refused")
	}

	targets := []pingTarget{
		{id: "A", host: "a", port: 1},
		{id: "B", host: "b", port: 1},
		{id: "C", host: "c", port: 1},
		{id: "D", host: "", port: 0}, // skipped (no host)
	}
	p.Refresh(context.Background(), targets)

	if got := p.PingMs("A"); got <= 0 {
		t.Fatalf("A ping not measured: %d", got)
	}
	if got := p.PingMs("C"); got != 0 {
		t.Fatalf("C should be unmeasured (failed), got %d", got)
	}
	if got := p.PingMs("D"); got != 0 {
		t.Fatalf("D should be unmeasured (skipped), got %d", got)
	}
	if best := p.bestID([]string{"A", "B", "C"}); best != "A" {
		t.Fatalf("best should be A (lowest ping), got %q", best)
	}
}

func TestPingerBestEmptyWhenNoneMeasured(t *testing.T) {
	p := newPinger()
	if best := p.bestID([]string{"X", "Y"}); best != "" {
		t.Fatalf("expected empty best with no measurements, got %q", best)
	}
}

// TestPingerRealDial sanity-checks measure against a real local listener.
func TestPingerRealDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	p := newPinger()
	ms := p.measure(context.Background(), host, port)
	if ms <= 0 {
		t.Fatalf("expected a positive measured ping to a live listener, got %d", ms)
	}
}
