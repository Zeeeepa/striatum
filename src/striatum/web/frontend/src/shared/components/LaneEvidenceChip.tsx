import type { LaneEvidenceChipProps } from "../types";

export function LaneEvidenceChip({ state = "not_yet_correlated", rationale }: LaneEvidenceChipProps) {
  const label = state.replaceAll("_", " ");
  const title = rationale ? `${label}: ${rationale}` : label;

  return (
    <span
      className={`lane-evidence-chip lane-evidence-chip--${state}`}
      data-component="LaneEvidenceChip"
      aria-label={`lane evidence: ${title}`}
      title={title}
      tabIndex={0}
    >
      {label}
      {rationale ? <span className="chip-detail">{rationale}</span> : null}
    </span>
  );
}
