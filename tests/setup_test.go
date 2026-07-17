//go:build wasm

package tests_test

import (
	"fmt"

	"github.com/tinywasm/indexdb"
	. "github.com/tinywasm/model"
	"github.com/tinywasm/storage"
)

// idGenerator implements the idGenerator interface for testing
type idGenerator struct {
	counter int
}

func (t *idGenerator) NewID() string {
	t.counter++
	return fmt.Sprintf("%d", t.counter) // Simple ID generation for tests
}

// SetupDB creates a new IndexDB instance for testing
func SetupDB(logger func(...any), dbName string, structTables ...any) storage.Conn {
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
		{Name: "ID", Type: Text(), DB: &FieldDB{PK: true}},
		{Name: "Name", Type: Text()},
		{Name: "Email", Type: Text()},
	}
}
func (u *User) Values() []any                 { return []any{u.ID, u.Name, u.Email} }
func (u *User) Pointers() []any               { return []any{&u.ID, &u.Name, &u.Email} }
func (u *User) EncodeFields(wr FieldWriter)   {}
func (u *User) DecodeFields(r FieldReader)    {}
func (u *User) IsNil() bool                  { return u == nil }

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
		{Name: "IDProduct", Type: Text(), DB: &FieldDB{PK: true}},
		{Name: "Name", Type: Text()},
		{Name: "Price", Type: Float()},
	}
}
func (p *Product) Values() []any                 { return []any{p.IDProduct, p.Name, p.Price} }
func (p *Product) Pointers() []any               { return []any{&p.IDProduct, &p.Name, &p.Price} }
func (p *Product) EncodeFields(wr FieldWriter)   {}
func (p *Product) DecodeFields(r FieldReader)    {}
func (p *Product) IsNil() bool                  { return p == nil }
