from __future__ import annotations

import json
import os
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT / "src") not in sys.path:
    sys.path.insert(0, str(ROOT / "src"))

from striatum import __version__ as STRIATUM_VERSION  # noqa: E402
from striatum.skills import install as install_skills  # noqa: E402
from striatum.skills import (  # noqa: E402
    bundled_template_sha256,
    load_manifest,
    manifest_path_for,
)


def _claude_paths(target: Path, namespace: str = "striatum-") -> list[Path]:
    return [
        target / ".claude" / "skills" / f"{namespace}{skill}" / "SKILL.md"
        for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover")
    ]


def test_install_claude_code_writes_five_skills_and_manifest(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="claude_code")
    assert result["profile"] == "claude_code"
    assert {f["status"] for f in result["files"]} == {"written"}
    for path in _claude_paths(tmp_path):
        assert path.exists(), path
    manifest_path = Path(result["manifest_path"])
    manifest = load_manifest(manifest_path)
    assert manifest is not None
    assert manifest["schema_version"] == "striatum.skills.manifest.v1"
    assert manifest["striatum_version"] == STRIATUM_VERSION
    assert {entry["path"] for entry in manifest["files"]} == {
        f".claude/skills/striatum-{skill}/SKILL.md"
        for skill in ("workflow", "scaffold", "claim-loop", "supervise", "recover")
    }


