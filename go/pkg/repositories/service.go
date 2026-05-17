package repositories

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

const latestRepoLocalSchemaVersion = 16

type Service struct {
	Runner db.Runner
}

func (s Service) Register(server *rpc.Server) {
	server.Register("repo.add", s.Add)
	server.Register("repo.list", s.List)
	server.Register("repo.resolve", s.Resolve)
	server.Register("repo.remove", s.Remove)
}

func (s Service) Add(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "repo.add requires daemon PostgreSQL", nil)
	}
	path := stringParam(envelope.Params, "path")
	if path == "" {
		return nil, rpc.NewError("schema_invalid", "repo.add requires path", nil)
	}
	repo, err := canonicalRepo(path)
	if err != nil {
		return nil, err
	}
	if exists(repoLocalSQLitePath(repo)) {
		return nil, rpc.NewError(
			"sqlite_source_present",
			"repo-local SQLite state exists; run striatum daemon migrate-repo-local --from sqlite --to pg before registering",
			nil,
		)
	}
	stateDir, err := operationalScratch(repo, boolParam(envelope.Params, "init"))
	if err != nil {
		return nil, err
	}
	identity, err := repoIdentity(repo)
	if err != nil {
		return nil, err
	}
	existing, err := findByIdentity(ctx, s.Runner, identity, false)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		existing["already_registered"] = true
		return map[string]any{
			"repository_id":      existing["repository_id"],
			"repo_root":          existing["repo_root"],
			"repo_identity":      existing["repo_identity"],
			"state_db_path":      existing["state_db_path"],
			"schema_version":     existing["last_schema_version"],
			"state":              existing["state"],
			"already_registered": true,
		}, nil
	}
	pathExisting, err := findByRoot(ctx, s.Runner, repo, false)
	if err != nil {
		return nil, err
	}
	if pathExisting != nil {
		return nil, rpc.NewError("path_conflict", "active repository path is occupied by a different repo identity", nil)
	}
	repositoryID := "repo_" + randomHex(16)
	displayName := stringParam(envelope.Params, "display_name")
	if displayName == "" {
		displayName = filepath.Base(repo)
	}
	if err := s.Runner.Exec(ctx, `
		INSERT INTO striatumd.repositories(repository_id, repo_identity, repo_root,
		  state_db_path, display_name, registered_at, removed_at, last_seen_at,
		  last_schema_version, state, settings_json)
		VALUES ($1, $2, $3, $4, $5, now(), NULL, now(), $6, 'active', '{}'::jsonb)`,
		repositoryID,
		identity,
		repo,
		stateDir,
		displayName,
		latestRepoLocalSchemaVersion,
	); err != nil {
		return nil, err
	}
	return map[string]any{
		"repository_id":  repositoryID,
		"repo_root":      repo,
		"repo_identity":  identity,
		"state_db_path":  stateDir,
		"schema_version": latestRepoLocalSchemaVersion,
		"state":          "active",
	}, nil
}

func (s Service) List(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "repo.list requires daemon PostgreSQL", nil)
	}
	rows, err := queryRows(ctx, s.Runner, `
		SELECT repository_id, repo_identity, repo_root, display_name,
		       registered_at, removed_at, last_seen_at,
		       last_schema_version, state, state_db_path
		  FROM striatumd.repositories
		 ORDER BY registered_at, repository_id`)
	if err != nil {
		return nil, err
	}
	return map[string]any{"repositories": rows}, nil
}

func (s Service) Resolve(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "repo.resolve requires daemon PostgreSQL", nil)
	}
	path := stringParam(envelope.Params, "path")
	if path == "" {
		path = stringParam(envelope.Params, "repo_root")
	}
	if path == "" {
		return nil, rpc.NewError("schema_invalid", "repo.resolve requires path", nil)
	}
	repo, err := canonicalRepo(path)
	if err != nil {
		return nil, err
	}
	row, err := findByRoot(ctx, s.Runner, repo, false)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, rpc.NewError("repo_not_registered", fmt.Sprintf("active repository path is not registered: %s", repo), nil)
	}
	return publicRepository(row), nil
}

func (s Service) Remove(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "repo.remove requires daemon PostgreSQL", nil)
	}
	identifier := stringParam(envelope.Params, "id")
	if identifier == "" {
		identifier = stringParam(envelope.Params, "repository_id")
	}
	if identifier == "" {
		return nil, rpc.NewError("schema_invalid", "repo.remove requires id", nil)
	}
	row, err := findRepository(ctx, s.Runner, identifier, true)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, rpc.NewError("not_found", fmt.Sprintf("unknown daemon repository %q", identifier), nil)
	}
	repositoryID := fmt.Sprint(row["repository_id"])
	if row["state"] == "removed" {
		return map[string]any{"repository_id": repositoryID, "state": "removed", "already_removed": true}, nil
	}
	if err := s.Runner.Exec(ctx, `
		UPDATE striatumd.repositories
		   SET state = 'removed', removed_at = now()
		 WHERE repository_id = $1`, repositoryID); err != nil {
		return nil, err
	}
	revoked, err := queryCount(ctx, s.Runner, `
		WITH revoked AS (
		  UPDATE striatumd.client_capabilities
		     SET revoked_at = now(), revoked_reason = 'repo_removed'
		   WHERE repository_id = $1 AND revoked_at IS NULL
		   RETURNING 1
		)
		SELECT COUNT(*) FROM revoked`, repositoryID)
	if err != nil {
		return nil, err
	}
	if err := s.Runner.Exec(ctx, `
		UPDATE striatumd.scheduler_cursors
		   SET state = 'removed'
		 WHERE repository_id = $1`, repositoryID); err != nil {
		return nil, err
	}
	return map[string]any{"repository_id": repositoryID, "state": "removed", "revoked_capabilities": revoked}, nil
}

