# Striatum Evidence Export

Run ID: `run_982b4ae0112e4cc9b7d71e82bb2d056f`
Branch: `striatum/dogfood-002-session-close`
Run state: `completed`
Exported at: `2026-05-08T02:29:45Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":3},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"runs":[{"branch_name":"striatum/dogfood-002-session-close","run_id":"run_982b4ae0112e4cc9b7d71e82bb2d056f","state":"completed"}]}
```

## Doctor Output

```json
{"ok":true,"problems":[],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_cdc2cd40a4be48eeb437526d3233df7b",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: author-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: author-claude-opus-001",
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "apply_change"
      },
      "content_sha256": "140d0862032b319f51c1c7a9ec2475c4fcac1156ca421b3a3db7ec2b22bfbd1e",
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_apply_change",
      "logical_name": "apply_handoff",
      "repo_path": "docs/dogfood/002/APPLY_HANDOFF.md",
      "session_id": "sess_ddc8256c819647c9a049d1b82044e96a"
    },
    {
      "artifact_id": "art_3c19eab2fa8a456bbdec636425dfa6a6",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: author-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: author-claude-opus-001",
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "content_sha256": "2432d39f219e4794ddf8f5a663570491845c0492a7fb781311ba593da082befb",
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_draft_change",
      "logical_name": "draft_handoff",
      "repo_path": "docs/dogfood/002/DRAFT_HANDOFF.md",
      "session_id": "sess_ddc8256c819647c9a049d1b82044e96a"
    },
    {
      "artifact_id": "art_c066df7202f34f62b93ced96c7b72624",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": null,
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_change"
      },
      "content_sha256": "58a4dddf445095707da851b7f88e181d4f04ed5f96f0f0fd5e680cda69911eed",
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_review_change",
      "logical_name": "review_finding",
      "repo_path": "docs/dogfood/002/review/FINDING.md",
      "session_id": "sess_fd288b42fca2449c85ec09b07dca8c96"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T02:29:45Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "author",
        "workflow_job_id": "apply_change"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_review_change",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_change"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_apply_change",
      "job_type": "synthesis",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "author",
      "state": "completed",
      "workflow_job_id": "apply_change"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "dependencies": [],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_draft_change",
      "job_type": "draft",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "author",
      "state": "completed",
      "workflow_job_id": "draft_change"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_change"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_draft_change",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "draft_change"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_review_change",
      "job_type": "review",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_change"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-002-session-close",
    "run_id": "run_982b4ae0112e4cc9b7d71e82bb2d056f",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T02:29:32Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T02:08:41Z",
      "role_id": "author",
      "session_id": "sess_ddc8256c819647c9a049d1b82044e96a",
      "slug": "author-claude_code-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T02:29:32Z",
      "lane_id": "codex",
      "non_fresh_reason": "operator-driven; supervised lane work deferred to a future RFC",
      "ordinal": 1,
      "registered_at": "2026-05-08T02:08:41Z",
      "role_id": "reviewer",
      "session_id": "sess_fd288b42fca2449c85ec09b07dca8c96",
      "slug": "reviewer-codex-1",
      "state": "closed"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_c066df7202f34f62b93ced96c7b72624",
      "job_id": "job_run_982b4ae0112e4cc9b7d71e82bb2d056f_review_change",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_fd288b42fca2449c85ec09b07dca8c96",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_fb7f5d656d2b4a9389e179635d74ce1c"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-002-rfc-0011-session-close",
    "workflow_version": "2026-05-08"
  }
}
```
