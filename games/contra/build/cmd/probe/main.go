//go:build !js

// Local acceptance executable. This is not the application's composition root.
package main

import (
	"encoding/json"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	contra "github.com/k0ngk0ng/wire-board/games/contra/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"log"
	"os"
	"time"
)

func main() {
	module, err := contra.NewProductionModule()
	if err != nil {
		log.Fatal(err)
	}
	if module.BotAction != nil || module.Reclaim != nil || module.BotSearch != nil {
		log.Fatal("human-only production module exposes computer capabilities")
	}
	context := gameapi.Context{Actor: 0, Players: 1, Automated: []bool{false}, Now: time.UnixMilli(100000)}
	match, err := module.Start(context, json.RawMessage(`{}`))
	if err != nil {
		log.Fatal(err)
	}
	context.Actor = -1
	for range 4 {
		context.Now = context.Now.Add(50 * time.Millisecond)
		match, err = module.OnTimer(context, match)
		if err != nil {
			log.Fatal(err)
		}
	}
	view, err := module.View(match, 0)
	if err != nil {
		log.Fatal(err)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		log.Fatal(err)
	}
	var result contra.View
	if err = json.Unmarshal(raw, &result); err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"rulesBuildID": runtime.CompiledRulesBuildID, "runtimeBuildID": runtime.CompiledRuntimeBuildID, "visualsID": result.VisualsID, "frames": result.Snapshot.Frame, "revision": result.Revision, "phase": result.Snapshot.Phase, "registered": false}); err != nil {
		log.Fatal(err)
	}
}
