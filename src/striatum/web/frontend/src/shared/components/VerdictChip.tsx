import type { VerdictChipProps } from "../types";
import { cx, labelize } from "./common";

function provenanceLabel(props: VerdictChipProps): string {
  if (props.provenance === "operator-override") {
    return props.rationale ? `override - ${props.rationale}` : "override";
  }
  if (props.provenance === "cycle-revised") {
    const index = props.cycleIndex ?? "?";
    const limit = props.cycleLimit ?? "?";
    return `cycle ${index} of ${limit}`;
  }
  return "";
}

export function VerdictChip(props: VerdictChipProps) {
  const label = labelize(props.verdict);
  const provenance = provenanceLabel(props);
  const aria = provenance ? `verdict: ${label}; ${provenance}` : `verdict: ${label}`;

  if (props.provenance === "operator-override" && props.rationale) {
    return (
      <details
        className={cx("verdict-chip", `verdict-${props.verdict}`, "verdict-chip--override")}
        data-component="VerdictChip"
      >
        <summary aria-label={aria}>
          <span aria-hidden="true" className="verdict-chip__dot verdict-chip__dot--override" />
          <span>{props.verdict}</span>
          <small>override - {props.rationale}</small>
        </summary>
        <div className="verdict-chip__card" role="note">
          {props.rationale}
        </div>
      </details>
    );
  }

  return (
    <span
      className={cx("verdict-chip", `verdict-${props.verdict}`, `verdict-chip--${props.provenance}`)}
      data-component="VerdictChip"
      aria-label={aria}
      title={provenance || label}
      tabIndex={0}
    >
      <span aria-hidden="true" className={`verdict-chip__dot verdict-chip__dot--${props.provenance}`} />
      {props.verdict}
      {provenance ? <small>{provenance}</small> : null}
    </span>
  );
}
