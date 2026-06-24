package db

import (
	"context"
	"fmt"
	"strconv"
)

// Deployer is the one-shot `striatum daemon deploy` engine (Q4: a thin verb over
// this core, so a future run step can invoke Apply unchanged). It applies every
// deploy-plan step over the SINGLE owner connection; the only write routed over a
// separate runtime view is the C1 finalizer's terminal schema_state self-record.
type Deployer struct {
	DaemonVersion string
	// DryRun reports the plan without applying any step or mutating the cursor.
	DryRun bool
	// Abort marks an in-flight cursor `aborted` instead of resuming.
	Abort bool
}

// DeployResult is the outcome of Deployer.Apply.
type DeployResult struct {
	PlanHash       string
	AppliedIndices []int
	Resumed        bool
	State          string
	StepsTotal     int
	DryRun         bool
}

// Apply runs (or resumes, or aborts, or dry-runs) the deploy over runner (the
// owner/admin connection). It is idempotent: a `complete`/in-sync cursor is a
// no-op, a crash mid-deploy resumes off the STORED transcript (BC-N1), and every
// resume re-verifies the full transcript bytes + already-applied DB stamps (M1)
// before applying anything.
func (d *Deployer) Apply(ctx context.Context, runner Runner) (*DeployResult, error) {
	if err := ensureDeploySubstrate(ctx, runner); err != nil {
		return nil, fmt.Errorf("ensure deploy substrate (0044): %w", err)
	}
	cursor, err := LoadDeployCursor(ctx, runner)
	if err != nil {
		return nil, err
	}

	switch cursor.State {
	case DeployStateComplete:
		// Idempotent no-op: a completed deploy re-run verifies the stored
		// transcript and returns. (A genuine pending change would have produced a
		// different plan_hash and a fresh materialization below, not a complete
		// cursor for that plan.)
		stored, err := LoadStoredPlan(ctx, runner, cursor.PlanHash)
		if err != nil {
			return nil, err
		}
		if stored != nil {
			if err := VerifyStoredTranscript(stored); err != nil {
				return nil, err
			}
		}
		return &DeployResult{PlanHash: cursor.PlanHash, State: DeployStateComplete, StepsTotal: planLen(stored), DryRun: d.DryRun}, nil

	case DeployStateInProgress, DeployStateStepCommitted, DeployStateFinalizing:
		return d.resume(ctx, runner, cursor)

	default: // none / aborted → materialize a fresh plan
		return d.materializeAndRun(ctx, runner)
	}
}

func planLen(p *DeployPlan) int {
	if p == nil {
		return 0
	}
	return len(p.Steps)
}

func (d *Deployer) materializeAndRun(ctx context.Context, runner Runner) (*DeployResult, error) {
	baseOwner, err := OwnerBundleVersion(ctx, runner)
	if err != nil {
		return nil, err
	}
	baseRuntime, err := ReadSchemaVersion(ctx, runner)
	if err != nil {
		return nil, err
	}
	plan, err := BuildPlan(baseOwner, baseRuntime)
	if err != nil {
		return nil, err
	}
	if d.DryRun {
		return &DeployResult{PlanHash: plan.PlanHash, State: DeployStateNone, StepsTotal: len(plan.Steps), DryRun: true}, nil
	}
	if err := materializePlan(ctx, runner, plan); err != nil {
		return nil, err
	}
	return d.runFrom(ctx, runner, plan, 0, false)
}

func (d *Deployer) resume(ctx context.Context, runner Runner, cursor DeployCursor) (*DeployResult, error) {
	stored, err := LoadStoredPlan(ctx, runner, cursor.PlanHash)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		// BC-N1 / M1: an in-flight cursor whose immutable transcript is absent is a
		// binary mismatch — apply NOTHING.
		return nil, &DeployPlanBinaryMismatchError{StepIndex: cursor.StepIndex, StepID: cursor.StepID, Stored: "", Binary: ""}
	}
	if d.Abort {
		if err := writeCursor(ctx, runner, cursor.PlanHash, DeployStateAborted, cursor.StepIndex, cursor.StepID); err != nil {
			return nil, err
		}
		return &DeployResult{PlanHash: cursor.PlanHash, State: DeployStateAborted, StepsTotal: len(stored.Steps), Resumed: true}, nil
	}
	// M1: full-transcript byte verification + already-applied DB-stamp check
	// BEFORE applying or finalizing any step.
	if err := VerifyStoredTranscript(stored); err != nil {
		return nil, err
	}
	committed := committedThrough(cursor)
	if err := VerifyAppliedDBStamps(ctx, runner, stored, committed); err != nil {
		return nil, err
	}
	if cursor.State == DeployStateFinalizing {
		if err := d.finalize(ctx, runner, stored); err != nil {
			return nil, err
		}
		return &DeployResult{PlanHash: stored.PlanHash, State: DeployStateComplete, StepsTotal: len(stored.Steps), Resumed: true}, nil
	}
	return d.runFrom(ctx, runner, stored, committed, true)
}

