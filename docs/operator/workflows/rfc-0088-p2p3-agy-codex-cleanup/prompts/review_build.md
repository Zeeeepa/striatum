# Build review — RFC 0088 P2+P3 (document_only, no interrogation)

Read TASK.md + RFC 0088 + .../artifacts/build/HANDOFF.md + the diff. Posture
from the packet. The implementer is codex one-shot in this dogfood and is
NOT interrogable — do not call interrogation tools. Read the artifacts and
the diff, run the verification commands (`cd go && go vet ./... && go test
./...`), confirm both commit boundaries (P2 commit, P3 commit) and that the
codex agent_loop verify-run evidence is recorded in HANDOFF.md. Write
.../artifacts/review/build/<lane>/REVIEW.md with finding front matter +
byline + a clear statement of what you verified and what you did NOT.
Finalize with ONE review.submit. threat_model: any dangling reference to
turn_driver/single_shot/--print-wrapper; no provenance regression (lane
byline still attested); ephemeral mcp-config for agy is 0600 and removed
after exit. ergonomics_dx: clean error when a removed lane is referenced in
a workflow.json (good failure mode); installer reuse path actually works
(claude bundle correctly seeds agy).
