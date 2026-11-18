package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func progressionEvidence(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		path := filepath.Join("..", "..", "..", ".local", "pokepath", "verification", "native-progression")
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Error(err)
			return
		}
		value, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "command": ". scripts/env.sh && go test ./games/pokepath/tests -run '^TestNativeProgression' -count=1", "implementation": "independent Go rules; no original game runtime"}, "", "  ")
		if err := os.WriteFile(filepath.Join(path, strings.ReplaceAll(t.Name(), "/", "-")+".json"), append(value, '\n'), 0600); err != nil {
			t.Error(err)
		}
	})
}

func progressionNew(t *testing.T) *engine.State {
	t.Helper()
	progressionEvidence(t)
	state, err := engine.New(engine.Options{Starter: "charmander"}, 4, 87654321, 1794919872358)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func progressionAct(t *testing.T, state *engine.State, actor int, action engine.Action) {
	t.Helper()
	action.Epoch = state.Epoch
	if err := engine.Apply(state, actor, action); err != nil {
		t.Fatalf("%s: %v", action.Type, err)
	}
}

func TestNativeProgressionInitialSharedShopE2E(t *testing.T) {
	state := progressionNew(t)
	view := engine.Project(state).Progression
	if view.EggPrice != 10 || view.RemainingEggs != 149 || len(view.Shop) != 10 {
		t.Fatalf("invalid initial shared shop: %+v", view)
	}
	progressionAct(t, state, 3, engine.Action{Type: "egg"})
	if state.Gold != 40 || len(state.Team) != 2 || state.Progression.EggPrice != 47 || len(state.Progression.EggPool) != 148 {
		t.Fatal("egg purchase did not preserve the original economy", state.Gold, state.Progression.EggPrice, len(state.Team))
	}
	firstID := state.Team[1].ID
	progressionAct(t, state, 1, engine.Action{Type: "team", PokemonID: firstID, Mode: "box"})
	if len(state.Team) != 1 || len(state.Box) != 1 || state.Box[0].ID != firstID {
		t.Fatal("stable owned identity was lost during box transfer")
	}
	progressionAct(t, state, 2, engine.Action{Type: "team", PokemonID: firstID, Mode: "team"})
	if len(state.Team) != 2 || len(state.Box) != 0 || state.Team[1].ID != firstID {
		t.Fatal("box transfer did not return the same Pokémon")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "egg", Epoch: state.Epoch}); err == nil {
		t.Fatal("the same shared gold was spent twice")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("failed purchase changed resources or random state")
	}
}

func TestNativeProgressionAtomicRemoveAllToBoxE2E(t *testing.T) {
	state := progressionNew(t)
	state.Gold = 1000000
	id := state.Team[0].ID
	progressionAct(t, state, 0, engine.Action{Type: "egg"})
	progressionAct(t, state, 1, engine.Action{Type: "egg"})
	progressionAct(t, state, 2, engine.Action{Type: "team", PokemonID: id, Mode: "box"})
	progressionAct(t, state, 2, engine.Action{Type: "team", PokemonID: id, Mode: "team"})
	progressionAct(t, state, 3, engine.Action{Type: "deploy", PokemonID: id, TileID: 1})
	locked, err := engine.Clone(state)
	if err != nil {
		t.Fatal(err)
	}
	locked.World.MovementLocked = true
	before, _ := engine.Marshal(locked)
	if err := engine.Apply(locked, 0, engine.Action{Type: "teamAll", Mode: "box", Epoch: locked.Epoch}); err == nil {
		t.Fatal("remove-all bypassed the late deployed member's movement lock")
	}
	after, _ := engine.Marshal(locked)
	if !bytes.Equal(before, after) {
		t.Fatal("remove-all partially moved members before a later rejection")
	}
	ids := []int{}
	for _, pokemon := range state.Team {
		ids = append(ids, pokemon.ID)
	}
	progressionAct(t, state, 0, engine.Action{Type: "teamAll", Mode: "box"})
	if len(state.Team) != 0 || len(state.Box) != 3 || len(state.Towers) != 0 {
		t.Fatal("remove-all did not retire towers and transfer the entire shared team")
	}
	for _, pokemon := range state.Box {
		if !slices.Contains(ids, pokemon.ID) || pokemon.TileID != -1 {
			t.Fatal("remove-all changed owned identity or left a deployment")
		}
	}
}

