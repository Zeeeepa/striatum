package mutations

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// RFC 0135 P1 (D216) — the FIRST LIVE instance of the sealed expectation barrier:
// fan-in with entity=job, seal=attempt (#345). This is RFC 0133's Slice 2 barrier,
// lifted to consume the entity/seal-generic predicate P0 minted in
// db.BarrierReadySQL.
//
// CUTOVER DISCIPLINE (RFC 0135 / RFC 0133 "Migration and rollout"): this is an
// OPT-IN, shadow mechanism. The shipped D206 per-completion run-branch merge
// (fanInIntegrateRunBranch in worktree.go) stays the DEFAULT path. The barrier and
// the per-completion path produce the SAME final tree for any completion order —
// proven by TestFaninBarrierSameFinalTreeAsPerCompletion (the same-final-tree
// equivalence fixture) — before any workflow flips to the barrier. P2 (assembly +
// the barrier_assembly job type) and the default-flip come later; nothing here
// wires the barrier into the live completion path or changes the default.
//
// The trap-killer property (RFC 0133 synthesis trap #1, generalized to seal):
// readiness JOINs each declared in-edge's staged contribution against the seat's
// LIVE attempt (staged.attempt = jobs.attempt). A requeue/resume/complete-stalled
// bumps jobs.attempt to a strictly-new value, so a stale-attempt staging row is
// STRUCTURALLY invisible to the barrier — never counted, never filtered after the
// fact. There is no COUNT(*) of staged refs per seat.

const (
	// faninBarrierEntityKind is the entity discriminator for the fan-in instance.
	faninBarrierEntityKind = "job"
	// faninBarrierSealColumn is the seal column for the fan-in instance: the live
	// attempt. (For the revision-coherence caller, P5, the seal is
	// review_generation; the predicate is identical in shape.)
	faninBarrierSealColumn = "attempt"

	// stagedRefPrefix is the attempt-addressed staging-ref namespace
	// refs/striatum/staged/<run>/<job>/<attempt> (RFC 0133). The `staged/` verb
	// prefix keeps these OUT of the integrate sweep's refs/striatum/<run>/ glob, and
	// the attempt suffix makes a stale attempt's ref structurally distinct.
	stagedRefPrefix = "refs/striatum/staged/"
	// voidedRefPrefix is the requeue-tombstone namespace
	// refs/striatum/voided/<run>/<job>/<attempt> (RFC 0133): when a seat is
	// requeued, its prior staging ref is renamed here so recovery can EXPLAIN why a
	// ref was excluded, rather than the ref silently vanishing.
	voidedRefPrefix = "refs/striatum/voided/"
)

// stagingRef builds the attempt-addressed staging ref for a sibling completion.
func stagingRef(runID, workflowJobID string, attempt int) string {
	if attempt < 1 {
		attempt = 1
	}
	return fmt.Sprintf("%s%s/%s/%d", stagedRefPrefix, runID, workflowJobID, attempt)
}

// voidedRef builds the requeue-tombstone ref for a superseded attempt.
func voidedRef(runID, workflowJobID string, attempt int) string {
	if attempt < 1 {
		attempt = 1
	}
	return fmt.Sprintf("%s%s/%s/%d", voidedRefPrefix, runID, workflowJobID, attempt)
}

