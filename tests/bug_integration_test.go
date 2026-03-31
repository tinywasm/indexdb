//go:build wasm

package tests_test

import (
	"testing"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/indexdb"
)

// SimpleUser implements the Model interface for testing
type SimpleUser struct {
	ID    string `db:"pk"`
	Email string `db:"unique"`
}

func (m *SimpleUser) ModelName() string { return "simple_users" }
func (m *SimpleUser) Schema() []Field {
	return []Field{
		{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}},
		{Name: "Email", Type: FieldText, DB: &FieldDB{Unique: true}},
	}
}
func (m *SimpleUser) Pointers() []any { return []any{&m.ID, &m.Email} }

// SimpleSession implements the Model interface for testing
type SimpleSession struct {
	ID     string `db:"pk"`
	UserID string `db:"ref=simple_users"`
}

func (m *SimpleSession) ModelName() string { return "simple_sessions" }
func (m *SimpleSession) Schema() []Field {
	return []Field{
		{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}},
		{Name: "UserID", Type: FieldText},
	}
}
func (m *SimpleSession) Pointers() []any { return []any{&m.ID, &m.UserID} }

// NumericPK implements the Model interface for testing
type NumericPK struct {
	ID    int64 `db:"pk"`
	Value string
}

func (m *NumericPK) ModelName() string { return "numeric_pks" }
func (m *NumericPK) Schema() []Field {
	return []Field{
		{Name: "ID", Type: FieldInt, DB: &FieldDB{PK: true}},
		{Name: "Value", Type: FieldText},
	}
}
func (m *NumericPK) Pointers() []any { return []any{&m.ID, &m.Value} }

func TestBugScenario(t *testing.T) {
	logger := func(args ...any) { t.Log(args...) }

	t.Run("MultipleInitialization", func(t *testing.T) {
		dbName := "multi_init_test"
		db1 := indexdb.New(dbName, nil, logger, &SimpleUser{})
		_ = db1

		// Initialize again
		db2 := indexdb.New(dbName, nil, logger, &SimpleUser{})
		if db2 == nil {
			t.Fatal("Second initialization failed")
		}
	})

	t.Run("WaitForSuccess", func(t *testing.T) {
		dbName := "wait_success_test"
		// This test is implicit because New blocks until initDone is closed.
		// If it doesn't block, subsequent operations would fail.
		db := indexdb.New(dbName, nil, logger, &SimpleUser{})

		err := db.Create(&SimpleUser{ID: "u1", Email: "u1@test.com"})
		if err != nil {
			t.Fatalf("Operation immediately after InitDB failed: %v", err)
		}
	})

	t.Run("TextPK", func(t *testing.T) {
		db := SetupDB(logger, "text_pk_test", &SimpleUser{})
		u := SimpleUser{ID: "alice", Email: "alice@test.com"}

		if err := db.Create(&u); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		var read SimpleUser
		err := db.Query(&read).Where("ID").Eq("alice").ReadOne()
		if err != nil {
			t.Fatalf("ReadOne failed: %v", err)
		}
		if read.Email != u.Email {
			t.Errorf("Expected email %s, got %s", u.Email, read.Email)
		}
	})

	t.Run("NumericPK", func(t *testing.T) {
		db := SetupDB(logger, "numeric_pk_test", &NumericPK{})
		n := NumericPK{ID: 123, Value: "test"}

		if err := db.Create(&n); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		var read NumericPK
		err := db.Query(&read).Where("ID").Eq(int64(123)).ReadOne()
		if err != nil {
			t.Fatalf("ReadOne failed: %v", err)
		}
		if read.ID != n.ID {
			t.Errorf("Expected ID %d, got %d", n.ID, read.ID)
		}
	})

	t.Run("TableNotFound", func(t *testing.T) {
		db := SetupDB(logger, "table_not_found_test", &SimpleUser{})

		var session SimpleSession
		err := db.Query(&session).Where("ID").Eq("s1").ReadOne()
		if err == nil {
			t.Fatal("Expected error for non-existent table, got nil")
		}
		t.Logf("Received expected error: %v", err)
	})

	t.Run("CursorConcurrency", func(t *testing.T) {
		db := SetupDB(logger, "concurrency_test", &SimpleUser{})

		// Seed some data
		for i := 0; i < 10; i++ {
			_ = db.Create(&SimpleUser{ID: string(rune('0' + i)), Email: "test@test.com"})
		}

		// Sequential rapid queries
		for i := 0; i < 5; i++ {
			var u SimpleUser
			err := db.Query(&u).Where("ID").Eq("0").ReadOne()
			if err != nil {
				t.Fatalf("Query %d failed: %v", i, err)
			}
		}
	})

	t.Run("EmptyResult", func(t *testing.T) {
		db := SetupDB(logger, "empty_result_test", &SimpleUser{})

		var u SimpleUser
		err := db.Query(&u).Where("ID").Eq("missing").ReadOne()
		if err == nil {
			t.Fatal("Expected error for missing record, got nil")
		}
		if err.Error() != "record not found" {
			t.Errorf("Expected 'record not found', got '%v'", err)
		}
	})
}
