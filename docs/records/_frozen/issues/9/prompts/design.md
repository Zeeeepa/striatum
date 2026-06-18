
# Design Synthesis

Produce the design synthesis artifact for this workflow. Do not patch code.

## Read

- `docs/issues/9/SPEC.md`, `docs/issues/10/SPEC.md`, `docs/issues/11/SPEC.md`
- `docs/ROADMAP.md` and `docs/TODO.md`
- existing source/tests named by the issue specs
- relevant prior dogfood review artifacts referenced by the issue specs

Focus on Content-Type validation, Origin/Referer enforcement, override modal context validation, and recovery dry-run no-side-effect guarantees.

## Output

The synthesis must name the implementation approach, exact write scope,
tests to add or update, acceptance criteria, known security/regression
risks, and how the downstream reviewer should verify the result.

Use `striatum.synthesis.v1` front matter and the exact `author:` line from
the work packet.
