# Synth — RFC 0043 V1.6

Reconcile three design proposals at
`docs/dogfood/052/design/{codex,claude_code,gemini}/DESIGN.md`. Produce
synthesis at `docs/dogfood/052/DESIGN_SYNTHESIS.md`:

- Pick one approach per F (F-escape, F-split-brain, F-lock, F-help).
- List exact files + edit order.
- Name acceptance verifiers (which pytest will catch each regression).
- Re-affirm that daemon-side substrate migration (gemini A1) is
  V2.0 scope, not here.

400-800 words. Sections: Decisions, Implementation order, Acceptance.
