//go:build wasm

package indexdb

import (
	"sort"
	"syscall/js"

	"github.com/tinywasm/jsvalue"

	"github.com/tinywasm/fmt"
	. "github.com/tinywasm/model"
	"github.com/tinywasm/storage"
)

// execute implements storage.Adapter for IndexDB.
func (d *adapter) execute(q storage.Query, m Model, factory func() Model, each func(Model), eachJS func(js.Value)) error {
	switch q.Action {
	case storage.ActionCreate:
		return d.create(q, m)
	case storage.ActionUpdate:
		return d.update(q, m)
	case storage.ActionDelete:
		return d.delete(q, m)
	case storage.ActionReadOne:
		return d.readOne(q, m)
	case storage.ActionReadAll:
		return d.readAll(q, factory, each, eachJS)
	default:
		return fmt.Err("Action not implemented")
	}
}

func (d *adapter) create(q storage.Query, m Model) error {
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
	_, err = jsvalue.AwaitRequest(req)
	return err
}

func (d *adapter) update(q storage.Query, m Model) error {
	store, err := d.getStore(q.Table, "readwrite")
	if err != nil {
		return err
	}

	fields := m.Schema()
	pkName := ""
	for _, f := range fields {
		if f.IsPK() {
			pkName = f.Name
			break
		}
	}

	// Optimize: single PK equality condition (handles updates with direct get and put)
	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" && q.Conditions[0].Field() == pkName {
		pkValue := q.Conditions[0].Value()
		getReq := store.Call("get", pkValue)
		val, err := jsvalue.AwaitRequest(getReq)
		if err != nil {
			return err
		}

		if !val.Truthy() || val.IsUndefined() {
			return storage.ErrNoRows
		}

		// Overwrite fields
		data := make(map[string]any)
		for _, f := range fields {
			jsVal := val.Get(f.Name)
			if !jsVal.IsUndefined() {
				switch f.Type.Storage() {
				case FieldText:
					data[f.Name] = jsVal.String()
				case FieldInt:
					data[f.Name] = int64(jsVal.Int())
				case FieldFloat:
					data[f.Name] = jsVal.Float()
				case FieldBool:
					data[f.Name] = jsVal.Bool()
				}
			}
		}

		for i, col := range q.Columns {
			if col == pkName {
				newVal := q.Values[i]
				if newVal == "" || newVal == nil || newVal == int(0) || newVal == int64(0) || newVal == float64(0) {
					continue
				}
			}
			data[col] = q.Values[i]
		}

		putReq := store.Call("put", data)
		_, err = jsvalue.AwaitRequest(putReq)
		return err
	}

	// For cursors, collect all matching records first to avoid nested AwaitRequest deadlocks
	type matchRecord struct {
		val js.Value
	}
	var matched []matchRecord

	req := store.Call("openCursor")
	err = processCursorRequest(req, func(cursor js.Value) bool {
		val := cursor.Get("value")
		if checkConditions(val, q.Conditions) {
			matched = append(matched, matchRecord{val: val})
		}
		return true
	})
	if err != nil {
		return err
	}

	for _, item := range matched {
		data := make(map[string]any)
		for _, f := range fields {
			jsVal := item.val.Get(f.Name)
			if !jsVal.IsUndefined() {
				switch f.Type.Storage() {
				case FieldText:
					data[f.Name] = jsVal.String()
				case FieldInt:
					data[f.Name] = int64(jsVal.Int())
				case FieldFloat:
					data[f.Name] = jsVal.Float()
				case FieldBool:
					data[f.Name] = jsVal.Bool()
				}
			}
		}

		for i, col := range q.Columns {
			if col == pkName {
				newVal := q.Values[i]
				if newVal == "" || newVal == nil || newVal == int(0) || newVal == int64(0) || newVal == float64(0) {
					continue
				}
			}
			data[col] = q.Values[i]
		}

		putReq := store.Call("put", data)
		_, err = jsvalue.AwaitRequest(putReq)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *adapter) delete(q storage.Query, m Model) error {
	store, err := d.getStore(q.Table, "readwrite")
	if err != nil {
		return err
	}

	fields := m.Schema()
	pkName := ""
	for _, f := range fields {
		if f.IsPK() {
			pkName = f.Name
			break
		}
	}

	// If it is a simple single equality condition on the PK, we can delete by key directly.
	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" && q.Conditions[0].Field() == pkName {
		pkValue := q.Conditions[0].Value()
		req := store.Call("delete", pkValue)
		_, err = jsvalue.AwaitRequest(req)
		return err
	}

	// Otherwise, find matching records using a cursor and delete them.
	req := store.Call("openCursor")

	return processCursorRequest(req, func(cursor js.Value) bool {
		val := cursor.Get("value")

		if checkConditions(val, q.Conditions) {
			cursor.Call("delete")
		}

		return true
	})
}

