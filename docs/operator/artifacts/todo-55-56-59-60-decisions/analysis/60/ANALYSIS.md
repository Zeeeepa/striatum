---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["todo_60", "rfc_0067", "spec_product_boundary", "decision_log"]
---

# TODO 60 Optional Git/PR Boundary Analysis
author: analyst-codex-001

## Scope

TODO 60 asks whether Striatum should grow optional Git and PR integration,
including commit authority, local Git behavior, hosted-provider boundaries,
credentials, and provenance requirements. RFC 0067 deliberately blocks
implementation beyond read-only local Git snapshotting until those product
decisions are explicit.

This artifact prepares options. It does not choose the policy.

## Fixed Constraints

The current product boundary is strict:

- Striatum is local-first and daemon/PostgreSQL-backed for live workflow state.
- Repository artifacts are durable provenance, not the live message bus.
- Hosted services, cloud APIs, external persistence, telemetry, transcript
  capture, and provider SDK integration are out of scope without an accepted
  product decision.
- The runner does not create commits automatically by current decision history:
  D017 says the coordinator starts/selects a branch and requests a commit from
  the human.
- RFC 0067 allows only read-only local Git snapshotting without a new decision:
  branch, HEAD SHA, dirty-tree summary, changed paths, and local ancestry
  metadata.

Any accepted TODO 60 policy should preserve those constraints or explicitly
record which constraint it changes.

## Decision Axis 1: Should Striatum Create Commits?

### Option A: Never create commits; produce commit-ready handoff only

Striatum may report local Git state, produce reviewed patch summaries, and
publish a commit request artifact, but the operator or human principal runs
`git commit` outside Striatum.

This is the most conservative fit with D017 and the local-first boundary. It
keeps Git side effects out of the daemon mutation surface and avoids defining
commit signing, author identity, hooks, branch policy, or rollback semantics.
The tradeoff is weaker end-to-end automation: Striatum can say "this is ready
to commit" but cannot prove the resulting commit matches the reviewed artifact
unless later archive/replay evidence captures the post-commit snapshot.

### Option B: Commit only after explicit operator confirmation

Striatum may create a local commit only from a durable commit-request artifact
that names the reviewed artifacts, allowed paths, base HEAD, resulting tree or
patch digest, commit message, author policy, and confirmation evidence. The
daemon performs the mutation after an explicit operator confirmation gesture.

This creates a useful local automation slice while preserving "no autonomous
commits by default." It also aligns with the existing pattern for gated
mutations: durable artifact first, daemon authority second, audit event always.
The cost is real product surface area. The decision must define whether the
confirmation can come from the AI operator or must come from the human
principal, how to handle hooks, how to refuse dirty trees outside write scope,
and whether commits are signed or left to local Git config.

### Option C: Allow workflow-declared autonomous local commits

Workflows could opt in to automatic local commit creation after review gates
pass, possibly limited to worktree-isolated jobs and exact write-scope paths.

This maximizes automation but is the largest boundary change. It weakens the
current D017 posture, creates a new class of side effects outside Striatum's
PostgreSQL live state, and would require strong policy around commit identity,
hooks, rollback, provenance, failed commits, and mismatch handling. If chosen,
it should likely be a later phase after Option B has proven safe.

## Decision Axis 2: Local Git Versus Hosted Providers

### Option A: Local Git only

Striatum supports local repository inspection and, if separately accepted,
local commit creation. It never pushes, opens PRs, comments on PRs, or calls
GitHub/GitLab/Bitbucket APIs.

This keeps the product fully local-first and avoids credential storage. It is
also the clearest answer for package users who do not want network behavior in
the runner. The downside is that PR creation remains a manual handoff.

### Option B: Local Git core plus optional provider plugin/connector

Core Striatum remains local-only. Hosted-provider operations live behind an
optional plugin or connector with explicit install/configuration, explicit
credential source outside durable runner state, and no default network access.
The core emits a PR-request artifact; the plugin may turn that into a hosted PR
only when explicitly invoked.

This respects the product boundary while leaving room for practical hosted
workflows. It avoids baking provider SDKs and credentials into core scheduling.
The cost is a sharper plugin contract: the decision must say whether plugin
outputs become durable Striatum artifacts, whether provider IDs can be recorded
in artifacts, and how redaction/archive rules treat URLs, PR numbers, and
remote branch names.

### Option C: First-class hosted-provider integration in core

Striatum core would know about hosted providers and directly create or update
PRs.

