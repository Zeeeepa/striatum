# Striatum Evidence Export

Run ID: `run_ea41c27b6fc34fa1a3a44e6f694caf96`
Branch: `striatum/dogfood-041-rfc-0038-ui-features`
Run state: `completed`
Exported at: `2026-05-13T00:22:02Z`

Live SQLite state remains ignored under `.striatum/` and is not part of this export.

## Status Output

```json
{"blocked_downstream_jobs":[],"claimable_jobs":[],"human_checkpoints":[],"jobs":{"canceled":2,"completed":14},"latest_non_accepting_review_verdicts":[],"next_actions":[],"open_blockers":[],"process_health":"<redacted-free-text>","provenance_mode":"<redacted-free-text>","runs":[{"branch_name":"striatum/dogfood-041-rfc-0038-ui-features","run_id":"run_ea41c27b6fc34fa1a3a44e6f694caf96","state":"completed"}],"sessions":[{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_e285a8a9ccf346e9b7f4b0b23e28ff1f","slug":"designer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_f2e83365d9d54f238c123e310c0adc67","slug":"designer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"designer","run_id":"<redacted-free-text>","session_id":"sess_f75ae65f49194fc3969884416fd700cf","slug":"designer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_d31af55e9a114e1c9708e5ce3c05ec7a","slug":"reviewer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_71cdfa557074476c81691e7cb1a51c95","slug":"implementer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_101129bd1fa14ff8a62541d9da095a0f","slug":"implementer-claude_code-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_41bb1715d08b495bb0c4217676f1ff02","slug":"reviewer-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_58b124be37f1401d8de9650878e12ce2","slug":"reviewer-claude_code-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"gemini","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_0f78af3efa004db9a36c8148407171a7","slug":"reviewer-gemini-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_b1521c67c2e147eba4d1959361556ffc","slug":"implementer-codex-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"implementer","run_id":"<redacted-free-text>","session_id":"sess_4ac291f666f34ae0a397f5d658912a6a","slug":"implementer-claude_code-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"coordinator","run_id":"<redacted-free-text>","session_id":"sess_d6f5a47fff8144c9a14fb2599884b251","slug":"coordinator-codex-1","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_762e6d2c6a7b48b994024e182ade598d","slug":"reviewer-codex-2","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"claude_code","operator_label":null,"pid":null,"role_id":"reviewer","run_id":"<redacted-free-text>","session_id":"sess_4bd04c2ee2ea4461980d26f5de30e185","slug":"reviewer-claude_code-3","state":"closed","supervisor_id":null},{"lane_attestation":"<redacted-free-text>","lane_attestation_reason":"<redacted-free-text>","lane_id":"codex","operator_label":null,"pid":null,"role_id":"coordinator","run_id":"<redacted-free-text>","session_id":"sess_22175cd7104043d39dce0e6eebca371a","slug":"coordinator-codex-2","state":"closed","supervisor_id":null}],"verdicts_by_posture":"<redacted-free-text>"}
```

## Doctor Output

