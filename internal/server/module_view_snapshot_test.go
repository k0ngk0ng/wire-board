package server

// Isolated copy failures before changing HTTP view snapshots: opaque payload
// bytes are rewritten by JSON serialization; a callback can mutate authoritative
// slices or terminal result details; nil and explicit empty attention diverge.
// Rule validation and HTTP lifecycle remain covered by module E2E tests.
import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestModuleViewOwnsOpaqueMatchSnapshot(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "populated", true: "empty-attention"}[empty], func(t *testing.T) {
			source := gameapi.Match{Schema: 1, Data: json.RawMessage("{\n \"secret\": \"<&>\"\n}"), Actors: []int{0}, TimeoutActors: []int{0}, Attention: []int{0}, Eliminated: []int{1}, Log: []string{"original"}, Result: &gameapi.Result{Ranks: []int{1, 2}, Winners: []int{0}, Scores: []int{4, 0}, Details: json.RawMessage("{\n \"event\": \"<&>\"\n}")}}
			if empty {
				source.Attention = []int{}
			}
			before, _ := json.Marshal(source)
			payload, details := bytes.Clone(source.Data), bytes.Clone(source.Result.Details)
			module := gameapi.Module{View: func(snapshot gameapi.Match, _ int) (any, error) {
				if !bytes.Equal(snapshot.Data, payload) || !bytes.Equal(snapshot.Result.Details, details) {
					t.Error("opaque payload was rewritten while making a view snapshot")
				}
				if !reflect.DeepEqual(source.Attention, snapshot.Attention) {
					t.Error("nil/empty attention changed during view snapshot")
				}
				snapshot.Data[0] = '!'
				snapshot.Actors[0], snapshot.TimeoutActors[0] = 1, 1
				if len(snapshot.Attention) != 0 {
					snapshot.Attention[0] = 1
				}
				snapshot.Eliminated[0], snapshot.Log[0] = 0, "changed"
				snapshot.Result.Ranks[0], snapshot.Result.Winners[0], snapshot.Result.Scores[0] = 2, 1, 0
				snapshot.Result.Details[0] = '!'
				return map[string]any{"visible": true}, nil
			}}
			s := &Server{modules: map[string]gameapi.Module{"snapshot": module}}
			room := &Room{Kind: "snapshot", Module: &source, Game: &game.State{Kind: "snapshot", Log: []string{}, Winners: []int{0}}}
			if _, err := s.moduleView(room, 0); err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(source)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("view callback changed the authoritative match", err)
			}
		})
	}
}

// Room clone failures: opaque module bytes change, mutable room metadata leaks,
// the module is shared by candidates, or the legacy non-module path is lost.
func TestModuleRoomCloneOwnsPayloadAndRoomMetadata(t *testing.T) {
	for _, withModule := range []bool{true, false} {
		t.Run(map[bool]string{true: "module", false: "legacy"}[withModule], func(t *testing.T) {
			room := &Room{Kind: "snapshot", ID: "original", Options: json.RawMessage(`{"mode":"original"}`), Seats: []Seat{{User: User{ID: "human", Name: "original"}}}, SaveVotes: []string{"human"}, Game: &game.State{Kind: "snapshot", Log: []string{"original"}, Winners: []int{0}}}
			if withModule {
				room.Module = &gameapi.Match{Schema: 1, Data: json.RawMessage("{\n \"secret\": \"<&>\"\n}"), Actors: []int{0}, Attention: []int{}, Log: []string{"original"}}
			}
			before, _ := json.Marshal(room)
			next, err := cloneBotRoom(room)
			if err != nil {
				t.Fatal(err)
			}
			if withModule {
				if !bytes.Equal(next.Module.Data, room.Module.Data) || !reflect.DeepEqual(next.Module.Attention, room.Module.Attention) {
					t.Error("room clone rewrote opaque payload or attention")
				}
				next.Module.Data[0], next.Module.Actors[0], next.Module.Log[0] = '!', 1, "changed"
			} else if next.Module != nil {
				t.Fatal("legacy room gained a module")
			}
			next.Options[0], next.Seats[0].Name, next.SaveVotes[0] = '!', "changed", "changed"
			next.Game.Log[0], next.Game.Winners[0] = "changed", 1
			after, err := json.Marshal(room)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("candidate room mutation escaped into source", err)
			}
		})
	}
}
