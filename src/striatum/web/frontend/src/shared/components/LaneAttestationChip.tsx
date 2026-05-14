import type { LaneAttestationChipProps } from "../types";
import { cx, labelize } from "./common";

export function LaneAttestationChip({
  attested,
  reason,
  supervisorId,
  operatorLabel,
}: LaneAttestationChipProps) {
  const state = attested ? "attested" : "unattested";
  const reasonText = attested ? "" : reason ?? "unattested";
  const aria = attested
    ? "lane attestation: attested"
    : `lane attestation: unattested; reason: ${labelize(reasonText)}`;
  const hasCard = Boolean(supervisorId || operatorLabel);

  if (hasCard) {
    return (
      <details
        className={cx("attestation-chip", `attestation-chip--${state}`)}
        data-component="LaneAttestationChip"
      >
        <summary aria-label={aria}>
          <span>{state}</span>
          {!attested && reason ? <small>{reason}</small> : null}
        </summary>
        <div className="attestation-chip__card" role="note">
          {supervisorId ? (
            <div>
              supervisor: <code>{supervisorId}</code>
            </div>
          ) : null}
          {operatorLabel ? (
            <div>
              operator: <code>{operatorLabel}</code>
            </div>
          ) : null}
        </div>
      </details>
    );
  }

  return (
    <span
      className={cx("attestation-chip", `attestation-chip--${state}`)}
      data-component="LaneAttestationChip"
      aria-label={aria}
      title={reasonText || state}
      tabIndex={0}
    >
      {state}
      {!attested && reason ? <small>{reason}</small> : null}
    </span>
  );
}
