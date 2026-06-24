package mutations

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0135 P2 (D215/D216/D269) — recoverable fan-in assembly with two-phase
// journaling + N=1 unification (#346). This graduates assembleFaninBarrier into a
// CRASH-RECOVERABLE operation backed by the striatumd.barrier_state journal
// (runtime migration 0030: sealed -> assembling -> committed|failed).
//
// CUTOVER DISCIPLINE: confirmed fan-in runs use this assembly by default at the
// downstream gate. STRIATUM_BARRIER_FANIN=0 restores the D206 per-completion
// run-branch merge. The explicit barrier_assembly job type remains as a compatibility
// dispatcher surface and still requires owner bundle 0013 before such a job can be
// persisted.
//
// TWO-STORE CONSISTENCY (RFC 0133 / RFC 0135 Risks). The PG state transition and
// the git CAS that advances the run branch cannot share one transaction, so the
// assembly is a two-phase journal:
//
//   - Phase 1 (journal the intent): compute the deterministic assembled commit via
//     assembleFaninBarrier, write target_commit_sha + tree_sha (+ base) to
//     barrier_state with state='assembling' — BEFORE touching git refs.
//   - Phase 2 (apply + finalize): CAS-advance the run branch to the journaled
//     target commit (idempotent: a re-run that finds the run branch already at /
//     descended from the target recognizes its OWN commit and does not
//     double-commit), then flip barrier_state to 'committed'.
//
// CRASH-MID-ASSEMBLY RECOVERY. A crashed assembler re-runs runBarrierAssembly. It
// reads the journaled state:
//   - 'committed': nothing to do (idempotent return).
//   - 'assembling' with a journaled target: it RECOGNIZES its own intent. If the
//     deterministic re-assembly yields the SAME target commit (the assembly is a
//     pure function of the frozen tip + live-staged contributions, which do not
//     change once the barrier has fired), it resumes Phase 2 against the journaled
//     target rather than re-deriving a different commit — never wedging, never
//     double-committing. (A journaled target that no longer matches the
//     deterministic re-assembly is a corruption signal, surfaced loudly.)
//   - 'sealed' / absent: a fresh assembly from Phase 1.
//
// N=1 UNIFICATION. A single-sibling barrier is NOT special-cased: it routes
// through this same path. assembleFaninBarrier folds the one staged contribution
// onto the frozen tip exactly as it folds N, producing the same commit shape the
// N>1 path produces, so linear (N=1) and fan-in (N>1) share one assembly, one
// journal, one recovery story.

// barrierAssemblyState is the journaled progress of a barrier_assembly job — the
// striatumd.barrier_state row. State walks sealed -> assembling -> committed|failed.
type barrierAssemblyState struct {
	Present         bool
	State           string
	TargetCommitSHA string
	TreeSHA         string
	BaseCommitSHA   string
	FailReason      string
}

const (
	barrierStateSealed     = "sealed"
	barrierStateAssembling = "assembling"
	barrierStateCommitted  = "committed"
	barrierStateFailed     = "failed"
)

// jobBarrierAssemblyTypePermitted reports whether the live striatumd.jobs
// jobs_job_type_check CHECK constraint permits the 'barrier_assembly' job_type —
// i.e. owner bundle 0013 has been applied (RFC 0135 P2). It probes
// pg_get_constraintdef rather than attempting the INSERT, so a behind-deployment
// is detected WITHOUT raising a CHECK violation that would abort the caller's
// transaction. This mirrors jobQuarantineStatePermitted (D209 guard d, #311 P0):
// the deployment-tolerance precedent. A missing constraint (renamed/dropped out of
// band) is treated as NOT permitted (fail-safe: prefer the proven D206
// per-completion path over an unguarded write that would CHECK-fail).
func jobBarrierAssemblyTypePermitted(ctx context.Context, runner db.TxRunner) (bool, error) {
	row, err := oneRow(ctx, runner, `
		SELECT COALESCE(bool_or(pg_get_constraintdef(oid) LIKE '%barrier_assembly%'), false) AS ok
		  FROM pg_constraint
		 WHERE conname = 'jobs_job_type_check'
		   AND conrelid = 'striatumd.jobs'::regclass`)
	if err != nil {
		return false, err
	}
	return row["ok"] == true, nil
}