func TestNativeProgressionWorldLocksCoverManagementE2E(t *testing.T) {
	state := progressionNew(t)
	state.Gold = 1000000
	id := state.Team[0].ID
	progressionAct(t, state, 0, engine.Action{Type: "buyItem", Stock: 0, ItemID: "protein"})
	progressionAct(t, state, 0, engine.Action{Type: "equip", PokemonID: id, ItemID: "protein"})
	progressionAct(t, state, 0, engine.Action{Type: "saveTeam", Slot: 0})
	if err := engine.WorldEnemyEvent(state, engine.EnemyPassiveEvent{Kind: "pressure"}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []engine.Action{{Type: "team", PokemonID: id, Mode: "box"}, {Type: "teamAll", Mode: "box"}, {Type: "equip", PokemonID: id, ItemID: "protein"}, {Type: "unequip", PokemonID: id}, {Type: "loadTeam", Slot: 0}} {
		t.Run(action.Type, func(t *testing.T) {
			copy, err := engine.Clone(state)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := engine.Marshal(copy)
			action.Epoch = copy.Epoch
			if err := engine.Apply(copy, 1, action); err == nil {
				t.Fatal("pressure did not lock an existing management action")
			}
			after, _ := engine.Marshal(copy)
			if !bytes.Equal(before, after) {
				t.Fatal("pressure rejection partially changed shared state")
			}
		})
	}
	state.World.Pressure = false
	if err := engine.Apply(state, 0, engine.Action{Type: "loadTeam", Slot: 0, Epoch: state.Epoch}); err == nil {
		t.Fatal("heavy-metal movement lock allowed formation replacement without tower placements")
	}
}

func TestNativeProgressionSettingsSecretAndContinuationE2E(t *testing.T) {
	state := progressionNew(t)
	for key, value := range map[string]float64{"audio.master": 0, "audio.music": 4, "audio.ui": 4, "audio.effects": 1} {
		progressionAct(t, state, 0, engine.Action{Type: "settings", Name: key, Value: value})
	}
	if !state.Progression.Secrets["chatot"] {
		t.Fatal("the real audio combination did not grant Chatot")
	}
	count := len(state.Team) + len(state.Box)
	progressionAct(t, state, 3, engine.Action{Type: "settings", Name: "audio.effects", Value: 1})
	if len(state.Team)+len(state.Box) != count {
		t.Fatal("audio secret granted duplicate owned Pokémon")
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	progressionAct(t, state, 1, engine.Action{Type: "egg"})
	progressionAct(t, restored, 1, engine.Action{Type: "egg"})
	left, _ := engine.Marshal(state)
	right, _ := engine.Marshal(restored)
	if !bytes.Equal(left, right) {
		t.Fatal("restored shop/random/secret state diverged")
	}
}

func TestNativeProgressionCustomPracticeCannotGrantPermanentSecretsE2E(t *testing.T) {
	state := progressionNew(t)
	state.Stars = 10000
	progressionAct(t, state, 0, engine.Action{Type: "custom", Settings: json.RawMessage(`{"mapId":4,"hearts":14,"health":0,"armor":0,"wave":[{"enemy":"rattata","amount":1}]}`)})
	owned := len(state.Team) + len(state.Box)
	for key, value := range map[string]float64{"audio.master": 0, "audio.music": 4, "audio.ui": 4, "audio.effects": 1} {
		progressionAct(t, state, 1, engine.Action{Type: "settings", Name: key, Value: value})
	}
	if state.Progression.Secrets["chatot"] || len(state.Team)+len(state.Box) != owned {
		t.Fatal("practice audio settings granted a permanent secret")
	}
	if len(engine.Project(state).Progression.Hotspots) != 0 {
		t.Fatal("practice exposed permanent map reward hotspots")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 2, engine.Action{Type: "secret", Name: "cacnea", X: 179, Y: 107, Epoch: state.Epoch}); err == nil {
		t.Fatal("practice map accepted a permanent secret action")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected practice secret changed collection or shared state")
	}
}

func TestNativeProgressionInvalidRequestsAreAtomicE2E(t *testing.T) {
	state := progressionNew(t)
	before, _ := engine.Marshal(state)
	for _, action := range []engine.Action{
		{Type: "buyItem", ItemID: "not-owned", Stock: 999},
		{Type: "team", PokemonID: state.Team[0].ID, Mode: "missing"},
		{Type: "equip", PokemonID: state.Team[0].ID, ItemID: "protein"},
		{Type: "saveTeam", Slot: 5},
		{Type: "loadTeam", Slot: -1},
		{Type: "secret", Name: "cacnea", X: 0, Y: 0},
		{Type: "settings", Name: "audio.effects", Value: 21},
		{Type: "settings", Name: "effectAlpha", Value: 5.5},
		{Type: "settings", Name: "gold", Value: 999999},
		{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50,"noItems":true}`)},
		{Type: "draftPick", Pick: 0},
		{Type: "claimReward", Slot: 0},
	} {
		action.Epoch = state.Epoch
		if err := engine.Apply(state, 1, action); err == nil {
			t.Fatal("invalid progression request was accepted", action.Type)
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid request partially mutated shared world", action.Type)
		}
	}
}

func TestNativeProgressionSavedFormationUsesRealPlacementE2E(t *testing.T) {
	state := progressionNew(t)
	pokemonID := state.Team[0].ID
	tileID := -1
	for _, tile := range engine.Project(state).Tiles {
		if tile.Terrain == 1 {
			tileID = tile.ID
			break
		}
	}
	if tileID < 0 {
		t.Fatal("route has no legal placement")
	}
	progressionAct(t, state, 1, engine.Action{Type: "deploy", PokemonID: pokemonID, TileID: tileID})
	progressionAct(t, state, 0, engine.Action{Type: "saveTeam", Slot: 0})
	progressionAct(t, state, 2, engine.Action{Type: "retire", PokemonID: pokemonID})
	progressionAct(t, state, 3, engine.Action{Type: "loadTeam", Slot: 0})
	if len(state.Towers) != 1 || state.Towers[0].PokemonID != pokemonID || state.Towers[0].TileID != tileID || state.Team[0].TileID != tileID {
		t.Fatal("formation did not restore the actual selected tower")
	}
}

func progressionChallengeFixture(t *testing.T) *engine.State {
	t.Helper()
	state := progressionNew(t)
	state.Gold = 1000000000
	state.Stars = 540
	state.TeamSlots = 10
	for route := 0; route < 5; route++ {
		state.Records[route] = 100
	}
	state.Records[5] = 40
	for len(state.Team)+len(state.Box) < 30 {
		progressionAct(t, state, 0, engine.Action{Type: "egg"})
	}
	return state
}

func TestNativeProgressionChallengeDraftAndRewardE2E(t *testing.T) {
	state := progressionChallengeFixture(t)
	progressionAct(t, state, 2, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50,"slotLimit":6,"toughEnemies":50,"draft":true,"noItems":true,"permadeath":true,"mirror":true}`)})
	if !state.Progression.Challenge.Active || !state.Progression.Draft.Active || state.Progression.Draft.Rounds != 6 || len(state.Team) != 0 || len(state.Box) != 30 || state.Health != 14 || state.Wave != 1 {
		t.Fatal("challenge did not establish a complete real draft", state.Progression.Challenge, state.Progression.Draft)
	}
	seen := map[int]bool{}
	for state.Progression.Draft.Active {
		draft := state.Progression.Draft
		if len(draft.Offers) == 0 || len(draft.Offers) > 3 || seen[draft.Offers[0]] {
			t.Fatal("invalid or duplicate draft offer", draft)
		}
		seen[draft.Offers[0]] = true
		progressionAct(t, state, 1, engine.Action{Type: "draftPick", Pick: 0, TargetID: draft.Token})
		before, _ := engine.Marshal(state)
		if err := engine.Apply(state, 3, engine.Action{Type: "draftPick", Pick: 0, TargetID: draft.Token, Epoch: state.Epoch}); err == nil {
			t.Fatal("stale draft token selected another member")
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("stale draft selection partially mutated state")
		}
	}
	if len(state.Team) != 6 || len(state.Box) != 24 {
		t.Fatal("draft did not honor the shared challenge capacity")
	}
	state.Wave = 100
	engine.OnWaveCompleted(state)
	if state.Ribbons != 10 || engine.Project(state).Progression.CurrentRibbons != 10 {
		t.Fatal("strict challenge completion did not award all ten ribbon tiers", state.Ribbons)
	}
	engine.OnWaveCompleted(state)
	if state.Ribbons != 10 {
		t.Fatal("repeat completion duplicated challenge ribbons")
	}
	progressionAct(t, state, 3, engine.Action{Type: "claimReward", Slot: 0})
	if len(state.Inventory) != 1 || state.Inventory[0].ID != "leftovers" {
		t.Fatal("route ribbon reward did not grant the real item")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "claimReward", Slot: 0, Epoch: state.Epoch}); err == nil {
		t.Fatal("route reward was claimable twice")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("duplicate reward claim changed the world")
	}
}

