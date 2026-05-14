import type { BylineLineProps } from "../types";
import { canonicalizeByline, cx } from "./common";

export function BylineLine({
  authorLine,
  expectedAuthorLine,
  attested,
  operatorLabel,
}: BylineLineProps) {
  const canonicalActual = canonicalizeByline(authorLine);
  const canonicalExpected = canonicalizeByline(expectedAuthorLine);
  const mismatch = Boolean(canonicalActual && canonicalExpected && canonicalActual !== canonicalExpected);
  const rendered =
    attested === false
      ? operatorLabel
        ? `author: operator [self-declared: ${operatorLabel}]`
        : "author: operator"
      : canonicalActual ?? "author: <missing>";
  const hasRenderableAuthor = attested === false || Boolean(canonicalActual);

  return (
    <span
      className={cx(
        "byline-line",
        !canonicalActual ? "byline-line--missing" : null,
        mismatch ? "byline-line--drift" : null,
        attested === false ? "byline-line--unattested" : null,
      )}
      data-component="BylineLine"
      aria-label={mismatch ? "author byline drift" : "author byline"}
      title={mismatch && canonicalExpected ? `expected ${canonicalExpected}` : rendered}
      tabIndex={0}
    >
      {mismatch ? <span aria-hidden="true" className="byline-line__dot" /> : null}
      {hasRenderableAuthor ? <code>{rendered}</code> : <em>{rendered}</em>}
      {mismatch && canonicalExpected ? (
        <small>
          expected <code>{canonicalExpected}</code>
        </small>
      ) : null}
    </span>
  );
}
