# Striatum Evidence Export

Run ID: `run_9407e2f5afa04c46a521cd1c28665803`
Branch: `striatum/dogfood-007-local-web-ui`
Run state: `completed`
Exported at: `2026-05-08T18:04:12Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":5},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-007-local-web-ui","run_id":"run_9407e2f5afa04c46a521cd1c28665803","state":"completed"}]}
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
      "artifact_id": "art_8277779d053b407b86b4b9ae1a382947",
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
      "content_sha256": "2646f2a6df78b086944df46eff8070937297563cf5abcb180034a74643d409fa",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_implement_v1",
      "logical_name": "v1_handoff",
      "repo_path": "docs/dogfood/007/BUILD_HANDOFF.md",
      "session_id": "sess_fc95b2a54b054aa88f26d4917cb99914"
    },
    {
      "artifact_id": "art_663663cb47fb4c7b89dac998e4bf499a",
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
      "content_sha256": "28db06802e0b7155076d953bd12929ce810ca0dda86c8f849545d92a00800f78",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_synthesize_v1_design",
      "logical_name": "v1_design",
      "repo_path": "docs/dogfood/007/DESIGN_SYNTHESIS.md",
      "session_id": "sess_1af608ca7de84addb56c5f8250abf9ab"
    },
    {
      "artifact_id": "art_cbef014271b347a899e069275fdeae69",
      "artifact_kind": "decision",
      "content_sha256": "9cd4162c10eac18b03716d27e1891e3bfc56f138e675b574523652ace603d1d7",
      "job_id": null,
      "logical_name": "dec_ca529b49c04848d68fef12cd476d592d",
      "repo_path": "docs/dogfood/007/decisions/V1_ACCEPTANCE.md",
      "session_id": null
    },
    {
      "artifact_id": "art_22c953666ab24737b42ee01664894050",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: researcher-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_web_ui_surface"
      },
      "content_sha256": "54015bb47a48df7881d73f15e8669c8860fd9a8e7b39da7530a64375dcaba20d",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_research_web_ui_surface",
      "logical_name": "web_ui_research",
      "repo_path": "docs/dogfood/007/research/WEB_UI_SURFACE.md",
      "session_id": "sess_5ea9daab6c7447b0b922820ca5723fd4"
    },
    {
      "artifact_id": "art_3608f51fd5cb48f9a3a8fe5af8b1aec4",
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
      "content_sha256": "8330d0e69dcd5b5b126ce239352cdd228d2d6e95c0e381c5777aa81b4799e705",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_build",
      "logical_name": "v1_build_review",
      "repo_path": "docs/dogfood/007/review/build/BUILD_REVIEW.md",
      "session_id": "sess_317ee3458220499194d07218857ffcbd"
    },
    {
      "artifact_id": "art_bba5110d6d604d7da2d3d567deffb273",
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
      "content_sha256": "1f0c9525cbf6ace7a751a3b15a42034481de1386fc6de382fccd9db97f8a46e8",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_design",
      "logical_name": "v1_design_review",
      "repo_path": "docs/dogfood/007/review/design/DESIGN_REVIEW.md",
      "session_id": "sess_9f61b074baa644a4a83008de17c61b9a"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T18:04:12Z",
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
          "depends_on_job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_design",
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
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_implement_v1",
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
        "workflow_job_id": "research_web_ui_surface"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_research_web_ui_surface",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_web_ui_surface"
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
          "depends_on_job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_implement_v1",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_v1"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_build",
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
          "depends_on_job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_synthesize_v1_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_v1_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_design",
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
          "depends_on_job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_research_web_ui_surface",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_web_ui_surface"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_synthesize_v1_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_v1_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-007-local-web-ui",
    "run_id": "run_9407e2f5afa04c46a521cd1c28665803",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T18:04:12Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:45:00Z",
      "role_id": "researcher",
      "session_id": "sess_5ea9daab6c7447b0b922820ca5723fd4",
      "slug": "researcher-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T18:04:12Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:45:01Z",
      "role_id": "designer",
      "session_id": "sess_1af608ca7de84addb56c5f8250abf9ab",
      "slug": "designer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T18:04:12Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 2,
      "registered_at": "2026-05-08T17:45:01Z",
      "role_id": "reviewer",
      "session_id": "sess_317ee3458220499194d07218857ffcbd",
      "slug": "reviewer-claude_code-2",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T18:04:12Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:45:01Z",
      "role_id": "reviewer",
      "session_id": "sess_9f61b074baa644a4a83008de17c61b9a",
      "slug": "reviewer-claude_code-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T18:04:12Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T17:45:01Z",
      "role_id": "implementer",
      "session_id": "sess_fc95b2a54b054aa88f26d4917cb99914",
      "slug": "implementer-codex-1",
      "state": "closed"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_bba5110d6d604d7da2d3d567deffb273",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_design",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_9f61b074baa644a4a83008de17c61b9a",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_81e9cd3683014619ad90367943da4be3"
    },
    {
      "findings_artifact_id": "art_3608f51fd5cb48f9a3a8fe5af8b1aec4",
      "job_id": "job_run_9407e2f5afa04c46a521cd1c28665803_review_v1_build",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_317ee3458220499194d07218857ffcbd",
      "verdict": "accept",
      "verdict_id": "verdict_4ff133103d8e4b7eaa6f5ae81309c796"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-007-local-web-ui",
    "workflow_version": "2026-05-08"
  }
}
```
