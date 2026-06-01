package sessionliveness

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestClassifyDiscoveryAwaitAckLeaseAndAttention(t *testing.T) {
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	tests := []struct {
		name string
		in   Activity
		want string
	}{
		{
			name: "discovery",
			in:   Activity{SessionState: "active", RegisteredAt: at(now.Add(-61 * time.Second))},
			want: StallDiscovery,
		},
		{
			name: "await packet",
			in: Activity{
				SessionState:    "active",
				RegisteredAt:    at(now.Add(-3 * time.Minute)),
				LastToolsListAt: at(now.Add(-91 * time.Second)),
			},
			want: StallAwaitPacket,
		},
		{
			name: "ack",
			in: Activity{
				SessionState:          "active",
				RegisteredAt:          at(now.Add(-3 * time.Minute)),
				LastToolsListAt:       at(now.Add(-2 * time.Minute)),
				LastAwaitPacketAt:     at(now.Add(-119 * time.Second)),
				LastPacketDeliveredAt: at(now.Add(-61 * time.Second)),
			},
			want: StallAck,
		},
		{
			name: "lease heartbeat",
			in: Activity{
				SessionState:           "active",
				RegisteredAt:           at(now.Add(-10 * time.Minute)),
				LastToolsListAt:        at(now.Add(-9 * time.Minute)),
				LastAwaitPacketAt:      at(now.Add(-8 * time.Minute)),
				LastPacketDeliveredAt:  at(now.Add(-7 * time.Minute)),
				LastAckAt:              at(now.Add(-6 * time.Minute)),
				ActiveLeaseID:          "lease_1",
				ActiveLeaseHeartbeatAt: at(now.Add(-331 * time.Second)),
			},
			want: StallLeaseHeartbeat,
		},
		{
			name: "question pending",
			in: Activity{
				SessionState:          "active",
				RegisteredAt:          at(now.Add(-10 * time.Minute)),
				LastToolsListAt:       at(now.Add(-9 * time.Minute)),
				LastAwaitPacketAt:     at(now.Add(-8 * time.Minute)),
				LastSessionQuestionAt: at(now.Add(-30 * time.Second)),
			},
			want: StallQuestionPending,
		},
		{
			name: "escalation pending",
			in: Activity{
				SessionState:           "active",
				RegisteredAt:           at(now.Add(-10 * time.Minute)),
				LastToolsListAt:        at(now.Add(-9 * time.Minute)),
				LastAwaitPacketAt:      at(now.Add(-8 * time.Minute)),
				LastSessionQuestionAt:  at(now.Add(-40 * time.Second)),
				LastSessionEscalateAt:  at(now.Add(-30 * time.Second)),
				LastSessionHeartbeatAt: at(now.Add(-50 * time.Second)),
			},
			want: StallEscalationPending,
		},
		{
			name: "protocol idle",
			in: Activity{
				SessionState:          "active",
				RegisteredAt:          at(now.Add(-20 * time.Minute)),
				LastToolsListAt:       at(now.Add(-19 * time.Minute)),
				LastAwaitPacketAt:     at(now.Add(-18 * time.Minute)),
				LastPacketDeliveredAt: at(now.Add(-17 * time.Minute)),
				LastAckAt:             at(now.Add(-16 * time.Minute)),
				LastMCPRequestAt:      at(now.Add(-301 * time.Second)),
			},
			want: StallProtocolIdle,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.in, policy, now)
			if got.StallClass != tc.want {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.want, got)
			}
		})
	}
}

