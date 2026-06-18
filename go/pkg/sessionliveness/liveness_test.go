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
			// #192: past DiscoverySeconds + BootstrapGraceSeconds (60+120=180s),
			// with no MCP activity, is a genuine discovery stall.
			in:   Activity{SessionState: "active", RegisteredAt: at(now.Add(-181 * time.Second))},
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
	registered := at(now.Add(-4 * time.Minute)) // well past DiscoverySeconds+BootstrapGrace (180s)

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
	registered := at(now.Add(-4 * time.Minute)) // past DiscoverySeconds+BootstrapGrace (180s)

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

// TestClassifyBootstrapGraceSuppressesColdStartDiscoveryStall guards #192: an
// agent CLI's normal cold start (model/session init before its first tools/list)
// routinely takes longer than the bare DiscoverySeconds (60s) — ~56s was measured
// on a HEALTHY claude lane that then proceeded normally, and claude routinely
// exceeds 60s. A lane that has recorded NO MCP activity must therefore NOT be
// flagged agent_mcp_discovery_stall until it misses the bootstrap grace window
// (DiscoverySeconds + BootstrapGraceSeconds = 180s); inside that window it reads
// quiet/working, not stalled. Past the window it still stalls (dead-at-spawn
// detection preserved via TestClassifyDeadAtSpawnIsNotDiscoveryStall).
func TestClassifyBootstrapGraceSuppressesColdStartDiscoveryStall(t *testing.T) {
	now := time.Date(2026, 6, 6, 16, 46, 31, 0, time.UTC)
	policy := DefaultPolicy()
	tests := []struct {
		name       string
		registered *time.Time
		pty        *time.Time
		wantStall  string
	}{
		{
			// The exact #192 timing: ~56s after spawn, zero MCP calls — a healthy
			// claude lane mid cold-start. Before the grace window this read as
			// agent_mcp_discovery_stall and killed naive watchers on first poll.
			name:       "56s cold start with no mcp is not a stall",
			registered: at(now.Add(-56 * time.Second)),
			wantStall:  "",
		},
		{
			// Still inside the grace window at 90s (past the bare 60s edge): a slow
			// but healthy boot must not be flagged.
			name:       "90s cold start still inside bootstrap grace",
			registered: at(now.Add(-90 * time.Second)),
			wantStall:  "",
		},
		{
			// Just past the bare DiscoverySeconds but well inside the grace window.
			name:       "61s cold start no longer trips the bare discovery edge",
			registered: at(now.Add(-61 * time.Second)),
			wantStall:  "",
		},
		{
			// Past the full bootstrap window with no signal at all => genuine stall.
			name:       "181s past bootstrap grace is a genuine discovery stall",
			registered: at(now.Add(-181 * time.Second)),
			wantStall:  StallDiscovery,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Activity{SessionState: "active", RegisteredAt: tc.registered, LastPTYActivityAt: tc.pty}, policy, now)
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
		})
	}
}

