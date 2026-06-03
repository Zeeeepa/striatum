#!/usr/bin/env python3
"""Unattended DoD driver: drive a multi-lane, review-gated, revision-capable run
to completion with ZERO operator rescue, N times consecutively.

"Unattended / zero rescue" = the driver only uses normal lane lifecycle verbs
(register-session, supervise start, and closing a lane whose job already
completed so a fresh-policy reviewer can register). It NEVER calls a rescue verb
(requeue-stale, override-verdict, retry-job, force). A run that reaches
`completed` this way is a clean pass; a run that hits needs_operator/failed (the
daemon escalated -> a human would be needed) is a FAIL.

Per RFC 0105's outside-CI acceptance: the standing proof of "no human intervention".
"""
import json
import subprocess
import sys
import time

REPO = sys.argv[1] if len(sys.argv) > 1 else "/tmp/striatum-floor-acceptance"
WORKFLOW = sys.argv[2] if len(sys.argv) > 2 else "workflow.json"
N = int(sys.argv[3]) if len(sys.argv) > 3 else 1
LANE = "claude"
RUN_TIMEOUT = 1500   # 25 min per run (draft -> review -> optional revision cycle)
POLL = 15


def cli(*args, check=True):
    """Run a striatum CLI verb scoped to REPO; return parsed JSON (or None)."""
    cmd = ["striatum", "--repo", REPO, *args]
    p = subprocess.run(cmd, capture_output=True, text=True)
    out = (p.stdout or "").strip()
    if p.returncode != 0 and check:
        return {"_error": (p.stderr or out).strip(), "_rc": p.returncode}
    try:
        return json.loads(out)
    except Exception:
        return {"_raw": out, "_err": (p.stderr or "").strip(), "_rc": p.returncode}


def run_state(summary):
    r = summary.get("run")
    if isinstance(r, dict):
        return r.get("state")
    return summary.get("state")


def jobs_of(summary):
    return summary.get("jobs", []) or []


def drive_run(run_id):
    """Drive one run to terminal state using only lifecycle verbs. Returns
    (ok, reason). ok=True iff the run reached `completed` with no escalation."""
    launched = {}   # (workflow_job_id, attempt) -> session_id
    try:
        return _drive_loop(run_id, launched)
    finally:
        # Close any lane still open at exit so idle claude processes do not pile
        # up across the N consecutive runs.
        for sid in launched.values():
            cli("supervise", "stop", "--session-id", sid,
                "--reason", "DoD run finished; closing lane", check=False)


def _drive_loop(run_id, launched):
    deadline = time.time() + RUN_TIMEOUT
    while time.time() < deadline:
        s = cli("run", "summary", "--run-id", run_id, check=False)
        if not isinstance(s, dict) or "_raw" in s:
            time.sleep(POLL); continue
        st = run_state(s)
        if st == "completed":
            return True, "completed"
        if st in ("needs_operator", "failed", "canceled"):
            return False, f"run state={st} (escalation / would need rescue)"
        if st in ("waiting_human", "blocked_human"):
            return False, f"run state={st} (human checkpoint — revision budget exhausted)"
        jobs = jobs_of(s)
        if jobs and all(j.get("state") == "completed" for j in jobs):
            return True, "all jobs completed"
        # 1) Close any lane whose job already completed, so a fresh-policy
        #    reviewer can register (mirrors the manual close-author-before-reviewer).
        for j in jobs:
            key = (j.get("workflow_job_id"), j.get("attempt"))
            if j.get("state") == "completed" and key in launched:
                sid = launched.pop(key)
                cli("supervise", "stop", "--session-id", sid,
                    "--reason", "lane completed its job; closing for fresh-policy", check=False)
        # 2) Launch a lane for each claimable job without one.
        for j in jobs:
            key = (j.get("workflow_job_id"), j.get("attempt"))
            if j.get("state") == "queued" and key not in launched:
                role = j.get("role_id") or ""
                sid = register_lane(run_id, role)
                if sid:
                    sup = cli("supervise", "start", "--session-id", sid, check=False)
                    if isinstance(sup, dict) and sup.get("state") in ("attached", "starting"):
                        launched[key] = sid
                        print(f"    launched {role} lane for {key} -> {sid}", flush=True)
                    else:
                        print(f"    supervise start failed for {key}: {sup}", flush=True)
        time.sleep(POLL)
    return False, "run timeout"


def register_lane(run_id, role):
    """register-session for role/LANE, handling the fresh-policy: if an active
    same-run session blocks a fresh registration, close completed sessions and
    retry once. Never uses --force-non-fresh (that would bypass the contract)."""
    for attempt in range(2):
        res = cli("register-session", "--run-id", run_id, "--role", role, "--lane", LANE, check=False)
        if isinstance(res, dict) and res.get("session_id"):
            return res["session_id"]
        msg = json.dumps(res)
        if "fresh" in msg and attempt == 0:
            # close any active session whose job is done, then retry
            close_completed_sessions(run_id)
            time.sleep(2)
            continue
        print(f"    register-session({role}) failed: {res}", flush=True)
        return None
    return None


def close_completed_sessions(run_id):
    """Close active sessions whose role's job already completed, freeing the
    fresh-policy for a downstream reviewer. `list sessions` returns {items:[...]}."""
    s = cli("run", "summary", "--run-id", run_id, check=False)
    done_roles = {j.get("role_id") for j in jobs_of(s) if j.get("state") == "completed"}
    listing = cli("list", "sessions", "--run-id", run_id, check=False)
    items = listing.get("items", []) if isinstance(listing, dict) else []
    for sess in items:
        if sess.get("state") == "active" and sess.get("role_id") in done_roles:
            cli("supervise", "stop", "--session-id", sess.get("session_id"),
                "--reason", "completed-job lane closed for fresh-policy", check=False)


def main():
    passes = 0
    for i in range(1, N + 1):
        print(f"=== DoD run {i}/{N} ===", flush=True)
        prep = cli("run", "prepare", "--workflow", WORKFLOW, check=False)
        run_id = prep.get("run_id") if isinstance(prep, dict) else None
        if not run_id:
            print(f"  prepare failed: {prep}", flush=True); break
        start = cli("run", "start", "--run-id", run_id, check=False)
        print(f"  run {run_id} start={start.get('state') if isinstance(start,dict) else start}", flush=True)
        ok, reason = drive_run(run_id)
        print(f"  run {i} -> {'PASS' if ok else 'FAIL'} ({reason}) [{run_id}]", flush=True)
        if not ok:
            # cancel the wedged run so it doesn't dangle, then stop the streak.
            cli("run", "cancel", "--run-id", run_id, "--reason", "DoD run did not complete unattended", check=False)
            print(f"=== STREAK BROKEN at run {i}: {reason} ===", flush=True)
            break
        passes += 1
    print(f"=== DoD RESULT: {passes}/{N} consecutive clean unattended passes ===", flush=True)
    sys.exit(0 if passes == N else 1)


if __name__ == "__main__":
    main()
