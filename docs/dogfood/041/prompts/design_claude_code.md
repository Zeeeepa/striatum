# Claude Code Design Prompt

Produce `docs/dogfood/041/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0038 emphasizing first-time-user UX for each new affordance.

**Tree browser (`/view/`):**

- Server-side initial render: collapsed tree showing top-level repo entries. Lazy-load directory contents via `GET /v1/repo/tree` on click.
- Click a directory → expand inline.
- Click a file → navigate to `/view/<path>` (existing single-file viewer with syntax highlighting via code-viewer island).
- Breadcrumb at top showing current path.
- Search box for fuzzy-match file paths (client-side, against the loaded tree).
- Keyboard nav: arrow keys to move; Enter to expand/open; Esc to collapse.
- Empty state for empty dirs.

**Workflow chooser wizard (`/workflows/new`):**

- Step 1: Pick shape (radio cards with `display_name`, `summary`, `recommended_for` — surfaced from `GET /workflow-templates`).
- Step 2: Pick lane set (radio cards filtered by `default_lane_sets`).
- Step 3: Pick lane modifiers (multi-select; client-side compatibility-matrix validation).
- Step 4: Fill required fields (workflow_id, name, scaffold_root, artifact_root, branch suggestion, lane commands).
- Step 5: Preview — `POST /workflows/generate/preview`. Show the generated `workflow.json` rendered as a graph (reuse the existing SVG graph) + the file list.
- Step 6: Confirm + Save — `POST /workflows/generate` with `confirm_write: true`. Show success state with a link to the new workflow detail page.
- Cancel any step → return to `/workflows`.
- Operator confirmation gate at Step 6 (same pattern as RFC 0036 chat tools — UI gesture required).

**Drag-drop workflow graph editor:**

- Replaces (or augments per synthesis choice) the existing form-driven `workflow_edit.html` editor.
- Library: react-flow. Nodes are draggable; edges clickable to delete or change `on` verdict; new nodes added via a palette (one per RFC 0034 §5 closed block kind).
- Per-node side panel exposes structured fields with proper widgets:
  - `role_id` → dropdown from registered roles
  - `lane_id` → dropdown from registered lanes
  - `review_posture` → radio buttons for the 9 first-class postures + custom text input
  - `write_scope.allowed_paths` → multi-row text inputs with add/remove
  - `expected_artifacts` → form rows with kind dropdown + logical_name + path + required toggle
  - `parallel_group` → text input with autocomplete from existing groups in the workflow
- Save → POST to existing workflow-edit endpoint (server-side `workflow validate` runs; field-level errors surface inline).

**Code viewer (`/view/<path>` for non-Markdown files):**

- Library: shiki. Bundle the 8 named grammars: json, py, ts/js, sh, yaml, toml, md, sql.
- Render: line numbers + collapsible-by-default for files > 500 lines + copy-to-clipboard button + raw-link.
- Language detection from file extension; fallback to plain `<pre>` for unknown types.
- Dark-mode parity via shiki's built-in theme switching (use the existing `prefers-color-scheme` signal).

**Promoted Edit affordance (Jinja2 server-template change, not an island):**

- On `/workflows/<path>` detail page, the current `<a class="muted">Edit</a>` becomes a primary button styled like "Run this workflow now".
- Visual placement: directly to the left or right of the Run button.
- Use the existing `class="primary-button"` style for parity.

**Accessibility:**

- All islands respect `prefers-color-scheme: dark` via the existing palette CSS variables.
- Keyboard nav: every interactive element reachable via Tab; visible focus indicator (use the existing focus styles).
- ARIA labels on filter pills, radio cards, multi-select widgets.
- Focus management on `<dialog>` elements (chooser-wizard preview-pane modal): trap focus inside; close returns focus to trigger.
- Skip-link from `base.html` (RFC 0037 already added this).

**Documentation deltas (claude side):**

- New `docs/FRONTEND_DEVELOPMENT.md` — contributor-side guide (node setup, `make ui-install/build/dev/test`, island mounting pattern, type contracts, how to add a new island).
- `docs/HOW_TO_HUMAN.md` — add walkthrough for `/view/`, `/workflows/new`, the graph editor.
- `docs/UBIQUITOUS_LANGUAGE.md` — entries for "frontend island", "tree browser", "workflow chooser", "graph editor", "code viewer".
- `docs/CLI_REFERENCE.md` — no new CLI verbs, but cross-reference the web routes.
- `CHANGELOG.md` — Decided D092 (already landed) + Added entries for the feature additions.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:` exactly.

The `handoff` kind does not require YAML front matter.

If permission to call `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