// TestClassifyDiscoverySatisfiedByOtherMCPActivity guards #63 F4: a supervised
// agent-loop lane that issued its initial tools/list before binding session_id
// (so last_tools_list_at stays null) but is actively driving the protocol over
// MCP must NOT be classified agent_mcp_discovery_stall once it is past the
// discovery deadline. The discovery deadline only protects against lanes that
// never reached the daemon at all; any recorded MCP activity disproves that.
func TestClassifyDiscoverySatisfiedByOtherMCPActivity(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	registered := at(now.Add(-2 * time.Minute)) // well past DiscoverySeconds (60s)

	tests := []struct {
		name      string
		in        Activity
		wantStall string
	}{
		{
			// Baseline preserved: no MCP activity at all => discovery stall.
			name:      "no activity still stalls discovery",
			in:        Activity{SessionState: "active", RegisteredAt: registered},
			wantStall: StallDiscovery,
		},
		{
			// await_packet recorded last_mcp_request_at + last_await_packet_at,
			// proving discovery despite a null last_tools_list_at.
			name: "await packet without tools list is not discovery stall",
			in: Activity{
				SessionState:      "active",
				RegisteredAt:      registered,
				LastMCPRequestAt:  at(now.Add(-5 * time.Second)),
				LastAwaitPacketAt: at(now.Add(-5 * time.Second)),
			},
			wantStall: "",
		},
		{
			// A bare last_mcp_request_at (any recorded mutation) clears discovery.
			name: "mcp request without tools list is not discovery stall",
			in: Activity{
				SessionState:     "active",
				RegisteredAt:     registered,
				LastMCPRequestAt: at(now.Add(-5 * time.Second)),
			},
			wantStall: "",
		},
		{
			// #63 F4 zombie guard: a lane that pinged MCP once then went silent
			// (stale last_mcp_request_at, null tools_list/await_packet) must fall
			// through to the protocol-idle catch-all, not short-circuit to "live"
			// via the await-packet branch. Before the fix this read as live forever.
			name: "stale lone mcp request trips protocol idle",
			in: Activity{
				SessionState:     "active",
				RegisteredAt:     at(now.Add(-10 * time.Minute)),
				LastMCPRequestAt: at(now.Add(-10 * time.Minute)),
			},
			wantStall: StallProtocolIdle,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.in, policy, now)
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
		})
	}
}

// TestClassifyActiveLeaseGovernsWorkingLane guards #63 F8: a lane holding an
// active lease and still heartbeating it is actively working and must NOT be
// flagged StallProtocolIdle merely because it issued no other MCP call within
// the protocol-idle window (a long mid-generation gap). The lease-heartbeat
// rung is the terminal classification for lease holders: a genuinely dead lease
// holder stops heartbeating and STILL trips StallLeaseHeartbeat, so dead-lane
// detection is preserved. Lanes with no active lease keep the protocol-idle
// catch-all (no new escape hatch).
func TestClassifyActiveLeaseGovernsWorkingLane(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	// Past discovery/await/ack: the lane already drove the protocol and now
	// holds a lease while generating.
	settled := func() Activity {
		return Activity{
			SessionState:          "active",
			RegisteredAt:          at(now.Add(-30 * time.Minute)),
			LastToolsListAt:       at(now.Add(-29 * time.Minute)),
			LastAwaitPacketAt:     at(now.Add(-28 * time.Minute)),
			LastPacketDeliveredAt: at(now.Add(-27 * time.Minute)),
			LastAckAt:             at(now.Add(-26 * time.Minute)),
		}
	}

	tests := []struct {
		name      string
		mutate    func(Activity) Activity
		wantStall string
		wantLease string
	}{
		{
			// False-positive gone: lease heartbeat is fresh (lane is working)
			// but last MCP request is far past the 300s protocol-idle window.
			name: "working lane with fresh lease heartbeat is not protocol idle",
			mutate: func(a Activity) Activity {
				a.LastMCPRequestAt = at(now.Add(-10 * time.Minute)) // stale MCP
				a.ActiveLeaseID = "lease_1"
				a.ActiveLeaseAcquiredAt = at(now.Add(-20 * time.Minute))
				a.ActiveLeaseHeartbeatAt = at(now.Add(-30 * time.Second)) // fresh
				return a
			},
			wantStall: "",
			wantLease: "live",
		},
		{
			// work.heartbeat stamps LastWorkHeartbeatAt; it must also count as a
			// fresh lease heartbeat.
			name: "working lane with fresh work heartbeat is not protocol idle",
			mutate: func(a Activity) Activity {
				a.LastMCPRequestAt = at(now.Add(-10 * time.Minute))
				a.ActiveLeaseID = "lease_1"
				a.ActiveLeaseAcquiredAt = at(now.Add(-20 * time.Minute))
				a.LastWorkHeartbeatAt = at(now.Add(-20 * time.Second)) // fresh
				return a
			},
			wantStall: "",
			wantLease: "live",
		},
		{
			// Dead-lane detection preserved: lease holder stopped heartbeating
			// past LeaseHeartbeatSeconds + slack (330s) => StallLeaseHeartbeat.
			name: "dead lease holder still trips lease heartbeat stall",
			mutate: func(a Activity) Activity {
				a.LastMCPRequestAt = at(now.Add(-10 * time.Minute))
				a.ActiveLeaseID = "lease_1"
				a.ActiveLeaseAcquiredAt = at(now.Add(-20 * time.Minute))
				a.ActiveLeaseHeartbeatAt = at(now.Add(-331 * time.Second)) // stale
				return a
			},
			wantStall: StallLeaseHeartbeat,
			wantLease: "stalled",
		},
		{
			// Dead lane that acquired a lease and never heartbeat: base is
			// acquired_at, so it trips lease-heartbeat once past the threshold.
			name: "lease holder that never heartbeat still stalls",
			mutate: func(a Activity) Activity {
				a.LastMCPRequestAt = at(now.Add(-10 * time.Minute))
				a.ActiveLeaseID = "lease_1"
				a.ActiveLeaseAcquiredAt = at(now.Add(-400 * time.Second)) // > 330s
				return a
			},
			wantStall: StallLeaseHeartbeat,
			wantLease: "stalled",
		},
		{
			// No escape hatch: a lane WITHOUT a lease and stale MCP still trips
			// protocol idle exactly as before.
			name: "no lease with stale mcp still trips protocol idle",
			mutate: func(a Activity) Activity {
				a.LastMCPRequestAt = at(now.Add(-301 * time.Second))
				return a
			},
			wantStall: StallProtocolIdle,
			wantLease: "no_lease",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.mutate(settled()), policy, now)
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
			if got.Lease != tc.wantLease {
				t.Fatalf("lease = %q, want %q; result = %#v", got.Lease, tc.wantLease, got)
			}
		})
	}
}

