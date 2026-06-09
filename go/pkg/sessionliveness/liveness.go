package sessionliveness

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	LastMCPRequestAt        = "last_mcp_request_at"
	LastToolsListAt         = "last_tools_list_at"
	LastAwaitPacketAt       = "last_await_packet_at"
	LastPacketDeliveredAt   = "last_packet_delivered_at"
	LastAckAt               = "last_ack_at"
	LastWorkBlockAt         = "last_work_block_at"
	LastWorkReleaseAt       = "last_work_release_at"
	LastWorkCompleteAt      = "last_work_complete_at"
	LastWorkHeartbeatAt     = "last_work_heartbeat_at"
	LastSessionReadyAt      = "last_session_ready_at"
	LastSessionHeartbeatAt  = "last_session_heartbeat_at"
	LastSessionQuestionAt   = "last_session_question_at"
	LastSessionEscalateAt   = "last_session_escalate_at"
	LastPTYActivityAt       = "last_pty_activity_at"
	LastToolCallStartedAt   = "last_tool_call_started_at"
	LastToolCallFinishedAt  = "last_tool_call_finished_at"
	LivenessStallClass      = "liveness_stall_class"
	LivenessStallSince      = "liveness_stall_since"
	ActiveLeaseID           = "active_lease_id"
	ActiveLeaseAcquiredAt   = "active_lease_acquired_at"
	ActiveLeaseExpiresAt    = "active_lease_expires_at"
	ActiveLeaseHeartbeatAt  = "active_lease_last_heartbeat_at"
	StallDiscovery          = "agent_mcp_discovery_stall"
	StallAwaitPacket        = "agent_await_packet_stall"
	StallAck                = "agent_ack_stall"
	StallLeaseHeartbeat     = "agent_lease_heartbeat_stall"
	StallQuestionPending    = "agent_question_pending"
	StallEscalationPending  = "agent_escalation_pending"
	StallProtocolIdle       = "agent_protocol_idle_stall"
	DeadlineDiscovery       = "mcp_discovery"
	DeadlineAwaitPacket     = "await_packet"
	DeadlineAck             = "packet_ack"
	DeadlineLeaseHeartbeat  = "lease_heartbeat"
	DeadlineQuestionPending = "question_pending"
	DeadlineEscalation      = "escalation_pending"
	DeadlineProtocolIdle    = "protocol_idle"
	DeadlineToolCall        = "tool_call"
)

// Protocol states (RFC 0101 Phase 1, Layer 1). These are precise, projection-
// only liveness states reported on Result.Protocol. They are NEVER persisted to
// the liveness_stall_class column (which keeps its constrained Stall* enum), so
// they do not interact with the migration 0012 CHECK constraint. They make
// supervise status / dashboard honest about a lane that is working rather than
// collapsing every live lane to a generic "live":
//
//   - ProtocolWorkingProtocol — fresh MCP/protocol activity within its window.
//   - ProtocolWorkingLocal     — protocol quiet but the child PTY is producing
//     output (the #80 honest-local-work case).
//   - ProtocolWorkingTool      — inside an MCP/tool call (started after finished),
//     with a visible since + deadline (the #83 in-tool case).
//   - ProtocolQuiet            — no fresh signal yet, still before the deadline.
//   - ProtocolDead             — the lane never reached the daemon and produced
//     no PTY output past the discovery deadline (the #117 case): report dead,
//     not a misleading agent_mcp_discovery_stall.
const (
	ProtocolInactive        = "inactive"
	ProtocolLive            = "live"
	ProtocolStalled         = "stalled"
	ProtocolAttention       = "attention"
	ProtocolWorkingProtocol = "working_protocol"
	ProtocolWorkingLocal    = "working_local"
	ProtocolWorkingTool     = "working_tool"
	ProtocolQuiet           = "quiet"
	ProtocolDead            = "dead"
)

type execer interface {
	Exec(context.Context, string, ...any) error
}

type Recorder interface {
	RecordSessionActivity(ctx context.Context, repositoryID string, sessionID string, columns ...string) error
}

type DBRecorder struct {
	Runner execer
}

func (r DBRecorder) RecordSessionActivity(ctx context.Context, repositoryID string, sessionID string, columns ...string) error {
	return Record(ctx, r.Runner, repositoryID, sessionID, columns...)
}

