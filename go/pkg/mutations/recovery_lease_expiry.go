package mutations

import (
	"context"
	"errors"
	"fmt"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

type worktreeAnchorOracleKey struct{}

type worktreeAnchorOracle struct {
	byWorktreeID map[string]map[string]any
}

func withWorktreeAnchorOracle(ctx context.Context, oracle *worktreeAnchorOracle) context.Context {
	return context.WithValue(ctx, worktreeAnchorOracleKey{}, oracle)
}

func worktreeAnchorOracleFromContext(ctx context.Context) *worktreeAnchorOracle {
	oracle, _ := ctx.Value(worktreeAnchorOracleKey{}).(*worktreeAnchorOracle)
	return oracle
}

func (o *worktreeAnchorOracle) lookup(worktreeID string) (map[string]any, bool) {
	if o == nil || o.byWorktreeID == nil {
		return nil, false
	}
	payload, ok := o.byWorktreeID[worktreeID]
	return payload, ok
}

// buildRunWorktreeAnchorOracle anchors expired repo-write worktrees before the
// sweep transaction opens. The in-transaction path may then abandon the worktree
// row and emit provenance without running git under lockRun.
func buildRunWorktreeAnchorOracle(ctx context.Context, runner db.Runner, repositoryID, runID string) (*worktreeAnchorOracle, error) {
	oracle := &worktreeAnchorOracle{byWorktreeID: map[string]map[string]any{}}
	rows, err := queryRows(ctx, runner, `
		SELECT j.job_id, j.attempt,
		       wt.worktree_id, wt.worktree_path, wt.base_branch
		  FROM striatumd.leases l
		  JOIN striatumd.jobs j
		    ON j.repository_id = l.repository_id
		   AND j.job_id = l.resource_id
		  JOIN striatumd.job_worktrees wt
		    ON wt.repository_id = j.repository_id
		   AND wt.job_id = j.job_id
		   AND wt.lease_id = l.lease_id
		   AND wt.state = 'active'
		 WHERE l.repository_id = $1
		   AND l.run_id = $2
		   AND l.state = 'active'
		   AND l.expires_at < $3::timestamptz
		 ORDER BY wt.worktree_id`,
		repositoryID,
		runID,
		nowString(),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return oracle, nil
	}
	repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		worktreeID := fmt.Sprint(row["worktree_id"])
		worktree := map[string]any{
			"worktree_id":   worktreeID,
			"worktree_path": row["worktree_path"],
			"base_branch":   row["base_branch"],
		}
		payload, err := anchorWorktreeCommitStack(ctx, repoRoot, runID, fmt.Sprint(row["job_id"]), fmt.Sprint(row["base_branch"]), worktree, intValue(row["attempt"]))
		if err != nil {
			return nil, err
		}
		oracle.byWorktreeID[worktreeID] = payload
	}
	return oracle, nil
}

func expireLeases(ctx context.Context, runner any, repositoryID, runID string) ([]map[string]any, error) {
	now := nowString()
	rows, err := queryRows(ctx, runner, `
		SELECT * FROM striatumd.leases
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND state = 'active'
		   AND expires_at < $3::timestamptz
		 FOR UPDATE`, repositoryID, runID, now)
	if err != nil {
		return nil, err
	}
	exec, ok := runner.(interface {
		Exec(context.Context, string, ...any) error
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support exec")
	}
	summaries := []map[string]any{}
	for _, lease := range rows {
		job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", fmt.Sprint(lease["resource_id"]), true)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		messageID := nullable(job["current_message_id"])
		repoWrite := isRepoWrite(job)
		jobState := "queued"
		messageState := "pending"
		if repoWrite {
			jobState = "stale_lease"
			messageState = "blocked"
		}
		if err := exec.Exec(ctx, `
			UPDATE striatumd.leases
			   SET state = 'expired', released_at = $1, release_reason = 'expired'
			 WHERE repository_id = $2 AND lease_id = $3`, now, repositoryID, lease["lease_id"]); err != nil {
			return nil, err
		}
		if err := exec.Exec(ctx, `
			UPDATE striatumd.jobs
			   SET state = $1, current_lease_id = NULL
			 WHERE repository_id = $2 AND job_id = $3`, jobState, repositoryID, job["job_id"]); err != nil {
			return nil, err
		}
		if messageID != nil {
			if err := exec.Exec(ctx, `
				UPDATE striatumd.queue_messages
				   SET state = $1, current_lease_id = NULL, updated_at = $2
				 WHERE repository_id = $3 AND message_id = $4`, messageState, now, repositoryID, messageID); err != nil {
				return nil, err
			}
		}
		if _, err := appendEvent(ctx, runner, repositoryID, runID, "lease.expired", nil, job["job_id"], messageID, nil, lease["lease_id"], map[string]any{
			"job_state":     jobState,
			"message_state": messageState,
		}); err != nil {
			return nil, err
		}
		worktrees, err := queryRows(ctx, runner, `
			SELECT *
			  FROM striatumd.job_worktrees
			 WHERE repository_id = $1
			   AND job_id = $2
			   AND state = 'active'
			 FOR UPDATE`, repositoryID, job["job_id"])
		if err != nil {
			return nil, err
		}
		for _, worktree := range worktrees {
			if fmt.Sprint(worktree["lease_id"]) != fmt.Sprint(lease["lease_id"]) {
				continue
			}
			anchorOracle := worktreeAnchorOracleFromContext(ctx)
			anchorPayload, ok := anchorOracle.lookup(fmt.Sprint(worktree["worktree_id"]))
			if !ok {
				if anchorOracle != nil {
					return nil, fmt.Errorf("missing precomputed worktree anchor for %s", worktree["worktree_id"])
				}
				repoRoot, err := activeRepositoryRoot(ctx, runner, repositoryID)
				if err != nil {
					return nil, err
				}
				anchorPayload, err = anchorWorktreeCommitStack(ctx, repoRoot, runID, fmt.Sprint(job["job_id"]), fmt.Sprint(worktree["base_branch"]), worktree, intValue(job["attempt"]))
				if err != nil {
					return nil, err
				}
			}
			if err := exec.Exec(ctx, `
				UPDATE striatumd.job_worktrees
				   SET state = 'abandoned'
				 WHERE repository_id = $1 AND worktree_id = $2`, repositoryID, worktree["worktree_id"]); err != nil {
				return nil, err
			}
			if _, err := appendEvent(ctx, runner, repositoryID, runID, "worktree.abandoned", nil, job["job_id"], nil, nil, lease["lease_id"], map[string]any{
				"worktree_id": fmt.Sprint(worktree["worktree_id"]),
				"base_branch": worktree["base_branch"],
				"anchor":      anchorPayload,
			}); err != nil {
				return nil, err
			}
		}
		summaries = append(summaries, map[string]any{
			"lease_id":      fmt.Sprint(lease["lease_id"]),
			"job_id":        fmt.Sprint(job["job_id"]),
			"message_id":    nullable(messageID),
			"job_state":     jobState,
			"message_state": messageState,
			"repo_write":    repoWrite,
		})
	}
	return summaries, nil
}
