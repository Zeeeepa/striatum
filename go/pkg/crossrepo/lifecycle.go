package crossrepo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

type Participant struct {
	RepositoryAlias string  `json:"repository_alias"`
	RepositoryID    string  `json:"repository_id"`
	LocalRunID      *string `json:"local_run_id"`
	State           string  `json:"state"`
}

func ListRuns(ctx context.Context, runner db.Runner) (map[string]any, error) {
	rows, err := queryRows(ctx, runner, `SELECT cross_repo_run_id, workflow_id, workflow_version, primary_repository_id, state FROM striatumd.cross_repo_runs ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	return map[string]any{"cross_repo_runs": rows}, nil
}

func DescribeRun(ctx context.Context, runner db.Runner, crossRepoRunID string) (map[string]any, error) {
	run, err := crossRepoRun(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	participants, err := participants(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"cross_repo_run_id":     crossRepoRunID,
		"workflow_id":           run["workflow_id"],
		"workflow_version":      run["workflow_version"],
		"primary_repository_id": run["primary_repository_id"],
		"state":                 run["state"],
		"participants":          participants,
	}, nil
}

func Why(ctx context.Context, runner db.Runner, crossRepoRunID string) (map[string]any, error) {
	described, err := DescribeRun(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"cross_repo_run_id": crossRepoRunID,
		"state":             described["state"],
		"participants":      described["participants"],
	}, nil
}

func StartRun(ctx context.Context, runner db.Runner, crossRepoRunID string, local LocalRunner) (map[string]any, error) {
	run, err := crossRepoRun(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	if run["state"] != "prepared" {
		return nil, fmt.Errorf("cross-repo run %s must be prepared before start", crossRepoRunID)
	}
	items, err := participants(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	started := []string{}
	for _, item := range items {
		if item.LocalRunID == nil {
			return nil, fmt.Errorf("participant %s has no local_run_id", item.RepositoryAlias)
		}
		if err := local.Start(ctx, item.RepositoryID, *item.LocalRunID); err != nil {
			_ = runner.Exec(ctx, "UPDATE striatumd.cross_repo_runs SET state = 'blocked', last_reconcile_error = $1 WHERE cross_repo_run_id = $2", err.Error(), crossRepoRunID)
			return map[string]any{"cross_repo_run_id": crossRepoRunID, "state": "blocked", "started": started}, nil
		}
		started = append(started, item.RepositoryAlias)
		if err := updateParticipant(ctx, runner, crossRepoRunID, item.RepositoryAlias, "running"); err != nil {
			return nil, err
		}
	}
	if err := runner.Exec(ctx, "UPDATE striatumd.cross_repo_runs SET state = 'running', started_at = $1 WHERE cross_repo_run_id = $2", time.Now().UTC(), crossRepoRunID); err != nil {
		return nil, err
	}
	return map[string]any{"cross_repo_run_id": crossRepoRunID, "state": "running", "started": started}, nil
}

func CancelRun(ctx context.Context, runner db.Runner, crossRepoRunID string, reason string, local LocalRunner) (map[string]any, error) {
	run, err := crossRepoRun(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	if terminal(fmt.Sprint(run["state"])) {
		return map[string]any{"cross_repo_run_id": crossRepoRunID, "state": run["state"], "canceled": []string{}}, nil
	}
	if err := runner.Exec(ctx, "UPDATE striatumd.cross_repo_runs SET state = 'canceling' WHERE cross_repo_run_id = $1", crossRepoRunID); err != nil {
		return nil, err
	}
	items, err := participants(ctx, runner, crossRepoRunID)
	if err != nil {
		return nil, err
	}
	canceled := []string{}
	blocked := []string{}
	blockedErrors := map[string]string{}
	skippedTerminal := []string{}
	skippedNoLocalRun := []string{}
	for _, item := range items {
		if terminal(item.State) {
			skippedTerminal = append(skippedTerminal, item.RepositoryAlias)
			continue
		}
		if item.LocalRunID == nil {
			if item.State == "preparing" {
				skippedNoLocalRun = append(skippedNoLocalRun, item.RepositoryAlias)
				if err := updateParticipant(ctx, runner, crossRepoRunID, item.RepositoryAlias, "canceled"); err != nil {
					return nil, err
				}
				continue
			}
			blocked = append(blocked, item.RepositoryAlias)
			blockedErrors[item.RepositoryAlias] = fmt.Sprintf("participant %s has no local_run_id", item.RepositoryAlias)
			_ = updateParticipant(ctx, runner, crossRepoRunID, item.RepositoryAlias, "blocked")
			continue
		}
		if err := local.Cancel(ctx, item.RepositoryID, *item.LocalRunID, reason); err != nil {
			blocked = append(blocked, item.RepositoryAlias)
			blockedErrors[item.RepositoryAlias] = err.Error()
			_ = updateParticipant(ctx, runner, crossRepoRunID, item.RepositoryAlias, "blocked")
			continue
		}
		canceled = append(canceled, item.RepositoryAlias)
		if err := updateParticipant(ctx, runner, crossRepoRunID, item.RepositoryAlias, "canceled"); err != nil {
			return nil, err
		}
	}
	state := "canceled"
	if len(blocked) > 0 {
		state = "blocked"
	}
	var lastError any
	if len(blockedErrors) > 0 {
		lastError = joinBlockedErrors(blockedErrors)
	}
	if err := runner.Exec(ctx, "UPDATE striatumd.cross_repo_runs SET state = $1, canceled_at = CASE WHEN $2 = 'canceled' THEN $3 ELSE canceled_at END, last_reconcile_error = $4 WHERE cross_repo_run_id = $5", state, state, time.Now().UTC(), lastError, crossRepoRunID); err != nil {
		return nil, err
	}
	result := map[string]any{"cross_repo_run_id": crossRepoRunID, "state": state, "canceled": canceled, "blocked": blocked}
	if len(blockedErrors) > 0 {
		result["blocked_errors"] = blockedErrors
	}
	if len(skippedTerminal) > 0 {
		result["skipped_terminal"] = skippedTerminal
	}
	if len(skippedNoLocalRun) > 0 {
		result["skipped_no_local_run"] = skippedNoLocalRun
	}
	return result, nil
}

func crossRepoRun(ctx context.Context, runner db.Runner, crossRepoRunID string) (map[string]any, error) {
	var workflowID, primaryID, state string
	var workflowVersion *string
	err := runner.QueryRow(ctx, "SELECT workflow_id, workflow_version, primary_repository_id, state FROM striatumd.cross_repo_runs WHERE cross_repo_run_id = $1", crossRepoRunID).Scan(&workflowID, &workflowVersion, &primaryID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("cross-repo run not found: %s", crossRepoRunID)
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"workflow_id": workflowID, "workflow_version": workflowVersion, "primary_repository_id": primaryID, "state": state}, nil
}

func participants(ctx context.Context, runner db.Runner, crossRepoRunID string) ([]Participant, error) {
	type queryer interface {
		Query(context.Context, string, ...any) (pgx.Rows, error)
	}
	q, ok := runner.(queryer)
	if !ok {
		return nil, fmt.Errorf("runner does not support row queries")
	}
	rows, err := q.Query(ctx, "SELECT repository_alias, repository_id, local_run_id, state FROM striatumd.cross_repo_run_repositories WHERE cross_repo_run_id = $1 ORDER BY repository_alias", crossRepoRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Participant{}
	for rows.Next() {
		var item Participant
		if err := rows.Scan(&item.RepositoryAlias, &item.RepositoryID, &item.LocalRunID, &item.State); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
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

func updateParticipant(ctx context.Context, runner db.Runner, runID string, alias string, state string) error {
	return runner.Exec(ctx, "UPDATE striatumd.cross_repo_run_repositories SET state = $1, last_observed_at = $2 WHERE cross_repo_run_id = $3 AND repository_alias = $4", state, time.Now().UTC(), runID, alias)
}

func terminal(state string) bool {
	return state == "canceled" || state == "completed" || state == "failed" || state == "aborted"
}

func joinBlockedErrors(blockedErrors map[string]string) string {
	aliases := make([]string, 0, len(blockedErrors))
	for alias := range blockedErrors {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	parts := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		parts = append(parts, fmt.Sprintf("%s: %s", alias, blockedErrors[alias]))
	}
	return strings.Join(parts, "; ")
}
