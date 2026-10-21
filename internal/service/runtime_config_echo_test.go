package service

import (
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/Resinat/Resin/internal/config"
	"github.com/Resinat/Resin/internal/probe"
	"github.com/Resinat/Resin/internal/state"
)

// PATCHing the concurrency setting must be reflected in the GET echo, not just
// in the probe manager.
func TestPatchSystemConfig_FixedModeUpdatesEffectiveEcho(t *testing.T) {
	dir := t.TempDir()
	engine, closer, err := state.PersistenceBootstrap(dir+"/state", dir+"/cache")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer closer.Close()

	mgr := probe.NewProbeManager(probe.ProbeConfig{Concurrency: 1000})

	base := config.NewDefaultRuntimeConfig()
	base.ProbeConcurrencySettings = 1000
	base.ProbeConcurrencyMode = "fixed"
	base.ProbeConcurrencyEffective = 1000
	var ptr atomic.Pointer[config.RuntimeConfig]
	ptr.Store(base)

	cp := &ControlPlaneService{Engine: engine, RuntimeCfg: &ptr, ProbeMgr: mgr}

	body, _ := json.Marshal(map[string]any{"probe_concurrency": 900})
	if _, err := cp.PatchRuntimeConfig(body); err != nil {
		t.Fatalf("patch 900: %v", err)
	}

	got := ptr.Load()
	if got.ProbeConcurrencySettings != 900 {
		t.Fatalf("settings = %d, want 900", got.ProbeConcurrencySettings)
	}
	if got.ProbeConcurrencyEffective != 900 {
		t.Fatalf("effective echo = %d, want 900", got.ProbeConcurrencyEffective)
	}
	if mgr.EffectiveConcurrency() != 900 {
		t.Fatalf("manager effective = %d, want 900", mgr.EffectiveConcurrency())
	}

	body2, _ := json.Marshal(map[string]any{"probe_concurrency": 1500})
	if _, err := cp.PatchRuntimeConfig(body2); err != nil {
		t.Fatalf("patch 1500: %v", err)
	}

	got = ptr.Load()
	if got.ProbeConcurrencySettings != 1500 || got.ProbeConcurrencyEffective != 1500 {
		t.Fatalf("after 1500: settings=%d effective=%d, want 1500/1500",
			got.ProbeConcurrencySettings, got.ProbeConcurrencyEffective)
	}
}

// The read-only echo fields must never be taken from the request body.
func TestPatchSystemConfig_EchoFieldsAreNotClientWritable(t *testing.T) {
	dir := t.TempDir()
	engine, closer, err := state.PersistenceBootstrap(dir+"/state", dir+"/cache")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer closer.Close()

	mgr := probe.NewProbeManager(probe.ProbeConfig{Concurrency: 800})

	base := config.NewDefaultRuntimeConfig()
	base.ProbeConcurrencySettings = 800
	base.ProbeConcurrencyMode = "fixed"
	base.ProbeConcurrencyEffective = 800
	base.ProbeConcurrencyReason = "seed reason"
	base.ProbeConcurrencyAdjustedAt = "2026-01-01T00:00:00Z"
	var ptr atomic.Pointer[config.RuntimeConfig]
	ptr.Store(base)

	cp := &ControlPlaneService{Engine: engine, RuntimeCfg: &ptr, ProbeMgr: mgr}

	body, _ := json.Marshal(map[string]any{
		"probe_concurrency":          700,
		"probe_concurrency_effective": 1,
		"probe_concurrency_reason":    "forged reason",
		"probe_concurrency_adjusted_at": "1999-01-01T00:00:00Z",
	})
	if _, err := cp.PatchRuntimeConfig(body); err != nil {
		t.Fatalf("patch: %v", err)
	}

	got := ptr.Load()
	if got.ProbeConcurrencyEffective != 700 {
		t.Fatalf("effective = %d, want 700 (governor/manager wins over body)", got.ProbeConcurrencyEffective)
	}
	if got.ProbeConcurrencyReason != "seed reason" {
		t.Fatalf("reason = %q, want the existing value preserved", got.ProbeConcurrencyReason)
	}
	if got.ProbeConcurrencyAdjustedAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("adjusted_at = %q, want preserved", got.ProbeConcurrencyAdjustedAt)
	}
}
