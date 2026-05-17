package recovery

import (
	"context"
	"fmt"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/mutations"
	"github.com/jackc/pgx/v5"
)

type queryRunner interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type ActiveRunSweep struct {
	Runner db.Runner
	Author string
}

func (s ActiveRunSweep) SweepOnce(ctx context.Context) (map[string]any, error) {
	if s.Runner == nil {
		return nil, fmt.Errorf("recovery sweep requires daemon PostgreSQL")
	}
	queryer, ok := s.Runner.(queryRunner)
	if !ok {
		return nil, fmt.Errorf("recovery sweep runner does not support queries")
	}
	author := s.Author
	if author == "" {
		author = "striatumd-go"
	}

	rows, err := queryer.Query(ctx, `
		SELECT r.repository_id, runs.run_id
		  FROM striatumd.repositories r
		  JOIN striatumd.runs runs
		    ON runs.repository_id = r.repository_id
		 WHERE r.state = 'active'
		   AND runs.state IN ('running', 'paused')
		 ORDER BY r.repository_id, runs.created_at, runs.run_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type activeRun struct {
		repositoryID string
		runID        string
	}
	activeRuns := []activeRun{}
	for rows.Next() {
		var row activeRun
		if err := rows.Scan(&row.repositoryID, &row.runID); err != nil {
			return nil, err
		}
		activeRuns = append(activeRuns, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sweeps := []map[string]any{}
	for _, run := range activeRuns {
		result, err := mutations.SweepRun(ctx, s.Runner, run.repositoryID, run.runID, author)
		if err != nil {
			result = map[string]any{"error": err.Error()}
			if cursorErr := upsertSchedulerCursor(ctx, s.Runner, run.repositoryID, run.runID, result, "sweep_degraded"); cursorErr != nil {
				return nil, cursorErr
			}
			sweeps = append(sweeps, map[string]any{
				"repository_id": run.repositoryID,
				"run_id":        run.runID,
				"error":         err.Error(),
			})
			continue
		}
		if err := upsertSchedulerCursor(ctx, s.Runner, run.repositoryID, run.runID, result, "active"); err != nil {
			return nil, err
		}
		sweeps = append(sweeps, map[string]any{
			"repository_id": run.repositoryID,
			"run_id":        run.runID,
			"result":        result,
		})
	}
	return map[string]any{"mode": "daemon", "sweeps": sweeps}, nil
}

func upsertSchedulerCursor(ctx context.Context, runner db.Runner, repositoryID string, runID string, result map[string]any, state string) error {
	return runner.Exec(ctx, `
		INSERT INTO striatumd.scheduler_cursors(
		  repository_id, run_id, cursor_kind, last_sweep_at,
		  next_sweep_after, last_result_json, state
		)
		VALUES ($1, $2, 'recovery', now(), NULL, $3, $4)
		ON CONFLICT (repository_id, run_id, cursor_kind)
		DO UPDATE SET last_sweep_at = now(),
		              next_sweep_after = NULL,
		              last_result_json = EXCLUDED.last_result_json,
		              state = EXCLUDED.state`,
		repositoryID, runID, result, state)
}
