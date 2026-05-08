# Striatum Evidence Export

Run ID: `run_c8cd066bc1344571bf875683d4edb892`
Branch: `striatum/dogfood-006-local-service-api`
Run state: `completed`
Exported at: `2026-05-08T17:42:06Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":5},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-006-local-service-api","run_id":"run_c8cd066bc1344571bf875683d4edb892","state":"completed"}]}
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
      "artifact_id": "art_b642585520584f0ba5b44a52237b712b",
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
      "content_sha256": "0d6672a49dc15afebda0872df5e4cab428cae5bd224efd1f2160bf826e332050",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_implement_v1",
      "logical_name": "v1_handoff",
      "repo_path": "docs/dogfood/006/BUILD_HANDOFF.md",
      "session_id": "sess_509580ff0345491bb2a073a6a7e1e241"
    },
    {
      "artifact_id": "art_31c130f7d447455b8a05f1d2ac4ce200",
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
      "content_sha256": "c5dd5f62630988757d96fb26c99922763e4cb9a54d95f0d706b4b34568f30e31",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_synthesize_v1_design",
      "logical_name": "v1_design",
      "repo_path": "docs/dogfood/006/DESIGN_SYNTHESIS.md",
      "session_id": "sess_af26a6c781004cda8bdb3c571ccd39e8"
    },
    {
      "artifact_id": "art_8ebb700472054b4a846385c2c7389f62",
      "artifact_kind": "decision",
      "content_sha256": "0aa4b0f605b315c459403ed32284ec1f596c8fc87b8ea3908130e08fa68ef239",
      "job_id": null,
      "logical_name": "dec_ae012b59f3a745cb922fa3b8cba90fd0",
      "repo_path": "docs/dogfood/006/decisions/V1_ACCEPTANCE.md",
      "session_id": null
    },
    {
      "artifact_id": "art_80195159ab2a41e3a1c434a6a5610866",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: researcher-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_service_surface"
      },
      "content_sha256": "541f2b557bf6575e4595d2a355f4985aceee77fdc60bb7f1cbc7ffe155a681ce",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_research_service_surface",
      "logical_name": "service_research",
      "repo_path": "docs/dogfood/006/research/SERVICE_SURFACE.md",
      "session_id": "sess_798de3e08343440d930e1e21c3ecb244"
    },
    {
      "artifact_id": "art_e028024eac544368a5234b8025a7ac5d",
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
      "content_sha256": "6c98b3ab09701937d6a65d7174838e1f26dd96738905773e9cd63e0f65651671",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_build",
      "logical_name": "v1_build_review",
      "repo_path": "docs/dogfood/006/review/build/BUILD_REVIEW.md",
      "session_id": "sess_3c7a45e6202449d481875eabbc87c94b"
    },
    {
      "artifact_id": "art_0c8fc68aab1e445a8077d13551e5da3f",
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
      "content_sha256": "44c157df06e8cb1d6965e268deea6250060ed0974512c1ca8fb4406b7f1c8ffa",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_design",
      "logical_name": "v1_design_review",
      "repo_path": "docs/dogfood/006/review/design/DESIGN_REVIEW.md",
      "session_id": "sess_7edce49174ab4ae693198620e18f7d14"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T17:42:06Z",
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
          "depends_on_job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_design",
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
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_implement_v1",
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
        "workflow_job_id": "research_service_surface"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_research_service_surface",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_service_surface"
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
          "depends_on_job_id": "job_run_c8cd066bc1344571bf875683d4edb892_implement_v1",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_v1"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_build",
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
          "depends_on_job_id": "job_run_c8cd066bc1344571bf875683d4edb892_synthesize_v1_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_v1_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_design",
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
          "depends_on_job_id": "job_run_c8cd066bc1344571bf875683d4edb892_research_service_surface",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_service_surface"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_synthesize_v1_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_v1_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-006-local-service-api",
    "run_id": "run_c8cd066bc1344571bf875683d4edb892",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T17:42:06Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 2,
      "registered_at": "2026-05-08T17:18:29Z",
      "role_id": "reviewer",
      "session_id": "sess_3c7a45e6202449d481875eabbc87c94b",
      "slug": "reviewer-claude_code-2",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T17:42:06Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:18:29Z",
      "role_id": "implementer",
      "session_id": "sess_509580ff0345491bb2a073a6a7e1e241",
      "slug": "implementer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T17:42:06Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:18:29Z",
      "role_id": "researcher",
      "session_id": "sess_798de3e08343440d930e1e21c3ecb244",
      "slug": "researcher-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T17:42:06Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:18:29Z",
      "role_id": "reviewer",
      "session_id": "sess_7edce49174ab4ae693198620e18f7d14",
      "slug": "reviewer-claude_code-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T17:42:06Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:18:29Z",
      "role_id": "designer",
      "session_id": "sess_af26a6c781004cda8bdb3c571ccd39e8",
      "slug": "designer-codex-1",
      "state": "closed"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_0c8fc68aab1e445a8077d13551e5da3f",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_design",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_7edce49174ab4ae693198620e18f7d14",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_86558a77e6274af6b5bda732cb4e9353"
    },
    {
      "findings_artifact_id": "art_e028024eac544368a5234b8025a7ac5d",
      "job_id": "job_run_c8cd066bc1344571bf875683d4edb892_review_v1_build",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_3c7a45e6202449d481875eabbc87c94b",
      "verdict": "accept",
      "verdict_id": "verdict_dddc257465954ecd856161cda9ba3850"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-006-local-service-api",
    "workflow_version": "2026-05-08"
  }
}
```
