import type { PostureChipProps } from "../types";
import { cx, labelize } from "./common";

export function PostureChip({ posture }: PostureChipProps) {
  const customName = posture.startsWith("custom:") ? posture.slice("custom:".length) : null;
  const label = customName ? `custom - ${customName}` : labelize(posture);

  return (
    <span
      className={cx("posture-chip", customName ? "posture-chip--custom" : `posture-chip--${posture}`)}
      data-component="PostureChip"
      aria-label={`review posture: ${label}`}
      title={label}
      tabIndex={0}
    >
      {customName ? (
        <>
          custom <small>{customName}</small>
        </>
      ) : (
        posture
      )}
    </span>
  );
}