func TestNativeProgressionNonDraftChallengeCanBuildActualTeamE2E(t *testing.T) {
	state := progressionChallengeFixture(t)
	id := state.Team[0].ID
	level := state.Team[0].Level
	progressionAct(t, state, 0, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":75,"slotLimit":6}`)})
	progressionAct(t, state, 1, engine.Action{Type: "team", PokemonID: id, Mode: "team"})
	pokemon := progressionMember(state, id)
	specification := engine.CatalogData().Species[pokemon.Species]
	if pokemon.Level != level || pokemon.Stats.Power != math.Floor(specification.PowerBase+specification.PowerPerLevel*75) {
		t.Fatal("non-draft challenge failed to normalize the real team's combat level")
	}
	route := engine.CatalogData().Routes["0"]
	tile := -1
	for _, placement := range route.Placements {
		if slices.Contains(specification.Terrains, placement.Terrain) {
			tile = placement.ID
			break
		}
	}
	if tile < 0 {
		t.Fatal("fixture partner has no valid initial placement")
	}
	progressionAct(t, state, 2, engine.Action{Type: "deploy", PokemonID: id, TileID: tile})
	progressionAct(t, state, 3, engine.Action{Type: "wave"})
	if !state.WaveActive || len(state.Towers) != 1 || len(state.Enemies) == 0 {
		t.Fatal("non-draft challenge could not begin an actual playable wave")
	}
	progressionAct(t, state, 0, engine.Action{Type: "surrender"})
	pokemon = progressionMember(state, id)
	if state.Progression.Challenge.Active || pokemon.Level != level || pokemon.Stats.Power != math.Floor(specification.PowerBase+specification.PowerPerLevel*float64(level)) {
		t.Fatal("surrender changed owned progression or retained normalized combat stats")
	}
}

func TestNativeProgressionPermadeathRetryCancelsChallengeWithoutLosingCollectionE2E(t *testing.T) {
	for _, mode := range []string{"checkpoint", "restart"} {
		t.Run(mode, func(t *testing.T) {
			state := progressionChallengeFixture(t)
			id := state.Team[0].ID
			owned := len(state.Team) + len(state.Box)
			progressionAct(t, state, 0, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50,"permadeath":true}`)})
			progressionAct(t, state, 1, engine.Action{Type: "team", PokemonID: id, Mode: "team"})
			state.Health = 0
			state.Wave = 30
			state.Phase = "defeat"
			progressionAct(t, state, 2, engine.Action{Type: "retry", Mode: mode})
			wave := 25
			health := 10
			if mode == "restart" {
				wave = 1
				health = 14
			}
			if state.Progression.Challenge.Active || len(state.Team)+len(state.Box) != owned || progressionMember(state, id) == nil || state.Wave != wave || state.Health != health || state.Phase != "prepare" {
				t.Fatal("permadeath retry did not cancel the challenge and preserve actual collection")
			}
			progressionAct(t, state, 3, engine.Action{Type: "team", PokemonID: id, Mode: "team"})
			if progressionMember(state, id).Stats.Power != 15 {
				t.Fatal("cancelled challenge retained the normalized level")
			}
		})
	}
}

func TestNativeProgressionEquipmentRestrictionsAndOwnershipE2E(t *testing.T) {
	state := progressionNew(t)
	state.Gold = 100000
	progressionAct(t, state, 0, engine.Action{Type: "buyItem", Stock: 0, ItemID: "protein"})
	if state.Gold != 94500 || len(state.Inventory) != 1 || state.Inventory[0].ID != "protein" {
		t.Fatal("item purchase did not charge the actual shared price")
	}
	progressionAct(t, state, 0, engine.Action{Type: "egg"})
	first := state.Team[0].ID
	second := state.Team[1].ID
	progressionAct(t, state, 2, engine.Action{Type: "equip", PokemonID: first, ItemID: "protein"})
	if state.Inventory[0].EquippedBy != first || state.Team[0].Item != "protein" {
		t.Fatal("equipment did not bind to stable owned identity")
	}
	if err := engine.Apply(state, 1, engine.Action{Type: "equip", PokemonID: second, ItemID: "protein", Epoch: state.Epoch}); err == nil {
		t.Fatal("one shared item was equipped twice")
	}
	progressionAct(t, state, 3, engine.Action{Type: "unequip", PokemonID: first})
	if state.Inventory[0].EquippedBy != 0 || state.Team[0].Item != "" {
		t.Fatal("unequip discarded ownership or retained item effects")
	}
}

func TestNativeProgressionExactUpgradePricesE2E(t *testing.T) {
	state := progressionNew(t)
	if state.Team[0].UpgradeCost != 40 {
		t.Fatal("the initial partner must use the factual 40 gold price", state.Team[0].UpgradeCost)
	}
	progressionAct(t, state, 1, engine.Action{Type: "upgrade", PokemonID: state.Team[0].ID})
	if state.Gold != 10 || state.Team[0].Level != 2 || state.Team[0].UpgradeCost != 49 {
		t.Fatal("original upgrade economy changed", state.Gold, state.Team[0].Level, state.Team[0].UpgradeCost)
	}
}

func TestNativeProgressionMapSecretsAndBellE2E(t *testing.T) {
	cases := []struct {
		key   string
		route int
		kind  string
	}{
		{"cacnea", 4, "pokemon"}, {"dhelmise", 11, "pokemon"}, {"greavard", 2, "pokemon"}, {"sandygast", 16, "pokemon"}, {"ducklett", 13, "pokemon"},
		{"manaphyCave", 11, "route"}, {"victiniCave", 26, "route"}, {"marshadowCave", 15, "route"}, {"secretCastle", 29, "route"}, {"secretChess", 34, "route"},
		{"spiritomb", 28, "pokemon"}, {"beedrillite", 39, "item"}, {"gengarite", 6, "item"}, {"bronzor", 27, "bell"},
	}
	for _, scenario := range cases {
		t.Run(scenario.key, func(t *testing.T) {
			state := progressionNew(t)
			state.Stars = 4000
			state.TeamSlots = 10
			state.Progression.Secrets["klefki"] = true
			progressionAct(t, state, 0, engine.Action{Type: "route", Route: scenario.route})
			var hotspot engine.SecretHotspot
			for _, candidate := range engine.Project(state).Progression.Hotspots {
				if candidate.Key == scenario.key {
					hotspot = candidate
				}
			}
			if hotspot.Key == "" {
				t.Fatal("unlocked route did not expose the real hotspot")
			}
			click := engine.Action{Type: "secret", Name: hotspot.Key, X: hotspot.X + hotspot.Width/2, Y: hotspot.Y + hotspot.Height/2}
			if scenario.kind == "bell" {
				state.Wave = 50
				for group, count := range []int{1, 3, 1, 2, 1, 4, 2, 4, 1} {
					if group > 0 {
						if err := engine.Advance(state, 700); err != nil {
							t.Fatal(err)
						}
					}
					for index := 0; index < count; index++ {
						if index > 0 {
							if err := engine.Advance(state, 200); err != nil {
								t.Fatal(err)
							}
						}
						progressionAct(t, state, 1, click)
					}
				}
			} else {
				progressionAct(t, state, 1, click)
			}
			if scenario.kind == "pokemon" || scenario.kind == "bell" {
				if !state.Progression.Secrets[scenario.key] || len(state.Team)+len(state.Box) != 2 {
					t.Fatal("secret did not grant one shared partner")
				}
			} else if scenario.kind == "item" {
				if !state.Progression.SecretItems[scenario.key] || len(state.Inventory) != 1 {
					t.Fatal("secret did not grant the actual item")
				}
			} else if state.Route == scenario.route || !state.Progression.SecretMaps[state.Route] {
				t.Fatal("secret route did not unlock and switch the shared world")
			}
			saved, err := engine.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := engine.Unmarshal(saved); err != nil {
				t.Fatal("secret state could not resume", err)
			}
		})
	}
}

func TestNativeProgressionNormalBellIsRouteAndWaveBoundedE2E(t *testing.T) {
	state := progressionNew(t)
	state.Stars = 4000
	progressionAct(t, state, 0, engine.Action{Type: "route", Route: 27})
	progressionAct(t, state, 0, engine.Action{Type: "wave"})
	progressionAct(t, state, 2, engine.Action{Type: "bell"})
	if state.Progression.BellUses != 1 || len(state.Enemies) == 0 || !engine.HasCondition(&state.Enemies[0].EnemyEffectState, "stun") {
		t.Fatal("bell did not affect actual enemies")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 3, engine.Action{Type: "bell", Epoch: state.Epoch}); err == nil {
		t.Fatal("ordinary bell was reusable within one wave")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected bell partially changed enemies or resource limits")
	}
}

func TestNativeProgressionCorruptSavedStateIsRejectedE2E(t *testing.T) {
	state := progressionNew(t)
	original, _ := engine.Marshal(state)
	for _, mutation := range []func(map[string]any){
		func(p map[string]any) { p["eggPrice"] = -1 },
		func(p map[string]any) { p["shopStock"] = []string{"missing-item"} },
		func(p map[string]any) { p["challenge"] = map[string]any{"active": true, "levelCap": 40} },
		func(p map[string]any) { p["reset"] = map[int]bool{99999: true} },
		func(p map[string]any) { p["dna"] = map[int]string{state.Team[0].ID: "charizard"} },
		func(p map[string]any) { p["appearance"] = map[int]string{state.Team[0].ID: "treecko"} },
		func(p map[string]any) { p["hiddenShiny"] = map[int]bool{state.Team[0].ID: true} },
		func(p map[string]any) { p["customSkins"] = map[int]bool{state.Team[0].ID: true} },
		func(p map[string]any) { p["forms"] = map[int]string{state.Team[0].ID: "treecko"} },
		func(p map[string]any) { p["waveRepeat"] = true },
		func(p map[string]any) { p["waveMirror"] = true },
		func(p map[string]any) { p["profileName"] = "" },
		func(p map[string]any) { p["unownStage"] = 6 },
		func(p map[string]any) { p["unownLetter"] = "A" },
		func(p map[string]any) { p["unownForms"] = map[int]string{state.Team[0].ID: "B"} },
		func(p map[string]any) { p["settings"].(map[string]any)["audio"].(map[string]any)["effects"] = 21 },
		func(p map[string]any) { p["settings"].(map[string]any)["theme"] = "unknown" },
		func(p map[string]any) { p["settings"].(map[string]any)["font"] = "unknown" },
		func(p map[string]any) {
			p["draft"] = map[string]any{"active": true, "rounds": 10, "round": 0, "token": 1, "pool": []int{99999}, "offers": []int{99999}}
		},
	} {
		var value map[string]any
		if err := json.Unmarshal(original, &value); err != nil {
			t.Fatal(err)
		}
		mutation(value["progression"].(map[string]any))
		encoded, _ := json.Marshal(value)
		if _, err := engine.Unmarshal(encoded); err == nil {
			t.Fatal("corrupt progression data was accepted as a resumable snapshot")
		}
	}
}

func TestNativeProgressionMegaStoneUsesEligibleFormAndKeepsOwnedIdentityE2E(t *testing.T) {
	state := progressionNew(t)
	state.Gold = 1000000000
	id := state.Team[0].ID
	for index := 0; index < 3; index++ {
		progressionAct(t, state, 0, engine.Action{Type: "upgrade", PokemonID: id, Count: 10})
	}
	progressionAct(t, state, 0, engine.Action{Type: "upgrade", PokemonID: id, Count: 5})
	if state.Team[0].Species != "charizard" || state.Team[0].Level != 36 {
		t.Fatal("the actual owned evolution chain did not reach Charizard")
	}
	progressionAct(t, state, 1, engine.Action{Type: "buyItem", Stock: 3, ItemID: "charizarditeX"})
	progressionAct(t, state, 2, engine.Action{Type: "equip", PokemonID: id, ItemID: "charizarditeX"})
	if state.Team[0].ID != id || state.Team[0].Species != "megaCharizardX" || state.Team[0].Level != 36 || state.Progression.Forms[id] != "charizard" {
		t.Fatal("the mega form changed ownership or required an unsupported level cap")
	}
	progressionAct(t, state, 3, engine.Action{Type: "unequip", PokemonID: id})
	if state.Team[0].Species != "charizard" || state.Team[0].Level != 36 || state.Team[0].ID != id {
		t.Fatal("removing a mega stone did not restore the same owned base form")
	}
}

func TestNativeProgressionBossAchievementRewardEventsE2E(t *testing.T) {
	type scenario struct {
		key  string
		id   string
		pass func(*engine.State, *engine.Enemy, *engine.EncounterProgress)
		fail func(*engine.State, *engine.Enemy, *engine.EncounterProgress)
	}
	cases := []scenario{
		{key: "shaymin", id: "0", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.ItemAttacks = 1 }},
		{key: "celebi", id: "1", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.AgeMS = 7600 }},
		{key: "lunala", id: "2", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveCounters["invisibilityBreaks"] = 6
		}},
		{key: "zapdos", id: "3", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveCounters["paralyses"] = 4
		}},
		{key: "moltres", id: "4", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.RingDamage = e.MaxHP * 0.88 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.RingDamage = e.MaxHP * 0.87 }},
		{key: "regirock", id: "5", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.IndirectDamage = 1 }},
		{key: "groudon", id: "6", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.GrassAttacks = 1 }},
		{key: "hooh", id: "7", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveFlags["revivalInterrupted"] = true
		}, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.Revived = true }},
		{key: "registeel", id: "8", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.MaxDamage = 501 }},
		{key: "regice", id: "9", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.CriticalBlocks = 50 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.CriticalBlocks = 49 }},
		{key: "regigigas", id: "10", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			s.Progression.Challenge = engine.ChallengeRules{Active: true, ToughEnemies: 50}
		}, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			s.Progression.Challenge.ToughEnemies = 25
		}},
		{key: "articuno", id: "11", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { s.Weather = "hail" }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { s.Weather = "rain" }},
		{key: "kyogre", id: "12", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.FinalTerrain = 1 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.FinalTerrain = 3 }},
		{key: "thundurus", id: "13", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.Conditions = []engine.Condition{{Kind: "stun", RemainingMS: 500, Stacks: 1}}
		}, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.Conditions = nil }},
		{key: "tapuFini", id: "14", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.Dodges = 11 }},
		{key: "nihilego", id: "15", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.NonWaterAttacker = true }},
		{key: "tapuKoko", id: "16", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.Stuns = 4 }},
		{key: "tapuBulu", id: "17", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.RegenReduced = true }},
		{key: "tapuLele", id: "18", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.PowerOver100 = true }},
		{key: "keldeo", id: "19", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveFlags["justifiedTriggered"] = true
		}},
		{key: "heatran", id: "20", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.BurnedEarly = true }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.BurnedEarly = false }},
		{key: "volcanion", id: "21", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.WaterDamage = e.MaxHP * 0.74 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.WaterDamage = e.MaxHP * 0.73 }},
		{key: "pheromosa", id: "22", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.Poisoned = true }},
		{key: "cobalion", id: "23", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveFlags["justifiedTriggered"] = true
		}},
		{key: "tornadus", id: "24", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.Slows = 1 }},
		{key: "stakataka", id: "25", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.ReceivedHits = 26 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.ReceivedHits = 27 }},
		{key: "terrakion", id: "26", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveFlags["justifiedTriggered"] = true
		}},
		{key: "magearna", id: "27", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.AgeMS = 60000 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.AgeMS = 59000 }},
		{key: "diancie", id: "28", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.Criticals = 1 }},
		{key: "zamazenta", id: "29", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.ReceivedHits = 200 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.ReceivedHits = 206 }},
		{key: "zacian", id: "30", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.ReceivedHits = 1 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { e.ReceivedHits = 10 }},
		{key: "virizion", id: "32", fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) {
			e.PassiveFlags["justifiedTriggered"] = true
		}},
		{key: "landorus", id: "38", pass: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.MaxDirectAttempt = 1000 }, fail: func(s *engine.State, e *engine.Enemy, m *engine.EncounterProgress) { m.MaxDirectAttempt = 1001 }},
	}
	for _, candidate := range cases {
		for _, success := range []bool{true, false} {
			name := candidate.key + "/failure"
			if success {
				name = candidate.key + "/success"
			}
			t.Run(name, func(t *testing.T) {
				state := progressionNew(t)
				specification := engine.CatalogData().Enemies[candidate.key]
				enemy := engine.Enemy{ID: state.NextID, Key: candidate.key, Species: candidate.key, AgeMS: 1000, EnemyEffectState: engine.EnemyEffectState{HP: 0, MaxHP: specification.Health, Boss: true, PassiveID: specification.PassiveID, PassiveFlags: map[string]bool{"spawned": true}, PassiveCounters: map[string]int{}}}
				state.NextID++
				metrics := engine.EncounterProgress{Observed: true}
				if candidate.pass != nil {
					candidate.pass(state, &enemy, &metrics)
				}
				if !success {
					candidate.fail(state, &enemy, &metrics)
				}
				state.Progression.Encounters[enemy.ID] = metrics
				engine.RecordProgressionEnemyDefeat(state, enemy)
				if state.Progression.Achievements[candidate.id] != success {
					t.Fatal("achievement predicate disagreed with declared boss event boundary")
				}
				if success {
					if !state.Progression.Secrets[candidate.key] || len(state.Team)+len(state.Box) != 2 || state.Progression.ExtraGold != 2 {
						t.Fatal("successful achievement did not grant exactly one shared legendary and income bonus")
					}
					engine.RecordProgressionEnemyDefeat(state, enemy)
					if len(state.Team)+len(state.Box) != 2 || state.Progression.ExtraGold != 2 {
						t.Fatal("duplicate death event granted an achievement twice")
					}
				}
			})
		}
	}
}

func TestNativeProgressionSecretRouteVictoryRewardsE2E(t *testing.T) {
	for _, scenario := range []struct {
		route  int
		reward string
	}{{20, "manaphy"}, {30, "mew"}, {31, "victini"}, {32, "marshadow"}, {33, "farfetchdGalar"}, {35, "calyrex"}, {46, "spectrier"}} {
		t.Run(scenario.reward, func(t *testing.T) {
			state := progressionNew(t)
			state.Stars = 10000
			state.Progression.SecretMaps[scenario.route] = true
			progressionAct(t, state, 0, engine.Action{Type: "route", Route: scenario.route})
			completeResultWave(t, state)
			if state.Progression.Secrets[scenario.reward] || len(state.Team)+len(state.Box) != 1 {
				t.Fatal("secret partner appeared before confirming the final result")
			}
			epoch, gold := state.Epoch, state.Gold
			progressionAct(t, state, 2, engine.Action{Type: "continueRoute"})
			if !state.Progression.Secrets[scenario.reward] || progressionFind(state, scenario.reward) == nil || len(state.Team)+len(state.Box) != 2 {
				t.Fatal("completed secret route did not grant its actual shared partner")
			}
			draftRejectAction(t, state, engine.Action{Type: "continueRoute", Epoch: epoch})
			if state.Gold != gold || len(state.Team)+len(state.Box) != 2 {
				t.Fatal("secret route victory granted the partner twice")
			}
		})
	}
}

func TestNativeProgressionProfileAndTutorialSecretsE2E(t *testing.T) {
	state := progressionNew(t)
	if state.Progression.ProfileName != "Player" {
		t.Fatal("initial native profile name does not match the original screen")
	}
	progressionAct(t, state, 0, engine.Action{Type: "profileSecret", Value: 7})
	progressionAct(t, state, 1, engine.Action{Type: "profileSecret", Value: 19})
	progressionAct(t, state, 2, engine.Action{Type: "tutorialSecret", Name: "secrets"})
	progressionAct(t, state, 3, engine.Action{Type: "tutorialSecret", Name: "secrets"})
	if !state.Progression.Secrets["luvdisc"] || !state.Progression.Secrets["tyrunt"] || len(state.Team)+len(state.Box) != 3 {
		t.Fatal("actual profile and tutorial interactions lost or duplicated secret partners")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "profileSecret", Value: 9, Epoch: state.Epoch}); err == nil {
		t.Fatal("unrelated achievement card granted a profile secret")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid profile secret changed the shared collection")
	}
}

func TestNativeProgressionUnownDialogueAndOwnedLetterE2E(t *testing.T) {
	state := progressionNew(t)
	progressionAct(t, state, 0, engine.Action{Type: "profile", Name: "Élodie"})
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 1, engine.Action{Type: "unown", Name: "yes", TargetID: 3, Epoch: state.Epoch}); err == nil {
		t.Fatal("out-of-order Unown dialogue was accepted")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("stale Unown question changed dialogue state")
	}
	for stage, answer := range []string{"是", "elodie", "28", "?", "?????"} {
		progressionAct(t, state, stage%4, engine.Action{Type: "unown", Name: answer, TargetID: stage})
	}
	unown := progressionFind(state, "unown")
	if state.Progression.UnownStage != 5 || !state.Progression.Secrets["unown"] || unown == nil || !strings.Contains(unown.Sprite.Image, "unown-question-idle.png") {
		t.Fatal("five actual Unown answers did not grant the selected owned letter")
	}
	id := unown.ID
	progressionAct(t, state, 2, engine.Action{Type: "rename", PokemonID: id, Name: "B"})
	if !strings.Contains(progressionMember(state, id).Sprite.Image, "unown-b-idle.png") {
		t.Fatal("Unown's valid owned alias did not change the letter")
	}
	progressionAct(t, state, 3, engine.Action{Type: "rename", PokemonID: id, Name: "未知图腾"})
	if !strings.Contains(progressionMember(state, id).Sprite.Image, "unown-b-idle.png") {
		t.Fatal("ordinary alias reset the selected Unown letter")
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil || restored.Progression.UnownStage != 5 || restored.Progression.UnownLetter != "?" || !strings.Contains(progressionMember(restored, id).Sprite.Image, "unown-b-idle.png") {
		t.Fatal("Unown dialogue or per-Pokémon letter was not resumable", err)
	}
}

func progressionFind(state *engine.State, key string) *engine.Pokemon {
	for index := range state.Team {
		if state.Team[index].Species == key {
			return &state.Team[index]
		}
	}
	for index := range state.Box {
		if state.Box[index].Species == key {
			return &state.Box[index]
		}
	}
	return nil
}

func progressionAcquire(t *testing.T, state *engine.State, key string) int {
	t.Helper()
	state.Gold = 1000000000
	if !slices.Contains(engine.CatalogData().EggPool, key) {
		for _, route := range engine.CatalogData().Routes {
			if len(route.ChallengeRewards) < 2 || route.ChallengeRewards[1] != key {
				continue
			}
			previous := state.Route
			state.Stars = max(state.Stars, max(540, route.UnlockStars))
			state.Records[route.ID] = 100
			state.Progression.SecretMaps[route.ID] = true
			state.Progression.Ribbons[route.ID] = map[string]int{"levelCap": 2, "slotLimit": 1}
			progressionAct(t, state, 0, engine.Action{Type: "route", Route: route.ID})
			progressionAct(t, state, 0, engine.Action{Type: "claimReward", Slot: 1})
			progressionAct(t, state, 0, engine.Action{Type: "route", Route: previous})
			break
		}
	}
	for progressionFind(state, key) == nil && len(state.Progression.EggPool) > 0 {
		progressionAct(t, state, 0, engine.Action{Type: "egg"})
	}
	pokemon := progressionFind(state, key)
	if pokemon == nil {
		t.Fatalf("catalog egg pool never offered %s", key)
	}
	return pokemon.ID
}

func TestNativeProgressionOwnedCustomizationE2E(t *testing.T) {
	state := progressionNew(t)
	state.Gold = 10000000000
	id := state.Team[0].ID
	progressionAct(t, state, 0, engine.Action{Type: "rename", PokemonID: id, Name: "火苗"})
	progressionAct(t, state, 1, engine.Action{Type: "favorite", PokemonID: id, Enabled: true})
	progressionAct(t, state, 0, engine.Action{Type: "buyItem", Stock: 7, ItemID: "mitsuesCocktail"})
	for state.Team[0].Level < 100 {
		progressionAct(t, state, 0, engine.Action{Type: "upgrade", PokemonID: id, Count: 10})
	}
	if !engine.Project(state).Progression.Growth[id].CanReset {
		t.Fatal("growth projection hid an actually eligible reset")
	}
	progressionAct(t, state, 2, engine.Action{Type: "reset", PokemonID: id})
	if state.Team[0].ID != id || state.Team[0].Level != 1 || state.Team[0].Species != "charmander" || !state.Progression.Reset[id] || state.Team[0].Alias != "火苗" || !state.Team[0].Favorite {
		t.Fatal("reset lost owned identity, reset gating or user choices")
	}
	if engine.Project(state).Progression.Growth[id].CanReset {
		t.Fatal("growth projection kept a now-ineligible reset enabled")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "reset", PokemonID: id, Epoch: state.Epoch}); err == nil {
		t.Fatal("level-one Pokémon could reset again")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected reset changed the world")
	}
	gold := state.Gold
	progressionAct(t, state, 3, engine.Action{Type: "upgrade", PokemonID: id, Count: 10})
	if state.Team[0].Level != 11 || state.Gold != gold || state.Team[0].UpgradeCost != 0 {
		t.Fatal("a reset Pokémon did not retain the original free upgrade behavior")
	}
}

func TestNativeProgressionSpeedCycleAndRoseUnlockE2E(t *testing.T) {
	state := progressionNew(t)
	for _, expected := range []float64{1.2, 1.7, 2, 2.5, 0.8} {
		progressionAct(t, state, 0, engine.Action{Type: "speed"})
		if state.Speed != expected {
			t.Fatal("ordinary speed cycle exposed a locked speed")
		}
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 1, engine.Action{Type: "speed", Value: 5, Epoch: state.Epoch}); err == nil {
		t.Fatal("fast speed bypassed the rose requirement")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("locked speed changed shared state")
	}
	progressionBuyAdmin(t, state, "rosesItem")
	progressionAct(t, state, 1, engine.Action{Type: "speed", Value: 2.5})
	for _, expected := range []float64{5, 0.4, 0.8} {
		progressionAct(t, state, 2, engine.Action{Type: "speed"})
		if state.Speed != expected {
			t.Fatal("rose did not unlock both additional original speeds")
		}
	}
}

func TestNativeProgressionBicycleWaveManagementAndRepeatE2E(t *testing.T) {
	state := progressionNew(t)
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "selectWave", Value: 50, Epoch: state.Epoch}); err == nil {
		t.Fatal("wave management bypassed bicycle and route completion requirements")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected wave management changed the shared route")
	}
	state.Records[0] = 100
	state.Stars = 100
	progressionBuyAdmin(t, state, "bicycle")
	progressionAct(t, state, 1, engine.Action{Type: "selectWave", Value: 100})
	progressionAct(t, state, 2, engine.Action{Type: "selectWave", Value: 1, Mode: "delta"})
	if state.Wave != 1 || state.RouteWaves[0] != 1 {
		t.Fatal("completed route wave selector did not wrap from 100 to 1")
	}
	id := state.Team[0].ID
	progressionLevel(t, state, id, 100)
	progressionAct(t, state, 1, engine.Action{Type: "deploy", PokemonID: id, TileID: 2})
	progressionAct(t, state, 2, engine.Action{Type: "repeatWave", Enabled: true})
	progressionAct(t, state, 3, engine.Action{Type: "mirrorWave", Enabled: true})
	progressionAct(t, state, 0, engine.Action{Type: "wave"})
	progressionAct(t, state, 1, engine.Action{Type: "mirrorWave", Enabled: false})
	if !engine.ProgressionMirrored(state) || !engine.ProgressionRepeatsWave(state) {
		t.Fatal("changing next-wave mirror moved the live wave or lost repeat mode")
	}
	saved, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(saved)
	if err != nil || !engine.ProgressionMirrored(restored) {
		t.Fatal("saving a live mirrored wave lost its frozen path", err)
	}
	for elapsed := 0; state.WaveActive && elapsed < 120000; elapsed += 1000 {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if state.WaveActive || state.Health <= 0 || state.Wave != 1 || state.Phase != "prepare" || engine.ProgressionMirrored(state) {
		t.Fatal("actual repeated wave failed to keep its wave number or apply next-wave mirror")
	}
	progressionAct(t, state, 2, engine.Action{Type: "repeatWave", Enabled: false})
	progressionAct(t, state, 3, engine.Action{Type: "wave"})
	for elapsed := 0; state.WaveActive && elapsed < 120000; elapsed += 1000 {
		if err := engine.Advance(state, 1000); err != nil {
			t.Fatal(err)
		}
	}
	if state.Wave != 2 || state.WaveActive || state.Health <= 0 {
		t.Fatal("disabling repeat did not resume normal wave advancement")
	}
}

func TestNativeProgressionRepeatedHundredthWaveHasNoFinalTransitionE2E(t *testing.T) {
	state := progressionNew(t)
	state.Records[0] = 100
	state.Stars = 100
	progressionBuyAdmin(t, state, "bicycle")
	progressionAct(t, state, 0, engine.Action{Type: "selectWave", Value: 100})
	progressionAct(t, state, 1, engine.Action{Type: "repeatWave", Enabled: true})
	progressionAct(t, state, 2, engine.Action{Type: "wave"})
	state.Enemies = []engine.Enemy{}
	if err := engine.Advance(state, 100); err != nil {
		t.Fatal(err)
	}
	if state.Phase != "prepare" || state.Wave != 100 || state.WaveActive {
		t.Fatal("repeated wave 100 incorrectly entered route completion")
	}
}

func TestNativeProgressionComputerUsesOwnedDNATerrainInChallengeE2E(t *testing.T) {
	state := progressionChallengeFixture(t)
	ditto := progressionAcquire(t, state, "ditto")
	lapras := progressionAcquire(t, state, "lapras")
	progressionAct(t, state, 0, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50}`)})
	progressionAct(t, state, 1, engine.Action{Type: "team", PokemonID: ditto, Mode: "team"})
	progressionAct(t, state, 1, engine.Action{Type: "team", PokemonID: lapras, Mode: "team"})
	progressionAct(t, state, 2, engine.Action{Type: "dna", PokemonID: ditto, TargetID: lapras})
	progressionAct(t, state, 2, engine.Action{Type: "team", PokemonID: lapras, Mode: "box"})
	progressionAct(t, state, 0, engine.Action{Type: "speed", Value: 2.5})
	action := engine.Propose(state)
	if action == nil || action.Type != "deploy" || action.PokemonID != ditto {
		t.Fatal("computer ignored the challenge's actual Ditto DNA and could not deploy", action)
	}
	progressionAct(t, state, 3, *action)
	tile := -1
	for _, placement := range engine.Project(state).Tiles {
		if placement.ID == progressionMember(state, ditto).TileID {
			tile = placement.Terrain
		}
	}
	if tile != 3 || len(state.Towers) != 1 {
		t.Fatal("computer did not use the copied water terrain through the normal action interface")
	}
}

func TestNativeProgressionComputerContinuesMirroredCompletedRouteE2E(t *testing.T) {
	state := progressionNew(t)
	state.Records[0] = 100
	state.Stars = 100
	progressionBuyAdmin(t, state, "bicycle")
	progressionLevel(t, state, state.Team[0].ID, 100)
	progressionAct(t, state, 0, engine.Action{Type: "speed", Value: 2.5})
	progressionAct(t, state, 1, engine.Action{Type: "mirrorWave", Enabled: true})
	state.Gold = 0
	for index := 0; !state.WaveActive && index < 10; index++ {
		action := engine.Propose(state)
		if action == nil {
			t.Fatal("computer stopped during a playable completed-route mirror mode")
		}
		progressionAct(t, state, index%state.Players, *action)
	}
	if !state.WaveActive || len(state.Towers) != 1 || !engine.ProgressionMirrored(state) {
		t.Fatal("computer could not deploy and start the actual mirrored replay")
	}
}

func TestNativeProgressionDNAUsesOwnedTargetAndOwnLevelE2E(t *testing.T) {
	state := progressionNew(t)
	dittoID := progressionAcquire(t, state, "ditto")
	if pokemon := progressionFind(state, "ditto"); pokemon != nil {
		inTeam := false
		for _, member := range state.Team {
			inTeam = inTeam || member.ID == dittoID
		}
		if !inTeam {
			last := state.Team[len(state.Team)-1].ID
			progressionAct(t, state, 0, engine.Action{Type: "team", PokemonID: last, Mode: "box"})
			progressionAct(t, state, 0, engine.Action{Type: "team", PokemonID: dittoID, Mode: "team"})
		}
	}
	starterID := state.Team[0].ID
	progressionAct(t, state, 1, engine.Action{Type: "upgrade", PokemonID: starterID, Count: 10})
	progressionAct(t, state, 1, engine.Action{Type: "upgrade", PokemonID: starterID, Count: 10})
	progressionAct(t, state, 2, engine.Action{Type: "dna", PokemonID: dittoID, TargetID: starterID})
	ditto := progressionFind(state, "ditto")
	if ditto == nil || ditto.ID != dittoID || state.Progression.DNA[dittoID] != "charmeleon" {
		t.Fatal("DNA did not preserve the owned Ditto and point to the selected actual form")
	}
	specification := engine.CatalogData().Species["charmeleon"]
	if ditto.Stats.Power != math.Floor(specification.PowerBase+specification.PowerPerLevel*float64(ditto.Level)) || ditto.Sprite.Image != specification.Sprite.Image {
		t.Fatal("copied stats used the target's level or failed to copy the actual sprite")
	}
	saved, _ := engine.Marshal(state)
	restored, err := engine.Unmarshal(saved)
	if err != nil || restored.Progression.DNA[dittoID] != "charmeleon" {
		t.Fatal("DNA state was not resumable", err)
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "dna", PokemonID: dittoID, TargetID: state.NextID + 99, Epoch: state.Epoch}); err == nil {
		t.Fatal("DNA accepted an unowned target")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid DNA changed an owned Pokémon")
	}
}