// committedThrough returns the count of fully-committed steps for a cursor:
// step_committed(k) ⇒ k+1 committed; in_progress(k) ⇒ k committed (step k is
// re-run idempotently on resume).
func committedThrough(cursor DeployCursor) int {
	if cursor.State == DeployStateStepCommitted {
		return cursor.StepIndex + 1
	}
	return cursor.StepIndex
}

func (d *Deployer) runFrom(ctx context.Context, runner Runner, plan *DeployPlan, next int, resumed bool) (*DeployResult, error) {
	prevHash, err := lastReceiptHash(ctx, runner, plan.PlanHash, next)
	if err != nil {
		return nil, err
	}
	var applied []int
	for i := next; i < len(plan.Steps); i++ {
		step := plan.Steps[i]
		rowHash, err := d.applyDeployStep(ctx, runner, plan, step, prevHash)
		if err != nil {
			return nil, fmt.Errorf("apply deploy step %d (%s): %w", step.StepIndex, step.StepID, err)
		}
		prevHash = rowHash
		applied = append(applied, step.StepIndex)
	}
	if err := d.finalize(ctx, runner, plan); err != nil {
		return nil, err
	}
	return &DeployResult{PlanHash: plan.PlanHash, AppliedIndices: applied, Resumed: resumed, State: DeployStateComplete, StepsTotal: len(plan.Steps)}, nil
}

