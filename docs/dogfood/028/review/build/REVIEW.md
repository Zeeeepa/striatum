---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Build review: RFC 0025 V1 Step 1 (devils_advocate)

author: reviewer-claude-opus-002

## Posture

Devil's advocate. Argue against the build's claims about
correctness, idempotency, design-review compliance.

## Counter-claims

### C1: "All 14 files written"

Test `test_claude_code_install_writes_full_bundle` enumerates the
expected paths and asserts the manifest's `files[]` matches.
**Survives.**

### C2: "Idempotent re-install (F4 addressed)"

`test_claude_code_idempotent_re_install` snapshots bundle file
bytes before re-install, calls install again, and asserts every
file is byte-identical. The third re-install reports
`skipped_unchanged` for every file. The manifest itself changes
(timestamp), which the test correctly ignores. **Survives.**

### C3: "Edit-detect + force semantics"

Test pair confirms: edit-detect refuses without `--force`;
`--force` overwrites. **Survives.**

### C4: "URL-leak invariant (F3)"

The test forbids any scheme. I read the rendered output of all 14
files manually — no schemes, no `/home/`, no source-repo paths.
The skill bodies use only relative references. **Survives.**

### C5: "Skill template byte-match (F1)"

`test_skill_templates_match_skills_module` reads each skill template
from both `skills/templates/claude_code/` and
`plugins/templates/claude_code/skills/` and asserts byte-equality.
This catches any future drift. **Survives.**

### C6: "Marketplace fixture is reentrant"

Tests confirm: first install creates the fixture with one entry;
second install updates in place (no duplicate entry). The merge
logic matches by `(name, source.path)`. **Survives.**

### C7: "Uninstall removes everything"

`test_uninstall_removes_tracked_files` confirms; bundle root
recursively removed. The bottom-up rmdir loop handles nested dirs
(skills/striatum-foo/SKILL.md → striatum-foo/ → skills/). **Survives.**

### C8: "Uninstall refuses modified files"

Test confirms; `--force` removes them. **Survives.**

### C9: "Doctor walks bundles"

I read the introspect.py addition: it iterates `PLUGIN_PROFILES`,
loads each `<bundle>/.manifest.json`, walks `files[]` for missing
files, and computes template drift via the same SHA pattern as
skills. **Survives.**

### C10: "init --with-plugins integrates cleanly"

Smoke test: `striatum init --with-plugins` produces the bundle
directly from the init flow. **Survives.**

### C11: "Helper expansion reuse"

`plugins.install._render` imports `skills.install._expand_helpers`
to render skill bodies identically. This avoids duplication; if
the helper expansion changes, both surfaces benefit. **Survives.**

### C12: "Manifest schema_version distinct"

`MANIFEST_SCHEMA_VERSION = "striatum.plugins.manifest.v1"` (not
`striatum.skills.manifest.v1`). A future migration that walks
manifests can distinguish the two. **Survives.**

### C13: "F2 simplified — doctor walks bundles directly"

The implementer skipped the `manifest_index.json` proposed in the
synthesis and instead walks `PLUGIN_PROFILES` directly. This is
strictly simpler and matches my F2 recommendation that the index
not blind doctor on deletion. **Survives.**

## Findings

### F1 (note, non-blocking): private-import coupling

`introspect.py` imports `_bundled_template_sha256` and
`_load_manifest` from `plugins.install` (private functions). When
Steps 2-3 land, refactor those into public helpers (rename to
drop the underscore) so doctor stays a public consumer. Not blocking.

### F2 (note, non-blocking): user-scope path for codex/gemini

The current `_bundle_root` only handles `claude_code` user scope.
Steps 2-3 need to add codex (`~/.codex/plugins/`) and gemini
(`~/.gemini/extensions/`). Trivial extension — note for the next
implementer.

## Verdict

**accept**

The build survives every counterargument. All four design-review
findings (F1-F4) are addressed; F2 even improved on the synthesis
by skipping the redundant `manifest_index.json`. The
`claude_code` profile is end-to-end shippable; Steps 2-3 plug
into the existing infrastructure.
