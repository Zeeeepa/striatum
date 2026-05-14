import type { LaneEvidenceChipProps } from "../types";

export function LaneEvidenceChip({ state = "not_yet_correlated" }: LaneEvidenceChipProps) {
  return (
    <span
      className={`lane-evidence-chip lane-evidence-chip--${state}`}
      data-component="LaneEvidenceChip"
      aria-label="lane evidence: not yet correlated"
      title="not yet correlated"
      tabIndex={0}
    >
      {state}
    </span>
  );
}
