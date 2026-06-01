package adapterconformance

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/sessionliveness"
)

// ClauseID is the ordered conformance-clause identifier C0..C12 (DESIGN §1.1).
// Ordering matters: a later clause is only meaningful once the earlier one
// fired (you cannot heartbeat a lease you never claimed).
type ClauseID int

const (
	// C0 — AdapterContract (hermetic, no CLI): the bootstrap-delivery
	// construction in go/pkg/agentloop matches the declared per-adapter contract.
	C0 ClauseID = iota
	// C1 — AdapterResolved + ProviderReady (pre-flight gate). Slice 2.
	C1
	// C2 — LaneEnvHardened (hermetic): the supervised child env carries the
	// required keys and none of the banned secret-bearing vars (#87).
	C2
	// C3 — BootstrapDelivered: last_tools_list_at non-null within Discovery. Slice 2.
	C3
	// C4 — AwaitPacketForeground: last_await_packet_at advances + lease created. Slice 2.
	C4
	// C5 — PacketAcked: job acked within AckSeconds. Slice 2.
	C5
	// C6 — HeartbeatObserved: last_work_heartbeat_at advances >=1x. Slice 2.
	C6
	// C7 — InterrogationRoundTrip: one interrogation answered before deadline. Slice 2.
	C7
	// C8 — ArtifactPublished: expected artifact row + valid front matter. Slice 2.
	C8
	// C9 — WorkCompleted: job completed; last_work_complete_at set. Slice 2.
	C9
	// C10 — ReceiveLoopContinues: next await returns no_work, no idle stall. Slice 2.
	C10
	// C11 — NoPrematureExit: Full lanes stay alive through C10. Slice 2.
	C11
	// C12 — NoWorkTreeCredentialLeak: no bearer / .gemini / helper residue. Slice 2.
	C12
)

// Name returns the canonical clause name (e.g. "C0_AdapterContract").
func (id ClauseID) Name() string {
	switch id {
	case C0:
		return "C0_AdapterContract"
	case C1:
		return "C1_AdapterResolvedProviderReady"
	case C2:
		return "C2_LaneEnvHardened"
	case C3:
		return "C3_BootstrapDelivered"
	case C4:
		return "C4_AwaitPacketForeground"
	case C5:
		return "C5_PacketAcked"
	case C6:
		return "C6_HeartbeatObserved"
	case C7:
		return "C7_InterrogationRoundTrip"
	case C8:
		return "C8_ArtifactPublished"
	case C9:
		return "C9_WorkCompleted"
	case C10:
		return "C10_ReceiveLoopContinues"
	case C11:
		return "C11_NoPrematureExit"
	case C12:
		return "C12_NoWorkTreeCredentialLeak"
	default:
		return "C?_Unknown"
	}
}

// String returns the short clause id ("C0".."C12").
func (id ClauseID) String() string {
	if id < C0 || id > C12 {
		return "C?"
	}
	return fmt.Sprintf("C%d", int(id))
}

// ContractProfile selects which clauses apply to an adapter (DESIGN §1.5).
//
//   - Full        — claude_code, codex: the multi-turn contract C0..C12.
//   - SingleShot  — agy: a reduced single-turn profile (the #95 persistent
//     multi-turn self-driving loop is deferred); the multi-turn clauses are
//     skipped via the ledger.
type ContractProfile string

const (
	Full       ContractProfile = "Full"
	SingleShot ContractProfile = "SingleShot"
)

// AdapterSpec declares one adapter under test: its name, the real
// bare-interactive command the agent-loop launches it with, and its profile.
// The Command is the argv0 (+ baseline flags) the production agent-loop sees;
// the conformance harness derives the bootstrap-delivery contract from it.
type AdapterSpec struct {
	Name    string
	Command []string
	Profile ContractProfile
}

// Adapters returns the three real adapters under test with their production
// bare-interactive commands (DESIGN §1.1). These are the argv0s the agent-loop
// keys on (loop.go bootstrapDeliveryModeFor / mcpconfig.go injectLaneMCPConfig);
// baseline interactive flags that do not affect the bootstrap contract are
// included to match the shapes a real lane uses.
func Adapters() []AdapterSpec {
	return []AdapterSpec{
		{
			Name:    "claude_code",
			Command: []string{"claude"},
			Profile: Full,
		},
		{
			Name:    "codex",
			Command: []string{"codex"},
			Profile: Full,
		},
		{
			Name:    "agy",
			Command: []string{"agy"},
			Profile: SingleShot,
		},
	}
}

// Status is the outcome category of a clause assertion (DESIGN §1.1/§1.3).
type Status string

const (
	// StatusPass — the clause asserted true.
	StatusPass Status = "Pass"
	// StatusContractFail — a contract-partition FailureClass (hard-blocks).
	StatusContractFail Status = "ContractFail"
	// StatusInfraOutcome — an infra-partition outcome (gated, re-runnable).
	StatusInfraOutcome Status = "InfraOutcome"
	// StatusDeferred — the clause is declared but its assertion requires the
	// Slice-2 live/fake-agent runner; it is NOT run (and NOT faked) in Slice 1.
	StatusDeferred Status = "Deferred"
)

