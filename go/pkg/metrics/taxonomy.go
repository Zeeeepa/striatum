package metrics

// RFC 0137 Phase B — the failure-mode-shaped taxonomy.
//
// These are CREATE-new, closed Go enums; they do NOT exist as source constants
// today. They are ANCHORED to the real lifecycle/recovery constants that do exist
// (go/pkg/mutations) by a union guardrail test that lives in package mutations
// (where the unexported constants are visible) and imports this package — see
// TestNecrosisDomainMatchesConfirmedDeadConstants. The import direction is
// mutations -> metrics; this package never imports mutations, so there is no
// cycle (RFC 0137 §"Avoid import cycles").
//
// The split between apoptosis (healthy programmed self-termination) and necrosis
// (confirmed-dead, uncontrolled death) is the spine of the taxonomy: the two
// share the same terminal DB transition, so the distinction is decided here at
// fold time from the durable event the terminator / recovery path wrote
// (classifyLifecycleEvent), and the necrosis set is pinned to EXACTLY the
// confirmed-dead classes so a new stall class cannot silently enter it.

// Origin is the closed, low-cardinality enum every failure-mode family carries.
// A flood of one origin value (e.g. a phantom-supervisor storm, RFC 0137 #417)
// is itself the leading indicator, with no bespoke metric required.
type Origin string

const (
	OriginDaemonCore     Origin = "daemon-core"
	OriginReconcileSweep Origin = "reconcile-sweep"
	OriginSupervisor     Origin = "supervisor"
	OriginLane           Origin = "lane"
)

// originDomain is the closed, sorted Origin enum. Sorted so any rendered series
// order derived from it is byte-stable for the golden test.
var originDomain = []Origin{
	OriginDaemonCore,
	OriginLane,
	OriginReconcileSweep,
	OriginSupervisor,
}

// OriginDomain returns a copy of the closed Origin enum.
func OriginDomain() []Origin {
	out := make([]Origin, len(originDomain))
	copy(out, originDomain)
	return out
}

func isCanonicalOrigin(o Origin) bool {
	for _, c := range originDomain {
		if c == o {
			return true
		}
	}
	return false
}

// ApoptosisReason is the closed set of HEALTHY programmed self-termination
// reasons. Healthy termination is the terminator declaring intent — it must
// never be confused with damage (necrosis).
type ApoptosisReason string

const (
	ApoptosisRunCompleted       ApoptosisReason = "run_completed"
	ApoptosisJobSucceeded       ApoptosisReason = "job_succeeded"
	ApoptosisLeaseHandoff       ApoptosisReason = "lease_handoff"
	ApoptosisSupervisorDrained  ApoptosisReason = "supervisor_drained"
	ApoptosisSessionClosedClean ApoptosisReason = "session_closed_clean"
)

// apoptosisDomain is the closed, sorted apoptosis-reason enum.
var apoptosisDomain = []ApoptosisReason{
	ApoptosisJobSucceeded,
	ApoptosisLeaseHandoff,
	ApoptosisRunCompleted,
	ApoptosisSessionClosedClean,
	ApoptosisSupervisorDrained,
}

// ApoptosisReasons returns a copy of the closed apoptosis-reason enum.
func ApoptosisReasons() []ApoptosisReason {
	out := make([]ApoptosisReason, len(apoptosisDomain))
	copy(out, apoptosisDomain)
	return out
}

// NecrosisReason is the CONFIRMED-DEAD set ONLY. This domain is anchored to the
// real mutations constants (TestNecrosisDomainMatchesConfirmedDeadConstants):
//
//   - agent_pid_dead          — mutations.stallClassAgentPIDDead
//     (recovery_decision_tree.go): the supervised agent process/pane is dead and
//     never engaged the work protocol.
//   - agent_exited_unsealed   — mutations.stallClassAgentExitedUnsealed
//     (recovery_decision_tree.go): the agent engaged the protocol and emitted
//     output, then its process died without calling work.complete.
//   - recovery_exhausted      — mutations.recoveryExhaustedBlockerKind
//     (recovery_escalation.go): autonomous recovery could not reclaim the job
//     within its per-job budget.
//   - session_unrecoverable_across_rotation — mutations.stallClassSessionUnrecoverableAcrossRotation
//     (recovery_decision_tree.go, RFC 0143 Slice A / #512): a confirmed-dead lane
//     whose owning session was locked out of resealing across a daemon boot-epoch
//     rotation. A strict refinement of agent_exited_unsealed, so it is also a
//     confirmed-dead necrosis reason.
//
// F-A6 (RFC 0137 §"design-run hardening"): liveness_deadline_missed is a
// REVERSIBLE pre-death observation (session.liveness_recovered proves it is not
// death) and is deliberately EXCLUDED from this set — it is routed to
// striatum_liveness_deadline_events_total instead. A recoverable stall moves the
// liveness-events counter, NEVER necrosis_total.
type NecrosisReason string

