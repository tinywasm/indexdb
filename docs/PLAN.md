---
PLAN: "refactor: use tinywasm/await instead of jsvalue.AwaitRequest"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — stop calling `jsvalue.AwaitRequest`; use `tinywasm/await`

## Why

`execute.go` calls `jsvalue.AwaitRequest` six times. `jsvalue` is being cut
down to a pure JS↔Go codec (`https://github.com/tinywasm/jsvalue/blob/main/docs/PLAN.md`)
and no longer exports that function — it lives in
`https://github.com/tinywasm/await/blob/main/docs/PLAN.md` as `await.Request`,
byte-for-byte the same behaviour.

**Prerequisite: `github.com/tinywasm/await` must be released before this plan
starts.**

## What does NOT change

`tx.go`'s `processCursorRequest` looks like the same pattern —
`addEventListener("success", ...)` / `addEventListener("error", ...)` behind a
channel — but it is not a one-shot wait: `onSuccess` fires again every time
`cursor.Call("continue")` runs, once per row. `await.Request` is one-shot by
design (both listeners are removed after the first event). **Do not migrate
`processCursorRequest` to `await.Request` or `await.Event`** — it would need
to re-register a listener per row, which is slower and no clearer. It stays
exactly as it is.

`adapter.go`'s `initialize`/`onUpgradeNeeded`/`onOpenExistingDB` machinery is
a multi-event database-lifecycle state machine (`upgradeneeded` →
`complete`/`error`/`abort` on a transaction, plus a `sync.Once`-guarded
completion channel). It is not a one-shot request/response either. **Leave it
untouched.**

This plan's scope is exactly the six `jsvalue.AwaitRequest` call sites in
`execute.go` — nothing else in this repository.

## Changes

### 1. `go.mod`

```bash
go get github.com/tinywasm/await@latest
go mod tidy
```

`jsvalue` stays as a dependency: `execute.go:381` calls
`jsvalue.ScanValue`, a codec function unaffected by this plan.

### 2. `execute.go`

Replace the import and every call site:

```go
// before
import "github.com/tinywasm/jsvalue"
...
_, err = jsvalue.AwaitRequest(req)

// after
import "github.com/tinywasm/await"
...
_, err = await.Request(req)
```

All six occurrences (lines 49, 72, 110, 161, 189, 218 in the current file —
confirm line numbers against the working tree, they will have drifted)
change identically: `jsvalue.AwaitRequest` → `await.Request`. No other
argument or return handling changes; the function signature is identical.

## Acceptance checklist

```bash
grep -rn "jsvalue.AwaitRequest" .        # → empty
grep -n "await.Request" execute.go       # → 6 matches
grep -n "processCursorRequest" tx.go     # → unchanged, still present
GOOS=js GOARCH=wasm go build ./...
gotest -tinygo
```
