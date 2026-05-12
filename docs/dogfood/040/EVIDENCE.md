# Striatum Evidence Export

Run ID: `run_907a9b013113416ba66aa818f2f5d0d1`
Branch: `striatum/dogfood-040-rfc-0040-mcp-driven-harness`
Run state: `completed`
Exported at: `2026-05-12T21:21:56Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"canceled":2,"completed":12},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-040-rfc-0040-mcp-driven-harness","run_id":"run_907a9b013113416ba66aa818f2f5d0d1","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_77f38cb73e41458083673ab3f20a903a","slug":"designer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_81e827e332514b5d9aeb9ff1c31520f5","slug":"designer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_49a055b75b4b4239b1e59bea0817b4c2","slug":"designer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_b0d0fcef320d40d4b558396aa3c084ca","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_43fe1250258b440ebb1637d1752b2346","slug":"implementer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_6793a624ba544781a110c0d9cdbd881d","slug":"implementer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_072c0f100e954b6d8ef14ade5eb9020f","slug":"reviewer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_ca8d65546aca44eba39676e1e8575d1f","slug":"reviewer-claude_code-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_a243a5a125424f86973e249fc547f140","slug":"reviewer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_fb401c3c3be64b15b3c554e3687d402e","slug":"implementer-codex-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_e720ea522bab43539125f26548371d41","slug":"reviewer-codex-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"coordinator","run_id":"<redacted-free-text>","session_id":"sess_163da345407346b887c625b388b6e956","slug":"coordinator-codex-1","state":"closed","supervisor_id":null}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.28.0' templates_changed=['claude_code/README.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`","plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.28.0' templates_changed=['gemini/GEMINI.md.tmpl', 'gemini/README.md.tmpl', 'gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.28.0' templates_changed=['codex/README.md.tmpl', 'codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_809b9b423f0347a89c62bd516851916e",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: implementer-claude-opus-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_ergonomics_claude"
      },
      "content_sha256": "561d3d501409c6c079381188b83b93dd2f76ec9691ea18cb8251b0701a5cc502",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_ergonomics_claude",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/040/BUILD_HANDOFF.md",
      "session_id": "sess_6793a624ba544781a110c0d9cdbd881d"
    },
    {
      "artifact_id": "art_6d22774668a547a7af880eb851cef662",
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
      "content_sha256": "c9d267daa26d9343df6d77f5d5e83366a3e706e663e6fc53adc71c0eeb235eef",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/040/DESIGN_SYNTHESIS.md",
      "session_id": "sess_77f38cb73e41458083673ab3f20a903a"
    },
    {
      "artifact_id": "art_0028b74a87b94d17b203970af4d31f70",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: implementer-claude-opus-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_ergonomics_claude"
      },
      "content_sha256": "d10e2ad69e1761e31efb5d03cdb0767f0194ce1763d8aac910fdfda4e45ec4fa",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_ergonomics_claude",
      "logical_name": "implement_ergonomics_handoff",
      "repo_path": "docs/dogfood/040/build/ergonomics/HANDOFF.md",
      "session_id": "sess_6793a624ba544781a110c0d9cdbd881d"
    },
    {
      "artifact_id": "art_f839ac09a1e947fdab50c7c446131752",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-002",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-002",
        "ordinal": 2,
        "role_id": "implementer",
        "workflow_job_id": "implement_systems_codex"
      },
      "content_sha256": "d307340845add4bd37ab6acf3f276f2b0eda99ee527fe85b4aa097bff88e3ec6",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex_a2",
      "logical_name": "implement_systems_handoff",
      "repo_path": "docs/dogfood/040/build/systems/HANDOFF.md",
      "session_id": "sess_fb401c3c3be64b15b3c554e3687d402e"
    },
    {
      "artifact_id": "art_64335c19bb5642a8b8e4fbb3e323c457",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_systems_codex"
      },
      "content_sha256": "ecc10af8448beeb2beb99473c2a1b5ad3c8cd24e6fd5708819c80c5fd46c7744",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex",
      "logical_name": "implement_systems_handoff",
      "repo_path": "docs/dogfood/040/build/systems/HANDOFF.md",
      "session_id": "sess_43fe1250258b440ebb1637d1752b2346"
    },
    {
      "artifact_id": "art_824bae7aaeeb4a4d8b9af01489dd878f",
      "artifact_kind": "decision",
      "content_sha256": "f0744760b1539f475e7c709fb3fe8044294111195777c67b74164ac74c10c5d1",
      "job_id": null,
      "logical_name": "dec_af557de1402d44489c0b9af7c93b0a5c",
      "repo_path": "docs/dogfood/040/decisions/cycle-exhaustion-codex-build-review.md",
      "session_id": null
    },
    {
      "artifact_id": "art_6d1a6455a7364ffbae37f5b76c875b1d",
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
      "content_sha256": "8e66861c116722521f7e5aa29b15d8550307a2b41e6d10f75760d29ff3b61762",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/040/design/claude_code/DESIGN.md",
      "session_id": "sess_81e827e332514b5d9aeb9ff1c31520f5"
    },
    {
      "artifact_id": "art_c687a3a4ec50421e83b6fa3fbbebfb2e",
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
      "content_sha256": "aa4a612a5c8afcb7a5209fb86f18046589341c20354519115a9e075b0e98eb59",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/040/design/codex/DESIGN.md",
      "session_id": "sess_77f38cb73e41458083673ab3f20a903a"
    },
    {
      "artifact_id": "art_3aba4168ba554de7818cc2c06edb5d4d",
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
      "content_sha256": "2351925783b54533c33d00a68a423ca81c8fdc5a2ce5e0cc2485d8c32adfd194",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/040/design/gemini/DESIGN.md",
      "session_id": "sess_49a055b75b4b4239b1e59bea0817b4c2"
    },
    {
      "artifact_id": "art_7eb4a43b58554e1badb0f3bf723d6f07",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-002",
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_claude"
      },
      "content_sha256": "8d09e453021e041725f3fbd0a69624effd42ed7204e368d650bdb070f08edb71",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_claude",
      "logical_name": "build_review_claude",
      "repo_path": "docs/dogfood/040/review/build/claude/REVIEW.md",
      "session_id": "sess_ca8d65546aca44eba39676e1e8575d1f"
    },
    {
      "artifact_id": "art_3643fc95131c40adb6d36e5be59142c6",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: reviewer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_codex"
      },
      "content_sha256": "60101374c7df1e357214c77cef7e0ed4e5ebd4508f2a3568fea85da98051480d",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex",
      "logical_name": "build_review_codex",
      "repo_path": "docs/dogfood/040/review/build/codex/REVIEW.md",
      "session_id": "sess_072c0f100e954b6d8ef14ade5eb9020f"
    },
    {
      "artifact_id": "art_15aaad55c4ab408c995338ba102be2ee",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-codex-gpt-5.5-002",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: reviewer-codex-gpt-5.5-002",
        "ordinal": 2,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_codex"
      },
      "content_sha256": "de5fd1833636d1ad44c9e26b1959b2e9674b43af1945ab710515e1a747dbbd58",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex_a2",
      "logical_name": "build_review_codex",
      "repo_path": "docs/dogfood/040/review/build/codex/REVIEW.md",
      "session_id": "sess_e720ea522bab43539125f26548371d41"
    },
    {
      "artifact_id": "art_783db8c1c4be4493901db89ca9505811",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-gemini-pro-001",
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": "author: reviewer-gemini-pro-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_gemini"
      },
      "content_sha256": "31781a3dc9bf014c1fc3f825f075f877b059a577af155097c37a0964932dcd8a",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_gemini",
      "logical_name": "build_review_gemini",
      "repo_path": "docs/dogfood/040/review/build/gemini/REVIEW.md",
      "session_id": "sess_a243a5a125424f86973e249fc547f140"
    },
    {
      "artifact_id": "art_a2a7d40ace9043a198a98bd4532c4e02",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_threat"
      },
      "content_sha256": "710f67cc8fa076e77025005a27adc60158775b7578d8236bac3523bdba90135e",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_design_threat",
      "logical_name": "design_review_threat",
      "repo_path": "docs/dogfood/040/review/design/threat/REVIEW.md",
      "session_id": "sess_b0d0fcef320d40d4b558396aa3c084ca"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-12T21:21:56Z",
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
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_claude_code",
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
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_codex",
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
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_gemini",
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
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_ergonomics_claude"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_design_threat",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design_threat"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_ergonomics_claude",
      "job_type": "build",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_ergonomics_claude"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_systems_codex"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_design_threat",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design_threat"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_systems_codex"
    },
    {
      "attempt": 2,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_systems_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex_a2",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_systems_codex"
    },
    {
      "attempt": 3,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_systems_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex_a3",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "canceled",
      "workflow_job_id": "implement_systems_codex"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_claude"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_ergonomics_claude",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_ergonomics_claude"
        },
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_systems_codex"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_claude",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_claude"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_codex"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_ergonomics_claude",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_ergonomics_claude"
        },
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_systems_codex"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex",
      "job_type": "review",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_codex"
    },
    {
      "attempt": 2,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_codex"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex_a2",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_systems_codex"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex_a2",
      "job_type": "review",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_codex"
    },
    {
      "attempt": 3,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_codex"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex_a3",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "canceled",
          "workflow_job_id": "implement_systems_codex"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex_a3",
      "job_type": "review",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "canceled",
      "workflow_job_id": "review_build_codex"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Gemini Pro",
        "lane_id": "gemini",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_gemini"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_ergonomics_claude",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_ergonomics_claude"
        },
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_implement_systems_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_systems_codex"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_gemini",
      "job_type": "review",
      "lane": "gemini",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_gemini"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_threat"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_design_threat",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_design_threat"
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
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-040-rfc-0040-mcp-driven-harness",
    "run_id": "run_907a9b013113416ba66aa818f2f5d0d1",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T15:03:06Z",
      "role_id": "designer",
      "session_id": "sess_49a055b75b4b4239b1e59bea0817b4c2",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T15:03:06Z",
      "role_id": "designer",
      "session_id": "sess_77f38cb73e41458083673ab3f20a903a",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T15:03:06Z",
      "role_id": "designer",
      "session_id": "sess_81e827e332514b5d9aeb9ff1c31520f5",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T19:28:21Z",
      "role_id": "reviewer",
      "session_id": "sess_b0d0fcef320d40d4b558396aa3c084ca",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T19:40:07Z",
      "role_id": "implementer",
      "session_id": "sess_43fe1250258b440ebb1637d1752b2346",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T19:40:07Z",
      "role_id": "implementer",
      "session_id": "sess_6793a624ba544781a110c0d9cdbd881d",
      "slug": "implementer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T20:09:13Z",
      "role_id": "reviewer",
      "session_id": "sess_072c0f100e954b6d8ef14ade5eb9020f",
      "slug": "reviewer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T20:09:13Z",
      "role_id": "reviewer",
      "session_id": "sess_a243a5a125424f86973e249fc547f140",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T20:09:13Z",
      "role_id": "reviewer",
      "session_id": "sess_ca8d65546aca44eba39676e1e8575d1f",
      "slug": "reviewer-claude_code-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T20:31:46Z",
      "role_id": "implementer",
      "session_id": "sess_fb401c3c3be64b15b3c554e3687d402e",
      "slug": "implementer-codex-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T21:03:31Z",
      "role_id": "reviewer",
      "session_id": "sess_e720ea522bab43539125f26548371d41",
      "slug": "reviewer-codex-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-12T21:21:36Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T21:20:18Z",
      "role_id": "coordinator",
      "session_id": "sess_163da345407346b887c625b388b6e956",
      "slug": "coordinator-codex-1",
      "state": "closed",
      "supervisor_id": null
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_a2a7d40ace9043a198a98bd4532c4e02",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_design_threat",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_b0d0fcef320d40d4b558396aa3c084ca",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_268afe531ada48c59f6cb54c4279c4fd"
    },
    {
      "findings_artifact_id": "art_3643fc95131c40adb6d36e5be59142c6",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_072c0f100e954b6d8ef14ade5eb9020f",
      "verdict": "needs_revision",
      "verdict_id": "verdict_e19e5421797e418091871f362399ff85"
    },
    {
      "findings_artifact_id": "art_7eb4a43b58554e1badb0f3bf723d6f07",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_claude",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_ca8d65546aca44eba39676e1e8575d1f",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_e0c0707c22bc4376b22e7deaede3a35a"
    },
    {
      "findings_artifact_id": "art_783db8c1c4be4493901db89ca9505811",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_gemini",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_a243a5a125424f86973e249fc547f140",
      "verdict": "accept",
      "verdict_id": "verdict_1e0e12a970b441d1aa892440eae93ff4"
    },
    {
      "findings_artifact_id": "art_15aaad55c4ab408c995338ba102be2ee",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex_a2",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_e720ea522bab43539125f26548371d41",
      "verdict": "needs_revision",
      "verdict_id": "verdict_1b8f3f38441740c2ad7e151b205f56ff"
    },
    {
      "findings_artifact_id": "art_15aaad55c4ab408c995338ba102be2ee",
      "job_id": "job_run_907a9b013113416ba66aa818f2f5d0d1_review_build_codex_a2",
      "posture": "threat_model",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_163da345407346b887c625b388b6e956",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_205f6c8b87144de286f02d4e223a1e17"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-040-rfc-0040-mcp-driven-harness",
    "workflow_version": "2026-05-13"
  }
}
```
