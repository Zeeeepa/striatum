# Build review (3-way)

Produce `docs/dogfood/063/review/build/<lane>/REVIEW.md` per posture.

**codex `threat_model`:** dual-name covers every reference;
PG state rename is code-only (no SQL); workflow upgrade idempotent;
no flag NAMES changed.

**claude `ergonomics_dx`:** deprecation warning is operator-readable
and points at `workflow upgrade`; CLI text clearer; existing
operator scripts continue to work; UBIQUITOUS_LANGUAGE.md updated.

**gemini adversarial:** workflow with BOTH names for same field
(which wins?); upgrade that loses formatting; new-write/legacy-read
producing stale states; operator script greps for old names breaks
silently.

**Write scope:** `docs/dogfood/063/review/build/<lane>/REVIEW.md`.