// loadBarrierAssemblyState reads the barrier_state journal for a barrier. An
// absent row (no assembly has begun) returns Present=false.
func loadBarrierAssemblyState(ctx context.Context, runner db.TxRunner, repositoryID, barrierID string) (barrierAssemblyState, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT state,
		       COALESCE(target_commit_sha,'') AS target_commit_sha,
		       COALESCE(tree_sha,'')          AS tree_sha,
		       COALESCE(base_commit_sha,'')   AS base_commit_sha,
		       COALESCE(fail_reason,'')       AS fail_reason
		  FROM striatumd.barrier_state
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repositoryID, barrierID)
	if err != nil {
		return barrierAssemblyState{}, err
	}
	if len(rows) == 0 {
		return barrierAssemblyState{Present: false}, nil
	}
	r := rows[0]
	return barrierAssemblyState{
		Present:         true,
		State:           fmt.Sprint(r["state"]),
		TargetCommitSHA: fmt.Sprint(r["target_commit_sha"]),
		TreeSHA:         fmt.Sprint(r["tree_sha"]),
		BaseCommitSHA:   fmt.Sprint(r["base_commit_sha"]),
		FailReason:      fmt.Sprint(r["fail_reason"]),
	}, nil
}

// journalBarrierAssemblyIntent writes (or refreshes) the two-phase journal intent
// for a barrier in state='assembling' BEFORE the git CAS: target_commit_sha +
// tree_sha (+ base). Idempotent on (repository_id, barrier_id): a re-run updates
// the intent in place. This is Phase 1 of the two-phase journal.
func journalBarrierAssemblyIntent(ctx context.Context, runner db.TxRunner, repositoryID, barrierID, runID, downstreamWorkflowJobID, assemblyWorkflowJobID, assemblyJobID, base, target, tree string) error {
	return runner.Exec(ctx, `
		INSERT INTO striatumd.barrier_state
		  (repository_id, barrier_id, run_id, downstream_workflow_job_id,
		   assembly_workflow_job_id, assembly_job_id,
		   state, base_commit_sha, target_commit_sha, tree_sha, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,'assembling',$7,$8,$9, now())
		ON CONFLICT (repository_id, barrier_id) DO UPDATE SET
		  run_id                    = EXCLUDED.run_id,
		  downstream_workflow_job_id = EXCLUDED.downstream_workflow_job_id,
		  assembly_workflow_job_id  = EXCLUDED.assembly_workflow_job_id,
		  assembly_job_id           = EXCLUDED.assembly_job_id,
		  state                     = 'assembling',
		  base_commit_sha           = EXCLUDED.base_commit_sha,
		  target_commit_sha         = EXCLUDED.target_commit_sha,
		  tree_sha                  = EXCLUDED.tree_sha,
		  fail_reason               = NULL,
		  updated_at                = now()`,
		repositoryID, barrierID, runID, downstreamWorkflowJobID,
		assemblyWorkflowJobID, assemblyJobID, base, target, tree)
}

// markBarrierAssemblyCommitted flips the journal to 'committed' AFTER the run
// branch CAS has landed (Phase 2 finalize). Idempotent.
func markBarrierAssemblyCommitted(ctx context.Context, runner db.TxRunner, repositoryID, barrierID string) error {
	return runner.Exec(ctx, `
		UPDATE striatumd.barrier_state
		   SET state = 'committed', committed_at = now(), updated_at = now()
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repositoryID, barrierID)
}

// markBarrierAssemblyFailed records a terminal, surfaced assembly failure (a real
// content conflict / corruption) — never a silent retry loop.
func markBarrierAssemblyFailed(ctx context.Context, runner db.TxRunner, repositoryID, barrierID, reason string) error {
	return runner.Exec(ctx, `
		UPDATE striatumd.barrier_state
		   SET state = 'failed', fail_reason = $3, updated_at = now()
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repositoryID, barrierID, reason)
}

