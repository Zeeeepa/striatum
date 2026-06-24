package reads

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
)

// deployUnrecordedDoctorBlock surfaces the RFC 0142 P4 deploy substrate as a
// read-only doctor block: the live deploy_cursor state, and — per the immutable
// deploy_plan transcript — any step that the cursor records as committed but the
// hash-chained deploy_receipt trail does NOT carry (`schema_deploy_unrecorded`),
// plus the M1 stamp/byte WARN: an already-applied runtime step whose stored
// transcript sha256 diverges from the live schema_migrations stamp.
//
// It is transcript-enumerated (per-step), reported as a WARNING (never a hard
// problem), matching the shadow-first contract of the sibling schema_drift block.
// Daemon-global; reads no secret; never fails closed — a read error is captured
// in the block and returned as a warning so doctor itself still returns. It skips
// cleanly (status=no_deploy) when no deploy is in flight or the 0044 substrate is
// absent on a pre-P4 database.
func deployUnrecordedDoctorBlock(ctx context.Context, runner db.Runner) (map[string]any, []string) {
	block := map[string]any{"checked": false}
	if runner == nil {
		block["skipped"] = "no runner"
		return block, nil
	}
	cursor, err := db.LoadDeployCursor(ctx, runner)
	if err != nil {
		block["checked"] = true
		block["error"] = err.Error()
		return block, []string{"schema_deploy.read_failed: " + err.Error()}
	}
	block["checked"] = true
	block["cursor_state"] = cursor.State
	if cursor.State == db.DeployStateNone {
		block["status"] = "no_deploy"
		return block, nil
	}
	block["plan_hash"] = cursor.PlanHash
	block["cursor_step_index"] = cursor.StepIndex

	stored, err := db.LoadStoredPlan(ctx, runner, cursor.PlanHash)
	if err != nil {
		block["error"] = err.Error()
		return block, []string{"schema_deploy.plan_read_failed: " + err.Error()}
	}
	if stored == nil {
		// An in-flight cursor whose immutable transcript is absent is itself the
		// BC-N1/M1 anomaly the deployer refuses on; surface it.
		block["status"] = "transcript_missing"
		return block, []string{fmt.Sprintf(
			"schema_deploy_unrecorded: deploy_cursor references plan_hash %s but no immutable deploy_plan transcript is recorded (BC-N1)", cursor.PlanHash)}
	}
	block["steps_total"] = len(stored.Steps)

	var warnings []string
	committed := committedThrough(cursor)
	unrecorded := []map[string]any{}
	for _, step := range stored.Steps {
		if step.StepIndex >= committed {
			continue // not yet committed; nothing to record yet
		}
		hasReceipt, err := deployStepHasReceipt(ctx, runner, cursor.PlanHash, step.StepIndex)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("schema_deploy.receipt_read_failed: step %d: %v", step.StepIndex, err))
			continue
		}
		if !hasReceipt {
			unrecorded = append(unrecorded, map[string]any{"step_index": step.StepIndex, "step_id": step.StepID})
			warnings = append(warnings, fmt.Sprintf(
				"schema_deploy_unrecorded: committed deploy step %d (%s) has no hash-chained receipt", step.StepIndex, step.StepID))
		}
	}
	block["unrecorded_steps"] = unrecorded

	// M1 stamp/byte WARN: an already-applied runtime step whose live DB stamp
	// diverges from the stored transcript sha256.
	if err := db.VerifyAppliedDBStamps(ctx, runner, stored, committed); err != nil {
		block["stamp_mismatch"] = err.Error()
		warnings = append(warnings, "schema_deploy_stamp_mismatch: "+err.Error())
	}

	if len(warnings) == 0 {
		block["status"] = "ok"
	} else {
		block["status"] = "unrecorded"
	}
	return block, warnings
}

// committedThrough mirrors db.committedThrough for the doctor's reporting (the db
// helper is unexported): step_committed(k) ⇒ k+1 committed; otherwise k.
func committedThrough(cursor db.DeployCursor) int {
	if cursor.State == db.DeployStateStepCommitted {
		return cursor.StepIndex + 1
	}
	return cursor.StepIndex
}

func deployStepHasReceipt(ctx context.Context, runner db.Runner, planHash string, stepIndex int) (bool, error) {
	value, err := runner.QueryScalar(ctx,
		`SELECT (EXISTS (SELECT 1 FROM striatumd.deploy_receipt WHERE plan_hash = $1 AND step_index = $2))::text`,
		planHash, stepIndex)
	if err != nil {
		return false, err
	}
	return value == "true", nil
}
