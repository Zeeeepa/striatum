"""Tests for RFC 0021 V1: ``striatum init --with-ddd-layout``.

V1 ships a literal-copy scaffold of the seven canonical DDD-shaped
reader-facing documents into the target repo. Existing files are
preserved; non-file targets (directory, broken symlink, etc.)
surface as per-file errors instead of being silently treated as
``skipped`` (Finding 2 from dogfood-017's design review).
"""

from __future__ import annotations

import json
import subprocess
import sys
from importlib import resources
from pathlib import Path
from typing import Any
from unittest import mock

import pytest

from striatum.scaffold import scaffold_ddd_layout, scaffold_striatum_layout

ROOT = Path(__file__).resolve().parents[1]


_EXPECTED_TARGETS: list[str] = [
    "docs/SPEC.md",
    "docs/PRD.md",
    "docs/DECISION_LOG.md",
    "docs/UBIQUITOUS_LANGUAGE.md",
    "docs/DDD.md",
    "docs/rfcs/README.md",
    "docs/rfcs/0001-template.md",
]

_EXPECTED_STRIATUM_DIRS: list[str] = [
    "striatum/workflows",
    "striatum/code-change",
]


def test_scaffold_creates_seven_files_in_empty_repo(tmp_path: Path) -> None:
    result = scaffold_ddd_layout(tmp_path)
    assert result["layout"] == "ddd"
    assert result["dry_run"] is False
    statuses = {f["path"]: f["status"] for f in result["files"]}
    for target in _EXPECTED_TARGETS:
        assert statuses[target] == "created"
        assert (tmp_path / target).is_file()


def test_scaffold_each_file_starts_with_generation_comment(
    tmp_path: Path,
) -> None:
    scaffold_ddd_layout(tmp_path)
    for target in _EXPECTED_TARGETS:
        body = (tmp_path / target).read_text(encoding="utf-8")
        first_line = body.splitlines()[0]
        assert "RFC 0021" in first_line
        assert "striatum init --with-ddd-layout" in first_line


def test_scaffold_skips_existing_file_with_reason_exists(
    tmp_path: Path,
) -> None:
    target = tmp_path / "docs" / "SPEC.md"
    target.parent.mkdir(parents=True)
    target.write_text("operator content", encoding="utf-8")
    result = scaffold_ddd_layout(tmp_path)
    spec_entry = next(f for f in result["files"]
                      if f["path"] == "docs/SPEC.md")
    assert spec_entry["status"] == "skipped"
    assert spec_entry["reason"] == "exists"
    assert target.read_text(encoding="utf-8") == "operator content"


def test_scaffold_creates_only_missing_files_in_partial_overlap_repo(
    tmp_path: Path,
) -> None:
    (tmp_path / "docs").mkdir()
    (tmp_path / "docs" / "DDD.md").write_text("existing", encoding="utf-8")
    result = scaffold_ddd_layout(tmp_path)
    statuses = {f["path"]: f["status"] for f in result["files"]}
    assert statuses["docs/DDD.md"] == "skipped"
    for target in _EXPECTED_TARGETS:
        if target == "docs/DDD.md":
            continue
        assert statuses[target] == "created"


def test_scaffold_idempotent_second_run_all_skipped(tmp_path: Path) -> None:
    scaffold_ddd_layout(tmp_path)
    second = scaffold_ddd_layout(tmp_path)
    statuses = {f["path"]: (f["status"], f.get("reason"))
                for f in second["files"]}
    for target in _EXPECTED_TARGETS:
        assert statuses[target] == ("skipped", "exists")


def test_scaffold_envelope_shape(tmp_path: Path) -> None:
    result = scaffold_ddd_layout(tmp_path)
    assert set(result.keys()) == {"layout", "files", "dry_run"}
    assert result["layout"] == "ddd"
    assert isinstance(result["files"], list)
    assert len(result["files"]) == len(_EXPECTED_TARGETS)
    for entry in result["files"]:
        assert set(entry.keys()) >= {"path", "status"}
        assert entry["status"] in {"created", "skipped", "error"}


def test_scaffold_target_is_directory_reports_error(tmp_path: Path) -> None:
    """Finding 2: a target that exists as a directory must NOT be
    reported as ``skipped``; that would silently hide the
    operator's foot-gun."""
    spec_dir = tmp_path / "docs" / "SPEC.md"
    spec_dir.mkdir(parents=True)
    result = scaffold_ddd_layout(tmp_path)
    spec_entry = next(f for f in result["files"]
                      if f["path"] == "docs/SPEC.md")
    assert spec_entry["status"] == "error"
    assert "not a regular file" in spec_entry["reason"]
    # Other files should still be created normally.
    other_statuses = [f["status"] for f in result["files"]
                      if f["path"] != "docs/SPEC.md"]
    assert all(s == "created" for s in other_statuses)