type Policy struct {
	DiscoverySeconds int
	// BootstrapGraceSeconds is an extra grace window, added to DiscoverySeconds,
	// before a session that has recorded NO MCP activity at all is flagged
	// agent_mcp_discovery_stall (#192). Real agent CLIs spend a model/session
	// cold-start before their first tools/list — claude routinely exceeds the
	// bare 60s DiscoverySeconds (~56s was measured on a HEALTHY lane that then
	// proceeded normally), so every cold spawn risked a transient false positive
	// that killed naive watchers on their first poll. The grace window only moves
	// the pre-discovery edge; once any MCP activity is recorded the lane is past
	// discovery and the normal (un-graced) protocol rungs apply. DiscoverySeconds
	// itself is unchanged, so the recovery-action discovery deadline and the
	// adapter-conformance C1/C3 budgets that anchor on it are unaffected.
	BootstrapGraceSeconds int
	AwaitPacketSeconds    int
	AckSeconds            int
	LeaseHeartbeatSeconds int
	LeaseHeartbeatSlack   int
	ProtocolIdleSeconds   int
	// ProtocolFreshSeconds is the window within which recent protocol activity
	// counts as working_protocol. PTYFreshSeconds is the window within which
	// recent PTY output counts as working_local. ToolCallSeconds is the visible
	// deadline a lane is allowed to sit inside a single MCP/tool call before the
	// working_tool state crosses its deadline (#83 visible timeout). These drive
	// the precise protocol-state classification only; they never relax the
	// existing stall edges.
	ProtocolFreshSeconds int
	PTYFreshSeconds      int
	ToolCallSeconds      int
}

type Activity struct {
	SessionState           string
	RegisteredAt           *time.Time
	LastMCPRequestAt       *time.Time
	LastToolsListAt        *time.Time
	LastAwaitPacketAt      *time.Time
	LastPacketDeliveredAt  *time.Time
	LastAckAt              *time.Time
	LastWorkBlockAt        *time.Time
	LastWorkReleaseAt      *time.Time
	LastWorkCompleteAt     *time.Time
	LastWorkHeartbeatAt    *time.Time
	LastSessionReadyAt     *time.Time
	LastSessionHeartbeatAt *time.Time
	LastSessionQuestionAt  *time.Time
	LastSessionEscalateAt  *time.Time
	LastPTYActivityAt      *time.Time
	LastToolCallStartedAt  *time.Time
	LastToolCallFinishedAt *time.Time
	PersistedStallClass    string
	PersistedStallSince    *time.Time
	ActiveLeaseID          string
	ActiveLeaseAcquiredAt  *time.Time
	ActiveLeaseExpiresAt   *time.Time
	ActiveLeaseHeartbeatAt *time.Time
}

type Result struct {
	Protocol        string
	Lease           string
	StallClass      string
	StallSince      *time.Time
	DeadlineName    string
	DeadlineSeconds int
	// ToolCallSince and ToolCallDeadline are populated only for the
	// working_tool protocol state (#83): they give the operator a visible
	// timestamp for when the in-flight MCP/tool call started and the wall-clock
	// instant by which it must finish before the lane is treated as stalled
	// inside a hidden tool call. They are nil for every other state.
	ToolCallSince    *time.Time
	ToolCallDeadline *time.Time
}

var allowedColumns = map[string]bool{
	LastMCPRequestAt:       true,
	LastToolsListAt:        true,
	LastAwaitPacketAt:      true,
	LastPacketDeliveredAt:  true,
	LastAckAt:              true,
	LastWorkBlockAt:        true,
	LastWorkReleaseAt:      true,
	LastWorkCompleteAt:     true,
	LastWorkHeartbeatAt:    true,
	LastSessionReadyAt:     true,
	LastSessionHeartbeatAt: true,
	LastSessionQuestionAt:  true,
	LastSessionEscalateAt:  true,
	LastPTYActivityAt:      true,
	LastToolCallStartedAt:  true,
	LastToolCallFinishedAt: true,
}

