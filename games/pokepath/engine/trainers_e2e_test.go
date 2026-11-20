package engine

import (
	"bytes"
	"reflect"
	"testing"
)

func trainerView(t *testing.T, state *State, actor int) View {
	t.Helper()
	return ProjectPlayer(state, actor)
}

func TestTrainersConcurrentPositionAndWaveIncomeE2E(t *testing.T) {
	state := coreNew(t, 2)
	first, second := trainerView(t, state, 0), trainerView(t, state, 1)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: first.Team[0].ID, TileID: 1})
	before, _ := Marshal(state)
	if err := Apply(state, 1, Action{Epoch: state.Epoch, Type: "deploy", PokemonID: second.Team[0].ID, TileID: 1}); err == nil {
		t.Fatal("two trainers occupied the same placement")
	}
	after, _ := Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("failed deployment changed trainer or battle state")
	}
	coreApply(t, state, 1, Action{Type: "deploy", PokemonID: second.Team[0].ID, TileID: 2})
	coreApply(t, state, 0, Action{Type: "wave"})
	for elapsed := 0; state.WaveActive && elapsed < 180000; elapsed += 100 {
		if err := Advance(state, 100); err != nil {
			t.Fatal(err)
		}
	}
	first, second = trainerView(t, state, 0), trainerView(t, state, 1)
	if state.Wave != 2 || state.WaveActive || first.Gold <= 50 || first.Gold != second.Gold || len(state.Towers) != 2 {
		t.Fatalf("shared wave did not award independent wallets: wave=%d gold=%d/%d towers=%d", state.Wave, first.Gold, second.Gold, len(state.Towers))
	}
	raw, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(raw)
	restoredRaw, marshalErr := Marshal(restored)
	if err != nil || marshalErr != nil || !bytes.Equal(raw, restoredRaw) {
		t.Fatalf("personal state failed round trip: %v", err)
	}
}

func TestTrainersInventoryFormationAndSettingsE2E(t *testing.T) {
	state := coreNew(t, 2)
	for actor := 0; actor < 2; actor++ {
		state.Trainers[actor].Gold = 1000000
	}
	state.Gold = 1000000
	for actor := 0; actor < 2; actor++ {
		view := trainerView(t, state, actor)
		stock := -1
		for _, offer := range view.Progression.Shop {
			if offer.Equipable {
				stock = offer.Stock
				break
			}
		}
		if stock < 0 {
			t.Fatal("no initial equipable offer")
		}
		coreApply(t, state, actor, Action{Type: "buyItem", Stock: stock})
		coreApply(t, state, actor, Action{Type: "saveTeam", Slot: 0})
	}
	before := trainerView(t, state, 1)
	coreApply(t, state, 0, Action{Type: "team", PokemonID: trainerView(t, state, 0).Team[2].ID, Mode: "box"})
	coreApply(t, state, 0, Action{Type: "settings", Name: "language", Value: 0})
	if len(trainerView(t, state, 0).Team) != 2 || len(trainerView(t, state, 0).Box) != 1 || !reflect.DeepEqual(before, trainerView(t, state, 1)) {
		after := trainerView(t, state, 1)
		if len(after.Team) != 3 || len(after.Box) != 0 || after.Gold != before.Gold || !reflect.DeepEqual(after.Items, before.Items) || !reflect.DeepEqual(after.Progression, before.Progression) {
			t.Fatal("personal inventory, formation or settings affected the partner")
		}
	}
	coreApply(t, state, 0, Action{Type: "loadTeam", Slot: 0})
	if len(trainerView(t, state, 0).Team) != 3 || len(trainerView(t, state, 1).Team) != 3 {
		t.Fatal("loading a personal formation changed another roster")
	}
}

func TestTrainersSharePlatformAbilitiesWithoutSharingControlE2E(t *testing.T) {
	state := coreNew(t, 2)
	host, err := newPokemon(state, "turtwig", 30)
	if err != nil {
		t.Fatal(err)
	}
	passenger, err := newPokemon(state, "rattata", 30)
	if err != nil {
		t.Fatal(err)
	}
	passenger.Owner = 1
	state.Team = append(state.Team, host, passenger)
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: host.ID, TileID: 1})
	coreApply(t, state, 1, Action{Type: "deploy", PokemonID: passenger.ID, TileID: 1})
	if len(state.Towers) != 2 || state.World.PassengerHosts[passenger.ID] != host.ID {
		t.Fatal("a partner cannot use another trainer's legal support platform")
	}
	coreApply(t, state, 0, Action{Type: "retire", PokemonID: host.ID})
	if len(state.Towers) != 0 || ownedPokemonFor(state, passenger.ID).TileID != -1 {
		t.Fatal("recalling a shared platform left its passenger stranded")
	}
}

func TestTrainersCampaignRewardsAndCorruptIdentityMapsE2E(t *testing.T) {
	state := coreNew(t, 2)
	if !GrantProgressionSecret(state, "luvdisc") {
		t.Fatal("campaign reward was not granted")
	}
	for actor := 0; actor < 2; actor++ {
		view := ProjectPlayer(state, actor)
		found := false
		for _, pokemon := range append(view.Team, view.Box...) {
			found = found || pokemon.Species == "luvdisc"
		}
		if !found || !view.Progression.Secrets["luvdisc"] {
			t.Fatalf("trainer %d did not receive the campaign reward", actor)
		}
	}
	state.Progression.Reset[99999] = true
	if _, err := Marshal(state); err == nil {
		t.Fatal("an unowned identity survived personal save validation")
	}
}
