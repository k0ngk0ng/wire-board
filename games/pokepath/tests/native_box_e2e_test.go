package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestNativePlatformBoxSlotReplacesOnlyTheQuotedPartner(t *testing.T) {
	module, ctx, match := nativeStart(t, 4)
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &envelope); err != nil {
		t.Fatal(err)
	}
	world, err := engine.Unmarshal(envelope["snapshot"])
	if err != nil {
		t.Fatal(err)
	}
	world.Gold = 1000000
	envelope["snapshot"], err = engine.Marshal(world)
	if err != nil {
		t.Fatal(err)
	}
	match.Data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"egg","quote":%d`, len(world.Progression.EggPool)-index))
	}
	fromView := nativeObserve(t, module, match, 0)
	from, err := engine.Unmarshal(fromView.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	first, second, third := from.Team[0], from.Team[1], from.Team[2]
	places := engine.CatalogData().Routes[fmt.Sprint(from.Route)].Placements
	index := slices.IndexFunc(places, func(tile engine.Placement) bool {
		return slices.Contains(engine.CatalogData().Species[first.Species].Terrains, tile.Terrain)
	})
	if index < 0 {
		t.Fatal("no legal placement for the starter")
	}
	tileID := places[index].ID
	match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"deploy","pokemonID":%d,"tileID":%d`, first.ID, tileID))
	for _, pokemon := range engine.ProjectPlayer(from, 0).Team[1:] {
		match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"team","pokemonID":%d,"mode":"box"`, pokemon.ID))
	}
	ctx.Actor = 0
	drop := func(id, target, slot, epoch int, input string) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"type":"teamSlot","epoch":%d,"pokemonID":%d,"targetID":%d,"slot":%d,"clientInputID":%q}`, epoch, id, target, slot, input))
	}
	input := drop(second.ID, first.ID, 0, fromView.Epoch, "visible-slot")
	moved, err := module.Apply(ctx, match, input)
	if err != nil {
		t.Fatal(err)
	}
	view := nativeObserve(t, module, moved, 0)
	after, err := engine.Unmarshal(view.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	personal := engine.ProjectPlayer(after, 0)
	if len(personal.Team) != 1 || personal.Team[0].ID != second.ID || personal.Team[0].TileID != -1 || len(after.Towers) != 0 || !slices.ContainsFunc(after.Box, func(pokemon engine.Pokemon) bool { return pokemon.ID == first.ID && pokemon.TileID == -1 }) {
		t.Fatalf("replacement did not retire the previous occupant and preserve both partners: team=%+v box=%+v towers=%+v", after.Team, after.Box, after.Towers)
	}
	if replay, err := module.Apply(ctx, moved, input); err != nil || !bytes.Equal(replay.Data, moved.Data) {
		t.Fatal("an exact retransmission was not idempotent", err)
	}
	ctx.Actor = 0
	for _, rejected := range []json.RawMessage{
		drop(third.ID, first.ID, 0, view.Epoch, "stale-occupant"),
		drop(third.ID, 0, 0, view.Epoch, "false-empty"),
		drop(third.ID, 0, 99, view.Epoch, "locked-slot"),
		drop(second.ID, 0, 1, view.Epoch, "already-in-team"),
		drop(third.ID, 0, 1, view.Epoch+1, "future-epoch"),
	} {
		if _, err := module.Apply(ctx, moved, rejected); err == nil {
			t.Fatalf("accepted an invalid drop: %s", rejected)
		}
	}
	filled, err := module.Apply(ctx, moved, drop(third.ID, 0, 1, view.Epoch, "empty-slot"))
	if err != nil {
		t.Fatal(err)
	}
	for _, seat := range []int{0, 1, 2, 3} {
		state, err := engine.Unmarshal(nativeObserve(t, module, filled, seat).Snapshot)
		personal := engine.ProjectPlayer(state, 0)
		if err != nil || len(personal.Team) != 2 || personal.Team[0].ID != second.ID || personal.Team[1].ID != third.ID || state.Gold != from.Gold {
			t.Fatalf("seat %d does not see the owner slot order and unchanged gold: %v", seat, err)
		}
	}
}