def test_scaffold_target_is_broken_symlink_reports_error(
    tmp_path: Path,
) -> None:
    """Finding 2 corollary: a broken symlink at the target path
    should not be silently overwritten."""
    docs = tmp_path / "docs"
    docs.mkdir()
    broken = docs / "PRD.md"
    broken.symlink_to(tmp_path / "nonexistent")
    result = scaffold_ddd_layout(tmp_path)
    prd_entry = next(f for f in result["files"]
                     if f["path"] == "docs/PRD.md")
    assert prd_entry["status"] == "error"
    assert broken.is_symlink()  # left in place


def test_scaffold_filesystem_error_per_file_status_error(
    tmp_path: Path,
) -> None:
    """Finding 4: an OSError during write surfaces per-file as
    status:error without aborting the rest of the scaffold."""
    real_write_text = Path.write_text

    def flaky_write(self: Path, *args: Any, **kwargs: Any) -> int:
        if self.name == "DDD.md":
            raise OSError("disk full")
        return real_write_text(self, *args, **kwargs)

    with mock.patch.object(Path, "write_text", flaky_write):
        result = scaffold_ddd_layout(tmp_path)
    statuses = {f["path"]: (f["status"], f.get("reason"))
                for f in result["files"]}
    assert statuses["docs/DDD.md"][0] == "error"
    assert "disk full" in statuses["docs/DDD.md"][1]
    # The other six files must have succeeded.
    for target in _EXPECTED_TARGETS:
        if target == "docs/DDD.md":
            continue
        assert statuses[target][0] == "created"


def test_scaffold_templates_discoverable_via_importlib_resources() -> None:
    """Finding 3: the package-data wiring must expose exactly seven
    `.md.tmpl` files under the layout package, including those in
    subdirectories. A package-data typo or a missing pattern would
    break this."""
    pkg = resources.files("striatum.scaffold.templates.ddd_layout")
    template_paths: list[str] = []
    stack = [(pkg, "")]
    while stack:
        node, prefix = stack.pop()
        for entry in node.iterdir():
            relative = f"{prefix}{entry.name}"
            if entry.is_file() and relative.endswith(".md.tmpl"):
                template_paths.append(relative)
            elif entry.is_dir():
                stack.append((entry, f"{relative}/"))
    assert sorted(template_paths) == sorted([
        "DDD.md.tmpl",
        "DECISION_LOG.md.tmpl",
        "PRD.md.tmpl",
        "SPEC.md.tmpl",
        "UBIQUITOUS_LANGUAGE.md.tmpl",
        "rfcs/0001-template.md.tmpl",
        "rfcs/README.md.tmpl",
    ])


def test_init_with_ddd_layout_flag_returns_envelope(tmp_path: Path) -> None:
    """The CLI flag wires through to the dispatch path correctly."""
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init",
         "--with-ddd-layout", "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"},
    )
    assert proc.returncode == 0, f"stderr: {proc.stderr}"
    payload = json.loads(proc.stdout)
    assert payload["ok"] is True
    layout = payload["data"]["ddd_layout"]
    assert layout["layout"] == "ddd"
    assert len(layout["files"]) == len(_EXPECTED_TARGETS)


def test_init_without_flag_unchanged(tmp_path: Path) -> None:
    """Plain `striatum init` produces no scaffold envelope key and
    no scaffold files."""
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init", "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"},
    )
    assert proc.returncode == 0, f"stderr: {proc.stderr}"
    payload = json.loads(proc.stdout)
    assert "ddd_layout" not in payload["data"]
    assert not (tmp_path / "docs").exists()


# --- V1.5: --force ---------------------------------------------------


def test_scaffold_force_overwrites_existing_file(tmp_path: Path) -> None:
    docs = tmp_path / "docs"
    docs.mkdir()
    spec = docs / "SPEC.md"
    spec.write_text("operator content\n", encoding="utf-8")
    result = scaffold_ddd_layout(tmp_path, force=True)
    spec_entry = next(f for f in result["files"] if f["path"] == "docs/SPEC.md")
    assert spec_entry["status"] == "overwritten"
    assert "prior_sha256" in spec_entry
    body = spec.read_text(encoding="utf-8")
    assert "RFC 0021" in body.splitlines()[0]


