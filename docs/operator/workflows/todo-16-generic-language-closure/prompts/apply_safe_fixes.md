# Apply Safe Generic-Language Fixes

Write `docs/operator/artifacts/todo-16-generic-language-closure/apply/HANDOFF.md`
with `author: generic-language-codex-gpt-5-002`.

Task:

- Read the scan artifact.
- Apply only narrowly scoped fixes for current generic-language drift.
- Add or adjust guardrails only when the scan identifies a concrete
  regression shape worth pinning.
- Keep shared operator status docs read-only:
  `docs/TODO.md`, `docs/ROADMAP.md`, and `docs/operator/BRIEF.md`.

Expected current candidate:

- `docs/rfcs/0056-consumer-repo-directory-structure-opinions.md` still uses
  `Engram-style dogfood corpus` in a generic layout recommendation. Replace
  that with generic structured-run wording.
- Broaden the existing stale-current-doc exact-phrase guardrail in
  `tests/test_doc_links.py` so accepted RFC/current Markdown docs are covered,
  not only the three files from the previous sweep.

Definition of done:

- The handoff lists each changed file and the reason.
- The handoff lists validation commands that should pass.
- The handoff reports any shared-doc updates needed instead of editing them.
