//go:build wasm

package tests_test

import (
	"fmt"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/indexdb"
	"github.com/tinywasm/orm"
)

// idGenerator implements the idGenerator interface for testing
type idGenerator struct {
	counter int
}

func (t *idGenerator) GetNewID() string {
	t.counter++
	return fmt.Sprintf("%d", t.counter) // Simple ID generation for tests
}

// SetupDB creates a new IndexDB instance for testing
// Now returns *orm.DB
func SetupDB(logger func(...any), dbName string, structTables ...any) *orm.DB {
	testDbName := "local_test_db"
	if dbName != "" {
		testDbName = dbName
	}

	// Create a test ID generator
	idGen := &idGenerator{}

	// Call the new primary constructor
	db := indexdb.New(testDbName, idGen, logger, structTables...)

	return db
}

// User represents a sample struct for testing table creation
type User struct {
	ID    string
	Name  string
	Email string
}

// ORM Model interface implementation
func (u *User) ModelName() string { return "user" }
func (u *User) Schema() []Field {
	return []Field{
		{Name: "ID", Type: FieldText, DB: &FieldDB{PK: true}},
		{Name: "Name", Type: FieldText},
		{Name: "Email", Type: FieldText},
	}
}
func (u *User) Values() []any   { return []any{u.ID, u.Name, u.Email} }
func (u *User) Pointers() []any { return []any{&u.ID, &u.Name, &u.Email} }

// TestProduct represents another sample struct for testing
type Product struct {
	IDProduct string
	Name      string
	Price     float64
}

// ORM Model interface implementation
func (p *Product) ModelName() string { return "product" }
func (p *Product) Schema() []Field {
	return []Field{
		{Name: "IDProduct", Type: FieldText, DB: &FieldDB{PK: true}},
		{Name: "Name", Type: FieldText},
		{Name: "Price", Type: FieldFloat},
	}
}
func (p *Product) Values() []any   { return []any{p.IDProduct, p.Name, p.Price} }
func (p *Product) Pointers() []any { return []any{&p.IDProduct, &p.Name, &p.Price} }
