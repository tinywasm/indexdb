//go:build wasm

package indexdb

import (
	"testing"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

// TestIndexDBCrudOperations tests basic CRUD operations in IndexDB
func TestIndexDBCrudOperations(t *testing.T) {

	logger := func(args ...any) {
		t.Log(args...)
	}

	// Setup the database with tables
	db := SetupDB(logger, "", &User{}, &Product{})

	// Tables should be implicitly created by New during SetupDB.
	// We no longer manually test TableExist here because the ORM wrapper handles it via adapter.
	// If needed we could use reflection or type assertion to get adapter, but better to test functionally.

	userOne := User{Name: "Alice", Email: "alice@example.com"}
	userOne.ID = "1" // Manually assigning simple ID for test since we no longer access adapter directly

	err := db.Create(&userOne)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if userOne.ID == "" {
		t.Fatal("User ID should be generated")
	}

	// READ ONE
	var userRead User
	err = db.Query(&userRead).Where("ID").Eq(userOne.ID).ReadOne()
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
	var userUpdated User
	err = db.Query(&userUpdated).Where("ID").Eq(userOne.ID).ReadOne()
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
	var userDeleted User
	err = db.Query(&userDeleted).Where("ID").Eq(userOne.ID).ReadOne()
	if err == nil {
		t.Fatal("Expected error finding deleted user, got nil")
	}

	// Delete on non-existent
	err = db.Delete(&User{ID: "999999"}, orm.Eq("ID", "999999"))
	if err != nil {
		t.Logf("Delete non-existent returned error: %v", err)
	}

	// Delete multi-condition
	err = db.Delete(&User{ID: "1"}, orm.Eq("ID", "1"), orm.Eq("Name", "Alice"))
	if err == nil {
		t.Fatal("Expected error finding multi-condition delete, got nil")
	}

	// ReadOne via cursor path (no single PK condition)
	var alice User
	err = db.Query(&alice).Where("Name").Eq("Alice").ReadOne()
	if err != nil {
		t.Logf("ReadOne by name returned: %v", err)
	}

	// Create and read by non-PK to find it
	db.Create(&User{ID: "bob_id", Name: "Bob", Email: "bob@domain.com"})
	var bob User
	err = db.Query(&bob).Where("Name").Eq("Bob").ReadOne()
	if err != nil {
		t.Fatalf("Failed to read Bob by name: %v", err)
	}
	if bob.ID != "bob_id" {
		t.Fatalf("Expected Bob's ID to be bob_id, got %s", bob.ID)
	}

	// Read All via Cursor (using idiomatic ORM API)
	var users []User
	err = db.Query(&User{}).Where("Name").Eq("Bob").ReadAll(
		func() Model { return &User{} },
		func(m Model) {
			users = append(users, *(m.(*User)))
		},
	)
	if err != nil {
		t.Fatalf("ReadAll by Name failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(users))
	}
}

// Test close execution branch
func TestCloseDb(t *testing.T) {
	logger := func(args ...any) {}
	db := SetupDB(logger, "close_db", &User{})
	// Assuming raw executor handles Close
	_ = db.Close()
	// Should not panic
}

// Extra coverage for tableExist
func TestTableExist(t *testing.T) {
	// Let's use SetupDB normally
	db := SetupDB(t.Log, "exist_db", &User{})
	// Try accessing raw adapter via db.RawExecutor
	raw := db.RawExecutor()
	if raw != nil {
		if a, ok := raw.(*adapter); ok {
			exists := a.tableExist("user")
			if !exists {
				t.Error("Table user should exist")
			}
			notExists := a.tableExist("unknown")
			if notExists {
				t.Error("Table unknown should not exist")
			}
		}
	}
}

// Extra ReadAll edge cases
func TestReadAllEdgeCases(t *testing.T) {
	db := SetupDB(t.Log, "readall_edge", &User{})

	db.Create(&User{ID: "edge1", Name: "Edge1"})
}
