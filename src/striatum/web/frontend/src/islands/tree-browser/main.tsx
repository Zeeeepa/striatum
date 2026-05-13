/**
 * Production entry point for the tree-browser island.
 *
 * The Vite multi-entry build emits this file as `island-tree-browser.js`,
 * which the `view_tree.html` Jinja2 template loads as a deferred ES module.
 * It does nothing on pages without the matching mount slot — the `mount`
 * helper short-circuits when `document.getElementById` returns null.
 */

import { createElement } from "react";
import "../../shared/theme.css";
import { mount } from "../../shared/mount";
import type { TreeBrowserProps } from "../../shared/types";
import TreeBrowser from "./TreeBrowser";

mount<TreeBrowserProps>({
  containerId: "island-tree-browser",
  label: "tree-browser",
  defaultProps: { rootPath: "" },
  render: (props) => createElement(TreeBrowser, props),
});
