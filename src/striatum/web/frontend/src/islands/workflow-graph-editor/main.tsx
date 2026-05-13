/**
 * Production entry point for the workflow-graph-editor island.
 *
 * The Vite multi-entry build emits this file as `island-workflow-graph-editor.js`,
 * which the `workflow_edit.html` Jinja2 template loads as a deferred ES
 * module alongside the `<script id="workflow-data">` and
 * `<script id="workflow-sha256">` payloads. It does nothing on pages without
 * the matching mount slot.
 */

import { createElement } from "react";
import "../../shared/theme.css";
import { mount } from "../../shared/mount";
import type { WorkflowGraphEditorProps } from "../../shared/types";
import WorkflowGraphEditor from "./WorkflowGraphEditor";

mount<WorkflowGraphEditorProps>({
  containerId: "island-workflow-graph-editor",
  label: "workflow-graph-editor",
  defaultProps: { path: "", saveUrl: "" },
  render: (props) => createElement(WorkflowGraphEditor, props),
});
