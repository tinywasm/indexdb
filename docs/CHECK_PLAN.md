# IndexedDB Adapter - Phase 2 (Refinements)

This master prompt continues from the previous integration round, refining the API according to the latest domain requirements of the `tinywasm/orm` ecosystem.

## Development Rules
- Constraints remain active: `gotest`, SRP, standard DI, pure stdlib testing, 500 lines limit, and flat hierarchy.
- **WASM Restrictions:** No `database/sql`, no system calls. Strictly utilize `syscall/js`.
- **Frontend Go Compatibility:** Use standard library replacements for tinygo compatibility. Use `tinywasm/fmt` instead of `fmt`/`strings`/`strconv`/`errors`; `tinywasm/time` instead of `time`; and `tinywasm/json` instead of `encoding/json`.
- **Frontend Optimization:** **Avoid using `map`** declarations in WASM code to prevent binary bloat. Use structs or slices for small collections instead.
- **Testing (`gotest`):** Run WASM tests automatically using `gotest` (do NOT use standard `go test`). The CLI command handles `-vet`, `-race`, and WASM tests automatically. 
- **Mocking & Assertions:** Standard `testing` lib only. **NEVER** use external assertion libraries (e.g., `testify`, `gomega`).

## Execution Steps

### 1. Direct ORM Injection (`indexdb.go` / `adapter.go`)
- The user expressed that having to manually wrap the adapter connection with `orm.New()` in the application code is tedious.
- Refactor the primary constructor or initialization function (e.g., `InitDB` or `New`) to **directly return an `*orm.DB`**.
- Example target signature: `func InitDB(dbName string, idg idGenerator, logger func(...any), structTables ...any) *orm.DB` (or an equivalent signature depending on your Phase 1 architectural decisions).
- The internal logic of the constructor will naturally configure the `IndexDBAdapter` instance (connecting to IndexedDB), but before returning to the user, it MUST wrap it by calling `orm.New(adapter)`, returning the ready-to-use ORM `*DB` wrapper.

### 2. Directory & Test Hygiene (`vendor/`, `js_example/`, and `tests/`)
- The repository incorrectly contains a `vendor/` folder. **DELETE IT completely.**
- The repository incorrectly contains a `js_example/` folder. **DELETE IT completely.**
- The repository incorrectly contains tests inside `wasm_tests/`. **MOVE all tests** to the standard `tests/` directory and ensure `wasm_tests/` is deleted.
- All integration tests testing the IndexedDB logic inside `tests/` MUST include the `//go:build wasm` build tag at the very top of the file to comply with the Dual Testing Pattern.

### 3. Tests & Verification
- Ensure all tests in `tests/` reflect the new constructor signature (they will receive `*orm.DB` directly from `indexdb.Init(...)` and therefore have immediate access to `db.Create()`, `db.Tx()`, etc.).
- Validate implementation fully with `gotest`. The `gotest` CLI will automatically detect the WASM build tags and execute the tests in the headless browser environment seamlessly.
