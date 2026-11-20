package engine

import "testing"

func TestAICleansingBossKeepsItsCounterAttackingE2E(t *testing.T) {
	state, err := New(Options{Starter: "charmander", Speed: 2.5}, 1, 123456, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	state.Team = nil
	for index := range state.Progression.ShopStock {
		state.Progression.ShopStock[index] = ""
	}
	state.Gold, state.Stars, state.TeamSlots, state.Wave, state.RouteWaves[0], state.Records[0] = 0, 99, 7, 100, 100, 99
	for _, member := range []struct {
		key   string
		level int
		item  string
	}{{"ferrothorn", 59, ""}, {"machamp", 65, "iron"}, {"sceptile", 62, "litCoal"}, {"ditto", 59, ""}, {"gengar", 59, "protein"}, {"noivern", 62, "carbos"}, {"jumpluff", 64, ""}} {
		pokemon, err := newPokemon(state, member.key, member.level)
		if err != nil {
			t.Fatal(err)
		}
		pokemon.Item = member.item
		if member.item != "" {
			state.Inventory = append(state.Inventory, OwnedItem{ID: member.item, EquippedBy: pokemon.ID})
		}
		if member.key == "ditto" {
			state.Progression.DNA[pokemon.ID] = "sceptile"
			if err := refreshPokemon(state, &pokemon); err != nil {
				t.Fatal(err)
			}
		}
		state.Team = append(state.Team, pokemon)
	}
	trace := []aiRecordedAction{}
	for elapsed := int64(0); elapsed < 120000 && state.Phase != "defeat" && state.Phase != "route_complete"; elapsed += 100 {
		if action := Propose(state); action != nil {
			gold := state.Gold
			if err := Apply(state, 0, *action); err != nil {
				t.Fatalf("%+v: %v", action, err)
			}
			trace = append(trace, aiRecordedAction{At: elapsed, Actor: 0, Action: *action, GoldBefore: gold, GoldAfter: state.Gold})
		}
		if err := Advance(state, 100); err != nil {
			t.Fatal(err)
		}
	}
	aiSaveArtifact(t, "cleansing-boss.json", map[string]any{"command": "go test ./games/pokepath/engine -run '^TestAICleansingBoss' -count=1", "controlledFixture": true, "actions": trace, "state": state, "passed": state.Records[0] == 100})
	if state.Records[0] != 100 || state.Health <= 0 {
		t.Fatalf("a legally owned counter remained outside the boss's path: wave=%d health=%d phase=%s", state.Wave, state.Health, state.Phase)
	}
}
