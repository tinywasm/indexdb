//go:build js && wasm

package tests

import (
	"fmt"

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
	db := indexdb.InitDB(testDbName, idGen, logger, structTables...)

	return db
}

// User represents a sample struct for testing table creation
// We need to implement orm.Model interface for User now
type User struct {
	ID    string
	Name  string
	Email string
}

func (u User) StructName() string {
	return "user"
}

// ORM Model interface implementation
func (u *User) TableName() string { return "user" }
func (u *User) Columns() []string { return []string{"ID", "Name", "Email"} }
func (u *User) Values() []any     { return []any{u.ID, u.Name, u.Email} }
func (u *User) Pointers() []any   { return []any{&u.ID, &u.Name, &u.Email} }

// TestProduct represents another sample struct for testing
type Product struct {
	IDProduct string
	Name      string
	Price     float64
}

func (p Product) StructName() string {
	return "product"
}

// ORM Model interface implementation
func (p *Product) TableName() string { return "product" }
func (p *Product) Columns() []string { return []string{"IDProduct", "Name", "Price"} }
func (p *Product) Values() []any     { return []any{p.IDProduct, p.Name, p.Price} }
func (p *Product) Pointers() []any   { return []any{&p.IDProduct, &p.Name, &p.Price} }
