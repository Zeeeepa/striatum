# Run On Proximal

This workflow is meant to run on `proximal` after the current in-flight
Striatum work has settled. It is intentionally a reliability reset evaluation,
not another feature campaign.

## Preflight

On `proximal`:

```bash
cd ~/git/striatum
git fetch origin
git status -sb
git pull --ff-only
make install
striatum daemon status
striatum operator bootstrap --markdown
striatum workflow validate docs/operator/workflows/striatum-reliability-reset-2026-06-16/workflow.json --json
```

The workflow's lane command is intentionally `striatum codex`, which resolves
the live daemon MCP endpoint and runtime token before launching Codex. If
`codex` is not available on `proximal`, stop and fix the lane command or
provider install before preparing the run.

Do not start the run until there are no non-terminal runs. Striatum's current
operator bootstrap treats these run states as terminal: `completed`, `failed`,
and `canceled`.

One quick check:

```bash
striatum status --json \
  | jq -r '.data.runs[]? | select((.state == "completed" or .state == "failed" or .state == "canceled") | not) | "\(.run_id)\t\(.state)"'
```

If that prints any rows, wait or settle those runs first.

## Start

```bash
run_json="$(striatum run prepare --workflow docs/operator/workflows/striatum-reliability-reset-2026-06-16/workflow.json --json)"
echo "$run_json" | jq .
run_id="$(echo "$run_json" | jq -r '.data.run_id // .run_id')"
striatum run start --run-id "$run_id" --json
striatum run drive --run-id "$run_id"
```

## Watch

```bash
striatum dashboard --run-id "$run_id"
striatum status --run-id "$run_id" --json
```

The final artifacts should land under:

```text
docs/operator/artifacts/striatum-reliability-reset-2026-06-16/
```

Do not treat this workflow as successful just because it reaches a terminal
state. The useful outcome is an accepted reset synthesis with a support ledger,
an evidence audit, and a final review that names the first tickets and the
feature-freeze release gate.