func findRepository(ctx context.Context, runner db.Runner, identifier string, includeRemoved bool) (map[string]any, error) {
	if row, err := findByID(ctx, runner, identifier, includeRemoved); err != nil || row != nil {
		return row, err
	}
	repo, err := canonicalRepo(identifier)
	if err != nil {
		repo = identifier
	}
	return findByRoot(ctx, runner, repo, includeRemoved)
}

func findByID(ctx context.Context, runner db.Runner, repositoryID string, includeRemoved bool) (map[string]any, error) {
	where := ""
	if !includeRemoved {
		where = " AND state != 'removed'"
	}
	rows, err := queryRows(ctx, runner, `SELECT * FROM striatumd.repositories WHERE repository_id = $1`+where, repositoryID)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func findByIdentity(ctx context.Context, runner db.Runner, identity string, includeRemoved bool) (map[string]any, error) {
	where := ""
	if !includeRemoved {
		where = " AND state != 'removed'"
	}
	rows, err := queryRows(ctx, runner, `SELECT * FROM striatumd.repositories WHERE repo_identity = $1`+where+` ORDER BY repository_id LIMIT 1`, identity)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func findByRoot(ctx context.Context, runner db.Runner, repo string, includeRemoved bool) (map[string]any, error) {
	where := ""
	if !includeRemoved {
		where = " AND state != 'removed'"
	}
	rows, err := queryRows(ctx, runner, `SELECT * FROM striatumd.repositories WHERE repo_root = $1`+where+` ORDER BY repository_id LIMIT 1`, repo)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func publicRepository(row map[string]any) map[string]any {
	return map[string]any{
		"repository_id":  row["repository_id"],
		"repo_root":      row["repo_root"],
		"repo_identity":  row["repo_identity"],
		"state_db_path":  row["state_db_path"],
		"display_name":   row["display_name"],
		"registered_at":  row["registered_at"],
		"removed_at":     row["removed_at"],
		"last_seen_at":   row["last_seen_at"],
		"schema_version": row["last_schema_version"],
		"state":          row["state"],
	}
}

func queryRows(ctx context.Context, runner db.Runner, sql string, args ...any) ([]map[string]any, error) {
	type queryer interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}
	q, ok := runner.(queryer)
	if !ok {
		return nil, fmt.Errorf("runner does not support row queries")
	}
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToMap)
}

func queryCount(ctx context.Context, runner db.Runner, sql string, args ...any) (int64, error) {
	var count int64
	if err := runner.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func canonicalRepo(value string) (string, error) {
	if hasParentTraversal(value) {
		return "", rpc.NewError("path_traversal", "repo path traversal is not allowed", nil)
	}
	expanded, err := expandHome(value)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(wd, expanded)
	}
	if hasSymlinkComponent(expanded) {
		return "", rpc.NewError("symlink_refused", "repo registration refuses symlink paths", nil)
	}
	resolved, err := filepath.EvalSymlinks(expanded)
	if err != nil {
		return "", rpc.NewError("repo_not_found", "repo path does not exist", map[string]any{"path": expanded})
	}
	stateDir := filepath.Join(resolved, ".striatum")
	if hasSymlinkComponent(stateDir) {
		return "", rpc.NewError("symlink_refused", "repo scratch directory symlink is not allowed", nil)
	}
	return resolved, nil
}

func operationalScratch(repo string, init bool) (string, error) {
	stateDir := filepath.Join(repo, ".striatum")
	if hasSymlinkComponent(stateDir) {
		return "", rpc.NewError("symlink_refused", "repo scratch directory symlink is not allowed", nil)
	}
	if init {
		if err := os.MkdirAll(filepath.Join(stateDir, "scratch"), 0o700); err != nil {
			return "", err
		}
		_ = os.Chmod(stateDir, 0o700)
		_ = os.Chmod(filepath.Join(stateDir, "scratch"), 0o700)
		if err := ensureGitignore(repo); err != nil {
			return "", err
		}
		return stateDir, nil
	}
	info, err := os.Stat(stateDir)
	if err != nil || !info.IsDir() {
		return "", rpc.NewError("repo_scratch_missing", "repo scratch is not initialized; rerun repo add with --init or run striatum init first", nil)
	}
	return stateDir, nil
}

func ensureGitignore(repo string) error {
	path := filepath.Join(repo, ".gitignore")
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		if line == ".striatum/" {
			return nil
		}
	}
	prefix := ""
	if len(body) > 0 && !strings.HasSuffix(string(body), "\n") {
		prefix = "\n"
	}
	return os.WriteFile(path, []byte(string(body)+prefix+".striatum/\n"), 0o644)
}

func repoLocalSQLitePath(repo string) string {
	return filepath.Join(repo, ".striatum", "state.sqlite3")
}

func repoIdentity(repo string) (string, error) {
	info, err := os.Stat(repo)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("repo stat is not syscall.Stat_t")
	}
	return fmt.Sprintf("inode:%d:%d:root:%s", stat.Dev, stat.Ino, repo), nil
}

func hasParentTraversal(value string) bool {
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".." {
			return true
		}
	}
	return false
}

func hasSymlinkComponent(path string) bool {
	current := ""
	if filepath.IsAbs(path) {
		current = string(os.PathSeparator)
	}
	for _, part := range strings.Split(filepath.Clean(path), string(os.PathSeparator)) {
		if part == "" || part == string(os.PathSeparator) {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true
		}
	}
	return false
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func expandHome(value string) (string, error) {
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(value, "~/")), nil
	}
	return value, nil
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func stringParam(params map[string]any, key string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return ""
}

func boolParam(params map[string]any, key string) bool {
	if value, ok := params[key].(bool); ok {
		return value
	}
	return false
}