This conflicts with the current boundary unless a broad product decision
changes Striatum from local-first orchestration into a network-integrated
development agent runner. It should be rejected or deferred unless the owner
wants that product expansion explicitly.

## Decision Axis 3: Confirmation Authority

Possible confirmation models:

- AI operator confirmation: the same agent/operator that drives routine
  Striatum work may approve a local commit/PR operation when workflow gates are
  satisfied.
- Human-principal confirmation: commit or hosted PR creation is treated like an
  escalation-class side effect and requires human approval.
- Workflow-declared authority: workflow config chooses whether local commits
  require AI operator or human-principal confirmation, while hosted-provider
  actions always require human-principal confirmation.

The conservative policy is human confirmation for hosted-provider actions and
at least explicit operator confirmation for local commits. Autonomous commit or
PR creation should not be a default.

## Decision Axis 4: Credential Boundary

If hosted providers are ever allowed, credentials should remain outside durable
runner state:

- No provider token in workflow JSON, artifacts, archive bundles, corpus
  exports, audit payloads, or `.striatum/` scratch.
- Credential lookup should be adapter/plugin-owned through environment,
  OS credential stores, or provider CLI auth already configured by the
  operator.
- Core Striatum should record only metadata-safe outcomes, such as provider
  kind, operation class, local request artifact digest, and optionally a
  redaction-safe external URL if the product decision permits it.

The decision should explicitly answer whether external identifiers such as PR
URLs, remote branch names, repository slugs, and commit SHAs are acceptable
durable provenance. Local commit SHAs are probably necessary if commits are in
scope; hosted URLs are a stronger privacy/network disclosure.

## Provenance And Review Requirements

For any write-capable Git integration, require a durable request artifact
before mutation. A local commit request should include:

- base HEAD and branch at request time,
- dirty-tree summary and changed paths,
- reviewed artifact paths and verdicts that authorize the commit,
- write-scope evidence for every changed path,
- proposed commit message and author policy,
- patch digest or resulting tree digest when available,
- confirmation actor class and timestamp,
- mutation result: created commit SHA or refusal reason.

For hosted PR integration, require a separate PR request artifact with:

- local commit SHA or branch state to publish,
- target provider and target remote/branch policy,
- PR title/body source,
- credential source class, not credential value,
- explicit network-operation confirmation,
- result metadata allowed by the privacy decision.

Review should verify that the Git operation is downstream of accepted workflow
artifacts, not a substitute for review. A "create commit" or "create PR"
operation must refuse when review gates are missing, when changed paths exceed
write scope, when base HEAD has drifted without revalidation, or when dirty
state includes untracked changes outside the request.

## What Should Remain Out Of Scope

Unless explicitly accepted in a later RFC, keep these out of scope:

- autonomous commits by default,
- push or PR creation from core Striatum,
- provider SDK imports in core scheduling or daemon code,
- credential persistence in Striatum live state, artifacts, archives, or
  corpus exports,
- remote CI orchestration, branch protection bypass, merge operations, or
  force-pushes,
- provider webhooks or background polling,
- transcript capture to justify commit/PR decisions,
- malicious-local-operator-resistant Git sealing guarantees.

## Safe First Slice

RFC 0067's read-only local Git snapshotting remains safe to implement without
settling the larger policy:

- expose branch, HEAD SHA, and dirty-tree summary through daemon-backed read
  surfaces,
- record changed-path lists and local ancestry metadata in status/archive
  evidence,
- keep the methods read-only and local-only,
- avoid remote/provider identifiers,
- add tests proving no commits, pushes, hosted calls, or credential reads occur.

This slice helps downstream decisions because it provides concrete provenance
inputs for later commit-request artifacts without crossing the write/network
boundary.

## Decision Package To Ask The Human Principal

The checkpoint can be framed as five independent choices:

1. Commit authority: handoff only, explicit-confirm local commits, or
   workflow-opt-in autonomous local commits.
2. Hosted boundary: never in Striatum, optional plugin/connector only, or
   first-class core integration.
3. Confirmation model: AI operator, human principal, or workflow-declared for
   local commits with human-only hosted actions.
4. Credential policy: external-only credentials with no durable persistence,
   plus a decision on whether redaction-safe provider URLs/IDs may be recorded.
5. Implementation sequencing: read-only local snapshots first, then commit
   request artifacts, then optional local commit apply, then optional provider
   plugin only if accepted.

The least disruptive path is read-only local snapshots plus commit/PR request
artifacts, with local commit apply requiring explicit confirmation and hosted
provider actions left to optional plugins/connectors or manual handoff. That is
an analysis recommendation for coherence, not a recorded product decision.
