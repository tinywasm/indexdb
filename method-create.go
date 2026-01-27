package indexdb

import (
	"syscall/js"

	"reflect"

	. "github.com/tinywasm/fmt"
)

var blob_exist bool
var blob_file any

// items support: []any (struct instances)
func (d *IndexDB) Create(table_name string, items ...any) (err error) {

	blob_exist = false
	blob_file = nil

	const e = "indexdb create"

	if len(items) == 0 {
		return nil
	}

	// Create table if it doesn't exist using the first item as template
	if d.err = d.prepareStoreWithTableCheck("create", table_name, items[0]); d.err != nil {
		return Errf("%s %v", e, d.err)
	}

	d.data = make([]map[string]any, len(items))

	// Find primary key field
	pk_field := ""
	pk_index := -1
	if len(items) > 0 {
		v := reflect.ValueOf(items[0])
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		st := v.Type()
		if st.Kind() == reflect.Struct {
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				fieldName := f.Name
				_, isPK := IDorPrimaryKey(table_name, fieldName)
				if isPK {
					if pk_field != "" {
						return Errf("%s multiple primary keys found", e)
					}
					pk_field = fieldName
					pk_index = i
				}
			}
		}
	}
	if pk_field == "" {
		return Errf("%s no primary key found", e)
	}

	for i, item := range items {

		v := reflect.ValueOf(item)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		st := v.Type()

		if st.Kind() == reflect.Struct {

			m := make(map[string]any)

			for j := 0; j < st.NumField(); j++ {
				f := st.Field(j)

				fieldName := f.Name

				fieldValue := v.Field(j)

				val := fieldValue.Interface()

				// Check if this is the primary key field
				_, isPK := IDorPrimaryKey(table_name, f.Name)
				if isPK {

					m[pk_field] = val

				} else {

					m[fieldName] = val

				}

			}

			d.data[i] = m

		}

	}

	// CHECK ID
	for i, data := range d.data {

		id, id_exist := data[pk_field]

		// d.Logger("DATA table_name DB", table_name, "id_exist:", id_exist)

		if !id_exist || id.(string) == "" {
			//agregar id al objeto si este no existe
			id = d.GetNewID() //id nuevo
			// d.Logger("NUEVO ID GENERADO:", id)

			data[pk_field] = id
		}

		// si todo esta ok retornamos el id
		d.data[i][pk_field] = id.(string)
	}

	// Set ID back to structs if they are pointers
	for i, item := range items {
		v := reflect.ValueOf(item)
		if v.Kind() == reflect.Ptr && pk_index >= 0 {
			elem := v.Elem()
			fieldVal := elem.Field(pk_index)
			fieldVal.SetString(d.data[i][pk_field].(string))
		}
	}

	// fmt.Println("DATA IN INDEX DB:", d.data)

	for _, data := range d.data {

		// Inserta cada elemento en el almacén de objetos
		result := d.store.Call("add", data)
		if result.IsNull() {
			return Err("error creating element in db table:", table_name, "id:", data[pk_field].(string))
		}
		// d.Logger("resultado:", result)

		// result.Call("addEventListener", "success", js.FuncOf(func(this js.Value, p []js.Value) any {
		// 	d.Logger("Elemento creado con éxito:", data)
		// 	return nil
		// }))

		// Manejar la respuesta de manera asincrónica
		result.Call("addEventListener", "error", js.FuncOf(func(this js.Value, p []js.Value) any {
			// Log más detalles sobre el error
			errorObject := p[0].Get("target").Get("error")
			errorMessage := errorObject.Get("message").String()
			d.logger("Error al crear elemento en la db tabla:", table_name, "id:", data[pk_field].(string), errorMessage)
			return nil
		}))

		// creamos url temporal para acceder al archivo en el dom
		if blob_file, blob_exist = data["blob"]; blob_exist {
			data["url"] = CreateBlobURL(blob_file)
		}

	}

	return nil
}
