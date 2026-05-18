package crossrepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
)

type PrepareRunner interface {
	Prepare(ctx context.Context, repositoryID string, alias string, crossRepoRunID string) (string, error)
}

type StartRunner interface {
	Start(ctx context.Context, repositoryID string, localRunID string) error
}

type CancelRunner interface {
	Cancel(ctx context.Context, repositoryID string, localRunID string, reason string) error
}

func PrepareRun(ctx context.Context, runner db.Runner, workflow map[string]any, local PrepareRunner, crossRepoRunID string) (map[string]any, error) {
	repositories, primaryAlias, err := repositories(workflow)
	if err != nil {
		return nil, err
	}
	if crossRepoRunID == "" {
		crossRepoRunID = "xrun_" + digest(fmt.Sprintf("%d", time.Now().UnixNano()))[:24]
	}
	body, _ := json.Marshal(workflow)
	now := time.Now().UTC()
	if err := runner.Exec(
		ctx,
		`INSERT INTO striatumd.cross_repo_runs (
		   cross_repo_run_id, workflow_id, workflow_version,
		   workflow_snapshot_hash, primary_repository_id, state, created_at,
		   reconcile_after
		 ) VALUES ($1, $2, $3, $4, $5, 'preparing', $6, $7)`,
		crossRepoRunID,
		stringValue(workflow["workflow_id"]),
		nullableString(workflow["workflow_version"]),
		digest(string(body)),
		repositories[primaryAlias],
		now,
		now,
	); err != nil {
		return nil, err
	}
	aliases := sortedKeys(repositories)
	for _, alias := range aliases {
		if err := runner.Exec(
			ctx,
			`INSERT INTO striatumd.cross_repo_run_repositories (
			  cross_repo_run_id, repository_alias, repository_id, state, last_observed_at
			) VALUES ($1, $2, $3, 'preparing', $4)`,
			crossRepoRunID,
			alias,
			repositories[alias],
			now,
		); err != nil {
			return nil, err
		}
	}
	participants := []map[string]any{}
	for _, alias := range aliases {
		localRunID, err := local.Prepare(ctx, repositories[alias], alias, crossRepoRunID)
		if err != nil {
			_ = runner.Exec(ctx, "UPDATE striatumd.cross_repo_runs SET state = 'aborted', last_reconcile_error = $1 WHERE cross_repo_run_id = $2", err.Error(), crossRepoRunID)
			return nil, err
		}
		if err := runner.Exec(
			ctx,
			`UPDATE striatumd.cross_repo_run_repositories
			    SET local_run_id = $1, state = 'prepared', prepared_at = $2, last_observed_at = $3
			  WHERE cross_repo_run_id = $4 AND repository_alias = $5`,
			localRunID,
			time.Now().UTC(),
			time.Now().UTC(),
			crossRepoRunID,
			alias,
		); err != nil {
			return nil, err
		}
		participants = append(participants, map[string]any{"repository_alias": alias, "repository_id": repositories[alias], "local_run_id": localRunID})
	}
	if err := runner.Exec(ctx, "UPDATE striatumd.cross_repo_runs SET state = 'prepared' WHERE cross_repo_run_id = $1", crossRepoRunID); err != nil {
		return nil, err
	}
	return map[string]any{"cross_repo_run_id": crossRepoRunID, "state": "prepared", "participants": participants}, nil
}

func repositories(workflow map[string]any) (map[string]string, string, error) {
	raw, ok := workflow["repositories"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil, "", fmt.Errorf("workflow is not a cross-repo workflow")
	}
	result := map[string]string{}
	for alias, body := range raw {
		repo, ok := body.(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("workflow repositories block is invalid")
		}
		result[alias] = stringValue(repo["repo_id"])
	}
	primary := stringValue(workflow["primary_repository"])
	if _, ok := result[primary]; !ok {
		return nil, "", fmt.Errorf("cross-repo workflow primary_repository is invalid")
	}
	return result, primary, nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func nullableString(value any) any {
	if value == nil {
		return nil
	}
	return fmt.Sprint(value)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