func DefaultPolicy() Policy {
	return Policy{
		DiscoverySeconds:      60,
		BootstrapGraceSeconds: 120,
		AwaitPacketSeconds:    90,
		AckSeconds:            60,
		LeaseHeartbeatSeconds: 300,
		LeaseHeartbeatSlack:   30,
		ProtocolIdleSeconds:   300,
		ProtocolFreshSeconds:  60,
		PTYFreshSeconds:       60,
		ToolCallSeconds:       180,
	}
}

func Record(ctx context.Context, runner execer, repositoryID string, sessionID string, columns ...string) error {
	if runner == nil || repositoryID == "" || sessionID == "" {
		return nil
	}
	set := map[string]bool{LastMCPRequestAt: true}
	for _, column := range columns {
		if !allowedColumns[column] {
			return fmt.Errorf("unknown session activity column %q", column)
		}
		set[column] = true
	}
	ordered := make([]string, 0, len(set))
	for column := range set {
		ordered = append(ordered, column)
	}
	sort.Strings(ordered)
	assignments := make([]string, 0, len(ordered))
	for _, column := range ordered {
		assignments = append(assignments, column+" = $1")
	}
	now := time.Now().UTC().Truncate(time.Second)
	return runner.Exec(
		ctx,
		"UPDATE striatumd.sessions SET "+strings.Join(assignments, ", ")+" WHERE repository_id = $2 AND session_id = $3",
		now,
		repositoryID,
		sessionID,
	)
}

func ProjectionFromRow(row map[string]any, now time.Time) map[string]any {
	activity := ActivityFromRow(row)
	result := Classify(activity, DefaultPolicy(), now)
	projection := map[string]any{
		"protocol":                       result.Protocol,
		"lease":                          result.Lease,
		"stall_class":                    nullableText(result.StallClass),
		"stall_since":                    timeValue(result.StallSince),
		"deadline_name":                  nullableText(result.DeadlineName),
		"deadline_seconds":               nullableInt(result.DeadlineSeconds),
		"last_mcp_request_at":            timeValue(activity.LastMCPRequestAt),
		"last_tools_list_at":             timeValue(activity.LastToolsListAt),
		"last_await_packet_at":           timeValue(activity.LastAwaitPacketAt),
		"last_packet_delivered_at":       timeValue(activity.LastPacketDeliveredAt),
		"last_ack_at":                    timeValue(activity.LastAckAt),
		"last_work_block_at":             timeValue(activity.LastWorkBlockAt),
		"last_work_release_at":           timeValue(activity.LastWorkReleaseAt),
		"last_work_complete_at":          timeValue(activity.LastWorkCompleteAt),
		"last_work_heartbeat_at":         timeValue(activity.LastWorkHeartbeatAt),
		"last_session_ready_at":          timeValue(activity.LastSessionReadyAt),
		"last_session_heartbeat_at":      timeValue(activity.LastSessionHeartbeatAt),
		"last_session_question_at":       timeValue(activity.LastSessionQuestionAt),
		"last_session_escalate_at":       timeValue(activity.LastSessionEscalateAt),
		"last_session_report_kind":       nullableText(lastSessionReportKind(activity)),
		"last_pty_activity_at":           timeValue(activity.LastPTYActivityAt),
		"last_tool_call_started_at":      timeValue(activity.LastToolCallStartedAt),
		"last_tool_call_finished_at":     timeValue(activity.LastToolCallFinishedAt),
		"tool_call_since":                timeValue(result.ToolCallSince),
		"tool_call_deadline":             timeValue(result.ToolCallDeadline),
		"active_lease_id":                nullableText(activity.ActiveLeaseID),
		"active_lease_expires_at":        timeValue(activity.ActiveLeaseExpiresAt),
		"active_lease_last_heartbeat_at": timeValue(activity.ActiveLeaseHeartbeatAt),
	}
	return projection
}

