//go:build js && wasm

package tests_test

import (
	"testing"

	"github.com/tinywasm/indexdb/tests"
	"github.com/tinywasm/orm"
)

// TestIndexDBCrudOperations tests basic CRUD operations in IndexDB
func TestIndexDBCrudOperations(t *testing.T) {

	logger := func(args ...any) {
		t.Log(args...)
	}

	// Setup the database
	db, adapter := tests.SetupDB(logger)

	// add tables
	// Initialize is on the adapter now (renamed from InitDB)
	adapter.Initialize(tests.User{}, tests.Product{})

	if !adapter.TableExist("user") {
		t.Fatal("Table 'user' should exist")
	}

	if !adapter.TableExist("product") {
		t.Fatal("Table 'product' should exist")
	}

	// CREATE User without id expected id to be auto generated
	// BUT orm.Create expects the ID to be set IF it's not auto-increment by DB.
	// IndexedDB can do auto-increment if keyPath is set and autoIncrement: true.
	// Our adapter creation logic sets keyPath but not autoIncrement explicitly in createTable?
	// `createObjectStore`, `map[string]interface{}{"keyPath": pk_name}` -> autoIncrement defaults to false.
	// So we need to generate ID manually or update createTable.
	// The legacy code had:
	/*
		if !id_exist || id.(string) == "" {
			id = d.GetNewID()
			data[pk_field] = id
		}
	*/
	// We should probably replicate this ID generation logic in `create` method of adapter or rely on caller?
	// The legacy code did it inside `Create`.
	// Let's manually set ID for now as we are using ORM.
	// OR we can make the adapter handle it if empty?
	// The `GetNewID` helper is on adapter.

	userOne := tests.User{Name: "Alice", Email: "alice@example.com"}
	userOne.ID = adapter.GetNewID() // Manually assigning ID using adapter's generator

	err := db.Create(&userOne)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if userOne.ID == "" {
		t.Fatal("User ID should be generated")
	}

	// READ ONE
	var userRead tests.User
	err = db.Query(&userRead).Where(orm.Eq("ID", userOne.ID)).ReadOne()
	if err != nil {
		t.Fatalf("Failed to read user: %v", err)
	}
	if userRead.Name != userOne.Name {
		t.Fatalf("Expected name %s, got %s", userOne.Name, userRead.Name)
	}

	// UPDATE user
	userOne.Email = "alice@newdomain.com"
	err = db.Update(&userOne, orm.Eq("ID", userOne.ID))
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify Update
	var userUpdated tests.User
	err = db.Query(&userUpdated).Where(orm.Eq("ID", userOne.ID)).ReadOne()
	if err != nil {
		t.Fatalf("Failed to read updated user: %v", err)
	}
	if userUpdated.Email != "alice@newdomain.com" {
		t.Fatalf("Expected email alice@newdomain.com, got %s", userUpdated.Email)
	}

	// DELETE
	err = db.Delete(&userOne, orm.Eq("ID", userOne.ID))
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify Delete
	var userDeleted tests.User
	err = db.Query(&userDeleted).Where(orm.Eq("ID", userOne.ID)).ReadOne()
	if err == nil {
		t.Fatal("Expected error finding deleted user, got nil")
	}
}
