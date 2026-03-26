//go:build wasm

package indexdb

import (
	"sync"
	"syscall/js"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

type idGenerator interface {
	GetNewID() string
}

type adapter struct {
	dbName string
	db     js.Value
	tables []any
	logger func(...any)
	idGen  idGenerator

	initDone      chan struct{}
	initOnce      sync.Once
	initCompleted bool
}

// Exec implements orm.Executor
func (d *adapter) Exec(query string, args ...any) error {
	if len(args) == 0 {
		return Err("no query passed")
	}
	q, ok := args[0].(orm.Query)
	if !ok {
		return Err("invalid query type")
	}
	if len(args) < 2 {
		return Err("missing model argument")
	}
	m, ok := args[1].(Model)
	if !ok {
		return Err("invalid model type")
	}

	return d.execute(q, m, nil, nil, nil)
}

// simpleScanner implements orm.Scanner
type simpleScanner struct {
	err  error
	ptrs []any
}

func (s *simpleScanner) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	return nil
}

// QueryRow implements orm.Executor
func (d *adapter) QueryRow(query string, args ...any) orm.Scanner {
	if len(args) == 0 {
		return &simpleScanner{err: Err("no query passed")}
	}
	q, ok := args[0].(orm.Query)
	if !ok {
		return &simpleScanner{err: Err("invalid query type")}
	}
	if len(args) < 2 {
		return &simpleScanner{err: Err("missing model argument")}
	}
	m, ok := args[1].(Model)
	if !ok {
		return &simpleScanner{err: Err("invalid model type")}
	}

	err := d.execute(q, m, nil, nil, nil)
	return &simpleScanner{err: err}
}

// simpleRows implements orm.Rows
type simpleRows struct {
	models []Model
	values []js.Value
	fields []Field
	idx    int
}

func (r *simpleRows) Next() bool {
	count := len(r.models)
	if count == 0 {
		count = len(r.values)
	}
	if r.idx < count {
		r.idx++
		return true
	}
	return false
}

func (r *simpleRows) Scan(dest ...any) error {
	if r.idx == 0 || (r.idx > len(r.models) && r.idx > len(r.values)) {
		return Err("invalid row cursor")
	}

	if len(r.models) > 0 {
		m := r.models[r.idx-1]
		ptrs := m.Pointers()
		if len(ptrs) != len(dest) {
			return Err("scan destination mismatch")
		}

		for i, p := range ptrs {
			destPtr := dest[i]
			srcPtr := p

			switch d := destPtr.(type) {
			case *string:
				*d = *(srcPtr.(*string))
			case *int:
				*d = *(srcPtr.(*int))
			case *int64:
				*d = *(srcPtr.(*int64))
			case *float64:
				*d = *(srcPtr.(*float64))
			case *bool:
				*d = *(srcPtr.(*bool))
			case *any:
				*d = *(srcPtr.(*any))
			}
		}
		return nil
	}

	val := r.values[r.idx-1]
	if len(r.fields) != len(dest) {
		return Err("scan destination mismatch with fields")
	}

	for i, field := range r.fields {
		jsVal := val.Get(field.Name)
		if jsVal.IsUndefined() {
			continue
		}

		destPtr := dest[i]
		switch field.Type {
		case FieldText:
			if p, ok := destPtr.(*string); ok {
				*p = jsVal.String()
			}
		case FieldInt:
			if p, ok := destPtr.(*int64); ok {
				*p = int64(jsVal.Int())
			} else if p, ok := destPtr.(*int); ok {
				*p = jsVal.Int()
			}
		case FieldFloat:
			if p, ok := destPtr.(*float64); ok {
				*p = jsVal.Float()
			}
		case FieldBool:
			if p, ok := destPtr.(*bool); ok {
				*p = jsVal.Bool()
			}
		}
	}

	return nil
}

func (r *simpleRows) Close() error { return nil }
func (r *simpleRows) Err() error   { return nil }

// Query implements orm.Executor
func (d *adapter) Query(query string, args ...any) (orm.Rows, error) {
	if len(args) == 0 {
		return nil, Err("no query passed")
	}
	q, ok := args[0].(orm.Query)
	if !ok {
		return nil, Err("invalid query type")
	}
	if len(args) < 2 {
		return nil, Err("missing model argument")
	}
	m, ok := args[1].(Model)
	if !ok {
		return nil, Err("invalid model type")
	}

	var models []Model
	var values []js.Value
	var factory func() Model

	if len(args) > 2 {
		if f, ok := args[2].(func() Model); ok {
			factory = f
		}
	}

	var each func(Model)
	var eachJS func(js.Value)

	if factory != nil {
		each = func(model Model) {
			models = append(models, model)
		}
	} else {
		eachJS = func(val js.Value) {
			values = append(values, val)
		}
	}

	err := d.execute(q, m, factory, each, eachJS)
	if err != nil {
		return nil, err
	}

	return &simpleRows{
		models: models,
		values: values,
		fields: m.Schema(),
		idx:    0,
	}, nil
}

