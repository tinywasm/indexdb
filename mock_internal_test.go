//go:build wasm

package indexdb

import (
	"syscall/js"
	"testing"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

type FakeModel struct {
	ID    string
	Score float64
	Age   int
	Valid bool
	Time  int64
	Blob  []byte
}

func (m *FakeModel) ModelName() string { return "fake" }
func (m *FakeModel) Schema() []Field {
	return []Field{
		{Name: "ID", Type: FieldText, PK: true},
		{Name: "Score", Type: FieldFloat},
		{Name: "Age", Type: FieldInt},
		{Name: "Valid", Type: FieldBool},
		{Name: "Time", Type: FieldInt},
		{Name: "Blob", Type: FieldBlob},
		{Name: "Missing", Type: FieldText},
	}
}
func (m *FakeModel) Values() []any {
	return []any{m.ID, m.Score, m.Age, m.Valid, m.Time, m.Blob, ""}
}
func (m *FakeModel) Pointers() []any {
	var missing string
	return []any{&m.ID, &m.Score, &m.Age, &m.Valid, &m.Time, &m.Blob, &missing}
}

func TestMapResultTypes(t *testing.T) {
	m := &FakeModel{}

	// Construct a synthetic js.Value object
	jsObj := js.ValueOf(map[string]any{
		"ID":    "test_id",
		"Score": 99.5,
		"Age":   42,
		"Valid": true,
		"Time":  1234567890,
		"Blob":  "skip", // not implemented for blobs yet
		// Missing field omitted intentionally to test IsUndefined
	})

	err := mapResult(jsObj, m)
	if err != nil {
		t.Fatalf("mapResult failed: %v", err)
	}

	if m.ID != "test_id" {
		t.Errorf("Expected ID 'test_id', got %v", m.ID)
	}
	if m.Score != 99.5 {
		t.Errorf("Expected Score 99.5, got %v", m.Score)
	}
	if m.Age != 42 {
		t.Errorf("Expected Age 42, got %v", m.Age)
	}
	if m.Valid != true {
		t.Errorf("Expected Valid true, got %v", m.Valid)
	}
	if m.Time != 1234567890 {
		t.Errorf("Expected Time 1234567890, got %v", m.Time)
	}
}

func TestCheckConditionOperators(t *testing.T) {
	// JS string
	strVal := js.ValueOf("hello")
	if !checkCondition(strVal, orm.Eq("f", "hello")) {
		t.Error("string = hello failed")
	}
	if !checkCondition(strVal, orm.Neq("f", "world")) {
		t.Error("string != world failed")
	}
	if checkCondition(strVal, orm.Gt("f", "world")) {
		t.Error("string > world should fail usually unless hello>world")
	} // hello is not > world

	strValWorld := js.ValueOf("world")
	if !checkCondition(strValWorld, orm.Gt("f", "hello")) {
		t.Error("string > hello failed")
	}
	if !checkCondition(strValWorld, orm.Gte("f", "hello")) {
		t.Error("string >= hello failed")
	}
	if !checkCondition(strVal, orm.Lt("f", "world")) {
		t.Error("string < world failed")
	}
	if !checkCondition(strVal, orm.Lte("f", "world")) {
		t.Error("string <= world failed")
	}

	// JS Number
	numVal := js.ValueOf(42.5)
	if !checkCondition(numVal, orm.Eq("f", 42.5)) {
		t.Error("num = 42.5 failed")
	}
	if checkCondition(numVal, orm.Eq("f", 42.0)) {
		t.Error("num = 42.0 should fail")
	}

	if !checkCondition(numVal, orm.Neq("f", 42.0)) {
		t.Error("num != 42.0 failed")
	}

	if !checkCondition(numVal, orm.Gt("f", 40.0)) {
		t.Error("num > 40.0 failed")
	}
	if !checkCondition(numVal, orm.Gt("f", 40)) {
		t.Error("num > 40 failed")
	}

	if !checkCondition(numVal, orm.Gte("f", 42.5)) {
		t.Error("num >= 42.5 failed")
	}
	if !checkCondition(numVal, orm.Gte("f", 42)) {
		t.Error("num >= 42 failed")
	}

	if !checkCondition(numVal, orm.Lt("f", 50.0)) {
		t.Error("num < 50.0 failed")
	}
	if !checkCondition(numVal, orm.Lt("f", 50)) {
		t.Error("num < 50 failed")
	}

	if !checkCondition(numVal, orm.Lte("f", 42.5)) {
		t.Error("num <= 42.5 failed")
	}
	if !checkCondition(numVal, orm.Lte("f", 43)) {
		t.Error("num <= 43 failed")
	}

	// JS Bool
	boolVal := js.ValueOf(true)
	if !checkCondition(boolVal, orm.Eq("f", true)) {
		t.Error("bool = true failed")
	}
	if !checkCondition(boolVal, orm.Neq("f", false)) {
		t.Error("bool != false failed")
	}

	// Unknown JS Type (Array)
	arrVal := js.ValueOf([]any{})
	if checkCondition(arrVal, orm.Eq("f", 1)) {
		t.Error("array eq should fail")
	}
}

func TestExecuteDefaultAction(t *testing.T) {
	logger := func(args ...any) { t.Log(args...) }

	adapter := newAdapter("test_db", nil, logger)
	err := adapter.execute(orm.Query{Action: 999}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unimplemented action")
	}
}

type UnknownTable struct{}

func (u *UnknownTable) ModelName() string { return "unknown_table" }
func (u *UnknownTable) Schema() []Field {
	return []Field{{Name: "id", Type: FieldText, PK: true}}
}
func (u *UnknownTable) Values() []any   { return []any{} }
func (u *UnknownTable) Pointers() []any { return []any{} }

func TestExecuteActionNotImplemented(t *testing.T) {
	logger := func(args ...any) { t.Log(args...) }

	// Create adapter directly to test execute default case
	adapter := newAdapter("test_db", nil, logger)
	err := adapter.execute(orm.Query{Action: 999}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unimplemented action")
	}

	// Test GetStore error on create
	err = adapter.execute(orm.Query{Action: orm.ActionCreate, Table: "non_existent"}, &UnknownTable{}, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unitialized DB / GetStore")
	}

	// Test GetStore error on update
	err = adapter.execute(orm.Query{Action: orm.ActionUpdate, Table: "non_existent"}, &UnknownTable{}, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unitialized DB / GetStore on update")
	}

	// Test GetStore error on delete
	err = adapter.execute(orm.Query{Action: orm.ActionDelete, Table: "non_existent"}, &UnknownTable{}, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unitialized DB / GetStore on delete")
	}

	// Test GetStore error on readOne
	err = adapter.execute(orm.Query{Action: orm.ActionReadOne, Table: "non_existent"}, &UnknownTable{}, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unitialized DB / GetStore on readOne")
	}

	// Test GetStore error on readAll
	err = adapter.execute(orm.Query{Action: orm.ActionReadAll, Table: "non_existent"}, &UnknownTable{}, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unitialized DB / GetStore on readAll")
	}

	// Test Query method error checks
	_, err = adapter.Query("", orm.Query{})
	if err == nil {
		t.Fatal("Expected error on invalid args to Query")
	}

	err = adapter.Exec("")
	if err == nil {
		t.Fatal("Expected error on Exec without query")
	}
}

func TestProcessRequests(t *testing.T) {
	// Create dummy requests using fake object to hit error paths
	dummyReq := js.ValueOf(map[string]any{
		"addEventListener": js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) > 1 && args[0].String() == "error" {
				cb := args[1]
				// It expects to be called as a DOM event where 'target' has error,
				// but our processRequest looks at req.Get("error") so we can just set it on our mock req
				dummyThis := js.ValueOf(map[string]any{
					"error": map[string]any{
						"message": "simulated obj error",
					},
				})
				cb.Invoke(dummyThis)
			}
			return nil
		}),
		"error": map[string]any{
			"message": "mock message",
		},
	})
	_, err := processRequest(dummyReq)
	if err == nil {
		t.Log("Expected error on mocked request")
	}

	err = processCursorRequest(dummyReq, func(c js.Value) bool { return false })
	if err == nil {
		t.Log("Expected error on mocked cursor")
	}
}

func TestAdapterQueryScanCoverage(t *testing.T) {
	adapter := newAdapter("test_db_scan", nil, nil)

	// We'll just call adapter.Query with the mock rows logic
	// Mock executing Query on uninitialized DB - hits execute error.
	rows, err := adapter.Query("", orm.Query{Action: orm.ActionReadAll, Table: "non_existent"}, &FakeModel{})
	if err == nil {
		t.Error("Expected error from Query on non existent")
	}
	_ = rows
}
