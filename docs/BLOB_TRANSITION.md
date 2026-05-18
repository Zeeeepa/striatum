# Blob Transition Runbook (RFC 0072)

Status: V1 shipped, operator migration pending
Date: 2026-05-18
Context: [RFC 0072](rfcs/0072-blob-backed-artifact-storage.md), [POSTGRES_TRANSITION.md](POSTGRES_TRANSITION.md)

This runbook walks the maintainer through transitioning a striatum
deployment to blob-backed artifact storage. The infrastructure
(daemon S3 client, publish/get_content/list_for_run RPC handlers,
doctor block, web UI viewer, bulk-migration script) shipped in
commits `154fac4` through `4fc41ae`. The remaining work is
operator-side: stand up an S3-compatible service, adopt repos
against it, then bulk-migrate the historical `docs/dogfood/`
content.

## Prerequisites

- A reachable S3-compatible service. Recommended local choice:
  [Garage](https://garagehq.deuxfleurs.fr/) or
  [MinIO](https://min.io/). Cloud S3 works too but breaks the
  local-first invariant; pick deliberately.
- The service's endpoint URL, access key, and secret key.
- A running `striatumd` (Go daemon) connected to PostgreSQL.

Garage smoke test reference (the maintainer's working deployment):

```
S3 endpoint    | http://127.0.0.1:3900 (region garage, path-style)
Access Key ID  | GK<...redacted...>
Secret         | in /root/garage-credentials.txt (0600)
```

## Step 1 — Configure the daemon

The daemon reads these environment variables at startup:

```bash
export STRIATUM_BLOB_ENDPOINT=http://127.0.0.1:3900   # required
export STRIATUM_BLOB_ACCESS_KEY=<access key>          # required
export STRIATUM_BLOB_SECRET_KEY=<secret>              # required
export STRIATUM_BLOB_REGION=garage                    # default us-east-1
export STRIATUM_BLOB_PATH_STYLE=true                  # default true; required for MinIO/Garage
export STRIATUM_BLOB_BUCKET_PREFIX=striatum-          # default striatum-
```

Restart `striatumd`. The startup log will include
`blob storage configured` when `STRIATUM_BLOB_ENDPOINT` is set.

**Credential placement gotcha**: if the daemon runs as a non-root
user but the secret is at `/root/garage-credentials.txt` (mode 0600,
root-only), the daemon cannot read it. Either move the credentials
to a path the daemon's user can read (e.g.,
`~/.config/striatum/blob-credentials` mode 0600) and `EnvironmentFile=`
them from systemd, or run the daemon as root for V1 and tighten
later.

## Step 2 — Verify with `daemon doctor`

```bash
striatum daemon doctor --json | jq .blob
```

A daemon-global success (no repository_id supplied) reports:

```json
{
  "configured": true,
  "reachable": true,
  "errors": []
}
```

A repository-scoped doctor (with `--repo /path/to/registered/repo`)
adds bucket-level diagnostics:

```json
{
  "configured": true,
  "reachable": true,
  "bucket": "striatum-<repository_id>",
  "bucket_exists": true,
  "bucket_status": "ok",
  "round_trip_ms": 12,
  "round_trip_sha256": "...",
  "errors": []
}
```

`bucket_status: "not_provisioned"` means the repo row has NULL
`blob_bucket`; you have not run `striatum adopt --apply-blob-creation`
for this repo yet (step 3).

`bucket_status: "missing"` means the bucket name is set but the
bucket does not exist on the endpoint. Re-run `adopt --apply-blob-creation`.

## Step 3 — Adopt repositories against the bucket

For a new repo:

```bash
striatum adopt --profile claude_code --apply-blob-creation /path/to/repo
```

The `--apply-blob-creation` flag tells the daemon to create the
per-repo bucket if it does not exist. Without it, an unprovisioned
bucket refuses with `blob_apply_required`.

For an already-adopted repo (no blob_bucket yet — the repo was
registered before blob was configured):

```bash
striatum adopt --apply-blob-creation /path/to/repo
```

The daemon detects the existing registration and backfills
`blob_bucket` on the `striatumd.repositories` row.

To explicitly choose the bucket name (instead of the default
`<prefix><repository_id>`):

```bash
striatum adopt --apply-blob-creation --blob-bucket striatum-myrepo /path/to/repo
```

**Exit code 12 (`repo_blob_conflict`)** means the chosen bucket is
already claimed by a different repository (a claim marker
`_striatum_repo_marker` exists with a different repository_id) or
contains striatum-shaped keys without any claim marker. Either pick
a different bucket name or empty the conflicting bucket.

## Step 4 — Verify new artifacts route to blob

Trigger a workflow that publishes a blob-routed artifact kind
(`finding`, `synthesis`, `support_ledger`, `action_item_ledger`,
`harness_improvement_proposal`, `findings_ledger`, or
`progress_note`). After the publish:

```bash
striatum --json invoke artifact.list_for_run --repository_id <id> --run_id <run>
```

Expected: each blob-routed artifact carries non-null `blob_key` and
`blob_sha256`. Decisional kinds (`decision`, `escalation`,
`work_plan`, `operator_brief`, `operator_report`) still have
`blob_key: null` — they stay git-tracked.

`striatum --json invoke artifact.get_content --repository_id <id> --artifact_id <art>`
returns `{ "source": "blob", "verified": true, ... }` for
blob-routed artifacts and `{ "source": "repo_path", ... }` for
legacy / decisional ones.

## Step 5 — Bulk-migrate `docs/dogfood/`

**This step is destructive for the working tree**: after it lands,
1,305 files across 66 dogfood directories move from git tracking
into blob storage.

Before running:

- Confirm `daemon doctor --repo <striatum repo> --json | jq .blob`
  reports `bucket_status: "ok"` for the striatum repo.
- Look up the bucket name:
  `psql -c "SELECT blob_bucket FROM striatumd.repositories WHERE repo_root = '/path/to/striatum'"`.
- Install the blob extra: `pip install -e '.[blob]'`.

Dry-run first:

```bash
striatum corpus migrate-historical-dogfoods \
  --bucket <bucket> \
  --dry-run --json | tee migration-dry-run.json
jq '.counts' migration-dry-run.json
```

Expected: `would_upload` ≈ 1,305 (the file count), `error: 0`.

Real run (idempotent — repeated runs skip already-uploaded files):

```bash
striatum corpus migrate-historical-dogfoods --bucket <bucket> --json | tee migration.json
jq '.counts' migration.json
```

Expected: `uploaded` + `skipped_already_present` ≈ 1,305,
`error: 0`. If any errors, **stop here** — the on-disk files are
still present, and re-running the migration after fixing the
underlying cause is safe.

Verify a sample by re-fetching one file's body and comparing sha256:

```bash
jq -r '.entries[0] | "\(.blob_key) \(.sha256)"' migration.json
# Pick the printed (blob_key, sha256); fetch via the S3 client and
# diff against the on-disk file.
```

## Step 6 — Remove `docs/dogfood/` from the working tree

Only after step 5 reports zero errors:

```bash
git rm -r docs/dogfood/
# Append docs/dogfood/ to .gitignore so future runs don't re-commit.
echo "docs/dogfood/" >> .gitignore
git add .gitignore
git commit -m "RFC 0072 step 8: docs/dogfood/ migrated to blob storage"
git push
```

After this commit, the working tree drops by ~1,300 files and
all of that content lives in
`s3://<bucket>/dogfood-historical/<dogfood_id>/<rel_path>`.

## Verifying the round trip after the cutover

```bash
# Web UI viewer (blob-routed artifacts):
$EDITOR / web-browser → http://localhost:<port>/run/<run_id>/artifacts/<artifact_id>

# Corpus export should produce the same redacted bundle:
striatum corpus export --since <ref> --out /tmp/corpus
striatum corpus verify --bundle /tmp/corpus

# Doctor stays green:
striatum daemon doctor --repo /path/to/striatum --json | jq '.blob.bucket_status'
# → "ok"
```

## What did NOT migrate to blob

Per RFC 0072 § Boundary, the following kinds stay git-tracked
because they are PR-review-shaped, not per-run data:

- `decision` (decision log entries)
- `escalation` (human-principal blocker artifacts)
- `work_plan` (run planning artifacts)
- `operator_brief` (cold-start cache for the maintainer)
- `operator_report` (per-run summaries the maintainer reads in PRs)

Source code, RFCs, CHANGELOG, ROADMAP, the `docs/operator/` cold-start
cache, and the documentation tree itself are also git-tracked
(unchanged behavior).

## Rollback

If something goes catastrophically wrong post-migration:

- The on-disk content is in git history at the pre-migration commit.
  `git revert <step-8 commit>` restores the files.
- The blob copies are not deleted by the migration script; they remain
  in S3. A re-run after rollback skips re-uploading (idempotent).

The only durably destructive case is losing the S3 backend itself
(LUKS key loss, hosted account closure). Mitigate with periodic
`aws s3 sync` or `garage repair` backups, especially before any
re-org of the storage layer.
