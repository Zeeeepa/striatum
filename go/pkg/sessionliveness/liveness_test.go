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
