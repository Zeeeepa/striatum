package lanehealth

import (
	"context"
	"reflect"
	"testing"
	"time"

	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

func TestClassify(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name     string
		facts    Facts
		expected Health
	}{
		{
			name: "No supervisor recorded",
			facts: Facts{
				SupervisorRecorded: false,
			},
			expected: Health{
				Reason: ReasonNoAttachedSupervisor,
			},
		},
		{
			name: "Supervisor not attached",
			facts: Facts{
				SupervisorRecorded: true,
				SupervisorState:    "detached",
			},
			expected: Health{
				Reason: ReasonNoAttachedSupervisor,
			},
		},
		{
			name: "PID missing",
			facts: Facts{
				SupervisorRecorded: true,
				SupervisorState:    "attached",
				PID:                0,
			},
			expected: Health{
				Reason: ReasonPIDMissing,
			},
		},
		{
			name: "Daemon supervisor missing (no pointer)",
			facts: Facts{
				SupervisorRecorded: true,
				SupervisorState:    "attached",
				PID:                123,
				HasPointer:         false,
			},
			expected: Health{
				Reason: ReasonDaemonSupervisorMissing,
				PID:    123,
			},
		},
		{
			name: "Pointer state mismatch",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "detached",
			},
			expected: Health{
				Reason: ReasonPointerStateMismatch,
				PID:    123,
			},
		},
		{
			name: "Daemon supervisor missing (no daemon record)",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       false,
			},
			expected: Health{
				Reason: ReasonDaemonSupervisorMissing,
				PID:    123,
			},
		},
		{
			name: "Daemon state mismatch",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       true,
				DaemonSupervisorID:        "dsup_1",
				DaemonState:               "detached",
			},
			expected: Health{
				Reason: ReasonDaemonStateMismatch,
				PID:    123,
			},
		},
		{
			name: "Pointer PID mismatch",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       true,
				DaemonSupervisorID:        "dsup_1",
				DaemonState:               "attached",
				PointerPID:                456,
			},
			expected: Health{
				Reason: ReasonPointerPIDMismatch,
				PID:    123,
			},
		},
		{
			name: "Tmux metadata corrupt",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       true,
				DaemonSupervisorID:        "dsup_1",
				DaemonState:               "attached",
				PointerPID:                123,
				PointerTmuxMeta: gosupervisor.TmuxMeta{
					Tmux: gosupervisor.TmuxMetaBlock{
						State: "backed", // IsValidTmux() will be false since pane_pid/sessionname/paneid are empty
					},
				},
			},
			expected: Health{
				Reason: ReasonTmuxMetadataCorrupt,
				PID:    123,
			},
		},
		{
			name: "Probe performed: PID gone",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       true,
				DaemonSupervisorID:        "dsup_1",
				DaemonState:               "attached",
				PointerPID:                123,
				ProbePerformed:            true,
				ProbeResult: gosupervisor.LaneLiveness{
					Alive: false,
					Class: "pid_gone",
				},
			},
			expected: Health{
				Bound:         true,
				Alive:         false,
				Reason:        ReasonPIDGone,
				LivenessClass: "pid_gone",
				PID:           123,
			},
		},
		{
			name: "Probe performed: start token unverified",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       true,
				DaemonSupervisorID:        "dsup_1",
				DaemonState:               "attached",
				PointerPID:                123,
				ProbePerformed:            true,
				ProbeResult: gosupervisor.LaneLiveness{
					Alive:  true,
					Backed: "tmux",
					Detail: "start_token_unverified",
				},
			},
			expected: Health{
				Bound:  true,
				Alive:  true,
				Reason: ReasonStartTokenUnverified,
				PID:    123,
			},
		},
		{
			name: "Attested and healthy lane",
			facts: Facts{
				SupervisorRecorded:        true,
				SupervisorState:           "attached",
				PID:                       123,
				HasPointer:                true,
				PointerDaemonSupervisorID: "dsup_1",
				PointerState:              "attached",
				HasDaemonSupervisor:       true,
				DaemonSupervisorID:        "dsup_1",
				DaemonState:               "attached",
				PointerPID:                123,
				ProbePerformed:            true,
				ProbeResult: gosupervisor.LaneLiveness{
					Alive: true,
					Class: "alive",
				},
			},
			expected: Health{
				Bound:         true,
				Alive:         true,
				Attested:      true,
				LivenessClass: "alive",
				PID:           123,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Classify(tt.facts, now)
			// Reset stall details in result for easy comparison
			res.Stall = tt.expected.Stall
			res.SupervisorID = tt.expected.SupervisorID
			res.Deliverable = tt.expected.Deliverable
			if !reflect.DeepEqual(res, tt.expected) {
				t.Errorf("Classify() = %#v, expected %#v", res, tt.expected)
			}
		})
	}
}

type namedTmuxRunner struct {
	name string
}

func (r namedTmuxRunner) Run(_ context.Context, _ ...string) (string, error) {
	return "", nil
}