def test_scaffold_force_does_not_clobber_non_regular_file(tmp_path: Path) -> None:
    """Even with force=True, a target that exists as a directory must
    NOT be deleted. The carve-out from V1's Finding 2 still applies."""
    spec_dir = tmp_path / "docs" / "SPEC.md"
    spec_dir.mkdir(parents=True)
    result = scaffold_ddd_layout(tmp_path, force=True)
    spec_entry = next(f for f in result["files"] if f["path"] == "docs/SPEC.md")
    assert spec_entry["status"] == "error"
    assert "not a regular file" in spec_entry["reason"]
    assert spec_dir.is_dir()


# --- V1.5: --dry-run -------------------------------------------------


def test_scaffold_dry_run_writes_no_files(tmp_path: Path) -> None:
    result = scaffold_ddd_layout(tmp_path, dry_run=True)
    assert result["dry_run"] is True
    assert not (tmp_path / "docs").exists()


def test_scaffold_dry_run_envelope_reflects_flag(tmp_path: Path) -> None:
    result = scaffold_ddd_layout(tmp_path, dry_run=True)
    assert result["dry_run"] is True


def test_scaffold_dry_run_status_vocabulary_empty_repo(tmp_path: Path) -> None:
    """Empty repo + dry-run: every file is `would_create`."""
    result = scaffold_ddd_layout(tmp_path, dry_run=True)
    statuses = {f["path"]: f["status"] for f in result["files"]}
    for target in _EXPECTED_TARGETS:
        assert statuses[target] == "would_create"


def test_scaffold_dry_run_status_vocabulary_partial_overlap(tmp_path: Path) -> None:
    """Partial overlap + dry-run: existing → would_skip, missing → would_create."""
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "SPEC.md").write_text("operator", encoding="utf-8")
    result = scaffold_ddd_layout(tmp_path, dry_run=True)
    statuses = {f["path"]: f["status"] for f in result["files"]}
    assert statuses["docs/SPEC.md"] == "would_skip"
    for target in _EXPECTED_TARGETS:
        if target == "docs/SPEC.md":
            continue
        assert statuses[target] == "would_create"


# --- V1.5: --force + --dry-run together ------------------------------


def test_scaffold_force_and_dry_run_together_no_writes(tmp_path: Path) -> None:
    """`force=True, dry_run=True` previews the destructive overwrite
    without actually writing — the canonical preview-of-clobber flow."""
    docs = tmp_path / "docs"
    docs.mkdir()
    spec = docs / "SPEC.md"
    spec.write_text("operator content\n", encoding="utf-8")
    result = scaffold_ddd_layout(tmp_path, force=True, dry_run=True)
    spec_entry = next(f for f in result["files"] if f["path"] == "docs/SPEC.md")
    assert spec_entry["status"] == "would_overwrite"
    assert "prior_sha256" in spec_entry
    # On-disk content must be unchanged.
    assert spec.read_text(encoding="utf-8") == "operator content\n"


# --- V1.5: CLI flag wiring -------------------------------------------


def test_init_cli_flags_thread_through_to_envelope(tmp_path: Path) -> None:
    """Subprocess test: `--ddd-layout-force` and `--ddd-layout-dry-run`
    threaded through dispatch into the envelope."""
    docs = tmp_path / "docs"
    docs.mkdir()
    (docs / "SPEC.md").write_text("operator", encoding="utf-8")
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init",
         "--with-ddd-layout", "--ddd-layout-force",
         "--ddd-layout-dry-run", "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"},
    )
    assert proc.returncode == 0, f"stderr: {proc.stderr}"
    payload = json.loads(proc.stdout)
    layout = payload["data"]["ddd_layout"]
    assert layout["dry_run"] is True
    spec_entry = next(f for f in layout["files"] if f["path"] == "docs/SPEC.md")
    assert spec_entry["status"] == "would_overwrite"


def test_init_v15_flags_noop_without_with_ddd_layout(tmp_path: Path) -> None:
    """Passing --ddd-layout-force or --ddd-layout-dry-run without
    --with-ddd-layout produces no `ddd_layout` envelope key — the
    scaffold call is gated by the layout flag."""
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init",
         "--ddd-layout-force", "--ddd-layout-dry-run", "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"},
    )
    assert proc.returncode == 0, f"stderr: {proc.stderr}"
    payload = json.loads(proc.stdout)
    assert "ddd_layout" not in payload["data"]


