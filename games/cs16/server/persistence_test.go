package cs16

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type legacyMatchState struct {
	Schema      int          `json:"schema"`
	Epoch       string       `json:"epoch"`
	BaseAt      int64        `json:"baseAt"`
	ObservedAt  int64        `json:"observedAt"`
	RulesSHA256 string       `json:"rulesSHA256"`
	World       engine.State `json:"world"`
}

func TestLazyHistoryPreservesLegacyStateAndSubsequentRules(t *testing.T) {
	ctx, match := cursorFixture(t)
	ctx.Now = ctx.Now.Add(time.Second)
	var err error
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	var original legacyMatchState
	if json.Unmarshal(match.Data, &original) != nil || len(original.World.History) != engine.MaxHistory {
		t.Fatal("ordinary persisted history missing")
	}
	lazy, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	if len(lazy.World.History) != 0 || len(lazy.history) == 0 {
		t.Fatal("control decode unnecessarily expanded history")
	}
	action := engine.Input{Seq: 1, Life: original.World.Players[0].Life, ViewFrame: original.World.Frame, MoveY: 1000, Yaw: 1000}
	updated, err := apply(ctx, match, actionBytes(t, inputAction{Type: "input", Epoch: original.Epoch, Input: action}))
	if err != nil {
		t.Fatal(err)
	}
	var stored legacyMatchState
	if json.Unmarshal(updated.Data, &stored) != nil || !reflect.DeepEqual(original.World.History, stored.World.History) {
		t.Fatal("control rewrite changed legacy history")
	}
	if stored.Schema != original.Schema || stored.Epoch != original.Epoch || stored.BaseAt != original.BaseAt {
		t.Fatal("storage format or match identity changed")
	}
	original.World.World = engine.MapByID(original.World.MapID)
	if err = engine.QueueInput(&original.World, 0, action); err != nil {
		t.Fatal(err)
	}
	engine.Advance(&original.World, original.World.Frame+6)
	for index := range original.World.History {
		original.World.History[index].Brushes = nil
	}
	ctx.Now = ctx.Now.Add(60 * time.Millisecond)
	actual, err := onTimer(ctx, updated)
	if err != nil {
		t.Fatal(err)
	}
	var actualState legacyMatchState
	if json.Unmarshal(actual.Data, &actualState) != nil {
		t.Fatal("new state no longer readable as legacy format")
	}
	expectedRaw, _ := json.Marshal(original.World)
	actualRaw, _ := json.Marshal(actualState.World)
	if !bytes.Equal(expectedRaw, actualRaw) {
		t.Fatal("lazy state changed subsequent complete rule outcome")
	}
	loaded, err := decode(actual)
	if err != nil {
		t.Fatal(err)
	}
	if err = loadHistory(loaded); err != nil || !reflect.DeepEqual(loaded.World.History, actualState.World.History) {
		t.Fatal("timer did not recover exact complete history", err)
	}
}

func TestUnusedHistoricalBrushCopiesDoNotAffectRewindOrCurrentDoor(t *testing.T) {
	for _, open := range []bool{false, true} {
		state := engine.State{Seed: 41, Frame: 100, Phase: "live", Options: engine.Options{RoundSeconds: 120, MaxRounds: 10}, PhaseEndsAt: 12100, Bomb: engine.Bomb{Carrier: -1, Defuser: -1}, World: engine.Map{Mode: "bomb", Planes: []engine.Plane{{Z: engine.NormalUnit, Dist: 0}, {Z: engine.NormalUnit, Dist: 36 * engine.Unit}}, Nodes: []engine.Node{{Plane: 0, Front: engine.ContentsEmpty, Back: engine.ContentsSolid}}, Root: 0, Hulls: []engine.Hull{{Root: 0, Nodes: []engine.Node{{Plane: 1, Front: engine.ContentsEmpty, Back: engine.ContentsSolid}}}}, Brushes: []engine.Brush{{ID: 1, Kind: "func_door", Health: 100, Bounds: engine.Bounds{Min: engine.Vec3{X: 180 * engine.Unit, Y: -50 * engine.Unit, Z: -100 * engine.Unit}, Max: engine.Vec3{X: 220 * engine.Unit, Y: 50 * engine.Unit, Z: 100 * engine.Unit}}}}}, Brushes: []engine.BrushState{{ID: 1, Open: open, Health: 100}}, Players: []engine.Player{{Seat: 0, Team: "t", Character: "terror", Alive: true, Life: 1, Health: 100, Z: 36 * engine.Unit, Grounded: true}, {Seat: 1, Team: "ct", Character: "urban", Alive: true, Life: 1, Health: 100, X: 400 * engine.Unit, Z: 36 * engine.Unit, Grounded: true}}}
		for index := range state.Players {
			engine.InitInventory(&state.Players[index])
		}
		for frame := int64(69); frame <= 100; frame++ {
			state.History = append(state.History, engine.History{Frame: frame, Players: []engine.HistoryPlayer{{Seat: 0, X: 0, Z: 36 * engine.Unit, Alive: true, Life: 1, Weapon: "glock18"}, {Seat: 1, X: 400 * engine.Unit, Z: 36 * engine.Unit, Alive: true, Life: 1, Weapon: "usp"}}, Brushes: []engine.BrushState{{ID: 1, Open: !open, Health: 100}}})
		}
		raw, _ := json.Marshal(state)
		var compact engine.State
		if json.Unmarshal(raw, &compact) != nil {
			t.Fatal("fixture invalid")
		}
		compact.World = state.World
		for index := range compact.History {
			compact.History[index].Brushes = nil
		}
		input := engine.Input{Seq: 1, Life: 1, ViewFrame: 90, Fire: true}
		if err := engine.QueueInput(&state, 0, input); err != nil {
			t.Fatal(err)
		}
		if err := engine.QueueInput(&compact, 0, input); err != nil {
			t.Fatal(err)
		}
		engine.Advance(&state, 101)
		engine.Advance(&compact, 101)
		for index := range state.History {
			state.History[index].Brushes = nil
		}
		for index := range compact.History {
			compact.History[index].Brushes = nil
		}
		full, _ := json.Marshal(state)
		small, _ := json.Marshal(compact)
		if !bytes.Equal(full, small) {
			t.Fatal("unused history brush changed authoritative current-door outcome", open)
		}
		if open && compact.Players[1].Health >= 100 {
			t.Fatal("open current door did not exercise rewind hit")
		}
		if !open && compact.Players[1].Health != 100 {
			t.Fatal("closed current door used historical open geometry")
		}
	}
}

