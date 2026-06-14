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

// AutoSpawnSweep is the daemon-side supervision.auto_spawn scheduler (RFC 0122):
// each tick it reconciles every running run that holds an active spawn-
// authorization grant, registering + supervising the queued auto_spawn lanes
// under the captured run owner. It is the standing process that finally removes
// the operator (model AND credential) from the spawn loop — the residual
// operator-side `run drive` could not close (RFC 0122 §Problem). Like
// ActiveRunSweep it runs on the resident scheduler interval and records a per-run
// scheduler cursor so a degraded run is visible, not silent.
type AutoSpawnSweep struct {
	Runner db.Runner
	Author string
}

func (s AutoSpawnSweep) SweepOnce(ctx context.Context) (map[string]any, error) {
	if s.Runner == nil {
		return nil, fmt.Errorf("auto_spawn sweep requires daemon PostgreSQL")
	}
	queryer, ok := s.Runner.(queryRunner)
	if !ok {
		return nil, fmt.Errorf("auto_spawn sweep runner does not support queries")
	}
	author := s.Author
	if author == "" {
		author = "striatumd-go"
	}

	// Only runs with an ACTIVE grant are candidates — a run with no grant is not
	// auto_spawn-authorized and must never be touched by the scheduler (C2/C6).
	rows, err := queryer.Query(ctx, `
		SELECT DISTINCT r.repository_id, runs.run_id, runs.created_at
		  FROM striatumd.repositories r
		  JOIN striatumd.runs runs
		    ON runs.repository_id = r.repository_id
		  JOIN striatumd.spawn_authorization_grants g
		    ON g.repository_id = runs.repository_id
		   AND g.run_id = runs.run_id
		   AND g.revoked_at IS NULL
		 WHERE r.state = 'active'
		   AND runs.state = 'running'
		 ORDER BY r.repository_id, runs.created_at, runs.run_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		repositoryID string
		runID        string
	}
	candidates := []candidate{}
	for rows.Next() {
		var c candidate
		var createdAt any
		if err := rows.Scan(&c.repositoryID, &c.runID, &createdAt); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sweeps := []map[string]any{}
	for _, c := range candidates {
		result, err := mutations.SchedulerSpawnOnce(ctx, s.Runner, c.repositoryID, c.runID, author)
		if err != nil {
			// A poisoned spawn (expired/missing grant, run-as failure) is recorded
			// loud as a degraded cursor + surfaced in the sweep result; the scheduler
			// never silently falls back (C2).
			result = map[string]any{"error": err.Error()}
			if cursorErr := upsertAutoSpawnCursor(ctx, s.Runner, c.repositoryID, c.runID, result, "spawn_degraded"); cursorErr != nil {
				return nil, cursorErr
			}
			sweeps = append(sweeps, map[string]any{
				"repository_id": c.repositoryID,
				"run_id":        c.runID,
				"error":         err.Error(),
			})
			continue
		}
		if err := upsertAutoSpawnCursor(ctx, s.Runner, c.repositoryID, c.runID, result, "active"); err != nil {
			return nil, err
		}
		sweeps = append(sweeps, map[string]any{
			"repository_id": c.repositoryID,
			"run_id":        c.runID,
			"result":        result,
		})
	}
	return map[string]any{"mode": "auto_spawn", "sweeps": sweeps}, nil
}

func upsertAutoSpawnCursor(ctx context.Context, runner db.Runner, repositoryID string, runID string, result map[string]any, state string) error {
	resultArg, err := db.JSONBArg(runner, result)
	if err != nil {
		return err
	}
	return runner.Exec(ctx, `
		INSERT INTO striatumd.scheduler_cursors(
		  repository_id, run_id, cursor_kind, last_sweep_at,
		  next_sweep_after, last_result_json, state
		)
		VALUES ($1, $2, 'auto_spawn', now(), NULL, $3::jsonb, $4)
		ON CONFLICT (repository_id, run_id, cursor_kind)
		DO UPDATE SET last_sweep_at = now(),
		              next_sweep_after = NULL,
		              last_result_json = EXCLUDED.last_result_json,
		              state = EXCLUDED.state`,
		repositoryID, runID, resultArg, state)
}

func upsertSchedulerCursor(ctx context.Context, runner db.Runner, repositoryID string, runID string, result map[string]any, state string) error {
	resultArg, err := db.JSONBArg(runner, result)
	if err != nil {
		return err
	}
	return runner.Exec(ctx, `
		INSERT INTO striatumd.scheduler_cursors(
		  repository_id, run_id, cursor_kind, last_sweep_at,
		  next_sweep_after, last_result_json, state
		)
		VALUES ($1, $2, 'recovery', now(), NULL, $3::jsonb, $4)
		ON CONFLICT (repository_id, run_id, cursor_kind)
		DO UPDATE SET last_sweep_at = now(),
		              next_sweep_after = NULL,
		              last_result_json = EXCLUDED.last_result_json,
		              state = EXCLUDED.state`,
		repositoryID, runID, resultArg, state)
}
