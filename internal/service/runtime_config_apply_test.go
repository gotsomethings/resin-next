package service

import (
	"sync/atomic"
	"testing"

	"github.com/Resinat/Resin/internal/config"
	"github.com/Resinat/Resin/internal/probe"
	"github.com/Resinat/Resin/internal/state"
)

// PATCH /system/config must move the probe manager's effective concurrency in
// fixed mode, end to end through the service layer (no HTTP).
func TestPatchRuntimeConfig_FixedModeAppliesEffectiveConcurrency(t *testing.T) {
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

	cp := &ControlPlaneService{
		Engine:     engine,
		RuntimeCfg: &ptr,
		ProbeMgr:   mgr,
	}

	if _, err := cp.PatchRuntimeConfig([]byte(`{"probe_concurrency":400}`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if got := mgr.SettingsConcurrency(); got != 400 {
		t.Fatalf("settings = %d, want 400", got)
	}
	if got := mgr.EffectiveConcurrency(); got != 400 {
		t.Fatalf("fixed mode must pin effective to 400, got %d", got)
	}

	// Switching to auto must not pin the value; the governor owns it from here.
	if _, err := cp.PatchRuntimeConfig([]byte(`{"probe_concurrency_mode":"auto"}`)); err != nil {
		t.Fatalf("patch mode: %v", err)
	}
	if got := mgr.SettingsConcurrency(); got != 400 {
		t.Fatalf("settings after mode switch = %d, want 400", got)
	}
}

// A PATCH that lowers the ceiling must clamp the effective value down.
func TestPatchRuntimeConfig_LowerCeilingClampsEffective(t *testing.T) {
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

	if _, err := cp.PatchRuntimeConfig([]byte(`{"probe_concurrency":100}`)); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if got := mgr.EffectiveConcurrency(); got != 100 {
		t.Fatalf("effective = %d, want 100 (clamped to new ceiling)", got)
	}
}
