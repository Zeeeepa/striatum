from __future__ import annotations

import hashlib
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import cast

import pytest

from striatum.corpus.manifest import write_manifest
from striatum.corpus.types import JSONL_FILES, SUB_KINDS, CorpusProvenance, CorpusRow
from striatum.corpus.verify import verify_corpus_bundle
from striatum.corpus.writer import write_jsonl_bundle
from striatum.errors import StriatumError


ROOT = Path(__file__).resolve().parents[1]


def _row(sub_kind: str = "rfc", external_id: str = "rfc:0001#demo") -> CorpusRow:
    return CorpusRow(
        external_id=external_id,
        sub_kind=sub_kind,
        content="Demo\n\nBody",
        provenance=CorpusProvenance(path="docs/rfcs/0001-demo.md", sha256="abc", commit="def"),
        observed_at="2026-05-17T00:00:00Z",
    )


def _bundle(root: Path) -> Path:
    bundle = root / "bundle"
    files = write_jsonl_bundle(bundle, [_row()])
    row_counts = {kind: 0 for kind in SUB_KINDS}
    row_counts["rfc"] = 1
    write_manifest(
        bundle,
        {
            "schema_version": "striatum.corpus_export.v1",
            "row_counts": row_counts,
            "files": files,
        },
    )
    return bundle


def test_verify_corpus_bundle_accepts_v1_without_contract_version(tmp_path: Path) -> None:
    result = verify_corpus_bundle(_bundle(tmp_path))

    assert result["status"] == "verified"
    assert result["corpus_contract_version"] == 1
    row_counts = cast(dict[str, int], result["row_counts"])
    assert row_counts["rfc"] == 1
    assert len(str(result["bundle_sha256"])) == 64


def test_verify_corpus_bundle_rejects_tampered_jsonl(tmp_path: Path) -> None:
    bundle = _bundle(tmp_path)
    (bundle / "rfcs.jsonl").write_text("tampered\n", encoding="utf-8")

    with pytest.raises(StriatumError, match="byte count mismatch|hash mismatch"):
        verify_corpus_bundle(bundle)


def test_verify_corpus_bundle_rejects_manifest_row_count_mismatch(tmp_path: Path) -> None:
    bundle = _bundle(tmp_path)
    manifest_path = bundle / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest["row_counts"]["rfc"] = 2
    write_manifest(bundle, manifest)

    with pytest.raises(StriatumError, match="row count mismatch"):
        verify_corpus_bundle(bundle)


def test_verify_corpus_bundle_rejects_wrong_file_sub_kind(tmp_path: Path) -> None:
    bundle = _bundle(tmp_path)
    filename = JSONL_FILES["rfc"]
    row = _row(sub_kind="commit", external_id="commit:def").to_json()
    body = (json.dumps(row, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")
    (bundle / filename).write_bytes(body)
    manifest = json.loads((bundle / "manifest.json").read_text(encoding="utf-8"))
    manifest["files"][filename]["sha256"] = hashlib.sha256(body).hexdigest()
    manifest["files"][filename]["bytes"] = len(body)
    manifest["files"][filename]["rows"] = 1
    manifest["row_counts"]["rfc"] = 0
    manifest["row_counts"]["commit"] = 1
    write_manifest(bundle, manifest)

    with pytest.raises(StriatumError, match="wrong file"):
        verify_corpus_bundle(bundle)


def test_corpus_verify_cli_is_local_read_only(tmp_path: Path) -> None:
    bundle = _bundle(tmp_path)
    env = os.environ.copy()
    env["PYTHONPATH"] = str(ROOT / "src")
    result = subprocess.run(
        [
            sys.executable,
            "-m",
            "striatum.cli",
            "--repo",
            str(tmp_path),
            "corpus",
            "verify",
            "--bundle",
            str(bundle),
            "--json",
        ],
        cwd=tmp_path,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    payload = json.loads(result.stdout)
    assert payload["ok"] is True
    assert payload["data"]["status"] == "verified"
