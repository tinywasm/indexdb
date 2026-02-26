# IndexedDB Adapter - Phase 3 (Coverage & Documentation)

This master prompt outlines the execution plan to increase test coverage of the `tinywasm/indexdb` adapter to >90% and fully document its API for seamless AI/LLM consumption.

## Requirements & Constraints
- **Testing Runner**: You MUST use the globally installed `gotest` CLI command. It automatically handles WASM tests, `-vet`, `-race`, and `-cover`. Simply execute `gotest` (no arguments) for the full suite, or `gotest -run TestName`. Do NOT use standard `go test`. If `gotest` is not installed, install it first using `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
- **Test Location & Tags**: All tests MUST be placed in the `tests/` directory. Since this is an IndexedDB adapter, all test files must include the `//go:build wasm` build tag at the top.
- **Constraints**: 500 lines limit, flat hierarchy for logic, SRP for files. DO NOT use external assertion libraries. Use the standard `testing` library only.
- **WASM Restrictions**: Strictly use `syscall/js`. No `database/sql` imports. Do not use maps on WASM, prefer slices and structs.

## Execution Steps

### 1. Create API Documentation (`docs/IMPLEMENTATION.md`)
- **Goal**: Document the finalized API so that any executing AI agent knows exactly how to consume the library.
- **Action**: Read the source code to understand the implementation details, and then create a new documentation file `docs/IMPLEMENTATION.md` outlining the usage.
- **Content Requirements**:
  - Explain the `InitDB` constructor pattern, highlighting that it directly returns an `*orm.DB`.
  - Provide a complete but concise usage snippet showing: 
    1. A model struct implementing the ORM interfaces.
    2. Initialization using `indexdb.InitDB`.
    3. A brief chain of ORM operations (`Create`, `Query().Where().ReadOne()`, `Update`, `Delete`).
  - Make sure to link this new documentation from `README.md` as an index.

### 2. Refactor of Adapter Name and Visibility
- **Goal**: Make the API idiomatic and easy to use. Internal elements should not be exported.
- **Action**: 
  - Rename `IndexDBAdapter` to an unexported `adapter` struct.
  - Rename `NewAdapter` to `New`.
  - Ensure all internal properties and methods not needed by the end user (e.g., `Initialize`) are unexported.

### 3. Comprehensive Test Expansion (Coverage > 90%)
- **Goal**: Escalate the current ~45% coverage to >90%.
- **Action**: Add new or expand existing integration tests within the `tests/` folder. Create files per domain if they grow too large (e.g. `tests/crud_test.go`, `tests/schema_test.go`).
- **Critical Paths to Cover**:
  - Validations upon `InitDB` with invalid models (e.g., missing interface methods).
  - Advanced CRUD edge cases (e.g., updating a non-existent ID, deleting a non-existent ID).
  - Query constraints and conditions, specifically if IndexedDB ranges are used.
  - Transactions logic (`db.Tx()`) handling and rollbacks if implemented.
  - Verification of `TableExist` behaviors and the underlying ObjectStore index creation.
- **Validation Check**: Run `gotest` repeatedly until the automated coverage report prints `✅ coverage: 9X.X%`.
