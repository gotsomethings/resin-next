package probe

import (
	"testing"
	"time"
)

func tightSample() ResourceSample {
	return ResourceSample{MemAvailableRatio: 0.05}
}

func relaxedSample() ResourceSample {
	return ResourceSample{MemAvailableRatio: 0.60, ContainerMemRatio: 0.20}
}

func newTestManager(settings int) *ProbeManager {
	return NewProbeManager(ProbeConfig{Concurrency: settings})
}

// A single tight sample must step down immediately (small-memory safety).
func TestGovernor_SingleTightSampleStepsDown(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m})

	g.Step(time.Now(), tightSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("single tight sample: effective = %d, want 700", got)
	}
}

// Sustained pressure keeps stepping down, but only once per pacing window.
func TestGovernor_SustainedPressureIsPaced(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m})

	base := time.Now()
	g.Step(base, tightSample()) // 700 (first, immediate)
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("first step = %d, want 700", got)
	}

	g.Step(base.Add(30*time.Second), tightSample()) // same window: no change
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("within pacing window: effective = %d, want 700", got)
	}

	g.Step(base.Add(60*time.Second), tightSample()) // next window: 490
	if got := m.EffectiveConcurrency(); got != 490 {
		t.Fatalf("second window: effective = %d, want 490", got)
	}
}

// Never drops below 10% of the configured ceiling, never to zero.
func TestGovernor_FloorIsTenPercent(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m})

	at := time.Now()
	for i := 0; i < 120; i++ {
		at = at.Add(30 * time.Second)
		g.Step(at, tightSample())
		if got := m.EffectiveConcurrency(); got < 100 {
			t.Fatalf("effective dropped below floor: %d", got)
		}
	}
	if got := m.EffectiveConcurrency(); got != 100 {
		t.Fatalf("floor = %d, want 100", got)
	}
}

// Recovery still needs 3 consecutive relaxed samples, then +30% of settings per window.
func TestGovernor_RecoveryNeedsThreeRelaxedSamples(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m})

	at := time.Now()
	g.Step(at, tightSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("setup = %d, want 700", got)
	}

	at = at.Add(60 * time.Second)
	g.Step(at, relaxedSample())
	g.Step(at.Add(30*time.Second), relaxedSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("two relaxed samples: effective = %d, want 700", got)
	}

	g.Step(at.Add(60*time.Second), relaxedSample()) // third relaxed + paced
	if got := m.EffectiveConcurrency(); got != 1000 {
		t.Fatalf("third relaxed sample: effective = %d, want 1000 (capped)", got)
	}
}

func TestGovernor_RecoveryIsStepwise(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(100)
	g := NewGovernor(GovernorConfig{Manager: m})

	at := time.Now()
	for i := 0; i < 4; i++ {
		at = at.Add(30 * time.Second)
		g.Step(at, relaxedSample())
	}
	if got := m.EffectiveConcurrency(); got != 400 {
		t.Fatalf("one recovery window = %d, want 400", got)
	}
}

// The 85%/70% band must not move anything.
func TestGovernor_HysteresisBand(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(500)
	g := NewGovernor(GovernorConfig{Manager: m})

	band := ResourceSample{MemAvailableRatio: 0.20, ContainerMemRatio: 0.75, LoadPerCPU: 0.80}
	at := time.Now()
	for i := 0; i < 10; i++ {
		at = at.Add(30 * time.Second)
		g.Step(at, band)
	}
	if got := m.EffectiveConcurrency(); got != 500 {
		t.Fatalf("band must not move effective: got %d, want 500", got)
	}
}

// Swap is not a pressure signal any more: a full swap must not step down.
func TestGovernor_SwapIsNotAPressureSignal(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m})

	sample := ResourceSample{MemAvailableRatio: 0.50, ContainerMemRatio: 0.10}
	g.Step(time.Now(), sample)
	if got := m.EffectiveConcurrency(); got != 1000 {
		t.Fatalf("healthy memory must not step down: got %d", got)
	}
}

func TestProbeManager_SettingsClampIsAtomic(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(1000)

	m.SetSettingsConcurrency(400)
	if got := m.EffectiveConcurrency(); got != 400 {
		t.Fatalf("lowering ceiling must clamp effective: got %d, want 400", got)
	}

	m.SetSettingsConcurrency(50)
	m.SetEffectiveConcurrency(5000)
	if got := m.EffectiveConcurrency(); got != 50 {
		t.Fatalf("effective must never exceed ceiling: got %d, want 50", got)
	}

	m.SetSettingsConcurrency(2000)
	if got := m.EffectiveConcurrency(); got != 50 {
		t.Fatalf("raising ceiling must not inflate effective: got %d, want 50", got)
	}
}

func TestProbeManager_AcquireSlotRespectsLimit(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(2)

	if !m.acquireSlot() || !m.acquireSlot() {
		t.Fatal("first two acquisitions must succeed")
	}
	if m.acquireSlot() {
		t.Fatal("third acquisition must be refused at limit 2")
	}
	m.releaseSlot()
	if !m.acquireSlot() {
		t.Fatal("slot must be reusable after release")
	}
}

// tight -> relaxed -> tight inside one minute must step down only once.
func TestGovernor_JitterDoesNotDoubleStep(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m})

	base := time.Now()
	g.Step(base, tightSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("first tight = %d, want 700", got)
	}

	g.Step(base.Add(10*time.Second), relaxedSample())
	g.Step(base.Add(20*time.Second), tightSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("second tight inside the minute = %d, want 700 (gate must hold)", got)
	}

	g.Step(base.Add(61*time.Second), tightSample())
	if got := m.EffectiveConcurrency(); got != 490 {
		t.Fatalf("tight after the gate = %d, want 490", got)
	}
}
