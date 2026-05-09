//go:build wasm

package indexdb

import (
	"strings"
	"syscall/js"
	"testing"
)

func TestGetStore_ExistingTable_Succeeds(t *testing.T) {
	dbName := "test_getstore_existing"
	// Ensure a fresh database for this test
	js.Global().Get("indexedDB").Call("deleteDatabase", dbName)

	db := SetupDB(nil, dbName, &User{})
	defer db.Close()

	// Get the adapter from the orm.DB
	// orm.DB exposes RawExecutor() which returns our adapter
	adapter := db.RawExecutor().(*adapter)

	store, err := adapter.getStore("user", "readonly")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !store.Truthy() {
		t.Fatal("Expected store to be truthy")
	}
}

func TestGetStore_MissingTable_ReturnsError(t *testing.T) {
	dbName := "test_getstore_missing"
	// Ensure a fresh database for this test
	js.Global().Get("indexedDB").Call("deleteDatabase", dbName)

	db := SetupDB(nil, dbName, &User{})
	defer db.Close()

	adapter := db.RawExecutor().(*adapter)

	store, err := adapter.getStore("nonexistent", "readonly")
	if err == nil {
		t.Fatal("Expected error for missing table, got nil")
	}

	// Case-insensitive check because tinywasm/fmt might translate words
	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "nonexistent") || !strings.Contains(errMsg, "not found") {
		t.Errorf("Expected error message to contain 'nonexistent' and 'not found', got: %v", err.Error())
	}

	if store.Truthy() {
		t.Fatal("Expected store to be falsy")
	}
}

func TestGetStore_NilDatabase_ReturnsError(t *testing.T) {
	d := &adapter{db: js.Value{}}
	store, err := d.getStore("user", "readonly")
	if err == nil {
		t.Fatal("Expected error for nil database, got nil")
	}

	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "database") || !strings.Contains(errMsg, "not") || !strings.Contains(errMsg, "initialized") {
		t.Errorf("Expected 'Database not initialized', got: %v", err.Error())
	}

	if store.Truthy() {
		t.Fatal("Expected store to be falsy")
	}
}

func TestGetStore_NoCrash_OnMissingTable(t *testing.T) {
	dbName := "test_getstore_nocrash"
	// Ensure a fresh database for this test
	js.Global().Get("indexedDB").Call("deleteDatabase", dbName)

	db := SetupDB(nil, dbName, &User{})
	defer db.Close()

	adapter := db.RawExecutor().(*adapter)

	// We want to ensure that calling getStore on a missing table doesn't panic.
	// Since we are not in TinyGo, recover() would work, but the goal is to NOT
	// reach the point of exception.

	executedAfter := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("getStore panicked: %v", r)
			}
		}()
		_, _ = adapter.getStore("really_not_there", "readonly")
		executedAfter = true
	}()

	if !executedAfter {
		t.Fatal("Code after getStore was not executed, possibly due to a panic")
	}
}
