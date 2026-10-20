package node

import (
	"sort"
	"sync"
	"time"
)

const (
	// DefaultProbeWindowSize is the number of platform-probe buckets kept per
	// node. One bucket per hour over 24h gives a rolling 24h success rate.
	DefaultProbeWindowSize = 24

	// DefaultP95SampleSize is the bounded raw-latency sample count per node.
	DefaultP95SampleSize = 32

	probeWindowBucket = time.Hour
)

// probeBucket is one hourly slot of the 24h rolling window.
type probeBucket struct {
	start int64 // unix-nano truncated to the hour; 0 = empty
	succ  int32
	total int32
}

// SlidingWindow is a fixed-size ring of hourly counters used to compute a
// rolling 24h probe success rate.
type SlidingWindow struct {
	mu      sync.Mutex
	buckets []probeBucket
	next    int
}

// NewSlidingWindow creates a window with n hourly buckets.
func NewSlidingWindow(n int) *SlidingWindow {
	if n <= 0 {
		n = DefaultProbeWindowSize
	}
	return &SlidingWindow{buckets: make([]probeBucket, n)}
}

// Record adds one probe outcome at time now.
func (w *SlidingWindow) Record(pass bool, now time.Time) {
	if w == nil {
		return
	}
	hour := now.Truncate(probeWindowBucket).UnixNano()

	w.mu.Lock()
	defer w.mu.Unlock()

	for i := range w.buckets {
		if w.buckets[i].start == hour {
			if pass {
				w.buckets[i].succ++
			}
			w.buckets[i].total++
			return
		}
	}

	slot := w.next
	w.next = (w.next + 1) % len(w.buckets)
	w.buckets[slot] = probeBucket{start: hour, total: 1}
	if pass {
		w.buckets[slot].succ = 1
	}
}

// Stats returns (successes, total) within the last 24h relative to now.
func (w *SlidingWindow) Stats(now time.Time) (succ, total int) {
	if w == nil {
		return 0, 0
	}
	horizon := now.Add(-time.Duration(len(w.buckets)) * probeWindowBucket).UnixNano()

	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range w.buckets {
		if b.start == 0 || b.start < horizon {
			continue
		}
		succ += int(b.succ)
		total += int(b.total)
	}
	return succ, total
}

// LatencyRing is a bounded ring of raw latency samples used for P95.
type LatencyRing struct {
	mu      sync.Mutex
	samples []time.Duration
	next    int
	filled  bool
}

// NewLatencyRing creates a ring holding n samples.
func NewLatencyRing(n int) *LatencyRing {
	if n <= 0 {
		n = DefaultP95SampleSize
	}
	return &LatencyRing{samples: make([]time.Duration, n)}
}

// Add records one latency sample.
func (r *LatencyRing) Add(d time.Duration) {
	if r == nil || d <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.samples[r.next] = d
	r.next = (r.next + 1) % len(r.samples)
	if r.next == 0 {
		r.filled = true
	}
}

// Len returns the number of valid samples.
func (r *LatencyRing) Len() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.filled {
		return len(r.samples)
	}
	return r.next
}

// P95 returns the 95th percentile sample, or 0 when empty.
func (r *LatencyRing) P95() time.Duration {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	snapshot := make([]time.Duration, 0, len(r.samples))
	if r.filled {
		snapshot = append(snapshot, r.samples...)
	} else {
		snapshot = append(snapshot, r.samples[:r.next]...)
	}
	r.mu.Unlock()

	if len(snapshot) == 0 {
		return 0
	}
	sort.Slice(snapshot, func(i, j int) bool { return snapshot[i] < snapshot[j] })
	idx := int(float64(len(snapshot)-1) * 0.95)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(snapshot) {
		idx = len(snapshot) - 1
	}
	return snapshot[idx]
}
