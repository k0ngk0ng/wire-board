//go:build js && wasm

// This research-only entry is built separately; no client asset is embedded.
package main

import (
	"encoding/json"
	"errors"
	"syscall/js"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/runtimebridge"
)

type request struct {
	Op                         string
	Rules, Sprites, Arena, HUD json.RawMessage
	Options                    engine.ChuSessionOptions
	Held                       [8]engine.Buttons
	Snapshot                   string
}

type reply struct {
	OK           bool                `json:"ok"`
	Error        string              `json:"error,omitempty"`
	View         *runtimebridge.View `json:"view,omitempty"`
	Snapshot     string              `json:"snapshot,omitempty"`
	DrawRequests int                 `json:"draw_requests,omitempty"`
	SpritePixels int                 `json:"sprite_pixels,omitempty"`
}

func main() {
	var assets runtimebridge.Assets
	var loaded bool
	var session *runtimebridge.Session
	call := js.FuncOf(func(_ js.Value, args []js.Value) any {
		response := reply{}
		var err error
		if len(args) == 0 || args[0].Type() != js.TypeString {
			err = errors.New("expected one JSON player request")
		} else {
			var req request
			err = json.Unmarshal([]byte(args[0].String()), &req)
			if err == nil {
				switch req.Op {
				case "load":
					var next runtimebridge.Assets
					next, err = runtimebridge.DecodeAssets(req.Rules, req.Sprites, req.Arena, req.HUD)
					if err == nil {
						assets, loaded, session = next, true, nil
					}
				case "prepare":
					if !loaded {
						err = errors.New("player assets are not loaded")
					} else {
						var next *runtimebridge.Session
						next, err = runtimebridge.NewSession(req.Options, assets)
						if err == nil {
							session = next
						}
					}
				case "restore":
					if !loaded {
						err = errors.New("player assets are not loaded")
					} else {
						var next *runtimebridge.Session
						next, err = runtimebridge.RestoreSession([]byte(req.Snapshot), assets)
						if err == nil {
							session = next
						}
					}
				case "ready", "step", "inspect", "save":
					if session == nil {
						err = errors.New("no independent match is prepared")
						break
					}
					switch req.Op {
					case "ready":
						err = session.Ready()
					case "save":
						var saved []byte
						saved, err = session.Save()
						response.Snapshot = string(saved)
					case "step":
						if len(args) != 2 || !args[1].InstanceOf(js.Global().Get("Uint8Array")) || args[1].Length() != 320*224*4 {
							err = errors.New("step needs a complete320x224 RGBA target")
							break
						}
						var frame runtimebridge.Frame
						frame, err = session.Step(req.Held)
						if err == nil {
							js.CopyBytesToJS(args[1], frame.Pixels)
							response.DrawRequests, response.SpritePixels = frame.DrawRequests, frame.SpritePixels
						}
					}
				default:
					err = errors.New("unknown local player operation")
				}
			}
		}
		if err != nil {
			response.Error = err.Error()
		} else {
			response.OK = true
			if session != nil {
				view := session.View()
				response.View = &view
			}
		}
		encoded, _ := json.Marshal(response)
		return string(encoded)
	})
	js.Global().Set("yuyuhakushoWasm", call)
	select {}
}
