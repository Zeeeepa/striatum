/**
 * Production entry point for the workflow-chooser island.
 *
 * The Vite multi-entry build emits this file as `island-workflow-chooser.js`,
 * which the `workflow_new.html` Jinja2 template loads as a deferred ES
 * module. It does nothing on pages without the matching mount slot.
 */

import { createElement } from "react";
import "../../shared/theme.css";
import { mount } from "../../shared/mount";
import type { WorkflowChooserProps } from "../../shared/types";
import WorkflowChooser from "./WorkflowChooser";

mount<WorkflowChooserProps>({
  containerId: "island-workflow-chooser",
  label: "workflow-chooser",
  defaultProps: { allowMutations: false },
  render: (props) => createElement(WorkflowChooser, props),
});