// TestClassifyPreciseWorkingStates guards RFC 0101 Phase 1 (Layer 1, G2): a
// lane that has cleared every stall rung is reported with a PRECISE protocol
// state — working_protocol / working_local / working_tool / quiet — instead of
// being collapsed to a generic "live", so supervise status is honest about what
// kind of progress signal (if any) is currently fresh.
func TestClassifyPreciseWorkingStates(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	// A lane that has discovered MCP, driven the protocol, and acked its packet,
	// so it is past every stall rung and reaches workingResult.
	settled := func() Activity {
		return Activity{
			SessionState:          "active",
			RegisteredAt:          at(now.Add(-30 * time.Minute)),
			LastToolsListAt:       at(now.Add(-29 * time.Minute)),
			LastAwaitPacketAt:     at(now.Add(-28 * time.Minute)),
			LastPacketDeliveredAt: at(now.Add(-27 * time.Minute)),
			LastAckAt:             at(now.Add(-26 * time.Minute)),
		}
	}

	tests := []struct {
		name             string
		mutate           func(Activity) Activity
		wantProtocol     string
		wantToolSince    bool
		wantToolDeadline bool
	}{
		{
			name: "working_protocol on fresh protocol activity",
			mutate: func(a Activity) Activity {
				a.LastWorkHeartbeatAt = at(now.Add(-10 * time.Second)) // within ProtocolFreshSeconds
				return a
			},
			wantProtocol: ProtocolWorkingProtocol,
		},
		{
			name: "working_local when protocol quiet but PTY fresh (#80)",
			mutate: func(a Activity) Activity {
				// Protocol last touched 5m ago (stale vs ProtocolFreshSeconds 60s)
				// but the child PTY produced output 10s ago.
				a.LastMCPRequestAt = at(now.Add(-5 * time.Minute))
				a.LastPTYActivityAt = at(now.Add(-10 * time.Second))
				return a
			},
			wantProtocol: ProtocolWorkingLocal,
		},
		{
			name: "working_tool while inside an MCP/tool call (#83)",
			mutate: func(a Activity) Activity {
				// A tool call started 30s ago with no finish recorded after it.
				a.LastMCPRequestAt = at(now.Add(-30 * time.Second))
				a.LastToolCallStartedAt = at(now.Add(-30 * time.Second))
				return a
			},
			wantProtocol:     ProtocolWorkingTool,
			wantToolSince:    true,
			wantToolDeadline: true,
		},
		{
			name: "working_tool takes precedence over fresh protocol activity",
			mutate: func(a Activity) Activity {
				a.LastWorkHeartbeatAt = at(now.Add(-5 * time.Second)) // would be working_protocol
				a.LastToolCallStartedAt = at(now.Add(-5 * time.Second))
				return a
			},
			wantProtocol:     ProtocolWorkingTool,
			wantToolSince:    true,
			wantToolDeadline: true,
		},
		{
			name: "tool call finished is not working_tool",
			mutate: func(a Activity) Activity {
				a.LastToolCallStartedAt = at(now.Add(-40 * time.Second))
				a.LastToolCallFinishedAt = at(now.Add(-39 * time.Second)) // finished after start
				a.LastWorkHeartbeatAt = at(now.Add(-39 * time.Second))    // fresh protocol
				return a
			},
			wantProtocol: ProtocolWorkingProtocol,
		},
		{
			name: "quiet when no signal is fresh but no deadline missed",
			mutate: func(a Activity) Activity {
				// Last protocol touch 90s ago (stale vs 60s ProtocolFreshSeconds) but
				// still inside the 300s protocol-idle deadline; no PTY, no tool call.
				a.LastMCPRequestAt = at(now.Add(-90 * time.Second))
				return a
			},
			wantProtocol: ProtocolQuiet,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.mutate(settled()), policy, now)
			if got.Protocol != tc.wantProtocol {
				t.Fatalf("protocol = %q, want %q; result = %#v", got.Protocol, tc.wantProtocol, got)
			}
			if got.StallClass != "" {
				t.Fatalf("working state must not carry a stall class; got %q", got.StallClass)
			}
			if tc.wantToolSince && got.ToolCallSince == nil {
				t.Fatalf("working_tool must expose a since; result = %#v", got)
			}
			if !tc.wantToolSince && got.ToolCallSince != nil {
				t.Fatalf("non-tool state must not expose a since; result = %#v", got)
			}
			if tc.wantToolDeadline {
				if got.ToolCallDeadline == nil {
					t.Fatalf("working_tool must expose a deadline; result = %#v", got)
				}
				if got.DeadlineName != DeadlineToolCall {
					t.Fatalf("working_tool deadline name = %q, want %q", got.DeadlineName, DeadlineToolCall)
				}
				// Deadline must be since + ToolCallSeconds.
				wantDeadline := got.ToolCallSince.Add(time.Duration(policy.ToolCallSeconds) * time.Second)
				if !got.ToolCallDeadline.Equal(wantDeadline) {
					t.Fatalf("deadline = %v, want %v", got.ToolCallDeadline, wantDeadline)
				}
			}
		})
	}
}

