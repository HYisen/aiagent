# Go Conventions

Idioms used in this project. Reviewers and AI tools should understand these
before flagging things as issues.

## Consumer Defines the Interface

In Go, the **consumer** (caller) defines the interface it needs, not the
implementer. A single concrete type can satisfy multiple interfaces, each
defined by a different package, with each interface containing only the
methods that particular consumer uses.

For example, `console.MultiLineRemote` satisfies both:

- `console.MultiLineChecker` — the subset needed by `console.Controller`
- `aiagent/tools/client/ui.MultiLineHelper` — the subset needed by
  `ui.Handler`

This is idiomatic Go. Do not flag it as duplication or inconsistency.
Consolidating them into one shared interface would violate interface
segregation and couple unrelated packages.

## Panic for Invariants

`panic` is used for invariant violations (fail-fast), not for ordinary
error handling. It is intentional — do not flag it as a crash bug.
The invariant is always accompanied by a comment explaining why the
condition must hold.
