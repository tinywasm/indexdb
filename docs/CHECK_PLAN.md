> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: indexdb — Migrate processRequest → jsvalue.AwaitRequest

## Context

`tinywasm/indexdb` (module `github.com/tinywasm/indexdb`) is the IndexedDB ORM adapter.
Its internal `processRequest` function in `tx.go` duplicates the goroutine+channel async
bridge pattern that now lives as `jsvalue.AwaitRequest` in `github.com/tinywasm/jsvalue`.

This plan replaces the local `processRequest` with `jsvalue.AwaitRequest` — a direct
drop-in since both share the same signature `(js.Value) (js.Value, error)`.

`processCursorRequest` is NOT moved — cursor iteration is IndexedDB-specific and has no
equivalent in the general jsvalue API.

**Prerequisite**: `github.com/tinywasm/jsvalue` must already export `AwaitRequest`.

## Goal

- Delete `processRequest` from `tx.go`.
- Replace all 4 call sites in `execute.go` with `jsvalue.AwaitRequest`.
- Add `require github.com/tinywasm/jsvalue` to `go.mod`.
- Zero behaviour change — pure refactor.

## TinyWasm Constraints (mandatory)

- No `import "errors"`, `"fmt"`, `"strings"` from stdlib — use `github.com/tinywasm/fmt`.
- All files that use `syscall/js` carry `//go:build wasm`.

## Changes

### `go.mod`

```
require github.com/tinywasm/jsvalue <version-with-AwaitRequest>
```

### `tx.go` — delete `processRequest`

Remove the entire `processRequest` function (lines from `func processRequest` to its closing
brace). Keep `processCursorRequest` and `getStore` unchanged.

### `execute.go` — replace call sites

| Line (approx) | Before | After |
|---|---|---|
| `create` | `_, err = processRequest(req)` | `_, err = jsvalue.AwaitRequest(req)` |
| `update` | `_, err = processRequest(req)` | `_, err = jsvalue.AwaitRequest(req)` |
| `delete` | `_, err = processRequest(req)` | `_, err = jsvalue.AwaitRequest(req)` |
| `readOne` | `result, err := processRequest(req)` | `result, err := jsvalue.AwaitRequest(req)` |

Add import `"github.com/tinywasm/jsvalue"` to `execute.go`.

## Stages

| # | Archivo | Acción |
|---|---|---|
| 1 | `indexdb/go.mod` | Agregar `github.com/tinywasm/jsvalue` |
| 2 | `indexdb/tx.go` | Eliminar función `processRequest` |
| 3 | `indexdb/execute.go` | Reemplazar 4 call sites + agregar import jsvalue |

## Verification

```bash
gotest
```

Sin regresiones. La suite existente cubre todos los call sites modificados.
