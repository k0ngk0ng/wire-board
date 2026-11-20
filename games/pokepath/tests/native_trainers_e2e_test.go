package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	pokepath "github.com/k0ngk0ng/wire-board/games/pokepath/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func trainerData(t *testing.T, module gameapi.Module, match gameapi.Match, actor int) map[string]any {
	t.Helper()
	value, err := module.View(match, actor)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func trainerMember(data map[string]any, index int) map[string]any {
	return data["team"].([]any)[index].(map[string]any)
}

func TestNativeTrainersStartWithSeparatePartnersAndWallets(t *testing.T) {
	for players := 1; players <= 4; players++ {
		module, _, match := nativeStart(t, players)
		ids := map[float64]bool{}
		for actor := 0; actor < players; actor++ {
			data := trainerData(t, module, match, actor)
			if data["gold"] != float64(50) || len(data["team"].([]any)) != 3 {
				t.Fatalf("players=%d actor=%d needs own 50 gold and three partners: gold=%v team=%v", players, actor, data["gold"], data["team"])
			}
			for index, key := range []string{"charmander", "treecko", "froaki"} {
				member := trainerMember(data, index)
				id := member["id"].(float64)
				if member["species"] != key || ids[id] {
					t.Fatalf("wrong or shared starter: %v", member)
				}
				ids[id] = true
			}
		}
	}
}

func TestNativeTrainersPurchasesSettingsAndOwnershipAreIndependent(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	first := trainerData(t, module, match, 0)
	second := trainerData(t, module, match, 1)
	firstID := int(trainerMember(first, 0)["id"].(float64))
	secondID := int(trainerMember(second, 0)["id"].(float64))
	match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"upgrade","pokemonID":%d`, firstID))
	if trainerData(t, module, match, 0)["gold"] != float64(10) || trainerData(t, module, match, 1)["gold"] != float64(50) {
		t.Fatal("an upgrade spent another trainer's gold")
	}
	ctx.Actor = 1
	before := append([]byte{}, match.Data...)
	for _, action := range []string{
		fmt.Sprintf(`"type":"upgrade","pokemonID":%d`, firstID),
		fmt.Sprintf(`"type":"retire","pokemonID":%d`, firstID),
		fmt.Sprintf(`"type":"deploy","pokemonID":%d,"tileID":1`, firstID),
		fmt.Sprintf(`"type":"target","pokemonID":%d,"mode":"last"`, firstID),
		fmt.Sprintf(`"type":"rename","pokemonID":%d,"name":"队友不能改名"`, firstID),
		fmt.Sprintf(`"type":"team","pokemonID":%d,"mode":"box"`, firstID),
	} {
		if _, err := module.Apply(ctx, match, json.RawMessage(fmt.Sprintf(`{"epoch":%v,%s}`, second["epoch"], action))); err == nil || !bytes.Equal(before, match.Data) {
			t.Fatalf("teammate action accepted or mutated input: %s", action)
		}
	}
	match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"egg","quote":%v,"clientInputID":"trainer-one-egg"`, nativeHTTPObject(second["progression"])["remainingEggs"]))
	owner := trainerData(t, module, match, 0)
	partner := trainerData(t, module, match, 1)
	if owner["gold"] != float64(10) || partner["gold"] != float64(40) || len(owner["team"].([]any)) != 3 || len(partner["team"].([]any)) != 4 {
		t.Fatal("egg purchase crossed trainer boundaries")
	}
	if nativeHTTPObject(owner["progression"])["eggPrice"] != float64(10) || nativeHTTPObject(partner["progression"])["eggPrice"] == float64(10) {
		t.Fatal("egg quotes are still shared")
	}
	match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"egg","quote":%v,"clientInputID":"trainer-one-egg"`, nativeHTTPObject(second["progression"])["remainingEggs"]))
	if trainerData(t, module, match, 1)["gold"] != float64(40) {
		t.Fatal("replayed input charged the personal wallet twice")
	}
	match = nativeAction(t, module, ctx, match, `"type":"profile","name":"独立训练家"`)
	if nativeHTTPObject(trainerData(t, module, match, 0)["progression"])["profileName"] == "独立训练家" || nativeHTTPObject(trainerData(t, module, match, 1)["progression"])["profileName"] != "独立训练家" {
		t.Fatal("profile change affected another trainer")
	}
	match = nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"deploy","pokemonID":%d,"tileID":1`, secondID))
	ctx.Actor = 0
	view := trainerData(t, module, match, 0)
	if len(view["towers"].([]any)) != 1 || len(view["battleTeam"].([]any)) != 7 || len(view["team"].([]any)) != 3 {
		t.Fatal("personal interface hid the shared battlefield or included teammates in the roster")
	}
	frozen, err := module.SaveControl(ctx, match, true)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := module.ResumeSaved(ctx, frozen)
	if err != nil {
		t.Fatal(err)
	}
	for actor := 0; actor < 2; actor++ {
		beforeView := trainerData(t, module, match, actor)
		afterView := trainerData(t, module, resumed, actor)
		for _, key := range []string{"gold", "team", "box", "items", "progression"} {
			if !reflect.DeepEqual(beforeView[key], afterView[key]) {
				t.Fatalf("save resume changed trainer %d %s", actor, key)
			}
		}
	}
}

func TestNativeTrainersEveryBotCanManageItsOwnRoster(t *testing.T) {
	module := pokepath.Module()
	_, ctx, match := nativeStart(t, 4)
	ctx.Automated = []bool{true, true, true, true}
	for actor := 0; actor < 4; actor++ {
		ctx.Actor = actor
		action, err := module.BotAction(ctx, match)
		if err != nil || len(action) == 0 {
			t.Fatalf("trainer %d cannot act: %s %v", actor, action, err)
		}
		match, err = module.Apply(ctx, match, action)
		if err != nil {
			t.Fatalf("trainer %d proposed illegal own action: %s %v", actor, action, err)
		}
	}
}
