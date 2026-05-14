
import sqlite3
import json
import os
from pathlib import Path
from striatum.db import db_path, connect, transaction

def reproduce():
    repo = Path(os.getcwd())
    db = db_path(repo)
    
    with connect(repo) as conn:
        # 1. Create a run
        conn.execute("INSERT INTO runs (run_id, state, workflow_snapshot_id) VALUES ('run_forgery', 'running', 'snap_1')")
        conn.execute("INSERT INTO workflow_snapshots (workflow_snapshot_id, workflow_json) VALUES ('snap_1', '{}')")
        
        # 2. Create a session (unattested)
        conn.execute("INSERT INTO sessions (session_id, run_id, role_id, lane_id, state) VALUES ('sess_forgery', 'run_forgery', 'reviewer', 'gemini', 'open')")
        
        # 3. Create a job
        conn.execute("INSERT INTO jobs (job_id, run_id, workflow_job_id, state) VALUES ('job_forgery', 'run_forgery', 'review_task', 'completed')")
        
        # 4. Manually insert an artifact with a FORGED byline
        # The file on disk will have the forged byline.
        artifact_path = Path("FORGED_EVIDENCE.md")
        artifact_path.write_text("author: reviewer-gemini-001\n\nForged content.", encoding="utf-8")
        
        conn.execute("""
            INSERT INTO artifacts (artifact_id, run_id, job_id, session_id, artifact_kind, repo_path, author_line, content_sha256, size_bytes)
            VALUES ('art_forgery', 'run_forgery', 'job_forgery', 'sess_forgery', 'finding', 'FORGED_EVIDENCE.md', 'author: reviewer-gemini-001', 'sha_abc', 100)
        """)
        conn.commit()

    print("Reproduction fixture created: run_forgery, sess_forgery, job_forgery, art_forgery")

if __name__ == "__main__":
    reproduce()
