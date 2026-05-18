/**
 * Vite multi-entry root — dev-only shell that mounts every island on the
 * same page so contributors can `make ui-dev` and click through them.
 *
 * Production pages do NOT load this file. Each Jinja2 page slot loads its
 * island's own production entry instead:
 *
 *     <div id="island-<name>" data-props='{...}'></div>
 *     <script type="module" src="/static/build/island-<name>.js" defer></script>
 *
 * The Vite build emits one bundle per `frontend/src/islands/<name>/main.tsx`
 * entry, plus a shared chunk for React, react-dom, and the helpers under
 * `frontend/src/shared/`.
 */

import { createElement } from "react";
import { mount } from "./shared/mount";
import "./shared/theme.css";

import TreeBrowser from "./islands/tree-browser";
import RecoveryPanel from "./islands/recovery-panel";
import CodeViewer from "./islands/code-viewer";
import type { RecoveryPanelProps } from "./islands/recovery-panel";

import type {
  CodeViewerProps,
  TreeBrowserProps,
} from "./shared/types";

mount<TreeBrowserProps>({
  containerId: "island-tree-browser",
  label: "tree-browser",
  defaultProps: { rootPath: "" },
  render: (props) => createElement(TreeBrowser, props),
});

mount<RecoveryPanelProps>({
  containerId: "island-recovery-panel",
  label: "recovery-panel",
  defaultProps: { runId: "" },
  render: (props) => createElement(RecoveryPanel, props),
});

mount<CodeViewerProps>({
  containerId: "island-code-viewer",
  label: "code-viewer",
  defaultProps: { path: "", language: "plaintext" },
  render: (props) => createElement(CodeViewer, props),
});
