import { createElement } from "react";
import "../../shared/theme.css";
import { mount } from "../../shared/mount";
import type { RecoveryPanelProps } from "./RecoveryPanel";
import RecoveryPanel from "./RecoveryPanel";

mount<RecoveryPanelProps>({
  containerId: "island-recovery-panel",
  label: "recovery-panel",
  defaultProps: { runId: "" },
  render: (props) => createElement(RecoveryPanel, props),
});
