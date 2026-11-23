package cs16

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestBombCueIntegerScheduleAndPrivacy(t *testing.T) {
	state := observerFixture()
	state.World.Players[0].Alive = true
	state.World.Bomb = engine.Bomb{Carrier: -1, Planted: true, X: -2000 * engine.Unit, Y: 600 * engine.Unit, Z: 25 * engine.Unit, ExplodesAt: 5500, Defuser: 1, DefuseAt: 3210, PickupAt: 77}
	cases := []struct {
		Elapsed int64 `json:"elapsed"`
		Code    int   `json:"code"`
	}{{-1, 0}, {0, 0}, {49, 0}, {50, 9}, {51, 9}, {189, 9}, {190, 17}, {191, 17}, {1030, 65}, {1099, 65}, {1100, 65}, {1101, 65}, {1169, 65}, {1170, 74}, {2010, 122}, {2089, 122}, {2090, 122}, {2091, 122}, {2149, 122}, {2150, 131}, {2979, 171}, {2980, 171}, {2981, 171}, {2982, 171}, {2989, 171}, {2990, 180}, {3781, 220}, {3782, 220}, {3783, 220}, {3829, 220}, {3830, 229}, {4390, 261}, {4499, 261}, {4500, 0}, {4501, 0}}
	for _, fixture := range cases {
		t.Run(fmt.Sprint(fixture.Elapsed), func(t *testing.T) {
			state.World.Frame = 1000 + fixture.Elapsed
			for _, seat := range []int{-1, 0, 1, 2, 3} {
				view := projection(state, seat)
				object := streamTestObject(t, view)
				want := any(nil)
				if fixture.Code > 0 {
					want = json.Number(fmt.Sprint(fixture.Code))
				}
				if object["bombCue"] != want || view.Bomb.ExplodesAt != 0 || view.Bomb.PickupAt != 0 {
					t.Fatal("cue or private deadline projection incorrect", seat, object["bombCue"], want, view.Bomb)
				}
				if seat == 0 && (view.Bomb.Defuser != -1 || view.Bomb.DefuseAt != 0) {
					t.Fatal("global cue leaked CT defuse progress")
				}
				if seat == 1 && (view.Bomb.X != 0 || view.Bomb.Y != 0 || view.Bomb.Z != 0 || view.Bomb.Defuser != 1 || view.Bomb.DefuseAt != 3210) {
					t.Fatal("global cue changed bomb visibility or authorized defuse progress", view.Bomb)
				}
			}
		})
	}
	state.World.Frame = 1100
	state.World.Bomb.Planted = false
	if streamTestObject(t, projection(state, 0))["bombCue"] != nil {
		t.Fatal("unplanted C4 retained global cue")
	}
	writeStreamArtifact(t, "bomb-cue-boundaries.json", map[string]any{"passed": true, "cases": cases, "seats": []int{-1, 0, 1, 2, 3}, "deadlinePublic": false, "hiddenPositionPublic": false})
}