func TestNativeProgressionSchemeShinyKeepsCopiedDNAAppearanceE2E(t *testing.T) {
	state := progressionNew(t)
	id := progressionAcquire(t, state, "ditto")
	if !slices.ContainsFunc(state.Team, func(pokemon engine.Pokemon) bool { return pokemon.ID == id }) {
		progressionAct(t, state, 0, engine.Action{Type: "team", PokemonID: state.Team[len(state.Team)-1].ID, Mode: "box"})
		progressionAct(t, state, 0, engine.Action{Type: "team", PokemonID: id, Mode: "team"})
	}
	progressionAct(t, state, 1, engine.Action{Type: "dna", PokemonID: id, TargetID: state.Team[0].ID})
	for index := range state.Team {
		state.Team[index].Shiny = state.Team[index].ID != id
	}
	for index := range state.Box {
		state.Box[index].Shiny = true
	}
	enemy := engine.Enemy{ID: state.NextID, Key: "rattata", Species: "rattata"}
	projectile := engine.Projectile{PokemonID: state.Team[0].ID}
	engine.RecordProgressionHit(state, &enemy, &projectile, engine.AttackProfile{ShinyChance: 1}, 1, 1, false, nil)
	ditto := progressionMember(state, id)
	if !ditto.Shiny || strings.Contains(ditto.Sprite.Image, "/shiny/") {
		t.Fatal("scheme applied the Ditto shiny to another Pokémon's copied sprite")
	}
}

