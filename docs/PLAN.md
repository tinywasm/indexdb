# PLAN: Replace `defer/recover` with `Truthy`-style pre-check — `tinywasm/indexdb`

## Installation prerequisite

Before implementing or running tests, install the project test runner:

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

`gotest` runs WASM tests in a real browser — required because IndexedDB only
exists in browser environments.

---

## Context

`tinywasm/indexdb` is a WASM-only IndexedDB driver for `tinywasm/orm`. It targets
the WebAssembly platform and may be compiled with either:

- **Standard Go** (`GOOS=js GOARCH=wasm`) — full runtime, `recover()` works
- **TinyGo** (`tinygo build -target=wasm`) — small runtime, `recover()` is **NOT supported**

The current implementation in [`tx.go`](../tx.go) uses `defer + recover()` to
catch JS exceptions thrown by `IDBDatabase.transaction()`. This pattern is
**broken under TinyGo**, leading to a latent bug: under TinyGo the goroutine
will die instead of returning a clean error.

---

## The bug

[`tx.go:106-131`](../tx.go) — `getStore`:

```go
func (d *adapter) getStore(tableName string, mode string) (store js.Value, err error) {
    if !d.db.Truthy() {
        return js.Value{}, Err("Database not initialized")
    }

    // IndexedDB's transaction() method throws an exception if the object store doesn't exist.
    // In Go WebAssembly, JS exceptions cause a panic, so we must recover and return an error.
    defer func() {
        if r := recover(); r != nil {
            err = Errf("Object store %s not found: %v", tableName, r)
        }
    }()

    tx := d.db.Call("transaction", tableName, mode)
    // ...
}
```

### Evidence from official TinyGo documentation

