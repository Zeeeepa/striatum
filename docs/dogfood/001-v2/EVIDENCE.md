# Striatum Evidence Export

Run ID: `run_4db045f7e3e643d6a75948dd1b86d6d6`
Branch: `striatum/dogfood-001-v2-harness-fixes`
Run state: `completed`
Exported at: `2026-05-08T00:33:34Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":3},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"runs":[{"branch_name":"striatum/dogfood-001-v2-harness-fixes","run_id":"run_4db045f7e3e643d6a75948dd1b86d6d6","state":"completed"}]}
```

## Doctor Output

```json
{"ok":false,"problems":["active session on terminal run: sess_edeebb4fa1634ef7b6298748c44135ce","active session on terminal run: sess_caa84d683fb6476ea9a696fc4f7e0a17","active session on terminal run: sess_9b12bb89327648809f72456adb32ae15","reviewer-independence unverified: reviewer session has no supervisor but author sess_edeebb4fa1634ef7b6298748c44135ce runs supervised (pid=1878886)","reviewer-independence unverified: reviewer session has no supervisor but author sess_edeebb4fa1634ef7b6298748c44135ce runs supervised (pid=1878886)"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_8ea3bc4c53f94e82a8911d67a8479215",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": null,
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "apply_change"
      },
      "content_sha256": "cb833dd678a5207f465b29a3f1060de16335eeae43eb3a9e443f1fbc88b8f57a",
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_apply_change",
      "logical_name": "apply_handoff",
      "repo_path": "docs/dogfood/001-v2/APPLY_HANDOFF.md",
      "session_id": "sess_edeebb4fa1634ef7b6298748c44135ce"
    },
    {
      "artifact_id": "art_807493e967784d269d39506e35e21466",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": null,
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": 1,
        "role_id": "author",
        "workflow_job_id": "draft_change"
      },
      "content_sha256": "d5b64da7732027fdaf0d929ce1c2094ab5bf5035011c3ae4284e30d2f22032a9",
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_draft_change",
      "logical_name": "draft_handoff",
      "repo_path": "docs/dogfood/001-v2/DRAFT_HANDOFF.md",
      "session_id": "sess_edeebb4fa1634ef7b6298748c44135ce"
    },
    {
      "artifact_id": "art_cef9119bf0994f7a843679ee98ade356",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": null,
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_change"
      },
      "content_sha256": "0907695d58f2a5933b57f760d0401505ad2745243d0cc45b8f427b9ed776cfb2",
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_review_change",
      "logical_name": "review_finding",
      "repo_path": "docs/dogfood/001-v2/review/FINDING.md",
      "session_id": "sess_9b12bb89327648809f72456adb32ae15"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T00:33:34Z",
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
          "depends_on_job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_review_change",
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
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_apply_change",
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
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_draft_change",
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
          "depends_on_job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_draft_change",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "draft_change"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_review_change",
      "job_type": "review",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_change"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-001-v2-harness-fixes",
    "run_id": "run_4db045f7e3e643d6a75948dd1b86d6d6",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "verdicts": [
    {
      "findings_artifact_id": "art_cef9119bf0994f7a843679ee98ade356",
      "job_id": "job_run_4db045f7e3e643d6a75948dd1b86d6d6_review_change",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_9b12bb89327648809f72456adb32ae15",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_e2815305524c44e8bd8dd65054194473"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-001-v2-harness-fixes",
    "workflow_version": "2026-05-07"
  }
}
```
