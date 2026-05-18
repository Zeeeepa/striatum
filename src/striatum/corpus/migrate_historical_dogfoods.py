"""RFC 0072 step 7: bulk-migrate ``docs/dogfood/`` into blob storage.

One-shot operator script. Walks the on-disk dogfood directories,
uploads every file to the configured per-repo S3 bucket at
``dogfood-historical/<dogfood_id>/<rel_path>``, and reports what
happened. Idempotent — files already present in the bucket with a
matching sha256 are skipped.

The script does **not** delete the on-disk files: that's the operator's
explicit step (``git rm -r docs/dogfood/`` plus an
``.gitignore`` update). The script's job is to make the deletion
safe — i.e., every file is verified to be available in blob storage.

Configuration is read from environment variables (matching
``go/pkg/blob.LoadConfig``):

- ``STRIATUM_BLOB_ENDPOINT`` (required) — base URL, e.g.
  ``http://127.0.0.1:3900``.
- ``STRIATUM_BLOB_ACCESS_KEY`` / ``STRIATUM_BLOB_SECRET_KEY`` (required).
- ``STRIATUM_BLOB_REGION`` (default ``us-east-1``).
- ``STRIATUM_BLOB_PATH_STYLE`` (default true).

The per-repo bucket name is supplied via ``--bucket`` (the operator
already knows it from the ``adopt`` flow).
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterator


@dataclass(frozen=True)
class MigrationEntry:
    """One source file's migration record."""

    dogfood_id: str
    rel_path: str
    blob_key: str
    sha256: str
    size_bytes: int
    status: str  # "uploaded", "skipped_already_present", "verified", "error"
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


def sha256_of(path: Path) -> tuple[str, int]:
    h = hashlib.sha256()
    size = 0
    with path.open("rb") as f:
        while True:
            chunk = f.read(64 * 1024)
            if not chunk:
                break
            h.update(chunk)
            size += len(chunk)
    return h.hexdigest(), size


def canonical_blob_key(dogfood_id: str, rel_path: str) -> str:
    """Canonical key shape, mirrored from ``go/pkg/blob.HistoricalDogfoodKey``.

    Path-shaped (browsable in the MinIO console); no sha256 in the
    key because integrity verification is via the bucket-level
    sha256 round-trip.
    """

    clean = rel_path.lstrip("/").removeprefix("./")
    return f"dogfood-historical/{dogfood_id}/{clean}"


def content_type_for(path: Path) -> str:
    suffix = path.suffix.lower()
    if suffix == ".md":
        return "text/markdown; charset=utf-8"
    if suffix == ".json":
        return "application/json"
    if suffix == ".txt":
        return "text/plain; charset=utf-8"
    if suffix in {".yml", ".yaml"}:
        return "application/x-yaml"
    if suffix == ".sh":
        return "text/x-shellscript; charset=utf-8"
    return "application/octet-stream"


class MigrationConfig:
    """Loaded once at script start. Mirrors blob.Config on the Go side."""

    def __init__(self) -> None:
        endpoint = os.environ.get("STRIATUM_BLOB_ENDPOINT", "").strip()
        if not endpoint:
            raise SystemExit("STRIATUM_BLOB_ENDPOINT is required")
        access = os.environ.get("STRIATUM_BLOB_ACCESS_KEY", "").strip()
        secret = os.environ.get("STRIATUM_BLOB_SECRET_KEY", "").strip()
        if not access:
            raise SystemExit("STRIATUM_BLOB_ACCESS_KEY is required")
        if not secret:
            raise SystemExit("STRIATUM_BLOB_SECRET_KEY is required")
        region = os.environ.get("STRIATUM_BLOB_REGION", "").strip() or "us-east-1"
        path_style = os.environ.get("STRIATUM_BLOB_PATH_STYLE", "").lower()
        secure = endpoint.startswith("https://")
        host = endpoint.removeprefix("https://").removeprefix("http://")
        self.host = host
        self.region = region
        self.access = access
        self.secret = secret
        self.secure = secure
        # path_style ∈ {"", "1", "true", "yes", "on"} → True; "0"/"false"/"no"/"off" → False.
        if path_style in ("0", "false", "no", "off"):
            self.path_style = False
        else:
            self.path_style = True


