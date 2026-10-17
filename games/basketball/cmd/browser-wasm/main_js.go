//go:build js && wasm

package main

import (
	"syscall/js"

	"github.com/k0ngk0ng/wire-board/games/basketball/runtimebridge"
)

func main() {
	bridge := runtimebridge.New()
	call := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"ok":false,"error":{"code":"bad_request","detail":"expected a JSON request string"}}`
		}
		return bridge.JSON(args[0].String())
	})
	js.Global().Set("basketballWasm", call)
	select {}
}
