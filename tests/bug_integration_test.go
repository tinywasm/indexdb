//go:build wasm

package tests_test

import (
	"fmt"
	"testing"
	"github.com/tinywasm/indexdb"
	"github.com/tinywasm/orm"
)

type SimpleUser struct {
	ID    string
	Email string
}

func (m *SimpleUser) TableName() string { return "simple_users" }
func (m *SimpleUser) Schema() []orm.Field {
	return []orm.Field{
		{Name: "ID", Type: orm.TypeText, Constraints: orm.ConstraintPK},
		{Name: "Email", Type: orm.TypeText, Constraints: orm.ConstraintUnique},
	}
}
func (m *SimpleUser) Values() []any   { return []any{m.ID, m.Email} }
func (m *SimpleUser) Pointers() []any { return []any{&m.ID, &m.Email} }

type SimpleSession struct {
	ID     string
	UserID string
}

func (m *SimpleSession) TableName() string { return "simple_sessions" }
func (m *SimpleSession) Schema() []orm.Field {
	return []orm.Field{
		{Name: "ID", Type: orm.TypeText, Constraints: orm.ConstraintPK},
		{Name: "UserID", Type: orm.TypeText},
	}
}
func (m *SimpleSession) Values() []any   { return []any{m.ID, m.UserID} }
func (m *SimpleSession) Pointers() []any { return []any{&m.ID, &m.UserID} }

type NumericPK struct {
	ID    int64
	Value string
}

func (m *NumericPK) TableName() string { return "numeric_pks" }
func (m *NumericPK) Schema() []orm.Field {
	return []orm.Field{
		{Name: "ID", Type: orm.TypeInt64, Constraints: orm.ConstraintPK},
		{Name: "Value", Type: orm.TypeText},
	}
}
func (m *NumericPK) Values() []any   { return []any{m.ID, m.Value} }
func (m *NumericPK) Pointers() []any { return []any{&m.ID, &m.Value} }

type NonExistent struct {
	ID string
}

// Implement methods for NonExistent just to satisfy orm.Model interface in case adapter panics earlier
func (m *NonExistent) TableName() string { return "non_existent_table" }
func (m *NonExistent) Schema() []orm.Field {
	return []orm.Field{{Name: "ID", Type: orm.TypeText, Constraints: orm.ConstraintPK}}
}
func (m *NonExistent) Values() []any   { return []any{m.ID} }
func (m *NonExistent) Pointers() []any { return []any{&m.ID} }

func TestBugScenario(t *testing.T) {
	logger := func(args ...any) { t.Log(args...) }
	dbName := "bug_integration_test_db"

	// 1. & 2. Multiple Initialization & Wait for Success
	db1 := indexdb.InitDB(dbName, nil, logger, &SimpleUser{}, &SimpleSession{}, &NumericPK{})
	if db1 == nil {
		t.Fatal("First InitDB returned nil")
	}

	db2 := indexdb.InitDB(dbName, nil, logger, &SimpleUser{}, &SimpleSession{}, &NumericPK{})
	if db2 == nil {
		t.Fatal("Second InitDB returned nil")
	}

	// 3. TEXT Primary Key
	user := SimpleUser{ID: "user_1", Email: "test@example.com"}
	err := db1.Create(&user)
	if err != nil {
		t.Fatalf("Failed to create user with text PK: %v", err)
	}

	var readUser SimpleUser
	err = db1.Query(&readUser).Where("ID").Eq("user_1").ReadOne()
	if err != nil {
		t.Fatalf("Failed to ReadOne user with text PK: %v", err)
	}
	if readUser.Email != "test@example.com" {
		t.Fatalf("Expected email test@example.com, got %s", readUser.Email)
	}

	// 4. NUMERIC Primary Key
	numEntry := NumericPK{ID: 42, Value: "Answer"}
	err = db1.Create(&numEntry)
	if err != nil {
		t.Fatalf("Failed to create entry with numeric PK: %v", err)
	}

	var readNum NumericPK
	err = db1.Query(&readNum).Where("ID").Eq(int64(42)).ReadOne()
	if err != nil {
		t.Fatalf("Failed to ReadOne entry with numeric PK: %v", err)
	}
	if readNum.Value != "Answer" {
		t.Fatalf("Expected value Answer, got %s", readNum.Value)
	}

	// 5. Table Not Found
	err = db1.Create(&NonExistent{ID: "ghost"})
	if err == nil {
		t.Fatal("Expected error when creating on a non-existent table, got nil")
	} else if err.Error() == "" {
		t.Fatal("Expected a non-empty error message")
	}

	// 6. Cursor Concurrency
	for i := 0; i < 10; i++ {
		u := SimpleUser{ID: fmt.Sprintf("concur_%d", i), Email: fmt.Sprintf("c%d@test.com", i)}
		err := db1.Create(&u)
		if err != nil {
			t.Fatalf("Failed to create concurrent user %d: %v", i, err)
		}
	}

	// Read them back quickly
	for i := 0; i < 10; i++ {
		var ru SimpleUser
		err := db1.Query(&ru).Where("ID").Eq(fmt.Sprintf("concur_%d", i)).ReadOne()
		if err != nil {
			t.Fatalf("Failed to read back concurrent user %d: %v", i, err)
		}
		if ru.Email != fmt.Sprintf("c%d@test.com", i) {
			t.Fatalf("Mismatch on concurrent read %d", i)
		}
	}

	// 7. Empty Result Behavior
	var missingUser SimpleUser
	err = db1.Query(&missingUser).Where("ID").Eq("not_found_user").ReadOne()
	if err == nil {
		t.Fatal("Expected error when reading non-existent record, got nil")
	} else if err.Error() != "record not found" {
		t.Fatalf("Expected 'record not found' error, got: %v", err)
	}
}
