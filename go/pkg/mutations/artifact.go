package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/blob"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

var allowedArtifactKinds = map[string]bool{
	"prompt":                       true,
	"finding":                      true,
	"findings_ledger":              true,
	"synthesis":                    true,
	"marker":                       true,
	"handoff":                      true,
	"decision":                     true,
	"patch_summary":                true,
	"test_report":                  true,
	"other":                        true,
	"support_ledger":               true,
	"action_item_ledger":           true,
	"harness_improvement_proposal": true,
	"escalation":                   true,
	"operator_brief":               true,
	"work_plan":                    true,
	"progress_note":                true,
	"operator_report":              true,
	"commit_request":               true,
	"pr_request":                   true,
	"auto_finalize_gate_evidence":  true,
}

func HandlePublishArtifact(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := stringParam(envelope, "session_id")
	jobID := stringParam(envelope, "job_id")
	leaseID := stringParam(envelope, "lease_id")
	kind := stringParam(envelope, "kind")
	logicalName := stringParam(envelope, "logical_name")
	pathText := stringParam(envelope, "path")
	if sessionID == "" || jobID == "" || leaseID == "" || kind == "" || logicalName == "" || pathText == "" {
		return nil, rpc.NewError("schema_invalid", "artifact.publish requires session_id, job_id, lease_id, kind, logical_name, and path", nil)
	}
	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return publishArtifact(ctx, tx, repositoryID, sessionID, jobID, leaseID, kind, logicalName, pathText)
	})
}

