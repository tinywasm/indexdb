//go:build js && wasm

package wasmtests_test

import (
	"testing"

	"github.com/tinywasm/indexdb/tests"
)

// TestIndexDBCrudOperations tests basic CRUD operations in IndexDB
func TestIndexDBCrudOperations(t *testing.T) {

	logger := func(args ...any) {
		t.Log(args...)
	}

	// Setup the database
	db := tests.SetupDB(logger)

	// add tables
	db.InitDB(tests.User{}, tests.Product{})

	if !db.TableExist("user") {
		t.Fatal("Table 'user' should exist")
	}

	if !db.TableExist("product") {
		t.Fatal("Table 'product' should exist")
	}

	// CREATE User without id expected id to be auto generated
	userOne := tests.User{Name: "Alice", Email: "alice@example.com"}
	err := db.Create("user", &userOne)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if userOne.ID == "" {
		t.Fatal("User ID should be auto-generated")
	}

	// UPDATE user
	userOne.Email = "alice@newdomain.com"
	err = db.Update("user", &userOne)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

}