func ActivityFromRow(row map[string]any) Activity {
	if row == nil {
		row = map[string]any{}
	}
	return Activity{
		SessionState:           stringValue(row["state"]),
		RegisteredAt:           timeFromAny(row["registered_at"]),
		LastMCPRequestAt:       timeFromAny(row[LastMCPRequestAt]),
		LastToolsListAt:        timeFromAny(row[LastToolsListAt]),
		LastAwaitPacketAt:      timeFromAny(row[LastAwaitPacketAt]),
		LastPacketDeliveredAt:  timeFromAny(row[LastPacketDeliveredAt]),
		LastAckAt:              timeFromAny(row[LastAckAt]),
		LastWorkBlockAt:        timeFromAny(row[LastWorkBlockAt]),
		LastWorkReleaseAt:      timeFromAny(row[LastWorkReleaseAt]),
		LastWorkCompleteAt:     timeFromAny(row[LastWorkCompleteAt]),
		LastWorkHeartbeatAt:    timeFromAny(row[LastWorkHeartbeatAt]),
		LastSessionReadyAt:     timeFromAny(row[LastSessionReadyAt]),
		LastSessionHeartbeatAt: timeFromAny(row[LastSessionHeartbeatAt]),
		LastSessionQuestionAt:  timeFromAny(row[LastSessionQuestionAt]),
		LastSessionEscalateAt:  timeFromAny(row[LastSessionEscalateAt]),
		LastPTYActivityAt:      timeFromAny(row[LastPTYActivityAt]),
		LastToolCallStartedAt:  timeFromAny(row[LastToolCallStartedAt]),
		LastToolCallFinishedAt: timeFromAny(row[LastToolCallFinishedAt]),
		PersistedStallClass:    stringValue(row[LivenessStallClass]),
		PersistedStallSince:    timeFromAny(row[LivenessStallSince]),
		ActiveLeaseID:          stringValue(row[ActiveLeaseID]),
		ActiveLeaseAcquiredAt:  timeFromAny(row[ActiveLeaseAcquiredAt]),
		ActiveLeaseExpiresAt:   timeFromAny(row[ActiveLeaseExpiresAt]),
		ActiveLeaseHeartbeatAt: timeFromAny(row[ActiveLeaseHeartbeatAt]),
	}
}