```json
{"ok":false,"problems":["plugin bundle outdated for profile 'gemini': manifest_version='1.20.1' running_version='1.28.0' templates_changed=['gemini/GEMINI.md.tmpl', 'gemini/README.md.tmpl', 'gemini/agents/striatum-recover.md.tmpl', 'gemini/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile gemini`","plugin bundle outdated for profile 'claude_code': manifest_version='1.20.1' running_version='1.28.0' templates_changed=['claude_code/README.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile claude_code`","plugin bundle outdated for profile 'codex': manifest_version='1.20.1' running_version='1.28.0' templates_changed=['codex/README.md.tmpl', 'codex/skills/recover.md.tmpl'] \u2014 run `striatum --repo /home/halbritt/git/striatum plugin install --profile codex`"],"schema_version":"1"}
```

## Snapshot

```json
{
  "artifacts": [
    {
      "artifact_id": "art_45386e82f4a649a49abdd3a75b20cf14",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: implementer-claude-opus-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_components_claude"
      },
      "content_sha256": "55fe8cddbaff13b8a167090e80bf865335a27b685faa3dbbf442c600dfda0c07",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/041/BUILD_HANDOFF.md",
      "session_id": "sess_101129bd1fa14ff8a62541d9da095a0f"
    },
    {
      "artifact_id": "art_6e9c37e7b8c04974aadc0b24aa2052b3",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: implementer-claude-opus-002",
        "ordinal": 2,
        "role_id": "implementer",
        "workflow_job_id": "implement_components_claude"
      },
      "content_sha256": "b48be5f74a9893c51d5d1bcc67b884feac5a71a2921fec7031783e7ba63224b8",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude_a2",
      "logical_name": "build_handoff",
      "repo_path": "docs/dogfood/041/BUILD_HANDOFF.md",
      "session_id": "sess_4ac291f666f34ae0a397f5d658912a6a"
    },
    {
      "artifact_id": "art_1940c43292b7462d9e8eeb8e62c87ef5",
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
      "content_sha256": "b356cae647eb4bf62c98a84d9fe8b82f832c37d9295cc6383eaca1c31efbb339",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_synthesize_design",
      "logical_name": "design_synthesis",
      "repo_path": "docs/dogfood/041/DESIGN_SYNTHESIS.md",
      "session_id": "sess_e285a8a9ccf346e9b7f4b0b23e28ff1f"
    },
    {
      "artifact_id": "art_bbf596af36ce47268e449362ad397fa3",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-claude-opus-002",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: implementer-claude-opus-002",
        "ordinal": 2,
        "role_id": "implementer",
        "workflow_job_id": "implement_components_claude"
      },
      "content_sha256": "63ccda7d8bec1f0b550f9d219332f7376b772ebebf2ced67290b5fae89ecf514",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude_a2",
      "logical_name": "implement_components_handoff",
      "repo_path": "docs/dogfood/041/build/components/HANDOFF.md",
      "session_id": "sess_4ac291f666f34ae0a397f5d658912a6a"
    },
    {
      "artifact_id": "art_002b8b1f428448b0bff0cf88b79cee34",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: implementer-claude-opus-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_components_claude"
      },
      "content_sha256": "a004ef087de225c8f1d7160fc8927a3fe0c75a7978588edf86990178168ee8b2",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude",
      "logical_name": "implement_components_handoff",
      "repo_path": "docs/dogfood/041/build/components/HANDOFF.md",
      "session_id": "sess_101129bd1fa14ff8a62541d9da095a0f"
    },
    {
      "artifact_id": "art_588b43b8b3254b0db2224e0dce7e2d8b",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-001",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-001",
        "ordinal": 1,
        "role_id": "implementer",
        "workflow_job_id": "implement_toolchain_codex"
      },
      "content_sha256": "2613b9f6e78ac44aabe6683274c69815f316c01f2cc2386e92642d4548e621ff",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex",
      "logical_name": "implement_toolchain_handoff",
      "repo_path": "docs/dogfood/041/build/toolchain/HANDOFF.md",
      "session_id": "sess_71cdfa557074476c81691e7cb1a51c95"
    },
    {
      "artifact_id": "art_a38ab66cba664babb7135d1226680e29",
      "artifact_kind": "handoff",
      "author": {
        "actual_author_line": "author: implementer-codex-gpt-5.5-002",
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": "author: implementer-codex-gpt-5.5-002",
        "ordinal": 2,
        "role_id": "implementer",
        "workflow_job_id": "implement_toolchain_codex"
      },
      "content_sha256": "7f1274fdd49d48d1387ad599adafb643b576978703f28f3df546ac4b1fb32f77",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex_a2",
      "logical_name": "implement_toolchain_handoff",
      "repo_path": "docs/dogfood/041/build/toolchain/HANDOFF.md",
      "session_id": "sess_b1521c67c2e147eba4d1959361556ffc"
    },
    {
      "artifact_id": "art_784d146af71943589d5a67c287018f88",
      "artifact_kind": "decision",
      "content_sha256": "35514c2d29d70c37be296294430548ed0a034fec31a724fe58666c210aab86a0",
      "job_id": null,
      "logical_name": "dec_251e8a5f3d674c409de0dad9eacd5844",
      "repo_path": "docs/dogfood/041/decisions/cycle-exhaustion-codex-build-review.md",
      "session_id": null
    },
    {
      "artifact_id": "art_98072b2249e9434db8ba2660b8dfd5a6",
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
      "content_sha256": "93a082435895fa465a6731524f7bcff4e142508cb4191002445ebffc0e89f503",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_claude_code",
      "logical_name": "claude_code_design",
      "repo_path": "docs/dogfood/041/design/claude_code/DESIGN.md",
      "session_id": "sess_f2e83365d9d54f238c123e310c0adc67"
    },
    {
      "artifact_id": "art_2bf7f1d2514d41079ffce3f9d405ef96",
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
      "content_sha256": "27a5c8a278d876cf824ab39ca6a75a463b61c348cf4d1a27221240d65408560f",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_codex",
      "logical_name": "codex_design",
      "repo_path": "docs/dogfood/041/design/codex/DESIGN.md",
      "session_id": "sess_e285a8a9ccf346e9b7f4b0b23e28ff1f"
    },
    {
      "artifact_id": "art_9fe295ce7aa24463b999592af2661924",
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
      "content_sha256": "f38517e2d48290e6e764ea79edcf56d5d33946ebf57782ef70e000e6ce6ca442",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_gemini",
      "logical_name": "gemini_design",
      "repo_path": "docs/dogfood/041/design/gemini/DESIGN.md",
      "session_id": "sess_f75ae65f49194fc3969884416fd700cf"
    },
    {
      "artifact_id": "art_b5c56878c7734287ba98748089cefc26",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-003",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-003",
        "ordinal": 3,
        "role_id": "reviewer",
        "workflow_job_id": "review_build_claude"
      },
      "content_sha256": "a20fd92f22bbe0fb9436ef03e1446e43de8397d9f1e66b3ce5c0b9902774da6f",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_claude_a2",
      "logical_name": "build_review_claude",
      "repo_path": "docs/dogfood/041/review/build/claude/REVIEW.md",
      "session_id": "sess_4bd04c2ee2ea4461980d26f5de30e185"
    },
    {
      "artifact_id": "art_21cab46a1f144030843423b855b5124b",
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
      "content_sha256": "de71c26f628c5f9e8760f05d5e4957512390d9a446d99843b1f0882805e898e6",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_claude",
      "logical_name": "build_review_claude",
      "repo_path": "docs/dogfood/041/review/build/claude/REVIEW.md",
      "session_id": "sess_58b124be37f1401d8de9650878e12ce2"
    },
    {
      "artifact_id": "art_c2d6792973d34a0ebdbf270f5e4efb95",
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
      "content_sha256": "b0166842faf9e8ca3eea7135193932504a5f82762b99a2882e59c0bb9bea6a1e",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex_a2",
      "logical_name": "build_review_codex",
      "repo_path": "docs/dogfood/041/review/build/codex/REVIEW.md",
      "session_id": "sess_762e6d2c6a7b48b994024e182ade598d"
    },
    {
      "artifact_id": "art_d82d36d8cb684895a1f05047a1233d0e",
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
      "content_sha256": "bff8fbb6adf2e433f426faec8319f8ed9a5aef4129f9274a8a7cc1560e1fe1a2",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex",
      "logical_name": "build_review_codex",
      "repo_path": "docs/dogfood/041/review/build/codex/REVIEW.md",
      "session_id": "sess_41bb1715d08b495bb0c4217676f1ff02"
    },
    {
      "artifact_id": "art_eff83584582d43c0886304c421b5d9c9",
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
      "content_sha256": "d7dd4444c615d6b9b41f2353fcdc52ed4673ee316f66a2b651252ae2bc263569",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_gemini",
      "logical_name": "build_review_gemini",
      "repo_path": "docs/dogfood/041/review/build/gemini/REVIEW.md",
      "session_id": "sess_0f78af3efa004db9a36c8148407171a7"
    },
    {
      "artifact_id": "art_fecce37c878946b78cee6ca348a17471",
      "artifact_kind": "finding",
      "author": {
        "actual_author_line": "author: reviewer-claude-opus-001",
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": "author: reviewer-claude-opus-001",
        "ordinal": 1,
        "role_id": "reviewer",
        "workflow_job_id": "review_design_ergonomics"
      },
      "content_sha256": "324d3f27b5e9c7e906de23a9ca78155defa7acb28ddaf0707dcaaa8f9be7d2c0",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_design_ergonomics",
      "logical_name": "design_review_ergonomics",
      "repo_path": "docs/dogfood/041/review/design/ergonomics/REVIEW.md",
      "session_id": "sess_d31af55e9a114e1c9708e5ce3c05ec7a"
    }
  ],
  "blocked_downstream_jobs": [],
  "blockers": [],
  "exported_at": "2026-05-13T00:22:02Z",
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
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_claude_code",
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
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_codex",
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
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_gemini",
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
        "workflow_job_id": "implement_components_claude"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_design_ergonomics",
          "latest_verdict": "accept_with_findings",
          "required_verdicts": [
            "accept",
            "accept_with_findings"
          ],
          "state": "completed",
          "workflow_job_id": "review_design_ergonomics"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude",
      "job_type": "build",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_components_claude"
    },
    {
      "attempt": 2,
      "author": {
        "display_model": "Claude Opus",
        "lane_id": "claude_code",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_components_claude"
      },
      "dependencies": [],
      "display_model": "Claude Opus",
      "fresh_session_required": false,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude_a2",
      "job_type": "build",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_components_claude"
    },
    {
      "attempt": 1,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_toolchain_codex"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_design_ergonomics",
          "latest_verdict": "accept_with_findings",
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
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_toolchain_codex"
    },
    {
      "attempt": 2,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_toolchain_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex_a2",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "completed",
      "workflow_job_id": "implement_toolchain_codex"
    },
    {
      "attempt": 3,
      "author": {
        "display_model": "Codex GPT-5.5",
        "lane_id": "codex",
        "line": null,
        "ordinal": null,
        "role_id": "implementer",
        "workflow_job_id": "implement_toolchain_codex"
      },
      "dependencies": [],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex_a3",
      "job_type": "build",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "implementer",
      "state": "canceled",
      "workflow_job_id": "implement_toolchain_codex"
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_components_claude"
        },
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_toolchain_codex"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_claude",
      "job_type": "review",
      "lane": "claude_code",
      "max_attempts": 1,
      "role_id": "reviewer",
      "state": "completed",
      "workflow_job_id": "review_build_claude"
    },
    {
      "attempt": 2,
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude_a2",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_components_claude"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_claude_a2",
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_components_claude"
        },
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_toolchain_codex"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex",
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex_a2",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_toolchain_codex"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex_a2",
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex_a3",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "canceled",
          "workflow_job_id": "implement_toolchain_codex"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex_a3",
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_components_claude",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_components_claude"
        },
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_implement_toolchain_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "implement_toolchain_codex"
        }
      ],
      "display_model": "Gemini Pro",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_gemini",
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
        "workflow_job_id": "review_design_ergonomics"
      },
      "dependencies": [
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_synthesize_design",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "synthesize_design"
        }
      ],
      "display_model": "Claude Opus",
      "fresh_session_required": true,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_design_ergonomics",
      "job_type": "review",
      "lane": "claude_code",
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
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_claude_code",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_claude_code"
        },
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_codex",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_codex"
        },
        {
          "depends_on_job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_design_gemini",
          "latest_verdict": null,
          "required_verdicts": null,
          "state": "completed",
          "workflow_job_id": "design_gemini"
        }
      ],
      "display_model": "Codex GPT-5.5",
      "fresh_session_required": false,
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_synthesize_design",
      "job_type": "synthesis",
      "lane": "codex",
      "max_attempts": 1,
      "role_id": "designer",
      "state": "completed",
      "workflow_job_id": "synthesize_design"
    }
  ],
  "run": {
    "branch_name": "striatum/dogfood-041-rfc-0038-ui-features",
    "run_id": "run_ea41c27b6fc34fa1a3a44e6f694caf96",
    "state": "completed"
  },
  "schema_version": "striatum.evidence.v1",
  "sessions": [
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T21:35:16Z",
      "role_id": "designer",
      "session_id": "sess_e285a8a9ccf346e9b7f4b0b23e28ff1f",
      "slug": "designer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T21:35:16Z",
      "role_id": "designer",
      "session_id": "sess_f2e83365d9d54f238c123e310c0adc67",
      "slug": "designer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T21:35:16Z",
      "role_id": "designer",
      "session_id": "sess_f75ae65f49194fc3969884416fd700cf",
      "slug": "designer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T22:02:26Z",
      "role_id": "reviewer",
      "session_id": "sess_d31af55e9a114e1c9708e5ce3c05ec7a",
      "slug": "reviewer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T22:13:44Z",
      "role_id": "implementer",
      "session_id": "sess_101129bd1fa14ff8a62541d9da095a0f",
      "slug": "implementer-claude_code-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T22:13:44Z",
      "role_id": "implementer",
      "session_id": "sess_71cdfa557074476c81691e7cb1a51c95",
      "slug": "implementer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "gemini",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T22:56:54Z",
      "role_id": "reviewer",
      "session_id": "sess_0f78af3efa004db9a36c8148407171a7",
      "slug": "reviewer-gemini-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T22:56:54Z",
      "role_id": "reviewer",
      "session_id": "sess_41bb1715d08b495bb0c4217676f1ff02",
      "slug": "reviewer-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_failed",
      "closed_at": "2026-05-12T23:08:54Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T22:56:54Z",
      "role_id": "reviewer",
      "session_id": "sess_58b124be37f1401d8de9650878e12ce2",
      "slug": "reviewer-claude_code-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-13T00:21:43Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T23:09:46Z",
      "role_id": "implementer",
      "session_id": "sess_4ac291f666f34ae0a397f5d658912a6a",
      "slug": "implementer-claude_code-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-13T00:21:43Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T23:09:46Z",
      "role_id": "implementer",
      "session_id": "sess_b1521c67c2e147eba4d1959361556ffc",
      "slug": "implementer-codex-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-13T00:21:43Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 1,
      "registered_at": "2026-05-12T23:10:26Z",
      "role_id": "coordinator",
      "session_id": "sess_d6f5a47fff8144c9a14fb2599884b251",
      "slug": "coordinator-codex-1",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-13T00:21:43Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "claude_code",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 3,
      "registered_at": "2026-05-12T23:58:39Z",
      "role_id": "reviewer",
      "session_id": "sess_4bd04c2ee2ea4461980d26f5de30e185",
      "slug": "reviewer-claude_code-3",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-13T00:21:43Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-12T23:58:39Z",
      "role_id": "reviewer",
      "session_id": "sess_762e6d2c6a7b48b994024e182ade598d",
      "slug": "reviewer-codex-2",
      "state": "closed",
      "supervisor_id": null
    },
    {
      "close_reason": "run_completed",
      "closed_at": "2026-05-13T00:21:43Z",
      "lane_attestation": "<redacted-free-text>",
      "lane_attestation_reason": "<redacted-free-text>",
      "lane_id": "codex",
      "non_fresh_reason": null,
      "operator_label": null,
      "ordinal": 2,
      "registered_at": "2026-05-13T00:21:31Z",
      "role_id": "coordinator",
      "session_id": "sess_22175cd7104043d39dce0e6eebca371a",
      "slug": "coordinator-codex-2",
      "state": "closed",
      "supervisor_id": null
    }
  ],
  "verdicts": [
    {
      "findings_artifact_id": "art_fecce37c878946b78cee6ca348a17471",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_design_ergonomics",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_d31af55e9a114e1c9708e5ce3c05ec7a",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_255bc5fad8c448ac84827c62eaae7b84"
    },
    {
      "findings_artifact_id": "art_d82d36d8cb684895a1f05047a1233d0e",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_41bb1715d08b495bb0c4217676f1ff02",
      "verdict": "needs_revision",
      "verdict_id": "verdict_636f20228b0848e68cb5674d60af45b9"
    },
    {
      "findings_artifact_id": "art_21cab46a1f144030843423b855b5124b",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_claude",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_58b124be37f1401d8de9650878e12ce2",
      "verdict": "needs_revision",
      "verdict_id": "verdict_5983c18953e141c4a2474da9bbb3b639"
    },
    {
      "findings_artifact_id": "art_eff83584582d43c0886304c421b5d9c9",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_gemini",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_0f78af3efa004db9a36c8148407171a7",
      "verdict": "reject",
      "verdict_id": "verdict_ba6bb359f38741498e9f39d6cc5af6e7"
    },
    {
      "findings_artifact_id": "art_eff83584582d43c0886304c421b5d9c9",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_gemini",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_d6f5a47fff8144c9a14fb2599884b251",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_2d82779ce20d48c7a0eb99ca46066dd8"
    },
    {
      "findings_artifact_id": "art_c2d6792973d34a0ebdbf270f5e4efb95",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex_a2",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_762e6d2c6a7b48b994024e182ade598d",
      "verdict": "needs_revision",
      "verdict_id": "verdict_a21e77e191a843348419a60519c0cc5c"
    },
    {
      "findings_artifact_id": "art_b5c56878c7734287ba98748089cefc26",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_claude_a2",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_4bd04c2ee2ea4461980d26f5de30e185",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_3de18c72d3164020b5238fea67e67652"
    },
    {
      "findings_artifact_id": "art_c2d6792973d34a0ebdbf270f5e4efb95",
      "job_id": "job_run_ea41c27b6fc34fa1a3a44e6f694caf96_review_build_codex_a2",
      "posture": "ergonomics_dx",
      "rationale": "<redacted-free-text>",
      "session_id": "sess_22175cd7104043d39dce0e6eebca371a",
      "verdict": "accept_with_findings",
      "verdict_id": "verdict_3624ef30972043179eced8ffb576df37"
    }
  ],
  "workflow": {
    "workflow_id": "dogfood-041-rfc-0038-ui-features",
    "workflow_version": "2026-05-13"
  }
}
```
