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
	DiscoverySeconds      int
	AwaitPacketSeconds    int
	AckSeconds            int
	LeaseHeartbeatSeconds int
	LeaseHeartbeatSlack   int
	ProtocolIdleSeconds   int
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
}

func DefaultPolicy() Policy {
	return Policy{
		DiscoverySeconds:      60,
		AwaitPacketSeconds:    90,
		AckSeconds:            60,
		LeaseHeartbeatSeconds: 300,
		LeaseHeartbeatSlack:   30,
		ProtocolIdleSeconds:   300,
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
	if !discovered(activity) {
		if missed(activity.RegisteredAt, policy.DiscoverySeconds, now) {
			return stallResult(activity, StallDiscovery, DeadlineDiscovery, policy.DiscoverySeconds, activity.RegisteredAt)
		}
		return Result{Protocol: "live", Lease: leaseState(activity, "")}
	}
	if !after(activity.LastAwaitPacketAt, activity.LastToolsListAt) {
		if missed(activity.LastToolsListAt, policy.AwaitPacketSeconds, now) {
			return stallResult(activity, StallAwaitPacket, DeadlineAwaitPacket, policy.AwaitPacketSeconds, activity.LastToolsListAt)
		}
		return Result{Protocol: "live", Lease: leaseState(activity, "")}
	}
	if activity.LastPacketDeliveredAt != nil && !hasAckEquivalentAfter(activity, activity.LastPacketDeliveredAt) {
		if missed(activity.LastPacketDeliveredAt, policy.AckSeconds, now) {
			return stallResult(activity, StallAck, DeadlineAck, policy.AckSeconds, activity.LastPacketDeliveredAt)
		}
		return Result{Protocol: "live", Lease: leaseState(activity, "")}
	}
	if activity.ActiveLeaseID != "" {
		base := latestTime(activity.LastWorkHeartbeatAt, activity.ActiveLeaseHeartbeatAt, activity.ActiveLeaseAcquiredAt)
		threshold := policy.LeaseHeartbeatSeconds + policy.LeaseHeartbeatSlack
		if missed(base, threshold, now) {
			return stallResult(activity, StallLeaseHeartbeat, DeadlineLeaseHeartbeat, threshold, base)
		}
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
		activity.RegisteredAt,
	)
	if missed(base, policy.ProtocolIdleSeconds, now) {
		return stallResult(activity, StallProtocolIdle, DeadlineProtocolIdle, policy.ProtocolIdleSeconds, base)
	}
	return Result{Protocol: "live", Lease: leaseState(activity, "")}
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
