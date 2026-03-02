# Implementation Plan: IndexDB Bug-Hunting Integration Tests

## Development Rules
- **Prerequisites:** External agents must install `gotest` first:
  ```bash
  go install github.com/tinywasm/devflow/cmd/gotest@latest
  ```
- **Standard Library Only:** NEVER use external assertion libraries (e.g., testify, gomega). Use only the standard testing and reflect APIs.
- **Testing Runner (gotest):** For Go tests, ALWAYS use the globally installed `gotest` CLI command. DO NOT use `go test` directly. Simply type `gotest` (no arguments) for the full suite.
- **WASM Compatibility:** Use `tinywasm/fmt` instead of `fmt`/`strings`/`strconv`/`errors`.
- **Single Responsibility Principle:** Every file must have a single purpose.

## Goal
Improve `indexdb` robustness by implementing targeted integration tests designed to find common bugs in IndexedDB adapters, specifically focusing on initialization, primary key handling, and asynchronous event resolution.

## Proposed Changes

### [Component] Tests
#### [NEW] [bug_integration_test.go](tests/bug_integration_test.go)
- Implement minimal models to reproduce potential edge cases:
  ```go
  type SimpleUser struct {
      ID    string `db:"pk"`
      Email string `db:"unique"`
  }

  type SimpleSession struct {
      ID     string `db:"pk"`
      UserID string `db:"ref=simple_users"` // Check if reference breaks anything
  }

  type NumericPK struct {
      ID    int64  `db:"pk"`
      Value string
  }
  ```
- **Test cases to implement:**
  1. **Multiple Initialization:** Call `InitDB` twice on the same database and verify it doesn't crash or duplicate stores.
  2. **Wait for Success:** Ensure that `Initialize` correctly blocks until the "complete" or "success" event is fired.
  3. **TEXT Primary Key:** Verify that `ReadOne` works correctly with a `string` ID (both via `get` optimization and cursor fallback).
  4. **NUMERIC Primary Key:** Verify that `ReadOne` handles `int64` PK correctly.
  5. **Table Not Found:** Trigger an operation on a non-existent table and verify it returns a clean Go error instead of a JS panic.
  6. **Cursor Concurrency:** Simulate rapid sequential queries to see if asynchronous events overlap or cause race conditions in the `done` channels.
  7. **Empty Result Behavior:** Verify `ReadOne` returns `Err("record not found")` consistently when no match exists.

### [Component] Adapter Logic
#### [MODIFY] [adapter.go](adapter.go) / [execute.go](execute.go)
- If bugs are found during testing, apply fixes immediately following the "Smallest Change" principle.

## Verification Plan

### Automated Tests
- Run the full suite using `gotest`:
```bash
gotest
```
- Focus on the bug integration test:
```bash
gotest -run TestBugScenario
```
