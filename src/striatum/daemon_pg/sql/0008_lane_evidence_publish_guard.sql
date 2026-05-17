ALTER TABLE striatumd.artifacts
  ADD COLUMN IF NOT EXISTS attestation_override_rationale text;
