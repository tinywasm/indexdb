# IndexedDB Adapter Implementation

This document is the **Master Prompt (PLAN.md)** for refactoring the `indexdb` library to act as an Adapter for the `tinywasm/orm` ecosystem. Every execution agent must follow this plan sequentially.

---

## Development Rules

- **SRP:** Single purpose per file.
- **DI:** Import `github.com/tinywasm/orm` and implement `orm.Adapter`.
- **Flat Hierarchy:** Files in the root.
- **Test Organization:** Tests in `tests/` and `wasm_tests/`.
- **WASM/Stlib Dual Testing Pattern:** Must use build tags (`//go:build wasm` for WASM behavior).
- **WASM Restrictions:** No `database/sql`, no system calls. Strictly utilize `syscall/js`.
- **Frontend Go Compatibility:** Use standard library replacements for tinygo compatibility. Use `tinywasm/fmt` instead of `fmt`/`strings`/`strconv`/`errors`; `tinywasm/time` instead of `time`; and `tinywasm/json` instead of `encoding/json`.
- **Frontend Optimization:** **Avoid using `map`** declarations in WASM code to prevent binary bloat. Use structs or slices for small collections instead.
- **Testing (`gotest`):** Run WASM tests automatically using `gotest` (do NOT use standard `go test`). The CLI command handles `-vet`, `-race`, and WASM tests automatically. *Note for AI Agents:* if `gotest` is not globally installed on the environment, you must install it first via `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
- **Mocking & Assertions:** Standard `testing` lib only. **NEVER** use external assertion libraries (e.g., `testify`, `gomega`).

---

## Architecture Overview

IndexedDB operates drastically different than pure SQL engines. It is an object store manipulated exclusively via JavaScript APIs. The Adapter logically maps the `orm.Query` into `IDBObjectStore` native executions.

```go
type IndexDBAdapter struct {
    db js.Value
}

func (i *IndexDBAdapter) Execute(q orm.Query, m orm.Model, factory func() orm.Model, each func(orm.Model)) error
```

---

## Execution Phases

### Phase 1: Adapter Structure (`adapter.go`)
1. Refactor `IndexDB` struct natively into `IndexDBAdapter` that properly satisfies `orm.Adapter`.
2. Introduce `github.com/tinywasm/orm` definitions in `go.mod`.
3. Provide an explicit `InitDB` structure maintaining the current `js.Global().Get("indexedDB").Call("open", ...)` behavior.

### Phase 2: WASM JS Promises & Asynchrony (`promise.go` & `tx.go`)
1. Encapsulate the Promise yielding capability in robust, pure go-routine blocks leveraging channels for asynchronous completions awaiting IndexedDB `onsuccess`/`onerror` callbacks.

### Phase 3: Translation and Execution (`execute.go`)
Implement `Execute(q orm.Query, m orm.Model, factory func() orm.Model, each func(orm.Model)) error`:

1. **Write Operations (`ActionCreate`, `ActionUpdate`, `ActionDelete`)**:
   - Establish a `"readwrite"` transaction block directed at the store mapped via `q.Table`.
   - Iterate structurally mapping `q.Columns` and `q.Values` onto a conventional JavaScript Map Object `{ "key": "value" }` using explicit `js.ValueOf(map[string]any{...})` allocations.
   - Deploy `store.add()` or `store.put()` or `store.delete()` and explicitly await its resolution event.

2. **Read Single (`ActionReadOne`)**:
   - Assemble an `"readonly"` transaction request.
   - Utilize native `store.get(key)` functionality or filter explicitly via `IDBCursor` iterating `q.Conditions`.
   - Bind result parameters scanning directly to `m.Pointers()`.

3. **Read Many (`ActionReadAll`)**:
   - Frame a `"readonly"` iteration establishing a raw browser `IDBCursor`.
   - Filter rows against `q.Conditions` limits.
   - **CRITICAL:** When iterating `q.Conditions` to filter rows natively in JS, remember `orm.Condition` is a sealed type. Extract values using `.Field()`, `.Operator()`, `.Value()`, and `.Logic()` exclusively.
   - Trigger `factory()`, apply generic JS mappings manually onto `Pointers()`, and push the instance natively dispatching `each(m)` synchronously.

### Phase 4: Cleanup and Verification
1. Remove deprecated legacy codebase completely. This includes all files with stateful methods or standalone structural bindings. **You MUST DELELTE**:
   - `method-create.go`, `method-read.go`, `method-update.go`, `method-delete.go`, `method-clear.go`
   - `x-lab-read-sync.go`, `x-promise-await.go`, `x-read-promises.go`
   - `read-process-item.go`, `index-check.go`, `blob-url.go`
   - Internal state properties from `indexdb.go` like `transaction`, `store`, `cursor`, `result`, and `data` map arrays that violate the stateless Adapter pattern.
2. Write rigid `//go:build wasm` integration testing scenarios validating comprehensive CRUD maneuvers natively on the browser IndexedDB (headless evaluation handled automatically by standard `gotest`).
