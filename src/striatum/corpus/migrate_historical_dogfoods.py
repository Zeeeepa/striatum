"""RFC 0072 step 7: bulk-migrate ``docs/dogfood/`` into blob storage.

One-shot operator script. Walks the on-disk dogfood directories and
issues a per-file ``corpus.migrate_historical_dogfood_file`` RPC to
the running daemon, which holds the S3 credentials and the per-repo
bucket binding. Idempotent: a remote object whose
``X-Striatum-Sha256`` metadata matches the local sha256 is reported
as ``skipped_already_present`` and no upload is performed.

The script does **not** delete the on-disk files: that's the operator's
explicit step (``git rm -r docs/dogfood/`` plus an ``.gitignore``
update). The script's job is to make the deletion safe — i.e., every
file is verified to be available in blob storage.

The Python side has no S3 client dependency: the daemon holds the
credentials, the daemon talks to S3, the CLI just walks the tree and
hands bodies to the daemon over the existing RPC envelope.
"""

from __future__ import annotations

import argparse
import base64
import json
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Iterator


@dataclass(frozen=True)
class MigrationEntry:
    """One source file's migration record."""

    dogfood_id: str
    rel_path: str
    blob_key: str
    sha256: str
    size_bytes: int
    status: str  # "uploaded", "skipped_already_present", "would_upload", "error"
    error: str | None = None


def iter_dogfood_files(dogfood_root: Path) -> Iterator[tuple[str, Path, str]]:
    """Yield (dogfood_id, file_path, rel_path) for every file under
    ``dogfood_root``. The dogfood_id is the immediate child directory
    name (e.g. ``001-v2``, ``054b``). rel_path is the file's path
    relative to that dogfood_id directory.
    """

    if not dogfood_root.is_dir():
        return
    for entry in sorted(dogfood_root.iterdir()):
        if not entry.is_dir():
            continue
        if entry.name.startswith("."):
            continue
        dogfood_id = entry.name
        for path in sorted(entry.rglob("*")):
            if not path.is_file():
                continue
            rel_path = path.relative_to(entry).as_posix()
            yield dogfood_id, path, rel_path


def canonical_blob_key(dogfood_id: str, rel_path: str) -> str:
    """Canonical key shape, mirrored from
    ``go/pkg/blob.HistoricalDogfoodKey``. Used by the CLI only for
    display in the per-file report; the daemon computes the
    authoritative key independently and returns it in the response.
    """

    clean = rel_path.lstrip("/").removeprefix("./")
    return f"dogfood-historical/{dogfood_id}/{clean}"


def migrate_one(
    repo: Path,
    dogfood_id: str,
    path: Path,
    rel_path: str,
    *,
    dry_run: bool,
) -> MigrationEntry:
    try:
        body = path.read_bytes()
    except OSError as exc:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=canonical_blob_key(dogfood_id, rel_path),
            sha256="",
            size_bytes=0,
            status="error",
            error=f"local read failed: {exc}",
        )

    try:
        from striatum.service_daemon import ServiceDaemonRpcError, call_repo_method

        params: dict[str, object] = {
            "dogfood_id": dogfood_id,
            "rel_path": rel_path,
            "body_base64": base64.b64encode(body).decode("ascii"),
        }
        if dry_run:
            params["dry_run"] = True
        result = call_repo_method(
            repo,
            "corpus.migrate_historical_dogfood_file",
            params,
        )
    except ServiceDaemonRpcError as exc:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=canonical_blob_key(dogfood_id, rel_path),
            sha256="",
            size_bytes=len(body),
            status="error",
            error=f"daemon rpc {exc.code}: {exc.message}",
        )
    except Exception as exc:  # noqa: BLE001 - any failure becomes a per-file error.
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=canonical_blob_key(dogfood_id, rel_path),
            sha256="",
            size_bytes=len(body),
            status="error",
            error=f"{type(exc).__name__}: {exc}",
        )

    if not isinstance(result, dict):
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=canonical_blob_key(dogfood_id, rel_path),
            sha256="",
            size_bytes=len(body),
            status="error",
            error="daemon returned unexpected response shape",
        )

    blob_key = str(result.get("blob_key") or canonical_blob_key(dogfood_id, rel_path))
    sha256 = str(result.get("sha256") or "")
    size = int(result.get("size_bytes") or len(body))
    status = str(result.get("status") or "error")
    return MigrationEntry(
        dogfood_id=dogfood_id,
        rel_path=rel_path,
        blob_key=blob_key,
        sha256=sha256,
        size_bytes=size,
        status=status,
    )


def run_migration(
    *,
    repo: Path,
    bucket: str,
    dry_run: bool,
    limit: int | None,
) -> tuple[list[MigrationEntry], dict[str, int]]:
    """Walk the on-disk dogfood tree and migrate each file through
    the daemon RPC.

    The ``bucket`` argument is kept for parity with the operator
    runbook and surfaced in the report header, but the daemon
    authoritatively resolves the bucket from
    ``striatumd.repositories.blob_bucket`` for the addressed repo.
    Passing a different value here does not redirect the upload.
    """

    del bucket  # daemon-side lookup is authoritative; see docstring.

    dogfood_root = repo / "docs" / "dogfood"
    entries: list[MigrationEntry] = []
    counts: dict[str, int] = {
        "uploaded": 0,
        "skipped_already_present": 0,
        "would_upload": 0,
        "error": 0,
    }
    for index, (dogfood_id, path, rel_path) in enumerate(iter_dogfood_files(dogfood_root)):
        if limit is not None and index >= limit:
            break
        entry = migrate_one(repo, dogfood_id, path, rel_path, dry_run=dry_run)
        entries.append(entry)
        counts[entry.status] = counts.get(entry.status, 0) + 1
    return entries, counts


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="striatum-corpus-migrate-historical-dogfoods",
        description=(
            "RFC 0072: bulk-migrate docs/dogfood/ into the per-repo S3 bucket "
            "via the daemon. Idempotent. Does not delete on-disk files."
        ),
    )
    parser.add_argument(
        "--repo",
        type=Path,
        default=Path.cwd(),
        help="Path to the striatum repo (default: cwd)",
    )
    parser.add_argument(
        "--bucket",
        required=True,
        help=(
            "Per-repo S3 bucket (informational; the daemon resolves the "
            "bucket authoritatively from striatumd.repositories.blob_bucket)."
        ),
    )
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument(
        "--limit",
        type=int,
        default=None,
        help="Stop after N files (for smoke testing)",
    )
    parser.add_argument("--json", action="store_true", help="Emit a JSON report on stdout")
    args = parser.parse_args(argv)

    entries, counts = run_migration(
        repo=args.repo.resolve(),
        bucket=args.bucket,
        dry_run=args.dry_run,
        limit=args.limit,
    )

    if args.json:
        report = {
            "ok": counts.get("error", 0) == 0,
            "counts": counts,
            "entries": [
                {
                    "dogfood_id": e.dogfood_id,
                    "rel_path": e.rel_path,
                    "blob_key": e.blob_key,
                    "sha256": e.sha256,
                    "size_bytes": e.size_bytes,
                    "status": e.status,
                    "error": e.error,
                }
                for e in entries
            ],
        }
        print(json.dumps(report, indent=2))
    else:
        for entry in entries:
            tag = entry.status if entry.status != "error" else f"error ({entry.error})"
            print(f"{tag:38} {entry.dogfood_id:12} {entry.rel_path}")
        print()
        print("Counts:")
        for key, value in sorted(counts.items()):
            print(f"  {key:30} {value}")

    return 0 if counts.get("error", 0) == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
