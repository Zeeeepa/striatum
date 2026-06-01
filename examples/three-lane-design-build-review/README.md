# Three-lane design + build + review

A runner-owned fixture for the historical "design / build / review"
three-lane flow that the legacy P001 bootstrap script ran via tmux
panes. Striatum's process adapter and supervised lanes have superseded
the bootstrap; this fixture reproduces the same shape end-to-end as a
plain `workflow.json` an operator can hand to `striatum run prepare`.

## Shape

```
design_codex     \
design_claude   --> synth --> review_design --> implement --> review_build_codex
design_gemini    /                                       \--> review_build_claude
                                                          \-> review_build_gemini
```

- Three parallel design lanes (`codex`, `claude_code`, `gemini`) each
  produce an independent `DESIGN.md`.
- A single synthesis job reconciles the three designs into one
  buildable plan.
- A design review (`ergonomics_dx`) gates the build.
- One implementer lands the smallest-scope item.
- Three parallel build reviewers run distinct postures
  (`threat_model`, `ergonomics_dx`, `devils_advocate`).
- Bounded `needs_revision` cycles let each review push back on the
  upstream job up to two iterations. `allow_same_lane: true` is set so
  the same lane can pick up the revision.

The fixture is generic. Replace `{{TASK}}` in the prompts with the
concrete task before `striatum run start`.

## Files

```
examples/three-lane-design-build-review/
  workflow.json
  README.md
  prompts/
    design.md
    synth.md
    review_design.md
    implement.md
    review_build.md
  roles/
    coordinator.md
    designer.md
    implementer.md
    reviewer.md
```

## Running

The lane commands are bare interactive agent CLIs (`codex`, `claude`,
`agy`) declared with `adapter_capabilities.agent_loop: true` and
`supervision.transport: pty_helper` so the daemon wraps each lane in the
RFC 0088 agent-loop executor — the agent connects as an MCP client and
claims work over the PTY. Do **not** add one-shot flags (`--print`,
`-p`, `codex exec`): those print once and exit without ever claiming a
work packet. Substitute the exact agent CLIs/models your target
repository uses, keeping the agent-loop lane shape intact. The fixture
validates as-is.

```bash
# Validate
striatum workflow validate examples/three-lane-design-build-review/workflow.json

# Prepare a run inside the target repository
striatum run prepare \
  --workflow path/to/three-lane-design-build-review/workflow.json \
  --branch striatum/your-task

# Start the run
striatum run start --run-id <id>

# Watch
striatum dashboard --run-id <id>
```

Each lane writes only inside its `write_scope.allowed_paths`. The
default scopes write under `docs/three-lane-design-build-review/`;
adjust them for your target repository before the first run.

## Artifacts

Successful runs produce the following durable artifacts under
`docs/three-lane-design-build-review/`:

- `design/codex/DESIGN.md`
- `design/claude_code/DESIGN.md`
- `design/gemini/DESIGN.md`
- `DESIGN_SYNTHESIS.md`
- `review/design/REVIEW.md`
- `build/HANDOFF.md`
- `review/build/codex/REVIEW.md`
- `review/build/claude_code/REVIEW.md`
- `review/build/gemini/REVIEW.md`

Plus whatever the implementer lands under `src/` and `tests/`.

## Historical lineage

The P001 prompt (retained under `prompts/`) ran a three-lane design /
build / review pipeline through three tmux panes that an operator had
to manage by hand. That script predated the process adapter, the
supervised-lane wrappers, the lease and recovery verbs, and the
durable-artifact contract.

This fixture is the runner-owned successor: the same three-lane
shape, but expressed as a validated `workflow.json` the
process adapter executes end-to-end. Closes the residual gap on
`docs/TODO.md` item 13 ("replace bootstrap scripts with runner-owned
workflows").

For a richer multi-lane example with ledgers and supervised
wrappers, see `examples/rfc-ledger-cleanup/` and the dogfood runs
under `docs/dogfood/`.
