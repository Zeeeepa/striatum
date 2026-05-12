# Striatum Evidence Export

Run ID: `run_8f8f347a99ef4e95993db8c288f2ad59`
Branch: `striatum/dogfood-039-rfc-0037-web-ui-ergonomic-improvements`
Run state: `completed`
Exported at: `2026-05-12T13:10:53Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"completed":7},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-039-rfc-0037-web-ui-ergonomic-improvements","run_id":"run_8f8f347a99ef4e95993db8c288f2ad59","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_e8ff4c2e5ec0424c98864f46a75fedc1","slug":"designer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_f2c69a7f36de4a67884678031f4ba12f","slug":"designer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_b454b99024ee4dd4b4bb0c7a6b135ecb","slug":"designer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_3fb9d0d1d6814c4e95201b82ffc5507c","slug":"reviewer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_58d88f8a36f6473e96bae01290036af2","slug":"implementer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_99db163ac89a4671ad5a43f22abd7fd9","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":null}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.27.0' templates_changed=['codex/README.md.tmpl', 'codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`","plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.27.0' templates_changed=['claude_code/README.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`","plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.27.0' templates_changed=['gemini/GEMINI.md.tmpl', 'gemini/README.md.tmpl', 'gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_f2736a97531b41f79869179a7505d279",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement"
      },
      "content_sha256": "979de753f0f015a0449d0f3d0e9d2608ec9c698e12c831758e0ff3d4164382f0",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_implement",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/039/BUILD_HANDOFF.md",
      "session_id": "sess_58d88f8a36f6473e96bae01290036af2"
    },
    {
      "artifact_id": "art_601886aee0d24b74aca706b44938d7ee",
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
      "content_sha256": "8579bad1dc38c6ba3e5f4c9e74393c158ca91a5d48b8a943265312cfec026f18",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/039/DESIGN_SYNTHESIS.md",
      "session_id": "sess_e8ff4c2e5ec0424c98864f46a75fedc1"
    },
    {
      "artifact_id": "art_4da392d34f79440593437dc4bd47f152",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: designer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: designer-claude-opus-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_claude_code"
      },
      "content_sha256": "c281f9db781adbbff8ff1539d0c26bc5d330b34dd0f1e03f2706f8132bac202c",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/039/design/claude_code/DESIGN.md",
      "session_id": "sess_f2c69a7f36de4a67884678031f4ba12f"
    },
    {
      "artifact_id": "art_e3878bedf8fc4db5ac5ed27c2d418a37",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: designer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: designer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_codex"
      },
      "content_sha256": "50796377ae3eeeec03da1e89598a855c1e34eb6f1942a3472f80e44922707cb7",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/039/design/codex/DESIGN.md",
      "session_id": "sess_e8ff4c2e5ec0424c98864f46a75fedc1"
    },
    {
      "artifact_id": "art_b5be0d4fd9e6430587ed8ab97a488289",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: designer-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: designer-gemini-pro-001",
        "ordinal": 1,
        "role_id": "designer",
        "workflow_job_id": "design_gemini"
      },
      "content_sha256": "bac758178fc6ad96e86506138ca4d649d604336cc8686cbe548861502f7aa0e3",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/039/design/gemini/DESIGN.md",
      "session_id": "sess_b454b99024ee4dd4b4bb0c7a6b135ecb"
    },
    {
      "artifact_id": "art_c2c83ef8cd514b7b9a15b608f8c081d4",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_ergonomics"
      },
      "content_sha256": "ccade11cc548310a3a0cfe8aafda60a4174f71d94e937193baed8b8621769c94",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_build_ergonomics",
      "logical_name": "build_review_ergonomics",
      "repo_path": "docs/dogfood/039/review/build/ergonomics/REVIEW.md",
      "session_id": "sess_99db163ac89a4671ad5a43f22abd7fd9"
    },
    {
      "artifact_id": "art_1a730f18e4a540188f663140f921dc46",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: reviewer-gemini-pro-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_ergonomics"
      },
      "content_sha256": "d970157950424a99874082bcc3177829d786b7ef85dea6a3d494c0f5c243e657",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_design_ergonomics",
      "logical_name": "design_review_ergonomics",
      "repo_path": "docs/dogfood/039/review/design/ergonomics/REVIEW.md",
      "session_id": "sess_3fb9d0d1d6814c4e95201b82ffc5507c"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-12T13:10:53Z",
  "jobs": [
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "design_claude_code"
      },
      "dependencies": [],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_claude_code",
      "job_type": "generic",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "design_claude_code"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "design_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_codex",
      "job_type": "generic",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "design_codex"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "designer",
        "workflow_job_id": "design_gemini"
      },
      "dependencies": [],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_gemini",
      "job_type": "generic",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "design_gemini"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_design_ergonomics",
          "latest_verdict": "accept",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design_ergonomics"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_implement",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_ergonomics"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_implement",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_build_ergonomics",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_ergonomics"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_ergonomics"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_design_ergonomics",
      "job_type": "review",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_design_ergonomics"
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
          "depends_on_job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-039-rfc-0037-web-ui-ergonomic-improvements",
    "run_id": "run_8f8f347a99ef4e95993db8c288f2ad59",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T13:10:40Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T11:27:44Z",
      "role_id": "designer",
      "session_id": "sess_b454b99024ee4dd4b4bb0c7a6b135ecb",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T13:10:40Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T11:27:44Z",
      "role_id": "designer",
      "session_id": "sess_e8ff4c2e5ec0424c98864f46a75fedc1",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T13:10:40Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T11:27:44Z",
      "role_id": "designer",
      "session_id": "sess_f2c69a7f36de4a67884678031f4ba12f",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T13:10:40Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T11:55:34Z",
      "role_id": "reviewer",
      "session_id": "sess_3fb9d0d1d6814c4e95201b82ffc5507c",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T13:10:40Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T12:07:14Z",
      "role_id": "implementer",
      "session_id": "sess_58d88f8a36f6473e96bae01290036af2",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T13:10:40Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T12:53:35Z",
      "role_id": "reviewer",
      "session_id": "sess_99db163ac89a4671ad5a43f22abd7fd9",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_1a730f18e4a540188f663140f921dc46",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_design_ergonomics",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_3fb9d0d1d6814c4e95201b82ffc5507c",
      "verdict": "accept",
      "verdict_id": "verdict_25a85f94448e4292b60a027e8ded0844"
    },
    {
      "findings_artifact_id": "art_c2c83ef8cd514b7b9a15b608f8c081d4",
      "job_id": "job_run_8f8f347a99ef4e95993db8c288f2ad59_review_build_ergonomics",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_99db163ac89a4671ad5a43f22abd7fd9",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_65f4cf9cb6784b6a93111390708e8222"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-039-rfc-0037-web-ui-ergonomic-improvements",
    "workflow_version": "2026-05-13"
  }
}
```