// faninBarrierReadinessSQL wraps the entity/seal-generic predicate P0 minted in
// db.BarrierReadySQL (entity=job, seal=attempt) with the three relations it JOINs,
// derived in-query from the real freeze record + staging table + striatumd.jobs.
// It is built ONCE here so the fan-in caller cannot drift from the canonical shape;
// the static guard TestBarrierPredicateHasNoRefCount scans this file and fails the
// build on any bare ref-COUNT(*) / recency-latest barrier shape.
//
// The three relations the predicate expects (all keyed on (entity_kind, entity_id)):
//
//   - barrier_in_edges  edge   — one row per DECLARED sibling seat, carrying
//     is_terminal_gap (TRUE when the seat is quarantined — RFC 0133
//     quarantine-as-terminal-in-edge, so a quarantined sibling does not deadlock
//     the barrier forever). entity_id is the stable workflow_job_id.
//   - entity_live_seal  live   — each seat's LIVE attempt (MAX(attempt) over the
//     COMPLETED job rows for that workflow_job_id). This is the load-bearing
//     relation: the entity's live seal, NOT a per-edge count.
//   - staged_contrib    staged — each seat's live staging row (the highest-attempt
//     non-voided staged contribution). A `recovery/`-prefixed staging ref is
//     EXCLUDED so a complete-stalled recovery ref cannot silently enter the
//     canonical set (RFC 0133). A `voided` (tombstoned) row is excluded too.
//
// The barrier id binds as $1 and the entity kind as $2 (the predicate's contract);
// $3 binds the repository id, scoping every CTE.
func faninBarrierReadinessSQL() (string, error) {
	predicate, err := db.BarrierReadySQL(db.BarrierSpec{
		EntityKind:         faninBarrierEntityKind,
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         faninBarrierSealColumn,
	})
	if err != nil {
		return "", err
	}
	// The CTEs supply the predicate's three relations. The freeze record's
	// declared_sibling_job_ids array expands into one in-edge per seat; the live
	// seal is MAX(attempt) over the seat's COMPLETED jobs; the staged contribution
	// is the seat's highest-attempt non-voided, non-recovery-prefixed staging row.
	// Note: a quarantined seat's is_terminal_gap is sourced from striatumd.jobs
	// state = 'canceled' WITH a recorded quarantine (RFC 0133 / #311 P0); a seat
	// with no completed job and no quarantine is neither live-sealed (live row
	// absent) nor terminal, so the INNER JOIN on entity_live_seal drops it and the
	// bool_and over the remaining edges cannot be TRUE while it is outstanding.
	return `WITH frozen AS (
    SELECT barrier_id, run_id, declared_sibling_job_ids
      FROM striatumd.fanin_freeze_points
     WHERE repository_id = $3 AND barrier_id = $1
), declared AS (
    SELECT '` + faninBarrierEntityKind + `'::text AS entity_kind,
           sib.value::text AS entity_id,
           frozen.run_id  AS run_id
      FROM frozen,
           jsonb_array_elements_text(frozen.declared_sibling_job_ids) AS sib(value)
), barrier_in_edges AS (
    SELECT d.entity_kind,
           d.entity_id,
           $1::text AS barrier_id,
           -- A quarantined seat (canceled with a quarantine marker) is a
           -- terminal-acceptable gap: the barrier fires WITH a recorded gap in the
           -- manifest rather than deadlocking. Resolved against the live job rows.
           COALESCE(bool_or(j.state = 'canceled' AND COALESCE(j.write_scope_json->>'quarantined','') = 'true'), false) AS is_terminal_gap
      FROM declared d
      LEFT JOIN striatumd.jobs j
        ON j.repository_id = $3 AND j.run_id = d.run_id AND j.workflow_job_id = d.entity_id
     GROUP BY d.entity_kind, d.entity_id
), live_completed AS (
    -- The live seal (MAX completed attempt) per seat, only for seats with a
    -- completed job. A seat with no completed job is absent here.
    SELECT j.workflow_job_id AS entity_id,
           MAX(j.attempt) AS attempt
      FROM striatumd.jobs j
      JOIN frozen ON j.repository_id = $3 AND j.run_id = frozen.run_id
     WHERE j.state = 'completed'
       AND j.workflow_job_id IN (SELECT entity_id FROM declared)
     GROUP BY j.workflow_job_id
), entity_live_seal AS (
    -- One row PER DECLARED in-edge so the predicate's INNER JOIN never DROPS an
    -- uncompleted seat (which would make bool_and ignore it and fire prematurely).
    -- An uncompleted seat gets live seal 0; attempts are >=1 and the staged
    -- sentinel is -1, so live(0) never equals staged(-1) — the edge is FALSE, and a
    -- live but unstaged completed seat (live=N, staged=-1) is FALSE too. The barrier
    -- fires only when every seat is staged at its live completed attempt.
    SELECT d.entity_kind,
           d.entity_id,
           COALESCE(lc.attempt, 0) AS attempt
      FROM declared d
      LEFT JOIN live_completed lc ON lc.entity_id = d.entity_id
), live_staged AS (
    -- The highest-attempt non-voided, non-recovery staging row per seat.
    SELECT s.workflow_job_id AS entity_id,
           MAX(s.attempt) AS attempt
      FROM striatumd.barrier_staged_contributions s
     WHERE s.repository_id = $3 AND s.barrier_id = $1
       AND s.status = 'staged'
       -- A complete-stalled recovery ref cannot silently enter the canonical set.
       AND s.staging_ref NOT LIKE 'refs/striatum/recovery/%'
     GROUP BY s.workflow_job_id
), staged_contrib AS (
    -- One row PER DECLARED in-edge so the predicate's LEFT JOIN always matches and
    -- an UNSTAGED seat yields a non-NULL, never-equal sentinel seal (-1) — the
    -- disjunct staged.attempt = live.attempt then evaluates to FALSE, not NULL, so
    -- bool_and FAILS CLOSED on an outstanding seat (a NULL would be silently
    -- skipped by bool_and and let an incomplete barrier fire).
    SELECT d.entity_kind,
           d.entity_id,
           COALESCE(ls.attempt, -1) AS attempt
      FROM declared d
      LEFT JOIN live_staged ls ON ls.entity_id = d.entity_id
)
` + predicate, nil
}

