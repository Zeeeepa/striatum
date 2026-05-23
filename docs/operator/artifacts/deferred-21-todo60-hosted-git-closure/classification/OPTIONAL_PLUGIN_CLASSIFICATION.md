---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/deferred-21-todo60-hosted-git-closure/audit/CORE_BOUNDARY_AUDIT.md", "docs/DECISION_LOG.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/operator/BRIEF.md", "docs/rfcs/0067-optional-git-pr-integration.md"]
---

# TODO 60 Hosted Provider Classification
author: todo60-classifier-codex-gpt-5-001

## Classification

Hosted Git/PR provider actions are optional-plugin/out-of-core work, not
remaining Striatum core work.

The core TODO 60 acceptance envelope is already satisfied by the local-only
slice:

- read-only `git.snapshot`;
- durable `commit_request` and `pr_request` artifact schemas;
- explicit-confirm local `git.commit_apply`.

D127 deliberately leaves hosted provider behavior outside core. A future
plugin may be proposed, but core Striatum must not grow implicit
GitHub/GitLab/etc. behavior, provider SDK imports, credential loading, push
or fetch operations, telemetry, or external persistence.

## Plugin Decision Prerequisites

Reopening hosted provider actions requires a later accepted RFC or decision
that defines at least:

- the exact hosted operations in scope, such as branch push, PR create, PR
  update, PR comment, reviewer assignment, or status checks;
- confirmation semantics, with human-principal confirmation required for
  hosted-provider actions unless a later decision explicitly narrows that
  rule;
- credential configuration and custody outside durable runner state and
  outside committed artifacts;
- audit semantics that preserve metadata-only Striatum audit rows and do not
  make hosted provider state authoritative workflow state;
- plugin packaging boundaries that keep provider SDKs and provider-specific
  dependencies out of Striatum core;
- failure behavior when the plugin, credentials, network, or provider account
  is missing or unavailable.

## Closure

Deferred item 21 is closed for Striatum core. There is no current D127 source
violation to repair.

The correct shared-doc follow-up is status hygiene only: `docs/rfcs/0067-optional-git-pr-integration.md`
still reads as pre-D127 blocked text, while D127/TODO/ROADMAP/SPEC record the
landed local core slice and the hosted-provider out-of-core boundary. That
RFC update is outside this packet's write scope and should be queued only by
an operator opening shared RFC docs.
