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
	Op                                   string
	Rules, Sprites, Arena, HUD, ArenaAux json.RawMessage
	Options                              engine.ChuSessionOptions
	Held                                 [8]engine.Buttons
	Snapshot, Checkpoint, Epoch          string
}

type reply struct {
	OK           bool                `json:"ok"`
	Error        string              `json:"error,omitempty"`
	View         *runtimebridge.View `json:"view,omitempty"`
	Preview      *runtimebridge.View `json:"preview,omitempty"`
	Snapshot     string              `json:"snapshot,omitempty"`
	Checkpoint   string              `json:"checkpoint,omitempty"`
	DrawRequests int                 `json:"draw_requests,omitempty"`
	SpritePixels int                 `json:"sprite_pixels,omitempty"`
}

// load-v2 transfers exact decompressed resource bytes independently of JSON.
// The tuple order is rules, sprites, selected arena, HUD, optional aux, metadata,
// descriptor. Empty aux is still an explicit Uint8Array.
func networkBytes(args []js.Value) (runtimebridge.AssetBytes, error) {
	var raw runtimebridge.AssetBytes
	if len(args) != 8 {
		return raw, errors.New("load-v2 needs seven exact Uint8Array resources")
	}
	fields := []*[]byte{&raw.Rules, &raw.Sprites, &raw.Arena, &raw.HUD, &raw.ArenaAux, &raw.RenderMetadata, &raw.Descriptor}
	for i, field := range fields {
		value := args[i+1]
		if value.Type() != js.TypeObject || !value.InstanceOf(js.Global().Get("Uint8Array")) || value.Length() > 64<<20 {
			return runtimebridge.AssetBytes{}, errors.New("load-v2 resource must be a bounded Uint8Array")
		}
		*field = make([]byte, value.Length())
		js.CopyBytesToGo(*field, value)
	}
	return raw, nil
}
func pixelTarget(args []js.Value) bool {
	return len(args) == 2 && args[1].Type() == js.TypeObject && args[1].InstanceOf(js.Global().Get("Uint8Array")) && args[1].Length() == 320*224*4
}
func main() {
	var assets runtimebridge.Assets
	var loaded bool
	var session *runtimebridge.Session
	call := js.FuncOf(func(_ js.Value, args []js.Value) any {
		response := reply{}
		var err error
		if len(args) == 0 || args[0].Type() != js.TypeString {
			err = errors.New("expected a JSON player request")
		} else {
			var req request
			err = json.Unmarshal([]byte(args[0].String()), &req)
			if err == nil {
				switch req.Op {
				case "load", "load-v2":
					var next runtimebridge.Assets
					if req.Op == "load" {
						next, err = runtimebridge.DecodeAssetsForArena(req.Rules, req.Sprites, req.Arena, req.HUD, req.ArenaAux, req.Options.Arena)
					} else {
						var raw runtimebridge.AssetBytes
						raw, err = networkBytes(args)
						if err == nil {
							next, err = runtimebridge.DecodeNetworkAssets(raw, req.Options.Arena)
						}
					}
					if err == nil {
						assets, loaded, session = next, true, nil
					}
				case "prepare", "restore", "restore-checkpoint":
					if !loaded {
						err = errors.New("player assets are not loaded")
					} else {
						var next *runtimebridge.Session
						switch req.Op {
						case "prepare":
							next, err = runtimebridge.NewSession(req.Options, assets)
						case "restore":
							next, err = runtimebridge.RestoreSession([]byte(req.Snapshot), assets)
						case "restore-checkpoint":
							next, err = runtimebridge.RestoreCheckpoint([]byte(req.Checkpoint), assets)
						}
						if err == nil {
							session = next
						}
					}
				case "ready", "step", "preview", "inspect", "save", "checkpoint":
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
					case "checkpoint":
						var saved []byte
						saved, err = session.Checkpoint(req.Epoch)
						response.Checkpoint = string(saved)
					case "step", "preview":
						if !pixelTarget(args) {
							err = errors.New("rendering needs a complete 320x224 RGBA target")
							break
						}
						var frame runtimebridge.Frame
						if req.Op == "preview" {
							frame, err = session.Preview()
						} else {
							frame, err = session.Step(req.Held)
						}
						if err == nil {
							js.CopyBytesToJS(args[1], frame.Pixels)
							response.DrawRequests, response.SpritePixels = frame.DrawRequests, frame.SpritePixels
							if req.Op == "preview" {
								response.Preview = &frame.View
							}
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