func Classify(activity Activity, policy Policy, now time.Time) Result {
	if activity.SessionState != "" && activity.SessionState != "active" {
		return Result{Protocol: "inactive", Lease: leaseState(activity, "")}
	}
	now = now.UTC()
	if pending, at := attentionPending(activity); pending != "" {
		if pending == StallQuestionPending {
			return stallResult(activity, StallQuestionPending, DeadlineQuestionPending, 0, at)
		}
		return stallResult(activity, StallEscalationPending, DeadlineEscalation, 0, at)
	}
	if result, stalled := staleToolCallResult(activity, policy, now); stalled {
		return result
	}
	if !discovered(activity) {
		// #192: an agent CLI's normal cold start (model/session init before its
		// first tools/list) routinely exceeds the bare DiscoverySeconds, so the
		// pre-discovery stall edge gets a dedicated bootstrap grace window. A lane
		// that has recorded NO MCP activity is only flagged once it misses
		// DiscoverySeconds + BootstrapGraceSeconds — long enough to absorb a real
		// cold start, so a healthy spawn no longer trips a transient false positive
		// on a watcher's first poll.
		bootstrapDeadlineSeconds := policy.DiscoverySeconds + policy.BootstrapGraceSeconds
		if missed(activity.RegisteredAt, bootstrapDeadlineSeconds, now) {
			// #117: a lane that never reached the daemon over MCP AND produced no
			// PTY output past the discovery deadline did not "stall while
			// discovering MCP" — it never produced anything, so for an operator it
			// is dead at spawn, not a lane that is still trying. Report the
			// operator-visible Protocol as "dead" so supervise status / dashboard
			// is honest. We deliberately keep StallClass as the underlying
			// agent_mcp_discovery_stall: it is the persisted enum the liveness
			// sweep records and the recovery-action library keys on (a dead lane
			// past discovery is still recovered via the discovery deadline), and
			// keeping it avoids widening the migration-0012 CHECK constraint. A
			// lane that IS producing PTY output (alive, just slow to bind MCP)
			// keeps the plain discovery-stall classification (Protocol "stalled").
			result := stallResult(activity, StallDiscovery, DeadlineDiscovery, bootstrapDeadlineSeconds, activity.RegisteredAt)
			if !ptyActive(activity) {
				result.Protocol = ProtocolDead
			}
			return result
		}
		return workingResult(activity, policy, now)
	}
	// The await-packet deadline is anchored on LastToolsListAt, so it is only
	// meaningful once tools/list has been recorded. A lane discovered via other
	// MCP activity (LastToolsListAt still nil) must NOT short-circuit to "live"
	// here — doing so would skip the protocol-idle catch-all and let a lane that
	// pinged once then died read as live forever (#63 F4 regression guard).
	if activity.LastToolsListAt != nil && !after(activity.LastAwaitPacketAt, activity.LastToolsListAt) {
		if missed(activity.LastToolsListAt, policy.AwaitPacketSeconds, now) {
			return stallResult(activity, StallAwaitPacket, DeadlineAwaitPacket, policy.AwaitPacketSeconds, activity.LastToolsListAt)
		}
		return workingResult(activity, policy, now)
	}
	if activity.LastPacketDeliveredAt != nil && !hasAckEquivalentAfter(activity, activity.LastPacketDeliveredAt) {
		if missed(activity.LastPacketDeliveredAt, policy.AckSeconds, now) {
			return stallResult(activity, StallAck, DeadlineAck, policy.AckSeconds, activity.LastPacketDeliveredAt)
		}
		return workingResult(activity, policy, now)
	}
	// An active lease is the authoritative liveness signal for a working lane.
	// The lease-heartbeat rung is the terminal classification for any lease
	// holder: the work-lease subsystem requires a heartbeat at least every
	// LeaseHeartbeatSeconds to keep the lease alive, so a lane that is still
	// heartbeating its lease is by definition actively working — even when it
	// is mid-generation and issues no other MCP call for longer than the
	// protocol-idle window (#63 F8). A genuinely dead lease holder stops
	// heartbeating and trips StallLeaseHeartbeat at LeaseHeartbeatSeconds +
	// slack, which the dead process cannot forge; the lease heartbeat cannot be
	// kept fresh without the lane being alive, so this does not weaken
	// dead-lane detection. Lanes with no active lease still fall through to the
	// protocol-idle catch-all below, unchanged.
	if activity.ActiveLeaseID != "" {
		base := latestTime(activity.LastWorkHeartbeatAt, activity.ActiveLeaseHeartbeatAt, activity.ActiveLeaseAcquiredAt)
		threshold := policy.LeaseHeartbeatSeconds + policy.LeaseHeartbeatSlack
		if missed(base, threshold, now) {
			// #145: a lease holder past its heartbeat deadline is NOT stalled if it
			// is demonstrably still producing output — fresh PTY frames
			// (working_local) or inside an MCP/tool call (working_tool). A long
			// foreground command (a full test suite, a browser-acceptance profile)
			// emits no work-heartbeat for minutes while the PTY/tool timeline stays
			// fresh; tripping StallLeaseHeartbeat there falsely transfers an
			// actively-working lane (the recovery decision tree's CASE 2 closes its
			// session mid-work and loses the artifact). This mirrors the
			// protocol-idle rung below, which already folds last_pty_activity_at into
			// its base, so the same G2 invariant ("never report stalled while
			// demonstrably producing output") holds for a lease holder too. A lane
			// that goes quiet past the PTY-fresh window resolves to ProtocolQuiet and
			// still trips the stall, preserving dead-lane detection.
			if working := workingResult(activity, policy, now); working.Protocol != ProtocolQuiet {
				return working
			}
			return stallResult(activity, StallLeaseHeartbeat, DeadlineLeaseHeartbeat, threshold, base)
		}
		return workingResult(activity, policy, now)
	}
	base := latestTime(
		activity.LastMCPRequestAt,
		activity.LastToolsListAt,
		activity.LastAwaitPacketAt,
		activity.LastPacketDeliveredAt,
		activity.LastAckAt,
		activity.LastWorkBlockAt,
		activity.LastWorkReleaseAt,
		activity.LastWorkCompleteAt,
		activity.LastWorkHeartbeatAt,
		activity.LastSessionReadyAt,
		activity.LastSessionHeartbeatAt,
		activity.LastSessionQuestionAt,
		activity.LastSessionEscalateAt,
		// RFC 0101 Phase 1 (Layer 1, #80): PTY output the helper recorded is a
		// real progress signal. A lane doing long LOCAL work between MCP calls
		// keeps producing PTY output, so including last_pty_activity_at here keeps
		// it out of agent_protocol_idle_stall and lets it surface as working_local
		// — honoring G2 (never report stalled while demonstrably producing
		// output). A lane that stops producing output still trips the idle stall
		// at ProtocolIdleSeconds, so dead-lane detection is preserved.
		activity.LastPTYActivityAt,
		activity.RegisteredAt,
	)
	if missed(base, policy.ProtocolIdleSeconds, now) {
		return stallResult(activity, StallProtocolIdle, DeadlineProtocolIdle, policy.ProtocolIdleSeconds, base)
	}
	return workingResult(activity, policy, now)
}