// faninBarrierReady evaluates the live-seal JOIN barrier for one fan-in barrier
// under the per-run advisory lock the caller already holds (RFC 0104 — the
// advisory-lock fire serialization). It returns whether the barrier is ready to
// fire (every declared in-edge is live-staged or a terminal gap), the per-edge
// classification used to build the manifest, and the freeze record. A NULL
// bool_and (no declared in-edges resolvable yet) is treated as NOT ready.
func faninBarrierReady(ctx context.Context, runner db.TxRunner, repositoryID, barrierID string) (bool, error) {
	query, err := faninBarrierReadinessSQL()
	if err != nil {
		return false, err
	}
	row := runner.QueryRow(ctx, query, barrierID, faninBarrierEntityKind, repositoryID)
	var ready *bool
	if err := row.Scan(&ready); err != nil {
		return false, fmt.Errorf("fan-in barrier readiness query: %w", err)
	}
	return ready != nil && *ready, nil
}

// faninFreezePoint is the immutable freeze record written once at fan-out.
type faninFreezePoint struct {
	BarrierID               string
	RunID                   string
	DownstreamWorkflowJobID string
	FrozenTipSHA            string
	FrozenTipTreeSHA        string
	DeclaredSiblingJobIDs   []string
}

// recordFaninFreezePoint writes the immutable freeze record. It is append-only (the
// migration's SELECT/INSERT-only grant + refuse-trigger make a later UPDATE/DELETE
// impossible), so a re-fire that observes an existing freeze point treats it as the
// canonical base rather than re-pointing it. The frozen tip is the run-branch tip
// at fan-out; the assembly folds staged refs onto it.
func recordFaninFreezePoint(ctx context.Context, runner db.TxRunner, repositoryID string, fp faninFreezePoint) error {
	declared, err := json.Marshal(fp.DeclaredSiblingJobIDs)
	if err != nil {
		return fmt.Errorf("marshal declared sibling job ids: %w", err)
	}
	var tree any
	if strings.TrimSpace(fp.FrozenTipTreeSHA) != "" {
		tree = fp.FrozenTipTreeSHA
	}
	return runner.Exec(ctx, `
		INSERT INTO striatumd.fanin_freeze_points
		  (repository_id, barrier_id, run_id, downstream_workflow_job_id,
		   frozen_tip_sha, frozen_tip_tree_sha, declared_sibling_job_ids)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		ON CONFLICT (repository_id, barrier_id) DO NOTHING`,
		repositoryID, fp.BarrierID, fp.RunID, fp.DownstreamWorkflowJobID,
		fp.FrozenTipSHA, tree, string(declared))
}