// TestClassifyFreshOutputSuppressesLeaseHeartbeatStall guards #145: a lease
// holder whose heartbeat base is past the lease-heartbeat deadline is NOT
// reported stalled when it is demonstrably still producing output — fresh PTY
// frames (working_local) or inside an MCP/tool call (working_tool). A long
// foreground command (a full test suite, a browser-acceptance profile) emits no
// work-heartbeat for minutes while the PTY/tool timeline stays fresh; before the
// fix this tripped StallLeaseHeartbeat and the recovery decision tree transferred
// the actively-working lane mid-work (closing its session and losing the
// artifact). This is the same G2 invariant the adjacent protocol-idle rung
// already honors by folding last_pty_activity_at into its base. A lease holder
// that goes quiet past the PTY window still trips the stall, so dead-lane
// detection is preserved.
func TestClassifyFreshOutputSuppressesLeaseHeartbeatStall(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	// A discovered, acked lease holder whose lease-heartbeat base is stale
	// (>330s) — the rung that would otherwise trip StallLeaseHeartbeat.
	staleLeaseHolder := func() Activity {
		return Activity{
			SessionState:           "active",
			RegisteredAt:           at(now.Add(-30 * time.Minute)),
			LastToolsListAt:        at(now.Add(-29 * time.Minute)),
			LastAwaitPacketAt:      at(now.Add(-28 * time.Minute)),
			LastPacketDeliveredAt:  at(now.Add(-27 * time.Minute)),
			LastAckAt:              at(now.Add(-26 * time.Minute)),
			ActiveLeaseID:          "lease_1",
			ActiveLeaseAcquiredAt:  at(now.Add(-20 * time.Minute)),
			ActiveLeaseHeartbeatAt: at(now.Add(-400 * time.Second)), // stale: > 330s deadline
		}
	}
	tests := []struct {
		name         string
		mutate       func(Activity) Activity
		wantProtocol string
		wantStall    string
	}{
		{
			name: "fresh PTY output suppresses lease-heartbeat stall (working_local)",
			mutate: func(a Activity) Activity {
				a.LastPTYActivityAt = at(now.Add(-10 * time.Second)) // long foreground command still emitting
				return a
			},
			wantProtocol: ProtocolWorkingLocal,
			wantStall:    "",
		},
		{
			name: "inside a fresh tool call suppresses lease-heartbeat stall (working_tool)",
			mutate: func(a Activity) Activity {
				a.LastToolCallStartedAt = at(now.Add(-20 * time.Second)) // in-flight tool call, no finish
				return a
			},
			wantProtocol: ProtocolWorkingTool,
			wantStall:    "",
		},
		{
			name: "no fresh output still trips lease-heartbeat stall (dead-lane detection preserved)",
			mutate: func(a Activity) Activity {
				a.LastPTYActivityAt = at(now.Add(-5 * time.Minute)) // older than PTYFreshSeconds: the lane went quiet
				return a
			},
			wantProtocol: ProtocolStalled,
			wantStall:    StallLeaseHeartbeat,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.mutate(staleLeaseHolder()), policy, now)
			if got.Protocol != tc.wantProtocol {
				t.Fatalf("protocol = %q, want %q; result = %#v", got.Protocol, tc.wantProtocol, got)
			}
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
		})
	}
}

func TestClassifyToolCallCrossesDeadlineToActionableStall(t *testing.T) {
	now := time.Date(2026, 6, 9, 16, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	started := now.Add(-time.Duration(policy.ToolCallSeconds+1) * time.Second)
	got := Classify(Activity{
		SessionState:          "active",
		RegisteredAt:          at(now.Add(-30 * time.Minute)),
		LastToolsListAt:       at(now.Add(-29 * time.Minute)),
		LastAwaitPacketAt:     at(now.Add(-28 * time.Minute)),
		LastPacketDeliveredAt: at(now.Add(-27 * time.Minute)),
		LastAckAt:             at(now.Add(-26 * time.Minute)),
		LastMCPRequestAt:      at(started),
		LastToolCallStartedAt: at(started),
	}, policy, now)

	if got.Protocol != ProtocolStalled {
		t.Fatalf("protocol = %q, want %q; result = %#v", got.Protocol, ProtocolStalled, got)
	}
	if got.StallClass != StallProtocolIdle {
		t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, StallProtocolIdle, got)
	}
	if got.DeadlineName != DeadlineToolCall {
		t.Fatalf("deadline = %q, want %q; result = %#v", got.DeadlineName, DeadlineToolCall, got)
	}
	if got.ToolCallSince == nil || got.ToolCallDeadline == nil {
		t.Fatalf("tool-call stall should keep since/deadline visible: %#v", got)
	}
}

