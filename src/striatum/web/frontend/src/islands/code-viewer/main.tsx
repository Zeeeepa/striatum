/**
 * Production entry point for the code-viewer island.
 *
 * The Vite multi-entry build emits this file as `island-code-viewer.js`,
 * which the `view_file.html` Jinja2 template loads as a deferred ES module
 * for non-Markdown text files. It does nothing on pages without the matching
 * mount slot — Markdown is rendered server-side upstream and never mounts
 * this island.
 */

import { createElement } from "react";
import "../../shared/theme.css";
import { mount } from "../../shared/mount";
import type { CodeViewerProps } from "../../shared/types";
import CodeViewer from "./CodeViewer";

mount<CodeViewerProps>({
  containerId: "island-code-viewer",
  label: "code-viewer",
  defaultProps: { path: "", language: "plaintext" },
  render: (props) => createElement(CodeViewer, props),
});
