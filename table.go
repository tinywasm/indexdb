package indexdb

import (
	"reflect"

	. "github.com/tinywasm/fmt"
)

// CreateTableIfNotExists creates a table for the given struct type if it doesn't exist
func (d *IndexDB) CreateTableIfNotExists(tableName string, structType any) error {
	// Check if table already exists
	if d.TableExist(tableName) {
		return nil
	}

	// Create the table
	return d.createTable(tableName, structType)
}

// createTable creates a table for the given struct type
func (d *IndexDB) createTable(tableName string, structType any) error {
	st := reflect.TypeOf(structType)
	if st.Kind() == reflect.Ptr {
		st = st.Elem()
	}

	if st.Kind() == reflect.Struct {

		table_name := tableName

		if st.NumField() != 0 {
			// Find primary key field
			pk_name := ""
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				fieldName := f.Name
				_, isPK := IDorPrimaryKey(table_name, fieldName)
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
			newTable := d.db.Call("createObjectStore", table_name, map[string]interface{}{"keyPath": pk_name})

			// Create indexes for all fields except primary key
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				fieldName := f.Name

				// Skip primary key field (it's the keyPath)
				_, unique := IDorPrimaryKey(table_name, fieldName)

				// Create index for the field
				newTable.Call("createIndex", fieldName, fieldName, map[string]interface{}{"unique": unique})
			}
		}
	}

	return nil
}

// TableExist checks if a table exists in the database
func (d IndexDB) TableExist(table_name string) bool {
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