// TestClassifyWedgedNoToolProgress guards #324: a lane that lost its daemon
// endpoint keeps repainting its spinner, so last_pty_activity_at stays fresh and
// the PTY-only working_local rung (through it, the lease-heartbeat rung) reports
// progress forever — even though the lane has made NO tool call for hours. The
// new rung consumes the tool-call timeline (the only rung that reads
// last_tool_call_finished_at) and reclassifies such a lane as
// wedged_no_tool_progress so the recovery decision tree's CASE-2 transfer path
// reclaims the slot. The discriminator is the tool-call history: a genuinely
// working lane (recent tool call), and a long foreground command with no
// tool-call history at all (#145), are BOTH left as working_local.
func TestClassifyWedgedNoToolProgress(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	staleTool := now.Add(-time.Duration(policy.ToolProgressSeconds+60) * time.Second)
	freshTool := now.Add(-30 * time.Second)
	freshSpinner := now.Add(-2 * time.Second) // a spinner frame keeps the PTY fresh

	// A discovered, acked lease holder whose lease heartbeat is stale (the
	// lease-heartbeat rung would otherwise fall to working_local on the fresh PTY).
	// The protocol timestamps are staggered (toolsList < awaitPacket < delivered <
	// ack) so the lane has cleared the await-packet and ack rungs and reaches the
	// active-lease branch — exactly the #324 mid-work wedge.
	base := func() Activity {
		return Activity{
			SessionState:           "active",
			RegisteredAt:           at(now.Add(-4 * time.Hour)),
			LastToolsListAt:        at(now.Add(-239 * time.Minute)),
			LastAwaitPacketAt:      at(now.Add(-238 * time.Minute)),
			LastPacketDeliveredAt:  at(now.Add(-237 * time.Minute)),
			LastAckAt:              at(now.Add(-236 * time.Minute)),
			ActiveLeaseID:          "lease_1",
			ActiveLeaseAcquiredAt:  at(now.Add(-235 * time.Minute)),
			ActiveLeaseHeartbeatAt: at(staleTool), // also stale: past lease-heartbeat deadline
		}
	}

	tests := []struct {
		name         string
		mutate       func(Activity) Activity
		wantProtocol string
		wantStall    string
	}{
		{
			// (a) fresh PTY spinner + stale tool-call timeline => wedged.
			name: "fresh spinner but stale tool calls classifies wedged",
			mutate: func(a Activity) Activity {
				a.LastPTYActivityAt = at(freshSpinner)
				a.LastToolCallStartedAt = at(staleTool)
				a.LastToolCallFinishedAt = at(staleTool)
				return a
			},
			wantProtocol: ProtocolStalled,
			wantStall:    StallToolProgress,
		},
		{
			// (b) recent tool calls => genuinely working, NOT flagged.
			name: "recent tool call is not wedged",
			mutate: func(a Activity) Activity {
				a.LastPTYActivityAt = at(freshSpinner)
				a.LastToolCallStartedAt = at(freshTool)
				a.LastToolCallFinishedAt = at(freshTool)
				return a
			},
			wantProtocol: ProtocolWorkingLocal,
			wantStall:    "",
		},
		{
			// #145 guard: a long foreground command with NO tool-call history at all
			// (fresh PTY only) stays working_local — the wedge rung must not fire
			// without a recorded tool-call timeline to age against.
			name: "no tool-call history stays working_local (long foreground command)",
			mutate: func(a Activity) Activity {
				a.LastPTYActivityAt = at(freshSpinner)
				return a
			},
			wantProtocol: ProtocolWorkingLocal,
			wantStall:    "",
		},
		{
			// A lane currently INSIDE a tool call (start with no finish after it) is
			// making progress by definition — working_tool, never wedged — even if
			// the start is old (handled by the working_tool / tool-call-deadline rung,
			// not this one).
			name: "in-flight tool call is not wedged",
			mutate: func(a Activity) Activity {
				a.LastPTYActivityAt = at(freshSpinner)
				a.LastToolCallStartedAt = at(freshTool)
				// no finish recorded after the start => in-flight
				return a
			},
			wantProtocol: ProtocolWorkingTool,
			wantStall:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.mutate(base()), policy, now)
			if got.Protocol != tc.wantProtocol {
				t.Fatalf("protocol = %q, want %q; result = %#v", got.Protocol, tc.wantProtocol, got)
			}
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
			if tc.wantStall == StallToolProgress && got.DeadlineName != DeadlineToolProgress {
				t.Fatalf("deadline = %q, want %q; result = %#v", got.DeadlineName, DeadlineToolProgress, got)
			}
		})
	}
}

