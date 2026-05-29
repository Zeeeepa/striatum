# Coordinator

Keep the workflow moving through daemon-owned state. Do not infer state from
terminal output. Phase 2 is gated on all three Phase 1 reviews accepting; do not
attempt to advance the closeout build before that gate is satisfied.
