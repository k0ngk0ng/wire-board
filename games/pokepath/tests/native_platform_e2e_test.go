package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	pokepath "github.com/k0ngk0ng/wire-board/games/pokepath/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type nativeObservation struct {
	Epoch          int               `json:"epoch"`
	Sequence       uint64            `json:"sequence"`
	EngineSequence uint64            `json:"engineSequence"`
	Clock          int64             `json:"clock"`
	ServerAt       int64             `json:"serverAt"`
	Gold           int64             `json:"gold"`
	Health         int               `json:"health"`
	Wave           int               `json:"wave"`
	Team           []json.RawMessage `json:"team"`
	Snapshot       json.RawMessage   `json:"snapshot"`
	AcceptedInputs []string          `json:"acceptedInputs"`
	Saving         bool              `json:"saving"`
}

func nativeContext(players, actor int) gameapi.Context {
	return gameapi.Context{Players: players, Actor: actor, Now: time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC), Automated: make([]bool, players)}
}

func nativeStart(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	module := pokepath.Module()
	ctx := nativeContext(players, 0)
	match, err := module.Start(ctx, json.RawMessage(`{"starter":"charmander"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := module.ValidateMatch(match, players); err != nil {
		t.Fatal(err)
	}
	return module, ctx, match
}

func nativeObserve(t *testing.T, module gameapi.Module, match gameapi.Match, viewer int) nativeObservation {
	t.Helper()
	value, err := module.View(match, viewer)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var observation nativeObservation
	if err := json.Unmarshal(encoded, &observation); err != nil {
		t.Fatal(err)
	}
	return observation
}

func nativeAction(t *testing.T, module gameapi.Module, ctx gameapi.Context, match gameapi.Match, fields string) gameapi.Match {
	t.Helper()
	view := nativeObserve(t, module, match, ctx.Actor)
	action := json.RawMessage(fmt.Sprintf(`{"epoch":%d,%s}`, view.Epoch, fields))
	next, err := module.Apply(ctx, match, action)
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
	if err := module.ValidateMatch(next, ctx.Players); err != nil {
		t.Fatal(err)
	}
	return next
}

func nativeComparableSnapshot(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	delete(object, "epoch")
	return object
}

func TestNativePlatformRegistrationOptionsAndSharedStart(t *testing.T) {
	module := pokepath.Module()
	if err := module.Validate(); err != nil {
		t.Fatal(err)
	}
	if module.ID != "pokepath" || module.Name != "宝可梦：巡径守卫" || module.MinPlayers != 1 || module.MaxPlayers != 4 || !module.SupportsSave || module.SaveControl == nil || module.ResumeSaved == nil || module.BotAction == nil || module.Reclaim == nil {
		t.Fatal("incomplete independent cooperative module")
	}
	for players := 1; players <= 4; players++ {
		module, ctx, match := nativeStart(t, players)
		view := nativeObserve(t, module, match, 0)
		if view.Epoch <= 0 || view.Gold != 50 || view.Health != 14 || view.Wave != 1 || len(view.Team) != 1 || len(view.Snapshot) == 0 || len(match.Actors) != players {
			t.Fatalf("cooperative start duplicated or omitted original resources: %+v", view)
		}
		ctx.Actor = players - 1
		nativeAction(t, module, ctx, match, `"type":"speed"`)
	}
	for _, value := range []struct {
		players int
		options string
	}{{0, `{}`}, {5, `{}`}, {1, `null`}, {1, `{"starter":"pikachu"}`}, {2, `{"starter":"treecko","gold":999}`}, {4, `{} {}`}} {
		if _, err := module.Configure(value.players, json.RawMessage(value.options)); err == nil {
			t.Fatalf("accepted invalid configuration: %+v", value)
		}
	}
}

func TestNativePlatformRejectsForgedInputsWithoutMutation(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	view := nativeObserve(t, module, match, 0)
	before := append([]byte(nil), match.Data...)
	for _, raw := range []string{fmt.Sprintf(`{"epoch":%d,"type":"tick","milliseconds":99999}`, view.Epoch), fmt.Sprintf(`{"epoch":%d,"type":"speed","gold":99999}`, view.Epoch), fmt.Sprintf(`{"epoch":%d,"type":"speed","actor":1}`, view.Epoch), fmt.Sprintf(`{"epoch":%d,"type":"load","snapshot":{}}`, view.Epoch), fmt.Sprintf(`{"epoch":%d,"type":"speed"}`, view.Epoch+1), `null`, `{}`, `{"epoch":1,"type":"speed"} {}`} {
		if _, err := module.Apply(ctx, match, json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted forged action: %s", raw)
		}
		if !bytes.Equal(before, match.Data) {
			t.Fatal("rejected action mutated the transaction input")
		}
	}
	for _, actor := range []int{-1, 2, 999} {
		ctx.Actor = actor
		if _, err := module.Apply(ctx, match, json.RawMessage(fmt.Sprintf(`{"epoch":%d,"type":"speed"}`, view.Epoch))); err == nil {
			t.Fatalf("accepted unauthenticated seat %d", actor)
		}
	}
}

func TestNativePlatformSaveResumeFreezesFullStateAndOldEpoch(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	var err error
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor = 0
	before := nativeObserve(t, module, match, 0)
	frozen, err := module.SaveControl(ctx, match, true)
	if err != nil {
		t.Fatal(err)
	}
	if !frozen.Paused || frozen.SystemDeadline != 0 || frozen.Result != nil {
		t.Fatal("save approval did not freeze the complete world")
	}
	serialized, _ := json.Marshal(frozen)
	var recovered gameapi.Match
	_ = json.Unmarshal(serialized, &recovered)
	ctx.Now = ctx.Now.Add(48 * time.Hour)
	resumed, err := module.ResumeSaved(ctx, recovered)
	if err != nil {
		t.Fatal(err)
	}
	after := nativeObserve(t, module, resumed, 0)
	if resumed.Paused || after.Epoch <= before.Epoch || !reflect.DeepEqual(nativeComparableSnapshot(t, before.Snapshot), nativeComparableSnapshot(t, after.Snapshot)) {
		t.Fatal("resume advanced offline rules or reset the saved world")
	}
	if _, err := module.Apply(ctx, resumed, json.RawMessage(fmt.Sprintf(`{"epoch":%d,"type":"speed"}`, before.Epoch))); err == nil {
		t.Fatal("a pre-save input survived the new control epoch")
	}
}

func TestNativePlatformSaveCancelPreservesOriginalPause(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	paused := nativeAction(t, module, ctx, match, `"type":"pause"`)
	if !paused.Paused {
		t.Fatal("manual pause missing")
	}
	frozen, err := module.SaveControl(ctx, paused, true)
	if err != nil {
		t.Fatal(err)
	}
	released, err := module.SaveControl(ctx, frozen, false)
	if err != nil {
		t.Fatal(err)
	}
	if !released.Paused || released.SystemDeadline != 0 {
		t.Fatal("cancelling a save resumed a manually paused world")
	}
}
