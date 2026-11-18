package pokepath_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func editorStart(t *testing.T) *engine.State {
	t.Helper()
	state, err := engine.New(engine.Options{Starter: "charmander"}, 2, 8191, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func editorAct(t *testing.T, state *engine.State, action engine.Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := engine.Apply(state, 1, action); err != nil {
		t.Fatal(err)
	}
}

func TestNativeEditorValidatesBeforeMutating(t *testing.T) {
	state := editorStart(t)
	before, _ := engine.Marshal(state)
	for _, raw := range []string{
		`{"mapId":9999,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`,
		`{"mapId":0,"hearts":15,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`,
		`{"mapId":0,"hearts":14,"health":1,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"unknown","amount":1}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1000000000}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":-1}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1.5}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":null,"amount":2}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"gold":999999,"wave":[{"enemy":"rattata","amount":1}]}`,
		`{"mapId":0,"hearts":14,"health":0,"armor":0,"returnHealth":999,"wave":[{"enemy":"rattata","amount":1}]}`,
		`{} {}`,
	} {
		if err := engine.Apply(state, 0, engine.Action{Type: "custom", Epoch: state.Epoch, Settings: json.RawMessage(raw)}); err == nil {
			t.Fatalf("accepted invalid editor input: %s", raw)
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid custom wave changed formal progress")
		}
	}
}

func TestNativeEditorWorldIsLocalAndCompleteInSave(t *testing.T) {
	state := editorStart(t)
	other := editorStart(t)
	catalogBefore, _ := json.Marshal(engine.CatalogData())
	state.Health = 9
	state.Wave = 5
	editorAct(t, state, engine.Action{Type: "custom", Settings: json.RawMessage(`{"mapId":0,"hearts":8,"health":25,"armor":10,"wave":[{"enemy":"rattata","amount":2},{"enemy":null,"amount":1},{"enemy":"ledyba","amount":1}]}`)})
	if state.Progression.CustomRoute == nil || other.Progression.CustomRoute != nil || state.Health != 8 || state.Wave != 1 {
		t.Fatal("custom wave was not isolated")
	}
	catalogAfter, _ := json.Marshal(engine.CatalogData())
	if !bytes.Equal(catalogBefore, catalogAfter) {
		t.Fatal("editor modified shared catalog")
	}
	editorAct(t, state, engine.Action{Type: "wave"})
	if err := engine.Advance(state, 1200); err != nil {
		t.Fatal(err)
	}
	raw, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, milliseconds := range []int64{20, 70, 110, 500} {
		if err := engine.Advance(state, milliseconds); err != nil {
			t.Fatal(err)
		}
		if err := engine.Advance(restored, milliseconds); err != nil {
			t.Fatal(err)
		}
		left, _ := engine.Marshal(state)
		right, _ := engine.Marshal(restored)
		if !bytes.Equal(left, right) {
			t.Fatal("custom battle diverged after restoration")
		}
	}
	gold, stars := state.Gold, state.Stars
	for index := 0; index < 1000 && state.WaveActive; index++ {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if state.WaveActive || state.Gold != gold || state.Stars != stars || state.Records[0] != 0 {
		t.Fatal("practice failed to finish or granted permanent rewards")
	}
	editorAct(t, state, engine.Action{Type: "custom_exit"})
	if state.Progression.CustomRoute != nil || state.Health != 9 || state.Wave != 5 || state.Route != 0 {
		t.Fatal("leaving practice lost the formal route state")
	}
}

func TestNativeEditorShareCodeUsesOnlyValidatedData(t *testing.T) {
	data := engine.EditorData{MapID: 0, Hearts: 14, Health: 20, Armor: 5, Wave: []engine.EditorSlot{{Enemy: "rattata", Amount: 3}, {Amount: 1}}}
	code, err := engine.EncodeEditorCode(data)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := engine.DecodeEditorCode(code)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(data)
	right, _ := json.Marshal(decoded)
	if !bytes.Equal(left, right) {
		t.Fatal("editor code did not round trip")
	}
	for _, invalid := range []string{"not base64", "e30=", code + "%%"} {
		if _, err := engine.DecodeEditorCode(invalid); err == nil {
			t.Fatal("accepted invalid custom code")
		}
	}
}

func TestNativeEditorClearsFormalWaveManagement(t *testing.T) {
	state := editorStart(t)
	state.Records[state.Route] = 100
	state.Stars = 100
	stock := -1
	for index, item := range state.Progression.ShopStock {
		if item == "bicycle" {
			stock = index
			break
		}
	}
	if stock < 0 {
		t.Fatal("fixture has no bicycle offer")
	}
	state.Gold = engine.CatalogData().Items["bicycle"].Price
	editorAct(t, state, engine.Action{Type: "buyItem", Stock: stock, ItemID: "bicycle"})
	state.Progression.WaveRepeat = true
	state.Progression.WaveMirror = true
	state.Progression.ActiveMirror = true
	editorAct(t, state, engine.Action{Type: "custom", Settings: json.RawMessage(`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`)})
	if state.Progression.WaveRepeat || state.Progression.WaveMirror || state.Progression.ActiveMirror {
		t.Fatal("practice inherited formal wave repeat or mirror state")
	}
}

func TestNativeEditorClearsFormalWorldAndAim(t *testing.T) {
	state := editorStart(t)
	pokemon := state.Team[0]
	route := engine.CatalogData().Routes["0"]
	terrain := engine.CatalogData().Species[pokemon.Species].Terrains[0]
	tile := -1
	for _, placement := range route.Placements {
		if placement.Terrain == terrain {
			tile = placement.ID
			break
		}
	}
	if tile < 0 {
		t.Fatal("fixture has no deployment tile")
	}
	editorAct(t, state, engine.Action{Type: "deploy", PokemonID: pokemon.ID, TileID: tile})
	state.World.Honey = 42
	state.AttackModes.Towers = map[int]*engine.AttackModeTower{pokemon.ID: {TowerID: state.Towers[0].ID, HasAim: true, Aim: engine.Point{X: 252, Y: 132}}}
	editorAct(t, state, engine.Action{Type: "custom", Settings: json.RawMessage(`{"mapId":0,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`)})
	if state.World.Honey != 0 || len(state.World.Towers) != 0 || len(state.AttackModes.Towers) != 0 {
		t.Fatal("practice inherited formal world or aiming state")
	}
}

func TestNativeEditorRetryKeepsThePracticeDeployment(t *testing.T) {
	state := editorStart(t)
	editorAct(t, state, engine.Action{Type: "custom", Settings: json.RawMessage(`{"mapId":0,"hearts":3,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`)})
	id := state.Team[0].ID
	editorAct(t, state, engine.Action{Type: "deploy", PokemonID: id, TileID: 1})
	tower := state.Towers[0]
	editorAct(t, state, engine.Action{Type: "wave"})
	for index := 0; index < 200 && state.WaveActive; index++ {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	editorAct(t, state, engine.Action{Type: "custom_retry"})
	if state.Phase != "prepare" || state.Health != 3 || len(state.Towers) != 1 || state.Towers[0].ID != tower.ID || state.Towers[0].PokemonID != id || state.Towers[0].TileID != tower.TileID || state.Team[0].TileID != tower.TileID {
		t.Fatal("retrying a practice erased the original deployment")
	}
}