func publishArtifact(
	ctx context.Context,
	runner any,
	repositoryID string,
	sessionID string,
	jobID string,
	leaseID string,
	kind string,
	logicalName string,
	pathText string,
) (map[string]any, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return nil, err
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID); err != nil {
		return nil, err
	}
	if kind == "transcript" {
		return nil, rpc.NewError("artifact_error", "transcript artifacts are not allowed by default", nil)
	}
	if !allowedArtifactKinds[kind] {
		return nil, rpc.NewError("artifact_error", fmt.Sprintf("artifact kind %q is not in the allowed kinds list", kind), nil)
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return nil, err
	}
	repoRoot := fmt.Sprint(run["repo_root"])
	if !pathAllowed(repoRoot, pathText, asMap(job["write_scope_json"])) {
		return nil, rpc.NewError("artifact_error", "artifact path is outside the job write scope", nil)
	}
	path, err := repoRelativePath(repoRoot, pathText, false)
	if err != nil {
		return nil, rpc.NewError("artifact_error", err.Error(), nil)
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, rpc.NewError("artifact_error", "artifact file does not exist", nil)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	payload, err = ensureRequiredFrontMatter(kind, path, payload)
	if err != nil {
		return nil, err
	}
	if err := validateMarkdownAuthorLine(ctx, runner, repositoryID, job, sessionID, path, payload); err != nil {
		return nil, err
	}
	if err := validateArtifactFrontMatter(kind, path, payload); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	now := nowString()
	existing, err := oneRow(ctx, runner, `
		SELECT * FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3 AND logical_name = $4
		 LIMIT 1`, repositoryID, job["run_id"], jobID, logicalName)
	if err == nil {
		if fmt.Sprint(existing["content_sha256"]) == digest && fmt.Sprint(existing["repo_path"]) == pathText {
			if kind == "escalation" {
				block, ok := frontMatterBlock(string(payload))
				if !ok {
					return nil, rpc.NewError("artifact_error", "escalation artifact front matter is required to link an escalation blocker", nil)
				}
				frontMatter, err := parseFrontMatterBlock(block)
				if err != nil {
					return nil, rpc.NewError("artifact_error", "escalation artifact front matter is required to link an escalation blocker", nil)
				}
				err = linkEscalationArtifact(ctx, runner, repositoryID, frontMatter, fmt.Sprint(existing["artifact_id"]), fmt.Sprint(job["run_id"]), jobID, sessionID, pathText, digest, now)
				if err != nil {
					return nil, err
				}
			}
			return map[string]any{"status": "already_published", "artifact_id": existing["artifact_id"]}, nil
		}
		return nil, rpc.NewError("artifact_error", "artifact logical name already exists with different content", nil)
	}
	if !errorsIsNoRows(err) {
		return nil, err
	}
	artifactID, err := newID("art")
	if err != nil {
		return nil, err
	}
	authorLine := firstAuthorLine(payload)

	// RFC 0072: blob-routed artifact kinds upload to the per-repo S3
	// bucket before the INSERT. The bucket is recorded on
	// striatumd.repositories.blob_bucket at adopt time. The upload
	// happens inside the publish transaction; the orphan-blob risk on
	// rollback (successful PUT, failed INSERT) is documented and
	// reconciled by a follow-on bucket-vs-PG cleanup job.
	var blobKey, blobSha256, blobContentType string
	if packageBlobClient != nil && isBlobRoutedKind(kind) {
		bucket, err := lookupRepoBlobBucket(ctx, runner, repositoryID)
		if err != nil {
			return nil, err
		}
		if bucket != "" {
			runID := fmt.Sprint(job["run_id"])
			blobKey = blob.ArtifactKey(runID, jobID, logicalName)
			blobContentType = artifactContentType(pathText)
			uploadedSha, err := packageBlobClient.PutBytes(ctx, bucket, blobKey, payload, blobContentType)
			if err != nil {
				return nil, rpc.NewError("blob_publish_failed", err.Error(), map[string]any{
					"bucket": bucket,
					"key":    blobKey,
				})
			}
			if uploadedSha != digest {
				return nil, rpc.NewError("blob_publish_failed", "sha256 mismatch after upload", map[string]any{
					"bucket":   bucket,
					"key":      blobKey,
					"expected": digest,
					"got":      uploadedSha,
				})
			}
			blobSha256 = digest
		}
	}

	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	if err := exec.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, session_id, logical_name,
		  artifact_kind, repo_path, content_sha256, size_bytes, publish_mode,
		  created_at, author_line, blob_key, blob_sha256, blob_content_type
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'create',$11,$12,$13,$14,$15)`,
		repositoryID,
		artifactID,
		job["run_id"],
		jobID,
		sessionID,
		logicalName,
		kind,
		pathText,
		digest,
		len(payload),
		now,
		nullable(authorLine),
		nullable(blobKey),
		nullable(blobSha256),
		nullable(blobContentType),
	); err != nil {
		return nil, err
	}
	if kind == "escalation" {
		block, ok := frontMatterBlock(string(payload))
		if !ok {
			return nil, rpc.NewError("artifact_error", "escalation artifact front matter is required to link an escalation blocker", nil)
		}
		frontMatter, err := parseFrontMatterBlock(block)
		if err != nil {
			return nil, rpc.NewError("artifact_error", "escalation artifact front matter is required to link an escalation blocker", nil)
		}
		err = linkEscalationArtifact(ctx, runner, repositoryID, frontMatter, artifactID, fmt.Sprint(job["run_id"]), jobID, sessionID, pathText, digest, now)
		if err != nil {
			return nil, err
		}
	}
	eventPayload := map[string]any{
		"logical_name": logicalName,
		"path":         pathText,
		"sha256":       digest,
	}
	if blobKey != "" {
		eventPayload["blob_key"] = blobKey
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "artifact.published", sessionID, jobID, nil, artifactID, leaseID, eventPayload); err != nil {
		return nil, err
	}
	result := map[string]any{"status": "published", "artifact_id": artifactID, "sha256": digest}
	if blobKey != "" {
		result["blob_key"] = blobKey
	}
	return result, nil
}

// blobRoutedKinds enumerates the artifact kinds whose body lives in
// S3-compatible blob storage per RFC 0072 § Boundary. Kinds outside
// this set keep the existing repo-path semantics: the body is reached
// via repo_path, not blob_key.
//
// The split is by review surface:
//   - Stays git-tracked (PR review surface): decision, escalation,
//     work_plan, operator_brief, operator_report. These are
//     decisional artifacts a human reads in a diff.
//   - Goes to blob (per-run data): finding, synthesis, *_ledger,
//     harness_improvement_proposal, progress_note. These pile up per
//     dogfood run and do not get PR review.
//
// Other kinds (handoff, prompt, marker, etc.) remain repo-path-only in
// V1; the maintainer can extend this set in a follow-on RFC.
var blobRoutedKinds = map[string]struct{}{
	"finding":                      {},
	"findings_ledger":              {},
	"synthesis":                    {},
	"support_ledger":               {},
	"action_item_ledger":           {},
	"harness_improvement_proposal": {},
	"progress_note":                {},
}

func isBlobRoutedKind(kind string) bool {
	_, ok := blobRoutedKinds[kind]
	return ok
}

// lookupRepoBlobBucket returns the per-repo bucket recorded at adopt
// time. Returns "" with no error when the repo's row has a NULL
// blob_bucket (operator has not enabled blob storage for this repo);
// callers then skip the upload and store the artifact body in the
// repo path only.
func lookupRepoBlobBucket(ctx context.Context, runner any, repositoryID string) (string, error) {
	row, err := oneRow(ctx, runner, `
		SELECT blob_bucket FROM striatumd.repositories
		 WHERE repository_id = $1 AND state != 'removed' LIMIT 1`, repositoryID)
	if err != nil {
		if errorsIsNoRows(err) {
			return "", nil
		}
		return "", err
	}
	bucket, _ := row["blob_bucket"].(string)
	return bucket, nil
}

// artifactContentType returns a conservative content type for the
// uploaded blob. Markdown files (the common case) get
// "text/markdown; charset=utf-8"; anything else gets
// "application/octet-stream" to avoid claiming a richer type than the
// runner actually verified.
func artifactContentType(pathText string) string {
	if strings.HasSuffix(strings.ToLower(pathText), ".md") {
		return "text/markdown; charset=utf-8"
	}
	if strings.HasSuffix(strings.ToLower(pathText), ".json") {
		return "application/json"
	}
	if strings.HasSuffix(strings.ToLower(pathText), ".txt") {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

func pathAllowed(repoRoot, pathText string, writeScope map[string]any) bool {
	resolved, err := repoRelativePath(repoRoot, pathText, false)
	if err != nil {
		return false
	}
	forbidden := asList(writeScope["forbidden_paths"])
	if len(forbidden) == 0 {
		forbidden = []any{".striatum"}
	}
	for _, item := range forbidden {
		text, ok := item.(string)
		if !ok {
			continue
		}
		denied, err := repoRelativePath(repoRoot, text, true)
		if err != nil {
			continue
		}
		if sameOrInside(resolved, denied) {
			return false
		}
	}
	for _, item := range asList(writeScope["allowed_paths"]) {
		text, ok := item.(string)
		if !ok {
			continue
		}
		base, err := repoRelativePath(repoRoot, text, true)
		if err != nil {
			continue
		}
		if sameOrInside(resolved, base) {
			return true
		}
	}
	return false
}

func repoRelativePath(repoRoot, pathText string, allowState bool) (string, error) {
	if filepath.IsAbs(pathText) {
		return "", fmt.Errorf("artifact path must be repo-relative")
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	resolved := filepath.Clean(filepath.Join(repoAbs, pathText))
	if !sameOrInside(resolved, repoAbs) {
		return "", fmt.Errorf("artifact path must stay inside the repository")
	}
	statePath := filepath.Join(repoAbs, ".striatum")
	if !allowState && sameOrInside(resolved, statePath) {
		return "", fmt.Errorf("artifact path cannot be under .striatum")
	}
	return resolved, nil
}

func sameOrInside(path string, base string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."))
}

func ensureRequiredFrontMatter(kind string, path string, payload []byte) ([]byte, error) {
	if kind != "synthesis" || !isMarkdownPath(path) {
		return payload, nil
	}
	text := string(payload)
	if strings.HasPrefix(text, "---\n") || strings.HasPrefix(text, "---\r\n") {
		return payload, nil
	}
	prepend := "---\nschema_version: \"striatum.synthesis.v1\"\nartifact_kind: \"synthesis\"\n---\n\n"
	updated := []byte(prepend + text)
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return nil, err
	}
	return updated, nil
}

type frontMatterField struct {
	required bool
	check    func(any) bool
}

type frontMatterSchema struct {
	fields map[string]frontMatterField
}

var frontMatterSchemas = map[string]frontMatterSchema{
	"decision": {
		fields: map[string]frontMatterField{
			"schema_version":     {true, equalsValue("striatum.decision.v1")},
			"artifact_kind":      {true, equalsValue("decision")},
			"decision_id":        {true, isStringValue},
			"run_id":             {true, isStringValue},
			"owner":              {true, equalsValue("human")},
			"outcome":            {true, oneOfValue("accepted", "rejected", "accepted_with_follow_up")},
			"follow_up_required": {true, isBoolValue},
			"title":              {true, isStringValue},
			"created_at":         {true, isStringValue},
		},
	},
	"finding": {
		fields: map[string]frontMatterField{
			"schema_version": {true, equalsValue("striatum.finding.v1")},
			"artifact_kind":  {true, equalsValue("finding")},
			"verdict_intent": {true, oneOfValue("accept", "accept_with_findings", "needs_revision", "reject")},
			"severity":       {false, oneOfValue("info", "low", "medium", "high", "critical")},
			"tags":           {false, isStringListValue},
		},
	},
	"findings_ledger": {
		fields: map[string]frontMatterField{
			"schema_version": {true, equalsValue("striatum.findings_ledger.v1")},
			"artifact_kind":  {true, equalsValue("findings_ledger")},
			"summary_count":  {true, isNonNegativeIntValue},
			"entries_path":   {false, isStringValue},
		},
	},
	"synthesis": {
		fields: map[string]frontMatterField{
			"schema_version": {true, equalsValue("striatum.synthesis.v1")},
			"artifact_kind":  {true, equalsValue("synthesis")},
			"inputs":         {false, isStringListValue},
		},
	},
	"support_ledger": {
		fields: map[string]frontMatterField{
			"schema_version":   {true, equalsValue("striatum.support_ledger.v1")},
			"artifact_kind":    {true, equalsValue("support_ledger")},
			"audited_artifact": {true, isStringValue},
			"claim_count":      {false, isNonNegativeIntValue},
		},
	},
	"action_item_ledger": {
		fields: map[string]frontMatterField{
			"schema_version":         {true, equalsValue("striatum.action_item_ledger.v1")},
			"artifact_kind":          {true, equalsValue("action_item_ledger")},
			"source_review_artifact": {true, isStringValue},
			"revision_round":         {true, isNonNegativeIntValue},
			"total_items":            {false, isNonNegativeIntValue},
		},
	},
	"harness_improvement_proposal": {
		fields: map[string]frontMatterField{
			"schema_version":   {true, equalsValue("striatum.harness_improvement_proposal.v1")},
			"artifact_kind":    {true, equalsValue("harness_improvement_proposal")},
			"target":           {true, oneOfValue("prompt", "workflow", "spec", "defaults", "documentation")},
			"expected_benefit": {true, isStringValue},
			"risk":             {false, isStringValue},
			"rollback":         {false, isStringValue},
		},
	},
	"escalation": {
		fields: map[string]frontMatterField{
			"schema_version":    {true, equalsValue("striatum.escalation.v1")},
			"artifact_kind":     {true, equalsValue("escalation")},
			"escalation_id":     {true, isNonEmptyStringValue},
			"run_id":            {true, isNonEmptyStringValue},
			"job_id":            {false, isNonEmptyStringValue},
			"session_id":        {false, isNonEmptyStringValue},
			"severity":          {true, oneOfValue("info", "low", "medium", "high", "critical")},
			"blocker_kind":      {true, oneOfValue("ambiguous_goal", "missing_authority", "contradicting_decisions", "no_available_reviewer_lane", "committee_stalemate", "override_required")},
			"description":       {true, isNonEmptyStringValue},
			"reasoning":         {true, isNonEmptyStringValue},
			"requested_action":  {true, isNonEmptyStringValue},
			"related_artifacts": {false, isStringListValue},
			"created_at":        {true, isNonEmptyStringValue},
		},
	},
	"operator_brief": {
		fields: map[string]frontMatterField{
			"schema_version":       {true, equalsValue("striatum.operator_brief.v1")},
			"artifact_kind":        {true, equalsValue("operator_brief")},
			"brief_id":             {true, isNonEmptyStringValue},
			"supersedes":           {true, isNullableNonEmptyStringValue},
			"scope_links":          {true, isStringListValue},
			"context_budget_lines": {true, isNonNegativeIntValue},
			"retrieval_priority":   {true, oneOfValue("high", "medium", "low")},
			"status":               {true, oneOfValue("current", "superseded")},
			"author":               {false, isNonEmptyStringValue},
		},
	},
	"work_plan": {
		fields: map[string]frontMatterField{
			"schema_version":     {true, equalsValue("striatum.work_plan.v1")},
			"artifact_kind":      {true, equalsValue("work_plan")},
			"plan_id":            {true, isNonEmptyStringValue},
			"scope_kind":         {true, oneOfValue("rfc", "phase", "initiative", "bugfix")},
			"scope_ref":          {true, isNonEmptyStringValue},
			"state":              {true, oneOfValue("open", "in_progress", "closed")},
			"opened_at":          {true, isNonEmptyStringValue},
			"closed_at":          {true, isNullableNonEmptyStringValue},
			"closure_summary":    {true, isNullableNonEmptyStringValue},
			"supersedes":         {true, isNullableNonEmptyStringValue},
			"retrieval_priority": {true, oneOfValue("high", "medium", "low")},
			"author":             {false, isNonEmptyStringValue},
		},
	},
	"progress_note": {
		fields: map[string]frontMatterField{
			"schema_version":     {true, equalsValue("striatum.progress_note.v1")},
			"artifact_kind":      {true, equalsValue("progress_note")},
			"note_date":          {true, isNonEmptyStringValue},
			"session_slug":       {true, isNonEmptyStringValue},
			"related_plan":       {true, isNullableNonEmptyStringValue},
			"related_brief":      {true, isNullableNonEmptyStringValue},
			"retrieval_priority": {true, oneOfValue("high", "medium", "low")},
			"author":             {false, isNonEmptyStringValue},
		},
	},
	"operator_report": {
		fields: map[string]frontMatterField{
			"schema_version":     {true, equalsValue("striatum.operator_report.v1")},
			"artifact_kind":      {true, equalsValue("operator_report")},
			"author":             {false, isNonEmptyStringValue},
			"retrieval_priority": {false, oneOfValue("high", "medium", "low")},
			"supersedes":         {false, isNullableNonEmptyStringValue},
		},
	},
	"commit_request": {
		fields: map[string]frontMatterField{
			"schema_version":      {true, equalsValue("striatum.commit_request.v1")},
			"artifact_kind":       {true, equalsValue("commit_request")},
			"request_id":          {true, isNonEmptyStringValue},
			"run_id":              {false, isNonEmptyStringValue},
			"base_head":           {true, isNonEmptyStringValue},
			"branch":              {true, isNonEmptyStringValue},
			"git_snapshot_hash":   {true, isNonEmptyStringValue},
			"included_paths":      {true, isNonEmptyStringListValue},
			"reviewed_artifacts":  {false, isNonEmptyStringListValue},
			"commit_message":      {true, isNonEmptyStringValue},
			"rationale":           {true, isNonEmptyStringValue},
			"confirmation_status": {true, oneOfValue("pending", "operator_confirmed", "human_confirmed", "refused")},
			"confirmed_by":        {false, isNullableNonEmptyStringValue},
			"confirmed_at":        {false, isNullableNonEmptyStringValue},
		},
	},
	"pr_request": {
		fields: map[string]frontMatterField{
			"schema_version":         {true, equalsValue("striatum.pr_request.v1")},
			"artifact_kind":          {true, equalsValue("pr_request")},
			"request_id":             {true, isNonEmptyStringValue},
			"run_id":                 {false, isNonEmptyStringValue},
			"target_branch":          {true, isNonEmptyStringValue},
			"summary":                {true, isNonEmptyStringValue},
			"body_draft":             {true, isNonEmptyStringValue},
			"related_commit_request": {false, isNullableNonEmptyStringValue},
			"local_commit_sha":       {false, isNullableNonEmptyStringValue},
			"provider_target":        {false, isNullableNonEmptyStringValue},
			"confirmation_status":    {true, oneOfValue("pending", "human_confirmed", "refused")},
			"confirmed_by":           {false, isNullableNonEmptyStringValue},
			"confirmed_at":           {false, isNullableNonEmptyStringValue},
		},
	},
	"auto_finalize_gate_evidence": {
		fields: map[string]frontMatterField{
			"schema_version":               {true, equalsValue("striatum.auto_finalize_gate_evidence.v1")},
			"artifact_kind":                {true, equalsValue("auto_finalize_gate_evidence")},
			"decision_id":                  {true, equalsValue("D125")},
			"gate_status":                  {true, oneOfValue("pending", "satisfied")},
			"live_success_count":           {true, isNonNegativeIntValue},
			"lane_shape_count":             {true, isNonNegativeIntValue},
			"lane_shapes":                  {true, isNonEmptyStringListValue},
			"contested_audit_chain_events": {true, isNonNegativeIntValue},
			"evidence_artifacts":           {true, isNonEmptyStringListValue},
			"created_at":                   {true, isNonEmptyStringValue},
		},
	},
}

func validateArtifactFrontMatter(kind string, path string, payload []byte) error {
	schema, ok := frontMatterSchemas[kind]
	if !ok || !isMarkdownPath(path) {
		return nil
	}
	block, ok := frontMatterBlock(string(payload))
	if !ok {
		return nil
	}
	parsed, err := parseFrontMatterBlock(block)
	if err != nil {
		return rpc.NewError("artifact_error", err.Error(), nil)
	}
	for name, field := range schema.fields {
		value, exists := parsed[name]
		if !exists {
			if field.required {
				return rpc.NewError("artifact_error", fmt.Sprintf("%s artifact front matter missing required field %q", kind, name), nil)
			}
			continue
		}
		if !field.check(value) {
			return rpc.NewError("artifact_error", fmt.Sprintf("%s artifact front matter field %q is invalid", kind, name), nil)
		}
	}
	extra := []string{}
	for name := range parsed {
		if _, ok := schema.fields[name]; !ok {
			extra = append(extra, name)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return rpc.NewError("artifact_error", fmt.Sprintf("%s artifact front matter has unknown fields: %s", kind, strings.Join(extra, ", ")), nil)
	}
	return nil
}

func frontMatterBlock(text string) (string, bool) {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), true
		}
	}
	return "", false
}

func parseFrontMatterBlock(block string) (map[string]any, error) {
	result := map[string]any{}
	for _, raw := range strings.Split(block, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("artifact front matter has invalid line %q", raw)
		}
		key := strings.TrimSpace(parts[0])
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("artifact front matter field %q is declared more than once", key)
		}
		result[key] = parseFrontMatterValue(strings.TrimSpace(parts[1]))
	}
	return result, nil
}

func parseFrontMatterValue(value string) any {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		body := strings.TrimSpace(value[1 : len(value)-1])
		if body == "" {
			return []string{}
		}
		items := []string{}
		for _, item := range strings.Split(body, ",") {
			items = append(items, strings.Trim(strings.TrimSpace(item), `"'`))
		}
		return items
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return value
}

