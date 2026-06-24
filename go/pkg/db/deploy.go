package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// RFC 0142 P4 — the one-shot `striatum daemon deploy` core. Schema mutation stops
// being an implicit side effect of the serving process's restart and becomes an
// explicit, ordered, resumable, provenance-tracked operation owned by THIS
// deployer. The serving daemon then holds zero create-DDL on the serving path
// (the decoupled boot in connection.go ConnectAndVerify), and a bad migration can
// never wedge the single writer on boot.
//
// Everything here is shadow-first inert until the operator opts in
// (STRIATUM_DEPLOY_DECOUPLED) and runs `striatum daemon deploy`; the legacy
// ConnectAndMigrate serve-boot path is unchanged when the flag is absent.

// Deploy cursor states (migration 0044 `deploy_cursor.state` CHECK). The
// lifecycle is idle → in_progress → step_committed → … → finalizing → complete,
// or → aborted on operator abort / fatal. `finalizing` is the DISTINCT
// finalization-boundary state the idempotent finalizer resumes into (C1).
const (
	DeployStateIdle          = "idle"
	DeployStateInProgress    = "in_progress"
	DeployStateStepCommitted = "step_committed"
	DeployStateFinalizing    = "finalizing"
	DeployStateComplete      = "complete"
	DeployStateAborted       = "aborted"
	// DeployStateNone is the synthesized cursor state for an absent deploy_cursor
	// row (or table) — NO deploy in flight. It is never written to the DB.
	DeployStateNone = "none"
)

// Deploy step roles.
const (
	DeployRoleRuntime = "runtime"
	DeployRoleOwner   = "owner"
)

// DeployStep is one ordered, content-addressed step in the immutable deploy
// transcript. StepIndex is stable by storage (BC-N1: resume reads it, never
// recomputes it). SHA256 is the embedded file's content hash — a resume that
// disagrees with it is forced to a typed mismatch (M1).
type DeployStep struct {
	StepIndex     int    `json:"step_index"`
	StepID        string `json:"step_id"`
	Role          string `json:"role"`
	SHA256        string `json:"sha256"`
	Transactional bool   `json:"transactional"`
}

// DeployPlan is the immutable ordered transcript materialized once before step 0
// (BC-N1) and persisted in deploy_plan keyed by PlanHash. RevokeStepIndex is the
// index of the terminal DDL-revoke step (0021), or -1 when the binary embeds no
// revoke.
type DeployPlan struct {
	PlanHash             string
	Steps                []DeployStep
	RevokeStepIndex      int
	BaseOwnerVersion     int
	BaseRuntimeVersion   int
	TargetOwnerVersion   int
	TargetRuntimeVersion int
}

// Typed deploy halts. All map to refuse-to-serve / abort-deploy with the DB left
// untouched; the boot path joins them to the non-restartable exit alongside
// AwaitingOwnerDDLError / SchemaDriftError.
var (
	// ErrAwaitingDeploy: a deploy is pending/incomplete (BC-N2), or a decoupled
	// binary over a `complete` transcript that is NOT in-sync (A3), or a no-revoke
	// binary on a revoke-applied DB (barrier b). DB untouched.
	ErrAwaitingDeploy = errors.New("awaiting_deploy")
	// ErrAwaitingDeployConfig: the binary ships the DDL-revoke (0021) but
	// STRIATUM_DEPLOY_DECOUPLED is OFF — for EVERY cursor state (M3 config gate).
	ErrAwaitingDeployConfig = errors.New("awaiting_deploy_config")
	// ErrDeployPlanBinaryMismatch: a stored transcript step's sha256 diverges from
	// the running binary's embedded bytes (M1). Apply NOTHING.
	ErrDeployPlanBinaryMismatch = errors.New("deploy_plan_binary_mismatch")
	// ErrDeployPlanDBStampMismatch: an already-applied step's DB stamp diverges
	// from the stored transcript (M1). Do NOT finalize; apply NOTHING.
	ErrDeployPlanDBStampMismatch = errors.New("deploy_plan_db_stamp_mismatch")
)

