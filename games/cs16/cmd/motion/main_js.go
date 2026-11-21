package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

var sourceSHA256 string

type request struct {
	Player  engine.Player       `json:"player"`
	Input   engine.Input        `json:"input"`
	Frames  int                 `json:"frames"`
	Brushes []engine.BrushState `json:"brushes,omitempty"`
	Players []engine.Player     `json:"players,omitempty"`
}

type displayRequest struct {
	From    *engine.Vec3        `json:"from"`
	To      *engine.Vec3        `json:"to"`
	Crouch  bool                `json:"crouch"`
	Brushes []engine.BrushState `json:"brushes,omitempty"`
}

func validPoint(point engine.Vec3) bool {
	limit := 65536 * engine.Unit
	return point.X >= -limit && point.X <= limit && point.Y >= -limit && point.Y <= limit && point.Z >= -limit && point.Z <= limit
}

func validHull(hull engine.Hull, planes []engine.Plane) bool {
	if hull.Root < 0 || hull.Root >= len(hull.Nodes) {
		return false
	}
	for _, node := range hull.Nodes {
		if node.Plane < 0 || node.Plane >= len(planes) || node.Front >= len(hull.Nodes) || node.Back >= len(hull.Nodes) {
			return false
		}
	}
	return true
}

func main() {
	world := engine.Map{}
	initialize := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString || len(args[0].String()) > 32*1024*1024 {
			return false
		}
		var candidate engine.Map
		if json.Unmarshal([]byte(args[0].String()), &candidate) != nil || candidate.ID == "" || len(candidate.Planes) == 0 || len(candidate.Hulls) < 2 {
			return false
		}
		for index := range candidate.Hulls {
			if !validHull(candidate.Hulls[index], candidate.Planes) {
				return false
			}
			candidate.Hulls[index].Planes = candidate.Planes
		}
		for brush := range candidate.Brushes {
			for hull := range candidate.Brushes[brush].Hulls {
				if !validHull(candidate.Brushes[brush].Hulls[hull], candidate.Planes) {
					return false
				}
				candidate.Brushes[brush].Hulls[hull].Planes = candidate.Planes
			}
		}
		world = candidate
		return true
	})
	move := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString || world.ID == "" || len(args[0].String()) > 64*1024 {
			return `{"error":"invalid motion request"}`
		}
		var value request
		if json.Unmarshal([]byte(args[0].String()), &value) != nil || value.Frames < 1 || value.Frames > 20 || value.Input.MoveX < -1000 || value.Input.MoveX > 1000 || value.Input.MoveY < -1000 || value.Input.MoveY > 1000 || value.Input.Yaw < 0 || value.Input.Yaw >= 65536 || value.Input.Pitch < -16000 || value.Input.Pitch > 16000 {
			return `{"error":"invalid motion values"}`
		}
		limit := 65536 * engine.Unit
		if value.Player.X < -limit || value.Player.X > limit || value.Player.Y < -limit || value.Player.Y > limit || value.Player.Z < -limit || value.Player.Z > limit {
			return `{"error":"motion position outside world"}`
		}
		player := engine.PredictMotionWithDynamics(world, value.Player, value.Input, value.Frames, value.Brushes, value.Players)
		encoded, err := json.Marshal(player)
		if err != nil {
			return `{"error":"motion result failed"}`
		}
		return string(encoded)
	})
	weapons := js.FuncOf(func(this js.Value, args []js.Value) any {
		encoded, _ := json.Marshal(engine.Weapons())
		return string(encoded)
	})
	canSmooth := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString || world.ID == "" || len(args[0].String()) > 64*1024 {
			return false
		}
		var value displayRequest
		if json.Unmarshal([]byte(args[0].String()), &value) != nil || value.From == nil || value.To == nil || !validPoint(*value.From) || !validPoint(*value.To) || len(value.Brushes) > 256 {
			return false
		}
		hull := 0
		if value.Crouch {
			hull = 1
		}
		state := engine.State{World: world, Brushes: value.Brushes}
		trace := engine.TraceWorld(&state, *value.From, *value.To, hull)
		return trace.Fraction == engine.NormalUnit && !trace.StartSolid && !trace.AllSolid
	})
	api := js.Global().Get("Object").New()
	api.Set("version", 1)
	api.Set("unit", engine.Unit)
	api.Set("fps", engine.FPS)
	api.Set("sourceSHA256", sourceSHA256)
	api.Set("init", initialize)
	api.Set("move", move)
	api.Set("weapons", weapons)
	api.Set("canSmooth", canSmooth)
	js.Global().Set("cs16Motion", api)
	select {}
}
