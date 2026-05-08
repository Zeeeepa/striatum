# Striatum Evidence Export

Run ID: `run_341193641a8e4e528333a704908acda4`
Branch: `striatum/dogfood-004-claude-supervised-wrapper`
Run state: `completed`
Exported at: `2026-05-08T09:00:50Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":5},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"runs":[{"branch_name":"striatum/dogfood-004-claude-supervised-wrapper","run_id":"run_341193641a8e4e528333a704908acda4","state":"completed"}]}
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
      "artifact_id": "art_0a6ae2a98d00438b8a35a60f3a90c4f4",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_wrapper"
      },
      "content_sha256": "81d13d2ba9650abad4282240399eb89c8e0e84986a413055335b45ad6ec7d2c4",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_implement_wrapper",
      "logical_name": "wrapper_handoff",
      "repo_path": "docs/dogfood/004/BUILD_HANDOFF.md",
      "session_id": "sess_2b71f9610e424313947487f1b0acf964"
    },
    {
      "artifact_id": "art_5ea5c87cc637414cbcea484e0576ee8a",
      "artifact_kind": "synthesis",
      "author": {
        "actual_author_line": "author: designer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: designer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "synthesize_wrapper_design"
      },
      "content_sha256": "8219ba3543d64b302cb81450809d82fa5f04ddb9f1b62efce7faa667976b4f06",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_synthesize_wrapper_design",
      "logical_name": "wrapper_design",
      "repo_path": "docs/dogfood/004/DESIGN_SYNTHESIS.md",
      "session_id": "sess_1bcfa76517b649e0a2b09e4ff385ee4f"
    },
    {
      "artifact_id": "art_fd0f0797e97844e09e0139b4cf52840a",
      "artifact_kind": "decision",
      "content_sha256": "a4fe8e1f1b916fc1362b0c855e39c5d93147c1f00918afa95509db5cfa9fd57b",
      "job_id": null,
      "logical_name": "dec_191214fea393400db73657720b6181bc",
      "repo_path": "docs/dogfood/004/decisions/WRAPPER_ACCEPTANCE.md",
      "session_id": null
    },
    {
      "artifact_id": "art_0def4c248d90490e9a582a72945fae4a",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: researcher-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_pipe_behavior"
      },
      "content_sha256": "7c839c67f369aecda2cdfe5cf0e96200761d8d96a688d384072c156e7fb81934",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_research_pipe_behavior",
      "logical_name": "pipe_research",
      "repo_path": "docs/dogfood/004/research/PIPE_BEHAVIOR.md",
      "session_id": "sess_1387957f4abb47c9b5d8a9853c8e03cc"
    },
    {
      "artifact_id": "art_7588967e30c943ef8d6777192aaecbd0",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-002",
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_wrapper_build"
      },
      "content_sha256": "c155e1d59570e1417bf5c1c0d61b9987e1365a144b11bbae9cdfe1210a818bac",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_build",
      "logical_name": "wrapper_build_review",
      "repo_path": "docs/dogfood/004/review/build/BUILD_REVIEW.md",
      "session_id": "sess_3fa0bba02f5444a0a967b08c4e0b1996"
    },
    {
      "artifact_id": "art_6ba48134f29143938c5e1d3a01c7a336",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_wrapper_design"
      },
      "content_sha256": "d85c0518c8f5820fea3042d4cd92cea0d0d6ead24a7ad1691512dabd7267aa58",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_design",
      "logical_name": "wrapper_design_review",
      "repo_path": "docs/dogfood/004/review/design/DESIGN_REVIEW.md",
      "session_id": "sess_dee7079782ab4be3b970d6835de38b75"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T09:00:50Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_wrapper"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_design",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_wrapper_design"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_341193641a8e4e528333a704908acda4_implement_wrapper",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_wrapper"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "researcher",
        "workflow_job_id": "research_pipe_behavior"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_341193641a8e4e528333a704908acda4_research_pipe_behavior",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_pipe_behavior"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_wrapper_build"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_341193641a8e4e528333a704908acda4_implement_wrapper",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_wrapper"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_build",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_wrapper_build"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_wrapper_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_341193641a8e4e528333a704908acda4_synthesize_wrapper_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_wrapper_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_design",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_wrapper_design"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "synthesize_wrapper_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_341193641a8e4e528333a704908acda4_research_pipe_behavior",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_pipe_behavior"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_341193641a8e4e528333a704908acda4_synthesize_wrapper_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_wrapper_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-004-claude-supervised-wrapper",
    "run_id": "run_341193641a8e4e528333a704908acda4",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T09:00:38Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T08:43:36Z",
      "role_id": "researcher",
      "session_id": "sess_1387957f4abb47c9b5d8a9853c8e03cc",
      "slug": "researcher-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T09:00:38Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T08:43:36Z",
      "role_id": "designer",
      "session_id": "sess_1bcfa76517b649e0a2b09e4ff385ee4f",
      "slug": "designer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T09:00:38Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T08:43:36Z",
      "role_id": "implementer",
      "session_id": "sess_2b71f9610e424313947487f1b0acf964",
      "slug": "implementer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T09:00:38Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 2,
      "registered_at": "2026-05-08T08:43:36Z",
      "role_id": "reviewer",
      "session_id": "sess_3fa0bba02f5444a0a967b08c4e0b1996",
      "slug": "reviewer-claude_code-2",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T09:00:38Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T08:43:36Z",
      "role_id": "reviewer",
      "session_id": "sess_dee7079782ab4be3b970d6835de38b75",
      "slug": "reviewer-claude_code-1",
      "state": "closed"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_6ba48134f29143938c5e1d3a01c7a336",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_design",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_dee7079782ab4be3b970d6835de38b75",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_3c9716ff57f7425d87ea58fec22fb004"
    },
    {
      "findings_artifact_id": "art_7588967e30c943ef8d6777192aaecbd0",
      "job_id": "job_run_341193641a8e4e528333a704908acda4_review_wrapper_build",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_3fa0bba02f5444a0a967b08c4e0b1996",
      "verdict": "accept",
      "verdict_id": "verdict_e413ef08f9c345009de84226db17e1d5"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-004-claude-supervised-wrapper",
    "workflow_version": "2026-05-08"
  }
}
```
