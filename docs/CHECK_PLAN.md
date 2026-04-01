# Plan: FieldDB Compatibility

## Depends on

- github.com/tinywasm/fmt with FieldDB support

## Problem

adapter.go accesses `f.PK`, `f.AutoInc`, `f.Unique` directly on `fmt.Field`. These fields moved to `fmt.FieldDB` struct behind `Field.DB *FieldDB` pointer.

Note: indexdb re-exports fmt types (`FieldText`, `Field`, etc.) so test files use `FieldText` not `fmt.FieldText`, but the struct shape is the same.

## Changes

### 1. adapter.go — use helper methods

| Line | Before | After |
|------|--------|-------|
| 372 | `if f.PK` | `if f.IsPK()` |
| 383 | `if f.AutoInc` | `if f.IsAutoInc()` |
| 399 | `"unique": f.Unique` | `"unique": f.IsUnique()` |

### 2. Test schema literals

**setup_internal_test.go** (2 literals):
```go
// Before
{Name: "ID", Type: FieldText, PK: true}

// After
{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}}
```

**mock_internal_test.go** (2 literals):
```go
// Before
{Name: "ID", Type: FieldText, PK: true}
{Name: "id", Type: FieldText, PK: true}

// After
{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}}
{Name: "id", Type: FieldText, DB: &FieldDB{PK: true}}
```

**tests/setup_test.go** (2 literals):
```go
// Before
{Name: "ID", Type: FieldText, PK: true}
{Name: "IDProduct", Type: FieldText, PK: true}

// After
{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}}
{Name: "IDProduct", Type: FieldText, DB: &FieldDB{PK: true}}
```

**tests/bug_integration_test.go** (3 literals):
```go
// Before
{Name: "ID", Type: FieldText, PK: true}
{Name: "Email", Type: FieldText, Unique: true}
{Name: "ID", Type: FieldInt, PK: true}

// After
{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}}
{Name: "Email", Type: FieldText, DB: &FieldDB{Unique: true}}
{Name: "ID", Type: FieldInt, DB: &FieldDB{PK: true}}
```

### 3. Verify FieldDB is re-exported

If indexdb re-exports fmt types, ensure `FieldDB` is also re-exported. Check the type alias file.

### 4. Update README.md

Line 38: `PK: true` → `DB: &indexdb.FieldDB{PK: true}` (and Unique on line 40).

### 5. Bump go.mod

Update `github.com/tinywasm/fmt` to version with FieldDB.

## Execution Order

1. Bump fmt dependency
2. Verify/add FieldDB re-export
3. Update adapter.go (3 lines)
4. Update test files (9 schema literals)
5. Update README.md (2 lines)
6. `go test ./...`
