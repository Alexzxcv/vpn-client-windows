// pinger measures the user's OWN latency to each VPN node by timing a TCP
// connect to host:port. This is distinct from backend.Location.LatencyMs, which
// is the control-plane→node RTT; the user's real ping is what should drive the
// "Auto (best)" choice and the ping readouts in the UI.
package app

import (
	"context"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Ping measurement tuning.
const (
	// pingProbeTimeout bounds a single TCP-connect probe.
	pingProbeTimeout = 2 * time.Second
	// pingProbeCount is how many probes we take per node; we keep the minimum
	// (least affected by jitter/scheduling).
	pingProbeCount = 3
	// pingTTL is how long a measured value is considered fresh; older values are
	// re-measured on the next request that needs them.
	pingTTL = 30 * time.Second
	// pingMaxConcurrent caps simultaneous probes so a long location list cannot
	// open an unbounded number of sockets at once.
	pingMaxConcurrent = 16
)

// pingResult is a cached measurement for one node.
type pingResult struct {
	ms       int       // measured RTT in ms (>0 when valid)
	measured time.Time // when it was measured
}

// pinger caches per-node measured ping keyed by location ID. Safe for
// concurrent use.
type pinger struct {
	// dial is the TCP dialer; overridable in tests.
	dial func(ctx context.Context, network, addr string) (net.Conn, error)

	mu    sync.Mutex
	cache map[string]pingResult
}

func newPinger() *pinger {
	var d net.Dialer
	return &pinger{
		dial:  d.DialContext,
		cache: make(map[string]pingResult),
	}
}

// PingMs returns the cached measured ping for a location id, or 0 if none is
// cached (regardless of freshness).
func (p *pinger) PingMs(id string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cache[id].ms
}

// pingTarget is the minimal info needed to probe a node.
type pingTarget struct {
	id   string
	host string
	port int
}

// Refresh measures (concurrently) every target whose cached value is missing or
// older than pingTTL, updating the cache. Targets without a host/port are
// skipped (their cache entry, if any, is left untouched). The provided ctx
// bounds the whole batch.
func (p *pinger) Refresh(ctx context.Context, targets []pingTarget) {
	now := time.Now()

	var stale []pingTarget
	p.mu.Lock()
	for _, t := range targets {
		if t.host == "" || t.port <= 0 {
			continue
		}
		cur, ok := p.cache[t.id]
		if ok && cur.ms > 0 && now.Sub(cur.measured) < pingTTL {
			continue
		}
		stale = append(stale, t)
	}
	p.mu.Unlock()

	if len(stale) == 0 {
		return
	}

	sem := make(chan struct{}, pingMaxConcurrent)
	var wg sync.WaitGroup
	for _, t := range stale {
		wg.Add(1)
		go func(t pingTarget) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			ms := p.measure(ctx, t.host, t.port)
			if ms <= 0 {
				return // failed probe — keep any previous value / fall back to latency
			}
			p.mu.Lock()
			p.cache[t.id] = pingResult{ms: ms, measured: time.Now()}
			p.mu.Unlock()
		}(t)
	}
	wg.Wait()
}

// measure runs up to pingProbeCount TCP-connect probes to host:port and returns
// the minimum RTT in ms, or 0 if every probe failed (or ctx was cancelled).
func (p *pinger) measure(ctx context.Context, host string, port int) int {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	best := 0
	for i := 0; i < pingProbeCount; i++ {
		if ctx.Err() != nil {
			break
		}
		probeCtx, cancel := context.WithTimeout(ctx, pingProbeTimeout)
		start := time.Now()
		conn, err := p.dial(probeCtx, "tcp", addr)
		elapsed := time.Since(start)
		cancel()
		if err != nil {
			continue
		}
		_ = conn.Close()
		ms := int(elapsed.Milliseconds())
		if ms < 1 {
			ms = 1 // sub-millisecond connect still counts as reachable
		}
		if best == 0 || ms < best {
			best = ms
		}
	}
	return best
}

// bestID returns the location id with the lowest measured ping among targets
// that have a cached value, or "" if none is measured. Ties break on the lowest
// id for determinism.
func (p *pinger) bestID(ids []string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	type cand struct {
		id string
		ms int
	}
	var cands []cand
	for _, id := range ids {
		if r := p.cache[id]; r.ms > 0 {
			cands = append(cands, cand{id: id, ms: r.ms})
		}
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].ms != cands[j].ms {
			return cands[i].ms < cands[j].ms
		}
		return cands[i].id < cands[j].id
	})
	return cands[0].id
}