// ClauseResult carries the outcome of one clause assertion plus the structured
// routing fields (DESIGN §1.2). FailureClass is FailureNone for a pass or a
// deferred clause.
type ClauseResult struct {
	Clause       ClauseID          `json:"clause"`
	ClauseName   string            `json:"clause_name"`
	Adapter      string            `json:"adapter"`
	Status       Status            `json:"status"`
	FailureClass FailureClass      `json:"failure_class,omitempty"`
	Message      string            `json:"message,omitempty"`
	Fields       map[string]string `json:"fields,omitempty"`
}

// pass builds a passing ClauseResult for the clause/adapter.
func (c Clause) pass(adapter, message string) ClauseResult {
	return ClauseResult{
		Clause:     c.ID,
		ClauseName: c.ID.Name(),
		Adapter:    adapter,
		Status:     StatusPass,
		Message:    message,
	}
}

// contractFail builds a contract-failure ClauseResult. The class MUST be a
// contract-partition class.
func (c Clause) contractFail(adapter string, class FailureClass, message string, fields map[string]string) ClauseResult {
	return ClauseResult{
		Clause:       c.ID,
		ClauseName:   c.ID.Name(),
		Adapter:      adapter,
		Status:       StatusContractFail,
		FailureClass: class,
		Message:      message,
		Fields:       fields,
	}
}

// deferred builds a Slice-2 deferred ClauseResult: the clause is declared with
// its expected FailureClass but is NOT asserted in Slice 1.
func (c Clause) deferred(adapter string) ClauseResult {
	return ClauseResult{
		Clause:       c.ID,
		ClauseName:   c.ID.Name(),
		Adapter:      adapter,
		Status:       StatusDeferred,
		FailureClass: FailureNone,
		Message:      "not asserted on the hermetic Tier-A path; C3..C10/C7 are asserted live by RunLive (runner.go), C1/C11/C12 are deferred to a follow-up slice (see doc.go)",
	}
}

// AssertInput carries the inputs a clause Assert needs. In Slice 1 only the
// hermetic clauses (C0, C2) consume it; the deferred clauses ignore it. In
// Slice 2 this grows a DaemonObserver and live timing.
type AssertInput struct {
	Adapter AdapterSpec
	// BaseEnv is the controlled base environment for the C2 lane-env golden
	// (the daemon's os.Environ() in production). When nil, C2 uses os.Environ().
	BaseEnv []string
}

// Clause is one ordered conformance clause (DESIGN §1.3). Deadline is the
// derived live deadline (DefaultPolicy x DEADLINE_SCALE) for the live clauses;
// it is zero for the hermetic clauses (C0, C2) which have no wall-clock budget.
// OnFailure is the contract FailureClass this clause emits when it fails (for
// the deferred clauses this documents the class the Slice-2 runner will emit).
type Clause struct {
	ID        ClauseID
	Deadline  time.Duration
	Profiles  []ContractProfile
	OnFailure FailureClass
	// Assert evaluates the clause. In Slice 1, C0 and C2 return real Pass/
	// ContractFail results; every other clause returns StatusDeferred.
	Assert func(in AssertInput) ClauseResult
}

// AppliesTo reports whether the clause runs for the given profile.
func (c Clause) AppliesTo(profile ContractProfile) bool {
	for _, p := range c.Profiles {
		if p == profile {
			return true
		}
	}
	return false
}

const (
	// DeadlineScaleEnv overrides the live-deadline multiplier.
	DeadlineScaleEnv = "STRIATUM_CONFORMANCE_DEADLINE_SCALE"
	// DefaultDeadlineScale is the default multiplier on DefaultPolicy() deadlines
	// (DESIGN §1.1): comfortably absorbs model/PTY latency while staying well
	// under a CI job timeout.
	DefaultDeadlineScale = 3.0
)

// DeadlineScale reads STRIATUM_CONFORMANCE_DEADLINE_SCALE (default 3.0). An
// unparseable or non-positive value falls back to the default.
func DeadlineScale() float64 {
	raw := strings.TrimSpace(os.Getenv(DeadlineScaleEnv))
	if raw == "" {
		return DefaultDeadlineScale
	}
	scale, err := strconv.ParseFloat(raw, 64)
	if err != nil || scale <= 0 {
		return DefaultDeadlineScale
	}
	return scale
}

// scaledDeadline derives a live clause deadline from a DefaultPolicy() seconds
// value scaled by DeadlineScale().
func scaledDeadline(seconds int) time.Duration {
	base := time.Duration(seconds) * time.Second
	return time.Duration(float64(base) * DeadlineScale())
}

