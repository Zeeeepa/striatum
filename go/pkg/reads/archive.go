package reads

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const archiveSchemaVersion = "striatum.run_archive.v1"
const archiveContractVersion = 2
const archiveVerificationDepth = "deep_chain"
const archiveArtifactContentPolicy = "metadata_only"

var archiveHybridDefaults = map[string]bool{
	"snapshot":                 true,
	"event_log":                true,
	"verify_replay_by_default": true,
}

var archiveJSONFiles = map[string]string{
	"run":               "run.json",
	"workflow_snapshot": "workflow_snapshot.json",
}

var archiveJSONLFiles = map[string]string{
	"sessions":                    "sessions.jsonl",
	"jobs":                        "jobs.jsonl",
	"job_dependencies":            "job_dependencies.jsonl",
	"queue_messages":              "queue_messages.jsonl",
	"leases":                      "leases.jsonl",
	"work_packets":                "work_packets.jsonl",
	"artifacts":                   "artifacts.jsonl",
	"verdicts":                    "verdicts.jsonl",
	"blockers":                    "blockers.jsonl",
	"command_requests":            "command_requests.jsonl",
	"process_executions":          "process_executions.jsonl",
	"job_worktrees":               "job_worktrees.jsonl",
	"process_supervisors":         "process_supervisors.jsonl",
	"process_supervisor_pointers": "process_supervisor_pointers.jsonl",
	"events":                      "events.jsonl",
}

var archiveKinds = []string{
	"run",
	"workflow_snapshot",
	"sessions",
	"jobs",
	"job_dependencies",
	"queue_messages",
	"leases",
	"work_packets",
	"artifacts",
	"verdicts",
	"blockers",
	"command_requests",
	"process_executions",
	"job_worktrees",
	"process_supervisors",
	"process_supervisor_pointers",
	"events",
}

type archiveTable struct {
	kind    string
	table   string
	orderBy string
}

var archiveRunScopedTables = []archiveTable{
	{"sessions", "sessions", "registered_at, session_id"},
	{"jobs", "jobs", "created_at, job_id"},
	{"queue_messages", "queue_messages", "created_at, message_id"},
	{"leases", "leases", "acquired_at, lease_id"},
	{"work_packets", "work_packets", "created_at, packet_id"},
	{"artifacts", "artifacts", "created_at, artifact_id"},
	{"verdicts", "verdicts", "created_at, verdict_id"},
	{"blockers", "blockers", "created_at, blocker_id"},
	{"command_requests", "command_requests", "created_at, request_id"},
	{"process_executions", "process_executions", "started_at, process_id"},
	{"job_worktrees", "job_worktrees", "created_at, worktree_id"},
	{"process_supervisors", "process_supervisors", "started_at, supervisor_id"},
	{"process_supervisor_pointers", "process_supervisor_pointers", "updated_at, supervisor_id"},
	{"events", "events", "event_id"},
}

