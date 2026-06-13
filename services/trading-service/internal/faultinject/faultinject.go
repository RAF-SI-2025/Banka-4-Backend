// Package faultinject implements the X-Saga-* adversarial test headers from
// the SAGA test specification. A request that starts an OTC exercise saga may
// name a forward phase (F1..F5) or compensator (C1..C5) to fail, and the
// orchestrator injects the failure at the corresponding step boundary.
//
// The hook is gated behind the SAGA_FAULT_INJECTION environment variable and
// must never be active in a release build: cmd/main.go refuses to start when
// the variable is set while gin runs in release mode.
package faultinject

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	HeaderForceFail           = "X-Saga-Force-Fail"
	HeaderForceFailKind       = "X-Saga-Force-Fail-Kind"
	HeaderCompensateFail      = "X-Saga-Compensate-Fail"
	HeaderCompensateFailTimes = "X-Saga-Compensate-Fail-Times"
	HeaderInjectDelay         = "X-Saga-Inject-Delay"

	KindBefore = "before"
	KindAfter  = "after"

	envToggle = "SAGA_FAULT_INJECTION"
)

// Spec is the parsed fault plan for one saga execution. It is persisted as
// JSON on the saga row so injected faults survive coordinator restarts, and
// mutated in place as one-shot faults are consumed.
type Spec struct {
	// ForceFailStep names the forward phase (F1..F5) to fail. With kind
	// "before" the phase fails without applying its side effects; with
	// "after" the side effects are applied and persisted first, simulating
	// a crash between the phase and its bookkeeping.
	ForceFailStep string `json:"force_fail_step,omitempty"`
	ForceFailKind string `json:"force_fail_kind,omitempty"`
	ForceFailUsed bool   `json:"force_fail_used,omitempty"`

	// CompensateFailStep names the compensator (C1..C5) that fails
	// CompensateFailRemaining times before starting to succeed.
	CompensateFailStep      string `json:"compensate_fail_step,omitempty"`
	CompensateFailRemaining int    `json:"compensate_fail_remaining,omitempty"`

	// DelayStep/DelayMs pause the named forward phase, opening a window for
	// external chaos (service pause, coordinator kill).
	DelayStep string `json:"delay_step,omitempty"`
	DelayMs   int    `json:"delay_ms,omitempty"`
}

// Enabled reports whether the fault-injection hook is switched on for this
// process.
func Enabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(envToggle)))
	return v == "1" || v == "true" || v == "enabled"
}

// GuardStartup returns an error when fault injection is enabled in a build
// that must not carry it (gin release mode). Call it during service startup.
func GuardStartup() error {
	if Enabled() && gin.Mode() == gin.ReleaseMode {
		return fmt.Errorf("%s is enabled but the service is running in release mode; refusing to start", envToggle)
	}
	return nil
}

var forwardSteps = map[string]bool{"F1": true, "F2": true, "F3": true, "F4": true, "F5": true}
var compensationSteps = map[string]bool{"C1": true, "C2": true, "C3": true, "C4": true, "C5": true}

// FromHeaders parses the X-Saga-* headers into a Spec. It returns nil when no
// fault headers are present and an error when they are malformed.
func FromHeaders(h http.Header) (*Spec, error) {
	spec := &Spec{}
	present := false

	if v := strings.TrimSpace(h.Get(HeaderForceFail)); v != "" {
		step := strings.ToUpper(v)
		if !forwardSteps[step] {
			return nil, fmt.Errorf("%s: unknown forward step %q", HeaderForceFail, v)
		}
		spec.ForceFailStep = step
		spec.ForceFailKind = KindBefore
		present = true
	}

	if v := strings.TrimSpace(h.Get(HeaderForceFailKind)); v != "" {
		kind := strings.ToLower(v)
		if kind != KindBefore && kind != KindAfter {
			return nil, fmt.Errorf("%s: must be %q or %q", HeaderForceFailKind, KindBefore, KindAfter)
		}
		if spec.ForceFailStep == "" {
			return nil, fmt.Errorf("%s requires %s", HeaderForceFailKind, HeaderForceFail)
		}
		spec.ForceFailKind = kind
	}

	if v := strings.TrimSpace(h.Get(HeaderCompensateFail)); v != "" {
		step := strings.ToUpper(v)
		if !compensationSteps[step] {
			return nil, fmt.Errorf("%s: unknown compensation step %q", HeaderCompensateFail, v)
		}
		spec.CompensateFailStep = step
		spec.CompensateFailRemaining = 1
		present = true
	}

	if v := strings.TrimSpace(h.Get(HeaderCompensateFailTimes)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("%s: must be a non-negative integer", HeaderCompensateFailTimes)
		}
		if spec.CompensateFailStep == "" {
			return nil, fmt.Errorf("%s requires %s", HeaderCompensateFailTimes, HeaderCompensateFail)
		}
		spec.CompensateFailRemaining = n
	}

	if v := strings.TrimSpace(h.Get(HeaderInjectDelay)); v != "" {
		step, ms, ok := strings.Cut(v, ":")
		step = strings.ToUpper(strings.TrimSpace(step))
		msStr := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(ms)), "ms")
		n, err := strconv.Atoi(msStr)
		if !ok || !forwardSteps[step] || err != nil || n < 0 {
			return nil, fmt.Errorf("%s: expected format Fi:Nms", HeaderInjectDelay)
		}
		spec.DelayStep = step
		spec.DelayMs = n
		present = true
	}

	if !present {
		return nil, nil
	}
	return spec, nil
}

// Marshal serializes the spec for persistence on the saga row.
func (s *Spec) Marshal() string {
	if s == nil {
		return ""
	}
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// Unmarshal parses a persisted spec; it returns nil for an empty value.
func Unmarshal(raw string) *Spec {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var s Spec
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil
	}
	return &s
}

type ctxKey struct{}

// WithSpec attaches a fault spec to the context.
func WithSpec(ctx context.Context, spec *Spec) context.Context {
	return context.WithValue(ctx, ctxKey{}, spec)
}

// SpecFromContext returns the fault spec attached to the context, if any.
func SpecFromContext(ctx context.Context) *Spec {
	spec, _ := ctx.Value(ctxKey{}).(*Spec)
	return spec
}

// Middleware parses X-Saga-* headers into the request context. When the hook
// is disabled the headers are rejected outright so a misconfigured deployment
// cannot silently ignore adversarial input.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		spec, err := FromHeaders(c.Request.Header)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if spec == nil {
			c.Next()
			return
		}
		if !Enabled() {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "saga fault injection is not enabled"})
			return
		}
		c.Request = c.Request.WithContext(WithSpec(c.Request.Context(), spec))
		c.Next()
	}
}
