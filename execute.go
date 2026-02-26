//go:build js && wasm

package indexdb

import (
	"syscall/js"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

// Execute implements orm.Adapter for IndexDB.
func (d *IndexDBAdapter) Execute(q orm.Query, m orm.Model, factory func() orm.Model, each func(orm.Model)) error {
	switch q.Action {
	case orm.ActionCreate:
		return d.create(q, m)
	case orm.ActionUpdate:
		return d.update(q, m)
	case orm.ActionDelete:
		return d.delete(q, m)
	case orm.ActionReadOne:
		return d.readOne(q, m)
	case orm.ActionReadAll:
		return d.readAll(q, factory, each)
	default:
		return Err("Action not implemented")
	}
}

func (d *IndexDBAdapter) create(q orm.Query, m orm.Model) error {
	// Establish a "readwrite" transaction block directed at the store mapped via q.Table.
	store, err := d.getStore(q.Table, "readwrite")
	if err != nil {
		return err
	}

	// Iterate structurally mapping q.Columns and q.Values onto a conventional JavaScript Map Object
	data := make(map[string]any)
	for i, col := range q.Columns {
		data[col] = q.Values[i]
	}

	// Deploy store.add() and explicitly await its resolution event.
	req := store.Call("add", data)
	_, err = processRequest(req)
	return err
}

func (d *IndexDBAdapter) update(q orm.Query, m orm.Model) error {
	// Establish a "readwrite" transaction block directed at the store mapped via q.Table.
	store, err := d.getStore(q.Table, "readwrite")
	if err != nil {
		return err
	}

	// Iterate structurally mapping q.Columns and q.Values onto a conventional JavaScript Map Object
	data := make(map[string]any)
	for i, col := range q.Columns {
		data[col] = q.Values[i]
	}

	// Deploy store.put() and explicitly await its resolution event.
	req := store.Call("put", data)
	_, err = processRequest(req)
	return err
}

func (d *IndexDBAdapter) delete(q orm.Query, m orm.Model) error {
	// Establish a "readwrite" transaction block
	store, err := d.getStore(q.Table, "readwrite")
	if err != nil {
		return err
	}

	// If we have a single equality condition on a key path, we can delete directly.
	// But IndexedDB delete takes a key or a KeyRange.
	// We need to determine the key.
	// For now, we assume simple delete by PK if possible, or we iterate and delete.
	// However, `store.delete` expects a key.

	// Extract primary key value if present in conditions
	// This is a simplification. A robust adapter would parse conditions more deeply.
	// If q.Conditions has a PK condition, we use it.

	// Assuming the first condition is the PK match for now (common in simple ORMs)
	// or we need to find the PK from the model definition if available?
	// The plan says: "If q.Conditions contains PK, use store.delete(pk)."

	var pkValue any
	// Iterate conditions to find PK. We don't know which field is PK here easily without Model metadata,
	// but usually `orm` passes conditions.
	// Let's look for "id" or similar, or just take the condition value if it's an equality check.

	// Better approach: If there's 1 condition and it's '=', use it as the key.
	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" {
		pkValue = q.Conditions[0].Value()
		req := store.Call("delete", pkValue)
		_, err = processRequest(req)
		return err
	}

	// If complex conditions, we might need to use a cursor to find keys to delete.
	// This is more complex and might require multiple transactions or a cursor update.
	// IndexedDB cursor.delete() can delete the record at the cursor position.

	// Create a cursor to find records matching conditions
	// However, we need to open the cursor, iterate, check conditions, and call cursor.delete().

	req := store.Call("openCursor")

	done := make(chan struct{})
	// Removed unused loopErr

	successFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		cursor := req.Get("result")
		if !cursor.Truthy() {
			close(done)
			return nil
		}

		// Get current value to check conditions
		val := cursor.Get("value")

		// Check conditions in Go
		match := true
		for _, cond := range q.Conditions {
			fieldVal := val.Get(cond.Field())
			if !checkCondition(fieldVal, cond) {
				match = false
				break
			}
		}

		if match {
			// Delete at cursor
			delReq := cursor.Call("delete")
			// We handle delete request success/error?
			// Usually it's async too. But we are inside a success callback of the cursor.
			// We can attach a success handler to the delete request, but we also need to continue the cursor.
			// The cursor continue should probably happen after delete success?
			// Or we can fire and forget if we trust it works in the same tx?
			// Let's try to wait for delete? No, that blocks the event loop potentially?
			// The standard way is to request delete, and on its success, continue cursor.

			dSuccess := js.FuncOf(func(this js.Value, args []js.Value) any {
				cursor.Call("continue")
				return nil
			})
			// We need to manage lifecycle of this callback...
			// To simplify, let's just assume delete works and call continue.
			// (This is risky but standard specific implementation details are tricky here without more elaborate promise chaining).

			// Actually, let's just call continue immediately. The transaction ensures consistency.
			delReq.Call("addEventListener", "success", dSuccess)

			// We'll leak dSuccess if we don't clean it up.
			// This path is getting complicated.
			// Let's stick to the plan: "Deploy store.add() or store.put() or store.delete() and explicitly await its resolution event."
			// If we are iterating, we are doing multiple deletes.

			// Alternative: Collect keys to delete, then delete them one by one?
			// Or just use the simple PK deletion for now as MVP.
		} else {
			cursor.Call("continue")
		}

		return nil
	})
	defer successFunc.Release()

	// If we are here, we probably didn't implement complex delete yet.
	// Let's implement the simple PK delete first as it covers 90% of cases.
	return Err("Complex delete with conditions not fully implemented yet, only single PK delete supported via conditions")
}

