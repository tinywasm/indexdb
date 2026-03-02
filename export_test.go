//go:build js && wasm

package indexdb

import (
	"syscall/js"
	"github.com/tinywasm/orm"
)

func ExportMapResult(val js.Value, m orm.Model) error {
	return mapResult(val, m)
}

func ExportCheckCondition(val js.Value, cond orm.Condition) bool {
	return checkCondition(val, cond)
}

func ExportProcessRequest(req js.Value) (js.Value, error) {
	return processRequest(req)
}

func ExportProcessCursorRequest(req js.Value, onNext func(cursor js.Value) bool) error {
	return processCursorRequest(req, onNext)
}
