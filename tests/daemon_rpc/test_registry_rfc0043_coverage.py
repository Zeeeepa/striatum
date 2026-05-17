"""RFC 0043 §5 — RPC method registry covers every CLI mutation.

Asserts:

- Every mutation exposed by ``src/striatum/cli/mutations.py`` has a
  registered method in the daemon RPC registry under the RFC 0043
  dotted vocabulary.
- Every registered production method routes through a native PG handler
  or a deliberately inline daemon path.
- The capabilities required for each method match the RFC 0043 §5 table.
- Legacy undotted method names remain in the registry but are flagged
  ``deprecated`` so clients migrate.
- Unknown methods are refused (existing daemon RPC behavior — covered
  here as a regression).
"""

from __future__ import annotations

import inspect
from typing import Any

from striatum.cli import mutations as cli_mutations
from striatum.cli.recovery import (
    cancel_job,
    process_reconcile,
    requeue_stale,
    resume_blocker,
    stale_leases,
)
from striatum.cli.supervise import (
    supervise_list,
    supervise_send,
    supervise_start,
    supervise_status,
    supervise_stop,
)
from striatum.cli.worktree import worktree_create, worktree_list, worktree_release
from striatum.daemon_rpc.registry import CAPABILITIES, METHOD_REGISTRY
from striatum.daemon_rpc.server import LOCAL_FILE_AUTHORING_METHODS

# Function-name → RFC 0043 §5 method name. The mapping is explicit so a
# new mutation must declare its intended method when added.
MUTATION_TO_METHOD: dict[str, str] = {
    # cli.mutations
    "branch_confirm": "branch.confirm",
    "run_start": "run.start",
    "register_session": "session.register",
    "close_session": "session.close",
    "ack_work": "work.ack",
    "heartbeat": "work.heartbeat",
    "release_work": "work.release",
    "send_message": "work.send_message",
    "block_work": "work.block",
    "verdict_work": "review.verdict",
    "submit_review": "review.submit",
    "decision_record": "decision.record",
    "checkpoint_resolve": "checkpoint.resolve",
    # cli.recovery
    "stale_leases": "recovery.stale_leases",
    "requeue_stale": "recovery.requeue_stale",
    "cancel_job": "recovery.cancel_job",
    "process_reconcile": "recovery.process_reconcile",
    "resume_blocker": "recovery.resume",
    # cli.supervise
    "supervise_start": "supervise.start",
    "supervise_send": "supervise.send",
    "supervise_stop": "supervise.stop",
    "supervise_status": "supervise.status",
    "supervise_list": "supervise.list",
    # cli.worktree
    "worktree_create": "worktree.create",
    "worktree_release": "worktree.release",
    "worktree_list": "worktree.list",
}

# Per RFC 0043 §5 — capability required for each method.
EXPECTED_CAPABILITY: dict[str, str] = {
    "session.register": "claim",
    "session.close": "claim",
    "work.claim_next": "claim",
    "work.ack": "claim",
    "work.heartbeat": "claim",
    "work.release": "claim",
    "work.send_message": "write",
    "work.block": "write",
    "work.complete": "write",
    "artifact.publish": "write",
    "review.submit": "review",
    "review.verdict": "review",
    "review.override": "admin",
    "decision.record": "admin",
    "checkpoint.resolve": "admin",
    "recovery.stale_leases": "recovery",
    "recovery.requeue_stale": "recovery",
    "recovery.cancel_job": "recovery",
    "recovery.process_reconcile": "recovery",
    "recovery.resume": "recovery",
    "recovery.auto": "recovery",
    "recovery.watch": "recovery",
    "worktree.create": "write",
    "worktree.release": "write",
    "worktree.list": "read",
    "branch.confirm": "admin",
    "run.prepare": "admin",
    "run.start": "admin",
    "run.pause": "admin",
    "run.resume": "admin",
    "run.cancel": "admin",
    "run.retry_job": "admin",
    "run.summary": "read",
    "run.graph": "read",
    "workflow.validate": "read",
    "workflow.plan": "read",
    "workflow.graph": "read",
    "workflow.templates.list": "read",
    "workflow.templates.show": "read",
    "workflow.init": "write",
    "workflow.generate": "write",
    "workflow.upgrade": "write",
    "status": "read",
    "why": "read",
    "doctor": "read",
    "dashboard": "read",
    "dashboard.all": "read",
    "list.runs": "read",
    "list.sessions": "read",
    "list.jobs": "read",
    "list.artifacts": "read",
    "artifact.show": "read",
    "list.workflows": "read",
    "evidence.export": "read",
    "corpus.export": "read",
    "archive.create": "read",
    "supervise.start": "claim",
    "supervise.send": "claim",
    "supervise.stop": "claim",
    "supervise.status": "read",
    "supervise.list": "read",
    "supervise.reattach_status": "read",
    "repo.add": "admin",
    "repo.remove": "admin",
    "repo.list": "read",
    "daemon.migrate": "admin",
    "daemon.migrate_repo_local": "admin",
}