func (d *IndexDBAdapter) readOne(q orm.Query, m orm.Model) error {
	store, err := d.getStore(q.Table, "readonly")
	if err != nil {
		return err
	}

	// Attempt to get by key if simple condition
	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" {
		key := q.Conditions[0].Value()
		req := store.Call("get", key)
		result, err := processRequest(req)
		if err != nil {
			return err
		}
		if !result.Truthy() {
			return Err("record not found")
		}
		return mapResult(result, m)
	}

	// Otherwise, iterate with cursor until first match
	req := store.Call("openCursor")
	var found bool

	err = processCursorRequest(req, func(cursor js.Value) bool {
		val := cursor.Get("value")

		// Check conditions
		match := true
		for _, cond := range q.Conditions {
			fieldVal := val.Get(cond.Field())
			if !checkCondition(fieldVal, cond) {
				match = false
				break
			}
		}

		if match {
			// Found it
			err := mapResult(val, m)
			if err != nil {
				d.logger("Mapping error:", err)
			}
			found = true
			return false // Stop iteration
		}

		return true // Continue iteration
	})

	if err != nil {
		return err
	}
	if !found {
		return Err("record not found")
	}
	return nil
}

func (d *IndexDBAdapter) readAll(q orm.Query, factory func() orm.Model, each func(orm.Model)) error {
	store, err := d.getStore(q.Table, "readonly")
	if err != nil {
		return err
	}

	req := store.Call("openCursor")

	return processCursorRequest(req, func(cursor js.Value) bool {
		val := cursor.Get("value")

		// Check conditions
		match := true
		for _, cond := range q.Conditions {
			fieldVal := val.Get(cond.Field())
			if !checkCondition(fieldVal, cond) {
				match = false
				break
			}
		}

		if match {
			newItem := factory()
			err := mapResult(val, newItem)
			if err != nil {
				d.logger("Mapping error:", err)
				return true // Continue even if mapping fails?
			}
			each(newItem)
		}

		return true // Continue iteration
	})
}

// mapResult maps a JS value to a Model's pointers
func mapResult(val js.Value, m orm.Model) error {
	cols := m.Columns()
	ptrs := m.Pointers()

	for i, col := range cols {
		jsVal := val.Get(col)
		if jsVal.IsUndefined() {
			continue
		}

		// Map JS value to Go pointer
		// We need to handle types.
		dest := ptrs[i]

		switch v := dest.(type) {
		case *string:
			*v = jsVal.String()
		case *int:
			*v =(jsVal.Int())
		case *float64:
			*v = jsVal.Float()
		case *bool:
			*v = jsVal.Bool()
		// Add other types as needed
		default:
			// Fallback or error?
			// For now, minimal support
		}
	}
	return nil
}

// checkCondition checks if a JS value satisfies a condition
func checkCondition(val js.Value, cond orm.Condition) bool {
	// Simple type checking and comparison
	// This needs to be robust for types (string, number, boolean)

	// Get Go value from JS value for comparison
	var goVal any
	switch val.Type() {
	case js.TypeString:
		goVal = val.String()
	case js.TypeNumber:
		goVal = val.Float()
	case js.TypeBoolean:
		goVal = val.Bool()
	default:
		return false // unknown type
	}

	condVal := cond.Value()

	switch cond.Operator() {
	case "=":
		return goVal == condVal
	case "!=":
		return goVal != condVal
	case ">":
		// Type assertions needed
		if v1, ok := goVal.(float64); ok {
			if v2, ok := condVal.(float64); ok {
				return v1 > v2
			}
			if v2, ok := condVal.(int); ok {
				return v1 > float64(v2)
			}
		}
	// ... Implement other operators
	}

	return false
}