func progressionMember(state *engine.State, id int) *engine.Pokemon {
	for index := range state.Team {
		if state.Team[index].ID == id {
			return &state.Team[index]
		}
	}
	for index := range state.Box {
		if state.Box[index].ID == id {
			return &state.Box[index]
		}
	}
	return nil
}

func progressionLevel(t *testing.T, state *engine.State, id, level int) {
	t.Helper()
	state.Gold = 10000000000
	for progressionMember(state, id).Level < level {
		count := 1
		remaining := level - progressionMember(state, id).Level
		if remaining >= 10 {
			count = 10
		} else if remaining >= 5 {
			count = 5
		}
		progressionAct(t, state, 0, engine.Action{Type: "upgrade", PokemonID: id, Count: count})
	}
}

func progressionBuyAdmin(t *testing.T, state *engine.State, key string) {
	t.Helper()
	state.Gold = 10000000000
	for !state.Progression.Unlocks[key] {
		owned := false
		for _, item := range state.Inventory {
			owned = owned || item.ID == key
		}
		if owned {
			return
		}
		stock := -1
		for index, candidate := range state.Progression.ShopStock {
			if candidate == key {
				stock = index
				break
			}
		}
		if stock < 0 {
			for _, index := range []int{6, 7} {
				if state.Progression.ShopStock[index] != "" {
					stock = index
					break
				}
			}
		}
		if stock < 0 {
			t.Fatalf("admin pool never offered %s", key)
		}
		progressionAct(t, state, 1, engine.Action{Type: "buyItem", Stock: stock, ItemID: state.Progression.ShopStock[stock]})
	}
}