// AwaitingDeployError carries the cursor state that triggered the BC-N2 / A3
// refuse-to-serve halt and the remediation. DB untouched.
type AwaitingDeployError struct {
	CursorState string
	Reason      string
}

func (e *AwaitingDeployError) Error() string {
	reason := e.Reason
	if reason == "" {
		reason = "a deploy is pending or incomplete"
	}
	return fmt.Sprintf(
		"awaiting_deploy: %s (deploy_cursor state %q); run `striatum daemon deploy` to apply the pending change, "+
			"then restart. The database was left untouched (no runtime migration ran).",
		reason, e.CursorState)
}
func (e *AwaitingDeployError) Unwrap() error { return ErrAwaitingDeploy }

// AwaitingDeployConfigError is the M3 config gate: a revoke-embedding binary with
// the decoupling flag OFF. DB untouched.
type AwaitingDeployConfigError struct{}

func (e *AwaitingDeployConfigError) Error() string {
	return "awaiting_deploy_config: this binary ships the DDL-revoke (owner bundle " +
		strconv.Itoa(DDLRevokeOwnerBundleVersion) + ") and must run on the decoupled deploy path; " +
		"set STRIATUM_DEPLOY_DECOUPLED=1 to serve verify-only, or run `striatum daemon deploy` to apply a pending change. " +
		"The database was left untouched (no runtime migration ran)."
}
func (e *AwaitingDeployConfigError) Unwrap() error { return ErrAwaitingDeployConfig }

// DeployPlanBinaryMismatchError names the divergent step for the M1 full-
// transcript byte check (the running binary disagrees with a STORED step).
type DeployPlanBinaryMismatchError struct {
	StepIndex int
	StepID    string
	Stored    string
	Binary    string
}

func (e *DeployPlanBinaryMismatchError) Error() string {
	return fmt.Sprintf(
		"deploy_plan_binary_mismatch: stored transcript step %d (%s) sha256 %s differs from this binary's embedded bytes %s; "+
			"resume with the binary that authored the deploy. The database was left untouched (apply NOTHING).",
		e.StepIndex, e.StepID, short(e.Stored), short(e.Binary))
}
func (e *DeployPlanBinaryMismatchError) Unwrap() error { return ErrDeployPlanBinaryMismatch }

// DeployPlanDBStampMismatchError names the divergent already-applied step for the
// M1 DB-stamp check.
type DeployPlanDBStampMismatchError struct {
	StepIndex int
	StepID    string
	Stored    string
	DBStamp   string
}

