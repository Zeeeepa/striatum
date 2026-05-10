# Claude Code Design Prompt

Produce `docs/dogfood/030/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0026 and RFC 0027 together. Focus on the failure modes that created the RFCs: operator-surrogate false bylines, local operator bypasses, direct source edits outside workflow control, and evidence that overstates provenance.

Your plan must cover schema/API changes, migrations, compatibility risk, test plan, and staged delivery. Include concrete touch points in `src/striatum/`, CLI commands, artifact publishing, verdict recording, evidence export, run summary, doctor/status, docs, and fixtures.

Be explicit about what cannot be proved: model-token authorship, independent human decision provenance, or adversarial local-root resistance. If the work packet supplies an `author:` line, copy it exactly into the artifact title block.
