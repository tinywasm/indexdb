//go:build wasm

package indexdb

import (
	"syscall/js"

	. "github.com/tinywasm/fmt"
)

// ProcessRequest handles an IndexedDB request (like add, put, get, delete)
// and returns the result or error via channels.
func ProcessRequest(req js.Value) (js.Value, error) {
	done := make(chan struct{})
	var result js.Value
	var err error

	// We need to define callbacks outside to be able to reference them if needed
	// or simply to keep code clean.

	onSuccess := js.FuncOf(func(this js.Value, args []js.Value) any {
		result = req.Get("result")
		close(done)
		return nil
	})
	defer onSuccess.Release()

	onError := js.FuncOf(func(this js.Value, args []js.Value) any {
		errVal := req.Get("error")
		errMsg := "Unknown IndexedDB error"
		if errVal.Truthy() {
			errMsg = errVal.Get("message").String()
		}
		err = Errf("IndexedDB request failed: %s", errMsg)
		close(done)
		return nil
	})
	defer onError.Release()

	req.Call("addEventListener", "success", onSuccess)
	req.Call("addEventListener", "error", onError)

	<-done
	return result, err
}

// ProcessCursorRequest handles an IndexedDB cursor request (openCursor).
// It iterates over the cursor and calls the provided callback for each item.
func ProcessCursorRequest(req js.Value, onNext func(cursor js.Value) bool) error {
	done := make(chan struct{})
	var err error

	// We need a persistent callback for 'success' because it's called multiple times for a cursor
	// However, since we are using 'continue' which triggers another success event on the SAME request object,
	// we can just reuse the same callback.

	var onSuccess js.Func
	onSuccess = js.FuncOf(func(this js.Value, args []js.Value) any {
		cursor := req.Get("result")

		if !cursor.Truthy() {
			// End of cursor iteration
			close(done)
			return nil
		}

		// Process current item
		shouldContinue := onNext(cursor)

		if shouldContinue {
			cursor.Call("continue")
		} else {
			// Stop iteration explicitly
			close(done)
		}
		return nil
	})
	defer onSuccess.Release()

	onError := js.FuncOf(func(this js.Value, args []js.Value) any {
		errVal := req.Get("error")
		errMsg := "Unknown IndexedDB cursor error"
		if errVal.Truthy() {
			errMsg = errVal.Get("message").String()
		}
		err = Errf("IndexedDB cursor failed: %s", errMsg)
		// Only close done on error if not already closed
		select {
		case <-done:
		default:
			close(done)
		}
		return nil
	})
	defer onError.Release()

	req.Call("addEventListener", "success", onSuccess)
	req.Call("addEventListener", "error", onError)

	<-done
	return err
}

// Transaction helper to start a transaction and get the object store.
// mode should be "readonly" or "readwrite".
func (d *IndexDBAdapter) getStore(tableName string, mode string) (store js.Value, err error) {
	if !d.db.Truthy() {
		return js.Value{}, Err("Database not initialized")
	}

	// IndexedDB's transaction() method throws an exception if the object store doesn't exist.
	// In Go WebAssembly, JS exceptions cause a panic, so we must recover and return an error.
	defer func() {
		if r := recover(); r != nil {
			err = Errf("Object store %s not found: %v", tableName, r)
		}
	}()

	tx := d.db.Call("transaction", tableName, mode)

	if !tx.Truthy() {
		return js.Value{}, Errf("Failed to create transaction for table %s", tableName)
	}

	store = tx.Call("objectStore", tableName)
	if !store.Truthy() {
		return js.Value{}, Errf("Failed to get object store for table %s", tableName)
	}

	return store, nil
}
