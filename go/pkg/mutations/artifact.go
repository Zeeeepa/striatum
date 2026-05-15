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
	existing, err := oneRow(ctx, runner, `
		SELECT * FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND job_id = $3 AND logical_name = $4
		 LIMIT 1`, repositoryID, job["run_id"], jobID, logicalName)
	if err == nil {
		if fmt.Sprint(existing["content_sha256"]) == digest && fmt.Sprint(existing["repo_path"]) == pathText {
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
	now := nowString()
	authorLine := firstAuthorLine(payload)
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
		  created_at, author_line
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'create',$11,$12)`,
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
	); err != nil {
		return nil, err
	}
	if _, err := appendEvent(ctx, runner, repositoryID, job["run_id"], "artifact.published", sessionID, jobID, nil, artifactID, leaseID, map[string]any{
		"logical_name": logicalName,
		"path":         pathText,
		"sha256":       digest,
	}); err != nil {
		return nil, err
	}
	return map[string]any{"status": "published", "artifact_id": artifactID, "sha256": digest}, nil
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
		result[strings.TrimSpace(parts[0])] = parseFrontMatterValue(strings.TrimSpace(parts[1]))
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
