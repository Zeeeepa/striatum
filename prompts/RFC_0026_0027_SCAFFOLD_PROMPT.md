# RFC 0026/0027 Dogfood Scaffold Prompt

Status: reusable
Date: 2026-05-10
author: coordinator-codex-gpt-5.5-001

Use this prompt to ask a fresh CLI agent to scaffold, validate, and stop on a
Striatum dogfood workflow for RFC 0026 and RFC 0027. The prompt intentionally
includes setup commands from session inception.

```text
You are working in /Users/halbritt/git/striatum.

Goal: from a fresh agent session, set up Striatum guidance, scaffold a full
dogfood workflow for RFC 0026 + RFC 0027, validate it, and stop. Do not
implement the RFCs and do not start the run.

Run these commands first:

pwd
git status --short --branch
git pull --ff-only
make install

Load Striatum agent guidance in user scope, not project scope.

If running Codex:

PYTHONPATH=src python3 -m striatum.cli skills install --profile codex --scope user --force --json
PYTHONPATH=src python3 -m striatum.cli plugin install --profile codex --scope user --force --with-marketplace --json

If running Claude Code:

PYTHONPATH=src python3 -m striatum.cli skills install --profile claude_code --scope user --force --json
PYTHONPATH=src python3 -m striatum.cli plugin install --profile claude_code --scope user --force --with-marketplace --json

If running Gemini:

PYTHONPATH=src python3 -m striatum.cli skills install --profile gemini --scope user --force --json
PYTHONPATH=src python3 -m striatum.cli plugin install --profile gemini --scope user --force --json

If unsure which CLI profile applies:

PYTHONPATH=src python3 -m striatum.cli skills install --profile generic --scope user --force --json

Read the generated Striatum workflow/claim-loop guidance returned by those
commands. If a native plugin loader is available, load the installed Striatum
plugin too. Generated user-scope skill/plugin files are setup only; do not
stage or commit them.

Now read, in order:

AGENTS.md
README.md
docs/INDEX.md
docs/SPEC.md
docs/DECISION_LOG.md
docs/UBIQUITOUS_LANGUAGE.md
docs/TODO.md
docs/rfcs/0026-lane-attestation-and-operator-byline-honesty.md
docs/rfcs/0027-sealed-patch-provenance-mode.md
docs/dogfood/003/workflow.json
docs/dogfood/029/workflow.json

Find the next dogfood id:

ls docs/dogfood | sort

Use the next numeric id after the highest existing numeric dogfood directory,
likely docs/dogfood/030/. If 030 already exists, use the next available id.

Create:

docs/dogfood/<id>/workflow.json
docs/dogfood/<id>/roles/coordinator.md
docs/dogfood/<id>/roles/designer.md
docs/dogfood/<id>/roles/reviewer.md
docs/dogfood/<id>/roles/implementer.md
docs/dogfood/<id>/prompts/design_codex.md
docs/dogfood/<id>/prompts/design_claude_code.md
docs/dogfood/<id>/prompts/design_gemini.md
docs/dogfood/<id>/prompts/synthesize_design.md
docs/dogfood/<id>/prompts/review_design.md
docs/dogfood/<id>/prompts/implement.md
docs/dogfood/<id>/prompts/review_build.md

Workflow shape:

- schema_version: striatum.workflow.v1
- branch mode: auto
- suggested branch: striatum/dogfood-<id>-rfc-0026-0027-provenance
- coordinator: codex lane
- lanes: codex, claude_code, gemini
- parallelism: declared, max_active_jobs at least 3, require_disjoint_write_scopes true
- three parallel fresh design jobs:
  - design_codex -> docs/dogfood/<id>/design/codex/DESIGN.md
  - design_claude_code -> docs/dogfood/<id>/design/claude_code/DESIGN.md
  - design_gemini -> docs/dogfood/<id>/design/gemini/DESIGN.md
- synthesis job:
  - synthesize_design -> docs/dogfood/<id>/DESIGN_SYNTHESIS.md
  - must reconcile RFC 0026 and RFC 0027 explicitly
- three adversarial fresh design reviews:
  - review_design_devils, posture devils_advocate
  - review_design_security, posture security
  - review_design_threat, posture threat_model
- implement job:
  - required_review_postures: devils_advocate, security, threat_model
  - blocked until all design reviews have accepting verdicts
- build reviews:
  - at least devils_advocate and security
- design reviews use reviewer_access_scope artifact_augmented
- build reviews use reviewer_access_scope repo_level
- all reviews use reviewer_context_policy fresh
- reviews use review_only_artifact write scopes
- include bounded needs_revision cycles from design reviews back to synthesize_design and build reviews back to implement
- forbidden_paths must include .striatum/ or .striatum/state.sqlite3 everywhere appropriate

Implementation write scope should be tight and include only likely surfaces:

src/striatum/
tests/
docs/SPEC.md
docs/UBIQUITOUS_LANGUAGE.md
docs/DECISION_LOG.md
docs/TODO.md
docs/rfcs/0026-lane-attestation-and-operator-byline-honesty.md
docs/rfcs/0027-sealed-patch-provenance-mode.md
docs/rfcs/README.md
docs/dogfood/<id>/
README.md
CHANGELOG.md
pyproject.toml

Prompt requirements:

- Design prompts ask for implementation plan, schema/API changes, migrations,
  compatibility risk, test plan, staging plan.
- Synthesis prompt demands one implementation plan with phases,
  accepted/deferred scope, and human-decision questions.
- Design review prompt attacks false provenance claims, local operator
  bypasses, attestation ambiguity, migration risk, cross-platform containment
  gaps, and over-broad write scopes.
- Implement prompt says implementation is blocked until design reviews have
  accepting verdicts.
- Build review prompt verifies behavior, tests, docs, and no overclaiming of
  model-token or decision provenance.

After writing files, run:

PYTHONPATH=src python3 -m striatum.cli workflow validate docs/dogfood/<id>/workflow.json --json
git diff --check
git status --short

Do not run these yet, but include them in your final answer as the exact next
commands for the human to begin the run:

PYTHONPATH=src python3 -m striatum.cli --repo /Users/halbritt/git/striatum init --json
PYTHONPATH=src python3 -m striatum.cli --repo /Users/halbritt/git/striatum workflow validate docs/dogfood/<id>/workflow.json --json
PYTHONPATH=src python3 -m striatum.cli --repo /Users/halbritt/git/striatum run prepare --workflow docs/dogfood/<id>/workflow.json --json
PYTHONPATH=src python3 -m striatum.cli --repo /Users/halbritt/git/striatum run start --run-id <run_id> --json
PYTHONPATH=src python3 -m striatum.cli --repo /Users/halbritt/git/striatum dashboard --run-id <run_id> --once

Report:

- files created
- validation result
- next-run commands
- assumptions
- anything you deliberately left out
```
