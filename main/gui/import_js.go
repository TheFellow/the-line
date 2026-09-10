//go:build js && wasm

package main

import (
	"fmt"
	"syscall/js"
)

// Browser import uses an ordinary file chooser, never private browser storage or
// an inspection-only path. The callback queues data; only Update edits the scene.
func (g *game) chooseCSV() {
	document := js.Global().Get("document")
	input := document.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".csv,text/csv")
	input.Set("id", "the-line-csv-import")
	input.Get("style").Set("display", "none")
	document.Get("body").Call("appendChild", input)
	var changed js.Func
	changed = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		if files.Length() == 0 {
			input.Call("remove")
			changed.Release()
			return nil
		}
		file := files.Index(0)
		if file.Get("size").Int() > 2<<20 {
			g.importRequests <- csvImport{err: fmt.Errorf("CSV import is limited to 2 MiB")}
			input.Call("remove")
			changed.Release()
			return nil
		}
		name := file.Get("name").String()
		var success, failure js.Func
		cleanup := func() { input.Call("remove"); changed.Release(); success.Release(); failure.Release() }
		success = js.FuncOf(func(_ js.Value, args []js.Value) any {
			g.importRequests <- csvImport{data: args[0].String(), name: name}
			cleanup()
			return nil
		})
		failure = js.FuncOf(func(_ js.Value, _ []js.Value) any {
			g.importRequests <- csvImport{err: fmt.Errorf("could not read CSV file %q", name)}
			cleanup()
			return nil
		})
		file.Call("text").Call("then", success, failure)
		return nil
	})
	input.Call("addEventListener", "change", changed)
	input.Call("click")
}