// TestClassifyWedgedNoToolProgressDisabledByZeroPolicy confirms a non-positive
// ToolProgressSeconds disables the rung (the lane falls back to working_local on
// its fresh PTY), so the feature is fully gated by policy.
func TestClassifyWedgedNoToolProgressDisabledByZeroPolicy(t *testing.T) {
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	policy.ToolProgressSeconds = 0
	staleTool := now.Add(-2 * time.Hour)
	got := Classify(Activity{
		SessionState:           "active",
		RegisteredAt:           at(now.Add(-4 * time.Hour)),
		LastToolsListAt:        at(now.Add(-239 * time.Minute)),
		LastAwaitPacketAt:      at(now.Add(-238 * time.Minute)),
		LastPacketDeliveredAt:  at(now.Add(-237 * time.Minute)),
		LastAckAt:              at(now.Add(-236 * time.Minute)),
		ActiveLeaseID:          "lease_1",
		ActiveLeaseAcquiredAt:  at(now.Add(-235 * time.Minute)),
		ActiveLeaseHeartbeatAt: at(staleTool),
		LastPTYActivityAt:      at(now.Add(-2 * time.Second)),
		LastToolCallStartedAt:  at(staleTool),
		LastToolCallFinishedAt: at(staleTool),
	}, policy, now)
	if got.Protocol != ProtocolWorkingLocal || got.StallClass != "" {
		t.Fatalf("zero ToolProgressSeconds should disable the wedge rung; got %#v", got)
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

// TestClassifyProbeBasisDeadlineElapsedOnly (RFC 0131 Layer 1) asserts the pure
// classifier stamps every STALL verdict with ProbeBasisDeadlineElapsedOnly —
// regardless of transport — because Classify() has no confirmed-dead oracle, and
// leaves ProbeBasis empty for a non-stall (working/quiet) verdict. The
// pty_confirmed_dead UPGRADE is the recovery decision tree's job
// (UpgradeProbeBasisConfirmedDead), tested separately.
func TestClassifyProbeBasisDeadlineElapsedOnly(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()

	// A protocol-idle stall on a lane that drove the protocol then went silent.
	stalled := func(transport TransportType) Activity {
		return Activity{
			SessionState:          "active",
			Transport:             transport,
			RegisteredAt:          at(now.Add(-20 * time.Minute)),
			LastToolsListAt:       at(now.Add(-19 * time.Minute)),
			LastAwaitPacketAt:     at(now.Add(-18 * time.Minute)),
			LastPacketDeliveredAt: at(now.Add(-17 * time.Minute)),
			LastAckAt:             at(now.Add(-16 * time.Minute)),
			LastMCPRequestAt:      at(now.Add(-301 * time.Second)),
		}
	}
	// A working lane: fresh MCP activity, every deadline satisfied. tools/list is
	// recent and await_packet landed after it, so the await/ack rungs pass and the
	// lane resolves to working_protocol (fresh last_mcp_request_at).
	working := func(transport TransportType) Activity {
		return Activity{
			SessionState:      "active",
			Transport:         transport,
			RegisteredAt:      at(now.Add(-5 * time.Minute)),
			LastToolsListAt:   at(now.Add(-10 * time.Second)),
			LastAwaitPacketAt: at(now.Add(-6 * time.Second)),
			LastMCPRequestAt:  at(now.Add(-5 * time.Second)),
		}
	}

	tests := []struct {
		name           string
		in             Activity
		wantStall      string
		wantProbeBasis ProbeBasis
	}{
		{
			name:           "pipe silent stall is deadline_elapsed_only",
			in:             stalled(TransportPipe),
			wantStall:      StallProtocolIdle,
			wantProbeBasis: ProbeBasisDeadlineElapsedOnly,
		},
		{
			name:           "pty_helper silent stall is deadline_elapsed_only (no oracle in Classify)",
			in:             stalled(TransportPTYHelper),
			wantStall:      StallProtocolIdle,
			wantProbeBasis: ProbeBasisDeadlineElapsedOnly,
		},
		{
			name:           "unknown transport silent stall is deadline_elapsed_only",
			in:             stalled(TransportUnknown),
			wantStall:      StallProtocolIdle,
			wantProbeBasis: ProbeBasisDeadlineElapsedOnly,
		},
		{
			name:           "pipe working lane has no probe basis",
			in:             working(TransportPipe),
			wantStall:      "",
			wantProbeBasis: ProbeBasisNone,
		},
		{
			name:           "pty_helper working lane has no probe basis",
			in:             working(TransportPTYHelper),
			wantStall:      "",
			wantProbeBasis: ProbeBasisNone,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.in, policy, now)
			if got.StallClass != tc.wantStall {
				t.Fatalf("stall class = %q, want %q; result = %#v", got.StallClass, tc.wantStall, got)
			}
			if got.ProbeBasis != tc.wantProbeBasis {
				t.Fatalf("probe basis = %q, want %q; result = %#v", got.ProbeBasis, tc.wantProbeBasis, got)
			}
		})
	}
}