> "The `recover` builtin is supported on most architectures, **with the notable
> exception of WebAssembly**."
>
> "On architectures where `recover` is not implemented, **a panic will always
> exit the program without running any deferred functions**."
>
> — [TinyGo Language Support](https://tinygo.org/docs/reference/lang-support/)

Tracking issue: [tinygo-org/tinygo#891 — implement recover](https://github.com/tinygo-org/tinygo/issues/891) (open since 2019).

### Impact

`getStore` is called from many sites in [`execute.go`](../execute.go):

```
execute.go:32   — Create
execute.go:51   — Update
execute.go:69   — Delete
execute.go:85   — ReadOne
execute.go:142  — ReadAll
```

Under TinyGo, calling any of those with a non-existent table name (typo,
schema mismatch, race condition during migration) **kills the WASM goroutine**
instead of returning the expected `error`. End user sees a frozen app instead
of a "table X not found" error.

---

## The fix: pre-check via `objectStoreNames.contains()`

`IDBDatabase.objectStoreNames` is a `DOMStringList` exposing `.contains(name)`.
Calling it never throws — it just returns `true`/`false`. Checking before
`transaction()` eliminates the exception path entirely.

### New `getStore` implementation

```go
func (d *adapter) getStore(tableName string, mode string) (js.Value, error) {
    if !d.db.Truthy() {
        return js.Value{}, Err("Database not initialized")
    }

    // Pre-check object store existence. Calling transaction() with an unknown
    // store throws a NotFoundError, which is unrecoverable under TinyGo wasm
    // (recover() not supported on this target — see tinygo.org/docs/reference/lang-support).
    storeNames := d.db.Get("objectStoreNames")
    if !storeNames.Truthy() || !storeNames.Call("contains", tableName).Bool() {
        return js.Value{}, Errf("Object store %s not found", tableName)
    }

    tx := d.db.Call("transaction", tableName, mode)
    if !tx.Truthy() {
        return js.Value{}, Errf("Failed to create transaction for table %s", tableName)
    }

    store := tx.Call("objectStore", tableName)
    if !store.Truthy() {
        return js.Value{}, Errf("Failed to get object store for table %s", tableName)
    }

    return store, nil
}
```

### Diff summary

```diff
 func (d *adapter) getStore(tableName string, mode string) (store js.Value, err error) {
     if !d.db.Truthy() {
         return js.Value{}, Err("Database not initialized")
     }

-    // IndexedDB's transaction() method throws an exception if the object store doesn't exist.
-    // In Go WebAssembly, JS exceptions cause a panic, so we must recover and return an error.
-    defer func() {
-        if r := recover(); r != nil {
-            err = Errf("Object store %s not found: %v", tableName, r)
-        }
-    }()
+    // Pre-check: objectStoreNames.contains() never throws. Avoids relying on
+    // recover() which is unsupported under TinyGo wasm.
+    storeNames := d.db.Get("objectStoreNames")
+    if !storeNames.Truthy() || !storeNames.Call("contains", tableName).Bool() {
+        return js.Value{}, Errf("Object store %s not found", tableName)
+    }

     tx := d.db.Call("transaction", tableName, mode)
     ...
 }
```

The named return values `(store js.Value, err error)` can be simplified back to
positional `(js.Value, error)` since the `defer` is gone — they were only named
to allow the deferred closure to mutate `err`.

---

## Why pre-check beats alternatives

| Approach | Works under TinyGo? | Notes |
|----------|---------------------|-------|
| **`objectStoreNames.contains()` pre-check** (chosen) | ✅ Yes | Standard IndexedDB API, no exception path, single extra JS call |
| `defer + recover()` (current) | ❌ No | Documented broken on wasm target |
| Inject JS try/catch wrapper | ✅ Yes | More plumbing, CSP concerns, overkill for a single call site |
| Iterate `objectStoreNames.Length()` and compare each | ✅ Yes | Already used in `tableExist` (adapter.go:405) but slower; `.contains()` is the canonical API |

Note on consistency: [`adapter.go:405-420`](../adapter.go) already has a
`tableExist` helper using manual iteration. After this fix, consider migrating
`tableExist` to also use `.contains()` for consistency and a small perf gain.
That refactor is **out of scope** for this PLAN — just flag it.

---

## Are there other `recover()` sites?

Audited the package:

```bash
grep -n "recover()" *.go
# tx.go:114 — the one this PLAN fixes
```

**Only one occurrence.** Other places that could theoretically panic from JS
exceptions:

| Call site | Panic risk | Mitigation already in place |
|-----------|-----------|-----------------------------|
| `req.Call("addEventListener", ...)` (tx.go:40,41,97,98) | Low | `req` already validated upstream |
| `d.db.Call("close")` (adapter.go:233) | None | `close` does not throw |
| `d.db.Call("createObjectStore", ...)` (adapter.go:393) | Possible if called outside upgrade tx | Already guarded by execution context (only called in `onupgradeneeded`) |
| `js.Global().Get("indexedDB").Call("open", ...)` (adapter.go:276) | None | Returns request, errors propagate via event listeners |
| `tx.Call("objectStore", tableName)` (tx.go:125) | None after pre-check | Pre-check guarantees the store exists |

No other defensive changes required.

---

## Tests required

All tests run in real browser via `gotest`. WASM-only build tag.

### `getstore_test.go` (new file in root, `//go:build wasm`, `package indexdb`)

`getStore` es un método privado — el test necesita `package indexdb` (acceso a internals),
por eso va en la raíz del paquete, no en `tests/`.

| Test | Verifies |
|------|----------|
| `TestGetStore_ExistingTable_Succeeds` | `getStore("users", "readonly")` after schema with "users" → returns valid `js.Value`, `err == nil` |
| `TestGetStore_MissingTable_ReturnsError` | `getStore("nonexistent", "readonly")` → returns `js.Value{}` and error message containing `"nonexistent"` and `"not found"` |
| `TestGetStore_NilDatabase_ReturnsError` | New adapter with `d.db = js.Value{}` → returns `"Database not initialized"` |
| `TestGetStore_NoCrash_OnMissingTable` | Critical regression test: calling `getStore` on missing table must NOT panic the goroutine. Use a sentinel goroutine that runs after the call and confirms it executes. |

### Existing test suites

Run the full existing test suite to confirm no regression:

```bash
gotest
```

`tx_test.go`, `execute_test.go`, etc. exercise `getStore` indirectly through
Create/ReadOne/ReadAll — they should all still pass.

---

## Checklist

Prerequisite: `go install github.com/tinywasm/devflow/cmd/gotest@latest`

- [ ] Edit [`tx.go:106-131`](../tx.go): replace `defer/recover` block with `objectStoreNames.contains()` pre-check
- [ ] Simplify named returns `(store js.Value, err error)` → `(js.Value, error)` (no longer needed for defer mutation)
- [ ] Add `getstore_test.go` (root, `package indexdb`) with the 4 tests above — private API requires same package
- [ ] Run `gotest` — full suite must pass with new tests included
- [ ] Update [`README.md`](../README.md) if the error message format for missing tables changed (currently `"Object store %s not found: %v"` → new `"Object store %s not found"`)
- [ ] (Optional, out of scope but flag in PR) Migrate `tableExist` in [`adapter.go:405-420`](../adapter.go) to also use `.contains()` for consistency
- [ ] `gopush 'fix(indexdb): replace recover with objectStoreNames.contains for TinyGo wasm compatibility'`

---

## Compatibility note for downstream consumers

The fix is **backward compatible**:

- Same function signature (positional return values are still `(js.Value, error)`)
- Same error semantics (returns error instead of panic — already the documented behavior, just now actually delivered under TinyGo)
- Error message changes from `"Object store X not found: <recovered panic>"` to `"Object store X not found"` — slightly cleaner, no semantic regression

No `gomod` changes required.
