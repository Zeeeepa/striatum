# Striatum Evidence Export

Run ID: `run_0e6a74ae8feb481cbc18a4b1435552b6`
Branch: `striatum/dogfood-003-tool-harness-profiles`
Run state: `completed`
Exported at: `2026-05-08T07:27:16Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":7},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"runs":[{"branch_name":"striatum/dogfood-003-tool-harness-profiles","run_id":"run_0e6a74ae8feb481cbc18a4b1435552b6","state":"completed"}]}
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
      "artifact_id": "art_d7a29bb794bf487cb4ae027adae2a9cd",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_profiles"
      },
      "content_sha256": "d3e0b57749a6e9f0a07f808ddd0a8c1e511daecf1843ae0d6394dbccb6fa2862",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_implement_profiles",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/003/BUILD_HANDOFF.md",
      "session_id": "sess_c90d0d749af1489bbaedae76aaa014f2"
    },
    {
      "artifact_id": "art_606770522aab4e6daf217e4ec5769d05",
      "artifact_kind": "synthesis",
      "author": {
        "actual_author_line": "author: designer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: designer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "synthesize_design"
      },
      "content_sha256": "64b67170289f13586d618ced82babc40a7a0ca220d397f700241fac13aca641a",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/003/DESIGN_SYNTHESIS.md",
      "session_id": "sess_77fad14086f240aea12e32fc05d4a049"
    },
    {
      "artifact_id": "art_ff31740adc544acfab6132000e89eb07",
      "artifact_kind": "decision",
      "content_sha256": "a6e327311349fb2b1d9e4490bab0c25b42f8cbafdc2f69a9418ccb272086cbb8",
      "job_id": null,
      "logical_name": "dec_6abd3957ab1748949ff0967221b346c4",
      "repo_path": "docs/dogfood/003/decisions/RFC_0010_ACCEPTANCE.md",
      "session_id": null
    },
    {
      "artifact_id": "art_985f067502ed4c0aad428e5c569ab67e",
      "artifact_kind": "harness_improvement_proposal",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_profiles"
      },
      "content_sha256": "c9ff8f8fefb2bfea3bd5066b5ce9a7e9cc1b7f93be2242fbc6d647f157f4ae86",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_implement_profiles",
      "logical_name": "harness_001",
      "repo_path": "docs/dogfood/003/findings/HARNESS-001.md",
      "session_id": "sess_c90d0d749af1489bbaedae76aaa014f2"
    },
    {
      "artifact_id": "art_c116c463f07b41e4942a330f5cbb30a2",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: researcher-claude-opus-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_claude_code"
      },
      "content_sha256": "14c9608d6eb4b419a5536c6cc1300e3210d17d2b682f2065e09256c7ca0f6381",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_claude_code",
      "logical_name": "claude_code_research",
      "repo_path": "docs/dogfood/003/research/claude_code/TOOL_RESEARCH.md",
      "session_id": "sess_7824195a67e34a1cb90fcb68d8c145e9"
    },
    {
      "artifact_id": "art_9be5e0b095334e188f41cc3c022302ea",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: researcher-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_codex"
      },
      "content_sha256": "293a16296072da0224e6611cd7a2541e6737e33329fb642f232dd11ba8af50c9",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_codex",
      "logical_name": "codex_research",
      "repo_path": "docs/dogfood/003/research/codex/TOOL_RESEARCH.md",
      "session_id": "sess_837de31580774e2583628ca249cb4b30"
    },
    {
      "artifact_id": "art_4ce817bc3c384cbf80a8c4caa9a75c10",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: researcher-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: researcher-gemini-pro-001",
        "ordinal": 1,
        "role_id": "researcher",
        "workflow_job_id": "research_gemini"
      },
      "content_sha256": "dc7d06297725bfe6e985acc6499272a75a3d50371d32dc89f682d3cba7b84b5e",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_gemini",
      "logical_name": "gemini_research",
      "repo_path": "docs/dogfood/003/research/gemini/TOOL_RESEARCH.md",
      "session_id": "sess_12b83fcf3ff34f5381a16315119e0902"
    },
    {
      "artifact_id": "art_31506b54283f437b988964dd32d509ed",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-002",
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_build"
      },
      "content_sha256": "6f5fca27597c429964d10ced4c10731e9e975be752d34b16c04347fcb7368474",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_build",
      "logical_name": "build_review",
      "repo_path": "docs/dogfood/003/review/build/BUILD_REVIEW.md",
      "session_id": "sess_a34be6274a2b45c1b16fbb8bf162b693"
    },
    {
      "artifact_id": "art_3739194e0efb41d389922bee972325a7",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_design"
      },
      "content_sha256": "91612fe84caa80711604e68e53877a191d7d01a2ee299a9bd152c5efa81719d1",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_design",
      "logical_name": "design_review",
      "repo_path": "docs/dogfood/003/review/design/DESIGN_REVIEW.md",
      "session_id": "sess_a8782f7511144fccab870b9e9b63c839"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-08T07:27:16Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_profiles"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_design",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_implement_profiles",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_profiles"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "researcher",
        "workflow_job_id": "research_claude_code"
      },
      "dependencies": [],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_claude_code",
      "job_type": "generic",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_claude_code"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "researcher",
        "workflow_job_id": "research_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_codex",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_codex"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "researcher",
        "workflow_job_id": "research_gemini"
      },
      "dependencies": [],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_gemini",
      "job_type": "generic",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "researcher",
      "state": "completed",
      "workflow_job_id": "research_gemini"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_implement_profiles",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_profiles"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_build",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_design",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_design"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "synthesize_design"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_claude_code"
        },
        {
          "depends_on_job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_codex"
        },
        {
          "depends_on_job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_research_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "research_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-003-tool-harness-profiles",
    "run_id": "run_0e6a74ae8feb481cbc18a4b1435552b6",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "researcher",
      "session_id": "sess_12b83fcf3ff34f5381a16315119e0902",
      "slug": "researcher-gemini-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "designer",
      "session_id": "sess_77fad14086f240aea12e32fc05d4a049",
      "slug": "designer-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "researcher",
      "session_id": "sess_7824195a67e34a1cb90fcb68d8c145e9",
      "slug": "researcher-claude_code-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "researcher",
      "session_id": "sess_837de31580774e2583628ca249cb4b30",
      "slug": "researcher-codex-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 2,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "reviewer",
      "session_id": "sess_a34be6274a2b45c1b16fbb8bf162b693",
      "slug": "reviewer-claude_code-2",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "reviewer",
      "session_id": "sess_a8782f7511144fccab870b9e9b63c839",
      "slug": "reviewer-claude_code-1",
      "state": "closed"
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-08T07:26:59Z",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "ordinal": 1,
      "registered_at": "2026-05-08T06:33:46Z",
      "role_id": "implementer",
      "session_id": "sess_c90d0d749af1489bbaedae76aaa014f2",
      "slug": "implementer-codex-1",
      "state": "closed"
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_3739194e0efb41d389922bee972325a7",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_design",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_a8782f7511144fccab870b9e9b63c839",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_611348f17e3343c9b5319a6028f1fe1b"
    },
    {
      "findings_artifact_id": "art_31506b54283f437b988964dd32d509ed",
      "job_id": "job_run_0e6a74ae8feb481cbc18a4b1435552b6_review_build",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_a34be6274a2b45c1b16fbb8bf162b693",
      "verdict": "accept",
      "verdict_id": "verdict_f792725dcb6b400caefb717a956e8bd9"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-003-tool-harness-profiles",
    "workflow_version": "2026-05-08"
  }
}
```
