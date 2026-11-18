package pokepath_test

import (
	"bytes"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestNativeDeploymentPreviewUsesTheActionRulesWithoutChangingState(t *testing.T) {
	state := waScenario(t)
	host := waPartner(t, state, "turtwig", 30)
	ordinary := waPartner(t, state, "charmander", 5)
	walker := waPartner(t, state, "rattata", 25)
	warp := waPartner(t, state, "celebi", 25)
	state.Team[2].Item = "airBalloon"
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "airBalloon", EquippedBy: walker})
	shared := waPlacement(t, state, 1)
	used := waPlacement(t, state, 1, shared.ID)
	high := waPlacement(t, state, 4)
	water := waPlacement(t, state, 3)
	open := waPlacement(t, state, 1, shared.ID, used.ID)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: host, TileID: shared.ID})
	waApply(t, state, 1, engine.Action{Type: "deploy", PokemonID: ordinary, TileID: used.ID})
	check := func(id int, expected map[int]string) {
		t.Helper()
		before, err := engine.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		preview := engine.ProjectDeployment(state, id)
		if preview.TileSize != 24 {
			t.Fatalf("wrong deployment grid: %+v", preview)
		}
		for _, tile := range engine.Project(state).Tiles {
			kind := preview.Sites[tile.ID]
			if value, exists := expected[tile.ID]; exists && kind != value {
				t.Fatalf("tile %d: preview=%q want %q", tile.ID, kind, value)
			}
			copy, err := engine.Clone(state)
			if err != nil {
				t.Fatal(err)
			}
			canDeploy := engine.Apply(copy, 3, engine.Action{Epoch: state.Epoch, Type: "deploy", PokemonID: id, TileID: tile.ID}) == nil
			if canDeploy != (kind == "empty" || kind == "host") {
				t.Fatalf("tile %d: preview=%q actual deployment=%v", tile.ID, kind, canDeploy)
			}
		}
		after, err := engine.Marshal(state)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("preview changed the authoritative state", err)
		}
	}
	check(walker, map[int]string{shared.ID: "host", used.ID: "occupied", open.ID: "empty", high.ID: "empty", water.ID: "blocked"})
	waApply(t, state, 2, engine.Action{Type: "deploy", PokemonID: walker, TileID: shared.ID})
	check(warp, map[int]string{shared.ID: "occupied"})
	waApply(t, state, 3, engine.Action{Type: "pause"})
	check(ordinary, map[int]string{open.ID: "blocked"})
	check(warp, map[int]string{open.ID: "empty"})
	waApply(t, state, 3, engine.Action{Type: "resume"})
	state.World.Pressure = true
	check(walker, map[int]string{open.ID: "blocked", high.ID: "blocked"})
}
