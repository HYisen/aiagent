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

## Named Result Parameters as Documentation

Naming a result parameter purely for documentation is acceptable, even if the
body does not touch it. The name in the signature tells the reader what the
returned value represents.

When the named return is not used in the body, prefer a local variable over
assigning to the named return directly. A local variable has a shorter,
clearer life-cycle — the reader sees it declared and knows nothing mutates it
before or after the visible update site. A named return lives for the whole
function, which adds tracking overhead: the reader must scan backward and
forward to confirm no other assignment touches it.

```go
// OK: name documents the result, local variable keeps body self-contained.
func filterOldAndMapSessionIDs(m map[int]string) (sessionIDs []int) {
var ret []int
...
return ret
}
```

## Panic for Invariants

`panic` is used for invariant violations (fail-fast), not for ordinary
error handling. It is intentional — do not flag it as a crash bug.
The invariant is always accompanied by a comment explaining why the
condition must hold.
