# Lane Health Module Design (RFC 0091) — Handoff Report

## 1. Observation

During our comprehensive read-only investigation of the `striatum` codebase, we located and analyzed the following ad-hoc liveness, attestation, and delivery checks, as well as duplicate parsing logic:

### A. Ad-hoc Lane Attestation & Liveness in Mutation Paths
* **File & Line Range**: `go/pkg/mutations/mutations.go` lines 654 to 726 (`sessionLaneAttestation`).
* **Observation**:
  ```go
  func sessionLaneAttestation(ctx context.Context, runner any, repositoryID, sessionID string) map[string]any {
      row, err := oneRow(ctx, runner, `
          SELECT ps.supervisor_id,
                 ps.pid,
                 ...
            FROM striatumd.process_supervisors ps
            LEFT JOIN striatumd.process_supervisor_pointers p ...
            LEFT JOIN striatumd.daemon_supervisors ds ...
           WHERE ps.repository_id = $1 AND ps.session_id = $2 AND ps.state = 'attached'
           ...`)
      ...
      live := gosupervisor.ProbeLaneLiveness(ctx, supervisionTmuxRunner, metadata, pid, attestationText(row["pid_start_time"]))
      if !live.Alive {
          ...
      }
      if live.Backed == "tmux" && live.Detail == "start_token_unverified" {
          return unattestedLane(supervisorID, pid, "start_token_unverified")
      }
      ...
  }
  ```
  This is a custom database query followed by a sequence of validation checks to return an ad-hoc compatible `map[string]any` with keys `attested`, `state`, `supervisor_id`, `pid`, `reason`, and `liveness`.

### B. Duplicate Lane Attestation in Read Paths
* **File & Line Range**: `go/pkg/reads/supervision.go` lines 861 to 884 (`applySupervisorLaneAttestation`).
* **Observation**:
  ```go
  func applySupervisorLaneAttestation(view map[string]any, hasSupervisor bool, live gosupervisor.LaneLiveness) {
      if !hasSupervisor {
          view["lane_attestation"] = "unattested"
          view["lane_attestation_reason"] = "no_attached_supervisor"
          return
      }
      if tmuxStartTokenUnverified(live) {
          view["lane_attestation"] = "unattested"
          view["lane_attestation_reason"] = "start_token_unverified"
          return
      }
      if !live.Alive {
          reason := live.Class
          if reason == "" {
              reason = "pid_gone"
          }
          view["lane_attestation"] = "unattested"
          view["lane_attestation_reason"] = reason
          return
      }
      view["lane_attestation"] = "attested"
      view["lane_attestation_reason"] = nil
  }
  ```
  The logic for mapping `start_token_unverified` and evaluating `!live.Alive` classes is duplicated verbatim from the mutation layer.

### C. Duplicate Attestation Logic in Read Detail Views
* **File & Line Range**: `go/pkg/reads/supervision.go` lines 190 to 209 (inside `HandleSuperviseShow` / `HandleSuperviseStatus`).
* **Observation**:
  ```go
  reattachAttested := reattachState == "" || reattachState == "reattachable"
  startTokenUnverified := tmuxStartTokenUnverified(laneLive)
  if state == "attached" && liveness == "alive" && reattachAttested && !startTokenUnverified {
      supervisor["lane_attestation"] = "attested"
      supervisor["lane_attestation_reason"] = nil
      return supervisor, nil
  }
  supervisor["lane_attestation"] = "unattested"
  switch {
  case liveness == "stalled":
      supervisor["lane_attestation_reason"] = "supervisor_stalled"
  case pidIdentityReason == "pid_identity_mismatch" || pidIdentityReason == "pid_identity_unavailable":
      supervisor["lane_attestation_reason"] = pidIdentityReason
  case startTokenUnverified:
      supervisor["lane_attestation_reason"] = "start_token_unverified"
  ...
  ```
  This is a second, parallel derivation of lane attestation using `reattachState` and custom flags in the read path.

