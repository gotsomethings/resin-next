package service

import (
	"strings"
	"time"

	"github.com/Resinat/Resin/internal/platform"
)

// probePatch is a partial update of probe/verdict config.
type probePatch struct {
	enabled         *bool
	url             *string
	method          *string
	headers         *string
	body            *string
	passStatus      []int
	hasPassStatus   bool
	followRedirects *bool
	interval        *time.Duration
	scope           *string

	verdictEnabled        *bool
	blacklistThreshold    *int
	rehabInterval         *time.Duration
	rehabSuccessThreshold *int
	blacklistEmptyPolicy  *string
	blacklistGraceK       *int
}

func probePatchFromCreate(req CreatePlatformRequest) probePatch {
	p := probePatch{
		enabled:               req.ProbeEnabled,
		url:                   req.ProbeURL,
		method:                req.ProbeMethod,
		headers:               req.ProbeHeaders,
		body:                  req.ProbeBody,
		followRedirects:       req.ProbeFollowRedirects,
		scope:                 req.ProbeScope,
		verdictEnabled:        req.VerdictEnabled,
		blacklistThreshold:    req.BlacklistThreshold,
		rehabSuccessThreshold: req.RehabSuccessThreshold,
		blacklistEmptyPolicy:  req.BlacklistEmptyPolicy,
		blacklistGraceK:       req.BlacklistGraceK,
	}
	if req.ProbePassStatus != nil {
		p.passStatus = req.ProbePassStatus
		p.hasPassStatus = true
	}
	if req.ProbeInterval != nil {
		if d, err := time.ParseDuration(*req.ProbeInterval); err == nil {
			p.interval = &d
		}
	}
	if req.RehabInterval != nil {
		if d, err := time.ParseDuration(*req.RehabInterval); err == nil {
			p.rehabInterval = &d
		}
	}
	return p
}

func probePatchFromMerge(patch mergePatch) (probePatch, *ServiceError) {
	var p probePatch

	if v, ok, serr := patch.optionalBool("probe_enabled"); serr != nil {
		return p, serr
	} else if ok {
		p.enabled = boolPtr(v)
	}
	if v, ok, serr := patch.optionalString("probe_url"); serr != nil {
		return p, serr
	} else if ok {
		p.url = &v
	}
	if v, ok, serr := patch.optionalString("probe_method"); serr != nil {
		return p, serr
	} else if ok {
		p.method = &v
	}
	if v, ok, serr := patch.optionalString("probe_headers"); serr != nil {
		return p, serr
	} else if ok {
		p.headers = &v
	}
	if v, ok, serr := patch.optionalString("probe_body"); serr != nil {
		return p, serr
	} else if ok {
		p.body = &v
	}
	if v, ok, serr := patch.optionalIntSlice("probe_pass_status"); serr != nil {
		return p, serr
	} else if ok {
		p.passStatus = v
		p.hasPassStatus = true
	}
	if v, ok, serr := patch.optionalBool("probe_follow_redirects"); serr != nil {
		return p, serr
	} else if ok {
		p.followRedirects = boolPtr(v)
	}
	if d, ok, serr := patch.optionalDurationString("probe_interval"); serr != nil {
		return p, serr
	} else if ok {
		p.interval = &d
	}
	if v, ok, serr := patch.optionalString("probe_scope"); serr != nil {
		return p, serr
	} else if ok {
		p.scope = &v
	}

	if v, ok, serr := patch.optionalBool("verdict_enabled"); serr != nil {
		return p, serr
	} else if ok {
		p.verdictEnabled = boolPtr(v)
	}
	if v, ok, serr := patch.optionalInt("blacklist_threshold"); serr != nil {
		return p, serr
	} else if ok {
		p.blacklistThreshold = intPtr(v)
	}
	if d, ok, serr := patch.optionalDurationString("rehab_interval"); serr != nil {
		return p, serr
	} else if ok {
		p.rehabInterval = &d
	}
	if v, ok, serr := patch.optionalInt("rehab_success_threshold"); serr != nil {
		return p, serr
	} else if ok {
		p.rehabSuccessThreshold = intPtr(v)
	}
	if v, ok, serr := patch.optionalString("blacklist_empty_policy"); serr != nil {
		return p, serr
	} else if ok {
		p.blacklistEmptyPolicy = &v
	}
	if v, ok, serr := patch.optionalInt("blacklist_grace_k"); serr != nil {
		return p, serr
	} else if ok {
		p.blacklistGraceK = intPtr(v)
	}
	return p, nil
}

