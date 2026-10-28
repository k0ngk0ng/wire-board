//go:build !js

package main

import (
	"encoding/json"
	"flag"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	contra "github.com/k0ngk0ng/wire-board/games/contra/server"
	"log"
	"os"
)

func main() {
	root := flag.String("root", ".", "source checkout")
	flag.Parse()
	id, manifest, err := runtime.ReadRulesBuild(*root)
	if err != nil {
		log.Fatal(err)
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		log.Fatal(err)
	}
	visuals, visualID, err := contra.ProductionVisualManifest()
	if err != nil {
		log.Fatal(err)
	}
	if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"rules": map[string]any{"id": id, "manifest": manifest, "bundle": bundle.Manifest()}, "visuals": map[string]any{"id": visualID, "manifest": visuals}}); err != nil {
		log.Fatal(err)
	}
}
