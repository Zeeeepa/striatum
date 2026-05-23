from __future__ import annotations

from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SOURCE_ROOTS = (ROOT / "src" / "striatum", ROOT / "go" / "pkg")
TMUX_TRANSCRIPT_COMMANDS = ("capture-pane", "pipe-pane", "save-buffer", "show-buffer")


def _source_files() -> list[Path]:
    paths: list[Path] = []
    for root in SOURCE_ROOTS:
        paths.extend(sorted(root.rglob("*.go")))
        paths.extend(sorted(root.rglob("*.py")))
    return paths


def test_production_source_does_not_capture_tmux_pane_text() -> None:
    offenders: list[str] = []
    for path in _source_files():
        text = path.read_text(encoding="utf-8")
        for token in TMUX_TRANSCRIPT_COMMANDS:
            if token in text:
                offenders.append(f"{path.relative_to(ROOT)} contains {token}")

    assert offenders == []


def test_tmux_metadata_probe_is_limited_to_window_and_pane_identity() -> None:
    pty_go = (ROOT / "go" / "pkg" / "supervisor" / "pty.go").read_text(encoding="utf-8")

    assert '"display-message"' in pty_go
    assert '"#{window_id} #{pane_id}"' in pty_go
    for token in TMUX_TRANSCRIPT_COMMANDS:
        assert token not in pty_go


def test_transcript_artifacts_remain_refused_by_publish_and_recovery_paths() -> None:
    expectations = {
        "src/striatum/daemon_pg/handlers/workflow_loop/artifact_publish.py": [
            'if kind == "transcript":',
            "transcript artifacts are not allowed by default",
        ],
        "src/striatum/daemon_pg/handlers/recovery_evidence/auto_finalize.py": [
            'if kind == "transcript":',
            "transcript artifacts are not auto-finalized",
        ],
        "go/pkg/mutations/artifact.go": [
            'if kind == "transcript" {',
            "transcript artifacts are not allowed by default",
        ],
        "go/pkg/mutations/recovery_auto_finalize.go": [
            'if kind == "transcript" {',
            "transcript artifacts are not auto-finalized",
        ],
        "go/pkg/mutations/recovery.go": [
            'if kind == "transcript" {',
            "transcript artifacts are not allowed by default",
        ],
    }
    for relative, snippets in expectations.items():
        text = (ROOT / relative).read_text(encoding="utf-8")
        for snippet in snippets:
            assert snippet in text, f"{relative} missing {snippet!r}"