func TestNativeProgressionLegalCombatFormsE2E(t *testing.T) {
	for _, scenario := range []struct {
		base   string
		level  int
		target string
	}{
		{base: "rockruff", level: 25, target: "lycanrocNight"},
		{base: "honedge", level: 60, target: "aegislashSword"},
		{base: "charcadet", level: 40, target: "ceruledge"},
		{base: "frillishM", level: 1, target: "frillishF"},
		{base: "applin", level: 60, target: "appletun"},
		{base: "ralts", level: 30, target: "gallade"},
		{base: "snorunt", level: 42, target: "froslass"},
		{base: "scyther", level: 50, target: "scizor"},
	} {
		t.Run(scenario.base, func(t *testing.T) {
			state := progressionNew(t)
			id := progressionAcquire(t, state, scenario.base)
			progressionLevel(t, state, id, scenario.level)
			if !slices.Contains(engine.Project(state).Progression.Growth[id].Forms, scenario.target) {
				t.Fatal("growth projection hid an eligible actual combat form")
			}
			progressionAct(t, state, 3, engine.Action{Type: "form", PokemonID: id, Mode: scenario.target})
			pokemon := progressionMember(state, id)
			if pokemon.ID != id || pokemon.Level != scenario.level || pokemon.Species != scenario.target || pokemon.Sprite.Image != engine.CatalogData().Species[scenario.target].Sprite.Image {
				t.Fatal("combat form selection changed ownership or did not change actual rules")
			}
			before, _ := engine.Marshal(state)
			if err := engine.Apply(state, 0, engine.Action{Type: "form", PokemonID: id, Mode: "charizard", Epoch: state.Epoch}); err == nil {
				t.Fatal("combat form accepted another collection number")
			}
			after, _ := engine.Marshal(state)
			if !bytes.Equal(before, after) {
				t.Fatal("invalid combat form changed the world")
			}
		})
	}
}

