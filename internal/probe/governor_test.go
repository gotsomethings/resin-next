package probe

import (
	"testing"
	"time"
)

func newTestManager(settings int) *ProbeManager {
	m := NewProbeManager(ProbeConfig{Concurrency: settings})
	return m
}

func tightSample() ResourceSample {
	return ResourceSample{MemAvailableRatio: 0.05}
}

func relaxedSample() ResourceSample {
	return ResourceSample{MemAvailableRatio: 0.60, ContainerMemRatio: 0.20}
}

func TestGovernor_StepDownAnd_Floor(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m, Now: func() time.Time { return time.Now() }})

	now := time.Now()
	// First tight sample: no change (needs 2 consecutive).
	g.Step(now, tightSample())
	if got := m.EffectiveConcurrency(); got != 1000 {
		t.Fatalf("after 1 tight sample effective = %d, want 1000", got)
	}

	// Second tight sample: x0.7 -> 700.
	g.Step(now.Add(61*time.Second), tightSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("after 2 tight samples effective = %d, want 700", got)
	}

	// Repeated tight samples converge to the floor (10% = 100), never 0.
	at := now.Add(61 * time.Second)
	for i := 0; i < 80; i++ {
		at = at.Add(61 * time.Second)
		g.Step(at, tightSample())
		if got := m.EffectiveConcurrency(); got < 100 {
			t.Fatalf("effective dropped below floor: %d", got)
		}
	}
	if got := m.EffectiveConcurrency(); got != 100 {
		t.Fatalf("after sustained pressure effective = %d, want floor 100", got)
	}
}

func TestGovernor_RecoverToSettings(t *testing.T) {
	m := newTestManager(1000)
	g := NewGovernor(GovernorConfig{Manager: m, Now: func() time.Time { return time.Now() }})

	now := time.Now()
	g.Step(now, tightSample())
	g.Step(now.Add(61*time.Second), tightSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("setup: effective = %d, want 700", got)
	}

	// 2 relaxed samples are not enough.
	at := now.Add(61 * time.Second)
	g.Step(at.Add(61*time.Second), relaxedSample())
	g.Step(at.Add(122*time.Second), relaxedSample())
	if got := m.EffectiveConcurrency(); got != 700 {
		t.Fatalf("after 2 relaxed samples effective = %d, want 700", got)
	}

	// 3rd relaxed sample: +30% of settings -> 1000 (capped).
	g.Step(at.Add(183*time.Second), relaxedSample())
	if got := m.EffectiveConcurrency(); got != 1000 {
		t.Fatalf("after 3 relaxed samples effective = %d, want 1000", got)
	}
}

func TestGovernor_RecoveryIsStepwise(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(100)
	g := NewGovernor(GovernorConfig{Manager: m, Now: func() time.Time { return time.Now() }})

	at := time.Now()
	for i := 0; i < 3; i++ {
		at = at.Add(61 * time.Second)
		g.Step(at, relaxedSample())
	}
	if got := m.EffectiveConcurrency(); got != 400 {
		t.Fatalf("one step up should add 30%% of settings: got %d, want 400", got)
	}
}

func TestGovernor_HysteresisBandDoesNothing(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(500)
	g := NewGovernor(GovernorConfig{Manager: m, Now: func() time.Time { return time.Now() }})

	band := ResourceSample{MemAvailableRatio: 0.20, ContainerMemRatio: 0.75, LoadPerCPU: 0.80}
	at := time.Now()
	for i := 0; i < 10; i++ {
		at = at.Add(61 * time.Second)
		g.Step(at, band)
	}
	if got := m.EffectiveConcurrency(); got != 500 {
		t.Fatalf("band samples must not change effective: got %d, want 500", got)
	}
}

func TestProbeManager_SettingsHotUpdate(t *testing.T) {
	m := newTestManager(1000)
	m.SetEffectiveConcurrency(1000)
	m.SetSettingsConcurrency(400)
	if got := m.EffectiveConcurrency(); got != 400 {
		t.Fatalf("lowering settings must clamp effective: got %d, want 400", got)
	}
	m.SetSettingsConcurrency(50)
	m.SetEffectiveConcurrency(5000)
	if got := m.EffectiveConcurrency(); got != 50 {
		t.Fatalf("effective must never exceed settings: got %d, want 50", got)
	}
}
