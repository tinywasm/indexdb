package indexdb

import (
	"reflect"

	. "github.com/tinywasm/fmt"
)

func (d *IndexDB) Delete(table_name string, items ...any) (err error) {

	const e = "Delete"

	if d.err = d.prepareStore("delete", table_name); d.err != nil {
		return Errf("%s %v", e, d.err)
	}

	for _, item := range items {

		v := reflect.ValueOf(item)

		st := v.Type()

		if st.Kind() == reflect.Struct {

			found := false

			for j := 0; j < st.NumField(); j++ {
				f := st.Field(j)

				// Check if this is the primary key field
				_, isPK := IDorPrimaryKey(table_name, f.Name)
				if isPK {

					fieldValue := v.Field(j)

					id := fieldValue.Interface()

					d.result = d.store.Call("delete", id)

					if d.result.IsNull() {

						return Errf("%s error when deleting in table: %s", e, table_name)

					}

					found = true

					break

				}

			}

			if !found {

				return Errf("%s id not found in table: %s", e, table_name)

			}

		}

	}

	return nil
}
