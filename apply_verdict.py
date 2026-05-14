
import sqlite3
from pathlib import Path
from striatum.db import connect
from striatum.cli.mutations import verdict_work

repo_path = Path("/home/halbritt/git/striatum")
conn = connect(repo_path)

try:
    result = verdict_work(
        conn,
        session_id="sess_81c6086c8993493ba013d8a0c39a0469",
        job_id="job_run_26232943fb8440e4bfdf00d4ffbc5d93_review_build_gemini",
        lease_id="lease_497c963b0fbe4336ada9430a1637e47d",
        verdict="needs_revision",
        findings_artifact_id=None,
        rationale="Fails critical adversarial honesty checks: byline forgery loophole, attestation-drift recomputation, and dangerous inferred_override logic. See docs/dogfood/054/review/build/gemini/REVIEW.md."
    )
    print(result)
except Exception as e:
    print(f"Error: {e}")
finally:
    conn.close()
