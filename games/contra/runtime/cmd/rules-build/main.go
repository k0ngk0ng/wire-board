//go:build !js

package main

import (
	"encoding/json"
	"flag"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"log"
	"os"
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	id, manifest, err := runtime.ReadRulesBuild(*root)
	if err != nil {
		log.Fatal(err)
	}
	bundle, err := engine.BuiltinLevelBundle()
	if err != nil {
		log.Fatal(err)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"id": id, "manifest": manifest, "bundle": bundle.Manifest()})
}