func staleToolCallResult(activity Activity, policy Policy, now time.Time) (Result, bool) {
	inTool, since := inToolCall(activity)
	if !inTool || policy.ToolCallSeconds <= 0 || !missed(since, policy.ToolCallSeconds, now) {
		return Result{}, false
	}
	result := stallResult(activity, StallProtocolIdle, DeadlineToolCall, policy.ToolCallSeconds, since)
	result.ToolCallSince = since
	if since != nil {
		deadline := since.UTC().Add(time.Duration(policy.ToolCallSeconds) * time.Second)
		result.ToolCallDeadline = &deadline
	}
	return result, true
}

// workingResult derives the precise protocol state for a lane that has cleared
// every stall rung (it is not stalled, dead, or awaiting attention). Instead of
// collapsing every such lane to a generic "live", it reports which kind of
// progress signal is currently fresh so supervise status / dashboard is honest
// (RFC 0101 Phase 1, Layer 1, G2):
//
//   - working_tool   — the lane is inside an MCP/tool call (a tool-call start
//     recorded with no matching finish after it). Exposes a visible since
//     (when the call started) and a deadline (#83) so an operator can see a lane
//     that is blocked inside a hidden call rather than a contentless "live".
//   - working_protocol — fresh MCP/protocol activity within ProtocolFreshSeconds.
//   - working_local  — protocol quiet, but the child PTY produced output within
//     PTYFreshSeconds (the #80 honest-local-work signal).
//   - quiet          — none of the above is fresh, but the lane has not yet
//     missed any deadline (the pre-deadline window). It is not stalled; it is
//     simply between signals.
//
// This never relaxes a stall edge: workingResult is only reached after every
// missed-deadline check has already returned, so a genuinely stalled or dead
// lane never reaches here. Lease state is reported unchanged.
func workingResult(activity Activity, policy Policy, now time.Time) Result {
	now = now.UTC()
	lease := leaseState(activity, "")

	// working_tool: a tool-call start that has no finish recorded after it means
	// the lane is currently inside that call. Surface a visible since + deadline.
	if inTool, since := inToolCall(activity); inTool {
		result := Result{
			Protocol:      ProtocolWorkingTool,
			Lease:         lease,
			DeadlineName:  DeadlineToolCall,
			ToolCallSince: since,
		}
		if policy.ToolCallSeconds > 0 {
			result.DeadlineSeconds = policy.ToolCallSeconds
			if since != nil {
				deadline := since.UTC().Add(time.Duration(policy.ToolCallSeconds) * time.Second)
				result.ToolCallDeadline = &deadline
			}
		}
		return result
	}

	protocolFresh := protocolActivityFresh(activity, policy, now)
	if protocolFresh {
		return Result{Protocol: ProtocolWorkingProtocol, Lease: lease}
	}

	if ptyActive(activity) && !missed(activity.LastPTYActivityAt, policy.PTYFreshSeconds, now) {
		return Result{Protocol: ProtocolWorkingLocal, Lease: lease}
	}

	// No fresh protocol or PTY signal, but no deadline missed either: the lane is
	// quiet, not stalled. Reporting "quiet" (rather than "live") tells the
	// operator there is currently no positive progress signal, while making clear
	// it is still inside its grace window.
	return Result{Protocol: ProtocolQuiet, Lease: lease}
}

