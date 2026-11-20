package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/k0ngk0ng/wire-board/games/hotlinemiami/engine"
)

func main() {
	world := engine.Map{}
	doors := []engine.Door{}
	broken := []int{}
	doorJSON, brokenJSON := "", ""
	init := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return false
		}
		candidate := engine.Map{}
		if json.Unmarshal([]byte(args[0].String()), &candidate) != nil || candidate.ID == "" || candidate.Width < 64*engine.Unit || candidate.Height < 64*engine.Unit || candidate.Width > 8192*engine.Unit || candidate.Height > 8192*engine.Unit {
			return false
		}
		world = candidate
		doors = nil
		broken = nil
		doorJSON = ""
		brokenJSON = ""
		return true
	})
	move := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 6 || world.ID == "" {
			return nil
		}
		values := make([]int, 6)
		for i := range values {
			if args[i].Type() != js.TypeNumber || !js.Global().Get("Number").Call("isSafeInteger", args[i]).Bool() {
				return nil
			}
			values[i] = args[i].Int()
		}
		if len(args) > 6 && args[6].Type() == js.TypeString && args[6].String() != doorJSON {
			next := []engine.Door{}
			if json.Unmarshal([]byte(args[6].String()), &next) != nil {
				return nil
			}
			doors = next
			doorJSON = args[6].String()
		}
		if len(args) > 7 && args[7].Type() == js.TypeString && args[7].String() != brokenJSON {
			next := []int{}
			if json.Unmarshal([]byte(args[7].String()), &next) != nil {
				return nil
			}
			broken = next
			brokenJSON = args[7].String()
		}
		x, y, aim := engine.PredictMotion(world, values[0], values[1], values[2], values[3], values[4], values[5], doors, broken)
		return []any{x, y, aim}
	})
	api := js.Global().Get("Object").New()
	weaponJSON, _ := json.Marshal(engine.Weapons())
	weapons := js.FuncOf(func(this js.Value, args []js.Value) any {
		return string(weaponJSON)
	})
	unarmedJSON, _ := json.Marshal(engine.UnarmedWeapon())
	unarmed := js.FuncOf(func(this js.Value, args []js.Value) any {
		return string(unarmedJSON)
	})
	api.Set("version", 1)
	api.Set("unit", engine.Unit)
	api.Set("fps", engine.FPS)
	api.Set("init", init)
	api.Set("move", move)
	api.Set("weapons", weapons)
	api.Set("unarmed", unarmed)
	js.Global().Set("hotlineMotion", api)
	select {}
}