func TestNativeProgressionFormationRestoresSelectedCombatFormE2E(t *testing.T) {
	state := progressionNew(t)
	id := progressionAcquire(t, state, "scyther")
	progressionLevel(t, state, id, 50)
	progressionAct(t, state, 0, engine.Action{Type: "form", PokemonID: id, Mode: "scizor"})
	progressionAct(t, state, 1, engine.Action{Type: "saveTeam", Slot: 2})
	progressionAct(t, state, 2, engine.Action{Type: "form", PokemonID: id, Mode: "scyther"})
	progressionAct(t, state, 3, engine.Action{Type: "loadTeam", Slot: 2})
	if progressionMember(state, id).Species != "scizor" {
		t.Fatal("formation did not restore the saved actual combat form")
	}
}

func TestNativeProgressionAppearanceCannotChangeRulesE2E(t *testing.T) {
	state := progressionNew(t)
	id := state.Team[0].ID
	progressionBuyAdmin(t, state, "sokudosPortfolio")
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "appearance", PokemonID: id, Mode: "charizard", Epoch: state.Epoch}); err == nil {
		t.Fatal("appearance bypassed the level or reset requirement")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected appearance changed shared state")
	}
	progressionLevel(t, state, id, 100)
	stats := state.Team[0].Stats
	if !slices.Contains(engine.Project(state).Progression.Growth[id].Appearances, "charmander") {
		t.Fatal("growth projection hid an eligible cosmetic appearance")
	}
	progressionAct(t, state, 2, engine.Action{Type: "appearance", PokemonID: id, Mode: "charmander"})
	if state.Team[0].Species != "charizard" || state.Team[0].Stats != stats || state.Team[0].Sprite.Image != engine.CatalogData().Species["charmander"].Sprite.Image {
		t.Fatal("cosmetic appearance changed combat rules or failed to change the sprite")
	}
	progressionAct(t, state, 3, engine.Action{Type: "buyItem", Stock: 3, ItemID: "charizarditeX"})
	progressionAct(t, state, 2, engine.Action{Type: "equip", PokemonID: id, ItemID: "charizarditeX"})
	before, _ = engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "appearance", PokemonID: id, Mode: "charmeleon", Epoch: state.Epoch}); err == nil {
		t.Fatal("mega form accepted a cosmetic appearance change")
	}
	after, _ = engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("rejected mega appearance changed the world")
	}
	progressionAct(t, state, 0, engine.Action{Type: "unequip", PokemonID: id})
	if state.Team[0].Sprite.Image != engine.CatalogData().Species["charmander"].Sprite.Image {
		t.Fatal("refreshing rules lost the selected cosmetic appearance")
	}
	saved, _ := engine.Marshal(state)
	restored, err := engine.Unmarshal(saved)
	if err != nil || restored.Team[0].Sprite != state.Team[0].Sprite {
		t.Fatal("cosmetic appearance did not survive shared save restoration", err)
	}
}