const (
	NecrosisAgentPIDDead        NecrosisReason = "agent_pid_dead"
	NecrosisAgentExitedUnsealed NecrosisReason = "agent_exited_unsealed"
	NecrosisRecoveryExhausted   NecrosisReason = "recovery_exhausted"
	// NecrosisSessionUnrecoverableAcrossRotation is the RFC 0143 Slice A (#512)
	// rotation-lockout refinement: a confirmed-dead lane whose owning session was
	// locked out of resealing across a daemon boot-epoch rotation. Additive.
	NecrosisSessionUnrecoverableAcrossRotation NecrosisReason = "session_unrecoverable_across_rotation"
)

// necrosisDomain is the closed, sorted necrosis-reason enum. The guardrail test
// asserts this set equals EXACTLY the confirmed-dead mutations constants.
var necrosisDomain = []NecrosisReason{
	NecrosisAgentExitedUnsealed,
	NecrosisAgentPIDDead,
	NecrosisRecoveryExhausted,
	NecrosisSessionUnrecoverableAcrossRotation,
}

// NecrosisReasons returns a copy of the closed necrosis-reason enum.
func NecrosisReasons() []NecrosisReason {
	out := make([]NecrosisReason, len(necrosisDomain))
	copy(out, necrosisDomain)
	return out
}

func isNecrosisReason(r NecrosisReason) bool {
	for _, c := range necrosisDomain {
		if c == r {
			return true
		}
	}
	return false
}

// LivenessDeadlineReason is the closed set of reasons for the non-terminal
// liveness-events counter (F-A6). deadline_missed and recovered are a reversible
// pair OUTSIDE the apoptosis/necrosis conservation law.
type LivenessDeadlineReason string

const (
	LivenessDeadlineMissed    LivenessDeadlineReason = "deadline_missed"
	LivenessDeadlineRecovered LivenessDeadlineReason = "recovered"
)

var livenessDeadlineDomain = []LivenessDeadlineReason{
	LivenessDeadlineMissed,
	LivenessDeadlineRecovered,
}

// LivenessDeadlineReasons returns a copy of the closed liveness-event reason enum.
func LivenessDeadlineReasons() []LivenessDeadlineReason {
	out := make([]LivenessDeadlineReason, len(livenessDeadlineDomain))
	copy(out, livenessDeadlineDomain)
	return out
}

// LifecycleClass is the apoptosis/necrosis split, plus the non-terminal liveness
// class (F-A6) and a "skip" sentinel for events that are neither (e.g. a
// recovery transfer-close of an honestly-stalled-but-not-dead lane).
type LifecycleClass string

const (
	ClassApoptosis LifecycleClass = "apoptosis"
	ClassNecrosis  LifecycleClass = "necrosis"
	ClassLiveness  LifecycleClass = "liveness"
	ClassSkip      LifecycleClass = ""
)

// Lifecycle-metric tag values stamped on durable termination events at the site
// (RFC 0137 Phase B deliverable #4). Only the genuinely AMBIGUOUS terminal event
// — session.closed, which is emitted for both clean closes and recovery-driven
// dead-lane closes — needs an explicit tag; every other terminal event is
// unambiguous by its event_type / blocker_kind and is classified directly.
const (
	// LifecycleTagKey is the payload key the recovery path stamps on a
	// session.closed event to declare the lifecycle-metric class at the site.
	LifecycleTagKey = "lifecycle_metric"
	// LifecycleTagNecrosis marks a confirmed-dead recovery close (necrosis).
	LifecycleTagNecrosis = "necrosis"
	// LifecycleTagRecoveryTransfer marks a recovery close of an honestly-stalled
	// but NOT confirmed-dead lane — neither a clean apoptosis nor a necrosis, so
	// the fold skips it (it shows up in lease_transitions instead).
	LifecycleTagRecoveryTransfer = "recovery_transfer"
)