func equalsValue(expected string) func(any) bool {
	return func(value any) bool { return fmt.Sprint(value) == expected }
}

func oneOfValue(options ...string) func(any) bool {
	allowed := map[string]bool{}
	for _, option := range options {
		allowed[option] = true
	}
	return func(value any) bool { return allowed[fmt.Sprint(value)] }
}

func isStringValue(value any) bool {
	_, ok := value.(string)
	return ok
}

func isNonEmptyStringValue(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func isNullableNonEmptyStringValue(value any) bool {
	if value == nil {
		return true
	}
	return isNonEmptyStringValue(value)
}

func isBoolValue(value any) bool {
	_, ok := value.(bool)
	return ok
}

func isNonNegativeIntValue(value any) bool {
	switch typed := value.(type) {
	case int:
		return typed >= 0
	case int16:
		return typed >= 0
	case int32:
		return typed >= 0
	case int64:
		return typed >= 0
	case float64:
		return typed >= 0 && typed == float64(int64(typed))
	case json.Number:
		parsed, err := typed.Int64()
		return err == nil && parsed >= 0
	}
	return false
}

func isStringListValue(value any) bool {
	switch typed := value.(type) {
	case []string:
		return true
	case []any:
		for _, item := range typed {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	}
	return false
}

func isNonEmptyStringListValue(value any) bool {
	switch typed := value.(type) {
	case []string:
		if len(typed) == 0 {
			return false
		}
		for _, item := range typed {
			if strings.TrimSpace(item) == "" {
				return false
			}
		}
		return true
	case []any:
		if len(typed) == 0 {
			return false
		}
		for _, item := range typed {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return false
			}
		}
		return true
	}
	return false
}

func validateMarkdownAuthorLine(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string, path string, payload []byte) error {
	if !isMarkdownPath(path) {
		return nil
	}
	text := string(payload)
	expected, err := expectedAuthorLine(ctx, runner, repositoryID, job, sessionID)
	if err != nil {
		return err
	}
	for _, line := range markdownTitleBlockAuthorLines(text) {
		canonical := canonicalBylineForm(line)
		if canonical == "" || canonical != expected {
			return rpc.NewError("artifact_error", "markdown artifact author line must match expected work packet author line", nil)
		}
	}
	return nil
}

func expectedAuthorLine(ctx context.Context, runner any, repositoryID string, job map[string]any, sessionID string) (string, error) {
	session, err := rowByID(ctx, runner, repositoryID, "sessions", "session_id", sessionID, false)
	if err != nil {
		return "", err
	}
	snapshotRun, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", fmt.Sprint(job["run_id"]), false)
	if err != nil {
		return "", err
	}
	snapshot, err := rowByID(ctx, runner, repositoryID, "workflow_snapshots", "workflow_snapshot_id", fmt.Sprint(snapshotRun["workflow_snapshot_id"]), false)
	if err != nil {
		return "", err
	}
	workflow := asMap(snapshot["workflow_json"])
	laneID, _ := asMap(job["lane_selector_json"])["lane_id"].(string)
	attestation := sessionLaneAttestation(ctx, runner, repositoryID, sessionID)
	attested, _ := attestation["attested"].(bool)
	operatorLabel, _ := nullable(session["operator_label"]).(string)
	author := artifactAuthorIdentity(
		workflow,
		fmt.Sprint(job["role_id"]),
		laneID,
		fmt.Sprint(job["workflow_job_id"]),
		intValue(session["ordinal"]),
		attested,
		operatorLabel,
	)
	line, _ := author["line"].(string)
	if line == "" {
		return "", rpc.NewError("artifact_error", "expected artifact author line could not be derived", nil)
	}
	return strings.ToLower(line), nil
}

func isMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func firstAuthorLine(payload []byte) string {
	for _, line := range markdownTitleBlockAuthorLines(string(payload)) {
		if canonical := canonicalBylineForm(line); canonical != "" {
			return canonical
		}
	}
	return ""
}

func markdownTitleBlockAuthorLines(text string) []string {
	lines := strings.Split(text, "\n")
	frontMatter := []string{}
	titleBlock := []string{}
	bodyStart := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				bodyStart = i + 1
				break
			}
		}
		if bodyStart > 0 {
			for _, line := range lines[1 : bodyStart-1] {
				if canonicalBylineForm(line) != "" {
					frontMatter = append(frontMatter, line)
				}
			}
		}
	}
	limit := bodyStart + 40
	if limit > len(lines) {
		limit = len(lines)
	}
	for _, line := range lines[bodyStart:limit] {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if canonicalBylineForm(line) != "" {
			titleBlock = append(titleBlock, line)
			break
		}
	}
	if len(frontMatter) > 0 {
		return frontMatter
	}
	return titleBlock
}