func HandleArchiveCreate(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	outText := stringParam(envelope, "out")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "archive.create requires run_id", nil)
	}
	if outText == "" {
		return nil, rpc.NewError("schema_invalid", "archive.create requires out", nil)
	}
	repoRoot, err := archiveRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	out, err := safeArchiveOutputPath(repoRoot, outText)
	if err != nil {
		return nil, err
	}
	runRows, err := collectRows(ctx, runner,
		// RFC 0167 P0 C2".2(d): explicit columns EXCLUDING created_by_principal_id —
		// SELECT r.* would 42501 under the column-scoped runs grant (bundle 0022).
		// Export carries no identity principal in P0.
		`SELECT r.repository_id, r.run_id, r.workflow_snapshot_id, r.repo_root, r.state,
		        r.branch_name, r.branch_base, r.branch_confirmed_at, r.branch_confirmed_by, r.created_at,
		        r.started_at, r.completed_at, r.stop_reason, r.paused_at, r.paused_reason, r.cross_repo_run_id
		   FROM striatumd.runs r
		  WHERE r.repository_id = $1 AND r.run_id = $2`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if len(runRows) == 0 {
		return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
	}
	workflowRows, err := collectRows(ctx, runner,
		`SELECT w.*
		   FROM striatumd.runs r
		   JOIN striatumd.workflow_snapshots w
		     ON w.repository_id = r.repository_id
		    AND w.workflow_snapshot_id = r.workflow_snapshot_id
		  WHERE r.repository_id = $1 AND r.run_id = $2`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if len(workflowRows) == 0 {
		return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
	}
	rows := map[string][]map[string]any{}
	for _, table := range archiveRunScopedTables {
		items, err := archiveRows(ctx, runner, table, repositoryID, runID)
		if err != nil {
			return nil, err
		}
		rows[table.kind] = redactArchiveRows(table.kind, items)
	}
	dependencies, err := archiveJobDependencies(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	rows["job_dependencies"] = redactArchiveRows("job_dependencies", dependencies)

	return writeRunArchive(
		out,
		repoRoot,
		repositoryID,
		runID,
		redactArchiveRow("run", normalizeArchiveRow(runRows[0])),
		redactArchiveRow("workflow_snapshot", normalizeArchiveRow(workflowRows[0])),
		rows,
	)
}

func archiveRepositoryRoot(ctx context.Context, runner db.Runner, repositoryID string) (string, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT repo_root
		   FROM striatumd.repositories
		  WHERE repository_id = $1 AND state = 'active'`,
		repositoryID,
	)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", rpc.NewError("repo_not_registered", "repository is not registered: "+repositoryID, nil)
	}
	root := fmt.Sprint(rows[0]["repo_root"])
	if root == "" {
		return "", rpc.NewError("repo_not_registered", "repository has no repo_root: "+repositoryID, nil)
	}
	return filepath.Abs(root)
}

func safeArchiveOutputPath(repoRoot string, pathText string) (string, error) {
	if filepath.IsAbs(pathText) {
		return "", rpc.NewError("path_outside_scope", "path must be repo-relative", nil)
	}
	repoResolved, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(repoResolved, pathText))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(repoResolved, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", rpc.NewError("path_outside_scope", "path must stay inside the repository", nil)
	}
	if rel == ".striatum" || strings.HasPrefix(rel, ".striatum"+string(os.PathSeparator)) {
		return "", rpc.NewError("path_outside_scope", "path must not be under .striatum", nil)
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		return "", rpc.NewError("path_outside_scope", "--out must be a directory", nil)
	}
	return target, nil
}

func archiveRows(ctx context.Context, runner db.Runner, table archiveTable, repositoryID string, runID string) ([]map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		fmt.Sprintf(
			`SELECT *
			   FROM striatumd.%s
			  WHERE repository_id = $1 AND run_id = $2
			  ORDER BY %s`,
			table.table,
			table.orderBy,
		),
		repositoryID,
		runID,
	)
	if err != nil {
		return nil, err
	}
	return normalizeArchiveRows(rows), nil
}

func archiveJobDependencies(ctx context.Context, runner db.Runner, repositoryID string, runID string) ([]map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT d.*
		   FROM striatumd.job_dependencies d
		   JOIN striatumd.jobs j
		     ON j.repository_id = d.repository_id
		    AND j.job_id = d.job_id
		  WHERE d.repository_id = $1 AND j.run_id = $2
		  ORDER BY d.job_id, d.depends_on_job_id`,
		repositoryID,
		runID,
	)
	if err != nil {
		return nil, err
	}
	return normalizeArchiveRows(rows), nil
}

func writeRunArchive(out string, repoRoot string, repositoryID string, runID string, run map[string]any, workflowSnapshot map[string]any, rows map[string][]map[string]any) (map[string]any, error) {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	files := map[string]map[string]any{}
	rowCounts := map[string]int{}
	runMeta, err := writeArchiveJSON(filepath.Join(out, archiveJSONFiles["run"]), run)
	if err != nil {
		return nil, err
	}
	files[archiveJSONFiles["run"]] = runMeta
	rowCounts["run"] = 1
	workflowMeta, err := writeArchiveJSON(filepath.Join(out, archiveJSONFiles["workflow_snapshot"]), workflowSnapshot)
	if err != nil {
		return nil, err
	}
	files[archiveJSONFiles["workflow_snapshot"]] = workflowMeta
	rowCounts["workflow_snapshot"] = 1
	for _, kind := range archiveKinds {
		filename, ok := archiveJSONLFiles[kind]
		if !ok {
			continue
		}
		kindRows := rows[kind]
		meta, err := writeArchiveJSONL(filepath.Join(out, filename), kindRows)
		if err != nil {
			return nil, err
		}
		files[filename] = meta
		rowCounts[kind] = len(kindRows)
	}
	for _, kind := range archiveKinds {
		if _, ok := rowCounts[kind]; !ok {
			rowCounts[kind] = 0
		}
	}
	manifest := buildArchiveManifest(repoRoot, repositoryID, runID, files, rowCounts)
	manifest["bundle_sha256"] = canonicalArchiveManifestSHA(manifest)
	if _, err := writeArchiveJSON(filepath.Join(out, "manifest.json"), manifest); err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(out, "manifest.json")
	return map[string]any{
		"status":        "archived",
		"run_id":        runID,
		"out":           displayArchivePath(repoRoot, out),
		"manifest_path": displayArchivePath(repoRoot, manifestPath),
		"row_counts":    rowCounts,
		"bundle_sha256": manifest["bundle_sha256"],
	}, nil
}

func buildArchiveManifest(repoRoot string, repositoryID string, runID string, files map[string]map[string]any, rowCounts map[string]int) map[string]any {
	normalizedCounts := map[string]int{}
	for _, kind := range archiveKinds {
		normalizedCounts[kind] = rowCounts[kind]
	}
	return map[string]any{
		"schema_version":           archiveSchemaVersion,
		"striatum_version":         "unknown",
		"repository_id":            repositoryID,
		"repo_root":                mustAbs(repoRoot),
		"run_id":                   runID,
		"generated_at":             time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		"archive_kind":             "run",
		"archive_contract_version": archiveContractVersion,
		"verification_depth":       archiveVerificationDepth,
		"hybrid_archive_defaults":  archiveHybridDefaults,
		"artifact_content_policy":  archiveArtifactContentPolicy,
		"schema": map[string]any{
			"row_shape_version": 1,
			"json_files":        archiveJSONFiles,
			"jsonl_files":       archiveJSONLFiles,
		},
		"row_counts": normalizedCounts,
		"files":      files,
	}
}

func writeArchiveJSON(path string, payload map[string]any) (map[string]any, error) {
	body, err := marshalIndentedArchiveJSON(payload)
	if err != nil {
		return nil, err
	}
	meta, err := writeArchiveFile(path, body)
	if err != nil {
		return nil, err
	}
	meta["rows"] = 1
	return meta, nil
}

func writeArchiveJSONL(path string, rows []map[string]any) (map[string]any, error) {
	var buf bytes.Buffer
	for _, row := range rows {
		body, err := marshalCompactArchiveJSON(row)
		if err != nil {
			return nil, err
		}
		buf.Write(body)
		buf.WriteByte('\n')
	}
	if len(rows) == 0 {
		buf.WriteByte('\n')
	}
	meta, err := writeArchiveFile(path, buf.Bytes())
	if err != nil {
		return nil, err
	}
	meta["rows"] = len(rows)
	return meta, nil
}

func writeArchiveFile(path string, body []byte) (map[string]any, error) {
	tmp := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	return map[string]any{
		"sha256": hex.EncodeToString(sum[:]),
		"bytes":  len(body),
	}, nil
}

func marshalIndentedArchiveJSON(payload any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func marshalCompactArchiveJSON(payload any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func canonicalArchiveManifestSHA(manifest map[string]any) string {
	canonical := map[string]any{}
	for key, value := range manifest {
		if key == "bundle_sha256" {
			continue
		}
		canonical[key] = value
	}
	body, _ := marshalCompactArchiveJSON(canonical)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func normalizeArchiveRows(rows []map[string]any) []map[string]any {
	normalized := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		normalized = append(normalized, normalizeArchiveRow(row))
	}
	return normalized
}

func normalizeArchiveRow(row map[string]any) map[string]any {
	normalized := map[string]any{}
	for key, value := range row {
		normalized[key] = normalizeArchiveValue(value)
	}
	return normalized
}

func normalizeArchiveValue(value any) any {
	switch item := value.(type) {
	case time.Time:
		return item.UTC().Truncate(time.Second).Format(time.RFC3339)
	case []byte:
		var decoded any
		if err := json.Unmarshal(item, &decoded); err == nil {
			return decoded
		}
		return string(item)
	case map[string]any:
		out := map[string]any{}
		for key, value := range item {
			out[key] = normalizeArchiveValue(value)
		}
		return out
	case []any:
		out := make([]any, 0, len(item))
		for _, value := range item {
			out = append(out, normalizeArchiveValue(value))
		}
		return out
	default:
		return value
	}
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func displayArchivePath(repoRoot string, path string) string {
	rel, err := filepath.Rel(mustAbs(repoRoot), mustAbs(path))
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return rel
	}
	return path
}