// stageFaninContribution records a sibling's completion as an attempt-addressed
// staging contribution AND cuts the staging git ref, co-transactionally with the
// caller's attempt/state update (RFC 0133). The merge-base contamination check
// asserts the staged commit DESCENDS from the frozen tip — a staged ref descended
// from the EVOLVED branch (base drift) is refused before the row is written, so a
// smuggled-base contribution can never enter the canonical set. (Caveat inherited
// from RFC 0133: merge-base proves topological ancestry, not content provenance.)
//
// The PG row is the authoritative record the barrier JOINs against the live
// attempt; the git ref is the witness. The row is keyed on
// (repository_id, barrier_id, workflow_job_id, attempt) so a re-stage of the same
// attempt is idempotent and a new attempt is a distinct row.
func stageFaninContribution(ctx context.Context, runner db.TxRunner, repoRoot, repositoryID, barrierID, runID, workflowJobID, jobID, commitSHA string, attempt int) (string, error) {
	fp, err := loadFaninFreezePoint(ctx, runner, repositoryID, barrierID)
	if err != nil {
		return "", err
	}
	// Merge-base contamination check (RFC 0133): the staged commit must descend from
	// the frozen tip. git merge-base --is-ancestor <frozen> <staged> == ancestor.
	if ok, err := gitIsAncestor(ctx, repoRoot, fp.FrozenTipSHA, commitSHA); err != nil {
		return "", err
	} else if !ok {
		return "", rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
			"fan-in staging refused for seat %s (attempt %d): staged commit %s does not descend from the frozen base %s (base drift / contaminated base, RFC 0133)",
			workflowJobID, attempt, commitSHA, fp.FrozenTipSHA),
			map[string]any{"workflow_job_id": workflowJobID, "attempt": attempt, "frozen_tip": fp.FrozenTipSHA, "staged_commit": commitSHA})
	}
	ref := stagingRef(runID, workflowJobID, attempt)
	if _, exit, err := integrateGit(ctx, repoRoot, "update-ref", ref, commitSHA); err != nil {
		return "", err
	} else if exit != 0 {
		return "", rpc.NewError("git_commit_apply_failed", fmt.Sprintf("could not write fan-in staging ref %q for seat %s", ref, workflowJobID), nil)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.barrier_staged_contributions
		  (repository_id, barrier_id, run_id, workflow_job_id, attempt, job_id, staging_ref, commit_sha, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'staged')
		ON CONFLICT (repository_id, barrier_id, workflow_job_id, attempt)
		DO UPDATE SET job_id = EXCLUDED.job_id,
		              staging_ref = EXCLUDED.staging_ref,
		              commit_sha = EXCLUDED.commit_sha,
		              status = 'staged',
		              voided_at = NULL,
		              void_reason = NULL`,
		repositoryID, barrierID, runID, workflowJobID, attempt, jobID, ref, commitSHA); err != nil {
		return "", err
	}
	return ref, nil
}

// voidFaninContribution tombstones a seat's prior staging contributions when the
// seat is requeued (RFC 0133 requeue tombstone). It marks every non-voided staging
// row for the seat at an attempt BELOW the new attempt as 'voided' (so it can never
// satisfy staged.attempt = jobs.attempt even if a later attempt collides) and
// renames the staging ref into refs/striatum/voided/… so recovery can EXPLAIN the
// exclusion. The live-seal JOIN already makes a stale attempt structurally
// invisible; the tombstone makes the exclusion LEGIBLE rather than implicit.
func voidFaninContribution(ctx context.Context, runner db.TxRunner, repoRoot, repositoryID, barrierID, runID, workflowJobID string, newAttempt int, reason string) error {
	rows, err := queryRows(ctx, runner, `
		SELECT attempt, staging_ref
		  FROM striatumd.barrier_staged_contributions
		 WHERE repository_id = $1 AND barrier_id = $2 AND workflow_job_id = $3
		   AND status = 'staged' AND attempt < $4`,
		repositoryID, barrierID, workflowJobID, newAttempt)
	if err != nil {
		return err
	}
	for _, r := range rows {
		attempt := intValue(r["attempt"])
		stagedRef := strings.TrimSpace(fmt.Sprint(r["staging_ref"]))
		// Rename the staging ref into the voided namespace (best-effort witness).
		if stagedRef != "" {
			if sha, err := gitRevParseCommit(ctx, repoRoot, stagedRef); err == nil {
				vref := voidedRef(runID, workflowJobID, attempt)
				_, _, _ = integrateGit(ctx, repoRoot, "update-ref", vref, sha)
				_, _, _ = integrateGit(ctx, repoRoot, "update-ref", "-d", stagedRef)
			}
		}
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.barrier_staged_contributions
		   SET status = 'voided', voided_at = now(), void_reason = $5
		 WHERE repository_id = $1 AND barrier_id = $2 AND workflow_job_id = $3
		   AND status = 'staged' AND attempt < $4`,
		repositoryID, barrierID, workflowJobID, newAttempt, reason)
}