func canonicalBylineForm(line string) string {
	stripped := strings.TrimSpace(line)
	for strings.HasPrefix(stripped, "#") {
		stripped = strings.TrimSpace(stripped[1:])
	}
	stripped = strings.ReplaceAll(stripped, "*", "")
	stripped = strings.ReplaceAll(stripped, "_", "")
	stripped = strings.TrimSpace(stripped)
	if !strings.HasPrefix(strings.ToLower(stripped), "author:") {
		return ""
	}
	return "author: " + strings.ToLower(strings.TrimSpace(strings.SplitN(stripped, ":", 2)[1]))
}

func errorsIsNoRows(err error) bool {
	return err == nil || err == pgx.ErrNoRows
}

func linkEscalationArtifact(
	ctx context.Context,
	runner any,
	repositoryID string,
	frontMatter map[string]any,
	artifactID string,
	runID string,
	jobID string,
	sessionID string,
	repoPath string,
	contentSha256 string,
	linkedAt string,
) error {
	escalationID, _ := frontMatter["escalation_id"].(string)
	if escalationID == "" {
		return rpc.NewError("artifact_error", "escalation artifact front matter missing escalation_id", nil)
	}
	frontRunID, _ := frontMatter["run_id"].(string)
	if frontRunID != runID {
		return rpc.NewError("artifact_error", "escalation artifact run_id must match publish context", nil)
	}
	if frontJobID, ok := frontMatter["job_id"].(string); ok && frontJobID != jobID {
		return rpc.NewError("artifact_error", "escalation artifact job_id must match publish context", nil)
	}
	if frontSessionID, ok := frontMatter["session_id"].(string); ok && frontSessionID != sessionID {
		return rpc.NewError("artifact_error", "escalation artifact session_id must match publish context", nil)
	}

	blocker, err := oneRow(ctx, runner, `
		SELECT blocker_id, run_id, severity, blocker_kind, payload_json
		  FROM striatumd.blockers
		 WHERE repository_id = $1
		   AND blocker_id = $2
		   FOR UPDATE`, repositoryID, escalationID)
	if err != nil {
		return rpc.NewError("artifact_error", "escalation artifact escalation_id does not match an existing blocker", nil)
	}
	if fmt.Sprint(blocker["run_id"]) != runID {
		return rpc.NewError("artifact_error", "escalation artifact blocker must belong to the same run", nil)
	}
	if !isEscalationClassBlocker(blocker) {
		return rpc.NewError("artifact_error", "escalation artifact blocker is not escalation-class", nil)
	}

	metadata := map[string]any{
		"artifact_id":    artifactID,
		"repo_path":      repoPath,
		"content_sha256": contentSha256,
		"linked_at":      linkedAt,
		"link_source":    "artifact.publish",
	}

	payloadJSONRaw := blocker["payload_json"]
	var payloadJSON map[string]any
	if payloadStr, ok := payloadJSONRaw.(string); ok {
		json.Unmarshal([]byte(payloadStr), &payloadJSON)
	} else if payloadBytes, ok := payloadJSONRaw.([]byte); ok {
		json.Unmarshal(payloadBytes, &payloadJSON)
	} else if payloadMap, ok := payloadJSONRaw.(map[string]any); ok {
		payloadJSON = payloadMap
	}
	if payloadJSON == nil {
		payloadJSON = map[string]any{}
	}

	if existingLink, ok := payloadJSON["escalation_artifact"].(map[string]any); ok {
		for _, key := range []string{"artifact_id", "repo_path", "content_sha256", "link_source"} {
			if existingLink[key] != metadata[key] {
				return rpc.NewError("artifact_error", "escalation blocker is already linked to a different artifact", nil)
			}
		}
		return nil
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return fmt.Errorf("runner does not support exec")
	}

	if err := exec.Exec(ctx, `
		UPDATE striatumd.blockers
		   SET payload_json = jsonb_set(
				   COALESCE(payload_json, '{}'::jsonb),
				   '{escalation_artifact}',
				   $1,
				   true
			   )
		 WHERE repository_id = $2
		   AND blocker_id = $3`, string(metadataBytes), repositoryID, escalationID); err != nil {
		return err
	}

	if err := exec.Exec(ctx, `
		UPDATE striatumd.escalation_inbox
		   SET payload_json = jsonb_set(
				   COALESCE(payload_json, '{}'::jsonb),
				   '{escalation_artifact}',
				   $1,
				   true
			   )
		 WHERE repository_id = $2
		   AND escalation_id = $3`, string(metadataBytes), repositoryID, escalationID); err != nil {
		return err
	}

	return nil
}

func isEscalationClassBlocker(blocker map[string]any) bool {
	severity := fmt.Sprint(blocker["severity"])
	kind := fmt.Sprint(blocker["blocker_kind"])
	return severity == "human_checkpoint" ||
		kind == "ambiguous_goal" ||
		kind == "missing_authority" ||
		kind == "contradicting_decisions" ||
		kind == "no_available_reviewer_lane" ||
		kind == "committee_stalemate" ||
		kind == "override_required" ||
		kind == "ai_self_declared"
}
