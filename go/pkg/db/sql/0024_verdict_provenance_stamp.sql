-- RFC 0118 P0-1 (GH #240): freeze each verdict's provenance basis at record
-- time. The run_45aa8852 incident showed a terminal run whose verdict
-- provenance was only reconstructable through live lanehealth probes — by the
-- time the operator audited the run the lanes were gone, so the probe read
-- the very emptiness it was supposed to explain. These columns stamp, on the
-- verdict row itself, what the admission gate saw when the verdict was
-- recorded:
--
--   lane_attestation_at_record    'attested' | 'unattested' — the SAME probe
--                                 value the admission gate decided on (threaded
--                                 from the gate, not re-probed, so a lane that
--                                 flips between probe points cannot stamp a
--                                 value inconsistent with the gate decision)
--   review_provenance_override    true when an operator decision escaped the
--                                 provenance gate for this verdict
--   review_provenance_decision_id the authorizing decision: the verdict-time
--                                 gate decision, or the work.claim_overridden
--                                 claim decision carried forward onto the
--                                 verdict (RFC 0118 reconciliation note 1)
--   supervisor_id_at_record       the attached supervisor identity at record
--                                 time (NULL when unattested)
--
-- Pre-migration rows keep NULL stamps. Per the operator decision on RFC 0118
-- decision #1 (fail-closed-after-migration), the run-completion gate treats a
-- NULL stamp as NOT attested — it re-verifies or escalates, never assumes.

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS lane_attestation_at_record text;

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS review_provenance_override boolean;

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS review_provenance_decision_id text;

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS supervisor_id_at_record text;