func (e *DeployPlanDBStampMismatchError) Error() string {
	return fmt.Sprintf(
		"deploy_plan_db_stamp_mismatch: already-applied step %d (%s) DB stamp %s differs from the stored transcript sha256 %s; "+
			"do NOT finalize. The database was left untouched (apply NOTHING).",
		e.StepIndex, e.StepID, short(e.DBStamp), short(e.Stored))
}
func (e *DeployPlanDBStampMismatchError) Unwrap() error { return ErrDeployPlanDBStampMismatch }

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// BuildPlan assembles the immutable deploy transcript from the binary's embedded
// migration + owner-bundle sets, starting from the supplied applied watermarks.
// Ordering (C3): pending non-revoke owner bundles (version in (baseOwner, 20]) →
// pending runtime migrations (version > baseRuntime) → the terminal DDL-revoke
// bundle 0021 LAST, if embedded and not yet applied. It uses the FULL OwnerBundles
// loader (so 0021 is visible) but special-cases the revoke to terminal — 0021 is
// NEVER an interior step.
func BuildPlan(baseOwner, baseRuntime int) (*DeployPlan, error) {
	bundles, err := OwnerBundles()
	if err != nil {
		return nil, err
	}
	migrations, err := Migrations()
	if err != nil {
		return nil, err
	}

	plan := &DeployPlan{
		RevokeStepIndex:    -1,
		BaseOwnerVersion:   baseOwner,
		BaseRuntimeVersion: baseRuntime,
	}
	idx := 0
	var revokeBundle *OwnerBundle

	// Pending non-revoke owner bundles (the revoke is held for terminal placement).
	for i := range bundles {
		b := bundles[i]
		if !isNonRevokeBundle(b.Version) {
			if b.Version >= DDLRevokeOwnerBundleVersion {
				rb := b
				revokeBundle = &rb
			}
			continue
		}
		if b.Version <= baseOwner {
			continue
		}
		plan.Steps = append(plan.Steps, DeployStep{
			StepIndex:     idx,
			StepID:        filepath.Base(b.Path),
			Role:          DeployRoleOwner,
			SHA256:        b.SHA256(),
			Transactional: true,
		})
		idx++
	}

	// Pending runtime migrations.
	for _, m := range migrations {
		if m.Version <= baseRuntime {
			continue
		}
		plan.Steps = append(plan.Steps, DeployStep{
			StepIndex:     idx,
			StepID:        filepath.Base(m.Path),
			Role:          DeployRoleRuntime,
			SHA256:        m.SHA256(),
			Transactional: true,
		})
		idx++
	}

	// Terminal DDL-revoke (0021), LAST, only if embedded and not yet applied.
	if revokeBundle != nil && baseOwner < DDLRevokeOwnerBundleVersion {
		plan.Steps = append(plan.Steps, DeployStep{
			StepIndex:     idx,
			StepID:        filepath.Base(revokeBundle.Path),
			Role:          DeployRoleOwner,
			SHA256:        revokeBundle.SHA256(),
			Transactional: true,
		})
		plan.RevokeStepIndex = idx
	}

	plan.TargetOwnerVersion = maxEmbeddedOwnerVersion(bundles)
	plan.TargetRuntimeVersion = LatestDaemonDBVersion
	plan.PlanHash = computePlanHash(plan)
	return plan, nil
}

func maxEmbeddedOwnerVersion(bundles []OwnerBundle) int {
	max := 0
	for _, b := range bundles {
		if b.Version > max {
			max = b.Version
		}
	}
	return max
}