func TestPersistenceOmitsOnlyUnusedHistoricalBrushCopies(t *testing.T) {
	ctx, match := cursorFixture(t)
	ctx.Now = ctx.Now.Add(time.Second)
	match, err := onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	var saved legacyMatchState
	if json.Unmarshal(match.Data, &saved) != nil || len(saved.World.History) != engine.MaxHistory || len(saved.World.Brushes) == 0 {
		t.Fatal("current collision state or complete target history missing")
	}
	for _, frame := range saved.World.History {
		if len(frame.Brushes) != 0 || len(frame.Players) != len(saved.World.Players) {
			t.Fatal("historical brush duplicates retained or target history dropped")
		}
	}
}

func TestLazyHistoryRejectsInvalidPayloadWhenRulesNeedIt(t *testing.T) {
	ctx, match := cursorFixture(t)
	var stored map[string]json.RawMessage
	if json.Unmarshal(match.Data, &stored) != nil {
		t.Fatal("state invalid")
	}
	var world map[string]json.RawMessage
	if json.Unmarshal(stored["world"], &world) != nil {
		t.Fatal("world invalid")
	}
	world["history"] = json.RawMessage(`"corrupt"`)
	stored["world"] = actionBytes(t, world)
	match.Data = actionBytes(t, stored)
	ctx.Now = ctx.Now.Add(30 * time.Millisecond)
	if _, err := onTimer(ctx, match); err == nil {
		t.Fatal("invalid history was ignored during authoritative rules")
	}
}

func TestHeaderFastPathRequiresExactScopedProjection(t *testing.T) {
	ctx, match := cursorFixture(t)
	first, err := streamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged, err := unchangedProjection(match, 0, first.Cursor); err != nil || !unchanged {
		t.Fatal("unchanged header not recognized", err)
	}
	if unchanged, _ := unchangedProjection(match, 1, first.Cursor); unchanged {
		t.Fatal("header fast path crossed viewer")
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Brushes[0].Open = !state.World.Brushes[0].Open
	changed, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if unchanged, _ := unchangedProjection(changed, 0, first.Cursor); unchanged {
		t.Fatal("header fast path ignored brush update")
	}
	state, err = decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Players[0].Alive = false
	changed, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if unchanged, _ := unchangedProjection(changed, 0, first.Cursor); unchanged {
		t.Fatal("header fast path ignored permission update")
	}
	bad := gameapi.Match{Data: json.RawMessage(`{"schema":1,"world":`)}
	if _, err = unchangedProjection(bad, 0, first.Cursor); err == nil {
		t.Fatal("invalid storage header accepted")
	}
}

func TestControlHeaderMatchesFullInputValidation(t *testing.T) {
	ctx, match := cursorFixture(t)
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	var err error
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	full, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	header, err := decodeInputState(match, ctx.Actor)
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"type": "input", "epoch": full.Epoch, "seq": 1, "life": full.World.Players[0].Life, "viewFrame": full.World.Frame, "moveY": 1000, "yaw": 500, "pitch": 0, "fire": false}
	variants := []map[string]any{base}
	for field, value := range map[string]any{"epoch": "old", "life": 99, "seq": 5000, "viewFrame": full.World.Frame + 1, "moveY": 1001, "yaw": 65536, "pitch": 16001, "health": 1000, "cancel": true} {
		item := map[string]any{}
		for key, v := range base {
			item[key] = v
		}
		item[field] = value
		variants = append(variants, item)
	}
	for _, raw := range variants {
		a, fullErr := validatedInput(ctx, full, actionBytes(t, raw))
		b, headerErr := validatedInput(ctx, header, actionBytes(t, raw))
		if (fullErr == nil) != (headerErr == nil) || fullErr == nil && !reflect.DeepEqual(a, b) {
			t.Fatal("control header changed input admission", raw, fullErr, headerErr)
		}
	}
}