// TestClassifyDeadAtSpawnIsNotDiscoveryStall guards #117: a lane that never
// reached the daemon over MCP AND produced no PTY output past the discovery
// deadline reads as DEAD (operator-visible Protocol), not a misleading
// agent_mcp_discovery_stall. The underlying StallClass is deliberately retained
// (the liveness sweep / recovery library key on it) but the protocol surface is
// honest. A lane producing PTY output past the deadline keeps the plain
// discovery stall (it is alive, just slow to bind MCP).
func TestClassifyDeadAtSpawnIsNotDiscoveryStall(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	registered := at(now.Add(-2 * time.Minute)) // past DiscoverySeconds (60s)

	tests := []struct {
		name          string
		in            Activity
		wantProtocol  string
		wantStall     string
		wantStallKept bool
	}{
		{
			name:         "dead: no mcp, no pty, past discovery deadline",
			in:           Activity{SessionState: "active", RegisteredAt: registered},
			wantProtocol: ProtocolDead,
			wantStall:    StallDiscovery,
		},
		{
			name: "alive-but-slow: pty output present keeps plain discovery stall",
			in: Activity{
				SessionState:      "active",
				RegisteredAt:      registered,
				LastPTYActivityAt: at(now.Add(-5 * time.Second)),
			},
			wantProtocol: ProtocolStalled,
			wantStall:    StallDiscovery,
		},
		{
			name: "before deadline with no signal is quiet, not dead",
			in: Activity{
				SessionState: "active",
				RegisteredAt: at(now.Add(-10 * time.Second)), // inside DiscoverySeconds
			},
			wantProtocol: ProtocolQuiet,
			wantStall:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.in, policy, now)
			if got.Protocol != tc.wantProtocol {
				t.Fatalf("protocol = %q, want %q; result = %#v", got.Protocol, tc.wantProtocol, got)
			}
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
		})
	}
}

