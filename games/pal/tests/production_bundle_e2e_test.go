package pal_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"testing/fstest"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestProductionAuthorityBundleRejectsDevelopmentAndPreservesLegacySave(t *testing.T) {
	if os.Getenv("PAL_PRODUCTION_E2E") != "1" {
		t.Skip("explicit production-only authority bundle and retained-core validation")
	}
	ctx := context.Background()
	current := "4aeeca84606436fc071b1b9004014ca11767ec048e8c3be5d411c6c0370d6ba3"
	legacy := "7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a"
	archive, err := os.ReadFile("../../../.local/pal/authority-prepared/authority.zip")
	if err != nil {
		t.Fatal(err)
	}
	currentCore, err := os.ReadFile("../../../.local/pal/runtime-candidates/audio/original/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	legacyCore, err := os.ReadFile("../../../.local/pal/runtime-versions/" + legacy + "/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	archiveHash := sha256.Sum256(archive)
	manifest := map[string]any{"format": 1, "resourceID": "58b3fddfd1bcdc37b6816cfd3ddba8a821683ff6f13b4cd5cbbab6f8ffe7255c", "authoritySHA256": hex.EncodeToString(archiveHash[:]), "current": current, "retained": []string{legacy}}
	makeFiles := func(value map[string]any, core, old []byte) fstest.MapFS {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return fstest.MapFS{"manifest.json": {Data: data}, "authority.zip": {Data: archive}, "cores/" + current + ".wasm": {Data: core}, "cores/" + legacy + ".wasm": {Data: old}}
	}
	files := makeFiles(manifest, currentCore, legacyCore)
	bundle, err := pal.OpenProductionBundle(ctx, files)
	if err != nil {
		t.Fatal(err)
	}
	defer bundle.Close(ctx)
	if bundle.Identity.BuildID != current || bundle.Identity.Profile != "original-98" || bundle.Module.MinPlayers != 1 || bundle.Module.MaxPlayers != 1 || !bundle.Module.SupportsSave || !bundle.Module.SupportsCheckpoint {
		t.Fatal("production bundle selected wrong core, profile or platform capabilities", bundle.Identity)
	}
	saved, err := os.ReadFile("../../../.local/pal/verification/campaign-tournament-state-1.json")
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(saved)
	c := gameapi.Context{Players: 1, Now: time.Unix(1800000000, 0)}
	state, err := bundle.Module.ResumeSaved(c, gameapi.Match{Schema: 1, Data: saved})
	if err != nil {
		t.Fatal("production retained runner could not resume original stage4 save", err)
	}
	v, err := bundle.Module.View(state, 0)
	if err != nil || !bytes.Equal(before, saved) {
		t.Fatal("production retained runner changed source bytes", err)
	}
	view := v.(pal.View)
	if view.Scene != 40 || len(view.Party) != 2 || view.Party[0].Role != 2 || view.Party[1].Role != 0 {
		t.Fatal("production retained original save lost its actual manor ending", view.Scene, view.Party)
	}
	for _, change := range []string{"missing-core", "changed-core", "wrong-resource", "duplicate-core", "extra-client-media"} {
		t.Run(change, func(t *testing.T) {
			value := map[string]any{}
			for key, item := range manifest {
				value[key] = item
			}
			x := makeFiles(value, currentCore, legacyCore)
			switch change {
			case "missing-core":
				delete(x, "cores/"+current+".wasm")
			case "changed-core":
				x["cores/"+current+".wasm"].Data = []byte("wrong core")
			case "wrong-resource":
				value["resourceID"] = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
				x = makeFiles(value, currentCore, legacyCore)
			case "duplicate-core":
				value["retained"] = []string{current}
				x = makeFiles(value, currentCore, legacyCore)
			case "extra-client-media":
				x["music.flac"] = &fstest.MapFile{Data: []byte("client sound")}
			}
			if bad, err := pal.OpenProductionBundle(ctx, x); err == nil {
				bad.Close(ctx)
				t.Fatal("invalid production authority was accepted", change)
			}
		})
	}
	development, err := os.ReadFile("../../../.local/pal/runtime-candidates/audio/hp-9999/rules.wasm")
	if err != nil {
		t.Fatal(err)
	}
	developmentHash := sha256.Sum256(development)
	developmentID := hex.EncodeToString(developmentHash[:])
	devManifest := map[string]any{"format": 1, "resourceID": manifest["resourceID"], "authoritySHA256": manifest["authoritySHA256"], "current": developmentID, "retained": []string{}}
	encoded, _ := json.Marshal(devManifest)
	devFiles := fstest.MapFS{"manifest.json": {Data: encoded}, "authority.zip": {Data: archive}, "cores/" + developmentID + ".wasm": {Data: development}}
	if bad, err := pal.OpenProductionBundle(ctx, devFiles); err == nil {
		bad.Close(ctx)
		t.Fatal("actual development-health binary entered the production loader")
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original current+retained authority package, unchanged real stage4 legacy save, single-player private-save/checkpoint capabilities, corrupted/missing/duplicate/media rejection and actual HP9999 production rejection; no formal registry or server-image claim", "current": current, "retained": legacy, "archiveSHA256": manifest["authoritySHA256"], "legacyScene": view.Scene}, "", "  ")
	if err = os.WriteFile("../../../.local/pal/verification/production-authority-bundle.json", append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