func TestNativeProgressionShinyAndCustomSkinBoundariesE2E(t *testing.T) {
	state := progressionNew(t)
	id := state.Team[0].ID
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "shiny", PokemonID: id, Enabled: true, Epoch: state.Epoch}); err == nil {
		t.Fatal("an unearned shiny appearance was accepted")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("failed shiny request changed the world")
	}
	state.Team[0].Shiny = true
	progressionAct(t, state, 1, engine.Action{Type: "shiny", PokemonID: id, Enabled: true})
	if !strings.Contains(state.Team[0].Sprite.Image, "/shiny/") || !state.Team[0].Shiny {
		t.Fatal("already earned shiny did not display")
	}
	progressionAct(t, state, 2, engine.Action{Type: "shiny", PokemonID: id, Enabled: false})
	if !strings.Contains(state.Team[0].Sprite.Image, "/normal/") || !state.Team[0].Shiny {
		t.Fatal("hiding shiny erased the earned shiny flag")
	}
	trapinch := progressionAcquire(t, state, "trapinch")
	progressionBuyAdmin(t, state, "rosesItem")
	progressionAct(t, state, 3, engine.Action{Type: "customSkin", PokemonID: trapinch, Enabled: true})
	if !strings.Contains(progressionMember(state, trapinch).Sprite.Image, "/skin/normal/") {
		t.Fatal("eligible custom skin did not replace the sprite path")
	}
	progressionAct(t, state, 0, engine.Action{Type: "customSkin", PokemonID: trapinch, Enabled: false})
	if strings.Contains(progressionMember(state, trapinch).Sprite.Image, "/skin/") {
		t.Fatal("custom skin could not be disabled")
	}
	if err := engine.Apply(state, 0, engine.Action{Type: "customSkin", PokemonID: id, Enabled: true, Epoch: state.Epoch}); err == nil {
		t.Fatal("custom skin accepted an unsupported collection number")
	}
}

func TestNativeProgressionSpecialEvolutionItemsAndShedinjaE2E(t *testing.T) {
	state := progressionNew(t)
	nincada := progressionAcquire(t, state, "nincada")
	progressionLevel(t, state, nincada, 20)
	if progressionMember(state, nincada).Species != "ninjask" || progressionFind(state, "shedinja") == nil || !state.Progression.Secrets["shedinja"] {
		t.Fatal("actual Nincada evolution did not grant Shedinja once")
	}
	inkay := progressionAcquire(t, state, "inkay")
	progressionLevel(t, state, inkay, 30)
	if progressionMember(state, inkay).Species != "inkay" {
		t.Fatal("Inkay evolved without the inverter requirement")
	}
	for !slices.ContainsFunc(state.Inventory, func(item engine.OwnedItem) bool { return item.ID == "inverter" }) {
		stock := -1
		for index, key := range state.Progression.ShopStock {
			if key == "inverter" {
				stock = index
				break
			}
		}
		if stock < 0 && state.Progression.ShopStock[0] != "" {
			stock = 0
		}
		if stock < 0 {
			t.Fatal("normal shop pool never offered inverter")
		}
		progressionAct(t, state, 0, engine.Action{Type: "buyItem", Stock: stock, ItemID: state.Progression.ShopStock[stock]})
	}
	progressionAct(t, state, 1, engine.Action{Type: "equip", PokemonID: inkay, ItemID: "inverter"})
	progressionAct(t, state, 2, engine.Action{Type: "upgrade", PokemonID: inkay})
	if progressionMember(state, inkay).Species != "malamar" {
		t.Fatal("equipped Inkay did not evolve on the next level")
	}
}

func TestNativeProgressionShieldAndSwordAchievementHitBoundariesE2E(t *testing.T) {
	for _, scenario := range []struct {
		key      string
		id       string
		hits     int
		unlocked bool
	}{
		{"zamazenta", "29", 0, true},
		{"zamazenta", "29", 204, true},
		{"zamazenta", "29", 205, false},
		{"zamazenta", "29", 206, false},
		{"zacian", "30", 0, true},
		{"zacian", "30", 4, true},
		{"zacian", "30", 5, false},
		{"zacian", "30", 6, false},
		{"zacian", "30", 50, false},
	} {
		t.Run(fmt.Sprintf("%s-%d", scenario.key, scenario.hits), func(t *testing.T) {
			state := progressionNew(t)
			spec := engine.CatalogData().Enemies[scenario.key]
			enemy := engine.Enemy{ID: state.NextID, Key: scenario.key, Species: scenario.key, EnemyEffectState: engine.EnemyEffectState{HP: 0, MaxHP: spec.Health, Boss: true, PassiveID: spec.PassiveID, ReceivedHits: scenario.hits, PassiveFlags: map[string]bool{"spawned": true}, PassiveCounters: map[string]int{}}}
			state.NextID++
			state.Progression.Encounters[enemy.ID] = engine.EncounterProgress{Observed: true}
			engine.RecordProgressionEnemyDefeat(state, enemy)
			if state.Progression.Achievements[scenario.id] != scenario.unlocked {
				t.Fatal("integer passive charge boundary produced the wrong achievement", scenario.hits)
			}
		})
	}
}