def test_install_idempotent_byte_identical(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    snapshot = {
        path: path.read_bytes()
        for path in _claude_paths(tmp_path)
    }
    second = install_skills(target=tmp_path, profile="claude_code")
    assert {f["status"] for f in second["files"]} == {"skipped_unchanged"}
    for path, data in snapshot.items():
        assert path.read_bytes() == data


def test_install_refuses_modified_file_without_force(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    target_skill = tmp_path / ".claude/skills/striatum-claim-loop/SKILL.md"
    edited = target_skill.read_text(encoding="utf-8") + "\n# operator note\n"
    target_skill.write_text(edited, encoding="utf-8")
    result = install_skills(target=tmp_path, profile="claude_code")
    statuses = {f["path"]: f["status"] for f in result["files"]}
    assert (
        statuses[".claude/skills/striatum-claim-loop/SKILL.md"]
        == "refused_modified"
    )
    # Operator edit preserved verbatim.
    assert target_skill.read_text(encoding="utf-8") == edited


def test_install_force_overwrites_modified_file(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    target_skill = tmp_path / ".claude/skills/striatum-claim-loop/SKILL.md"
    target_skill.write_text("hand-edit\n", encoding="utf-8")
    result = install_skills(target=tmp_path, profile="claude_code", force=True)
    statuses = {f["path"]: f["status"] for f in result["files"]}
    assert (
        statuses[".claude/skills/striatum-claim-loop/SKILL.md"] == "written"
    )
    assert "hand-edit" not in target_skill.read_text(encoding="utf-8")


def test_install_dry_run_writes_nothing(tmp_path: Path) -> None:
    fresh = tmp_path / "first"
    fresh.mkdir()
    result = install_skills(target=fresh, profile="claude_code", dry_run=True)
    assert {f["status"] for f in result["files"]} == {"dry_run"}
    for path in _claude_paths(fresh):
        assert not path.exists()
    # Manifest is also not written under dry-run.
    assert not Path(result["manifest_path"]).exists()


def test_install_generic_profile_single_guide(tmp_path: Path) -> None:
    result = install_skills(target=tmp_path, profile="generic")
    paths = [f["path"] for f in result["files"]]
    assert paths == ["striatum-STRIATUM_AGENT_GUIDE.md"]
    guide = tmp_path / "striatum-STRIATUM_AGENT_GUIDE.md"
    assert guide.exists()
    manifest = load_manifest(Path(result["manifest_path"]))
    assert manifest is not None
    assert manifest["profile"] == "generic"


def test_install_namespace_prefix_respected(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code", namespace="foo-")
    expected = tmp_path / ".claude/skills/foo-workflow/SKILL.md"
    assert expected.exists(), list(tmp_path.glob(".claude/skills/*"))


def test_install_scope_user_writes_to_home(tmp_path: Path) -> None:
    fake_home = tmp_path / "home"
    fake_home.mkdir()
    result = install_skills(
        target=tmp_path, profile="claude_code", scope="user", home=fake_home
    )
    assert result["scope"] == "user"
    expected = fake_home / ".claude/skills/striatum-workflow/SKILL.md"
    assert expected.exists()


def test_init_with_skills_installs_after_init(tmp_path: Path) -> None:
    import subprocess

    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    proc = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "init",
            "--with-skills",
            "claude_code",
            "--json",
        ],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )
    assert proc.returncode == 0, proc.stderr
    payload = json.loads(proc.stdout)
    assert payload["ok"] is True
    assert (tmp_path / ".striatum" / "state.sqlite3").exists()
    assert (tmp_path / ".claude/skills/striatum-workflow/SKILL.md").exists()


def test_doctor_reports_skills_missing(tmp_path: Path) -> None:
    from striatum.cli.introspect import doctor
    from striatum.db import connect, init_repo

    init_repo(tmp_path)
    install_skills(target=tmp_path, profile="claude_code")
    # Remove one file so the manifest no longer matches disk.
    target = tmp_path / ".claude/skills/striatum-recover/SKILL.md"
    target.unlink()
    with connect(tmp_path) as conn:
        result = doctor(conn, repo=tmp_path, run_id=None, verbose=True)
    checks = {row["check"] for row in result["problem_records"]}
    assert "skills_missing" in checks


def test_doctor_reports_skills_outdated_on_version(tmp_path: Path) -> None:
    from striatum.cli.introspect import doctor
    from striatum.db import connect, init_repo

    init_repo(tmp_path)
    install_skills(target=tmp_path, profile="claude_code")
    manifest_path = manifest_path_for(target=tmp_path, profile="claude_code")
    payload = load_manifest(manifest_path)
    assert payload is not None
    payload["striatum_version"] = "0.0.0-pretend-stale"
    manifest_path.write_text(
        json.dumps(payload, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    with connect(tmp_path) as conn:
        result = doctor(conn, repo=tmp_path, run_id=None, verbose=True)
    checks = {row["check"] for row in result["problem_records"]}
    assert "skills_outdated" in checks


def test_doctor_reports_skills_outdated_on_template_sha(tmp_path: Path) -> None:
    from striatum.cli.introspect import doctor
    from striatum.db import connect, init_repo

    init_repo(tmp_path)
    install_skills(target=tmp_path, profile="claude_code")
    manifest_path = manifest_path_for(target=tmp_path, profile="claude_code")
    payload = load_manifest(manifest_path)
    assert payload is not None
    # Pretend a template's bundled hash drifted by recording a fake
    # sha for one entry. Doctor should flag the divergence.
    payload["files"][0]["template_sha256"] = "00" * 32
    manifest_path.write_text(
        json.dumps(payload, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )
    with connect(tmp_path) as conn:
        result = doctor(conn, repo=tmp_path, run_id=None, verbose=True)
    checks = {row["check"] for row in result["problem_records"]}
    assert "skills_outdated" in checks


def test_no_external_url_invariant(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    install_skills(target=tmp_path, profile="generic")
    rendered = []
    for path in _claude_paths(tmp_path):
        rendered.append(path.read_text(encoding="utf-8"))
    rendered.append(
        (tmp_path / "striatum-STRIATUM_AGENT_GUIDE.md").read_text(encoding="utf-8")
    )
    for body in rendered:
        assert "http://" not in body
        assert "https://" not in body


def test_manifest_excludes_itself(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    manifest_path = manifest_path_for(target=tmp_path, profile="claude_code")
    manifest = load_manifest(manifest_path)
    assert manifest is not None
    paths = {entry["path"] for entry in manifest["files"]}
    assert ".manifest.json" not in "".join(paths) or all(
        ".manifest.json" not in p for p in paths
    )


def test_bundled_template_sha_matches_manifest_after_install(tmp_path: Path) -> None:
    install_skills(target=tmp_path, profile="claude_code")
    manifest = load_manifest(
        manifest_path_for(target=tmp_path, profile="claude_code")
    )
    assert manifest is not None
    for entry in manifest["files"]:
        assert (
            entry["template_sha256"]
            == bundled_template_sha256(entry["template"])
        )


def test_install_unknown_profile_raises(tmp_path: Path) -> None:
    from striatum.errors import InvalidTransitionError

    with pytest.raises(InvalidTransitionError):
        install_skills(target=tmp_path, profile="codex")
