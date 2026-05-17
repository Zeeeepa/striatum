ALTER TABLE striatumd.runs
  DROP CONSTRAINT IF EXISTS runs_state_check;

ALTER TABLE striatumd.runs
  ADD CONSTRAINT runs_state_check CHECK (state IN (
    'needs_branch_confirmation','ready','running','blocked',
    'completed','failed','canceled','compromised'
  ));

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS superseded_by_decision_id text;

ALTER TABLE striatumd.verdicts
  ADD COLUMN IF NOT EXISTS superseded_at timestamptz;
