---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---

# Design review: RFC 0025 V1 Step 1 (devils_advocate)

author: reviewer-claude-opus-001

## Posture

Devil's advocate. Argue against scope, architecture, manifest
choices, idempotency.

## Counter-claims

### C1: "Mirror skills/install.py is the right abstraction"

Concern: copy-pasting a 200-line install pipeline into
`plugins/install.py` creates two parallel maintenance points.
**Counter:** the surface differs in important ways (manifest path,
namespace defaults, marketplace fixture, profile semantics). Trying
to abstract both behind one entry creates a "config object" pattern
that's worse than two clear modules. The duplication is honest.
**Survives.**

### C2: "Five skill bodies duplicated as templates"

Concern: same content lives in `skills/templates/claude_code/` and
`plugins/templates/claude_code/skills/`. Drift risk. **Finding (F1,
non-blocking):** Add a CI test that asserts the bytes match — when
a skill template is updated, both copies must move together. Or:
import the skill templates from the plugins module so there's one
source of truth at the package-resources level.

### C3: "Manifest at <bundle>/.manifest.json (not <repo>/.striatum)"

Concern: this differs from RFC 0015 where the manifest lives at the
target's `.striatum/skills.manifest.json`. Two manifest patterns to
reason about. **Counter:** The synthesis is correct that bundles are
self-describing — Claude Code's plugin format expects a directory
to be the unit of install. The RFC explicitly says
`<bundle>/.manifest.json`. **Survives.**

### C4: "manifest_index.json centralizes doctor lookups"

Concern: a parallel index file at `.striatum/plugins/manifest_index.json`
is yet another source of truth that can drift from the bundle
manifests. **Finding (F2, non-blocking):** Make the index file
**optional** for doctor lookups. Doctor should walk
`.striatum/plugins/*/` for `<dir>/.manifest.json` files as the
primary path, and fall back to / cross-check the index. That way
deletion of the index doesn't blind doctor.

### C5: "Marketplace fixture default-on"

Concern: `--with-marketplace` defaults to True. Operators who don't
use the Claude Code marketplace flow get an unused JSON file. Not
harmful but not free. **Counter:** RFC § 4 explicitly mandates this.
RFC § Open Questions notes the marketplace name collision concern
already. Acceptable. **Survives.**

### C6: "URL-leak test catches all leaks"

The test walks for `https?://`, `git://`, `file://`, source-repo
path, home dir. Concern: false positives — code blocks in skill
bodies that legitimately reference a URL (e.g. linking to RFC
documents). **Finding (F3, non-blocking):** The synthesis is
silent on what's whitelisted. The implementer must decide: is
referencing `docs/rfcs/0025-...` (no scheme) allowed? What about
`https://docs.python.org` in a code block? Recommend: forbid all
schemes including in code blocks; reference RFC files by *relative
path inside the bundle* if needed.

### C7: "init --with-plugins ordering"

Concern: if the operator runs `striatum init --with-plugins
--with-skills`, what runs first? **Counter:** order doesn't matter
because they write to different paths. **Survives.**

### C8: "Idempotency claims hold"

Concern: `generated_at` timestamp in the manifest changes every
install. Re-installing produces a manifest with a new timestamp →
manifest file SHA differs. The acceptance criteria say "byte-
identical" for the *bundle*, but the manifest itself changes. **Finding
(F4, non-blocking):** Either (a) test asserts only bundle files
are byte-identical (manifest excluded), or (b) manifest's
generated_at is *first-install-stamp* and skipped on idempotent
re-installs. Recommend (a) — operators get a fresh stamp per
install attempt.

### C9: "Five slash commands is the right surface"

Concern: V1 ships five commands. The RFC's Open Questions section
explicitly raises this as something to revisit. **Counter:** the
synthesis honors the RFC. **Survives.**

### C10: "Step 1 ships shared infrastructure plus claude_code"

Concern: shipping CLI + doctor + init together with claude_code
profile is a lot for one dogfood. **Counter:** the CLI surface is
nearly identical to skills, doctor checks are small, init flag is
trivial. The profile templates are the bulk of the work and that's
all in one place. **Survives.**

## Findings

### F1 (recommend, non-blocking): Test that skill bodies match

Add `test_plugin_skill_templates_match_skills_templates` that
asserts `plugins/templates/claude_code/skills/<name>.md.tmpl` is
byte-identical to `skills/templates/claude_code/<name>.md.tmpl`.
Catches drift on any future skill update.

### F2 (recommend, non-blocking): Doctor walks bundles, index is fallback

Doctor should `glob('.striatum/plugins/*/.manifest.json')` as
primary, cross-check against `manifest_index.json` as secondary.
Index deletion doesn't blind doctor.

### F3 (note, non-blocking): URL-leak whitelist policy

The implementer should pick a stance on URLs in code blocks /
documentation references and document it in the test or in
PLUGIN_SHAPE.md. Recommend forbidding all schemes.

### F4 (note, non-blocking): Idempotency test scope

The test should compare bundle files (excluding `.manifest.json`'s
`generated_at` field) for byte-identity, not the manifest itself.
Document this in the test or synthesis.

## Verdict

**accept_with_findings**

Four findings, all non-blocking. Scope is appropriate for V1 Step 1.
The architecture is the right one. F1 (drift) and F4 (manifest
timestamp) are the most important to address.