// computePlanHash is the content-addressed identity of the transcript: a sha256
// over a canonical, order-stable serialization of the base/target watermarks plus
// the ordered (index, role, step_id, sha256) lines. The revoke-last ordering
// changes plan_hash but not ExpectedFingerprint (which hashes the unordered set),
// exactly as §3.2 requires.
func computePlanHash(plan *DeployPlan) string {
	var b strings.Builder
	b.WriteString("striatum.deploy_plan.v1\n")
	fmt.Fprintf(&b, "base_owner=%d\n", plan.BaseOwnerVersion)
	fmt.Fprintf(&b, "base_runtime=%d\n", plan.BaseRuntimeVersion)
	fmt.Fprintf(&b, "target_owner=%d\n", plan.TargetOwnerVersion)
	fmt.Fprintf(&b, "target_runtime=%d\n", plan.TargetRuntimeVersion)
	b.WriteString("steps:\n")
	for _, s := range plan.Steps {
		fmt.Fprintf(&b, "%d:%s:%s:%s\n", s.StepIndex, s.Role, s.StepID, s.SHA256)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// binaryStepSHA returns the running binary's embedded content hash for a stored
// step, looked up by (role, step_id). A missing step_id is itself a binary
// divergence (the binary does not embed a file the stored transcript names).
func binaryStepSHA(step DeployStep) (string, bool, error) {
	switch step.Role {
	case DeployRoleRuntime:
		set, err := MigrationSHASet()
		if err != nil {
			return "", false, err
		}
		sha, ok := set[step.StepID]
		return sha, ok, nil
	case DeployRoleOwner:
		bundles, err := OwnerBundles()
		if err != nil {
			return "", false, err
		}
		for _, b := range bundles {
			if filepath.Base(b.Path) == step.StepID {
				return b.SHA256(), true, nil
			}
		}
		return "", false, nil
	default:
		return "", false, fmt.Errorf("unknown deploy step role %q", step.Role)
	}
}

// VerifyStoredTranscript is the M1 full-transcript byte verifier (PROPOSAL §3.4a).
// For EVERY step in the stored transcript it checks the stored sha256 against the
// running binary's embedded bytes; ANY divergence returns a typed
// DeployPlanBinaryMismatchError and the caller applies NOTHING. It is a PURE READ
// (no DB). Called on every resume before any apply and as finalizer step 0.
func VerifyStoredTranscript(stored *DeployPlan) error {
	for _, step := range stored.Steps {
		sha, ok, err := binaryStepSHA(step)
		if err != nil {
			return err
		}
		if !ok || sha != step.SHA256 {
			return &DeployPlanBinaryMismatchError{
				StepIndex: step.StepIndex,
				StepID:    step.StepID,
				Stored:    step.SHA256,
				Binary:    sha,
			}
		}
	}
	return nil
}

// VerifyAppliedDBStamps is the second half of M1: for every already-applied step
// (StepIndex < appliedThrough) the stored transcript sha256 must equal the DB
// stamp (schema_migrations.sha256 for runtime, owner_bundle_meta.sha256 for
// owner). A divergence returns a typed DeployPlanDBStampMismatchError; the caller
// must NOT finalize.
func VerifyAppliedDBStamps(ctx context.Context, runner Runner, stored *DeployPlan, appliedThrough int) error {
	for _, step := range stored.Steps {
		if step.StepIndex >= appliedThrough {
			continue
		}
		dbStamp, err := deployStepDBStamp(ctx, runner, step)
		if err != nil {
			return err
		}
		if dbStamp == "" {
			// Not stamped yet — the step is recorded as applied in the cursor but the
			// stamp is absent; treat as a stamp mismatch (something rewrote it).
			return &DeployPlanDBStampMismatchError{StepIndex: step.StepIndex, StepID: step.StepID, Stored: step.SHA256, DBStamp: ""}
		}
		if dbStamp != step.SHA256 {
			return &DeployPlanDBStampMismatchError{StepIndex: step.StepIndex, StepID: step.StepID, Stored: step.SHA256, DBStamp: dbStamp}
		}
	}
	return nil
}

func deployStepDBStamp(ctx context.Context, runner Runner, step DeployStep) (string, error) {
	version := versionFromStepID(step.StepID)
	switch step.Role {
	case DeployRoleRuntime:
		return runner.QueryScalar(ctx,
			"SELECT COALESCE((SELECT sha256 FROM striatumd.schema_migrations WHERE version = $1), '')", version)
	case DeployRoleOwner:
		return runner.QueryScalar(ctx,
			"SELECT COALESCE((SELECT sha256 FROM striatumd.owner_bundle_meta WHERE version = $1), '')", version)
	default:
		return "", fmt.Errorf("unknown deploy step role %q", step.Role)
	}
}

func versionFromStepID(stepID string) int {
	v, err := strconv.Atoi(strings.SplitN(stepID, "_", 2)[0])
	if err != nil {
		return -1
	}
	return v
}

// LoadStoredPlan reads the immutable transcript from deploy_plan by plan_hash. It
// returns (nil, nil) when no row exists (no deploy materialized).
func LoadStoredPlan(ctx context.Context, runner Runner, planHash string) (*DeployPlan, error) {
	stepsJSON, err := runner.QueryScalar(ctx,
		"SELECT COALESCE((SELECT steps::text FROM striatumd.deploy_plan WHERE plan_hash = $1), '')", planHash)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(stepsJSON) == "" {
		return nil, nil
	}
	var steps []DeployStep
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return nil, fmt.Errorf("decode stored deploy_plan steps: %w", err)
	}
	plan := &DeployPlan{PlanHash: planHash, Steps: steps, RevokeStepIndex: -1}
	if err := scanPlanScalars(ctx, runner, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func scanPlanScalars(ctx context.Context, runner Runner, plan *DeployPlan) error {
	row := runner.QueryRow(ctx,
		`SELECT revoke_step_index, base_owner_version, base_runtime_version, target_owner_version, target_runtime_version
		   FROM striatumd.deploy_plan WHERE plan_hash = $1`, plan.PlanHash)
	return row.Scan(&plan.RevokeStepIndex, &plan.BaseOwnerVersion, &plan.BaseRuntimeVersion,
		&plan.TargetOwnerVersion, &plan.TargetRuntimeVersion)
}

// DeployCursor is the live deploy_cursor singleton (or a synthesized none).
type DeployCursor struct {
	State     string
	PlanHash  string
	StepIndex int
	StepID    string
}

// LoadDeployCursor reads the singleton deploy_cursor, synthesizing
// DeployStateNone when the table or row is absent (NO deploy in flight). It is a
// defensive read used by both the deployer (resume) and the boot path (A).
func LoadDeployCursor(ctx context.Context, runner Runner) (DeployCursor, error) {
	present, err := runner.QueryScalar(ctx,
		"SELECT (to_regclass('striatumd.deploy_cursor') IS NOT NULL)::text")
	if err != nil {
		return DeployCursor{}, err
	}
	if strings.TrimSpace(present) != "true" {
		return DeployCursor{State: DeployStateNone}, nil
	}
	state, err := runner.QueryScalar(ctx,
		"SELECT COALESCE((SELECT state FROM striatumd.deploy_cursor WHERE id = 'singleton'), '')")
	if err != nil {
		return DeployCursor{}, err
	}
	if strings.TrimSpace(state) == "" || strings.TrimSpace(state) == DeployStateIdle {
		return DeployCursor{State: DeployStateNone}, nil
	}
	cur := DeployCursor{State: strings.TrimSpace(state)}
	planHash, err := runner.QueryScalar(ctx,
		"SELECT COALESCE((SELECT plan_hash FROM striatumd.deploy_cursor WHERE id = 'singleton'), '')")
	if err != nil {
		return DeployCursor{}, err
	}
	cur.PlanHash = strings.TrimSpace(planHash)
	stepIndex, err := runner.QueryScalar(ctx,
		"SELECT COALESCE((SELECT step_index FROM striatumd.deploy_cursor WHERE id = 'singleton'), 0)::text")
	if err != nil {
		return DeployCursor{}, err
	}
	cur.StepIndex, _ = strconv.Atoi(strings.TrimSpace(stepIndex))
	return cur, nil
}

func baseName(path string) string { return filepath.Base(path) }

func marshalSteps(steps []DeployStep) (string, error) {
	if steps == nil {
		steps = []DeployStep{}
	}
	out, err := json.Marshal(steps)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ensureDeploySubstrate applies migration 0044 (deploy_cursor + deploy_plan +
// deploy_receipt) idempotently before transcript materialization. 0044 is
// CREATE TABLE IF NOT EXISTS + a role-guarded GRANT, so a re-run is a no-op; it
// is NEVER a numbered deploy step (the substrate must exist before the cursor it
// records can be written).
func ensureDeploySubstrate(ctx context.Context, runner Runner) error {
	if err := ensureMetaTable(ctx, runner); err != nil {
		return err
	}
	body, err := migrationFS.ReadFile("sql/0044_deploy_cursor.sql")
	if err != nil {
		return err
	}
	return runner.Exec(ctx, string(body))
}

// receiptRowHash is the per-step hash-chain link: sha256(prev || plan_hash ||
// step_index || step_id || step_sha256). A gap or reorder breaks the chain, which
// the doctor block detects.
func receiptRowHash(prev, planHash string, step DeployStep) string {
	sum := sha256.Sum256([]byte(prev + "\x00" + planHash + "\x00" +
		strconv.Itoa(step.StepIndex) + "\x00" + step.StepID + "\x00" + step.SHA256))
	return hex.EncodeToString(sum[:])
}
