package service

import (
	"strings"
	"time"

	"github.com/Resinat/Resin/internal/platform"
)

// failoverPatch is a partial update of platform.FailoverConfig.
type failoverPatch struct {
	enabled     *bool
	maxSwitches *int
	deadline    *time.Duration
	stage       *string
}

func failoverPatchFromCreate(req CreatePlatformRequest) failoverPatch {
	return failoverPatch{
		enabled:     req.FailoverEnabled,
		maxSwitches: req.FailoverMaxSwitches,
		stage:       req.FailoverStage,
	}
}

func failoverPatchFromMerge(patch mergePatch) (failoverPatch, *ServiceError) {
	var p failoverPatch

	if v, ok, serr := patch.optionalBool("failover_enabled"); serr != nil {
		return p, serr
	} else if ok {
		p.enabled = boolPtr(v)
	}
	if v, ok, serr := patch.optionalInt("failover_max_switches"); serr != nil {
		return p, serr
	} else if ok {
		p.maxSwitches = intPtr(v)
	}
	if d, ok, serr := patch.optionalDurationString("failover_deadline"); serr != nil {
		return p, serr
	} else if ok {
		p.deadline = &d
	}
	if v, ok, serr := patch.optionalString("failover_stage"); serr != nil {
		return p, serr
	} else if ok {
		p.stage = &v
	}
	return p, nil
}

func applyFailoverPatch(cfg *platform.FailoverConfig, p failoverPatch) {
	if p.enabled != nil {
		cfg.Enabled = *p.enabled
	}
	if p.maxSwitches != nil {
		cfg.MaxSwitches = *p.maxSwitches
	}
	if p.deadline != nil {
		cfg.Deadline = *p.deadline
	}
	if p.stage != nil {
		cfg.Stage = strings.ToLower(strings.TrimSpace(*p.stage))
	}
}

func validateFailoverConfig(c platform.FailoverConfig) *ServiceError {
	if c.MaxSwitches < 1 || c.MaxSwitches > 10 {
		return invalidArg("failover_max_switches: must be between 1 and 10")
	}
	if c.Deadline < time.Second || c.Deadline > time.Minute {
		return invalidArg("failover_deadline: must be between 1s and 1m")
	}
	switch c.Stage {
	case "", "connect":
	default:
		return invalidArg("failover_stage: only \"connect\" is implemented; first_byte/any are reserved and rejected")
	}
	return nil
}
