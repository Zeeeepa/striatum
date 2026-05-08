# Striatum Evidence Export

Run ID: `run_833b407118184930b154288684dadbee`
Branch: `striatum/dogfood-005-process-adapter-completion`
Run state: `completed`
Exported at: `2026-05-08T16:59:15Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":5},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-005-process-adapter-completion","run_id":"run_833b407118184930b154288684dadbee","state":"completed"}]}
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
      "artifact_id": "art_91689286386b4e2fb0874705f8195692",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_v1"
      },
      "content_sha256": "2279b1fb07e7647c1090bb0842074854ef2ac3300993e0c6709e0674ff5f6d8e",
      "job_id": "job_run_833b407118184930b154288684dadbee_implement_v1",
      "logical_name": "v1_handoff",
      "repo_path": "docs/dogfood/005/BUILD_HANDOFF.md",
      "session_id": "sess_0d1a4d8ca0f844e3930a3987bceb921b"
    },
    {
      "artifact_id": "art_fca2e15725ca45d1a9b49ab991eb5f99",
      "artifact_kind": "synthesis",
      "author": {
        "actual_author_line": "author: designer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: designer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "synthesize_v1_design"
      },
      "content_sha256": "6378267cf5662bdd55d8eed5b4df0487e9788888a8212bcda4fe0735c3c0b714",
      "job_id": "job_run_833b407118184930b154288684dadbee_synthesize_v1_design",
      "logical_name": "v1_design",
      "repo_path": "docs/dogfood/005/DESIGN_SYNTHESIS.md",
      "session_id": "sess_0f77fe4511bc4ee08e4b7cadad4bbc05"
    },
    {
      "artifact_id": "art_86243f07ca1c492bb4d8b4be5b685ce5",
      "artifact_kind": "decision",
      "content_sha256": "7e113b14985a17752d482ee6cbfa45949f5bd9d95a863edcf7ce96c9526b9e68",
      "job_id": null,
      "logical_name": "dec_f3cb9562eabb48d2b8db23436719ecf2",
      "repo_path": "docs/dogfood/005/decisions/V1_ACCEPTANCE.md",
      "session_id": null
    },
    {
      "artifact_id": "art_448a8bed60904c6e99ac54e570db1a0b",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: researcher-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_current_adapter_path"
      },
      "content_sha256": "8d4bc5f5fd1c7defd4601863a917ff8a348995cee2dc346cc55b3754d723587c",
      "job_id": "job_run_833b407118184930b154288684dadbee_research_current_adapter_path",
      "logical_name": "adapter_research",
      "repo_path": "docs/dogfood/005/research/CURRENT_ADAPTER.md",
      "session_id": "sess_7dfff632fea14ac3b9906198623c109b"
    },
    {
      "artifact_id": "art_57a2afdf7d484c43bd5d532694b6b2d6",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-002",
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_v1_build"
      },
      "content_sha256": "f8b9949588c03626abd9ebf2f045bed89baa6a94a7241fc1613647b884c41f0e",
      "job_id": "job_run_833b407118184930b154288684dadbee_review_v1_build",
      "logical_name": "v1_build_review",
      "repo_path": "docs/dogfood/005/review/build/BUILD_REVIEW.md",
      "session_id": "sess_64bd7dd21ecb4646b10184549c98b524"
    },
    {
      "artifact_id": "art_a394cadb519e474c976d22fbc6a9fb11",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_v1_design"
      },
      "content_sha256": "8e16f48a946c6a30623832d8089b987c2e5d8842d33fabbba91df5fd2c292a49",
      "job_id": "job_run_833b407118184930b154288684dadbee_review_v1_design",
      "logical_name": "v1_design_review",
      "repo_path": "docs/dogfood/005/review/design/DESIGN_REVIEW.md",
      "session_id": "sess_ba4f322d16a74d6f95df97c1182150f0"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T16:59:15Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_v1"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_833b407118184930b154288684dadbee_review_v1_design",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_v1_design"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_833b407118184930b154288684dadbee_implement_v1",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_v1"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "researcher",
        "workflow_job_id": "research_current_adapter_path"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_833b407118184930b154288684dadbee_research_current_adapter_path",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_current_adapter_path"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_v1_build"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_833b407118184930b154288684dadbee_implement_v1",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_v1"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_833b407118184930b154288684dadbee_review_v1_build",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_v1_build"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_v1_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_833b407118184930b154288684dadbee_synthesize_v1_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_v1_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_833b407118184930b154288684dadbee_review_v1_design",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_v1_design"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "synthesize_v1_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_833b407118184930b154288684dadbee_research_current_adapter_path",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_current_adapter_path"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_833b407118184930b154288684dadbee_synthesize_v1_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_v1_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-005-process-adapter-completion",
    "run_id": "run_833b407118184930b154288684dadbee",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T16:59:06Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T16:29:50Z",
      "role_id": "implementer",
      "session_id": "sess_0d1a4d8ca0f844e3930a3987bceb921b",
      "slug": "implementer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T16:59:06Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T16:29:50Z",
      "role_id": "designer",
      "session_id": "sess_0f77fe4511bc4ee08e4b7cadad4bbc05",
      "slug": "designer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T16:59:06Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 2,
      "registered_at": "2026-05-08T16:29:50Z",
      "role_id": "reviewer",
      "session_id": "sess_64bd7dd21ecb4646b10184549c98b524",
      "slug": "reviewer-claude_code-2",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T16:59:06Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T16:29:50Z",
      "role_id": "researcher",
      "session_id": "sess_7dfff632fea14ac3b9906198623c109b",
      "slug": "researcher-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T16:59:06Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T16:29:50Z",
      "role_id": "reviewer",
      "session_id": "sess_ba4f322d16a74d6f95df97c1182150f0",
      "slug": "reviewer-claude_code-1",
      "state": "closed"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_a394cadb519e474c976d22fbc6a9fb11",
      "job_id": "job_run_833b407118184930b154288684dadbee_review_v1_design",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_ba4f322d16a74d6f95df97c1182150f0",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_3733a147c62f4d2797857a6ea349167c"
    },
    {
      "findings_artifact_id": "art_57a2afdf7d484c43bd5d532694b6b2d6",
      "job_id": "job_run_833b407118184930b154288684dadbee_review_v1_build",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_64bd7dd21ecb4646b10184549c98b524",
      "verdict": "accept",
      "verdict_id": "verdict_bf42b7ed67974a74bfceffa4d192f163"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-005-process-adapter-completion",
    "workflow_version": "2026-05-08"
  }
}
```