// inToolCall reports whether the lane is currently inside an MCP/tool call: a
// tool-call start has been recorded and no tool-call finish has been recorded at
// or after it. The returned timestamp is when the in-flight call started (#83).
func inToolCall(activity Activity) (bool, *time.Time) {
	if activity.LastToolCallStartedAt == nil {
		return false, nil
	}
	if after(activity.LastToolCallFinishedAt, activity.LastToolCallStartedAt) {
		return false, nil
	}
	// A finish recorded at exactly the same instant as the start counts as
	// completed (the call returned within the timestamp resolution).
	if activity.LastToolCallFinishedAt != nil &&
		activity.LastToolCallFinishedAt.UTC().Equal(activity.LastToolCallStartedAt.UTC()) {
		return false, nil
	}
	return true, activity.LastToolCallStartedAt
}

// ptyActive reports whether the lane has ever produced PTY output the helper
// recorded (last_pty_activity_at is set). Used both to keep an honestly-working
// local lane out of the dead classification (#117) and to drive working_local.
func ptyActive(activity Activity) bool {
	return activity.LastPTYActivityAt != nil
}

// protocolActivityFresh reports whether any protocol/MCP signal is fresh within
// ProtocolFreshSeconds. It mirrors the discovered() signal set so a lane that
// just made a protocol call reads working_protocol.
func protocolActivityFresh(activity Activity, policy Policy, now time.Time) bool {
	if policy.ProtocolFreshSeconds <= 0 {
		return false
	}
	latest := latestTime(
		activity.LastMCPRequestAt,
		activity.LastToolsListAt,
		activity.LastAwaitPacketAt,
		activity.LastPacketDeliveredAt,
		activity.LastAckAt,
		activity.LastWorkBlockAt,
		activity.LastWorkReleaseAt,
		activity.LastWorkCompleteAt,
		activity.LastWorkHeartbeatAt,
		activity.LastSessionReadyAt,
		activity.LastSessionHeartbeatAt,
		activity.LastSessionQuestionAt,
		activity.LastSessionEscalateAt,
		activity.ActiveLeaseHeartbeatAt,
	)
	return latest != nil && !missed(latest, policy.ProtocolFreshSeconds, now)
}

func RemoveProjectionSourceFields(row map[string]any) {
	for _, key := range []string{
		"registered_at",
		LastMCPRequestAt,
		LastToolsListAt,
		LastAwaitPacketAt,
		LastPacketDeliveredAt,
		LastAckAt,
		LastWorkBlockAt,
		LastWorkReleaseAt,
		LastWorkCompleteAt,
		LastWorkHeartbeatAt,
		LastSessionReadyAt,
		LastSessionHeartbeatAt,
		LastSessionQuestionAt,
		LastSessionEscalateAt,
		LastPTYActivityAt,
		LastToolCallStartedAt,
		LastToolCallFinishedAt,
		LivenessStallClass,
		LivenessStallSince,
		ActiveLeaseID,
		ActiveLeaseAcquiredAt,
		ActiveLeaseExpiresAt,
		ActiveLeaseHeartbeatAt,
	} {
		delete(row, key)
	}
}

func stallResult(activity Activity, class string, deadline string, seconds int, base *time.Time) Result {
	var since *time.Time
	if activity.PersistedStallClass == class {
		since = activity.PersistedStallSince
	}
	if since == nil && base != nil {
		value := base.UTC().Add(time.Duration(seconds) * time.Second)
		since = &value
	}
	return Result{
		Protocol:        protocolState(class),
		Lease:           leaseState(activity, class),
		StallClass:      class,
		StallSince:      since,
		DeadlineName:    deadline,
		DeadlineSeconds: seconds,
	}
}

func protocolState(class string) string {
	switch class {
	case StallQuestionPending, StallEscalationPending:
		return "attention"
	case "":
		return "live"
	default:
		return "stalled"
	}
}

func leaseState(activity Activity, class string) string {
	if activity.ActiveLeaseID == "" {
		return "no_lease"
	}
	if class == StallLeaseHeartbeat {
		return "stalled"
	}
	return "live"
}

func missed(base *time.Time, seconds int, now time.Time) bool {
	return base != nil && seconds >= 0 && !base.UTC().Add(time.Duration(seconds)*time.Second).After(now.UTC())
}

func attentionPending(activity Activity) (string, *time.Time) {
	kind := ""
	var at *time.Time
	if activity.LastSessionQuestionAt != nil {
		kind = StallQuestionPending
		at = activity.LastSessionQuestionAt
	}
	if after(activity.LastSessionEscalateAt, at) {
		kind = StallEscalationPending
		at = activity.LastSessionEscalateAt
	}
	if kind == "" || progressAfter(activity, at) {
		return "", nil
	}
	return kind, at
}

