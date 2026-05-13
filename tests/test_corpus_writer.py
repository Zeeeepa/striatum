from __future__ import annotations

import json
from pathlib import Path

import pytest

from striatum.corpus.types import JSONL_FILES, SUB_KINDS, CorpusProvenance, CorpusRow
from striatum.corpus.writer import verify_jsonl_files, write_jsonl_bundle
from striatum.errors import StriatumError


def _row(sub_kind: str, external_id: str) -> CorpusRow:
    return CorpusRow(
        external_id=external_id,
        sub_kind=sub_kind,
        content=f"content {external_id}",
        provenance=CorpusProvenance(path="docs/source.md", sha256="abc", commit="def"),
        observed_at="2026-05-13T00:00:00Z",
    )


def test_writer_emits_all_files_with_newline_and_zero_rows(tmp_path: Path) -> None:
    files = write_jsonl_bundle(tmp_path, [_row("rfc", "rfc:0001#b")])

    assert list(files) == [JSONL_FILES[kind] for kind in SUB_KINDS]
    for filename in files:
        data = (tmp_path / filename).read_bytes()
        assert data.endswith(b"\n")
    assert files["decision_log_rows.jsonl"]["rows"] == 0
    assert (tmp_path / "decision_log_rows.jsonl").read_text(encoding="utf-8") == "\n"


def test_writer_sorts_rows_and_keeps_locked_key_order(tmp_path: Path) -> None:
    files = write_jsonl_bundle(
        tmp_path,
        [_row("rfc", "rfc:0001#z"), _row("rfc", "rfc:0001#a")],
    )
    assert files["rfcs.jsonl"]["rows"] == 2

    lines = (tmp_path / "rfcs.jsonl").read_text(encoding="utf-8").splitlines()
    first = json.loads(lines[0])
    assert first["external_id"] == "rfc:0001#a"
    assert list(first) == [
        "source_kind",
        "external_id",
        "sub_kind",
        "content",
        "provenance",
        "observed_at",
    ]
    assert verify_jsonl_files(tmp_path, files)["rfc"] == 2


def test_writer_rejects_duplicate_sub_kind_external_id(tmp_path: Path) -> None:
    with pytest.raises(StriatumError, match="duplicate corpus row"):
        write_jsonl_bundle(tmp_path, [_row("rfc", "rfc:0001#a"), _row("rfc", "rfc:0001#a")])


def test_writer_verification_rejects_tampered_file(tmp_path: Path) -> None:
    files = write_jsonl_bundle(tmp_path, [_row("rfc", "rfc:0001#a")])
    (tmp_path / "rfcs.jsonl").write_text("tampered\n", encoding="utf-8")

    with pytest.raises(StriatumError, match="hash mismatch"):
        verify_jsonl_files(tmp_path, files)
