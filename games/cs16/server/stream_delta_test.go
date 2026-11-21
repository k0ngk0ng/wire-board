package cs16

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func streamTestObject(t *testing.T, value any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	return object
}

func streamTestApplyObject(target, delta map[string]any) {
	if removed, ok := delta["remove"].([]any); ok {
		for _, key := range removed {
			delete(target, key.(string))
		}
	}
	if values, ok := delta["values"].(map[string]any); ok {
		for key, value := range values {
			target[key] = value
		}
	}
	if objects, ok := delta["objects"].(map[string]any); ok {
		for key, value := range objects {
			streamTestApplyObject(target[key].(map[string]any), value.(map[string]any))
		}
	}
}

func streamTestMerge(t *testing.T, previous View, packet any) View {
	t.Helper()
	value := streamTestObject(t, packet)
	if value["schema"] == json.Number("1") {
		var result View
		raw, _ := json.Marshal(value)
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if value["schema"] != json.Number("2") || value["epoch"] != previous.Epoch || value["baseFrame"] != json.Number(jsonNumber(previous.Frame)) || value["baseEventId"] != json.Number(jsonNumber(previous.EventID)) {
		t.Fatal("delta does not name exact connection baseline", value)
	}
	merged := streamTestObject(t, previous)
	delete(merged, "recover")
	for _, name := range []string{"frame", "serverAt", "eventId", "events"} {
		merged[name] = value[name]
	}
	if global, ok := value["global"].(map[string]any); ok {
		streamTestApplyObject(merged, global)
	}
	if collections, ok := value["collections"].(map[string]any); ok {
		for name, change := range collections {
			idName := "id"
			if name == "players" {
				idName = "seat"
			}
			items := merged[name].([]any)
			changes := change.(map[string]any)
			if removed, ok := changes["remove"].([]any); ok {
				for _, id := range removed {
					for index, item := range items {
						if item.(map[string]any)[idName] == id {
							items = append(items[:index], items[index+1:]...)
							break
						}
					}
				}
			}
			if upserts, ok := changes["upsert"].([]any); ok {
				for _, item := range upserts {
					upsert := item.(map[string]any)
					index := -1
					for candidate, old := range items {
						if old.(map[string]any)[idName] == upsert["id"] {
							index = candidate
						}
					}
					if full, ok := upsert["full"]; ok {
						if index < 0 {
							items = append(items, full)
						} else {
							items[index] = full
						}
					} else if patch, ok := upsert["patch"].(map[string]any); ok && index >= 0 {
						streamTestApplyObject(items[index].(map[string]any), patch)
					} else {
						t.Fatal("patch for unknown entity", name, upsert)
					}
				}
			}
			merged[name] = items
		}
	}
	raw, _ := json.Marshal(merged)
	var result View
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func jsonNumber(value int64) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func streamTestExpected(state *matchState, seat int, eventID int64) View {
	value := projection(state, seat)
	value.Events = []EventView{}
	for _, event := range state.World.Events {
		if event.ID > eventID && eventVisible(state, seat, event) {
			value.Events = append(value.Events, projectedEvent(event))
		}
	}
	return value
}

func TestStreamFieldDeltasRebuildExactAuthoritativeView(t *testing.T) {
	ctx := gameapi.Context{Players: 10, Actor: 0, Now: time.Unix(100000, 0)}
	match, err := start(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := streamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	previous := first.Data.(View)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	cursor := first.Cursor
	packets := []int{}
	fixtures := []map[string]any{}
	for step := 1; step <= 100; step++ {
		state.World.Frame += 3
		state.World.Players[0].X += 123
		state.World.Players[0].Yaw += 11
		state.World.Players[0].AppliedSeq++
		state.World.Players[0].ReceivedSeq++
		state.World.Players[0].Input.Yaw += 11
		state.World.Players[0].Previous.Yaw += 11
		state.World.Players[0].RecoilPitch = 0
		if step == 2 {
			state.World.Players[0].RecoilPitch = 190
			state.World.Players[0].Inventory = append(state.World.Players[0].Inventory, engine.WeaponState{ID: "ak47", Ammo: 30})
		}
		state.World.Bomb.X += 7
		state.World.Brushes[0].Progress = step
		match, err = envelope(state, false, state.BaseAt+state.World.Frame*10)
		if err != nil {
			t.Fatal(err)
		}
		packet, streamErr := streamView(match, 0, cursor)
		if streamErr != nil || packet.Unchanged {
			t.Fatal("changed frame missing", streamErr)
		}
		wire := streamTestObject(t, packet.Data)
		if wire["schema"] != json.Number("2") || wire["mapId"] != nil || wire["options"] != nil || wire["rulesSHA256"] != nil {
			t.Fatal("ordinary packet is not sparse schema2", wire)
		}
		raw, _ := json.Marshal(packet.Data)
		if bytes.Contains(raw, []byte(`"inventory"`)) != (step == 2) {
			t.Fatal("unchanged inventory resent or changed inventory omitted", string(raw))
		}
		merged := streamTestMerge(t, previous, packet.Data)
		expected := streamTestExpected(state, 0, previous.EventID)
		if !reflect.DeepEqual(streamTestObject(t, merged), streamTestObject(t, expected)) {
			t.Fatal("delta lost authoritative values", step, streamTestObject(t, merged), streamTestObject(t, expected))
		}
		if step <= 3 {
			fixtures = append(fixtures, map[string]any{"before": streamTestObject(t, previous), "packet": streamTestObject(t, packet.Data), "after": streamTestObject(t, expected)})
		}
		retry, retryErr := streamView(match, 0, cursor)
		if retryErr != nil || !reflect.DeepEqual(retry, packet) {
			t.Fatal("unsent packet advanced connection baseline", retryErr)
		}
		packets = append(packets, len(raw))
		previous, cursor = merged, packet.Cursor
	}
	sort.Ints(packets)
	if packets[94] > 1600 {
		t.Fatal("ordinary delta body exceeds reserved 1600-byte body budget", packets[94])
	}
	writeStreamArtifact(t, "field-delta.json", map[string]any{"passed": true, "samples": len(packets), "bodyP95Bytes": packets[94], "bodyMaxBytes": packets[99], "scope": "production StreamView sparse ten-seat body; excludes platform envelope and WebSocket framing"})
	writeStreamArtifact(t, "field-delta-cross-language.json", fixtures)
}

func TestStreamDeltasEntityReplacementRemovalAndEventEdges(t *testing.T) {
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Players[1].Team = state.World.Players[0].Team
	state.World.Hostages = []engine.Hostage{{ID: 900, X: 1, Health: 100}}
	state.World.Grenades = []engine.Grenade{{ID: 901, Owner: 0, Active: true}}
	state.World.Drops = []engine.Drop{{ID: 902, X: state.World.Players[0].X, Y: state.World.Players[0].Y, Z: state.World.Players[0].Z, Weapon: engine.WeaponState{ID: "ak47"}}}
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	first, err := streamView(match, -1, "")
	if err != nil {
		t.Fatal(err)
	}
	previous := first.Data.(View)
	state.World.Frame += 3
	state.World.Players[1].Life++
	state.World.Players[1].X += 99
	state.World.Hostages[0].Health = 50
	state.World.Grenades = []engine.Grenade{{ID: 903, Owner: 0, Active: true}}
	state.World.Drops = []engine.Drop{}
	state.World.Brushes = []engine.BrushState{}
	state.World.EventID += 2
	state.World.Events = append(state.World.Events, engine.Event{ID: state.World.EventID - 1, Frame: state.World.Frame, Kind: "shot", Seat: 0, Target: -1}, engine.Event{ID: state.World.EventID, Frame: state.World.Frame, Kind: "death", Seat: 1, Target: 0})
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	packet, err := streamView(match, -1, first.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	wire := streamTestObject(t, packet.Data)
	if wire["schema"] != json.Number("2") {
		t.Fatal("entity changes did not use delta")
	}
	collections := wire["collections"].(map[string]any)
	player := collections["players"].(map[string]any)["upsert"].([]any)[0].(map[string]any)
	if player["full"] == nil || player["patch"] != nil || len(wire["events"].([]any)) != 2 {
		t.Fatal("life replacement or event edge lost", wire)
	}
	merged := streamTestMerge(t, previous, packet.Data)
	if !reflect.DeepEqual(streamTestObject(t, merged), streamTestObject(t, streamTestExpected(state, -1, previous.EventID))) {
		t.Fatal("entity collections did not reconstruct full current state", wire)
	}
	state.World.Frame += 3
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	next, err := streamView(match, -1, packet.Cursor)
	if err != nil || len(streamTestObject(t, next.Data)["events"].([]any)) != 0 {
		t.Fatal("event edge replayed", err)
	}
}

func TestStreamDeltasBoundCorruptionAndConnectionScope(t *testing.T) {
	ctx, match := cursorFixture(t)
	first, err := streamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Cursor) > 64*1024 {
		t.Fatal("cursor exceeds 64KiB")
	}
	for _, cursor := range []string{first.Cursor + "x", strings.Repeat("x", 64*1024+1), first.Cursor[:len(first.Cursor)-1], first.Cursor + ":extra"} {
		result, recoverErr := streamView(match, 0, cursor)
		if recoverErr != nil {
			t.Fatal(recoverErr)
		}
		full, ok := result.Data.(View)
		if result.Unchanged || !ok || !full.Recover || full.Brushes == nil || len(full.Events) != 0 {
			t.Fatal("corrupt cursor did not recover full view")
		}
	}
	ctx.Now = ctx.Now.Add(30 * time.Millisecond)
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	fast, err := streamView(match, 0, first.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	slow, err := streamView(match, 0, "")
	if err != nil || streamTestObject(t, fast.Data)["schema"] != json.Number("2") || streamTestObject(t, slow.Data)["schema"] != json.Number("1") {
		t.Fatal("independent connection baseline changed", err)
	}
	foreign, err := streamView(match, 1, fast.Cursor)
	if err != nil || !foreign.Data.(View).Recover || foreign.Data.(View).You != 1 {
		t.Fatal("baseline crossed authenticated seat", err)
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Frame += 3
	for range 300 {
		state.World.Players[0].Inventory = append(state.World.Players[0].Inventory, engine.WeaponState{ID: "ak47", Ammo: 30})
	}
	large, err := envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	oversized, err := streamView(large, 0, fast.Cursor)
	if err != nil || oversized.Cursor == "" || len(oversized.Cursor) > 64*1024 {
		t.Fatal("oversized baseline did not retain explicit recovery cursor", err)
	}
	recovery, err := streamView(large, 0, oversized.Cursor)
	if err != nil || !recovery.Data.(View).Recover || len(recovery.Data.(View).Events) != 0 {
		t.Fatal("oversized baseline replayed prior event edges", err)
	}
}

func TestStreamDeltasVisibilityReplacesHiddenFields(t *testing.T) {
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Players[1].Team = state.World.Players[0].Team
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	first, err := streamView(match, 0, "")
	if err != nil || !first.Data.(View).Players[1].Visible {
		t.Fatal("visible baseline missing", err)
	}
	state.World.Players[1].Team = "ct"
	state.World.Frame += 3
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	packet, err := streamView(match, 0, first.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	change := packet.Data.(DeltaView).Collections["players"].Upsert[0]
	var hidden map[string]json.RawMessage
	if json.Unmarshal(change.Full, &hidden) != nil || change.Patch != nil || string(hidden["visible"]) != "false" || hidden["x"] != nil || hidden["inventory"] != nil || hidden["input"] != nil || hidden["money"] != nil {
		t.Fatal("hidden replacement retained sensitive fields", string(change.Full))
	}
	merged := streamTestMerge(t, first.Data.(View), packet.Data)
	if merged.Players[1].Visible || merged.Players[1].X != nil || !reflect.DeepEqual(streamTestObject(t, merged), streamTestObject(t, streamTestExpected(state, 0, first.Data.(View).EventID))) {
		t.Fatal("hidden replacement failed to clear old coordinates")
	}
}

func writeStreamArtifact(t *testing.T, name string, value any) {
	t.Helper()
	directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "server")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(directory, name), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