// TestClassifyPipeTransportRPCRung (RFC 0131 Layer 2 / 131-B) asserts the
// pipe-transport liveness rung: a pipe lane that is mid-RPC (fresh
// last_mcp_request_at) reads working_local rather than stalling on a stale
// await_packet / ack deadline, because a pipe lane has no PTY oracle and its only
// honest progress signal is the daemon-side RPC touch. It must NEVER weaken
// dead-lane detection (a genuinely-silent pipe lane still stalls) and must NOT
// change pty_helper classification (the rung is pipe-scoped).
func TestClassifyPipeTransportRPCRung(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()

	// A lane that drove tools/list long ago (its await_packet deadline elapsed) but
	// just made an MCP request (mid-RPC). For a PTY lane this would read
	// agent_await_packet_stall; for a pipe lane the RPC rung should suppress that.
	midRPC := func(transport TransportType) Activity {
		return Activity{
			SessionState:     "active",
			Transport:        transport,
			RegisteredAt:     at(now.Add(-30 * time.Minute)),
			LastToolsListAt:  at(now.Add(-10 * time.Minute)), // past AwaitPacketSeconds (90s)
			LastMCPRequestAt: at(now.Add(-5 * time.Second)),  // fresh: mid-RPC
		}
	}
	// A genuinely-silent pipe lane: even its last_mcp_request_at is stale. The RPC
	// rung must NOT fire — dead-lane detection is preserved.
	silentPipe := Activity{
		SessionState:     "active",
		Transport:        TransportPipe,
		RegisteredAt:     at(now.Add(-30 * time.Minute)),
		LastToolsListAt:  at(now.Add(-10 * time.Minute)),
		LastMCPRequestAt: at(now.Add(-10 * time.Minute)),
	}

	tests := []struct {
		name         string
		in           Activity
		wantProtocol string
		wantStall    string
		wantNotStall string
	}{
		{
			name:         "pipe lane mid-RPC reads working_local, not await_packet stall",
			in:           midRPC(TransportPipe),
			wantProtocol: ProtocolWorkingLocal,
			wantStall:    "",
			wantNotStall: StallAwaitPacket,
		},
		{
			name:         "pty_helper lane mid-RPC keeps its prior classification (rung is pipe-scoped)",
			in:           midRPC(TransportPTYHelper),
			wantProtocol: ProtocolStalled,
			wantStall:    StallAwaitPacket,
		},
		{
			name:         "genuinely-silent pipe lane still stalls (dead-lane detection preserved)",
			in:           silentPipe,
			wantProtocol: ProtocolStalled,
			wantStall:    StallAwaitPacket,
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
			if tc.wantNotStall != "" && got.StallClass == tc.wantNotStall {
				t.Fatalf("stall class must NOT be %q; result = %#v", tc.wantNotStall, got)
			}
		})
	}
}

