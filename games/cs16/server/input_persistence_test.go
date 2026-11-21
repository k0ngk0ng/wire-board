package cs16

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func ordinaryInputReference(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	state, err := decode(match)
	if err != nil {
		return match, err
	}
	action, err := validatedInput(ctx, state, raw)
	if err != nil {
		return match, err
	}
	if err = engine.QueueInput(&state.World, ctx.Actor, action.Input); err != nil {
		return match, err
	}
	updated, err := envelope(state, computerParticipants(ctx, match.HasComputerPlayers), ctx.Now.UnixMilli())
	updated.SystemDeadline = match.SystemDeadline
	return updated, err
}

func TestQueuedInputPersistenceMatchesCompleteRules(t *testing.T) {
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"} {
		for _, friendly := range []bool{false, true} {
			ctx := gameapi.Context{Now: time.Unix(1700000000, 0), Players: 10, Automated: make([]bool, 10)}
			options, _ := json.Marshal(map[string]any{"map": mapID, "roundSeconds": 120, "maxRounds": 2, "friendlyFire": friendly})
			actual, err := start(ctx, options)
			if err != nil {
				t.Fatal(err)
			}
			expected := actual
			for tick := 0; tick < 40; tick++ {
				ctx.Now = ctx.Now.Add(30 * time.Millisecond)
				for actor := range 10 {
					ctx.Actor = actor
					state, err := decode(expected)
					if err != nil {
						t.Fatal(err)
					}
					player := state.World.Players[actor]
					raw, _ := json.Marshal(inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: player.ReceivedSeq + 1, Life: player.Life, ViewFrame: state.World.Frame, Yaw: tick * 137, MoveX: 1000, Crouch: tick%3 == 0, Jump: tick%7 == 0}})
					before := bytes.Clone(actual.Data)
					actual, err = apply(ctx, actual, raw)
					if err != nil {
						t.Fatal(mapID, tick, actor, err)
					}
					expected, err = ordinaryInputReference(ctx, expected, raw)
					if err != nil || bytes.Equal(before, actual.Data) {
						t.Fatal("input did not enter a new snapshot", err)
					}
					var first, second legacyMatchState
					if json.Unmarshal(actual.Data, &first) != nil || json.Unmarshal(expected.Data, &second) != nil || !reflect.DeepEqual(first, second) {
						t.Fatal("queued input changed authoritative state", mapID, tick, actor)
					}
				}
				actual, err = onTimer(ctx, actual)
				if err != nil {
					t.Fatal(err)
				}
				expected, err = onTimer(ctx, expected)
				if err != nil || !bytes.Equal(actual.Data, expected.Data) {
					t.Fatal("subsequent complete physics diverged", mapID, tick, err)
				}
			}
		}
	}
}

func TestQueuedInputPreservesOpaqueArchiveAndOriginalBytes(t *testing.T) {
	ctx, match := cursorFixture(t)
	ctx.Now = ctx.Now.Add(time.Second)
	match, err := onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	var indented bytes.Buffer
	if err = json.Indent(&indented, match.Data, "", "  "); err != nil {
		t.Fatal(err)
	}
	match.Data = bytes.Replace(indented.Bytes(), []byte("{\n"), []byte("{\n  \"extension\": {\"nested\":[\"} \\\" world\",{\"players\":[null],\"finished\":true}]},\n"), 1)
	before := bytes.Clone(match.Data)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[ctx.Actor].Life, ViewFrame: state.World.Frame, MoveY: 1000}})
	updated, err := apply(ctx, match, raw)
	if err != nil || !bytes.Equal(match.Data, before) {
		t.Fatal("source archive mutated", err)
	}
	var original, result map[string]json.RawMessage
	if json.Unmarshal(before, &original) != nil || json.Unmarshal(updated.Data, &result) != nil || !bytes.Equal(original["extension"], result["extension"]) {
		t.Fatal("unrelated opaque archive extension was rewritten")
	}
}

func TestQueuedInputArchiveValidationMatchesOriginalPath(t *testing.T) {
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[ctx.Actor].Life, ViewFrame: state.World.Frame}})
	for _, data := range []string{
		string(match.Data),
		strings.Replace(string(match.Data), "\"schema\":1", "\"schema\":1,\"schema\":1", 1),
		strings.Replace(string(match.Data), "\"schema\":1", "\"schema\":\"1\"", 1),
		strings.Replace(string(match.Data), "\"schema\":1", "\"schema\":0", 1),
		strings.Replace(string(match.Data), "\"world\":", "\"\\u0077orld\":", 1),
		string(match.Data[:len(match.Data)-1]),
	} {
		candidate := match
		candidate.Data = json.RawMessage(data)
		actual, got := apply(ctx, candidate, raw)
		expected, want := ordinaryInputReference(ctx, candidate, raw)
		if (got == nil) != (want == nil) {
			t.Fatal("archive validation changed", data, got, want)
		}
		if got == nil {
			var first, second legacyMatchState
			if json.Unmarshal(actual.Data, &first) != nil || json.Unmarshal(expected.Data, &second) != nil || !reflect.DeepEqual(first, second) {
				t.Fatal("archive layout changed rule output")
			}
		}
	}
}
