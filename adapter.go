//go:build js && wasm

package indexdb

import (
	"reflect"
	"sync"
	"syscall/js"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

// Ensure IndexDBAdapter satisfies orm.Adapter
var _ orm.Adapter = (*IndexDBAdapter)(nil)

type idGenerator interface {
	GetNewID() string
}

type structName interface {
	StructName() string
}

type IndexDBAdapter struct {
	dbName string
	db     js.Value
	tables []any
	logger func(...any)
	idGen  idGenerator

	initDone      chan struct{}
	initOnce      sync.Once
	initCompleted bool
}

// NewAdapter creates a new IndexDBAdapter.
func NewAdapter(dbName string, idg idGenerator, logger func(...any)) *IndexDBAdapter {
	if logger == nil {
		logger = func(args ...any) {
			Println(args...)
		}
	}

	return &IndexDBAdapter{
		dbName:   dbName,
		db:       js.Value{},
		idGen:    idg,
		logger:   logger,
		initDone: make(chan struct{}),
	}
}

// InitDB initializes the IndexedDB database and creates object stores based on the provided structs.
func (d *IndexDBAdapter) InitDB(structTables ...any) {
	d.tables = structTables

	// Open connection to IndexedDB
	req := js.Global().Get("indexedDB").Call("open", d.dbName)

	// Add event listeners
	req.Call("addEventListener", "error", js.FuncOf(d.onShowDbError))
	req.Call("addEventListener", "success", js.FuncOf(d.onOpenExistingDB))
	req.Call("addEventListener", "upgradeneeded", js.FuncOf(d.onUpgradeNeeded))

	// Wait until init is done
	<-d.initDone
}

func (d *IndexDBAdapter) open(p *js.Value, message string) error {
	d.db = p.Get("target").Get("result")

	if !d.db.Truthy() {
		return Err("error open", d.dbName, message)
	}
	return nil
}

func (d *IndexDBAdapter) onUpgradeNeeded(this js.Value, p []js.Value) any {
	// The event is fired on the request object, so 'this' is the request.
	// p[0] is the event object.

	// We need to set d.db before creating tables, as the connection is opened in upgrade needed transaction
	err := d.open(&p[0], "upgradeneeded")
	if err != nil {
		d.logger(err)
		return nil
	}

	for i, table := range d.tables {
		t, ok := table.(structName)
		if !ok {
			d.logger("error table", i, "does not implement structName interface (Name() string)")
			continue
		}

		err := d.createTable(t.StructName(), table)
		if err != nil {
			d.logger(err)
			continue
		}
	}

	// Wait for the version change transaction to complete
	transaction := p[0].Get("target").Get("transaction")
	transaction.Call("addEventListener", "complete", js.FuncOf(func(this js.Value, p []js.Value) any {
		d.initOnce.Do(func() { d.initCompleted = true; close(d.initDone) })
		return nil
	}))
	transaction.Call("addEventListener", "error", js.FuncOf(func(this js.Value, p []js.Value) any {
		d.logger("version change transaction error")
		d.initOnce.Do(func() { d.initCompleted = true; close(d.initDone) })
		return nil
	}))
	transaction.Call("addEventListener", "abort", js.FuncOf(func(this js.Value, p []js.Value) any {
		d.logger("version change transaction aborted")
		d.initOnce.Do(func() { d.initCompleted = true; close(d.initDone) })
		return nil
	}))

	return nil
}

func (d *IndexDBAdapter) onShowDbError(this js.Value, p []js.Value) any {
	d.logger("indexDB Error", p[0])
	return nil
}

func (d *IndexDBAdapter) onOpenExistingDB(this js.Value, p []js.Value) any {
	err := d.open(&p[0], "OPEN")
	if err != nil {
		d.logger("open existing db error:", err)
		return nil
	}

	if !d.initCompleted {
		d.logger("open existing db success")
	}

	d.initOnce.Do(func() { d.initCompleted = true; close(d.initDone) })
	return nil
}

// createTable creates a table for the given struct type.
// Adapted from table.go but for IndexDBAdapter.
func (d *IndexDBAdapter) createTable(tableName string, structType any) error {
	st := reflect.TypeOf(structType)
	if st.Kind() == reflect.Ptr {
		st = st.Elem()
	}

	if st.Kind() == reflect.Struct {
		if st.NumField() != 0 {
			// Find primary key field
			pk_name := ""
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				fieldName := f.Name
				// IDorPrimaryKey is from github.com/tinywasm/fmt
				_, isPK := IDorPrimaryKey(tableName, fieldName)
				if isPK {
					if pk_name != "" {
						return Err("multiple primary keys found in struct")
					}
					pk_name = fieldName
				}
			}
			if pk_name == "" {
				return Err("no primary key found in struct")
			}

			// Create object store
			newTable := d.db.Call("createObjectStore", tableName, map[string]interface{}{"keyPath": pk_name})

			// Create indexes for all fields except primary key
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				fieldName := f.Name

				// Skip primary key field (it's the keyPath)
				_, unique := IDorPrimaryKey(tableName, fieldName)

				// Create index for the field
				newTable.Call("createIndex", fieldName, fieldName, map[string]interface{}{"unique": unique})
			}
		}
	}
	return nil
}

// TableExist checks if a table exists in the database
func (d *IndexDBAdapter) TableExist(table_name string) bool {
	if !d.db.Truthy() {
		return false
	}
	// Get the list of object store names from the database
	objectStoreNames := d.db.Get("objectStoreNames")
	length := objectStoreNames.Length()

	// Iterate through the table names and check if the table already exists
	for i := 0; i < length; i++ {
		name := objectStoreNames.Index(i).String()
		if name == table_name {
			return true
		}
	}

	return false
}

// Helper to access the ID generator
func (d *IndexDBAdapter) GetNewID() string {
	if d.idGen != nil {
		return d.idGen.GetNewID()
	}
	return ""
}
