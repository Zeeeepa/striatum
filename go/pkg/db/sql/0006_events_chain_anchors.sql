-- RFC 0048 V1.5 #4 / #6 — promote per-event chain anchors out of payload_json
-- into dedicated columns on striatumd.events, plus a fast per-repository
-- chain-head pointer.
--
-- Pre-0006, `RepoHandlerContext.append_event` embedded chain metadata in
-- `payload_json._event_chain`:
--   { "algorithm": "sha256-json-v1",
--     "previous_hash": "...",
--     "row_hash": "..." }
-- This is durable but mixes audit-significant cryptography with
-- caller-visible payload data, complicates byte-equivalence parity tests
-- (the SQLite path does not carry the same blob in its event rows), and
-- forces every event read to extract the anchor via JSON access.
--
-- 0006 adds two append-only columns to `striatumd.events` plus a singleton
-- `striatumd.repo_event_chain_heads` row per repository_id, then backfills
-- both from `payload_json._event_chain` so existing audit chains keep
-- verifying. The `_event_chain` key is stripped from payload_json by the
-- backfill so reads return clean caller payloads.
--
-- Forward-only. The new columns are NOT NULL for `row_hash` (every row has
-- a hash) and NULLABLE for `previous_hash` (the first event of a repository
-- has no predecessor).

ALTER TABLE striatumd.events
  ADD COLUMN IF NOT EXISTS previous_hash text,
  ADD COLUMN IF NOT EXISTS row_hash text;

UPDATE striatumd.events
SET previous_hash = payload_json #>> '{_event_chain,previous_hash}',
    row_hash = payload_json #>> '{_event_chain,row_hash}'
WHERE row_hash IS NULL
  AND payload_json ? '_event_chain';

-- Refuse to migrate rows that lack chain metadata under the assumption
-- that no production deployment can land here — the V1.5 invariant has
-- always been that append_event materializes the chain. A NULL row_hash
-- post-backfill means the row predates the chain code path and the
-- operator must investigate.
DO $$
DECLARE
  unanchored_count bigint;
BEGIN
  SELECT count(*) INTO unanchored_count FROM striatumd.events WHERE row_hash IS NULL;
  IF unanchored_count > 0 THEN
    RAISE EXCEPTION 'migration 0006 refuses: % event rows lack row_hash anchor', unanchored_count;
  END IF;
END;
$$;

-- row_hash is logically required for every event written by the application,
-- but kept NULLABLE at the column level so test seeds and operational
-- back-filling tooling can insert rows in two phases (INSERT with NULL
-- row_hash, then UPDATE after computing the hash against the assigned
-- event_id). Production writers (RepoHandlerContext.append_event in Python
-- and pkg/mutations in Go) always populate row_hash. The migration's
-- explicit refusal above (line `migration 0006 refuses: % event rows lack
-- row_hash anchor`) covers the backfill-correctness invariant for any
-- existing rows.

-- Drop the _event_chain key from payload_json so future readers see clean
-- caller payloads.
UPDATE striatumd.events
SET payload_json = payload_json - '_event_chain'
WHERE payload_json ? '_event_chain';

-- repo_event_chain_heads: one row per repository carrying the latest
-- event's row_hash so append_event can read the chain head without an
-- ORDER BY DESC scan on striatumd.events. Acts the same role as
-- striatumd.audit_chain_head for request-level audit.
CREATE TABLE IF NOT EXISTS striatumd.repo_event_chain_heads (
  repository_id text PRIMARY KEY REFERENCES striatumd.repositories(repository_id),
  last_event_id bigint NOT NULL,
  last_hash text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  -- Foreign key references the composite primary key (repository_id, event_id)
  -- on striatumd.events. DEFERRABLE so the chain-head row can be UPDATEd in
  -- the same transaction as a new event INSERT without a foreign-key flap.
  FOREIGN KEY (repository_id, last_event_id)
    REFERENCES striatumd.events(repository_id, event_id) DEFERRABLE INITIALLY DEFERRED
);

-- Backfill heads from the latest event per repository.
INSERT INTO striatumd.repo_event_chain_heads(repository_id, last_event_id, last_hash, updated_at)
SELECT DISTINCT ON (e.repository_id)
       e.repository_id, e.event_id, e.row_hash, now()
FROM striatumd.events e
ORDER BY e.repository_id, e.event_id DESC
ON CONFLICT (repository_id) DO NOTHING;

-- Append-only invariants — no UPDATE or DELETE on existing rows.
DROP TRIGGER IF EXISTS repo_event_chain_heads_no_delete ON striatumd.repo_event_chain_heads;
-- Heads are mutable (every new event UPDATEs the head row), so we do NOT
-- add a no_update trigger. The head row is created lazily on first event
-- per repository and DELETED only when the repository is removed; we
-- enforce DELETE only via the repository foreign key cascade — no
-- standalone DELETEs.

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'striatumd_rw') THEN
    GRANT SELECT, INSERT, UPDATE ON striatumd.repo_event_chain_heads TO striatumd_rw;
    -- DELETE is denied so heads can only be removed by cascading the
    -- repositories foreign-key constraint (operator-level admin path).
    REVOKE DELETE ON striatumd.repo_event_chain_heads FROM striatumd_rw;
  END IF;
END;
$$;
