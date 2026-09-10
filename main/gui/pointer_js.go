//go:build js && wasm

package main

import "syscall/js"

var browserPointer struct {
	installed   bool
	interrupted bool
	listener    js.Func
}

// Ebitengine's browser focus check covers the document, but a canvas can lose
// focus on its own. Its mouse events also stop at the canvas boundary. Discard
// unfinished gestures on either event instead of mistaking lost input for a drop.
func pointerInterrupted() bool {
	if !browserPointer.installed {
		canvas := js.Global().Get("document").Call("querySelector", "canvas")
		if !canvas.IsNull() && !canvas.IsUndefined() {
			browserPointer.listener = js.FuncOf(func(_ js.Value, args []js.Value) any {
				browserPointer.interrupted = true
				if args[0].Get("type").String() == "mouseleave" {
					// Blur resets Ebitengine's held buttons. It cannot observe a
					// subsequent mouseup outside the canvas.
					canvas.Call("blur")
				}
				return nil
			})
			canvas.Call("addEventListener", "blur", browserPointer.listener)
			canvas.Call("addEventListener", "mouseleave", browserPointer.listener)
			js.Global().Call("addEventListener", "blur", browserPointer.listener)
			browserPointer.installed = true
		}
	}
	interrupted := browserPointer.interrupted
	browserPointer.interrupted = false
	return interrupted
}