// LifecycleEvent is one durable striatumd.events row as the sweep-tick fold sees
// it. Only the event_type and a small set of CLOSED-ENUM payload fields are
// carried; the fold derives the closed-enum metric labels from them and NOTHING
// else reaches the wire (no run/job/session id, path, branch, sha, prompt, or
// byline) — the same defense-in-depth redaction contract as RunObservation.
type LifecycleEvent struct {
	EventType    string // striatumd.events.event_type
	StallClass   string // payload stall_class (confirmed-dead classification)
	BlockerKind  string // payload blocker_kind (recovery_exhausted)
	LifecycleTag string // payload lifecycle_metric tag (stamped at the site)
	// LeaseReason / LeaseTransfer are read from a lease.released payload only and
	// decide whether that release is a clean HANDOFF (apoptosis lease_handoff) or
	// a completion/expiry release the lifecycle fold skips. They stay empty/false
	// for every other event type.
	LeaseReason   string // lease.released payload reason
	LeaseTransfer bool   // lease.released payload transfer flag
}

// leaseStateDomain is the closed enum for the lease_transitions from/to labels
// (striatumd.leases.state values across the migrations). Any value outside it
// buckets to leaseStateOther, so the from/to labels cannot grow cardinality.
var leaseStateDomain = map[string]bool{
	"active":      true,
	"released":    true,
	"expired":     true,
	"removed":     true,
	"abandoned":   true,
	"stale_lease": true,
}

const leaseStateOther = "other"

func bucketLeaseState(s string) string {
	if s == "" {
		return leaseStateOther
	}
	if leaseStateDomain[s] {
		return s
	}
	return leaseStateOther
}

// leaseReasonBucket maps the many raw release_reason literals (lifecycle.go /
// recovery.go) into a CLOSED, low-cardinality category enum. This abstraction is
// the privacy/cardinality contract for the reason label: a new raw reason cannot
// grow the series count, and the bucketed category is still enough to tell a
// clean completion from a recovery requeue/transfer storm (RFC 0137 §3
// "stale-lease storms, dead-agent-recovery thrash").
var leaseReasonBucket = map[string]string{
	"completed":                     "completion",
	"auto_finalized":                "completion",
	"auto_published":                "completion",
	"operator_complete_stalled":     "completion",
	"verdict":                       "review_outcome",
	"reject":                        "review_outcome",
	"needs_revision":                "review_outcome",
	"recovery_requeue":              "recovery_requeue",
	"test_requeue":                  "recovery_requeue",
	"recovery_transfer":             "recovery_transfer",
	"operator_transfer":             "recovery_transfer",
	"recovery_quarantine":           "recovery_quarantine",
	"expired":                       "expiry",
	"blocked":                       "blocked",
	"canceled":                      "canceled",
	"operator_release_before_close": "operator",
}

const leaseReasonOther = "other"

func bucketLeaseReason(reason string) string {
	if reason == "" {
		return leaseReasonOther
	}
	if b, ok := leaseReasonBucket[reason]; ok {
		return b
	}
	return leaseReasonOther
}

// leaseTransitionTarget derives the closed-enum `to` state and the RAW transition
// reason for a durable lease.* event the lease_transitions fold sees. Leases only
// ever transition out of 'active', so `from` is fixed by the caller; this decides
// the destination state and the reason that addLeaseTransition then buckets.
//
//   - lease.released targets the 'released' state and keeps its payload reason.
//   - lease.expired normally targets 'expired'. BUT a repo-write lane whose lease
//     expires is parked in stale_lease limbo, not re-queued: the expiry path stamps
//     job_state="stale_lease" on the event (go/pkg/mutations/recovery.go) while the
//     lease row goes to 'expired'. That stale-lease parking is the RFC 0137 primary
//     stale-lease storm signal, so it must render as a DISTINCT to="stale_lease"
//     series an operator can alert on — collapsing it into to="expired" (the prior
//     behavior) made the signal unobservable and left the stale_lease enum member
//     dead (prior-review F1). A non-repo-write expiry (job re-queued) stays
//     to="expired" so ordinary expiry remains distinct.
//   - lease.expired carries no `reason` payload field, but its lease row's
//     release_reason is literally 'expired' (recovery.go), so the reason is pinned
//     to "expired" (which buckets to "expiry") rather than falling into the generic
//     "other" bucket the empty reason would otherwise hit.
func leaseTransitionTarget(eventType, reason, jobState string) (to string, transitionReason string) {
	if eventType == "lease.expired" {
		if jobState == "stale_lease" {
			return "stale_lease", "expired"
		}
		return "expired", "expired"
	}
	return "released", reason
}