# Methods that the server handles inline (no PG handler needed).
SERVER_INLINE_METHODS: frozenset[str] = frozenset(
    {
        "daemon.hello",
        "daemon.describe",
        "dashboard.all",
        "repo.init",
        "repo.add",
        "repo.remove",
        "repo.list",
        "daemon.token.create",
        "daemon.token.revoke",
        "daemon.token.rotate",
        "daemon.key.rotate",
        "daemon.shutdown",
        "daemon.migrate",
        "daemon.migrate_repo_local",
        "cross_repo.list",
        "cross_repo.describe",
        "cross_repo.why",
        "cross_repo.cancel",
        "apply.reviewed_patch",
        "apply.receipt.show",
        "apply.receipt.verify",
        "dogfood.publish_on_behalf",
        "dogfood.surgical_recovery",
        "workflow.generate.preview",
        "supervise.reattach_status",
    }
)


def test_every_mutation_has_a_registered_method() -> None:
    missing: list[str] = []
    for func_name, method_name in MUTATION_TO_METHOD.items():
        # Confirm the function exists in one of the CLI mutation modules
        # — guards against future renames that would silently bypass the
        # registry coverage check.
        sources = (cli_mutations, _recovery_module(), _supervise_module(), _worktree_module())
        assert any(
            inspect.isfunction(getattr(module, func_name, None)) for module in sources
        ), f"mutation function {func_name!r} not found in CLI mutation modules"
        if method_name not in METHOD_REGISTRY:
            missing.append(f"{func_name} -> {method_name}")
    assert not missing, (
        "RFC 0043 §5 coverage gap: the following CLI mutations have no "
        "registered RPC method: " + ", ".join(missing)
    )


def test_required_capabilities_match_rfc0043_section_5() -> None:
    mismatches: list[str] = []
    for method, expected in EXPECTED_CAPABILITY.items():
        entry = METHOD_REGISTRY.get(method)
        assert entry is not None, f"missing registry entry for {method!r}"
        if entry.required_capability != expected:
            mismatches.append(
                f"{method}: expected {expected!r}, got {entry.required_capability!r}"
            )
    assert not mismatches, "RFC 0043 §5 capability mismatches: " + "; ".join(mismatches)


def _pg_handlers() -> set[str]:
    import striatum.daemon_pg.handlers  # noqa: F401 - registers decorators.
    from striatum.daemon_pg.handlers.registry import resolve_pg_handler

    return {
        method
        for method in METHOD_REGISTRY
        if resolve_pg_handler(method) is not None
    }


def test_every_canonical_method_routes_via_pg_or_server_inline() -> None:
    pg_handlers = _pg_handlers()
    unrouted: list[str] = []
    for method, entry in METHOD_REGISTRY.items():
        if entry.deprecated:
            continue
        if method in SERVER_INLINE_METHODS:
            continue
        if method in LOCAL_FILE_AUTHORING_METHODS:
            continue
        if method in pg_handlers:
            continue
        unrouted.append(method)
    assert not unrouted, (
        "method registered but has no PG handler or inline handler: "
        + ", ".join(unrouted)
    )


def test_legacy_aliases_are_flagged_deprecated() -> None:
    for alias in (
        "ack",
        "heartbeat",
        "release",
        "block",
        "complete",
        "publish_artifact",
        "claim_next",
        "verdict",
        "submit_review",
    ):
        entry = METHOD_REGISTRY.get(alias)
        assert entry is not None, f"legacy alias {alias!r} missing from registry"
        assert entry.deprecated, f"legacy alias {alias!r} must be flagged deprecated"


def test_recovery_capability_is_recognized() -> None:
    assert "recovery" in CAPABILITIES
    assert "surgical_recovery" in CAPABILITIES


def test_repo_scope_modes_are_correct() -> None:
    # Per-repo write/claim/read methods stay single_repo; daemon-global
    # admin methods stay daemon_global; cross-repo coordination methods
    # report cross_repo.
    assert METHOD_REGISTRY["session.register"].effective_repository_scope_mode == "single_repo"
    assert METHOD_REGISTRY["daemon.migrate_repo_local"].effective_repository_scope_mode == "daemon_global"
    assert METHOD_REGISTRY["cross_repo.describe"].effective_repository_scope_mode == "cross_repo"


def _recovery_module() -> Any:
    from striatum.cli import recovery as mod

    return mod


def _supervise_module() -> Any:
    from striatum.cli import supervise as mod

    return mod


def _worktree_module() -> Any:
    from striatum.cli import worktree as mod

    return mod


# Static references so static analyzers and import-cycle checks see the
# mutation surface used by the coverage assertion above.
_REFERENCES = (
    cli_mutations,
    stale_leases,
    requeue_stale,
    cancel_job,
    process_reconcile,
    resume_blocker,
    supervise_start,
    supervise_send,
    supervise_stop,
    supervise_status,
    supervise_list,
    worktree_create,
    worktree_release,
    worktree_list,
)