// assembleFaninBarrier folds every live-staged sibling contribution onto the
// frozen tip in canonical workflow_job_id order, producing ONE deterministic
// assembled tree — the deferred post-completion join (RFC 0133 Slice 3, the part
// P1 ships as the opt-in mechanism without yet promoting it to a barrier_assembly
// job type). It reuses the EXACT merge-tree/commit-tree plumbing the D206
// per-completion path uses (mergeTreeWriteTree → commit-tree), so the assembled
// tree is byte-identical to the per-completion result for disjoint write scopes —
// the property TestFaninBarrierSameFinalTreeAsPerCompletion asserts before any
// workflow flips to the barrier.
//
// It returns the assembled tree sha, the resulting commit sha, and the ordered
// in-edges it folded (for the manifest). It NEVER advances the run branch ref by
// itself; the caller (the opt-in barrier_assembly path, P2) does the CAS
// update-ref under the lock. P1 keeps the assembly OUT of the default completion
// path — the default stays D206 per-completion.
func assembleFaninBarrier(ctx context.Context, runner db.TxRunner, repoRoot, repositoryID, barrierID string) (treeSHA, commitSHA string, edges []faninManifestInEdge, err error) {
	fp, err := loadFaninFreezePoint(ctx, runner, repositoryID, barrierID)
	if err != nil {
		return "", "", nil, err
	}
	edges, err = faninBarrierManifest(ctx, runner, repositoryID, barrierID)
	if err != nil {
		return "", "", nil, err
	}
	// Fold each live-staged sibling commit onto the frozen tip, in canonical
	// workflow_job_id order (edges are already sorted by entity_id). Quarantined /
	// unstaged seats contribute no commit (their gap is recorded in the manifest).
	tip := fp.FrozenTipSHA
	tree, err := gitTreeOf(ctx, repoRoot, tip)
	if err != nil {
		return "", "", nil, err
	}
	for _, edge := range edges {
		if edge.Status != "staged_live" || edge.CommitSHA == "" {
			continue
		}
		// If the staged commit already includes the current tip, fast-forward.
		if ok, err := gitIsAncestor(ctx, repoRoot, tip, edge.CommitSHA); err != nil {
			return "", "", nil, err
		} else if ok {
			tip = edge.CommitSHA
			tree, err = gitTreeOf(ctx, repoRoot, tip)
			if err != nil {
				return "", "", nil, err
			}
			continue
		}
		stdout, stderr, exit, err := mergeTreeWriteTree(ctx, repoRoot, tip, edge.CommitSHA)
		if err != nil {
			return "", "", nil, err
		}
		if exit != 0 {
			conflicts := parseMergeTreeConflicts(stdout)
			real := filterRealFanInConflicts(ctx, repoRoot, tip, edge.CommitSHA, conflicts)
			if len(real) > 0 {
				return "", "", nil, rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
					"fan-in barrier assembly of seat %s conflicts in %d path(s): %s. Parallel fan-in lanes must use disjoint write scopes (RFC 0101).",
					edge.EntityID, len(real), strings.Join(real, ", ")),
					map[string]any{"conflicting_paths": real, "workflow_job_id": edge.EntityID})
			}
			detail := strings.TrimSpace(stdout + "\n" + stderr)
			return "", "", nil, rpc.NewError("git_commit_apply_failed", fmt.Sprintf(
				"fan-in barrier assembly merge-tree for seat %s exited %d with no real content conflict: %s", edge.EntityID, exit, detail), nil)
		}
		newTree := firstLine(stdout)
		if !isFullGitSHA(newTree) {
			return "", "", nil, rpc.NewError("git_commit_apply_failed", fmt.Sprintf("fan-in barrier assembly produced invalid tree oid %q for seat %s", newTree, edge.EntityID), nil)
		}
		msg := fmt.Sprintf("striatum fan-in barrier: assemble seat %s (attempt %d)", edge.EntityID, edge.Seal)
		cout, cexit, err := integrateGit(ctx, repoRoot,
			"-c", "user.name=striatum-fanin", "-c", "user.email=fanin@striatum.local",
			"commit-tree", newTree, "-p", tip, "-p", edge.CommitSHA, "-m", msg)
		if err != nil {
			return "", "", nil, err
		}
		if cexit != 0 {
			return "", "", nil, rpc.NewError("git_commit_apply_failed", fmt.Sprintf("fan-in barrier assembly commit-tree failed for seat %s: %s", edge.EntityID, strings.TrimSpace(cout)), nil)
		}
		tip = firstLine(cout)
		if !isFullGitSHA(tip) {
			return "", "", nil, rpc.NewError("git_commit_apply_failed", fmt.Sprintf("fan-in barrier assembly commit-tree produced invalid commit oid %q for seat %s", tip, edge.EntityID), nil)
		}
		tree = newTree
	}
	return tree, tip, edges, nil
}

