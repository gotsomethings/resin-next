package probe

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ResourceSample is one governance observation. Ratios are 0..1; a zero value
// means "unknown" and is ignored by the pressure checks.
type ResourceSample struct {
	MemAvailableRatio float64 // host MemAvailable / MemTotal
	ContainerMemRatio float64 // cgroup memory.current / memory.max
	SwapUsedRatio     float64
	LoadPerCPU        float64
	CPUUtilization    float64
}

// GovernorConfig configures the probe-concurrency governor.
type GovernorConfig struct {
	Manager  *ProbeManager
	Sample   func() (ResourceSample, error)
	Interval time.Duration
	OnAdjust func(effective int, reason string, at time.Time)
	GC       func()
	// GCInterval is how often GC runs (default 6h). The retention window
	// itself lives in RuntimeConfig.VerdictGCRetention (default 72h).
	GCInterval time.Duration
	Now      func() time.Time
}

// Governor samples host/container pressure and moves the probe manager's
// effective concurrency between 10% and 100% of the configured ceiling.
type Governor struct {
	cfg      GovernorConfig
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup

	lastAdjust time.Time
	tightRun   int
	relaxRun   int
}

// NewGovernor creates a Governor.
func NewGovernor(cfg GovernorConfig) *Governor {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.GCInterval <= 0 {
		cfg.GCInterval = 6 * time.Hour
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sample == nil {
		cfg.Sample = ReadResourceSample
	}
	return &Governor{cfg: cfg, stopCh: make(chan struct{})}
}

// Start launches the sampling loop.
func (g *Governor) Start() {
	if g == nil || g.cfg.Manager == nil {
		return
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		ticker := time.NewTicker(g.cfg.Interval)
		defer ticker.Stop()
		lastGC := g.cfg.Now()
		for {
			select {
			case <-g.stopCh:
				return
			case <-ticker.C:
				sample, err := g.cfg.Sample()
				if err == nil {
					g.Step(g.cfg.Now(), sample)
				}
				if g.cfg.GC != nil && g.cfg.Now().Sub(lastGC) >= g.cfg.GCInterval {
					g.cfg.GC()
					lastGC = g.cfg.Now()
				}
			}
		}
	}()
}

// Stop stops the loop.
func (g *Governor) Stop() {
	if g == nil {
		return
	}
	g.stopOnce.Do(func() { close(g.stopCh) })
	g.wg.Wait()
}

// Step applies one observation. Exported for deterministic tests.
func (g *Governor) Step(now time.Time, s ResourceSample) {
	m := g.cfg.Manager
	if m == nil {
		return
	}
	settings := m.SettingsConcurrency()
	if settings <= 0 {
		return
	}
	floor := settings / 10
	if floor < 1 {
		floor = 1
	}
	effective := m.EffectiveConcurrency()

	tight, reason := isTight(s)
	relaxed := isRelaxed(s)

	if tight {
		g.tightRun++
		g.relaxRun = 0
		if g.tightRun >= 2 && now.Sub(g.lastAdjust) >= time.Minute {
			next := effective * 7 / 10
			if next < floor {
				next = floor
			}
			if next < effective {
				m.SetEffectiveConcurrency(next)
				g.lastAdjust = now
				g.tightRun = 1
				g.report(next, reason, now)
			}
		}
		return
	}

	if relaxed {
		g.relaxRun++
		g.tightRun = 0
		if g.relaxRun >= 3 && now.Sub(g.lastAdjust) >= time.Minute && effective < settings {
			next := effective + settings*3/10
			if next > settings {
				next = settings
			}
			m.SetEffectiveConcurrency(next)
			g.lastAdjust = now
			g.relaxRun = 1
			g.report(next, "pressure released", now)
		}
		return
	}

	// Hysteresis band: keep counters, take no action.
}

func (g *Governor) report(effective int, reason string, at time.Time) {
	log.Printf("[governor] probe concurrency -> %d (%s)", effective, reason)
	if g.cfg.OnAdjust != nil {
		g.cfg.OnAdjust(effective, reason, at)
	}
}

func isTight(s ResourceSample) (bool, string) {
	switch {
	case s.MemAvailableRatio > 0 && s.MemAvailableRatio < 0.10:
		return true, "host memory available < 10%"
	case s.ContainerMemRatio >= 0.85:
		return true, "container memory >= 85%"
	case s.SwapUsedRatio > 0.50:
		return true, "swap used > 50%"
	case s.LoadPerCPU > 0.90:
		return true, "load per CPU > 0.9"
	case s.CPUUtilization > 0.85:
		return true, "CPU utilization > 85%"
	}
	return false, ""
}

func isRelaxed(s ResourceSample) bool {
	if s.MemAvailableRatio > 0 && s.MemAvailableRatio <= 0.30 {
		return false
	}
	if s.ContainerMemRatio >= 0.70 {
		return false
	}
	if s.SwapUsedRatio > 0.50 {
		return false
	}
	if s.LoadPerCPU >= 0.70 {
		return false
	}
	if s.CPUUtilization >= 0.70 {
		return false
	}
	return true
}

// ReadResourceSample reads Linux /proc and cgroup v1/v2 counters. On platforms
// without them it returns an empty sample (governor treats that as relaxed).
func ReadResourceSample() (ResourceSample, error) {
	var s ResourceSample

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		vals := parseMeminfo(string(data))
		if total := vals["MemTotal"]; total > 0 {
			s.MemAvailableRatio = float64(vals["MemAvailable"]) / float64(total)
		}
		if swapTotal := vals["SwapTotal"]; swapTotal > 0 {
			s.SwapUsedRatio = float64(swapTotal-vals["SwapFree"]) / float64(swapTotal)
		}
	}

	s.ContainerMemRatio = readCgroupMemRatio()

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 0 {
			if load, err := strconv.ParseFloat(fields[0], 64); err == nil {
				cpus := runtime.NumCPU()
				if cpus > 0 {
					s.LoadPerCPU = load / float64(cpus)
				}
			}
		}
	}

	s.CPUUtilization = readCPUUtilization()
	return s, nil
}

func parseMeminfo(data string) map[string]int64 {
	out := make(map[string]int64)
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
			out[key] = v
		}
	}
	return out
}

func readCgroupMemRatio() float64 {
	read := func(path string) (int64, bool) {
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, false
		}
		v, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}

	if cur, ok := read("/sys/fs/cgroup/memory.current"); ok {
		if max, ok := read("/sys/fs/cgroup/memory.max"); ok && max > 0 {
			return float64(cur) / float64(max)
		}
	}
	if cur, ok := read("/sys/fs/cgroup/memory/memory.usage_in_bytes"); ok {
		if max, ok := read("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok && max > 0 && max < (1<<60) {
			return float64(cur) / float64(max)
		}
	}
	return 0
}

var (
	cpuMu       sync.Mutex
	cpuPrevIdle uint64
	cpuPrevAll  uint64
)

func readCPUUtilization() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return 0
	}
	var total uint64
	for _, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			continue
		}
		total += v
	}
	idle, _ := strconv.ParseUint(fields[4], 10, 64)

	cpuMu.Lock()
	defer cpuMu.Unlock()
	prevIdle, prevAll := cpuPrevIdle, cpuPrevAll
	cpuPrevIdle, cpuPrevAll = idle, total
	if prevAll == 0 || total <= prevAll {
		return 0
	}
	dTotal := float64(total - prevAll)
	dIdle := float64(idle - prevIdle)
	if dTotal <= 0 {
		return 0
	}
	util := (dTotal - dIdle) / dTotal
	if util < 0 {
		return 0
	}
	if util > 1 {
		return 1
	}
	return util
}
