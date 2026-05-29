package lanehealth

import (
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
