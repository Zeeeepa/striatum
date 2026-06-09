package mutations

import (
	"context"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// freshReviewLineageEvent is the audit event recorded when a claim is refused
// because the claimant's supervised process is not a genuinely fresh reviewer.
const freshReviewLineageEvent = "work.claim_ineligible_fresh_review_lineage"

// freshReviewProcessLineageRefusal enforces fresh-review PROCESS independence at
// work.claim (#222). The existing fresh-session gate only guarantees a new
// session id; a long-lived lane process can register a fresh session and review
// its own upstream work, recording a contaminated verdict. For a fresh-context
// review job this gate additionally requires that the claimant's supervised
// process (process_supervisor_pointers.pid + pid_start_time) did not do durable
// work upstream of the review in this run.
//
// It returns a non-nil structured no_work refusal (ineligible_reason
// fresh_review_process_lineage) when the claim must be refused, leaving the job
// queued for a real fresh reviewer; nil means the claim may proceed. It is run
// after the fresh-session gate and before any lease/packet is created.
func freshReviewProcessLineageRefusal(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID string, job map[string]any) (map[string]any, error) {
	if !boolValue(job["fresh_session_required"]) || !isVerdictCapableJobType(fmt.Sprint(job["job_type"])) {
		return nil, nil
	}
	jobID := fmt.Sprint(job["job_id"])

	pid, startToken, found, err := sessionSupervisedProcessIdentity(ctx, tx, repositoryID, sessionID)
	if err != nil {
		return nil, err
	}
	if !found || pid <= 0 || strings.TrimSpace(startToken) == "" {
		// A supervised fresh-review lane whose process identity cannot be verified
		// must not claim either: the daemon cannot prove reviewer independence.
		return freshReviewLineageRefusal(ctx, tx, repositoryID, runID, sessionID, jobID, map[string]any{
			"identity_status": "missing_or_unverified",
			"hint":            "this fresh-review job requires a verifiable supervised process identity (pid + pid_start_time) to prove reviewer independence; register/start a fresh supervised session and retry.",
		})
	}

	upstreamJobIDs, err := upstreamReviewJobIDs(ctx, tx, repositoryID, runID, jobID)
	if err != nil {
		return nil, err
	}
	if len(upstreamJobIDs) == 0 {
		return nil, nil
	}
	contaminating, err := upstreamWorkByProcess(ctx, tx, repositoryID, upstreamJobIDs, pid, startToken)
	if err != nil {
		return nil, err
	}
	if contaminating == nil {
		return nil, nil
	}
	return freshReviewLineageRefusal(ctx, tx, repositoryID, runID, sessionID, jobID, map[string]any{
		"contaminating_session_id": contaminating["session_id"],
		"contaminating_job_id":     contaminating["job_id"],
		"process_pid":              pid,
		"process_pid_start_time":   startToken,
		"hint":                     "the supervised process claiming this fresh review already did durable work upstream of it in this run; a genuinely fresh reviewer process must claim it. The job remains queued.",
	})
}

// sessionSupervisedProcessIdentity returns the claimant session's current
// supervised process identity (pid + pid_start_time). found is false when the
// session has no supervisor pointer at all.
func sessionSupervisedProcessIdentity(ctx context.Context, tx db.TxRunner, repositoryID, sessionID string) (pid int, startToken string, found bool, err error) {
	rows, err := queryRows(ctx, tx, `
		SELECT pid, pid_start_time
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND session_id = $2
		   AND state IN ('starting','attached','detached')
		 ORDER BY updated_at DESC
		 LIMIT 1`, repositoryID, sessionID)
	if err != nil {
		return 0, "", false, err
	}
	if len(rows) == 0 {
		return 0, "", false, nil
	}
	row := rows[0]
	startToken = ""
	if value := row["pid_start_time"]; value != nil {
		startToken = strings.TrimSpace(fmt.Sprint(value))
	}
	return intValue(row["pid"]), startToken, true, nil
}

// upstreamReviewJobIDs returns every job_id the review depends on, expanded to
// all attempts of those upstream workflow jobs so a process that did an earlier
// attempt is still recognized as upstream contamination.
func upstreamReviewJobIDs(ctx context.Context, tx db.TxRunner, repositoryID, runID, reviewJobID string) ([]string, error) {
	rows, err := queryRows(ctx, tx, `
		SELECT DISTINCT j2.job_id
		  FROM striatumd.job_dependencies dep
		  JOIN striatumd.jobs j1
		    ON j1.repository_id = dep.repository_id AND j1.job_id = dep.depends_on_job_id
		  JOIN striatumd.jobs j2
		    ON j2.repository_id = dep.repository_id
		   AND j2.run_id = j1.run_id
		   AND j2.workflow_job_id = j1.workflow_job_id
		 WHERE dep.repository_id = $1 AND dep.job_id = $2`, repositoryID, reviewJobID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := strings.TrimSpace(fmt.Sprint(row["job_id"])); id != "" && id != reviewJobID {
			out = append(out, id)
		}
	}
	return out, nil
}

// upstreamWorkByProcess returns the first upstream (session_id, job_id) whose
// work packet was claimed by the given supervised process identity, or nil if
// that process did no durable upstream work. A claimed work packet — not mere
// session registration — is the contamination signal.
func upstreamWorkByProcess(ctx context.Context, tx db.TxRunner, repositoryID string, upstreamJobIDs []string, pid int, startToken string) (map[string]any, error) {
	rows, err := queryRows(ctx, tx, `
		SELECT wp.session_id, wp.job_id
		  FROM striatumd.work_packets wp
		  JOIN striatumd.process_supervisor_pointers p
		    ON p.repository_id = wp.repository_id AND p.session_id = wp.session_id
		 WHERE wp.repository_id = $1
		   AND wp.job_id = ANY($2)
		   AND p.pid = $3
		   AND p.pid_start_time = $4
		 LIMIT 1`, repositoryID, upstreamJobIDs, pid, startToken)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// freshReviewLineageRefusal records a deduped audit event for the ineligible
// claim attempt and returns the structured no_work refusal.
func freshReviewLineageRefusal(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID, jobID string, detail map[string]any) (map[string]any, error) {
	if err := recordFreshReviewLineageAudit(ctx, tx, repositoryID, runID, sessionID, jobID, detail); err != nil {
		return nil, err
	}
	res := map[string]any{
		"status":            "no_work",
		"ineligible_reason": "fresh_review_process_lineage",
		"job_id":            jobID,
	}
	for key, value := range detail {
		res[key] = value
	}
	return res, nil
}

// recordFreshReviewLineageAudit appends the ineligible-claim audit event once
// per (session, job): a polling lane retries work.claim, so the event is deduped
// to avoid flooding the event log.
func recordFreshReviewLineageAudit(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID, jobID string, detail map[string]any) error {
	existing, err := queryRows(ctx, tx, `
		SELECT 1
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = $3
		   AND actor_session_id = $4 AND job_id = $5
		 LIMIT 1`, repositoryID, runID, freshReviewLineageEvent, sessionID, jobID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	payload := map[string]any{"ineligible_reason": "fresh_review_process_lineage"}
	for key, value := range detail {
		payload[key] = value
	}
	_, err = appendEvent(ctx, tx, repositoryID, runID, freshReviewLineageEvent, sessionID, jobID, nil, nil, nil, payload)
	return err
}

// HandleClaimOverride is the admin-only escape for the fresh-review process
// lineage gate (#222). It directly claims an existing pending job for a session
// when a scoped, accepted operator decision authorizes exactly that
// (session_id, job_id). It deliberately bypasses the lineage gate (that is the
// point of the override) but nothing else: the run must be running, the session
// active, and the job still claimable. Capability `admin` (enforced by the RPC
// registry) means a session-bound lane token can never reach this path — there
// is no normal-lane claim-next --force.
func HandleClaimOverride(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID := strings.TrimSpace(stringParam(envelope, "session_id"))
	jobID := strings.TrimSpace(stringParam(envelope, "job_id"))
	decisionID := strings.TrimSpace(stringParam(envelope, "decision_id"))
	if sessionID == "" || jobID == "" || decisionID == "" {
		return nil, rpc.NewError("schema_invalid", "work.claim_override requires session_id, job_id, and decision_id", nil)
	}
	leaseSeconds := intParam(envelope, "lease_seconds", 3600)
	if leaseSeconds <= 0 {
		leaseSeconds = 3600
	}
	return withTxRetryOnDeadlock(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if err := lockRunForSession(ctx, tx, repositoryID, sessionID); err != nil {
			return nil, err
		}
		session, err := rowByID(ctx, tx, repositoryID, "sessions", "session_id", sessionID, true)
		if err != nil {
			return nil, err
		}
		if state := fmt.Sprint(session["state"]); state != "active" {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("session is %s; work.claim_override requires an active claimant session", state), nil)
		}
		runID := fmt.Sprint(session["run_id"])
		run, err := rowByID(ctx, tx, repositoryID, "runs", "run_id", runID, true)
		if err != nil {
			return nil, err
		}
		if state := fmt.Sprint(run["state"]); state != "running" {
			return nil, rpc.NewError("invalid_transition", "work.claim_override requires a running run", nil)
		}
		if err := requireClaimOverrideDecision(ctx, tx, repositoryID, runID, sessionID, jobID, decisionID); err != nil {
			return nil, err
		}
		job, err := rowByID(ctx, tx, repositoryID, "jobs", "job_id", jobID, true)
		if err != nil {
			return nil, err
		}
		if fmt.Sprint(job["run_id"]) != runID {
			return nil, rpc.NewError("invalid_transition", "job does not belong to the claimant session's run", nil)
		}
		rows, err := queryRows(ctx, tx, `
			SELECT qm.*
			  FROM striatumd.queue_messages qm
			 WHERE qm.repository_id = $1
			   AND qm.job_id = $2
			   AND qm.kind = 'work'
			   AND qm.state = 'pending'
			 ORDER BY qm.priority DESC, qm.created_at ASC
			 LIMIT 1
			 FOR UPDATE OF qm SKIP LOCKED`, repositoryID, jobID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, rpc.NewError("invalid_transition", "no pending work message for job "+jobID+"; it may already be claimed or terminal", nil)
		}
		chosen := rows[0]
		result, err := claimChosenJob(ctx, tx, repositoryID, sessionID, runID, session, run, job, chosen, leaseSeconds)
		if err != nil {
			return nil, err
		}
		if _, err := appendEvent(ctx, tx, repositoryID, runID, "work.claim_overridden", sessionID, jobID, chosen["message_id"], nil, nil, map[string]any{
			"decision_id": decisionID,
			"override":    "fresh_review_process_lineage",
		}); err != nil {
			return nil, err
		}
		result["override_decision_id"] = decisionID
		return result, nil
	})
}

// requireClaimOverrideDecision validates that decisionID names an accepted
// operator decision artifact for this run whose recorded scope matches the exact
// (session_id, job_id). A missing, non-accepting, unscoped (broad), or
// mismatched decision is refused.
func requireClaimOverrideDecision(ctx context.Context, tx db.TxRunner, repositoryID, runID, sessionID, jobID, decisionID string) error {
	artifacts, err := queryRows(ctx, tx, `
		SELECT artifact_id FROM striatumd.artifacts
		 WHERE repository_id = $1 AND run_id = $2 AND artifact_kind = 'decision' AND logical_name = $3
		 LIMIT 1`, repositoryID, runID, decisionID)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		return rpc.NewError("invalid_transition", "work.claim_override requires an accepted decision artifact ("+decisionID+") for this run", nil)
	}
	events, err := queryRows(ctx, tx, `
		SELECT payload_json->>'outcome' AS outcome,
		       payload_json->>'subject_session_id' AS subject_session_id,
		       payload_json->>'subject_job_id' AS subject_job_id
		  FROM striatumd.events
		 WHERE repository_id = $1 AND run_id = $2 AND event_type = 'decision.recorded'
		   AND payload_json->>'decision_id' = $3
		 ORDER BY event_id DESC
		 LIMIT 1`, repositoryID, runID, decisionID)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return rpc.NewError("invalid_transition", "decision "+decisionID+" has no recorded outcome", nil)
	}
	ev := events[0]
	outcome := rowString(ev, "outcome")
	if outcome != "accepted" && outcome != "accepted_with_follow_up" {
		return rpc.NewError("invalid_transition", "work.claim_override requires an accepting decision; "+decisionID+" outcome is "+outcome, nil)
	}
	subjectSession := rowString(ev, "subject_session_id")
	subjectJob := rowString(ev, "subject_job_id")
	if subjectSession == "" || subjectJob == "" {
		return rpc.NewError("invalid_transition", "work.claim_override requires a decision scoped to an exact (session_id, job_id); "+decisionID+" is a broad decision (refused)", nil)
	}
	if subjectSession != sessionID || subjectJob != jobID {
		return rpc.NewError("invalid_transition", "decision "+decisionID+" authorizes a different (session_id, job_id); override refused", nil)
	}
	return nil
}

func rowString(row map[string]any, key string) string {
	if value := row[key]; value != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}
