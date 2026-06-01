package adapterconformance

import (
	"context"
	"fmt"
	"testing"

	"github.com/halbritt/striatum/go/pkg/adapterconformance/testagent"
)

// These are the RFC 0101 Layer 2 Tier-A taxonomy-arming self-tests: they drive
// the fake-agent lifecycle against an in-process daemon (NewHarness over pgtest)
// and assert the happy path passes every in-scope clause and each broken mode
// fails EXACTLY its declared contract FailureClass. They are PG-gated: NewHarness
// calls pgtest.Pool, which t.Skip()s cleanly when STRIATUM_PG_TEST_URL is unset,
// so they need a CI PostgreSQL but never fail without one.

// newRun seeds the fixture and harness for one conformance run. repoID is unique
// per test so parallel pgtest databases do not collide.
func newRun(t *testing.T, repoID string) (context.Context, *Harness, *Fixture) {
	t.Helper()
	ctx := context.Background()
	h := NewHarness(t, repoID) // SKIPs here when STRIATUM_PG_TEST_URL is unset.
	fx := SeedFixture(t, ctx, h.Runner, repoID)
	return ctx, h, fx
}

// requireClause fetches a clause result or fails the test.
func requireClause(t *testing.T, report LiveReport, id ClauseID) ClauseResult {
	t.Helper()
	res, ok := report.Clause(id)
	if !ok {
		t.Fatalf("clause %s not evaluated; clauses=%v", id, report.Clauses)
	}
	return res
}

// assertPass asserts a clause passed.
func assertPass(t *testing.T, report LiveReport, id ClauseID) {
	t.Helper()
	res := requireClause(t, report, id)
	if res.Status != StatusPass {
		t.Fatalf("clause %s status = %s (%s / %s), want Pass", id, res.Status, res.FailureClass, res.Message)
	}
}

// assertContractFail asserts a clause failed with exactly the expected class.
func assertContractFail(t *testing.T, report LiveReport, id ClauseID, want FailureClass) {
	t.Helper()
	res := requireClause(t, report, id)
	if res.Status != StatusContractFail {
		t.Fatalf("clause %s status = %s (%s), want ContractFail %s", id, res.Status, res.Message, want)
	}
	if res.FailureClass != want {
		t.Fatalf("clause %s FailureClass = %s, want %s (%s)", id, res.FailureClass, want, res.Message)
	}
	if !res.FailureClass.IsContract() {
		t.Fatalf("clause %s class %s is not in the contract partition", id, res.FailureClass)
	}
}

// TestRunLiveHappyPathAllClausesPass drives the full happy-path lifecycle WITH a
// scripted interrogation and asserts every in-scope clause (C3..C10 + C7) passes.
func TestRunLiveHappyPathAllClausesPass(t *testing.T) {
	ctx, h, fx := newRun(t, "repo_conf_happy")

	report := RunLive(t, ctx, h, fx, "fake_agent", testagent.ModeHappy, LiveScope{IncludeInterrogation: true})
	if report.AgentErr != nil {
		t.Fatalf("happy-path agent error: %v", report.AgentErr)
	}
	if !report.AgentResult.Completed || !report.AgentResult.ReachedIdle {
		t.Fatalf("happy path should complete and reach idle: %+v", report.AgentResult)
	}
	for _, id := range []ClauseID{C3, C4, C5, C6, C7, C8, C9, C10} {
		assertPass(t, report, id)
	}
	if _, failed := report.FirstContractFailure(); failed {
		t.Fatalf("happy path had a contract failure: %v", report.Clauses)
	}
}

// TestRunLiveHappyPathWithoutInterrogation drives the happy path WITHOUT a
// scripted interrogation: C7 is out of scope and not evaluated, every other
// in-scope clause passes.
func TestRunLiveHappyPathWithoutInterrogation(t *testing.T) {
	ctx, h, fx := newRun(t, "repo_conf_happy_nointg")

	report := RunLive(t, ctx, h, fx, "fake_agent", testagent.ModeHappy, LiveScope{})
	if report.AgentErr != nil {
		t.Fatalf("agent error: %v", report.AgentErr)
	}
	for _, id := range []ClauseID{C3, C4, C5, C6, C8, C9, C10} {
		assertPass(t, report, id)
	}
	if _, ok := report.Clause(C7); ok {
		t.Fatalf("C7 should not be evaluated when interrogation is out of scope")
	}
}

// TestRunLiveBrokenModesArmTaxonomy proves each broken mode yields EXACTLY its
// declared contract FailureClass on the first failing in-scope clause.
func TestRunLiveBrokenModesArmTaxonomy(t *testing.T) {
	cases := []struct {
		mode          testagent.Mode
		clause        ClauseID
		want          FailureClass
		interrogation bool
	}{
		{testagent.ModeNeverToolsList, C3, BootstrapStall, false},
		{testagent.ModeAwaitNever, C4, AwaitPacketStall, false},
		{testagent.ModeAckNever, C5, AckStall, false},
		{testagent.ModeNoHeartbeat, C6, HeartbeatMissed, false},
		{testagent.ModeIgnoreInterrogation, C7, InterrogationIgnored, true},
		{testagent.ModeExitBeforeComplete, C9, CompleteMissing, false},
	}
	for i, tc := range cases {
		tc := tc
		t.Run(string(tc.mode), func(t *testing.T) {
			repoID := fmt.Sprintf("repo_conf_broken_%d", i)
			ctx, h, fx := newRun(t, repoID)
			report := RunLive(t, ctx, h, fx, "fake_agent", tc.mode, LiveScope{IncludeInterrogation: tc.interrogation})
			// The broken modes deliberately stop early; the agent returns no error.
			if report.AgentErr != nil {
				t.Fatalf("mode %s unexpected agent error: %v", tc.mode, report.AgentErr)
			}
			assertContractFail(t, report, tc.clause, tc.want)
		})
	}
}

// TestRunLiveExitBeforeCompleteAlsoMissesArtifactAndLoop documents that an
// agent that exits before complete fails C9 (CompleteMissing) and the no-work
// loop clause C10 (NoWorkLoopMissed), while still passing the earlier clauses it
// did satisfy (C3..C6) and C8 (it did publish the artifact before exiting).
func TestRunLiveExitBeforeCompleteAlsoMissesArtifactAndLoop(t *testing.T) {
	ctx, h, fx := newRun(t, "repo_conf_exit_early")
	report := RunLive(t, ctx, h, fx, "fake_agent", testagent.ModeExitBeforeComplete, LiveScope{})

	for _, id := range []ClauseID{C3, C4, C5, C6} {
		assertPass(t, report, id)
	}
	// It published the artifact before exiting, so C8 passes.
	assertPass(t, report, C8)
	// But it never completed (C9) and never re-awaited (C10).
	assertContractFail(t, report, C9, CompleteMissing)
	assertContractFail(t, report, C10, NoWorkLoopMissed)
}
