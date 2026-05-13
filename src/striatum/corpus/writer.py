"""Deterministic JSONL corpus bundle writer."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path

from striatum.corpus.types import JSONL_FILES, SOURCE_KIND, SUB_KINDS, CorpusRow
from striatum.errors import StriatumError


def write_jsonl_bundle(out: Path, rows: list[CorpusRow]) -> dict[str, dict[str, int | str]]:
    """Write fixed-order JSONL files and return file metadata."""
    out.mkdir(parents=True, exist_ok=True)
    by_kind: dict[str, list[CorpusRow]] = {kind: [] for kind in SUB_KINDS}
    seen: set[tuple[str, str]] = set()
    for row in rows:
        if row.sub_kind not in by_kind:
            raise StriatumError(f"unknown corpus sub_kind: {row.sub_kind}", exit_code=6)
        key = (row.sub_kind, row.external_id)
        if key in seen:
            raise StriatumError(f"duplicate corpus row: {row.sub_kind} {row.external_id}", exit_code=6)
        seen.add(key)
        by_kind[row.sub_kind].append(row)

    files: dict[str, dict[str, int | str]] = {}
    for sub_kind in SUB_KINDS:
        filename = JSONL_FILES[sub_kind]
        ordered = sorted(
            by_kind[sub_kind],
            key=lambda item: (item.external_id, item.provenance.path, item.observed_at),
        )
        lines = [
            json.dumps(row.to_json(), ensure_ascii=False, separators=(",", ":"), sort_keys=False)
            for row in ordered
        ]
        body = ("\n".join(lines) + "\n").encode("utf-8")
        tmp = out / f".{filename}.tmp"
        final = out / filename
        tmp.write_bytes(body)
        tmp.replace(final)
        files[filename] = {
            "sha256": hashlib.sha256(body).hexdigest(),
            "rows": len(ordered),
            "bytes": len(body),
        }
    return files


def verify_jsonl_files(out: Path, files: dict[str, dict[str, int | str]]) -> dict[str, int]:
    """Re-read finalized files and verify hashes, row counts, and row shape."""
    counts = {kind: 0 for kind in SUB_KINDS}
    seen: set[tuple[str, str]] = set()
    filename_to_kind = {filename: kind for kind, filename in JSONL_FILES.items()}
    for filename, expected in files.items():
        path = out / filename
        data = path.read_bytes()
        digest = hashlib.sha256(data).hexdigest()
        if digest != expected["sha256"]:
            raise StriatumError(f"corpus file hash mismatch: {filename}", exit_code=6)
        rows = 0
        if data:
            for line in data.decode("utf-8").splitlines():
                if line == "":
                    continue
                row = json.loads(line)
                if row.get("source_kind") != SOURCE_KIND:
                    raise StriatumError(f"invalid corpus row source_kind in {filename}", exit_code=6)
                sub_kind = row.get("sub_kind")
                if sub_kind not in SUB_KINDS:
                    raise StriatumError(f"invalid corpus row sub_kind in {filename}", exit_code=6)
                if sub_kind != filename_to_kind[filename]:
                    raise StriatumError(f"corpus row in wrong file: {filename}", exit_code=6)
                external_id = row.get("external_id")
                if not isinstance(external_id, str) or not external_id:
                    raise StriatumError(f"invalid corpus row external_id in {filename}", exit_code=6)
                key = (str(sub_kind), external_id)
                if key in seen:
                    raise StriatumError(f"duplicate corpus row: {sub_kind} {external_id}", exit_code=6)
                seen.add(key)
                rows += 1
                counts[str(sub_kind)] += 1
        if rows != expected["rows"]:
            raise StriatumError(f"corpus row count mismatch: {filename}", exit_code=6)
    return counts
