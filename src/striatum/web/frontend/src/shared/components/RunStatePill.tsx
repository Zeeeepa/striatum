import type {
  JobState,
  JobStatePillProps,
  RunRenderState,
  RunStatePillProps,
} from "../types";
import { cx, labelize } from "./common";

function stateClass(state: string, classModifier?: string): string {
  return cx(
    "status-pill",
    `status-${state}`,
    classModifier ? `status-pill--${classModifier}` : null,
  );
}

function runRenderState({ state, pausedAt }: RunStatePillProps): RunRenderState {
  if (pausedAt && (state === "ready" || state === "running")) return "paused";
  return state;
}

export function RunStatePill(props: RunStatePillProps) {
  const renderState = runRenderState(props);
  const label = labelize(renderState);
  const title = renderState === "paused" ? `paused at ${props.pausedAt}` : label;

  return (
    <span
      className={stateClass(renderState, props.classModifier)}
      data-component="RunStatePill"
      aria-label={`run state: ${label}`}
      title={title}
      tabIndex={0}
    >
      {renderState}
    </span>
  );
}

export function JobStatePill({ state, classModifier }: JobStatePillProps) {
  const label = labelize(state);

  return (
    <span
      className={stateClass(state, classModifier)}
      data-component="JobStatePill"
      aria-label={`job state: ${label}`}
      title={label}
      tabIndex={0}
    >
      {state satisfies JobState}
    </span>
  );
}