// TestClassifyPipeReadLivenessRung (RFC 0131 #350) asserts the synthetic
// pipe-read liveness signal: a working pipe lane mid long LOCAL generation with no
// MCP call but a fresh last_pipe_read_at reads working_local (kept out of the
// protocol-idle stall the confidence gate would otherwise have to debounce), AND
// the signal is forgery-resistant w.r.t. the Layer-4 escape-valve cap — a pipe
// lane whose pipe-read stays fresh (chatter) but whose tool-call timeline has gone
// stale is reclassified wedged_no_tool_progress (stalled), exactly like the #324
// PTY-spinner case, so a synthetic pipe read can never defer escalation past the cap.
func TestClassifyPipeReadLivenessRung(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	staleProtocol := now.Add(-time.Duration(policy.ProtocolIdleSeconds+120) * time.Second)
	staleTool := now.Add(-time.Duration(policy.ToolProgressSeconds+60) * time.Second)
	freshRead := now.Add(-2 * time.Second)

	tests := []struct {
		name         string
		in           Activity
		wantProtocol string
		wantStall    string
	}{
		{
			// A working pipe lane: discovered (one old MCP request), no fresh protocol,
			// no active lease, but a fresh pipe-read keeps it working_local rather than
			// tripping agent_protocol_idle_stall.
			name: "fresh pipe-read keeps a working pipe lane out of protocol-idle",
			in: Activity{
				SessionState:     "active",
				Transport:        TransportPipe,
				RegisteredAt:     at(now.Add(-30 * time.Minute)),
				LastMCPRequestAt: at(staleProtocol),
				LastPipeReadAt:   at(freshRead),
			},
			wantProtocol: ProtocolWorkingLocal,
			wantStall:    "",
		},
		{
			// FORGERY RESISTANCE: a pipe lane whose pipe-read stays fresh (chatter) but
			// which has a recorded tool-call history that has gone stale is wedged — the
			// #324 guard fires regardless of which local-output signal kept it fresh. So
			// a chattering-but-hung pipe lane still reaches the gate and escalates.
			name: "fresh pipe-read but stale tool calls classifies wedged (forgery-resistant)",
			in: Activity{
				SessionState:           "active",
				Transport:              TransportPipe,
				RegisteredAt:           at(now.Add(-4 * time.Hour)),
				LastToolsListAt:        at(now.Add(-239 * time.Minute)),
				LastAwaitPacketAt:      at(now.Add(-238 * time.Minute)),
				LastPacketDeliveredAt:  at(now.Add(-237 * time.Minute)),
				LastAckAt:              at(now.Add(-236 * time.Minute)),
				ActiveLeaseID:          "lease_1",
				ActiveLeaseAcquiredAt:  at(now.Add(-235 * time.Minute)),
				ActiveLeaseHeartbeatAt: at(staleTool),
				LastPipeReadAt:         at(freshRead),
				LastToolCallStartedAt:  at(staleTool),
				LastToolCallFinishedAt: at(staleTool),
			},
			wantProtocol: ProtocolStalled,
			wantStall:    StallToolProgress,
		},
		{
			// A genuinely-silent pipe lane (pipe-read also stale) still stalls —
			// dead-lane detection is preserved.
			name: "stale pipe-read still stalls (dead-lane detection preserved)",
			in: Activity{
				SessionState:     "active",
				Transport:        TransportPipe,
				RegisteredAt:     at(now.Add(-30 * time.Minute)),
				LastMCPRequestAt: at(staleProtocol),
				LastPipeReadAt:   at(staleProtocol),
			},
			wantProtocol: ProtocolStalled,
			wantStall:    StallProtocolIdle,
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

// TestUpgradeProbeBasisConfirmedDead (RFC 0131 Layer 1) asserts the recovery
// decision tree's basis upgrade: a deadline_elapsed_only verdict becomes
// pty_confirmed_dead ONLY for a pty_helper lane (a pipe/unknown lane has no PTY
// oracle, so it stays deadline_elapsed_only), and a basis that is not
// deadline_elapsed_only is never altered.
func TestUpgradeProbeBasisConfirmedDead(t *testing.T) {
	tests := []struct {
		name      string
		transport TransportType
		current   ProbeBasis
		want      ProbeBasis
	}{
		{
			name:      "pty_helper deadline upgrades to confirmed_dead",
			transport: TransportPTYHelper,
			current:   ProbeBasisDeadlineElapsedOnly,
			want:      ProbeBasisPTYConfirmedDead,
		},
		{
			name:      "pipe deadline stays deadline_elapsed_only",
			transport: TransportPipe,
			current:   ProbeBasisDeadlineElapsedOnly,
			want:      ProbeBasisDeadlineElapsedOnly,
		},
		{
			name:      "unknown transport deadline stays deadline_elapsed_only",
			transport: TransportUnknown,
			current:   ProbeBasisDeadlineElapsedOnly,
			want:      ProbeBasisDeadlineElapsedOnly,
		},
		{
			name:      "already confirmed_dead is unchanged",
			transport: TransportPTYHelper,
			current:   ProbeBasisPTYConfirmedDead,
			want:      ProbeBasisPTYConfirmedDead,
		},
		{
			name:      "empty basis (non-stall) is never upgraded",
			transport: TransportPTYHelper,
			current:   ProbeBasisNone,
			want:      ProbeBasisNone,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := UpgradeProbeBasisConfirmedDead(tc.transport, tc.current)
			if got != tc.want {
				t.Fatalf("upgrade(%q, %q) = %q, want %q", tc.transport, tc.current, got, tc.want)
			}
		})
	}
}

// TestActivityFromRowDerivesTransport (RFC 0131 Layer 1) asserts ActivityFromRow
// reads the supervised lane's transport from the supervisor pointer metadata in
// whatever shape pgx hands it back (a map, a JSON string, raw JSON bytes), and
// degrades to TransportUnknown when the metadata is absent or unrecognized.
func TestActivityFromRowDerivesTransport(t *testing.T) {
	tests := []struct {
		name string
		row  map[string]any
		want TransportType
	}{
		{
			name: "pty_helper from map metadata",
			row:  map[string]any{SupervisorPointerMetadata: map[string]any{"transport": "pty_helper"}},
			want: TransportPTYHelper,
		},
		{
			name: "pipe from JSON-string metadata",
			row:  map[string]any{SupervisorPointerMetadata: `{"transport":"pipe","require_tmux":false}`},
			want: TransportPipe,
		},
		{
			name: "pty_helper from JSON-bytes metadata",
			row:  map[string]any{SupervisorPointerMetadata: []byte(`{"transport":"pty_helper"}`)},
			want: TransportPTYHelper,
		},
		{
			name: "absent metadata is unknown",
			row:  map[string]any{},
			want: TransportUnknown,
		},
		{
			name: "unrecognized transport value is unknown",
			row:  map[string]any{SupervisorPointerMetadata: map[string]any{"transport": "carrier_pigeon"}},
			want: TransportUnknown,
		},
		{
			name: "malformed metadata json is unknown",
			row:  map[string]any{SupervisorPointerMetadata: "{not json"},
			want: TransportUnknown,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ActivityFromRow(tc.row).Transport
			if got != tc.want {
				t.Fatalf("transport = %q, want %q", got, tc.want)
			}
			// TransportFromRow is the standalone derivation ActivityFromRow uses;
			// assert they agree.
			if direct := TransportFromRow(tc.row); direct != tc.want {
				t.Fatalf("TransportFromRow = %q, want %q", direct, tc.want)
			}
		})
	}
}