// gitTreeOf returns the tree oid of a commit.
func gitTreeOf(ctx context.Context, repoRoot, commit string) (string, error) {
	out, exit, err := integrateGit(ctx, repoRoot, "rev-parse", "--verify", commit+"^{tree}")
	if err != nil {
		return "", err
	}
	if exit != 0 {
		return "", rpc.NewError("git_commit_apply_failed", fmt.Sprintf("could not resolve tree of %q", commit), nil)
	}
	tree := firstLine(out)
	if !isFullGitSHA(tree) {
		return "", rpc.NewError("git_commit_apply_failed", fmt.Sprintf("invalid tree oid %q for %q", tree, commit), nil)
	}
	return tree, nil
}

// loadFaninFreezePoint reads the immutable freeze record for a barrier.
func loadFaninFreezePoint(ctx context.Context, runner any, repositoryID, barrierID string) (faninFreezePoint, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT barrier_id, run_id, downstream_workflow_job_id,
		       frozen_tip_sha, COALESCE(frozen_tip_tree_sha,'') AS frozen_tip_tree_sha,
		       declared_sibling_job_ids
		  FROM striatumd.fanin_freeze_points
		 WHERE repository_id = $1 AND barrier_id = $2`,
		repositoryID, barrierID)
	if err != nil {
		return faninFreezePoint{}, err
	}
	if len(rows) == 0 {
		return faninFreezePoint{}, rpc.NewError("invalid_transition", fmt.Sprintf("no fan-in freeze point for barrier %q", barrierID), nil)
	}
	r := rows[0]
	declared := []string{}
	switch v := r["declared_sibling_job_ids"].(type) {
	case []any:
		for _, item := range v {
			declared = append(declared, fmt.Sprint(item))
		}
	case string:
		_ = json.Unmarshal([]byte(v), &declared)
	case []byte:
		_ = json.Unmarshal(v, &declared)
	}
	return faninFreezePoint{
		BarrierID:               fmt.Sprint(r["barrier_id"]),
		RunID:                   fmt.Sprint(r["run_id"]),
		DownstreamWorkflowJobID: fmt.Sprint(r["downstream_workflow_job_id"]),
		FrozenTipSHA:            fmt.Sprint(r["frozen_tip_sha"]),
		FrozenTipTreeSHA:        fmt.Sprint(r["frozen_tip_tree_sha"]),
		DeclaredSiblingJobIDs:   declared,
	}, nil
}

// faninManifestInEdge is one row of the join_manifest.v1 in_edges list (P0's
// contract): the SEALED contribution each declared seat joined at.
type faninManifestInEdge struct {
	EntityID   string `json:"entity_id"`
	Seal       int    `json:"seal"`
	Status     string `json:"status"` // staged_live | quarantined
	CommitSHA  string `json:"commit_sha,omitempty"`
	StagingRef string `json:"staging_ref,omitempty"`
	DamageCode string `json:"damage_code,omitempty"`
}

// faninBarrierManifest snapshots, under the held advisory lock, the per-seat join
// provenance the barrier fired on — exactly what the join_manifest.v1 artifact
// records. The seal each contribution joined at is the load-bearing field: it is
// the seat's LIVE attempt, so a superseded-seal contribution is provably absent.
func faninBarrierManifest(ctx context.Context, runner db.TxRunner, repositoryID, barrierID string) ([]faninManifestInEdge, error) {
	fp, err := loadFaninFreezePoint(ctx, runner, repositoryID, barrierID)
	if err != nil {
		return nil, err
	}
	edges := make([]faninManifestInEdge, 0, len(fp.DeclaredSiblingJobIDs))
	for _, seat := range fp.DeclaredSiblingJobIDs {
		edge, err := faninManifestEdgeForSeat(ctx, runner, repositoryID, barrierID, fp.RunID, seat)
		if err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].EntityID < edges[j].EntityID })
	return edges, nil
}

func faninManifestEdgeForSeat(ctx context.Context, runner db.TxRunner, repositoryID, barrierID, runID, seat string) (faninManifestInEdge, error) {
	// Quarantined (canceled + quarantine marker) seats are terminal gaps.
	jobRows, err := queryRows(ctx, runner, `
		SELECT state, COALESCE(write_scope_json->>'quarantined','') AS quarantined
		  FROM striatumd.jobs
		 WHERE repository_id = $1 AND run_id = $2 AND workflow_job_id = $3`,
		repositoryID, runID, seat)
	if err != nil {
		return faninManifestInEdge{}, err
	}
	quarantined := false
	for _, jr := range jobRows {
		if fmt.Sprint(jr["state"]) == "canceled" && fmt.Sprint(jr["quarantined"]) == "true" {
			quarantined = true
		}
	}
	if quarantined {
		return faninManifestInEdge{EntityID: seat, Seal: 0, Status: "quarantined", DamageCode: "seat_quarantined"}, nil
	}
	// The live-seal staged contribution: the highest-attempt non-voided,
	// non-recovery staging row, joined on the seat's live attempt.
	rows, err := queryRows(ctx, runner, `
		SELECT s.attempt, s.commit_sha, s.staging_ref
		  FROM striatumd.barrier_staged_contributions s
		  JOIN (
		        SELECT MAX(attempt) AS attempt
		          FROM striatumd.jobs
		         WHERE repository_id = $1 AND run_id = $2 AND workflow_job_id = $3
		           AND state = 'completed'
		       ) live ON live.attempt = s.attempt
		 WHERE s.repository_id = $1 AND s.barrier_id = $4 AND s.workflow_job_id = $3
		   AND s.status = 'staged'
		   AND s.staging_ref NOT LIKE 'refs/striatum/recovery/%'`,
		repositoryID, runID, seat, barrierID)
	if err != nil {
		return faninManifestInEdge{}, err
	}
	if len(rows) == 0 {
		// Outstanding seat (not live-staged, not quarantined). The manifest is only
		// snapshotted on a READY barrier, so this should not occur on a fired
		// barrier; record it honestly as an unsatisfied seat so a premature snapshot
		// is legible rather than silently dropping the seat.
		return faninManifestInEdge{EntityID: seat, Seal: 0, Status: "quarantined", DamageCode: "seat_not_yet_staged"}, nil
	}
	r := rows[0]
	return faninManifestInEdge{
		EntityID:   seat,
		Seal:       intValue(r["attempt"]),
		Status:     "staged_live",
		CommitSHA:  fmt.Sprint(r["commit_sha"]),
		StagingRef: fmt.Sprint(r["staging_ref"]),
	}, nil
}
