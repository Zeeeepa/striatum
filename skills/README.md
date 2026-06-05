# Optional Skills

Operator skills that ship with the striatum repo but are **not** part of
the generated core bundle. The core skills (`striatum-scaffold`,
`striatum-workflow`, `striatum-supervise`, `striatum-recover`,
`striatum-claim-loop`, `striatum-mcp`) are rendered by
`striatum skills install` from embedded templates and cover run
mechanics; optional skills compose those mechanics into higher-level
operator workflows.

## Installing an optional skill

`striatum skills install` does not render these yet. Symlink (preferred —
updates with the checkout) or copy the skill directory into your agent's
skills directory:

```sh
# Claude Code, user scope (available in every repo):
ln -s "$(pwd)/skills/optional/refactoring-campaign" ~/.claude/skills/refactoring-campaign
```

Skill directories here follow the standard shape: a `SKILL.md` with
front-matter `name` and `description`, optional reference files, and
optional `scripts/`.

## Catalog

| Skill | What it drives |
|---|---|
| `optional/refactoring-campaign/` | The three-stage behavior-preserving refactoring campaign (`examples/refactoring-campaign/`): instantiate, run stage 0/1/2 in order, hand artifacts between stages, stop at refusal gates, integrate on accept. |

## Conventions

- Optional skills are operator tooling: they drive daemon verbs and the
  generated core skills; they must not advance workflow state by editing
  PostgreSQL or repository runner state directly.
- Keep skills generic to target repositories; campaign- or repo-specific
  state belongs in the target repo's `striatum/` tree, not in the skill.
- A skill that proves broadly useful is a candidate for promotion into
  the generated bundle (`go/pkg/installers/templates/skills/`) — that is
  a product change with its own decision.