func TestProdProbeSelectsRunAsTmuxRunnerFromMetadata(t *testing.T) {
	defaultRunner := namedTmuxRunner{name: "default"}
	probe := ProdProbe{
		Runner: defaultRunner,
		RunAsRunner: func(runAsUser string) gosupervisor.TmuxRunner {
			if runAsUser != "striatum-lane" {
				t.Fatalf("RunAsRunner user = %q, want striatum-lane", runAsUser)
			}
			return namedTmuxRunner{name: "run-as"}
		},
	}

	runner := probe.runnerForMeta(gosupervisor.TmuxMeta{
		Tmux: gosupervisor.TmuxMetaBlock{RunAsUser: " striatum-lane "},
	})
	got, ok := runner.(namedTmuxRunner)
	if !ok || got.name != "run-as" {
		t.Fatalf("runnerForMeta returned %#v, want run-as runner", runner)
	}
}

func TestProdProbeUsesInjectedRunnerWithoutRunAsMetadata(t *testing.T) {
	defaultRunner := namedTmuxRunner{name: "default"}
	probe := ProdProbe{
		Runner: defaultRunner,
		RunAsRunner: func(runAsUser string) gosupervisor.TmuxRunner {
			t.Fatalf("RunAsRunner called for user %q", runAsUser)
			return nil
		},
	}

	runner := probe.runnerForMeta(gosupervisor.TmuxMeta{})
	got, ok := runner.(namedTmuxRunner)
	if !ok || got.name != "default" {
		t.Fatalf("runnerForMeta returned %#v, want injected default runner", runner)
	}
}

// healthyBoundFacts returns Facts for a fully attached, bound, probe-alive lane
// (the structural prerequisites for delivery classification), parameterized by
// the recorded delivery degradation.
func healthyBoundFacts(deliveryDegraded bool, deliveryReason string, probeAlive bool) Facts {
	return Facts{
		SupervisorRecorded:        true,
		SupervisorState:           "attached",
		PID:                       123,
		HasPointer:                true,
		PointerDaemonSupervisorID: "dsup_1",
		PointerState:              "attached",
		HasDaemonSupervisor:       true,
		DaemonSupervisorID:        "dsup_1",
		DaemonState:               "attached",
		PointerPID:                123,
		ProbePerformed:            true,
		ProbeResult:               gosupervisor.LaneLiveness{Alive: probeAlive, Class: "alive"},
		DeliveryDegraded:          deliveryDegraded,
		DeliveryReason:            deliveryReason,
	}
}

// TestClassifyAttachClientExitedDeliveryReconciliation guards #63 F7: an exited
// tmux attach-session OBSERVER client must not mark packet delivery degraded
// while the pane is alive and the real transport is healthy. Genuine transport
// failures (helper_process_gone, stdin_reader_missing) must still degrade
// delivery, and an attach exit on a NOT-alive pane must not be silently cleared.
func TestClassifyAttachClientExitedDeliveryReconciliation(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name            string
		facts           Facts
		wantDeliverable bool
		wantReason      string
	}{
		{
			name:            "benign attach exit on a live pane is deliverable",
			facts:           healthyBoundFacts(true, "attach_client_exited", true),
			wantDeliverable: true,
			wantReason:      "",
		},
		{
			name:            "helper_process_gone still degrades delivery",
			facts:           healthyBoundFacts(true, "helper_process_gone", true),
			wantDeliverable: false,
			wantReason:      "helper_process_gone",
		},
		{
			name:            "stdin_reader_missing still degrades delivery",
			facts:           healthyBoundFacts(true, "stdin_reader_missing", true),
			wantDeliverable: false,
			wantReason:      "stdin_reader_missing",
		},
		{
			// Without positive probe evidence that the pane is alive we keep the
			// recorded degradation rather than assume the transport is healthy.
			name:            "attach exit on a dead pane is not cleared",
			facts:           healthyBoundFacts(true, "attach_client_exited", false),
			wantDeliverable: false,
			wantReason:      "attach_client_exited",
		},
		{
			name:            "healthy lane stays deliverable",
			facts:           healthyBoundFacts(false, "", true),
			wantDeliverable: true,
			wantReason:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.facts, now)
			if got.Deliverable != tt.wantDeliverable {
				t.Fatalf("Deliverable = %v, want %v (health=%#v)", got.Deliverable, tt.wantDeliverable, got)
			}
			if got.DeliveryReason != tt.wantReason {
				t.Fatalf("DeliveryReason = %q, want %q", got.DeliveryReason, tt.wantReason)
			}
		})
	}
}

func TestLegacyMapCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		health   Health
		expected map[string]any
	}{
		{
			name: "Attested",
			health: Health{
				SupervisorID:  "sup_1",
				PID:           123,
				Attested:      true,
				LivenessClass: "alive",
			},
			expected: map[string]any{
				"attested":      true,
				"state":         "attested",
				"supervisor_id": "sup_1",
				"pid":           123,
				"reason":        nil,
				"liveness":      "alive",
			},
		},
		{
			name: "Unattested - no attached supervisor",
			health: Health{
				Reason: ReasonNoAttachedSupervisor,
			},
			expected: map[string]any{
				"attested":      false,
				"state":         "unattested",
				"supervisor_id": nil,
				"pid":           nil,
				"reason":        "no_attached_supervisor",
			},
		},
		{
			name: "Unattested - pid missing",
			health: Health{
				SupervisorID: "sup_1",
				Reason:       ReasonPIDMissing,
			},
			expected: map[string]any{
				"attested":      false,
				"state":         "unattested",
				"supervisor_id": "sup_1",
				"pid":           nil,
				"reason":        "pid_missing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LegacyMap(tt.health)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("LegacyMap() = %#v, expected %#v", got, tt.expected)
			}
		})
	}
}