// Contract returns the ordered C0..C12 clause list for the given adapter. C0
// and C2 carry real hermetic Asserts; C1 and C3..C12 are declared with their
// ID/Deadline/Profile/OnFailure and a deferred Assert (Slice 2). The clauses
// are returned in ascending ClauseID order.
func Contract() []Clause {
	policy := sessionliveness.DefaultPolicy()
	full := []ContractProfile{Full}
	both := []ContractProfile{Full, SingleShot}

	return []Clause{
		{
			ID:        C0,
			Deadline:  0, // hermetic: no wall-clock budget
			Profiles:  both,
			OnFailure: AdapterContractViolation,
			Assert:    assertC0,
		},
		{
			ID:        C1,
			Deadline:  scaledDeadline(policy.DiscoverySeconds),
			Profiles:  both,
			OnFailure: AdapterUnavailable, // infra; the pre-flight gate (Slice 2)
			Assert:    deferredAssert(C1),
		},
		{
			ID:        C2,
			Deadline:  0, // hermetic: env snapshot, no wall-clock budget
			Profiles:  both,
			OnFailure: EnvSecretLeak,
			Assert:    assertC2,
		},
		{
			ID:        C3,
			Deadline:  scaledDeadline(policy.DiscoverySeconds),
			Profiles:  both,
			OnFailure: BootstrapStall,
			Assert:    deferredAssert(C3),
		},
		{
			ID:        C4,
			Deadline:  scaledDeadline(policy.AwaitPacketSeconds),
			Profiles:  both,
			OnFailure: AwaitPacketStall,
			Assert:    deferredAssert(C4),
		},
		{
			ID:        C5,
			Deadline:  scaledDeadline(policy.AckSeconds),
			Profiles:  both,
			OnFailure: AckStall,
			Assert:    deferredAssert(C5),
		},
		{
			ID:        C6,
			Deadline:  scaledDeadline(policy.LeaseHeartbeatSeconds),
			Profiles:  full,
			OnFailure: HeartbeatMissed,
			Assert:    deferredAssert(C6),
		},
		{
			ID:        C7,
			Deadline:  scaledDeadline(policy.ProtocolIdleSeconds),
			Profiles:  full,
			OnFailure: InterrogationIgnored,
			Assert:    deferredAssert(C7),
		},
		{
			ID:        C8,
			Deadline:  scaledDeadline(policy.ProtocolIdleSeconds),
			Profiles:  both,
			OnFailure: ArtifactMissing,
			Assert:    deferredAssert(C8),
		},
		{
			ID:        C9,
			Deadline:  scaledDeadline(policy.ProtocolIdleSeconds),
			Profiles:  both,
			OnFailure: CompleteMissing,
			Assert:    deferredAssert(C9),
		},
		{
			ID:        C10,
			Deadline:  scaledDeadline(policy.ProtocolIdleSeconds),
			Profiles:  full,
			OnFailure: NoWorkLoopMissed,
			Assert:    deferredAssert(C10),
		},
		{
			ID:        C11,
			Deadline:  scaledDeadline(policy.ProtocolIdleSeconds),
			Profiles:  full,
			OnFailure: TurnExitEarly,
			Assert:    deferredAssert(C11),
		},
		{
			ID:        C12,
			Deadline:  0, // post-teardown filesystem/git scan, no wall-clock budget
			Profiles:  both,
			OnFailure: TokenLeakInWorkTree,
			Assert:    deferredAssert(C12),
		},
	}
}

// deferredAssert returns an Assert that records the clause as NOT asserted on
// the hermetic Tier-A path. The daemon-lifecycle clauses C3..C10 (and C7) ARE
// asserted live by the Slice-2 fake-agent runner (RunLive in runner.go) over an
// in-process striatumd + pgtest; they remain deferred here only because the
// hermetic path has no daemon to observe. C1 (real-CLI pre-flight), C11
// (no-premature-exit through the PTY path), and C12 (work-tree credential scan)
// stay fully deferred to a follow-up slice (see doc.go). It never fakes a
// Pass/Fail result.
func deferredAssert(id ClauseID) func(AssertInput) ClauseResult {
	return func(in AssertInput) ClauseResult {
		c := Clause{ID: id}
		return c.deferred(in.Adapter.Name)
	}
}

// LiveClauses are the daemon-lifecycle clause ids the Slice-2 fake-agent runner
// (RunLive) asserts live against the in-process daemon over pgtest. C7 is
// asserted only when an interrogation is scripted (LiveScope.IncludeInterrogation).
func LiveClauses() []ClauseID { return []ClauseID{C3, C4, C5, C6, C7, C8, C9, C10} }

// StillDeferredClauses are the clause ids that remain deferred after Slice 2:
// C1 (real-CLI pre-flight), C11 (no-premature-exit via supervise.start/PTY), and
// C12 (work-tree credential scan). See doc.go for the follow-up.
func StillDeferredClauses() []ClauseID { return []ClauseID{C1, C11, C12} }