// barrierAssemblyResult is what runBarrierAssembly returns to the assembly job
// handler: the committed run-branch tip, the assembled tree, the manifest in-edges,
// and how the run got there (a fresh assembly, an idempotent no-op, or a resumed
// crash recovery).
type barrierAssemblyResult struct {
	CommitSHA string
	TreeSHA   string
	Edges     []faninManifestInEdge
	Mode      string // assembled | already_committed | resumed_recovery
}

// runBarrierAssembly is the recoverable, two-phase-journaled barrier assembly the
// `barrier_assembly` job runs under the per-run advisory lock the caller holds
// (RFC 0104 fire serialization). It folds every live-staged sibling contribution
// onto the frozen tip (reusing P1's assembleFaninBarrier) and CAS-advances the run
// branch to the assembled commit, journaling its intent to striatumd.barrier_state
// BEFORE the git CAS so a crash mid-assembly resumes idempotently.
//
// N=1 is NOT special-cased: a single declared sibling routes through this exact
// path (assembleFaninBarrier folds one staged contribution exactly as it folds N).
//
// runRef is the run-branch ref (e.g. refs/heads/<run-branch>); repoRoot is the
// daemon-owned repository the run branch lives in.
func runBarrierAssembly(ctx context.Context, runner db.TxRunner, repoRoot, runRef, repositoryID, barrierID, assemblyWorkflowJobID, assemblyJobID string) (barrierAssemblyResult, error) {
	fp, err := loadFaninFreezePoint(ctx, runner, repositoryID, barrierID)
	if err != nil {
		return barrierAssemblyResult{}, err
	}

	// Crash-recovery: read the journal first.
	prior, err := loadBarrierAssemblyState(ctx, runner, repositoryID, barrierID)
	if err != nil {
		return barrierAssemblyResult{}, err
	}

	// Already committed: idempotent no-op (a re-fire / re-run after success).
	if prior.Present && prior.State == barrierStateCommitted {
		edges, mErr := faninBarrierManifest(ctx, runner, repositoryID, barrierID)
		if mErr != nil {
			return barrierAssemblyResult{}, mErr
		}
		return barrierAssemblyResult{
			CommitSHA: prior.TargetCommitSHA,
			TreeSHA:   prior.TreeSHA,
			Edges:     edges,
			Mode:      "already_committed",
		}, nil
	}
	if prior.Present && prior.State == barrierStateFailed {
		return barrierAssemblyResult{}, rpc.NewError("invalid_transition", fmt.Sprintf(
			"barrier %q assembly is in terminal state 'failed' (%s); it must be recovered before re-assembly",
			barrierID, prior.FailReason),
			map[string]any{"barrier_id": barrierID, "fail_reason": prior.FailReason})
	}

	// Phase 1: deterministically (re)assemble. The assembly is a pure function of
	// the frozen tip + the live-staged contributions, which are frozen once the
	// barrier has fired, so a re-run yields the SAME target commit.
	treeSHA, commitSHA, edges, err := assembleFaninBarrier(ctx, runner, repoRoot, repositoryID, barrierID)
	if err != nil {
		// A real content conflict / plumbing failure is terminal and surfaced, not a
		// silent retry. Record it on the journal (best-effort) before propagating.
		if prior.Present {
			_ = markBarrierAssemblyFailed(ctx, runner, repositoryID, barrierID, err.Error())
		}
		return barrierAssemblyResult{}, err
	}

	// Crash-mid-assembly: if a prior 'assembling' intent exists, it must MATCH the
	// deterministic re-assembly. A mismatch is corruption (the journaled intent
	// disagrees with the only commit the staged contributions can produce) — surface
	// it loudly rather than silently double-committing a different tree.
	mode := "assembled"
	if prior.Present && prior.State == barrierStateAssembling && prior.TargetCommitSHA != "" {
		if prior.TargetCommitSHA != commitSHA {
			return barrierAssemblyResult{}, rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
				"barrier %q crash-recovery mismatch: journaled assembly intent %s does not match the deterministic re-assembly %s (corruption — the frozen contributions can only produce one commit)",
				barrierID, prior.TargetCommitSHA, commitSHA),
				map[string]any{"barrier_id": barrierID, "journaled_target": prior.TargetCommitSHA, "reassembled": commitSHA})
		}
		mode = "resumed_recovery"
	}

	// Journal the intent (state='assembling') BEFORE the git CAS (two-phase journal
	// Phase 1). Idempotent on (repository_id, barrier_id).
	if err := journalBarrierAssemblyIntent(ctx, runner, repositoryID, barrierID, fp.RunID,
		fp.DownstreamWorkflowJobID, assemblyWorkflowJobID, assemblyJobID,
		fp.FrozenTipSHA, commitSHA, treeSHA); err != nil {
		return barrierAssemblyResult{}, err
	}

	// Phase 2: CAS-advance the run branch to the journaled target commit. The CAS is
	// idempotent — if the run branch already points at (or descends from) the target
	// (this assembler's OWN commit applied on a prior, crashed run), advanceRunBranchToAssembled
	// recognizes it and does NOT double-commit.
	if err := advanceRunBranchToAssembled(ctx, repoRoot, runRef, fp.FrozenTipSHA, commitSHA, barrierID); err != nil {
		return barrierAssemblyResult{}, err
	}

	// Phase 2 finalize: flip the journal to 'committed'.
	if err := markBarrierAssemblyCommitted(ctx, runner, repositoryID, barrierID); err != nil {
		return barrierAssemblyResult{}, err
	}

	return barrierAssemblyResult{
		CommitSHA: commitSHA,
		TreeSHA:   treeSHA,
		Edges:     edges,
		Mode:      mode,
	}, nil
}