// leaseHandoffReasons is the CLOSED set of lease.released reasons that mean the
// lane's hold was cleanly HANDED OFF to a fresh session (a supersession or an
// operator/recovery transfer) rather than completed or failed. A lease.released
// carrying one of these — or the payload transfer flag — is the durable source
// for the apoptosis lease_handoff reason: the emit sites are the superseding
// close (lifecycle.go releasing a displaced session's lease with
// reason="superseded") and the --transfer release (lifecycle.go, transfer=true).
// A completion/expiry release is NOT a handoff (completion is already counted via
// job.completed; expiry surfaces in lease_transitions), so it is excluded here so
// lease_handoff stays a healthy-handoff signal rather than a generic release tally.
var leaseHandoffReasons = map[string]bool{
	"superseded":        true,
	"recovery_transfer": true,
	"operator_transfer": true,
}

func isLeaseHandoffReason(reason string) bool {
	return leaseHandoffReasons[reason]
}

// classifyLifecycleEvent maps a durable event to its (class, origin, reason)
// metric coordinates. ok=false (ClassSkip) means the event is not a
// lifecycle/liveness signal this fold counts. This is the SINGLE point where the
// apoptosis/necrosis split is read: the terminator that declared intent wrote a
// healthy-termination event type (apoptosis); only the recovery/liveness paths
// that detected an unannounced exit wrote a necrosis tag / confirmed-dead stall
// class / recovery_exhausted blocker kind.
func classifyLifecycleEvent(ev LifecycleEvent) (LifecycleClass, Origin, string, bool) {
	switch ev.EventType {
	case "run.completed":
		return ClassApoptosis, OriginDaemonCore, string(ApoptosisRunCompleted), true
	case "job.completed":
		return ClassApoptosis, OriginLane, string(ApoptosisJobSucceeded), true
	case "session.closed":
		// Necrosis when the recovery path tagged it OR it carries a confirmed-dead
		// stall class. A recovery TRANSFER close (honest stall, not dead) is skipped;
		// any other session.closed is a clean programmed close (apoptosis).
		if ev.LifecycleTag == LifecycleTagNecrosis || isNecrosisReason(NecrosisReason(ev.StallClass)) {
			reason := NecrosisReason(ev.StallClass)
			if !isNecrosisReason(reason) {
				return ClassSkip, "", "", false
			}
			return ClassNecrosis, OriginReconcileSweep, string(reason), true
		}
		if ev.LifecycleTag == LifecycleTagRecoveryTransfer {
			return ClassSkip, "", "", false
		}
		return ClassApoptosis, OriginLane, string(ApoptosisSessionClosedClean), true
	case "supervisor.stopped":
		// A supervisor stop is a programmed drain/termination of the supervisor's
		// own lifecycle (operator stop OR the reconcile sweep reaping a terminal-run
		// supervisor) — healthy apoptosis, origin supervisor. This wires the
		// previously-hollow supervisor_drained reason to its real durable event
		// (mutations.go / supervision_control.go emit "supervisor.stopped").
		return ClassApoptosis, OriginSupervisor, string(ApoptosisSupervisorDrained), true
	case "lease.released":
		// A handoff release (operator/recovery --transfer, or a supersession) is a
		// clean programmed release of the lane's hold so a fresh session can pick the
		// work up — apoptosis lease_handoff. This wires the previously-hollow
		// lease_handoff reason to its real durable event (lifecycle.go emits
		// "lease.released" carrying reason / transfer). A completion or expiry
		// release is not a handoff and is skipped here.
		if ev.LeaseTransfer || isLeaseHandoffReason(ev.LeaseReason) {
			return ClassApoptosis, OriginLane, string(ApoptosisLeaseHandoff), true
		}
		return ClassSkip, "", "", false
	case "session.liveness_deadline_missed":
		// F-A6: a reversible pre-death observation — liveness counter, never necrosis.
		return ClassLiveness, OriginLane, string(LivenessDeadlineMissed), true
	case "session.liveness_recovered":
		// F-A6: the recovery half of the reversible pair.
		return ClassLiveness, OriginLane, string(LivenessDeadlineRecovered), true
	case "run.escalated", "recovery.job_quarantined":
		if NecrosisReason(ev.BlockerKind) == NecrosisRecoveryExhausted {
			return ClassNecrosis, OriginReconcileSweep, string(NecrosisRecoveryExhausted), true
		}
		return ClassSkip, "", "", false
	}
	return ClassSkip, "", "", false
}