### D. Ad-hoc Delivery Pre-Checks & Database Bypass in Mutation Control
* **File & Line Range**: `go/pkg/mutations/supervision_control.go` lines 1009 to 1095 (`reconcileSupervisorForDelivery`).
* **Observation**:
  ```go
  func reconcileSupervisorForDelivery(ctx context.Context, runner db.TxRunner, repositoryID string, supervisor supervisorControlRow, phase string) error {
      ...
      if reason, degraded := supervisorDeliveryDegraded(supervisor.Metadata); degraded {
          return rpc.NewError("invalid_transition", "supervisor delivery is degraded: "+reason, nil)
      }
      live := gosupervisor.ProbeLaneLiveness(ctx, supervisionTmuxRunner, supervisor.Metadata, supervisor.PID, supervisor.PIDStartTime)
      ...
      err := runner.QueryRow(ctx, `
          SELECT state, daemon_supervisor_id
            FROM striatumd.process_supervisor_pointers
           WHERE repository_id = $1 AND supervisor_id = $2
           FOR UPDATE`, ...).Scan(...)
      ...
  }
  ```
  Here, the delivery pre-checks are fully ad-hoc, manual, and bypass standard domain stores with direct pgx `QueryRow` invocations inside the transaction.

### E. Ad-hoc Target Validation in Interrogation Paths
* **File & Line Range**: `go/pkg/mutations/interrogation.go` lines 387 to 417 (`requireLiveTarget`).
* **Observation**:
  ```go
  func requireLiveTarget(ctx context.Context, runner any, repositoryID, targetSessionID string) error {
      ...
      attestation := sessionLaneAttestation(ctx, runner, repositoryID, targetSessionID)
      if attested, _ := attestation["attested"].(bool); attested {
          return nil
      }
      ...
  }
  ```
  This requires the live interrogation target to be attested, relying on the legacy `sessionLaneAttestation` map shape.

### F. Duplicate Attestation and Liveness Checks in Status Read Paths
* **File & Line Range**: `go/pkg/reads/status.go` lines 263 to 279 (inside `statusActiveSessions`).
* **Observation**:
  ```go
  if !live.Alive {
      row["lane_attestation"] = "unattested"
      row["lane_attestation_reason"] = live.Class
      continue
  }
  if tmuxStartTokenUnverified(live) {
      row["lane_attestation"] = "unattested"
      row["lane_attestation_reason"] = "start_token_unverified"
      continue
  }
  row["lane_attestation"] = "attested"
  row["lane_attestation_reason"] = nil
  ```
  This is yet another ad-hoc replication of the exact attestation rules.

---

## 2. Logic Chain

1. **Rule Parity Friction**: The rule `Attested = Bound && Alive && start_token_verified` is derived in two packages (`pkg/mutations` and `pkg/reads`) and spread over at least 5 functions (`sessionLaneAttestation`, `applySupervisorLaneAttestation`, `HandleSuperviseShow`, `statusActiveSessions`, `reconcileSupervisorForDelivery`).
2. **Maintenance Drag**: If new attestation/liveness axes or fallback policies are added in the future (e.g. byline/lease updates), the maintainer would have to update and align all 5 distinct parsing sites manually, which is highly error-prone and a major bug surface.
3. **Seams for Testability**: Currently, testing these rules requires mocking the global `supervisionTmuxRunner` or using real database/process fixtures because the load logic and structural rules are coupled. A pure domain classifier (`lanehealth.Classify`) completely decouples evaluation rules from database details and process execution.
4. **Resolution**: By introducing the new `go/pkg/lanehealth` module, we can isolate and unify these rules behind a single interface (`Checker.Check`) and a pure, zero-dependency classifier (`Classify`), preserving wire formatting exactly through `LegacyMap`.

---

## 3. Caveats

* **Transactional Locking**: As specified by RFC 0091, `lanehealth` does NOT manage transactional locks (such as `FOR UPDATE` queries inside mutations). The callers (`reconcileSupervisorForDelivery`) must still acquire their transactional locks explicitly prior to invoking the checker.
* **Scope Bounds**: No changes are proposed to the underlying PostgreSQL schema or the RPC JSON wire contracts. All mapped values are translated into identical legacy representations.
* **Network/Process Dependency**: In production, the checker relies on `supervisor.ProbeLaneLiveness` which performs standard signals or tmux CLI calls. In test suites, a test implementation of `Probe` interface will return static/canned liveness objects.

---

## 4. Conclusion

We propose the following full design and interface layout for the new `go/pkg/lanehealth` module and `supervisor.TmuxMeta` structure.

### A. The Single Metadata Codec: `go/pkg/supervisor/tmux_meta.go`
To prevent circular dependency packages between `lanehealth` and `supervisor`, the metadata structure is defined inside `go/pkg/supervisor`:

