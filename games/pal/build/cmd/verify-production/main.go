package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func main() {
	path := flag.String("package", "", "prepared server package directory")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	bundle, err := pal.OpenProductionBundle(ctx, os.DirFS(*path))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer bundle.Close(context.Background())
	if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"passed": true, "identity": bundle.Identity, "singlePlayer": bundle.Module.MinPlayers == 1 && bundle.Module.MaxPlayers == 1, "supportsSave": bundle.Module.SupportsSave, "supportsCheckpoint": bundle.Module.SupportsCheckpoint}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
