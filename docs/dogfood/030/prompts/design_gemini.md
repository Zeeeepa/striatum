# Gemini Design Prompt

Produce `docs/dogfood/030/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0026 and RFC 0027 together. Emphasize cross-platform containment reality, patch identity, digest-bound review semantics, apply preconditions, receipts, migration strategy, compatibility risk, tests, and rollout staging.

Your plan must state which parts are implementable in the current local CLI architecture and which require a later platform-specific authority boundary. Treat macOS, Linux, and Windows containment differences as first-class risks, not footnotes.

Include a test plan with adversarial cases for false provenance claims, path bypasses, digest substitution, base-tree drift, unattested sessions, and review verdicts over the wrong patch. If the work packet supplies an `author:` line, copy it exactly into the artifact title block.
