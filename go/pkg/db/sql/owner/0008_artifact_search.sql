-- RFC 0119 / D179 — owner bundle 0008: artifact metadata full-text search.
--
-- The recall hot tier indexes immutable artifact metadata only. This DDL lives
-- in the owner bundle because it alters the protected artifacts table; runtime
-- startup migrations must not carry owner-table DDL (D187).

ALTER TABLE striatumd.artifacts
  ADD COLUMN IF NOT EXISTS search_vector tsvector
  GENERATED ALWAYS AS (
    to_tsvector('english',
      coalesce(logical_name, '') || ' ' ||
      coalesce(artifact_kind, '') || ' ' ||
      coalesce(replace(repo_path, '/', ' '), '') || ' ' ||
      coalesce(author_line, '')
    )
  ) STORED;

CREATE INDEX IF NOT EXISTS idx_artifacts_search
  ON striatumd.artifacts USING GIN (search_vector);
