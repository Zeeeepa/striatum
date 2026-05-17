package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

const latestRepoLocalSchemaVersion = 16

type repoRegistration struct {
	RepositoryID  string
	RepoRoot      string
	RepoIdentity  string
	StateDBPath   string
	SchemaVersion int
	State         string
}

func (s Service) RepoInit(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.Runner == nil {
		return nil, rpc.NewError("daemon_db_missing", "repo.init requires daemon PostgreSQL", nil)
	}
	repo, err := s.resolveRepoForInit(ctx, envelope.Params)
	if err != nil {
		return nil, err
	}
	if exists(filepath.Join(repo, ".striatum", "state.sqlite3")) {
		return nil, rpc.NewError(
			"sqlite_retired",
			"repo.init refuses repo-local SQLite state; register PostgreSQL-backed daemon state instead",
			map[string]any{
				"path":              filepath.Join(repo, ".striatum", "state.sqlite3"),
				"python_dependency": false,
				"sqlite_dependency": false,
			},
		)
	}
	stateDir, err := initOperationalScratch(repo)
	if err != nil {
		return nil, err
	}
	identity, err := repoIdentity(repo)
	if err != nil {
		return nil, err
	}
	existing, err := s.findInitRegistration(ctx, "repo_identity", identity)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return repoInitResult(*existing, true), nil
	}
	existing, err = s.findInitRegistration(ctx, "repo_root", repo)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, rpc.NewError("path_conflict", "active repository path is occupied by a different repo identity", nil)
	}

	repositoryID := stringParam(envelope.Params, "repository_id")
	if repositoryID == "" {
		repositoryID = "repo_" + randomHex(16)
	}
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
	return repoInitResult(repoRegistration{
		RepositoryID:  repositoryID,
		RepoRoot:      repo,
		RepoIdentity:  identity,
		StateDBPath:   stateDir,
		SchemaVersion: latestRepoLocalSchemaVersion,
		State:         "active",
	}, false), nil
}

func (s Service) resolveRepoForInit(ctx context.Context, params map[string]any) (string, error) {
	path := stringParam(params, "path")
	if path == "" {
		path = stringParam(params, "repo_root")
	}
	if path != "" {
		return canonicalRepo(path)
	}
	repositoryID := stringParam(params, "repository_id")
	if repositoryID == "" {
		return "", rpc.NewError("schema_invalid", "repo.init requires path or repository_id", nil)
	}
	existing, err := s.findInitRegistration(ctx, "repository_id", repositoryID)
	if err != nil {
		return "", err
	}
	if existing == nil {
		return "", rpc.NewError("repo_not_registered", "repo.init repository_id is not registered", nil)
	}
	return canonicalRepo(existing.RepoRoot)
}

func (s Service) findInitRegistration(ctx context.Context, field string, value string) (*repoRegistration, error) {
	if value == "" {
		return nil, nil
	}
	where := "repository_id = $1"
	switch field {
	case "repo_identity":
		where = "repo_identity = $1"
	case "repo_root":
		where = "repo_root = $1"
	}
	row := s.Runner.QueryRow(ctx, `
		SELECT repository_id, repo_root, repo_identity, state_db_path,
		       last_schema_version, state
		  FROM striatumd.repositories
		 WHERE `+where+` AND state != 'removed'
		 ORDER BY repository_id
		 LIMIT 1`, value)
	var found repoRegistration
	err := row.Scan(
		&found.RepositoryID,
		&found.RepoRoot,
		&found.RepoIdentity,
		&found.StateDBPath,
		&found.SchemaVersion,
		&found.State,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &found, nil
}

func repoInitResult(row repoRegistration, already bool) map[string]any {
	return map[string]any{
		"repository_id":      row.RepositoryID,
		"repo_root":          row.RepoRoot,
		"repo_identity":      row.RepoIdentity,
		"state_db_path":      row.StateDBPath,
		"schema_version":     row.SchemaVersion,
		"state":              row.State,
		"already_registered": already,
		"substrate":          "postgres",
		"sqlite_dependency":  false,
		"python_dependency":  false,
		"source_import_mode": "none",
	}
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

func initOperationalScratch(repo string) (string, error) {
	stateDir := filepath.Join(repo, ".striatum")
	if hasSymlinkComponent(stateDir) {
		return "", rpc.NewError("symlink_refused", "repo scratch directory symlink is not allowed", nil)
	}
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

func ensureGitignore(repo string) error {
	path := filepath.Join(repo, ".gitignore")
	body, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(body), "\n") {
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
