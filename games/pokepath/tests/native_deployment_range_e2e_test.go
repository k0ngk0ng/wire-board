package pokepath_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func checkCandidateRange(t *testing.T, state *engine.State, id, tileID int, expected float64, shape string, halfWidth float64) engine.DeploymentRange {
	t.Helper()
	before, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := engine.ProjectDeploymentRange(state, id, tileID)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(preview.Range-expected) > 1e-9 || preview.Shape != shape || preview.HalfWidth != halfWidth {
		t.Fatalf("tile %d range=%+v, want %g %s %g", tileID, preview, expected, shape, halfWidth)
	}
	copy, err := engine.Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	waApply(t, copy, 3, engine.Action{Type: "deploy", PokemonID: id, TileID: tileID})
	deployed, err := engine.ProjectPokemon(copy, id)
	if err != nil || deployed.CombatStats == nil || math.Abs(preview.Range-deployed.CombatStats.Range) > 1e-9 || preview.Shape != deployed.CombatRangeType || preview.InnerRange != deployed.CombatInnerRange || preview.HalfWidth != deployed.CombatRangeHalfWidth {
		t.Fatalf("preview does not agree with the deployed partner: %+v %+v %v", preview, deployed.CombatStats, err)
	}
	after, err := engine.Marshal(state)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("looking at a tile changed the room", err)
	}
	return preview
}

func TestNativeDeploymentRangeUsesTheDestinationAndLiveSupport(t *testing.T) {
	state := waScenario(t)
	scout := waPartner(t, state, "pidgey", 5)
	light := waPartner(t, state, "patrat", 5)
	field := waPlacement(t, state, 1, 1, 2)
	high := waPlacement(t, state, 4)
	var neighbor engine.Placement
	seen := false
	for _, tile := range engine.Project(state).Tiles {
		if tile.ID != field.ID && tile.Terrain != 3 && math.Hypot(tile.X-field.X, tile.Y-field.Y) < 100 && math.Hypot(tile.X-high.X, tile.Y-high.Y) > 130 {
			neighbor, seen = tile, true
			break
		}
	}
	if !seen {
		t.Fatal("route fixture lacks a tile that illuminates only the field")
	}
	waApply(t, state, 1, engine.Action{Type: "deploy", PokemonID: light, TileID: neighbor.ID})
	checkCandidateRange(t, state, scout, field.ID, 140, "circle", 24)
	checkCandidateRange(t, state, scout, high.ID, 250, "circle", 24)
	waApply(t, state, 2, engine.Action{Type: "deploy", PokemonID: scout, TileID: field.ID})
	checkCandidateRange(t, state, scout, high.ID, 250, "circle", 24)
	waApply(t, state, 1, engine.Action{Type: "retire", PokemonID: light})
	checkCandidateRange(t, state, scout, field.ID, 125, "circle", 24)
	if _, err := engine.ProjectDeploymentRange(state, scout, waPlacement(t, state, 3).ID); err == nil {
		t.Fatal("ineligible tile returned a footprint")
	}
}

func TestNativeDeploymentRangePreservesSpecialShapesAndNoGuard(t *testing.T) {
	state := waScenario(t)
	guard := waPartner(t, state, "megaPidgeot", 50)
	star := waPartner(t, state, "yamask", 10)
	active := waPartner(t, state, "victini", 50)
	ring := waPartner(t, state, "hoothoot", 20)
	state.Team[1].Item = "starPiece"
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "starPiece", EquippedBy: star})
	field := waPlacement(t, state, 1)
	other := waPlacement(t, state, 1, field.ID)
	high := waPlacement(t, state, 4)
	checkCandidateRange(t, state, guard, high.ID, 2200, "circle", 24)
	waApply(t, state, 1, engine.Action{Type: "deploy", PokemonID: active, TileID: field.ID})
	checkCandidateRange(t, state, star, other.ID, state.Team[1].Stats.Range, "star", 36)
	value := checkCandidateRange(t, state, ring, other.ID, state.Team[3].Stats.Range, "donut", 36)
	if value.InnerRange != 100 {
		t.Fatalf("donut hole = %v", value.InnerRange)
	}
}

func TestNativeDeploymentRangeDoublesSandShotOnlyOnTheRecordedRoutes(t *testing.T) {
	state := waScenario(t)
	id := waPartner(t, state, "cacnea", 20)
	base := state.Team[0].Stats.Range
	checkCandidateRange(t, state, id, waPlacement(t, state, 1).ID, base, "cross", 24)
	state.Stars = 500
	waApply(t, state, 0, engine.Action{Type: "route", Route: 3})
	checkCandidateRange(t, state, id, waPlacement(t, state, 1).ID, base*2, "cross", 24)
}
