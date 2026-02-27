//go:build js && wasm

package tests_test

import (
	"testing"

	"github.com/tinywasm/indexdb/tests"
	"github.com/tinywasm/orm"
)

// TestCRUDOperations tests basic Create, Read, Update, Delete operations
func TestCRUDOperations(t *testing.T) {
	logger := func(args ...any) {
		t.Log(args...)
	}

	db := tests.SetupDB(logger, "test_crud_db", tests.User{})

	user := &tests.User{ID: "1", Name: "John Doe", Email: "john@example.com"}

	// Create
	err := db.Create(user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read One
	var readUser tests.User
	err = db.Query(&readUser).Where(orm.Eq("ID", "1")).ReadOne()
	if err != nil {
		t.Fatalf("ReadOne failed: %v", err)
	}
	if readUser.Name != "John Doe" {
		t.Errorf("Expected Name 'John Doe', got '%s'", readUser.Name)
	}

	// Update
	user.Name = "Jane Doe"
	err = db.Update(user, orm.Eq("ID", "1"))
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Read again to verify update
	var updatedUser tests.User
	err = db.Query(&updatedUser).Where(orm.Eq("ID", "1")).ReadOne()
	if err != nil {
		t.Fatalf("ReadOne after update failed: %v", err)
	}
	if updatedUser.Name != "Jane Doe" {
		t.Errorf("Expected Name 'Jane Doe', got '%s'", updatedUser.Name)
	}

	// Delete
	err = db.Delete(user, orm.Eq("ID", "1"))
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Read should fail
	var deletedUser tests.User
	err = db.Query(&deletedUser).Where(orm.Eq("ID", "1")).ReadOne()
	if err == nil {
		t.Fatal("Expected error reading deleted user, got nil")
	}
}

// TestQueryConstraints tests query operators
func TestQueryConstraints(t *testing.T) {
	logger := func(args ...any) {
		t.Log(args...)
	}

	db := tests.SetupDB(logger, "test_query_db", tests.Product{})

	p1 := &tests.Product{IDProduct: "p1", Name: "Apple", Price: 1.5}
	p2 := &tests.Product{IDProduct: "p2", Name: "Banana", Price: 2.0}
	p3 := &tests.Product{IDProduct: "p3", Name: "Cherry", Price: 3.0}

	db.Create(p1)
	db.Create(p2)
	db.Create(p3)

	// Test > Operator
	// Note: The adapter implementation for ReadAll iterates and filters in Go currently.
	// So we need to make sure checkCondition is correct.

	var expensiveProducts []tests.Product
	factory := func() orm.Model { return &tests.Product{} }
	each := func(m orm.Model) {
		expensiveProducts = append(expensiveProducts, *(m.(*tests.Product)))
	}

	err := db.Query(&tests.Product{}).Where(orm.Gt("Price", 1.8)).ReadAll(factory, each)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(expensiveProducts) != 2 {
		t.Errorf("Expected 2 products > 1.8, got %d", len(expensiveProducts))
	}
}

// TestEdgeCases tests update/delete on non-existent records
func TestEdgeCases(t *testing.T) {
	logger := func(args ...any) {
		t.Log(args...)
	}

	db := tests.SetupDB(logger, "test_edge_db", tests.User{})

	user := &tests.User{ID: "999", Name: "Ghost", Email: "ghost@example.com"}

	// Update non-existent
	// IndexedDB Put will insert if not exists (upsert) unless checking specifically?
	// The adapter uses `store.put()`, which IS an upsert operation.
	// If we want Update to fail if not found, we need to check existence or use `update` method on cursor?
	// Standard SQL UPDATE usually affects 0 rows if not found but returns success.
	// ORM behavior depends.
	// Let's assume `store.put` behavior (Upsert) is acceptable OR if we want strict update we need to change adapter.
	// Usually Update in ORMs implies existing record.
	// But `store.put` will create it if key doesn't exist.

	// Let's check what happens.
	err := db.Update(user, orm.Eq("ID", "999"))
	if err != nil {
		t.Logf("Update non-existent returned error: %v", err)
	} else {
		// If it succeeded, check if it created the record.
		var ghost tests.User
		err = db.Query(&ghost).Where(orm.Eq("ID", "999")).ReadOne()
		if err == nil {
			t.Log("Update on non-existent record acted as Upsert (created record)")
		} else {
			t.Log("Update on non-existent record succeeded but record not found (strange)")
		}
	}

	// Delete non-existent
	// IndexedDB delete returns success even if key not found.
	err = db.Delete(user, orm.Eq("ID", "999"))
	if err != nil {
		t.Errorf("Delete non-existent should usually succeed (idempotent), but got error: %v", err)
	}
}