func (d *adapter) readOne(q storage.Query, m Model) error {
	store, err := d.getStore(q.Table, "readonly")
	if err != nil {
		return err
	}

	// Attempt to get by key if simple condition. We only do this if we are querying the PK.
	// For simplicity, we'll try `get` first if it's a single equality, and fall back to cursor.
	if len(q.Conditions) == 1 && q.Conditions[0].Operator() == "=" {
		key := q.Conditions[0].Value()
		req := store.Call("get", key)
		result, err := jsvalue.AwaitRequest(req)
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
		match := checkConditions(val, q.Conditions)

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
		return storage.ErrNoRows
	}
	return nil
}

type matchedItem struct {
	model Model
	val   js.Value
}

func (d *adapter) readAll(q storage.Query, factory func() Model, each func(Model), eachJS func(js.Value)) error {
	store, err := d.getStore(q.Table, "readonly")
	if err != nil {
		return err
	}

	req := store.Call("openCursor")

	var matched []matchedItem

	err = processCursorRequest(req, func(cursor js.Value) bool {
		val := cursor.Get("value")

		if checkConditions(val, q.Conditions) {
			var newItem Model
			if factory != nil {
				newItem = factory()
				if newItem != nil {
					err := mapResult(val, newItem)
					if err != nil {
						d.logger("Mapping error:", err)
						return true // Continue iteration
					}
				}
			}
			matched = append(matched, matchedItem{model: newItem, val: val})
		}

		return true // Continue iteration
	})
	if err != nil {
		return err
	}

	// Apply OrderBy
	if len(q.OrderBy) > 0 {
		sort.Slice(matched, func(i, j int) bool {
			for _, order := range q.OrderBy {
				col := order.Column()
				jsA := matched[i].val.Get(col)
				jsB := matched[j].val.Get(col)

				// Compare jsA and jsB
				switch jsA.Type() {
				case js.TypeString:
					strA := jsA.String()
					strB := jsB.String()
					if strA != strB {
						if order.Dir() == "DESC" {
							return strA > strB
						}
						return strA < strB
					}
				case js.TypeNumber:
					numA := jsA.Float()
					numB := jsB.Float()
					if numA != numB {
						if order.Dir() == "DESC" {
							return numA > numB
						}
						return numA < numB
					}
				case js.TypeBoolean:
					boolA := jsA.Bool()
					boolB := jsB.Bool()
					if boolA != boolB {
						if order.Dir() == "DESC" {
							return boolA && !boolB
						}
						return !boolA && boolB
					}
				}
			}
			return false
		})
	}

	// Apply Offset and Limit
	start := q.Offset
	if start < 0 {
		start = 0
	}
	if start > len(matched) {
		start = len(matched)
	}

	end := len(matched)
	if q.Limit > 0 {
		end = start + q.Limit
		if end > len(matched) {
			end = len(matched)
		}
	}

	sliced := matched[start:end]

	// Output results
	for _, item := range sliced {
		if each != nil {
			each(item.model)
		} else if eachJS != nil {
			eachJS(item.val)
		}
	}

	return nil
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

		if err := jsvalue.ScanValue(jsVal, ptrs[i]); err != nil {
			return err
		}
	}
	return nil
}