def build_client(cfg: MigrationConfig) -> Any:
    from minio import Minio

    return Minio(
        cfg.host,
        access_key=cfg.access,
        secret_key=cfg.secret,
        secure=cfg.secure,
        region=cfg.region,
    )


def remote_sha256(client: Any, bucket: str, key: str) -> str | None:
    """Return the sha256 of the existing object at ``bucket/key`` if it
    exists. Sha is stored in user metadata under X-Amz-Meta-X-Striatum-Sha256
    by ``go/pkg/blob.Client.PutBytes``; if absent, return "" (object
    exists but no sha known).
    Returns ``None`` if the object does not exist.
    """

    from minio.error import S3Error

    try:
        stat = client.stat_object(bucket, key)
    except S3Error as exc:
        if exc.code in ("NoSuchKey", "NoSuchBucket"):
            return None
        raise
    meta = getattr(stat, "metadata", None) or {}
    for header_key, value in meta.items():
        if header_key.lower() in (
            "x-amz-meta-x-striatum-sha256",
            "x-striatum-sha256",
        ):
            return str(value)
    return ""


def migrate_one(
    client: Any,
    bucket: str,
    dogfood_id: str,
    path: Path,
    rel_path: str,
    *,
    dry_run: bool,
) -> MigrationEntry:
    key = canonical_blob_key(dogfood_id, rel_path)
    try:
        digest, size = sha256_of(path)
    except OSError as exc:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=key,
            sha256="",
            size_bytes=0,
            status="error",
            error=f"local read failed: {exc}",
        )

    existing = remote_sha256(client, bucket, key)
    if existing == digest:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=key,
            sha256=digest,
            size_bytes=size,
            status="skipped_already_present",
        )

    if dry_run:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=key,
            sha256=digest,
            size_bytes=size,
            status="would_upload",
        )

    import io

    try:
        body = path.read_bytes()
        client.put_object(
            bucket,
            key,
            io.BytesIO(body),
            length=len(body),
            content_type=content_type_for(path),
            metadata={"X-Striatum-Sha256": digest},
        )
    except Exception as exc:  # noqa: BLE001 - propagate any S3 failure verbatim.
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=key,
            sha256=digest,
            size_bytes=size,
            status="error",
            error=f"put failed: {type(exc).__name__}: {exc}",
        )

    # Verify round-trip
    verify = remote_sha256(client, bucket, key)
    if verify is None:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=key,
            sha256=digest,
            size_bytes=size,
            status="error",
            error="verify failed: object missing after put",
        )
    if verify and verify != digest:
        return MigrationEntry(
            dogfood_id=dogfood_id,
            rel_path=rel_path,
            blob_key=key,
            sha256=digest,
            size_bytes=size,
            status="error",
            error=f"verify failed: remote sha {verify} != local sha {digest}",
        )
    return MigrationEntry(
        dogfood_id=dogfood_id,
        rel_path=rel_path,
        blob_key=key,
        sha256=digest,
        size_bytes=size,
        status="uploaded",
    )


def run_migration(
    *,
    repo: Path,
    bucket: str,
    dry_run: bool,
    limit: int | None,
) -> tuple[list[MigrationEntry], dict[str, int]]:
    cfg = MigrationConfig()
    client = build_client(cfg)

    dogfood_root = repo / "docs" / "dogfood"
    entries: list[MigrationEntry] = []
    counts = {"uploaded": 0, "skipped_already_present": 0, "would_upload": 0, "error": 0}
    for index, (dogfood_id, path, rel_path) in enumerate(iter_dogfood_files(dogfood_root)):
        if limit is not None and index >= limit:
            break
        entry = migrate_one(client, bucket, dogfood_id, path, rel_path, dry_run=dry_run)
        entries.append(entry)
        counts[entry.status] = counts.get(entry.status, 0) + 1
    return entries, counts


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="striatum-corpus-migrate-historical-dogfoods",
        description=(
            "RFC 0072: bulk-migrate docs/dogfood/ into the configured "
            "per-repo S3 bucket. Idempotent. Does not delete on-disk files."
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
        help="Per-repo S3 bucket (recorded on striatumd.repositories.blob_bucket)",
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