// advanceRunBranchToAssembled CAS-advances the run branch ref to the assembled
// commit. It is idempotent for crash recovery: if the run branch already points at
// the target (or a descendant of it — a concurrent integration already applied
// this assembler's own commit), it returns without re-committing. Otherwise it
// compare-and-swaps the ref from the expected tip to the target.
//
// The expected tip is resolved live (the frozen base if the ref is absent, else
// the current ref); the CAS guards against a concurrent mutation between the
// read and the swap. A genuine race that loses the CAS re-reads and retries.
func advanceRunBranchToAssembled(ctx context.Context, repoRoot, runRef, frozenBase, target, barrierID string) error {
	const maxAttempts = 6
	for i := 0; i < maxAttempts; i++ {
		tip, err := gitRevParseCommit(ctx, repoRoot, runRef)
		if err != nil {
			// The run ref may not exist yet (a fresh run branch); seed the CAS from
			// the frozen base.
			tip = frozenBase
		}
		// Idempotent recognition: the run branch is already at / descended from the
		// assembled commit — this assembler's own work (possibly from a crashed
		// prior run) already landed. Do not double-commit.
		if tip == target {
			return nil
		}
		if ok, ancErr := gitIsAncestor(ctx, repoRoot, target, tip); ancErr == nil && ok {
			return nil
		}
		// Compare-and-swap the ref from the observed tip to the assembled commit. The
		// assembled commit descends from the frozen base, and the per-run advisory
		// lock (held by the caller) serializes fire, so under normal operation the run
		// branch is at the frozen base / a prior assembly.
		_, exit, err := integrateGit(ctx, repoRoot, "update-ref", runRef, target, tip)
		if err != nil {
			return err
		}
		if exit == 0 {
			return nil
		}
		// Lost the CAS: re-read and retry. If the ref did not move, fail loudly.
		refreshed, rErr := gitRevParseCommit(ctx, repoRoot, runRef)
		if rErr != nil {
			return rErr
		}
		if refreshed == tip {
			return rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
				"barrier %q assembly compare-and-swap failed advancing the run branch %q to %s without movement",
				barrierID, runRef, target),
				map[string]any{"barrier_id": barrierID, "run_ref": runRef, "target": target})
		}
	}
	return rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
		"barrier %q assembly exhausted %d compare-and-swap retries advancing the run branch %q to %s",
		barrierID, maxAttempts, runRef, target),
		map[string]any{"barrier_id": barrierID, "run_ref": runRef, "target": target})
}
