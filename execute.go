//go:build wasm

package indexdb

import (
	"syscall/js"

	. "github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

// execute implements orm.Adapter for IndexDB.
func (d *adapter) execute(q orm.Query, m Model, factory func() Model, each func(Model), eachJS func(js.Value)) error {
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
		return d.readAll(q, factory, each, eachJS)
	default:
		return Err("Action not implemented")
	}
}

func (d *adapter) create(q orm.Query, m Model) error {
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

func (d *adapter) update(q orm.Query, m Model) error {
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

func (d *adapter) delete(q orm.Query, m Model) error {
	store, err := d.getStore(q.Table, "readwrite")
	if err != nil {
		return err
	}

	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" {
		pkValue := q.Conditions[0].Value()
		req := store.Call("delete", pkValue)
		_, err = processRequest(req)
		return err
	}

	return Err("delete requires a single equality condition on the primary key")
}

func (d *adapter) readOne(q orm.Query, m Model) error {
	store, err := d.getStore(q.Table, "readonly")
	if err != nil {
		return err
	}

	// Attempt to get by key if simple condition. We only do this if we are querying the PK.
	// For simplicity, we'll try `get` first if it's a single equality, and fall back to cursor.
	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" {
		key := q.Conditions[0].Value()
		req := store.Call("get", key)
		result, err := processRequest(req)
		if err == nil && result.Truthy() {
			return mapResult(result, m)
		}
		// If not found by key, maybe it wasn't the PK. Fall back to cursor.
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

func (d *adapter) readAll(q orm.Query, factory func() Model, each func(Model), eachJS func(js.Value)) error {
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
			if factory != nil && each != nil {
				newItem := factory()
				if newItem != nil {
					err := mapResult(val, newItem)
					if err != nil {
						d.logger("Mapping error:", err)
						return true // Continue even if mapping fails?
					}
					each(newItem)
				}
			} else if eachJS != nil {
				eachJS(val)
			}
		}

		return true // Continue iteration
	})
}

// mapResult maps a JS value to a Model's pointers
func mapResult(val js.Value, m Model) error {
	fields := m.Schema()
	ptrs := m.Pointers()

	for i, field := range fields {
		jsVal := val.Get(field.Name)
		if jsVal.IsUndefined() {
			continue
		}

		ptr := ptrs[i]
		switch field.Type {
		case FieldText:
			if p, ok := ptr.(*string); ok {
				*p = jsVal.String()
			}
		case FieldInt:
			if p, ok := ptr.(*int64); ok {
				*p = int64(jsVal.Int())
			} else if p, ok := ptr.(*int); ok {
				*p = jsVal.Int()
			}
		case FieldFloat:
			if p, ok := ptr.(*float64); ok {
				*p = jsVal.Float()
			}
		case FieldBool:
			if p, ok := ptr.(*bool); ok {
				*p = jsVal.Bool()
			}
		case FieldBlob:
			// []byte from Uint8Array if needed — skip for now, not used in indexdb
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
		} else if v1, ok := goVal.(string); ok {
			if v2, ok := condVal.(string); ok {
				return v1 > v2
			}
		}
	case ">=":
		if v1, ok := goVal.(float64); ok {
			if v2, ok := condVal.(float64); ok {
				return v1 >= v2
			}
			if v2, ok := condVal.(int); ok {
				return v1 >= float64(v2)
			}
		} else if v1, ok := goVal.(string); ok {
			if v2, ok := condVal.(string); ok {
				return v1 >= v2
			}
		}
	case "<":
		if v1, ok := goVal.(float64); ok {
			if v2, ok := condVal.(float64); ok {
				return v1 < v2
			}
			if v2, ok := condVal.(int); ok {
				return v1 < float64(v2)
			}
		} else if v1, ok := goVal.(string); ok {
			if v2, ok := condVal.(string); ok {
				return v1 < v2
			}
		}
	case "<=":
		if v1, ok := goVal.(float64); ok {
			if v2, ok := condVal.(float64); ok {
				return v1 <= v2
			}
			if v2, ok := condVal.(int); ok {
				return v1 <= float64(v2)
			}
		} else if v1, ok := goVal.(string); ok {
			if v2, ok := condVal.(string); ok {
				return v1 <= v2
			}
		}
	}

	return false
}