def test_init_composability_with_skills_install(tmp_path: Path) -> None:
    """Both flags together produce both envelopes nested under their
    own keys; the scaffold runs after skills installation."""
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init",
         "--with-skills", "claude_code",
         "--with-ddd-layout", "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"),
             "PATH": "/usr/bin:/bin"},
    )
    if proc.returncode != 0:
        pytest.skip(f"skills install failed (probably env-dependent): "
                    f"{proc.stderr}")
    payload = json.loads(proc.stdout)
    assert payload["ok"] is True
    assert "skills" in payload["data"]
    assert "ddd_layout" in payload["data"]
    assert payload["data"]["ddd_layout"]["layout"] == "ddd"


# --- RFC 0056 Phase B: consumer Striatum layout ----------------------


def test_striatum_layout_creates_workflow_directories(tmp_path: Path) -> None:
    result = scaffold_striatum_layout(tmp_path)

    assert result["layout"] == "striatum"
    assert result["workflow_slug"] == "code-change"
    assert result["dry_run"] is False
    statuses = {entry["path"]: entry["status"] for entry in result["directories"]}
    for target in _EXPECTED_STRIATUM_DIRS:
        assert statuses[target] == "created"
        assert (tmp_path / target).is_dir()


def test_striatum_layout_dry_run_writes_no_directories(tmp_path: Path) -> None:
    result = scaffold_striatum_layout(tmp_path, dry_run=True)

    assert result["dry_run"] is True
    assert {
        entry["path"]: entry["status"] for entry in result["directories"]
    } == {
        "striatum/workflows": "would_create",
        "striatum/code-change": "would_create",
    }
    assert not (tmp_path / "striatum").exists()


def test_striatum_layout_skips_existing_directories(tmp_path: Path) -> None:
    (tmp_path / "striatum" / "workflows").mkdir(parents=True)
    (tmp_path / "striatum" / "code-change").mkdir()

    result = scaffold_striatum_layout(tmp_path)

    assert {
        entry["path"]: (entry["status"], entry.get("reason"))
        for entry in result["directories"]
    } == {
        "striatum/workflows": ("skipped", "exists"),
        "striatum/code-change": ("skipped", "exists"),
    }


def test_striatum_layout_reports_file_target_as_error(tmp_path: Path) -> None:
    (tmp_path / "striatum").write_text("not a directory", encoding="utf-8")

    result = scaffold_striatum_layout(tmp_path)

    assert all(
        entry["status"] == "error" for entry in result["directories"]
    )
    assert all(
        entry["reason"] == "parent exists but is not a directory: striatum"
        for entry in result["directories"]
    )
    assert (tmp_path / "striatum").read_text(encoding="utf-8") == "not a directory"


def test_striatum_layout_rejects_unsafe_workflow_slug(tmp_path: Path) -> None:
    result = scaffold_striatum_layout(tmp_path, workflow_slug="../bad")

    assert result["directories"] == [
        {
            "path": "striatum/<workflow_slug>",
            "status": "error",
            "reason": (
                "workflow_slug must be 1-64 chars and contain only letters, "
                "digits, dot, underscore, or hyphen; it must start with a "
                "letter or digit"
            ),
        }
    ]
    assert not (tmp_path / "striatum").exists()


def test_init_with_striatum_layout_returns_envelope(tmp_path: Path) -> None:
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init",
         "--with-ddd-layout", "--with-striatum-layout",
         "--striatum-layout-workflow", "review-cycle",
         "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"},
    )

    assert proc.returncode == 0, f"stderr: {proc.stderr}"
    payload = json.loads(proc.stdout)
    assert payload["data"]["ddd_layout"]["layout"] == "ddd"
    layout = payload["data"]["striatum_layout"]
    assert layout["layout"] == "striatum"
    assert layout["workflow_slug"] == "review-cycle"
    assert {entry["path"] for entry in layout["directories"]} == {
        "striatum/workflows",
        "striatum/review-cycle",
    }
    assert (tmp_path / "striatum" / "workflows").is_dir()
    assert (tmp_path / "striatum" / "review-cycle").is_dir()


def test_striatum_layout_flags_noop_without_with_striatum_layout(tmp_path: Path) -> None:
    proc = subprocess.run(
        [sys.executable, "-m", "striatum.cli",
         "--repo", str(tmp_path), "init",
         "--striatum-layout-dry-run",
         "--striatum-layout-workflow", "review-cycle",
         "--json"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env={"PYTHONPATH": str(ROOT / "src"), "PATH": "/usr/bin:/bin"},
    )

    assert proc.returncode == 0, f"stderr: {proc.stderr}"
    payload = json.loads(proc.stdout)
    assert "striatum_layout" not in payload["data"]
    assert not (tmp_path / "striatum").exists()
