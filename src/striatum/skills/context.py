"""Template context for RFC 0015 skill rendering.

V1 uses a curated whitelist of CLI verbs (research artifact §
"Smallest set of CLI verbs to cover"). Promoting to a parser-walked
table is a V1.5 follow-up; the byte-shape of the bundle does not
change until that lands.
"""

from __future__ import annotations

from typing import Any

from striatum import __version__ as STRIATUM_VERSION
from striatum.artifacts import ALLOWED_ARTIFACT_KINDS

# Curated verb table. Order is presentation order in the rendered
# Markdown; do not sort alphabetically.
VERB_TABLE: dict[str, dict[str, str]] = {
    "scaffold": {
        "init": "Create `.striatum/state.sqlite3` in the target repo.",
        "workflow init": "Scaffold a starter `workflow.json` + roles/prompts.",
        "workflow validate": "Validate a workflow file; non-zero exit on error.",
        "run prepare": "Snapshot the workflow and prepare a run id.",
        "run start": "Transition a prepared run to `running`.",
        "branch confirm": "Confirm or create the run's working branch.",
    },
    "claim_loop": {
        "register-session": "Open an agent session for a (role, lane) pair.",
        "claim-next": "Claim the next ready work packet for a session.",
        "ack": "Acknowledge a claimed packet and start work.",
        "heartbeat": "Extend the active lease while work continues.",
        "publish-artifact": "Publish a build artifact for a job.",
        "verdict": "Record a non-finding verdict for a review job.",
        "submit-review": "Publish a finding artifact + verdict in one call.",
        "complete": "Mark a non-review job complete.",
        "worktree create": "Create the per-job git worktree (RFC 0008).",
        "worktree release": "Release a per-job worktree when work is done.",
    },
    "supervise": {
        "supervise start": "Start a long-lived agent CLI for the session.",
        "supervise send": "Deliver a packet to a supervisor's stdin pipe.",
        "supervise stop": "Stop a supervisor (SIGTERM, SIGKILL after 5s).",
        "supervise status": "Probe a supervisor's liveness.",
        "supervise list": "List supervisors for a run.",
    },
    "recover": {
        "status": "Read run/job/blocker state.",
        "why": "Explain why a job is in its current state.",
        "doctor --verbose": "Surface structured consistency problems.",
        "recovery stale-leases": "Lazy-expire and report stale leases.",
        "recovery requeue-stale": "Requeue an expired non-repo-write job.",
        "recovery process-reconcile": "Reconcile externally-killed processes.",
        "recovery resume": "Resolve remediated process-adapter blockers.",
        "checkpoint resolve": "Resolve a `human_checkpoint` blocker.",
        "dashboard --once": "Render a single dashboard frame for scripts.",
    },
}

# Boundary statements reused across skills. Each is a single sentence
# the renderer may inline into a template's "What not to do" block.
BOUNDARIES: tuple[str, ...] = (
    "Do not write to `.striatum/state.sqlite3` directly; the runner is the only writer (D006/D009).",
    "Do not treat marker files, tmux panes, or terminal output as workflow state.",
    "Do not advance state by printing phrases; advance by calling the CLI verbs in your work packet.",
    "Do not capture stdout/stderr transcripts; D028 keeps them off by default.",
    "Do not derive bylines from job titles; use the byline supplied in your work packet.",
    "Do not parse a supervisor's own output for workflow state; supervisors send DEVNULL.",
)


def gather_template_context() -> dict[str, Any]:
    """Return the template context.

    Pure: deterministic for a given installed runner version.
    """
    return {
        "striatum_version": STRIATUM_VERSION,
        "verbs": VERB_TABLE,
        "boundaries": BOUNDARIES,
        "front_matter_kinds": sorted(ALLOWED_ARTIFACT_KINDS),
    }
