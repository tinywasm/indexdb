//go:build js && wasm

package tests_test

import (
	"testing"

	"github.com/tinywasm/indexdb"
	"github.com/tinywasm/indexdb/tests"
)

// TestInitDBValidations tests InitDB validation logic.
// However, InitDB currently panics or logs errors rather than returning them directly for some cases,
// or returns a valid DB instance even if some tables fail.
// We need to verify behavior.
func TestInitDBValidations(t *testing.T) {
	logger := func(args ...any) {
		t.Log(args...)
	}

	// Case 1: Initialize with a valid struct
	db := tests.SetupDB(logger, "test_valid_db", tests.User{})
	if db == nil {
		t.Fatal("Expected valid DB instance, got nil")
	}

	// Case 2: Initialize with a struct missing StructName interface
	// This requires a new struct type that doesn't implement StructName
	type InvalidStruct struct {
		ID string
	}
	// We expect this to NOT panic, but log an error and skip the table.
	// Since we can't easily capture logs without mocking the logger (which we do),
	// we can check if the table exists afterwards (it shouldn't).

	// We need a way to check if table exists.
	// The adapter has TableExist method, but it's on the adapter, which is hidden behind orm.DB.
	// We can't access it directly unless we export it or use reflection.
	// But `orm.DB` doesn't expose the adapter.
	// So we can try to perform an operation on it, which should fail.

	// However, InitDB returns *orm.DB.
	// If we pass an invalid struct, the adapter's Initialize method logs "error table ... does not implement ...".
	// The DB instance is still returned.

	// Let's create a separate DB for this test to avoid conflicts
	dbInvalid := indexdb.InitDB("test_invalid_db", nil, logger, InvalidStruct{})
	if dbInvalid == nil {
		t.Fatal("Expected DB instance even with invalid table, got nil")
	}

	// Try to use the invalid table. It should fail.
	// We need to implement orm.Model for InvalidStruct to even pass it to Create/Query
	// But if it implemented orm.Model, it might satisfy some interfaces?
	// The requirement is `structName` interface: `StructName() string`.
	// Our InvalidStruct does not have it.

	// Wait, if we pass it to InitDB, it is `any`.
	// Inside Initialize: `t, ok := table.(structName)`
	// If !ok, it logs and continues.

	// So the object store is NOT created.
	// But we can't easily verify this without `TableExist` or trying to open a transaction on it.

	// To verify, we would need to try to run a query on a non-existent table.
	// But to run a query, we need a model that implements `orm.Model`.
	// If we make InvalidStruct implement `orm.Model` but NOT `structName`, we can test this.

	// But `orm.Model` doesn't require `StructName`.
	// So:
}

type InvalidModel struct {
	ID string
}

// Implement orm.Model but NOT structName
func (u *InvalidModel) TableName() string { return "invalid_model" } // This returns the table name for ORM
func (u *InvalidModel) Columns() []string { return []string{"ID"} }
func (u *InvalidModel) Values() []any     { return []any{u.ID} }
func (u *InvalidModel) Pointers() []any   { return []any{&u.ID} }

func TestInitDB_InvalidModel(t *testing.T) {
	logger := func(args ...any) {
		t.Log(args...)
	}

	// Initialize with a model that fails the `structName` interface check
	db := indexdb.InitDB("test_invalid_model_db", nil, logger, &InvalidModel{})

	// Try to insert. The adapter should try to open a transaction on "invalid_model".
	// Since it wasn't created (because it failed the check), this should fail.

	err := db.Create(&InvalidModel{ID: "1"})
	if err == nil {
		t.Fatal("Expected error creating record on non-existent table, got nil")
	}
	// The error should be related to "Failed to create transaction" or "NotFoundError" from IndexedDB
}

// TestTableExistVerification checks if tables are correctly created.
// Since we cannot access TableExist directly on the adapter (it is unexported type),
// we verify by successfully performing operations.
func TestTableExistVerification(t *testing.T) {
	logger := func(args ...any) {
		t.Log(args...)
	}

	db := tests.SetupDB(logger, "test_table_exist_db", tests.Product{})

	// If table exists, we can write to it.
	p := tests.Product{IDProduct: "p1", Name: "Test Product", Price: 10.0}
	err := db.Create(&p)
	if err != nil {
		t.Fatalf("Failed to create product, implying table might not exist: %v", err)
	}
}