func applyProbePatch(probe *platform.ProbeConfig, verdict *platform.VerdictConfig, p probePatch) *ServiceError {
	if p.enabled != nil {
		probe.Enabled = *p.enabled
	}
	if p.url != nil {
		probe.URL = strings.TrimSpace(*p.url)
	}
	if p.method != nil {
		probe.Method = strings.ToUpper(strings.TrimSpace(*p.method))
	}
	if p.headers != nil {
		probe.Headers = *p.headers
	}
	if p.body != nil {
		probe.Body = *p.body
	}
	if p.hasPassStatus {
		probe.PassStatus = append([]int(nil), p.passStatus...)
	}
	if p.followRedirects != nil {
		probe.FollowRedirects = *p.followRedirects
	}
	if p.interval != nil {
		probe.Interval = *p.interval
	}
	if p.scope != nil {
		probe.Scope = *p.scope
	}

	if p.verdictEnabled != nil {
		verdict.Enabled = *p.verdictEnabled
	}
	if p.blacklistThreshold != nil {
		verdict.Threshold = *p.blacklistThreshold
	}
	if p.rehabInterval != nil {
		verdict.RehabInterval = *p.rehabInterval
	}
	if p.rehabSuccessThreshold != nil {
		verdict.RehabSuccessThreshold = *p.rehabSuccessThreshold
	}
	if p.blacklistEmptyPolicy != nil {
		verdict.EmptyPolicy = *p.blacklistEmptyPolicy
	}
	if p.blacklistGraceK != nil {
		verdict.GraceK = *p.blacklistGraceK
	}
	return nil
}

func validateProbeConfig(c platform.ProbeConfig) *ServiceError {
	if c.Enabled {
		if strings.TrimSpace(c.URL) == "" {
			return invalidArg("probe_url: required when probe_enabled is true")
		}
		if _, err := parseHTTPAbsoluteURL("probe_url", c.URL); err != nil {
			return err
		}
	}
	switch c.Method {
	case "", "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD":
	default:
		return invalidArg("probe_method: must be GET, POST, PUT, PATCH, DELETE, or HEAD")
	}
	for _, code := range c.PassStatus {
		if code < 100 || code > 599 {
			return invalidArg("probe_pass_status: codes must be between 100 and 599")
		}
	}
	if c.Interval < 30*time.Second {
		return invalidArg("probe_interval: must be >= 30s")
	}
	for _, token := range strings.Split(c.Scope, ",") {
		switch strings.TrimSpace(token) {
		case "", "untested", "failed", "blacklisted":
		default:
			return invalidArg("probe_scope: only untested, failed, blacklisted are allowed")
		}
	}
	return nil
}

func validateVerdictConfig(v platform.VerdictConfig) *ServiceError {
	if v.Threshold < 1 {
		return invalidArg("blacklist_threshold: must be >= 1")
	}
	if v.RehabInterval < time.Minute {
		return invalidArg("rehab_interval: must be >= 1m")
	}
	if v.RehabSuccessThreshold < 1 {
		return invalidArg("rehab_success_threshold: must be >= 1")
	}
	switch v.EmptyPolicy {
	case "", "strict", "grace":
	default:
		return invalidArg("blacklist_empty_policy: must be strict or grace")
	}
	return nil
}

func boolPtr(v bool) *bool { return &v }