// Close implements orm.Executor
func (d *adapter) Close() error {
	if d.db.Truthy() {
		d.db.Call("close")
	}
	return nil
}

// newAdapter creates a new adapter.
func newAdapter(dbName string, idg idGenerator, logger func(...any)) *adapter {
	if logger == nil {
		logger = func(args ...any) {
			Println(args...)
		}
	}

	return &adapter{
		dbName:   dbName,
		db:       js.Value{},
		idGen:    idg,
		logger:   logger,
		initDone: make(chan struct{}),
	}
}

// Compiler converts ORM queries into engine instructions.
type compiler struct{}

func (c *compiler) Compile(q orm.Query, m Model) (orm.Plan, error) {
	// Our adapter executes queries directly. We can pass the query and model as args in the plan.
	return orm.Plan{Mode: q.Action, Query: "", Args: []any{q, m}}, nil
}

// New initializes the IndexedDB database and returns an *orm.DB instance.
func New(dbName string, idg idGenerator, logger func(...any), structTables ...any) *orm.DB {
	adapter := newAdapter(dbName, idg, logger)
	adapter.initialize(structTables...)
	compiler := &compiler{}
	return orm.New(adapter, compiler)
}

// initialize initializes the IndexedDB database and creates object stores based on the provided structs.
func (d *adapter) initialize(structTables ...any) {
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

func (d *adapter) open(p *js.Value, message string) error {
	d.db = p.Get("target").Get("result")

	if !d.db.Truthy() {
		return Err("error open", d.dbName, message)
	}
	return nil
}

func (d *adapter) onUpgradeNeeded(this js.Value, p []js.Value) any {
	// The event is fired on the request object, so 'this' is the request.
	// p[0] is the event object.

	// We need to set d.db before creating tables, as the connection is opened in upgrade needed transaction
	err := d.open(&p[0], "upgradeneeded")
	if err != nil {
		d.logger(err)
		return nil
	}

	for i, table := range d.tables {
		m, ok := table.(Model)
		if !ok {
			d.logger("table", i, "does not implement Model interface, skipping")
			continue
		}

		err := d.createTable(m)
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

func (d *adapter) onShowDbError(this js.Value, p []js.Value) any {
	d.logger("indexDB Error", p[0])
	return nil
}

func (d *adapter) onOpenExistingDB(this js.Value, p []js.Value) any {
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

// createTable creates an IndexedDB object store from the model's Schema.
func (d *adapter) createTable(m Model) error {
	if d.initCompleted {
		return Err("Dynamic table creation after initialization is not supported in IndexedDB adapter")
	}

	fields := m.Schema()
	tableName := m.ModelName()

	pkName := ""
	for _, f := range fields {
		if f.PK {
			pkName = f.Name
			break
		}
	}
	if pkName == "" {
		return Err("no primary key found in schema for table", tableName)
	}

	autoIncrement := false
	for _, f := range fields {
		if f.AutoInc {
			autoIncrement = true
			break
		}
	}

	opts := map[string]interface{}{"keyPath": pkName}
	if autoIncrement {
		opts["autoIncrement"] = true
	}
	newStore := d.db.Call("createObjectStore", tableName, opts)

	for _, f := range fields {
		if f.Name == pkName {
			continue
		}
		newStore.Call("createIndex", f.Name, f.Name, map[string]interface{}{"unique": f.Unique})
	}
	return nil
}

// tableExist checks if a table exists in the database
func (d *adapter) tableExist(tableName string) bool {
	if !d.db.Truthy() {
		return false
	}
	// Get the list of object store names from the database
	objectStoreNames := d.db.Get("objectStoreNames")
	length := objectStoreNames.Length()

	// Iterate through the table names and check if the table already exists
	for i := 0; i < length; i++ {
		name := objectStoreNames.Index(i).String()
		if name == tableName {
			return true
		}
	}

	return false
}

// getNewID helper to access the ID generator
func (d *adapter) getNewID() string {
	if d.idGen != nil {
		return d.idGen.GetNewID()
	}
	return ""
}