```go
package supervisor

import "encoding/json"

// TmuxMeta is the single, structured codec for supervisor pointer metadata blocks.
type TmuxMeta struct {
	AgentLoopMode    string                 `json:"agent_loop_mode,omitempty"`
	Tmux             TmuxMetaBlock          `json:"tmux,omitempty"`
	DeliveryLiveness *DeliveryLivenessBlock `json:"delivery_liveness,omitempty"`
}

type TmuxMetaBlock struct {
	State                  string                 `json:"state,omitempty"`
	SessionName            string                 `json:"session_name,omitempty"`
	WindowID               string                 `json:"window_id,omitempty"`
	PaneID                 string                 `json:"pane_id,omitempty"`
	PaneStartToken         string                 `json:"pane_start_token,omitempty"`
	AttachCommand          string                 `json:"attach_command,omitempty"`
	UnavailableReason      string                 `json:"unavailable_reason,omitempty"`
	CapturedAt             string                 `json:"captured_at,omitempty"`
	ProbeSkippedAt         string                 `json:"probe_skipped_at,omitempty"`
	LastOkAt               string                 `json:"last_ok_at,omitempty"`
	LastUnavailableDetail  string                 `json:"last_unavailable_detail,omitempty"`
	LivenessState          string                 `json:"liveness_state,omitempty"`
	PanePID                int                    `json:"pane_pid,omitempty"`
	AttachClientPID        int                    `json:"attach_client_pid,omitempty"`
	ProbeUnavailableCount  int                    `json:"probe_unavailable_count,omitempty"`
	DeliveryLiveness       *DeliveryLivenessBlock `json:"delivery_liveness,omitempty"`
	AttachClientLastExit   map[string]any         `json:"attach_client_last_exit,omitempty"`
	Liveness               map[string]any         `json:"liveness,omitempty"`
}

type DeliveryLivenessBlock struct {
	Class       string `json:"class,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ObservedAt  string `json:"observed_at,omitempty"`
	ReportedAt  string `json:"reported_at,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	Healthy     bool   `json:"healthy,omitempty"`
}

// IsValidTmux validates whether the Tmux metadata shape conforms to a backed session.
func (m TmuxMeta) IsValidTmux() bool {
	if m.Tmux.State != "backed" {
		return true // Not backed, so not invalid tmux
	}
	return m.Tmux.PanePID > 0 && m.Tmux.SessionName != "" && m.Tmux.PaneID != ""
}
```

