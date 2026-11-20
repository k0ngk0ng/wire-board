package engine

import (
	"bytes"
	"reflect"
	"testing"
)

func TestInventoryStableIDSwapAcrossSharedSeatsAndRestore(t *testing.T) {
	state := coreNew(t, 4)
	for index := range state.Progression.ShopStock {
		state.Progression.ShopStock[index] = ""
	}
	state.Inventory = []OwnedItem{{ID: "protein", EquippedBy: state.Team[0].ID}, {ID: "quickClaw"}, {ID: "lifeOrb"}}
	state.Team[0].Item = "protein"
	before := Project(state)
	coreApply(t, state, 0, Action{Type: "swapItems", ItemID: "protein", Name: "lifeOrb"})
	want := []OwnedItem{{ID: "lifeOrb"}, {ID: "quickClaw"}, {ID: "protein", EquippedBy: state.Team[0].ID}}
	if !reflect.DeepEqual(state.Inventory, want) || state.Gold != before.Gold || state.Health != before.Health || state.Team[0].Item != "protein" {
		t.Fatal("inventory order must retain ownership and shared resources", state.Inventory)
	}
	raw, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	coreApply(t, restored, 0, Action{Type: "swapItems", ItemID: "protein", Name: "lifeOrb"})
	if !reflect.DeepEqual(restored.Inventory, []OwnedItem{{ID: "protein", EquippedBy: state.Team[0].ID}, {ID: "quickClaw"}, {ID: "lifeOrb"}}) {
		t.Fatal("restored shared inventory lost its stable order")
	}
}

func TestInventoryInvalidSwapIsTransactional(t *testing.T) {
	state := coreNew(t, 4)
	for index := range state.Progression.ShopStock {
		state.Progression.ShopStock[index] = ""
	}
	state.Inventory = []OwnedItem{{ID: "protein"}, {ID: "quickClaw"}}
	for _, item := range []struct {
		actor  int
		action Action
	}{{0, Action{ItemID: "protein", Name: "missing"}}, {0, Action{ItemID: "missing", Name: "quickClaw"}}, {0, Action{ItemID: "protein", Name: "protein"}}, {0, Action{}}, {-1, Action{ItemID: "protein", Name: "quickClaw"}}, {4, Action{ItemID: "protein", Name: "quickClaw"}}} {
		before, _ := Marshal(state)
		item.action.Type = "swapItems"
		item.action.Epoch = state.Epoch
		if Apply(state, item.actor, item.action) == nil {
			t.Fatal("invalid inventory swap was accepted", item)
		}
		after, _ := Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("failed inventory swap changed shared state")
		}
	}
	before, _ := Marshal(state)
	if Apply(state, 0, Action{Type: "swapItems", Epoch: state.Epoch + 1, ItemID: "protein", Name: "quickClaw"}) == nil {
		t.Fatal("old control epoch accepted")
	}
	after, _ := Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("old epoch changed inventory order")
	}
}