// applyDeployStep is the Q3-A transactional protocol: the step DDL + its version
// stamp + the cursor advance (→ step_committed(k)) + the hash-chained per-step
// receipt all commit in ONE transaction, so a committed step always carries its
// stamp and receipt (exactly-once), and a crash leaves the cursor at the last
// fully-committed step (resume re-runs the next idempotently). C3 NOTE: the
// per-step owner-oid snapshot + `ALTER … OWNER TO striatumd_rw` reconcile (§3.3b),
// which keeps a runtime step's newly-created objects striatumd_rw-owned before the
// terminal revoke commits, is NOT yet wired here. It is latent for the normal
// base-20 activation (the only deployed step is the terminal 0021, which creates no
// new ownable object) and is the documented apply-run follow-up (DRAFT.md "Known
// remaining items" #1).
func (d *Deployer) applyDeployStep(ctx context.Context, runner Runner, plan *DeployPlan, step DeployStep, prevHash string) (string, error) {
	sql, label, version, err := stepRecord(step)
	if err != nil {
		return "", err
	}
	tx, err := runner.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()

	if err := tx.Exec(ctx, sql); err != nil {
		return "", err
	}
	switch step.Role {
	case DeployRoleRuntime:
		if err := tx.Exec(ctx,
			`INSERT INTO striatumd.schema_migrations(version, label, sha256, daemon_version)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (version) DO NOTHING`,
			version, label, step.SHA256, d.daemonVersion()); err != nil {
			return "", err
		}
		if err := tx.Exec(ctx,
			`INSERT INTO striatumd.schema_meta(key, value) VALUES ('substrate_version', $1)
			 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
			strconv.Itoa(version)); err != nil {
			return "", err
		}
	case DeployRoleOwner:
		if err := tx.Exec(ctx,
			`INSERT INTO striatumd.owner_bundle_meta(version, label, sha256, daemon_version)
			 VALUES ($1, $2, $3, $4) ON CONFLICT (version) DO NOTHING`,
			version, label, step.SHA256, d.daemonVersion()); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown deploy step role %q", step.Role)
	}

	rowHash := receiptRowHash(prevHash, plan.PlanHash, step)
	if err := tx.Exec(ctx,
		`INSERT INTO striatumd.deploy_cursor(id, plan_hash, state, step_index, step_id, updated_at)
		 VALUES ('singleton', $1, $2, $3, $4, now())
		 ON CONFLICT (id) DO UPDATE
		   SET plan_hash = EXCLUDED.plan_hash, state = EXCLUDED.state,
		       step_index = EXCLUDED.step_index, step_id = EXCLUDED.step_id, updated_at = now()`,
		plan.PlanHash, DeployStateStepCommitted, step.StepIndex, step.StepID); err != nil {
		return "", err
	}
	if err := tx.Exec(ctx,
		`INSERT INTO striatumd.deploy_receipt(plan_hash, step_index, step_id, sha256, prev_hash, row_hash, daemon_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (plan_hash, step_index) DO NOTHING`,
		plan.PlanHash, step.StepIndex, step.StepID, step.SHA256, prevHash, rowHash, d.daemonVersion()); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	committed = true
	return rowHash, nil
}

// finalize is the idempotent C1 finalizer: (0) re-verify the full transcript (M1)
// and abort on mismatch writing NOTHING; (1) advance to finalizing; (2)
// RecordSchemaFingerprint; (3) advance finalizing → complete LAST.
func (d *Deployer) finalize(ctx context.Context, runner Runner, plan *DeployPlan) error {
	// (0) M1 pre-finalizer gate.
	if err := VerifyStoredTranscript(plan); err != nil {
		return err
	}
	stepID := ""
	stepIndex := 0
	if n := len(plan.Steps); n > 0 {
		stepID = plan.Steps[n-1].StepID
		stepIndex = plan.Steps[n-1].StepIndex
	}
	if err := writeCursor(ctx, runner, plan.PlanHash, DeployStateFinalizing, stepIndex, stepID); err != nil {
		return err
	}
	// (2) self-record the fingerprint (the C1 writer of schema_state on the deploy
	// path). gated by the (0) verification above.
	if err := RecordSchemaFingerprint(ctx, runner, d.daemonVersion()); err != nil {
		return err
	}
	// (3) advance to complete LAST.
	return writeCursor(ctx, runner, plan.PlanHash, DeployStateComplete, stepIndex, stepID)
}

func (d *Deployer) daemonVersion() string {
	if d.DaemonVersion == "" {
		return "dev"
	}
	return d.DaemonVersion
}

// materializePlan inserts the immutable transcript and sets the cursor to
// in_progress(0) — both in ONE transaction, BEFORE step 0 (BC-N1). The
// deploy_plan INSERT is ON CONFLICT (plan_hash) DO NOTHING (INSERT-once).
func materializePlan(ctx context.Context, runner Runner, plan *DeployPlan) error {
	stepsJSON, err := marshalSteps(plan.Steps)
	if err != nil {
		return err
	}
	tx, err := runner.BeginTx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(context.Background())
		}
	}()
	if err := tx.Exec(ctx,
		`INSERT INTO striatumd.deploy_plan(plan_hash, steps, revoke_step_index,
		   base_owner_version, base_runtime_version, target_owner_version, target_runtime_version)
		 VALUES ($1, $2::jsonb, $3, $4, $5, $6, $7)
		 ON CONFLICT (plan_hash) DO NOTHING`,
		plan.PlanHash, stepsJSON, plan.RevokeStepIndex,
		plan.BaseOwnerVersion, plan.BaseRuntimeVersion, plan.TargetOwnerVersion, plan.TargetRuntimeVersion); err != nil {
		return err
	}
	if err := tx.Exec(ctx,
		`INSERT INTO striatumd.deploy_cursor(id, plan_hash, state, step_index, step_id, updated_at)
		 VALUES ('singleton', $1, $2, 0, '', now())
		 ON CONFLICT (id) DO UPDATE
		   SET plan_hash = EXCLUDED.plan_hash, state = EXCLUDED.state,
		       step_index = 0, step_id = '', updated_at = now()`,
		plan.PlanHash, DeployStateInProgress); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func writeCursor(ctx context.Context, runner Runner, planHash, state string, stepIndex int, stepID string) error {
	return runner.Exec(ctx,
		`INSERT INTO striatumd.deploy_cursor(id, plan_hash, state, step_index, step_id, updated_at)
		 VALUES ('singleton', $1, $2, $3, $4, now())
		 ON CONFLICT (id) DO UPDATE
		   SET plan_hash = EXCLUDED.plan_hash, state = EXCLUDED.state,
		       step_index = EXCLUDED.step_index, step_id = EXCLUDED.step_id, updated_at = now()`,
		planHash, state, stepIndex, stepID)
}

func lastReceiptHash(ctx context.Context, runner Runner, planHash string, before int) (string, error) {
	if before <= 0 {
		return "", nil
	}
	return runner.QueryScalar(ctx,
		`SELECT COALESCE((SELECT row_hash FROM striatumd.deploy_receipt
		   WHERE plan_hash = $1 AND step_index = $2), '')`, planHash, before-1)
}

// stepRecord resolves a stored step back to the binary's SQL, label, and version
// for application + stamping.
func stepRecord(step DeployStep) (sql, label string, version int, err error) {
	switch step.Role {
	case DeployRoleRuntime:
		migrations, mErr := Migrations()
		if mErr != nil {
			return "", "", 0, mErr
		}
		for _, m := range migrations {
			if baseName(m.Path) == step.StepID {
				return m.SQL, m.Label, m.Version, nil
			}
		}
		return "", "", 0, fmt.Errorf("runtime migration %q not embedded in this binary", step.StepID)
	case DeployRoleOwner:
		bundles, bErr := OwnerBundles()
		if bErr != nil {
			return "", "", 0, bErr
		}
		for _, b := range bundles {
			if baseName(b.Path) == step.StepID {
				return b.SQL, b.Label, b.Version, nil
			}
		}
		return "", "", 0, fmt.Errorf("owner bundle %q not embedded in this binary", step.StepID)
	default:
		return "", "", 0, fmt.Errorf("unknown deploy step role %q", step.Role)
	}
}
