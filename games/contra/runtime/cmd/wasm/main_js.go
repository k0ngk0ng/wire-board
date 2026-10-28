//go:build js && wasm

package main

import (
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"syscall/js"
)

func main() {
	bridge := runtime.New()
	callback := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"ok":false,"error":"expected JSON request"}`
		}
		return bridge.JSON(args[0].String())
	})
	js.Global().Set("contraRuntime", callback)
	select {}
}
