"""Argparse construction for the Striatum CLI."""

from __future__ import annotations

import argparse
import os
from pathlib import Path

from striatum import __version__


def build_parser() -> argparse.ArgumentParser:
    """Build the top-level argument parser."""
    parser = argparse.ArgumentParser(prog="striatum")
    parser.add_argument(
        "--version",
        action="version",
        version=f"striatum {__version__}",
        help="print the running Striatum version and exit",
    )
    parser.add_argument("--repo", default=".", help="repository root")
    # RFC 0043 §3: --no-daemon retired; the daemon-required CLI is the only
    # supported mode. Parsing `--no-daemon` returns the standard argparse
    # "unrecognized arguments" error (exit code 2). --daemon remains as the
    # opt-in for V1 RFC 0028 daemon-read routing until that path is fully
    # absorbed by the RFC 0030 method registry.
    parser.add_argument(
        "--daemon",
        action="store_true",
        help="route supported read commands through the RFC 0028 registry-backed mode",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    init = sub.add_parser("init")
    init.add_argument("--json", action="store_true")
    init.add_argument(
        "--with-skills",
        nargs="?",
        const="claude_code",
        default=None,
        help=(
            "After init, also write the agent skill bundle for the given "
            "profile (default: claude_code). RFC 0015."
        ),
    )
    init.add_argument(
        "--with-plugins",
        nargs="?",
        const="claude_code",
        default=None,
        help=(
            "After init, also write the agent-CLI plugin bundle for the "
            "given profile (default: claude_code). RFC 0025."
        ),
    )
    init.add_argument(
        "--with-ddd-layout",
        action="store_true",
        help=(
            "After init, also scaffold the DDD-shaped human-facing doc layout "
            "(docs/SPEC.md, PRD.md, DECISION_LOG.md, UBIQUITOUS_LANGUAGE.md, "
            "DDD.md, rfcs/README.md, rfcs/0001-template.md) into the target "
            "repo. Existing files are preserved. RFC 0021."
        ),
    )
    init.add_argument(
        "--ddd-layout-force",
        action="store_true",
        help=(
            "With --with-ddd-layout, overwrite existing regular-file targets "
            "with the template body. Non-regular-file targets (directories, "
            "broken symlinks) still error and are not touched. Each "
            "overwritten file's prior content sha256 is recorded in the "
            "envelope for audit. RFC 0021 V1.5."
        ),
    )
    init.add_argument(
        "--ddd-layout-dry-run",
        action="store_true",
        help=(
            "With --with-ddd-layout, report what would happen without "
            "writing any files. Envelope statuses use the would_* "
            "vocabulary. Combine with --ddd-layout-force to preview "
            "destructive overwrites. RFC 0021 V1.5."
        ),
    )

    skills = sub.add_parser("skills")
    skills_sub = skills.add_subparsers(dest="skills_command", required=True)
    skills_install = skills_sub.add_parser("install")
    skills_install.add_argument(
        "--profile",
        choices=["claude_code", "codex", "gemini", "generic", "all"],
        default="claude_code",
    )
    skills_install.add_argument(
        "--scope", choices=["project", "user"], default="project"
    )
    skills_install.add_argument("--namespace", default="striatum-")
    skills_install.add_argument("--force", action="store_true")
    skills_install.add_argument("--dry-run", action="store_true")
    skills_install.add_argument("--json", action="store_true")

    plugin = sub.add_parser("plugin")
    plugin_sub = plugin.add_subparsers(dest="plugin_command", required=True)
    plugin_install = plugin_sub.add_parser("install")
    plugin_install.add_argument(
        "--profile",
        choices=["claude_code", "codex", "gemini", "all"],
        default="claude_code",
    )
    plugin_install.add_argument(
        "--scope", choices=["project", "user"], default="project"
    )
    plugin_install.add_argument("--namespace", default="striatum")
    plugin_install.add_argument("--target", default=None)
    plugin_install.add_argument("--force", action="store_true")
    plugin_install.add_argument("--dry-run", action="store_true")
    plugin_install.add_argument(
        "--with-marketplace", dest="with_marketplace", action="store_true", default=True,
    )
    plugin_install.add_argument(
        "--no-marketplace", dest="with_marketplace", action="store_false",
    )
    plugin_install.add_argument("--json", action="store_true")
    plugin_uninstall = plugin_sub.add_parser("uninstall")
    plugin_uninstall.add_argument(
        "--profile",
        choices=["claude_code", "codex", "gemini", "all"],
        default="claude_code",
    )
    plugin_uninstall.add_argument(
        "--scope", choices=["project", "user"], default="project"
    )
    plugin_uninstall.add_argument("--namespace", default="striatum")
    plugin_uninstall.add_argument("--target", default=None)
    plugin_uninstall.add_argument("--force", action="store_true")
    plugin_uninstall.add_argument("--json", action="store_true")

    self_update = sub.add_parser(
        "self-update",
        help=(
            "Reinstall striatum-orchestrator from a source checkout and "
            "refresh the operator skills + plugin profile in one step. "
            "Useful for operators on target repos who hit version-staleness "
            "when the source moves."
        ),
        description=(
            "Run `pip install -e <source> --force-reinstall --user "
            "--break-system-packages --no-deps`, then `striatum plugin "
            "install --profile <profile>` and `striatum skills install "
            "--profile <profile>`. Failures of the plugin/skills steps "
            "are non-fatal — the pip step is the authoritative success "
            "signal."
        ),
    )
    self_update.add_argument(
        "--source",
        default=str(Path(__file__).resolve().parents[3]),
        help="Path to the striatum source checkout to install from (defaults to the repo containing this CLI module).",
    )
    self_update.add_argument(
        "--profile",
        choices=["claude_code", "codex", "gemini", "all"],
        default="claude_code",
        help="Operator profile passed to plugin install + skills install.",
    )
    self_update.add_argument(
        "--scope",
        choices=["project", "user"],
        default="user",
        help="Operator-scope flag passed to plugin install + skills install (default user; project for in-repo overrides).",
    )
    self_update.add_argument(
        "--skip-plugin", action="store_true",
        help="Skip the plugin install step.",
    )
    self_update.add_argument(
        "--skip-skills", action="store_true",
        help="Skip the skills install step.",
    )
    self_update.add_argument(
        "--dry-run", action="store_true",
        help="Report the commands that would run without executing them.",
    )
    self_update.add_argument("--json", action="store_true")

    adopt = sub.add_parser(
        "adopt",
        help="guided day-zero setup for a target repository",
    )
    adopt.add_argument(
        "--profile",
        choices=["claude_code", "codex", "gemini", "generic", "all"],
        default="claude_code",
    )
    adopt.add_argument("--postgres-url")
    adopt.add_argument("--dry-run", action="store_true")
    adopt.add_argument("--no-skills", dest="with_skills", action="store_false", default=True)
    adopt.add_argument("--no-plugins", dest="with_plugins", action="store_false", default=True)
    adopt.add_argument(
        "--no-ddd-layout",
        dest="with_ddd_layout",
        action="store_false",
        default=True,
    )
    adopt.add_argument("--no-register", dest="register", action="store_false", default=True)
    adopt.add_argument("--json", action="store_true")

    daemon = sub.add_parser("daemon")
    daemon_sub = daemon.add_subparsers(dest="daemon_command", required=True)
    daemon_start = daemon_sub.add_parser("start")
    daemon_start.add_argument(
        "--core",
        choices=["python", "go"],
        default=os.environ.get("STRIATUM_DAEMON_CORE"),
        help=(
            "daemon implementation to launch; defaults to "
            "STRIATUM_DAEMON_CORE or python"
        ),
    )
    daemon_start.add_argument("--sweep-interval-seconds", type=float, default=60.0)
    daemon_start.add_argument("--max-sweeps", type=int, default=None)
    daemon_start.add_argument("--postgres-url")
    daemon_start.add_argument("--json", action="store_true")
    daemon_doctor = daemon_sub.add_parser("doctor")
    daemon_doctor.add_argument("--postgres-url")
    daemon_doctor.add_argument("--apply-migrations", action="store_true")
    daemon_doctor.add_argument(
        "--provision-rw-role",
        action="store_true",
        help="create the local striatumd_rw runtime role when the current Postgres user can create roles",
    )
    daemon_doctor.add_argument(
        "--repair-grants",
        action="store_true",
        help="apply the runtime role grants/revokes required for append-only daemon tables",
    )
    daemon_doctor.add_argument(
        "--explain",
        action="store_true",
        help="show per-method native PG, inline, or CLI-local daemon authority",
    )
    daemon_doctor.add_argument("--json", action="store_true")
    daemon_migrate = daemon_sub.add_parser("migrate")
    daemon_migrate.add_argument("--from", dest="from_substrate", choices=["sqlite"], required=True)
    daemon_migrate.add_argument("--to", dest="to_substrate", choices=["pg"], required=True)
    daemon_migrate.add_argument("--postgres-url")
    daemon_migrate.add_argument("--source-registry")
    daemon_migrate.add_argument("--dry-run", action="store_true")
    daemon_migrate.add_argument("--keep-sqlite-readonly", action="store_true")
    daemon_migrate.add_argument("--json", action="store_true")
    daemon_migrate_repo = daemon_sub.add_parser(
        "migrate-repo-local",
        help=(
            "migrate one repo-local .striatum/state.sqlite3 into daemon "
            "PostgreSQL state"
        ),
        description=(
            "Migrate a repo's local SQLite workflow state into the "
            "daemon's PostgreSQL substrate. Runs in a single serializable "
            "Postgres transaction with byte-equivalent audit-chain "
            "re-anchor. After commit the source is finalized as a 0444 "
            "tombstone (default) or deleted (with --no-keep-sqlite-readonly "
            "+ --confirm-delete)."
        ),
    )
    daemon_migrate_repo.add_argument(
        "--from",
        dest="from_substrate",
        choices=["sqlite"],
        required=True,
        help="source substrate; only 'sqlite' is supported in V1.6",
    )
    daemon_migrate_repo.add_argument(
        "--to",
        dest="to_substrate",
        choices=["pg"],
        required=True,
        help="destination substrate; only 'pg' is supported in V1.6",
    )
    daemon_migrate_repo.add_argument(
        "--repo",
        dest="repo_local_repo",
        default=None,
        help="path to the target repo's working tree; defaults to the top-level --repo",
    )
    daemon_migrate_repo.add_argument(
        "--postgres-url",
        help="override STRIATUM_DAEMON_DB_URL for this migration only",
    )
    daemon_migrate_repo.add_argument(
        "--dry-run",
        action="store_true",
        help="report what would be migrated without writing to Postgres or finalizing SQLite",
    )
    daemon_migrate_repo.add_argument(
        "--keep-sqlite-readonly",
        dest="keep_sqlite_readonly",
        action="store_true",
        default=True,
        help="rename state.sqlite3 to state.sqlite3.tombstone and chmod it 0444 (default)",
    )
    daemon_migrate_repo.add_argument(
        "--no-keep-sqlite-readonly",
        dest="keep_sqlite_readonly",
        action="store_false",
        help="delete state.sqlite3 after migration; requires --confirm-delete",
    )
    daemon_migrate_repo.add_argument(
        "--confirm-delete",
        action="store_true",
        help="acknowledgement required when --no-keep-sqlite-readonly is used",
    )
    daemon_migrate_repo.add_argument(
        "--json",
        action="store_true",
        help="emit machine-readable JSON output to stdout instead of text",
    )
    daemon_status = daemon_sub.add_parser("status")
    daemon_status.add_argument("--json", action="store_true")
    daemon_stop = daemon_sub.add_parser("stop")
    daemon_stop.add_argument("--json", action="store_true")
    daemon_health = daemon_sub.add_parser("health")
    daemon_health.add_argument("--json", action="store_true")
    daemon_audit = daemon_sub.add_parser("audit")
    daemon_audit.add_argument("--limit", type=_positive_int, default=100)
    daemon_audit.add_argument("--json", action="store_true")
    daemon_sweep = daemon_sub.add_parser("sweep")
    daemon_sweep.add_argument("--json", action="store_true")
    daemon_service = daemon_sub.add_parser("service")
    daemon_service_sub = daemon_service.add_subparsers(dest="service_command", required=True)
    daemon_service_install = daemon_service_sub.add_parser("install")
    daemon_service_install.add_argument("--manager", choices=["auto", "systemd", "launchd"], default="auto")
    daemon_service_install.add_argument("--dry-run", action="store_true")
    daemon_service_install.add_argument("--json", action="store_true")
    daemon_service_start = daemon_service_sub.add_parser("start")
    daemon_service_start.add_argument("--manager", choices=["auto", "systemd", "launchd"], default="auto")
    daemon_service_start.add_argument("--dry-run", action="store_true")
    daemon_service_start.add_argument("--json", action="store_true")
    daemon_service_status = daemon_service_sub.add_parser("status")
    daemon_service_status.add_argument("--manager", choices=["auto", "systemd", "launchd"], default="auto")
    daemon_service_status.add_argument("--json", action="store_true")

    repo_cmd = sub.add_parser("repo")
    repo_sub = repo_cmd.add_subparsers(dest="repo_command", required=True)
    repo_add = repo_sub.add_parser("add")
    repo_add.add_argument("path")
    repo_add.add_argument("--display-name")
    repo_add.add_argument(
        "--init",
        action="store_true",
        help="initialize .striatum/state.sqlite3 before registering when absent",
    )
    repo_add.add_argument("--no-migrate", action="store_true")
    repo_add.add_argument("--json", action="store_true")
    repo_list = repo_sub.add_parser("list")
    repo_list.add_argument("--json", action="store_true")
    repo_remove = repo_sub.add_parser("remove")
    repo_remove.add_argument("id")
    repo_remove.add_argument("--json", action="store_true")

    cross_repo = sub.add_parser("cross-repo")
    cross_repo_sub = cross_repo.add_subparsers(dest="cross_repo_command", required=True)
    cross_repo_list = cross_repo_sub.add_parser("list")
    cross_repo_list.add_argument("--postgres-url")
    cross_repo_list.add_argument("--json", action="store_true")
    cross_repo_describe = cross_repo_sub.add_parser("describe")
    cross_repo_describe.add_argument("cross_repo_run_id")
    cross_repo_describe.add_argument("--postgres-url")
    cross_repo_describe.add_argument("--json", action="store_true")
    cross_repo_why = cross_repo_sub.add_parser("why")
    cross_repo_why.add_argument("cross_repo_run_id")
    cross_repo_why.add_argument("--postgres-url")
    cross_repo_why.add_argument("--json", action="store_true")
    cross_repo_cancel = cross_repo_sub.add_parser("cancel")
    cross_repo_cancel.add_argument("cross_repo_run_id")
    cross_repo_cancel.add_argument("--reason", default="operator canceled cross-repo run")
    cross_repo_cancel.add_argument("--postgres-url")
    cross_repo_cancel.add_argument("--json", action="store_true")

    workflow = sub.add_parser("workflow")
    workflow_sub = workflow.add_subparsers(dest="workflow_command", required=True)
    validate = workflow_sub.add_parser("validate")
    validate.add_argument("path")
    validate.add_argument("--json", action="store_true")
    lint = workflow_sub.add_parser("lint")
    lint.add_argument("path")
    lint.add_argument(
        "--strict",
        action="store_true",
        help=(
            "refuse workflows with lint warnings unless "
            "--override-rationale is provided"
        ),
    )
    lint.add_argument(
        "--override-rationale",
        help="explicit operator rationale for accepting strict lint warnings",
    )
    lint.add_argument("--json", action="store_true")
    plan = workflow_sub.add_parser("plan")
    plan.add_argument("path")
    plan.add_argument("--json", action="store_true")
    graph = workflow_sub.add_parser("graph")
    graph.add_argument("path")
    graph.add_argument("--format", choices=["mermaid", "json", "dot"], default="mermaid")
    graph.add_argument("--json", action="store_true")
    workflow_init = workflow_sub.add_parser("init")
    workflow_init.add_argument("path")
    workflow_init.add_argument(
        "--style",
        choices=["minimal", "review", "code-change"],
        default="review",
        help="template style for the generated workflow",
    )
    workflow_init.add_argument("--json", action="store_true")
    workflow_upgrade_p = workflow_sub.add_parser(
        "upgrade",
        description=(
            "RFC 0040 V1: backport the per-model harness-profile fragments "
            "from the bundled template catalog into an existing workflow.json. "
            "Refuse-on-conflict default; pass --force to overwrite a "
            "non-default instruction. --dry-run reports the change set without "
            "writing. Refuses if any non-terminal run references the workflow."
        ),
    )
    workflow_upgrade_p.add_argument("path")
    workflow_upgrade_p.add_argument(
        "--force",
        action="store_true",
        help="overwrite native_delegation.instruction even when it conflicts with the catalog default",
    )
    workflow_upgrade_p.add_argument(
        "--dry-run",
        action="store_true",
        help="report the change set without writing the workflow.json",
    )
    workflow_upgrade_p.add_argument(
        "--add-phases",
        action="store_true",
        help=(
            "rewrite a v1 workflow to v1.1 by inferring phases from "
            "parallel_group clusters; preview-only unless --apply is passed"
        ),
    )
    workflow_upgrade_p.add_argument(
        "--apply",
        action="store_true",
        help="with --add-phases, write the rewritten workflow.json",
    )
    workflow_upgrade_p.add_argument("--json", action="store_true")
    templates = workflow_sub.add_parser("templates")
    templates_sub = templates.add_subparsers(dest="templates_command", required=True)
    templates_list = templates_sub.add_parser("list")
    templates_list.add_argument("--kind", choices=["shape", "lane_set"])
    templates_list.add_argument("--json", action="store_true")
    templates_show = templates_sub.add_parser("show")
    templates_show.add_argument("template_id")
    templates_show.add_argument("--json", action="store_true")
    generate = workflow_sub.add_parser(
        "generate",
        description=(
            "Generate a workflow tree from the local template catalog. "
            "Start with `striatum workflow templates list`; example: "
            "striatum workflow generate workflows/my-change --shape code_change "
            "--lane-set local --artifact-root striatum/my-change --dry-run --json. "
            "`workflow validate` remains authoritative."
        ),
    )
    generate.add_argument("path")
    generate.add_argument("--shape", required=True)
    generate.add_argument("--lane-set", required=True)
    generate.add_argument("--artifact-root", required=True)
    generate.add_argument("--lane-modifier", action="append", default=[])
    generate.add_argument("--plan")
    generate.add_argument("--workflow-id")
    generate.add_argument("--name")
    generate.add_argument("--workflow-version")
    generate.add_argument("--branch-suggestion")
    generate.add_argument("--lane-command", action="append", default=[])
    generate.add_argument("--lane-display-model", action="append", default=[])
    generate.add_argument("--option", action="append", default=[])
    generate.add_argument("--dry-run", action="store_true")
    generate.add_argument("--json", action="store_true")

    run = sub.add_parser("run")
    run_sub = run.add_subparsers(dest="run_command", required=True)
    prepare = run_sub.add_parser("prepare")
    prepare.add_argument("--workflow", required=True)
    prepare.add_argument("--json", action="store_true")
    start = run_sub.add_parser("start")
    start.add_argument("--run-id", required=True)
    start.add_argument("--json", action="store_true")
    cancel_p = run_sub.add_parser("cancel")
    cancel_p.add_argument("--run-id", required=True)
    cancel_p.add_argument("--reason")
    cancel_p.add_argument("--json", action="store_true")
    pause_p = run_sub.add_parser("pause")
    pause_p.add_argument("--run-id", required=True)
    pause_p.add_argument("--reason")
    pause_p.add_argument("--json", action="store_true")
    resume_p = run_sub.add_parser("resume")
    resume_p.add_argument("--run-id", required=True)
    resume_p.add_argument("--json", action="store_true")
    retry_job_p = run_sub.add_parser("retry-job")
    retry_job_p.add_argument("--run-id", required=True)
    retry_job_p.add_argument("--job-id", required=True)
    retry_job_p.add_argument("--json", action="store_true")
    summary = run_sub.add_parser("summary")
    summary.add_argument("--run-id", required=True)
    summary.add_argument("--path", required=True)
    summary.add_argument("--json", action="store_true")
    run_graph_p = run_sub.add_parser("graph")
    run_graph_p.add_argument("--run-id", required=True)
    run_graph_p.add_argument(
        "--format",
        choices=["mermaid", "json", "ascii"],
        default="mermaid",
    )
    # RFC 0016 step 3: graph orientation (ignored when --format != ascii).
    run_graph_p.add_argument(
        "--graph-orient",
        choices=["tb", "lr"],
        default="tb",
    )
    run_graph_p.add_argument(
        "--graph-style",
        choices=["auto", "layered", "list", "fancy"],
        default="layered",
    )
    run_graph_p.add_argument("--json", action="store_true")

    branch = sub.add_parser("branch")
    branch_sub = branch.add_subparsers(dest="branch_command", required=True)
    confirm = branch_sub.add_parser("confirm")
    confirm.add_argument("--run-id", required=True)
    confirm.add_argument("--branch", required=True)
    confirm.add_argument("--create", action="store_true")
    confirm.add_argument("--use-current", action="store_true")
    confirm.add_argument(
        "--strict",
        action="store_true",
        help="require the current git branch to match --branch before recording (safe default for CI)",
    )
    confirm.add_argument("--json", action="store_true")

    register = sub.add_parser("register-session")
    register.add_argument("--run-id", required=True)
    register.add_argument("--role", required=True)
    register.add_argument("--lane", required=True)
    register.add_argument("--capability", action="append", default=[])
    register.add_argument("--fresh", action="store_true")
    register.add_argument("--parent-session-id")
    # HARNESS-003: operator escape hatch for the fresh-reviewer policy.
    # When the workflow declares a fresh reviewer and an active author
    # session already exists on the run, register-session refuses unless
    # --force-non-fresh is passed with a non-empty --reason. The reason
    # is stored on the session row (``non_fresh_reason``) so evidence
    # exports record the breach explicitly.
    register.add_argument("--force-non-fresh", action="store_true")
    register.add_argument("--reason")
    register.add_argument("--operator-label")
    register.add_argument("--json", action="store_true")

    # RFC 0011: session command group. ``close`` is the only subcommand
    # for now; future entries (e.g. ``list``, ``inspect``) can extend
    # the group without polluting the top-level subparser namespace.
    session = sub.add_parser("session")
    session_sub = session.add_subparsers(dest="session_command", required=True)
    session_close = session_sub.add_parser("close")
    session_close.add_argument("--session-id", required=True)
    session_close.add_argument("--reason", required=True)
    session_close.add_argument("--json", action="store_true")

    claim = sub.add_parser("claim-next")
    claim.add_argument("--session-id", required=True)
    claim.add_argument("--lease-seconds", type=int, default=1800)
    claim.add_argument("--json", action="store_true")

    ack = sub.add_parser("ack")
    add_work_identity(ack)

    heartbeat = sub.add_parser("heartbeat")
    heartbeat.add_argument("--session-id", required=True)
    heartbeat.add_argument("--lease-id", required=True)
    heartbeat.add_argument("--extend-seconds", type=int, default=1800)
    heartbeat.add_argument("--json", action="store_true")

    release = sub.add_parser("release")
    add_work_identity(release)
    release.add_argument("--reason", required=True)
    release.add_argument("--requeue", action="store_true")

    send = sub.add_parser("send")
    send.add_argument("--session-id", required=True)
    send.add_argument("--kind", required=True)
    send.add_argument("--body-json", default="{}")
    send.add_argument("--json", action="store_true")

    block = sub.add_parser("block")
    block.add_argument("--session-id", required=True)
    block.add_argument("--job-id", required=True)
    block.add_argument("--lease-id", required=True)
    block.add_argument("--kind", required=True)
    block.add_argument("--severity", choices=["blocked", "human_checkpoint"], required=True)
    block.add_argument("--description", required=True)
    block.add_argument("--json", action="store_true")

    publish = sub.add_parser(
        "publish-artifact",
        help="record an artifact reference for a claimed packet",
        description=(
            "Record an artifact reference for a claimed packet. V1.41: "
            "--kind and --logical-name default from the workflow's "
            "expected_artifacts when --path matches a declared artifact "
            "path. Pass them explicitly only to override or when the path "
            "doesn't match a declared artifact."
        ),
    )
    publish.add_argument("--session-id", required=True)
    publish.add_argument("--job-id", required=True)
    publish.add_argument("--lease-id", required=True)
    publish.add_argument(
        "--kind",
        help="artifact kind; defaults from expected_artifacts when --path matches",
    )
    publish.add_argument(
        "--logical-name",
        help="logical name; defaults from expected_artifacts when --path matches",
    )
    publish.add_argument("--path", required=True)
    publish.add_argument(
        "--allow-no-process-execution",
        dest="allow_no_process_execution",
        action="store_true",
        help=(
            "RFC 0046 V1: opt out of the lane evidence guard. Required "
            "when publishing under a model byline for a session that "
            "has no matching process_executions row covering the "
            "artifact path. Must be paired with --override-rationale."
        ),
    )
    publish.add_argument(
        "--override-rationale",
        dest="override_rationale",
        default=None,
        help=(
            "non-empty operator explanation written to the artifact's "
            "attestation_override_rationale column and to a "
            "provenance.publish_without_process_execution event"
        ),
    )
    publish.add_argument("--json", action="store_true")

    complete = sub.add_parser("complete")
    complete.add_argument("--session-id", required=True)
    complete.add_argument("--job-id", required=True)
    complete.add_argument("--lease-id", required=True)
    complete.add_argument("--summary")
    complete.add_argument("--json", action="store_true")

    verdict = sub.add_parser("verdict")
    verdict.add_argument("--session-id", required=True)
    verdict.add_argument("--job-id", required=True)
    verdict.add_argument("--lease-id", required=True)
    verdict.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings", "needs_revision", "reject"],
        required=True,
    )
    verdict.add_argument("--findings-artifact-id")
    verdict.add_argument("--rationale")
    verdict.add_argument("--json", action="store_true")

    override_verdict = sub.add_parser(
        "override-verdict",
        help="operator override of a reviewer's verdict",
        description=(
            "Operator override of a reviewer's verdict. V1.41: "
            "--auto-fresh-session registers a fresh operator session "
            "when the supplied --session-id already has a verdict for "
            "this job (the override path requires a different session)."
        ),
    )
    override_verdict.add_argument("--session-id", required=True)
    override_verdict.add_argument("--job-id", required=True)
    override_verdict.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings"],
        required=True,
    )
    override_verdict.add_argument("--findings-artifact-id")
    override_verdict.add_argument("--rationale", required=True)
    override_verdict.add_argument(
        "--auto-fresh-session",
        action="store_true",
        help=(
            "if --session-id already has a verdict for this job, "
            "automatically register a fresh operator reviewer session "
            "on the same lane and use it for the override"
        ),
    )
    override_verdict.add_argument("--json", action="store_true")

    submit_review = sub.add_parser("submit-review")
    submit_review.add_argument("--session-id", required=True)
    submit_review.add_argument("--job-id", required=True)
    submit_review.add_argument("--lease-id", required=True)
    submit_review.add_argument("--path", required=True)
    submit_review.add_argument(
        "--verdict",
        choices=["accept", "accept_with_findings", "needs_revision", "reject"],
        required=True,
    )
    submit_review.add_argument("--logical-name", default="review")
    submit_review.add_argument("--kind", default="finding")
    submit_review.add_argument("--rationale")
    submit_review.add_argument(
        "--allow-no-process-execution",
        dest="allow_no_process_execution",
        action="store_true",
        help=(
            "RFC 0046 V1: opt out of the lane evidence guard for this "
            "submit-review (operator-on-behalf flows). Must be paired "
            "with --override-rationale."
        ),
    )
    submit_review.add_argument(
        "--override-rationale",
        dest="override_rationale",
        default=None,
        help="non-empty operator explanation recorded with the artifact",
    )
    submit_review.add_argument("--json", action="store_true")

    evidence = sub.add_parser("evidence")
    evidence_sub = evidence.add_subparsers(dest="evidence_command", required=True)
    evidence_export = evidence_sub.add_parser("export")
    evidence_export.add_argument("--run-id", required=True)
    evidence_export.add_argument("--path", required=True)
    evidence_export.add_argument("--json", action="store_true")

    corpus = sub.add_parser("corpus")
    corpus_sub = corpus.add_subparsers(dest="corpus_command", required=True)
    corpus_export = corpus_sub.add_parser("export")
    corpus_export.add_argument("--since", required=True)
    corpus_export.add_argument("--out", required=True)
    corpus_export.add_argument("--json", action="store_true")
    corpus_verify = corpus_sub.add_parser("verify")
    corpus_verify.add_argument("--bundle", required=True)
    corpus_verify.add_argument("--json", action="store_true")

    archive = sub.add_parser("archive")
    archive_sub = archive.add_subparsers(dest="archive_command", required=True)
    archive_create = archive_sub.add_parser("create")
    archive_create.add_argument("--run-id", required=True)
    archive_create.add_argument("--out", required=True)
    archive_create.add_argument("--json", action="store_true")
    archive_verify = archive_sub.add_parser("verify")
    archive_verify.add_argument("--bundle", required=True)
    archive_verify.add_argument("--json", action="store_true")

    decision = sub.add_parser("decision")
    decision_sub = decision.add_subparsers(dest="decision_command", required=True)
    decision_record = decision_sub.add_parser("record")
    decision_record.add_argument("--run-id", required=True)
    decision_record.add_argument("--path", required=True)
    decision_record.add_argument(
        "--outcome",
        choices=["accepted", "rejected", "accepted_with_follow_up"],
        required=True,
    )
    decision_record.add_argument("--title", required=True)
    decision_record.add_argument("--decision-id")
    decision_record.add_argument("--rationale")
    decision_record.add_argument("--follow-up")
    decision_record.add_argument("--json", action="store_true")

    status = sub.add_parser("status")
    status.add_argument("--run-id")
    status.add_argument("--json", action="store_true")

    why = sub.add_parser("why")
    why.add_argument("id")
    why.add_argument("--json", action="store_true")

    doctor = sub.add_parser("doctor")
    doctor.add_argument("--run-id")
    doctor.add_argument(
        "--first-run",
        action="store_true",
        help="run day-zero checks for daemon, Postgres, token, registration, MCP, and sample read routing",
    )
    doctor.add_argument(
        "--verbose",
        action="store_true",
        help="include structured problem_records alongside the existing problems list",
    )
    doctor.add_argument("--json", action="store_true")

    recovery = sub.add_parser("recovery")
    recovery_sub = recovery.add_subparsers(dest="recovery_command", required=True)
    stale_leases = recovery_sub.add_parser("stale-leases")
    stale_leases.add_argument("--run-id", required=True)
    stale_leases.add_argument("--json", action="store_true")
    requeue_stale = recovery_sub.add_parser("requeue-stale")
    requeue_stale.add_argument("--run-id", required=True)
    requeue_stale.add_argument("--job-id", required=True)
    requeue_stale.add_argument(
        "--force",
        action="store_true",
        help="(GH #19) requeue a repo_write stale job after manual inspection; pair with --justification",
    )
    requeue_stale.add_argument(
        "--justification",
        help="(GH #19) operator-override audit-chained reason; required with --force",
    )
    requeue_stale.add_argument("--json", action="store_true")
    cancel_job_p = recovery_sub.add_parser("cancel-job")
    cancel_job_p.add_argument("--run-id", required=True)
    cancel_job_p.add_argument("--job-id", required=True)
    cancel_job_p.add_argument("--reason", required=True)
    cancel_job_p.add_argument(
        "--cascade",
        action="store_true",
        help="also cancel downstream blocked jobs whose only path was through this job",
    )
    cancel_job_p.add_argument("--json", action="store_true")
    process_reconcile_p = recovery_sub.add_parser(
        "process-reconcile",
        help=(
            "RFC 0014 V1: walk process_executions rows in 'running' "
            "state and transition externally-killed processes to 'lost', "
            "re-running output validation on the newly-lost rows."
        ),
    )
    process_reconcile_p.add_argument("--run-id", required=True)
    process_reconcile_p.add_argument("--json", action="store_true")
    resume_p = recovery_sub.add_parser(
        "resume",
        help=(
            "Resolve a process-adapter blocker after operator remediation "
            "and return the job to running, optionally completing it."
        ),
    )
    resume_p.add_argument("--blocker-id", required=True)
    resume_p.add_argument(
        "--complete",
        action="store_true",
        help="complete the resumed job after revalidation",
    )
    resume_p.add_argument(
        "--session-id",
        help="lease-owning session id; required with --complete",
    )
    resume_p.add_argument("--summary")
    resume_p.add_argument(
        "--force",
        action="store_true",
        help="required for nonzero-exit or timeout blockers",
    )
    resume_p.add_argument(
        "--extend-seconds",
        type=int,
        default=1800,
        help="extend the existing process-adapter lease while resuming",
    )
    resume_p.add_argument("--json", action="store_true")

    # RFC 0020 V1: autonomous-recovery sweeper.
    recovery_auto = recovery_sub.add_parser(
        "auto",
        help=(
            "RFC 0020 V1: one-shot autonomous-recovery sweep — lazy "
            "lease expiry, optional process reconcile, optional review-"
            "only requeue, checkpoint-timeout escalation, eligible-"
            "blocker doctor flag. Composable with cron / systemd timer."
        ),
    )
    recovery_auto.add_argument("--run-id", required=True)
    recovery_auto.add_argument("--dry-run", action="store_true")
    recovery_auto.add_argument(
        "--autonomous-review-requeue",
        dest="autonomous_review_requeue",
        action="store_true",
        default=None,
    )
    recovery_auto.add_argument(
        "--autonomous-process-reconcile",
        dest="autonomous_process_reconcile",
        action="store_true",
        default=None,
    )
    recovery_auto.add_argument(
        "--max-requeue", dest="max_requeues_per_sweep", type=int, default=None
    )
    recovery_auto.add_argument(
        "--checkpoint-timeout",
        dest="checkpoint_timeout_seconds",
        type=int,
        default=None,
    )
    recovery_auto.add_argument(
        "--eligible-after",
        dest="eligible_after_seconds",
        type=int,
        default=None,
    )
    recovery_auto.add_argument("--json", action="store_true")

    # V1.41 (harness friction burn-down): auto-publish on stale lease when
    # a session wrote the expected_artifact to disk but never called
    # ack/publish/complete. Drives down the recurring claude-no-publish
    # anti-pattern that has shown up across 6+ dogfoods.
    recovery_auto_publish = recovery_sub.add_parser(
        "auto-publish",
        help=(
            "V1.41 burn-down: walk stale leases and auto-publish "
            "on-disk artifacts when the expected_artifacts path is "
            "present and conformant. Two-condition gate: artifact "
            "byline must match expected_author_line exactly and path "
            "must match the workflow's declared path exactly."
        ),
    )
    recovery_auto_publish.add_argument("--run-id", required=True)
    recovery_auto_publish.add_argument(
        "--dry-run",
        action="store_true",
        help="report what would be published without writing",
    )
    recovery_auto_publish.add_argument("--json", action="store_true")

    recovery_auto_finalize = recovery_sub.add_parser(
        "auto-finalize",
        help=(
            "RFC 0051 V1: publish and complete work from stable declared "
            "expected artifacts with valid front matter and byline metadata. "
            "Dry-run by default; live mode requires workflow opt-in or --force."
        ),
    )
    recovery_auto_finalize.add_argument("--run-id", required=True)
    recovery_auto_finalize.add_argument("--job-id")
    recovery_auto_finalize.add_argument(
        "--dry-run",
        dest="dry_run",
        action="store_true",
        default=True,
        help="report eligible artifacts without writing (default)",
    )
    recovery_auto_finalize.add_argument(
        "--live",
        dest="dry_run",
        action="store_false",
        help="perform eligible auto-finalize transitions",
    )
    recovery_auto_finalize.add_argument(
        "--mtime-grace-seconds",
        type=int,
        default=None,
        help="minimum artifact file age before it can auto-finalize",
    )
    recovery_auto_finalize.add_argument(
        "--force",
        action="store_true",
        help="allow live mode before workflow recovery.auto_finalize opt-in",
    )
    recovery_auto_finalize.add_argument(
        "--allow-no-process-execution",
        dest="allow_no_process_execution",
        action="store_true",
        help=(
            "allow model-bylined auto-finalize when no clean supervised "
            "process execution row exists; records an explicit provenance event"
        ),
    )
    recovery_auto_finalize.add_argument("--json", action="store_true")

    # RFC 0020 step 3: long-lived sweeper daemon.
    recovery_watch = recovery_sub.add_parser(
        "watch",
        help=(
            "RFC 0020 step 3: long-lived autonomous-recovery sweeper. "
            "Wraps `recovery auto` in a sleep loop with single-instance "
            "pidfile + signal-driven shutdown + JSONL emission."
        ),
    )
    recovery_watch.add_argument("--run-id", required=True)
    recovery_watch.add_argument(
        "--interval-seconds", type=float, default=60.0,
        help="seconds between sweeps; default 60",
    )
    exit_group = recovery_watch.add_mutually_exclusive_group()
    exit_group.add_argument(
        "--exit-on-terminal", dest="exit_on_terminal",
        action="store_true", default=True,
        help="exit cleanly when the run reaches a terminal state (default)",
    )
    exit_group.add_argument(
        "--no-exit-on-terminal", dest="exit_on_terminal",
        action="store_false",
        help="keep looping even after the run reaches a terminal state",
    )
    recovery_watch.add_argument(
        "--max-sweeps", type=int, default=None,
        help="cap total sweeps; default unlimited",
    )
    recovery_watch.add_argument(
        "--autonomous-review-requeue",
        dest="autonomous_review_requeue",
        action="store_true",
        default=None,
    )
    recovery_watch.add_argument(
        "--autonomous-process-reconcile",
        dest="autonomous_process_reconcile",
        action="store_true",
        default=None,
    )
    recovery_watch.add_argument(
        "--max-requeue", dest="max_requeues_per_sweep", type=int, default=None,
    )
    recovery_watch.add_argument(
        "--checkpoint-timeout", dest="checkpoint_timeout_seconds", type=int, default=None,
    )
    recovery_watch.add_argument(
        "--eligible-after", dest="eligible_after_seconds", type=int, default=None,
    )
    recovery_watch.add_argument("--json", action="store_true")

    checkpoint = sub.add_parser("checkpoint")
    checkpoint_sub = checkpoint.add_subparsers(dest="checkpoint_command", required=True)
    checkpoint_resolve_p = checkpoint_sub.add_parser("resolve")
    checkpoint_resolve_p.add_argument("--blocker-id", required=True)
    checkpoint_resolve_p.add_argument(
        "--action",
        choices=["continue", "cancel"],
        required=True,
    )
    checkpoint_resolve_p.add_argument(
        "--decision-id",
        help="optional decision artifact id to record alongside the resolution",
    )
    checkpoint_resolve_p.add_argument("--json", action="store_true")

    escalation = sub.add_parser(
        "escalation",
        help="inspect and resolve human-principal escalation blockers",
    )
    escalation_sub = escalation.add_subparsers(dest="escalation_command", required=True)
    escalation_list = escalation_sub.add_parser("list")
    escalation_list.add_argument("--run-id")
    escalation_list.add_argument(
        "--state",
        choices=["open", "resolved", "canceled", "all"],
        default="open",
    )
    escalation_list.add_argument("--limit", type=_positive_int, default=100)
    escalation_list.add_argument("--json", action="store_true")

    escalation_show = escalation_sub.add_parser("show")
    escalation_show.add_argument("--escalation-id", required=True)
    escalation_show.add_argument("--json", action="store_true")

    escalation_resolve = escalation_sub.add_parser("resolve")
    escalation_resolve.add_argument("--escalation-id", required=True)
    escalation_resolve.add_argument("--decision-id")
    escalation_resolve.add_argument("--resolution-note")
    escalation_resolve.add_argument("--json", action="store_true")

    adapter = sub.add_parser("adapter")
    adapter_sub = adapter.add_subparsers(dest="adapter_command", required=True)
    adapter_run = adapter_sub.add_parser("run")
    adapter_run.add_argument("--session-id", required=True)
    adapter_run.add_argument("--lease-id", required=True)
    adapter_run.add_argument("--stdin", choices=["packet", "none"], default="packet")
    adapter_run.add_argument("--inherit-stdio", action="store_true")
    adapter_run.add_argument(
        "--timeout-seconds",
        type=int,
        default=None,
        help=(
            "RFC 0014 V1: SIGTERM the child after N seconds and block the "
            "job with process_timeout_exceeded. Overrides any "
            "lanes.<id>.adapter_timeout_seconds default. Omit (or set to "
            "the lane default) to keep the historical unbounded behaviour."
        ),
    )
    adapter_run.add_argument("--json", action="store_true")

    worktree = sub.add_parser("worktree")
    worktree_sub = worktree.add_subparsers(dest="worktree_command", required=True)
    worktree_create = worktree_sub.add_parser("create")
    worktree_create.add_argument("--session-id", required=True)
    worktree_create.add_argument("--job-id", required=True)
    worktree_create.add_argument("--lease-id", required=True)
    worktree_create.add_argument("--json", action="store_true")
    worktree_release = worktree_sub.add_parser("release")
    worktree_release.add_argument("--worktree-id", required=True)
    worktree_release.add_argument("--json", action="store_true")
    worktree_list = worktree_sub.add_parser("list")
    worktree_list.add_argument("--run-id")
    worktree_list.add_argument("--json", action="store_true")

    supervise = sub.add_parser("supervise")
    supervise_sub = supervise.add_subparsers(dest="supervise_command", required=True)
    supervise_start_p = supervise_sub.add_parser("start")
    supervise_start_p.add_argument("--session-id", required=True)
    supervise_start_p.add_argument("--json", action="store_true")
    supervise_send_p = supervise_sub.add_parser("send")
    supervise_send_p.add_argument("--session-id", required=True)
    supervise_send_p.add_argument("--packet-id", required=True)
    supervise_send_p.add_argument("--json", action="store_true")
    supervise_stop_p = supervise_sub.add_parser("stop")
    supervise_stop_p.add_argument("--session-id", required=True)
    supervise_stop_p.add_argument("--reason", required=True)
    supervise_stop_p.add_argument("--json", action="store_true")
    supervise_status_p = supervise_sub.add_parser("status")
    supervise_status_p.add_argument("--session-id", required=True)
    supervise_status_p.add_argument("--json", action="store_true")
    supervise_list_p = supervise_sub.add_parser("list")
    supervise_list_p.add_argument("--run-id", required=True)
    supervise_list_p.add_argument("--state")
    supervise_list_p.add_argument("--json", action="store_true")

    serve = sub.add_parser(
        "serve",
        help=(
            "RFC 0012 V1: run the local HTTP / Unix-socket service. "
            "Localhost-only by default; non-loopback hosts refused at "
            "startup with exit 8."
        ),
    )
    serve.add_argument("--unix")
    serve.add_argument("--host", default=None)
    serve.add_argument("--port", type=int, default=None)
    serve.add_argument("--token", default=None)
    serve.add_argument("--allow-mutations", action="store_true")
    serve.add_argument("--idle-timeout-seconds", type=int, default=None)
    serve.add_argument("--web", action="store_true")
    serve.add_argument("--json", action="store_true")

    dashboard = sub.add_parser("dashboard")
    dash_target = dashboard.add_mutually_exclusive_group()
    dash_target.add_argument("--run-id")
    dash_target.add_argument("--all", action="store_true")
    dashboard.add_argument("--refresh", type=float, default=2.0)
    dashboard.add_argument("--once", action="store_true")
    dashboard.add_argument("--json", action="store_true")
    # RFC 0016 V1: graph panel.
    graph_group = dashboard.add_mutually_exclusive_group()
    graph_group.add_argument("--graph", dest="graph", action="store_true", default=None)
    graph_group.add_argument("--no-graph", dest="graph", action="store_false")
    dashboard.add_argument("--graph-only", action="store_true")
    dashboard.add_argument(
        "--graph-style",
        choices=["auto", "layered", "list", "fancy"],
        default="auto",
    )
    dashboard.add_argument("--graph-no-cycles", action="store_true")
    # RFC 0016 step 3: graph orientation.
    dashboard.add_argument(
        "--graph-orient",
        choices=["tb", "lr"],
        default="tb",
    )

    list_cmd = sub.add_parser("list")
    list_sub = list_cmd.add_subparsers(dest="list_command", required=True)

    list_runs_p = list_sub.add_parser("runs")
    list_runs_p.add_argument("--state")
    list_runs_p.add_argument("--limit", type=_positive_int, default=100)
    list_runs_p.add_argument("--json", action="store_true")

    list_sessions_p = list_sub.add_parser("sessions")
    list_sessions_p.add_argument("--run-id", required=True)
    list_sessions_p.add_argument("--state")
    list_sessions_p.add_argument("--role")
    list_sessions_p.add_argument("--lane")
    list_sessions_p.add_argument("--json", action="store_true")

    list_jobs_p = list_sub.add_parser("jobs")
    list_jobs_p.add_argument("--run-id", required=True)
    list_jobs_p.add_argument("--state")
    list_jobs_p.add_argument("--workflow-job-id")
    list_jobs_p.add_argument("--json", action="store_true")

    list_artifacts_p = list_sub.add_parser("artifacts")
    list_artifacts_p.add_argument("--run-id", required=True)
    list_artifacts_p.add_argument("--kind")
    list_artifacts_p.add_argument("--json", action="store_true")

    list_workflows_p = list_sub.add_parser("workflows")
    list_workflows_p.add_argument("--limit", type=_positive_int, default=100)
    list_workflows_p.add_argument("--json", action="store_true")

    # V1.41 (harness friction burn-down): byline helper. Operators
    # publishing-on-behalf need the exact expected_author_line without
    # spelunking `python3 -c "from striatum.artifacts import ..."`.
    byline = sub.add_parser(
        "byline",
        help=(
            "print the exact expected author line for a (session, job) "
            "pair; useful when publishing artifacts on behalf of a "
            "stalled session"
        ),
    )
    byline.add_argument("--session-id", required=True)
    byline.add_argument("--job-id", required=True)
    byline.add_argument("--json", action="store_true")

    # V1.41: with --session-id this remains the packet helper operators use
    # for publish-on-behalf flows. Without --session-id it is the
    # human-principal escalation inbox.
    inbox = sub.add_parser(
        "inbox",
        help=(
            "print the human-principal escalation inbox, or the current "
            "packet for a session when --session-id is supplied"
        ),
    )
    inbox.add_argument("--session-id")
    inbox.add_argument("--run-id")
    inbox.add_argument(
        "--state",
        choices=["open", "resolved", "canceled", "all"],
        default="open",
    )
    inbox.add_argument("--limit", type=_positive_int, default=100)
    inbox.add_argument("--json", action="store_true")

    return parser


def _positive_int(value: str) -> int:
    """Argparse type for ``--limit``: require a positive integer."""
    try:
        parsed = int(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"expected a positive integer, got {value!r}") from exc
    if parsed <= 0:
        raise argparse.ArgumentTypeError(f"expected a positive integer, got {value!r}")
    return parsed


def add_work_identity(parser: argparse.ArgumentParser) -> None:
    """Add standard work ownership arguments."""
    parser.add_argument("--session-id", required=True)
    parser.add_argument("--message-id", required=True)
    parser.add_argument("--lease-id", required=True)
    parser.add_argument("--json", action="store_true")