// TestProjectionExposesNewLivenessColumns asserts the read-layer projection
// surfaces the new PTY/tool-call timestamps and, for an in-tool lane, the
// visible tool_call_since / tool_call_deadline (#83).
func TestProjectionExposesNewLivenessColumns(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	started := now.Add(-30 * time.Second)
	row := map[string]any{
		"state":               "active",
		"registered_at":       now.Add(-30 * time.Minute),
		LastToolsListAt:       now.Add(-29 * time.Minute),
		LastAwaitPacketAt:     now.Add(-28 * time.Minute),
		LastPacketDeliveredAt: now.Add(-27 * time.Minute),
		LastAckAt:             now.Add(-26 * time.Minute),
		LastMCPRequestAt:      started,
		LastPTYActivityAt:     now.Add(-15 * time.Second),
		LastToolCallStartedAt: started,
	}
	projection := ProjectionFromRow(row, now)
	if projection["protocol"] != ProtocolWorkingTool {
		t.Fatalf("protocol = %v, want working_tool", projection["protocol"])
	}
	for _, key := range []string{"last_pty_activity_at", "last_tool_call_started_at", "tool_call_since", "tool_call_deadline"} {
		if projection[key] == nil {
			t.Fatalf("projection[%q] is nil; want a timestamp: %#v", key, projection)
		}
	}
	if projection["last_tool_call_finished_at"] != nil {
		t.Fatalf("last_tool_call_finished_at should be nil when never finished: %#v", projection["last_tool_call_finished_at"])
	}
}

func TestRecordAcceptsToolCallAndPTYColumns(t *testing.T) {
	for _, column := range []string{LastPTYActivityAt, LastToolCallStartedAt, LastToolCallFinishedAt} {
		runner := &recordFakeRunner{}
		if err := Record(context.Background(), runner, "repo_1", "sess_1", column); err != nil {
			t.Fatalf("Record(%s): %v", column, err)
		}
		if !strings.Contains(runner.sql, column+" = $1") {
			t.Fatalf("sql for %s = %s", column, runner.sql)
		}
	}
}

func TestRecordUpdatesMCPRequestAndRequestedColumns(t *testing.T) {
	runner := &recordFakeRunner{}
	err := Record(context.Background(), runner, "repo_1", "sess_1", LastToolsListAt)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if !strings.Contains(runner.sql, "last_mcp_request_at = $1") || !strings.Contains(runner.sql, "last_tools_list_at = $1") {
		t.Fatalf("sql = %s", runner.sql)
	}
	if runner.args[1] != "repo_1" || runner.args[2] != "sess_1" {
		t.Fatalf("args = %#v", runner.args)
	}
}

func TestRecordRejectsUnknownColumns(t *testing.T) {
	err := Record(context.Background(), &recordFakeRunner{}, "repo_1", "sess_1", "last_bad_at")
	if err == nil {
		t.Fatal("expected error")
	}
}

type recordFakeRunner struct {
	sql  string
	args []any
}

func (r *recordFakeRunner) Exec(_ context.Context, sql string, args ...any) error {
	r.sql = sql
	r.args = args
	return nil
}

func at(value time.Time) *time.Time {
	v := value.UTC()
	return &v
}
