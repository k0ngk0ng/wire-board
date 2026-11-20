package pokepath_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestNativeTrainersLegacySaveKeepsProgressAndBecomesIndependent(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &envelope); err != nil {
		t.Fatal(err)
	}
	world, err := engine.Unmarshal(envelope["snapshot"])
	if err != nil {
		t.Fatal(err)
	}
	world.Trainers = nil
	world.Team = world.Team[:1]
	world.Progression = engine.ProgressionState{}
	if err := engine.InitProgression(world); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(world, 0, engine.Action{Type: "upgrade", Epoch: world.Epoch, PokemonID: world.Team[0].ID}); err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(world, 0, engine.Action{Type: "deploy", Epoch: world.Epoch, PokemonID: world.Team[0].ID, TileID: 1}); err != nil {
		t.Fatal(err)
	}
	world.Gold, world.Wave, world.RouteWaves[0], world.Records[0] = 314, 12, 12, 11
	oldVersion := "pokepath-native-1/401d71da4e3677ea2f18c69822760e066026d1e96ed303234a43b5e8a7d5a616/" + engine.CatalogData().Source.SourceSHA256
	world.RulesVersion = oldVersion
	envelope["snapshot"], err = json.Marshal(world)
	if err != nil {
		t.Fatal(err)
	}
	catalog := engine.CatalogData()
	identity := fmt.Sprintf("pokepath-independent-go:%d:%d:%s:%s:%s", 2, engine.SchemaVersion, oldVersion, catalog.Version, catalog.Source.SourceSHA256)
	envelope["hash"], _ = json.Marshal(fmt.Sprintf("%x", sha256.Sum256([]byte(identity))))
	match.Data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte{}, match.Data...)
	restored, err := module.Restore(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, match.Data) {
		t.Fatal("migration overwrote the original save candidate")
	}
	ids := map[float64]bool{}
	for actor := 0; actor < 2; actor++ {
		view := trainerData(t, module, restored, actor)
		if view["gold"] != float64(314) || view["wave"] != float64(12) || len(view["team"].([]any)) != 3 || trainerMember(view, 0)["level"] != float64(2) {
			t.Fatalf("migration lost trainer %d progress: gold=%v wave=%v team=%v", actor, view["gold"], view["wave"], view["team"])
		}
		for _, entry := range view["team"].([]any) {
			id := entry.(map[string]any)["id"].(float64)
			if ids[id] {
				t.Fatal("migrated trainers share a Pokémon identifier")
			}
			ids[id] = true
		}
	}
	second, err := module.Restore(ctx, restored)
	if err != nil {
		t.Fatal(err)
	}
	if len(trainerData(t, module, second, 1)["team"].([]any)) != 3 || trainerData(t, module, second, 1)["gold"] != float64(314) {
		t.Fatal("repeated restore duplicated initial grants")
	}
}
