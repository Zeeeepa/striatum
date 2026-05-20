# ruff: noqa
"""GH #11 regression: ``recovery auto-publish --dry-run`` must perform
no workflow-domain database writes, emit no workflow-domain events, take
no lease, and write no artifact.

The pre-fix path called ``expire_leases`` outside of any ``dry_run``
gate, mutating the leases row before the supposedly read-only preview
ran. This regression test snapshots every table that the live path
touches and asserts byte-for-byte equivalence after a dry-run sweep.
It intentionally does not prohibit separate metadata-only audit
telemetry; the invariant here is no domain-side effects.
"""

from __future__ import annotations
import pytest; pytest.skip("legacy sqlite eradicated", allow_module_level=True)

import json
from pathlib import Path
from typing import Any

from striatum.legacy_sqlite.db import connect
from test_cli_mvp import claim, prepare_started_run, register, write_artifact
from test_web_ui import _http_post_json, _spawn_service, _stop_service


DOMAIN_SIDE_EFFECT_TABLES = (
    "leases",
    "jobs",
    "queue_messages",
    "events",
    "artifacts",
    "verdicts",
    "runs",
)


def _snapshot_table(conn: Any, table: str) -> list[tuple[object, ...]]:
    rows = conn.execute(f"SELECT * FROM {table} ORDER BY 1").fetchall()
    return [tuple(row) for row in rows]


def _snapshot(repo: Path) -> dict[str, list[tuple[object, ...]]]:
    out: dict[str, list[tuple[object, ...]]] = {}
    with connect(repo) as conn:
        for table in DOMAIN_SIDE_EFFECT_TABLES:
            out[table] = _snapshot_table(conn, table)
    return out


def _seed_stale_publishable_artifact(repo: Path) -> tuple[str, str]:
    run_id = prepare_started_run(repo)
    session_id = register(repo, run_id, "author", "codex")
    packet = claim(repo, session_id)
    job_id = str(packet["job"]["job_id"])
    lease_id = str(packet["lease"]["lease_id"])
    expected = packet["expected_artifacts"][0]
    artifact_path = str(expected["path"])
    write_artifact(
        repo,
        artifact_path,
        text=f"{expected['author_line']}\n\npublishable stale artifact\n",
    )
    with connect(repo) as conn:
        conn.execute(
            "UPDATE leases SET state = 'expired' WHERE lease_id = ?",
            (lease_id,),
        )
        conn.execute(
            "UPDATE jobs SET state = 'stale_lease' WHERE job_id = ?",
            (job_id,),
        )
        conn.commit()
    return run_id, artifact_path


def test_dry_run_via_v1_invoke_writes_nothing(tmp_path: Path) -> None:
    """Dry-run through the same route the recovery-panel island uses."""
    run_id, artifact_path = _seed_stale_publishable_artifact(tmp_path)
    before = _snapshot(tmp_path)

    proc, port = _spawn_service(tmp_path, "--web", "--allow-mutations")
    try:
        status, body = _http_post_json(
            port,
            "/v1/invoke",
            {
                "argv": [
                    "recovery",
                    "auto-publish",
                    "--run-id",
                    run_id,
                    "--dry-run",
                ],
            },
        )
        envelope = json.loads(body)
        assert status == 200, envelope
        assert envelope["ok"] is True
        data = envelope["data"]
        assert data["dry_run"] is True
        assert data["published_count"] == 1
        assert data["published"][0]["would_publish"][0]["path"] == artifact_path
    finally:
        _stop_service(proc)

    after = _snapshot(tmp_path)
    for table in DOMAIN_SIDE_EFFECT_TABLES:
        assert before[table] == after[table], (
            f"dry-run mutated {table!r}: before={before[table]!r} "
            f"after={after[table]!r}"
        )


def test_dry_run_via_direct_call_writes_nothing_when_lease_wall_clock_stale(
    tmp_path: Path,
) -> None:
    """Even when ``expire_leases`` *would* have marked an active lease
    expired (wall-clock past ``expires_at``), the dry-run path must not
    mutate the leases row.
    """
    from striatum.cli.recovery import auto_publish_stale_artifacts

    run_id = prepare_started_run(tmp_path)
    session_id = register(tmp_path, run_id, "author", "codex")
    packet = claim(tmp_path, session_id)
    job_id = str(packet["job"]["job_id"])
    lease_id = str(packet["lease"]["lease_id"])
    expected = packet["expected_artifacts"][0]
    artifact_path = str(expected["path"])
    write_artifact(
        tmp_path,
        artifact_path,
        text=f"{expected['author_line']}\n\npublishable stale artifact\n",
    )
    # Move the lease into the past WITHOUT marking it expired.
    with connect(tmp_path) as conn:
        conn.execute(
            "UPDATE leases SET expires_at = '2000-01-01T00:00:00Z' "
            "WHERE lease_id = ?",
            (lease_id,),
        )
        conn.commit()

    before = _snapshot(tmp_path)
    with connect(tmp_path) as conn:
        result = auto_publish_stale_artifacts(
            conn,
            repo=tmp_path,
            run_id=run_id,
            dry_run=True,
        )

    assert result["dry_run"] is True
    assert result["published_count"] == 1
    # ``would_expire`` flags the wall-clock-stale-but-active lease, so
    # operators can tell the UI why the row showed up without us having
    # touched the row.
    preview = result["published"][0]
    assert preview["would_publish"][0]["path"] == artifact_path
    assert preview["would_expire"] is True
    assert str(preview["session_id"]) == session_id
    _ = job_id  # silence unused warning; bound for diagnostic context

    after = _snapshot(tmp_path)
    for table in DOMAIN_SIDE_EFFECT_TABLES:
        assert before[table] == after[table], (
            f"dry-run mutated {table!r}: before={before[table]!r} "
            f"after={after[table]!r}"
        )


def test_live_run_still_publishes_and_writes(tmp_path: Path) -> None:
    """Sanity: removing the dry-run gate doesn't break the live path."""
    from striatum.cli.recovery import auto_publish_stale_artifacts

    run_id, _artifact_path = _seed_stale_publishable_artifact(tmp_path)
    with connect(tmp_path) as conn:
        result = auto_publish_stale_artifacts(
            conn,
            repo=tmp_path,
            run_id=run_id,
            dry_run=False,
        )

    assert result["dry_run"] is False
    assert result["published_count"] == 1