### B. The Unified Lane Health Module: `go/pkg/lanehealth/lanehealth.go`
```go
package lanehealth

import (
	"context"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/sessionliveness"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

type LaneReason string

const (
	ReasonNone                     LaneReason = ""
	ReasonNoAttachedSupervisor     LaneReason = "no_attached_supervisor"
	ReasonPIDMissing               LaneReason = "pid_missing"
	ReasonDaemonSupervisorMissing  LaneReason = "daemon_supervisor_missing"
	ReasonPointerStateMismatch     LaneReason = "pointer_state_mismatch"
	ReasonDaemonStateMismatch      LaneReason = "daemon_state_mismatch"
	ReasonPointerPIDMismatch       LaneReason = "pointer_pid_mismatch"
	ReasonTmuxMetadataCorrupt      LaneReason = "tmux_metadata_corrupt"
	ReasonStartTokenUnverified     LaneReason = "start_token_unverified"
	ReasonPIDGone                  LaneReason = "pid_gone"
	ReasonSupervisorStalled        LaneReason = "supervisor_stalled"
	ReasonPIDIdentityMismatch      LaneReason = "pid_identity_mismatch"
	ReasonPIDIdentityUnavailable   LaneReason = "pid_identity_unavailable"
)

type Health struct {
	Bound, Alive, Attested, Deliverable bool
	Stall         sessionliveness.Result
	Reason        LaneReason
	LivenessClass string
	SupervisorID  string
	PID           int
}

func (h Health) LiveTarget() bool { return h.Attested && h.Alive }
func (h Health) CanDeliver() bool { return h.Alive && h.Deliverable }
func (h Health) IsStalled() bool  { return h.Stall.StallClass != "" }

// Probe is the injected process probe port.
type Probe interface {
	ProbeLane(ctx context.Context, meta gosupervisor.TmuxMeta, pid int, startToken string) gosupervisor.LaneLiveness
}

// ProdProbe is the production adapter wrapping gosupervisor.ProbeLaneLiveness.
type ProdProbe struct {
	Runner gosupervisor.TmuxRunner
}

func (p ProdProbe) ProbeLane(ctx context.Context, meta gosupervisor.TmuxMeta, pid int, startToken string) gosupervisor.LaneLiveness {
	// Reconstruct map shape for legacy compatibility under supervisor package.
	metadata := map[string]any{
		"agent_loop_mode": meta.AgentLoopMode,
	}
	tmux := map[string]any{
		"state":                    meta.Tmux.State,
		"session_name":             meta.Tmux.SessionName,
		"window_id":                meta.Tmux.WindowID,
		"pane_id":                  meta.Tmux.PaneID,
		"pane_start_token":         meta.Tmux.PaneStartToken,
		"attach_command":           meta.Tmux.AttachCommand,
		"unavailable_reason":       meta.Tmux.UnavailableReason,
		"captured_at":              meta.Tmux.CapturedAt,
		"probe_skipped_at":         meta.Tmux.ProbeSkippedAt,
		"last_ok_at":               meta.Tmux.LastOkAt,
		"last_unavailable_detail":  meta.Tmux.LastUnavailableDetail,
		"liveness_state":           meta.Tmux.LivenessState,
		"pane_pid":                 meta.Tmux.PanePID,
		"attach_client_pid":        meta.Tmux.AttachClientPID,
		"probe_unavailable_count":  meta.Tmux.ProbeUnavailableCount,
	}
	if meta.Tmux.DeliveryLiveness != nil {
		tmux["delivery_liveness"] = map[string]any{
			"class":       meta.Tmux.DeliveryLiveness.Class,
			"reason":      meta.Tmux.DeliveryLiveness.Reason,
			"observed_at": meta.Tmux.DeliveryLiveness.ObservedAt,
			"reported_at": meta.Tmux.DeliveryLiveness.ReportedAt,
			"remediation": meta.Tmux.DeliveryLiveness.Remediation,
			"healthy":     meta.Tmux.DeliveryLiveness.Healthy,
		}
	}
	if meta.Tmux.AttachClientLastExit != nil {
		tmux["attach_client_last_exit"] = meta.Tmux.AttachClientLastExit
	}
	if meta.Tmux.Liveness != nil {
		tmux["liveness"] = meta.Tmux.Liveness
	}
	metadata["tmux"] = tmux

	if meta.DeliveryLiveness != nil {
		metadata["delivery_liveness"] = map[string]any{
			"class":       meta.DeliveryLiveness.Class,
			"reason":      meta.DeliveryLiveness.Reason,
			"observed_at": meta.DeliveryLiveness.ObservedAt,
			"reported_at": meta.DeliveryLiveness.ReportedAt,
			"remediation": meta.DeliveryLiveness.Remediation,
			"healthy":     meta.DeliveryLiveness.Healthy,
		}
	}

	return gosupervisor.ProbeLaneLiveness(ctx, p.Runner, metadata, pid, startToken)
}

type Facts struct {
	SupervisorRecorded bool
	SupervisorID       string
	PID                int
	PIDStartTime       string
	SupervisorState    string

	HasPointer               bool
	PointerDaemonSupervisorID string
	PointerPID               int
	PointerPIDStartTime      string
	PointerState             string
	PointerTmuxMeta          gosupervisor.TmuxMeta

	HasDaemonSupervisor bool
	DaemonSupervisorID  string
	DaemonState         string

	ProbePerformed bool
	ProbeResult    gosupervisor.LaneLiveness

	DeliveryDegraded bool
	DeliveryReason   string

	SessionActivity sessionliveness.Activity
	LivenessPolicy  sessionliveness.Policy
}

// Classify computes composite lane health using a pure state machine.
func Classify(f Facts, now time.Time) Health {
	h := Health{
		SupervisorID: f.SupervisorID,
		PID:          f.PID,
		Deliverable:  !f.DeliveryDegraded,
	}

	// 1. Basic attachment checks
	if !f.SupervisorRecorded || f.SupervisorState != "attached" {
		h.Reason = ReasonNoAttachedSupervisor
		return h
	}

	// 2. Process check
	if f.PID <= 0 {
		h.Reason = ReasonPIDMissing
		return h
	}

	// 3. Pointer & Daemon structure validation
	if !f.HasPointer || f.PointerDaemonSupervisorID == "" {
		h.Reason = ReasonDaemonSupervisorMissing
		return h
	}
	if f.PointerState != "attached" {
		h.Reason = ReasonPointerStateMismatch
		return h
	}
	if !f.HasDaemonSupervisor || f.DaemonSupervisorID == "" {
		h.Reason = ReasonDaemonSupervisorMissing
		return h
	}
	if f.DaemonState != "attached" {
		h.Reason = ReasonDaemonStateMismatch
		return h
	}
	if f.PointerPID > 0 && f.PointerPID != f.PID {
		h.Reason = ReasonPointerPIDMismatch
		return h
	}

	// 4. Metadata integrity
	if f.PointerTmuxMeta.Tmux.State == "backed" && !f.PointerTmuxMeta.IsValidTmux() {
		h.Reason = ReasonTmuxMetadataCorrupt
		return h
	}

	h.Bound = true

	// 5. Active probe evaluation
	if f.ProbePerformed {
		h.Alive = f.ProbeResult.Alive
		h.LivenessClass = f.ProbeResult.Class
		if !f.ProbeResult.Alive {
			reason := f.ProbeResult.Class
			if reason == "" {
				reason = "pid_gone"
			}
			h.Reason = LaneReason(reason)
			return h
		}
		if f.ProbeResult.Backed == "tmux" && f.ProbeResult.Detail == "start_token_unverified" {
			h.Reason = ReasonStartTokenUnverified
			return h
		}
	}

	// 6. Stall classification
	h.Stall = sessionliveness.Classify(f.SessionActivity, f.LivenessPolicy, now)

	h.Attested = true
	return h
}

// Checker aggregates database loading with liveness probing.
type Checker struct {
	Probe Probe
}

func (c Checker) Check(ctx context.Context, runner db.Runner, repositoryID, sessionID string) (Health, error) {
	// Execute unified single facts query
	// (Returns all session activities + supervisors + pointers + daemon supervisors)
	// Build Facts struct, call c.Probe.ProbeLane if a live process supervisor is attached,
	// then execute Classify(facts, time.Now().UTC()).
	return Health{}, nil // implementation detail
}

// LegacyMap maps unified health properties into the compatible legacy map.
func LegacyMap(h Health) map[string]any {
	if h.Attested {
		return map[string]any{
			"attested":      true,
			"state":         "attested",
			"supervisor_id": h.SupervisorID,
			"pid":           h.PID,
			"reason":        nil,
			"liveness":      h.LivenessClass,
		}
	}
	var reasonVal any = string(h.Reason)
	if h.Reason == ReasonNone {
		reasonVal = nil
	}
	var supervisorIDVal any = h.SupervisorID
	if h.SupervisorID == "" {
		supervisorIDVal = nil
	}
	var pidVal any = h.PID
	if h.PID <= 0 {
		pidVal = nil
	}
	return map[string]any{
		"attested":      false,
		"state":         "unattested",
		"supervisor_id": nullable(supervisorIDVal),
		"pid":           nullable(pidVal),
		"reason":        reasonVal,
	}
}

func nullable(v any) any {
	if v == nil {
		return nil
	}
	return v
}
```

---

## 5. Verification Method

Once Phase 1 and 2 are implemented, verification must cover the following areas:

### A. Independent Verification Commands
1. **Unit tests for pure classifier**:
   Run tests targeting the pure state machine:
   ```bash
   go test -v ./go/pkg/lanehealth/...
   ```
2. **Integration tests**:
   Verify compatibility of the unified checker over actual database schemas:
   ```bash
   go test -v ./go/pkg/mutations/... -run TestSupervision
   go test -v ./go/pkg/reads/... -run TestSupervision
   ```
3. **Core project smoke verification**:
   ```bash
   make lint
   make typecheck
   make test
   make smoke
   ```

### B. Invalidation Conditions
* The classification is invalid if the exact reason string representation maps deviate from the existing wire definitions (e.g. `start_token_unverified` mapping to a different string or `tmux_metadata_corrupt` disappearing).
* If a database join inside `Checker.Check` causes query slow-downs, we may optimize it using lazy load queries or a structured caching scheme.