// discovered reports whether the session has demonstrably discovered MCP. The
// discovery deadline exists to catch a lane that never reached the daemon over
// MCP at all. A tools/list call is the canonical discovery signal, but any
// other recorded MCP protocol activity (await_packet, ack, work block/release/
// complete, heartbeat, session report, packet delivery) is conclusive proof the
// lane discovered MCP and bound its session — even if the initial tools/list
// was issued before the session_id was bound and therefore never recorded
// against the session. Gating discovery solely on last_tools_list_at would
// otherwise demote actively-working supervised agent-loop lanes to the
// agent_mcp_discovery_stall class, which in turn demotes their attested byline
// (RFC 0026 / D149). last_mcp_request_at is stamped on every recorded mutation,
// so it captures the general case; the explicit columns guard against future
// callers that bypass that default.
func discovered(activity Activity) bool {
	return activity.LastToolsListAt != nil ||
		activity.LastMCPRequestAt != nil ||
		activity.LastAwaitPacketAt != nil ||
		activity.LastPacketDeliveredAt != nil ||
		activity.LastAckAt != nil ||
		activity.LastWorkBlockAt != nil ||
		activity.LastWorkReleaseAt != nil ||
		activity.LastWorkCompleteAt != nil ||
		activity.LastWorkHeartbeatAt != nil ||
		activity.LastSessionReadyAt != nil ||
		activity.LastSessionHeartbeatAt != nil ||
		activity.LastSessionQuestionAt != nil ||
		activity.LastSessionEscalateAt != nil
}

func progressAfter(activity Activity, at *time.Time) bool {
	return anyAfter(at,
		activity.LastToolsListAt,
		activity.LastAwaitPacketAt,
		activity.LastPacketDeliveredAt,
		activity.LastAckAt,
		activity.LastWorkBlockAt,
		activity.LastWorkReleaseAt,
		activity.LastWorkCompleteAt,
		activity.LastWorkHeartbeatAt,
		activity.LastSessionReadyAt,
		activity.LastSessionHeartbeatAt,
	)
}

func hasAckEquivalentAfter(activity Activity, deliveredAt *time.Time) bool {
	return anyAfter(deliveredAt,
		activity.LastAckAt,
		activity.LastWorkBlockAt,
		activity.LastWorkReleaseAt,
		activity.LastWorkCompleteAt,
		activity.LastSessionQuestionAt,
		activity.LastSessionEscalateAt,
	)
}

func anyAfter(base *time.Time, candidates ...*time.Time) bool {
	for _, candidate := range candidates {
		if after(candidate, base) {
			return true
		}
	}
	return false
}

func after(candidate *time.Time, base *time.Time) bool {
	if candidate == nil {
		return false
	}
	if base == nil {
		return true
	}
	return candidate.UTC().After(base.UTC())
}

func latestTime(values ...*time.Time) *time.Time {
	var latest *time.Time
	for _, value := range values {
		if after(value, latest) {
			latest = value
		}
	}
	return latest
}

func lastSessionReportKind(activity Activity) string {
	kind := ""
	var latest *time.Time
	reports := []struct {
		kind string
		at   *time.Time
	}{
		{"ready", activity.LastSessionReadyAt},
		{"heartbeat", activity.LastSessionHeartbeatAt},
		{"question", activity.LastSessionQuestionAt},
		{"escalate", activity.LastSessionEscalateAt},
	}
	for _, report := range reports {
		if after(report.at, latest) {
			kind = report.kind
			latest = report.at
		}
	}
	return kind
}

func timeFromAny(value any) *time.Time {
	switch typed := value.(type) {
	case time.Time:
		v := typed.UTC()
		return &v
	case *time.Time:
		if typed == nil {
			return nil
		}
		v := typed.UTC()
		return &v
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		if parsed, err := time.Parse(time.RFC3339Nano, typed); err == nil {
			v := parsed.UTC()
			return &v
		}
	case []byte:
		return timeFromAny(string(typed))
	}
	return nil
}

func timeValue(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