// checkConditions checks a slice of conditions sequentially
func checkConditions(val js.Value, conditions []storage.Condition) bool {
	if len(conditions) == 0 {
		return true
	}

	cond := conditions[0]
	fieldVal := val.Get(cond.Field())
	match := checkCondition(fieldVal, cond)

	for i := 1; i < len(conditions); i++ {
		cond = conditions[i]
		fieldVal = val.Get(cond.Field())
		condMatch := checkCondition(fieldVal, cond)
		if cond.Logic() == "OR" {
			match = match || condMatch
		} else {
			match = match && condMatch
		}
	}

	return match
}

// checkCondition checks if a JS value satisfies a condition
func checkCondition(val js.Value, cond storage.Condition) bool {
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
		return compareAny(goVal, condVal)
	case "!=":
		return !compareAny(goVal, condVal)
	case "IN":
		return valueInList(goVal, condVal)
	case "LIKE":
		sVal, okS := goVal.(string)
		patVal, okP := condVal.(string)
		if okS && okP {
			return matchLike(sVal, patVal)
		}
		return false
	case ">":
		if v1, ok := goVal.(float64); ok {
			if v2, ok := condVal.(float64); ok {
				return v1 > v2
			}
			if v2, ok := condVal.(int); ok {
				return v1 > float64(v2)
			}
			if v2, ok := condVal.(int64); ok {
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
			if v2, ok := condVal.(int64); ok {
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
			if v2, ok := condVal.(int64); ok {
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
			if v2, ok := condVal.(int64); ok {
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

func compareAny(a, b any) bool {
	if a == b {
		return true
	}
	var fA, fB float64
	var okA, okB bool
	switch v := a.(type) {
	case float64:
		fA, okA = v, true
	case int64:
		fA, okA = float64(v), true
	case int:
		fA, okA = float64(v), true
	}
	switch v := b.(type) {
	case float64:
		fB, okB = v, true
	case int64:
		fB, okB = float64(v), true
	case int:
		fB, okB = float64(v), true
	}
	if okA && okB {
		return fA == fB
	}
	return false
}

func valueInList(goVal any, list any) bool {
	switch l := list.(type) {
	case []any:
		for _, item := range l {
			if compareAny(goVal, item) {
				return true
			}
		}
	case []string:
		for _, item := range l {
			if compareAny(goVal, item) {
				return true
			}
		}
	case []int64:
		for _, item := range l {
			if compareAny(goVal, item) {
				return true
			}
		}
	case []int:
		for _, item := range l {
			if compareAny(goVal, item) {
				return true
			}
		}
	case []float64:
		for _, item := range l {
			if compareAny(goVal, item) {
				return true
			}
		}
	}
	return false
}

func matchLike(s, pattern string) bool {
	if len(pattern) == 0 {
		return s == ""
	}
	if pattern == "%" {
		return true
	}

	hasPrefixWildcard := pattern[0] == '%'
	hasSuffixWildcard := pattern[len(pattern)-1] == '%'

	cleanPattern := pattern
	if hasPrefixWildcard {
		cleanPattern = cleanPattern[1:]
	}
	if hasSuffixWildcard {
		cleanPattern = cleanPattern[:len(cleanPattern)-1]
	}

	if hasPrefixWildcard && hasSuffixWildcard {
		return containsSubstring(s, cleanPattern)
	}
	if hasPrefixWildcard {
		return hasSuffix(s, cleanPattern)
	}
	if hasSuffixWildcard {
		return hasPrefix(s, cleanPattern)
	}
	return s == pattern
}

func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