func TestBombCueStreamEdgesAreSparseAndRecoverable(t *testing.T) {
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Bomb = engine.Bomb{Carrier: -1, Planted: true, ExplodesAt: 5500, Defuser: -1}
	state.World.Frame = 1049
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	packet, err := streamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	previous, cursor := packet.Data.(View), packet.Cursor
	records := []map[string]any{}
	for _, frame := range []int64{1050, 1051, 1189, 1190, 1191, 2100, 2170, 5500, 5501} {
		state.World.Frame = frame
		match, err = envelope(state, false, ctx.Now.UnixMilli())
		if err != nil {
			t.Fatal(err)
		}
		packet, err = streamView(match, 0, cursor)
		if err != nil {
			t.Fatal(err)
		}
		wire := streamTestObject(t, packet.Data)
		if wire["schema"] != json.Number("2") {
			t.Fatal("ordinary C4 update required full snapshot")
		}
		merged := streamTestMerge(t, previous, packet.Data)
		expected := streamTestExpected(state, 0, previous.EventID)
		if !reflect.DeepEqual(streamTestObject(t, merged), streamTestObject(t, expected)) {
			t.Fatal("cue edge failed exact delta reconstruction", frame)
		}
		before, after := streamTestObject(t, previous)["bombCue"], streamTestObject(t, merged)["bombCue"]
		raw, _ := json.Marshal(packet.Data)
		if bytes.Contains(raw, []byte(`"bombCue"`)) != (before != after) || bytes.Contains(raw, []byte(`"explodesAt"`)) || bytes.Contains(raw, []byte(`"inventory"`)) || bytes.Contains(raw, []byte(`"brushes"`)) {
			t.Fatal("C4 update resent private, unchanged, or static values", string(raw))
		}
		baseline, valid := readStreamCursor(packet.Cursor, 0)
		if !valid || !bytes.Equal(baseline.Object["bombCue"], mustJSON(t, after)) && after != nil {
			t.Fatal("cursor omitted exact current sound code")
		}
		var bomb engine.Bomb
		if json.Unmarshal(baseline.Object["bomb"], &bomb) != nil || bomb.ExplodesAt != 0 {
			t.Fatal("cursor exposed private bomb timer")
		}
		recovery, recoverErr := syncSnapshot(match, 0)
		if recoverErr != nil || !reflect.DeepEqual(streamTestObject(t, recovery)["bombCue"], after) {
			t.Fatal("recovery lost current cue baseline", recoverErr)
		}
		records = append(records, map[string]any{"frame": frame, "bodyBytes": len(raw), "cue": after})
		previous, cursor = merged, packet.Cursor
	}
	writeStreamArtifact(t, "bomb-cue-stream.json", map[string]any{"passed": true, "packets": records})
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBotMemoryProjectionRemainsPrivateAcrossNetworkEntrypoints(t *testing.T) {
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.BotMemories = []engine.BotMemory{{Life: 1, Round: 1, SoundCursor: 63, HeardSource: 1, HeardLife: 1, HeardPoint: engine.Vec3{X: 9999991, Y: 9999992, Z: 9999993}, HeardFrame: 11, HeardSince: 10, HeardUntil: 261, ProgressPoint: engine.Vec3{X: 9999994}, ProgressAt: 15, EscapePoint: engine.Vec3{Y: 9999995}, EscapeUntil: 300, EscapeCount: 2}}
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decode(match)
	if err != nil || !reflect.DeepEqual(restored.World.BotMemories, state.World.BotMemories) {
		t.Fatal("private bot memory did not survive storage round trip", err)
	}
	for _, seat := range []int{-1, 0, 1} {
		full, fullErr := project(match, seat)
		recovery, recoveryErr := syncSnapshot(match, seat)
		packet, streamErr := streamView(match, seat, "")
		if fullErr != nil || recoveryErr != nil || streamErr != nil {
			t.Fatal(fullErr, recoveryErr, streamErr)
		}
		baseline, valid := readStreamCursor(packet.Cursor, seat)
		if !valid {
			t.Fatal("invalid public cursor")
		}
		for _, data := range []any{full, recovery, packet.Data, baseline.Object} {
			raw := mustJSON(t, data)
			for _, forbidden := range []string{"botMemories", "heardPoint", "soundCursor", "escapePoint", "999999"} {
				if bytes.Contains(raw, []byte(forbidden)) {
					t.Fatal("public entrypoint leaked internal bot memory", seat, forbidden)
				}
			}
		}
		restored.World.BotMemories[0].HeardPoint.X++
		changed, changedErr := envelope(restored, false, ctx.Now.UnixMilli())
		if changedErr != nil {
			t.Fatal(changedErr)
		}
		same, sameErr := streamView(changed, seat, packet.Cursor)
		if sameErr != nil || !same.Unchanged || same.Cursor != packet.Cursor {
			t.Fatal("private memory mutation changed public cursor", sameErr)
		}
		restored.World.Frame += 3
		changed, changedErr = envelope(restored, false, ctx.Now.UnixMilli())
		if changedErr != nil {
			t.Fatal(changedErr)
		}
		delta, deltaErr := streamView(changed, seat, packet.Cursor)
		if deltaErr != nil || streamTestObject(t, delta.Data)["schema"] != json.Number("2") {
			t.Fatal("bot memory changed ordinary sparse stream", deltaErr)
		}
		if bytes.Contains(mustJSON(t, delta.Data), []byte("999999")) || bytes.Contains(mustJSON(t, delta.Data), []byte("botMemories")) {
			t.Fatal("ordinary delta leaked internal memory")
		}
		restored.World.Frame -= 3
	}
	writeStreamArtifact(t, "bot-memory-privacy.json", map[string]any{"passed": true, "seats": []int{-1, 0, 1}, "entrypoints": []string{"project", "syncSnapshot", "streamView", "cursor", "ordinaryDelta"}, "storageRoundTrip": true})
}
