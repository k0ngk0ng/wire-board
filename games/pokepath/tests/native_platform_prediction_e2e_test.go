package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestNativePlatformSharedCostAndDiscardedCandidate(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	view := nativeObserve(t, module, match, 0)
	var owned struct {
		ID int `json:"id"`
	}
	_ = json.Unmarshal(view.Team[0], &owned)
	before := append([]byte(nil), match.Data...)
	upgraded := nativeAction(t, module, ctx, match, fmt.Sprintf(`"type":"upgrade","pokemonID":%d`, owned.ID))
	ctx.Actor = 1
	if _, err := module.Apply(ctx, upgraded, json.RawMessage(fmt.Sprintf(`{"epoch":%d,"type":"upgrade","pokemonID":%d}`, nativeObserve(t, module, upgraded, 1).Epoch, owned.ID))); err == nil {
		t.Fatal("second seat spent already exhausted shared gold")
	}
	if !bytes.Equal(before, match.Data) {
		t.Fatal("accepted candidate mutated its transaction input")
	}
	retried := nativeAction(t, module, ctx, match, `"type":"egg","quote":149`)
	recovered := nativeObserve(t, module, retried, 1)
	if recovered.Gold != 40 || len(recovered.Team) != 2 {
		t.Fatal("a discarded candidate leaked into another player's transaction")
	}
}

func TestNativePlatformPredictionCorrelationAndStream(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	initial, err := module.StreamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	for actor := 0; actor < 2; actor++ {
		ctx.Actor = actor
		match = nativeAction(t, module, ctx, match, `"type":"speed","clientInputID":"same-id"`)
	}
	ctx.Actor = 0
	before := append([]byte(nil), match.Data...)
	match = nativeAction(t, module, ctx, match, `"type":"speed","clientInputID":"same-id"`)
	if !bytes.Equal(before, match.Data) {
		t.Fatal("a repeated local prediction input executed twice")
	}
	view := nativeObserve(t, module, match, 0)
	if _, err := module.Apply(ctx, match, json.RawMessage(fmt.Sprintf(`{"type":"wave","epoch":%d,"clientInputID":"same-id"}`, view.Epoch))); err == nil {
		t.Fatal("a repeated input ID changed its meaning")
	}
	ctx.Actor = 1
	match = nativeAction(t, module, ctx, match, `"type":"pause","clientInputID":"only-second"`)
	if !reflect.DeepEqual(nativeObserve(t, module, match, 0).AcceptedInputs, []string{"same-id"}) || !reflect.DeepEqual(nativeObserve(t, module, match, 1).AcceptedInputs, []string{"same-id", "only-second"}) || len(nativeObserve(t, module, match, -1).AcceptedInputs) != 0 {
		t.Fatal("recovery exposed or lost another seat's input confirmations")
	}
	stream, err := module.StreamView(match, 0, initial.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(stream.Data)
	var update struct {
		Snapshot json.RawMessage `json:"snapshot"`
		Ops      []struct {
			Actor int    `json:"actor"`
			ID    string `json:"clientInputID"`
		} `json:"operations"`
		From *uint64 `json:"fromSequence"`
	}
	_ = json.Unmarshal(encoded, &update)
	if len(update.Snapshot) != 0 || len(update.Ops) != 3 || update.Ops[0].Actor != 0 || update.Ops[1].Actor != 1 || update.Ops[0].ID != "same-id" || update.Ops[1].ID != "same-id" || update.From == nil {
		t.Fatalf("ambiguous prediction stream or repeated full state: %s", encoded)
	}
	if bytes.Contains(nativeObserve(t, module, match, 0).Snapshot, []byte("clientInputID")) {
		t.Fatal("network correlation metadata entered the independent game rules")
	}
}

func TestNativePlatformStreamCarriesOnlyDeterministicOperations(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	initial, err := module.StreamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	match = nativeAction(t, module, ctx, match, `"type":"speed","clientInputID":"compact:1"`)
	for _, cursor := range []string{initial.Cursor, fmt.Sprintf("%d:%d", nativeObserve(t, module, match, 0).Epoch, nativeObserve(t, module, match, 0).Sequence)} {
		update, err := module.StreamView(match, 0, cursor)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(update.Data)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		allowed := map[string]bool{"epoch": true, "sequence": true, "engineSequence": true, "clock": true, "serverAt": true, "players": true, "paused": true, "saving": true, "gold": true, "health": true, "wave": true, "route": true, "fromSequence": true, "operations": true}
		for field := range fields {
			if !allowed[field] {
				t.Fatalf("normal deterministic stream repeats %s: %d bytes", field, len(raw))
			}
		}
		for _, required := range []string{"epoch", "sequence", "engineSequence", "clock", "gold", "health", "wave", "fromSequence"} {
			if _, ok := fields[required]; !ok {
				t.Fatalf("compact stream lost %s", required)
			}
		}
		if len(raw) > 1024 {
			t.Fatalf("single action stream exceeds 1024 bytes: %d", len(raw))
		}
	}
	for _, cursor := range []string{"invalid", "999:0", "1:999999"} {
		update, err := module.StreamView(match, 0, cursor)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(update.Data)
		var full map[string]json.RawMessage
		_ = json.Unmarshal(raw, &full)
		if len(full["snapshot"]) == 0 || len(full["team"]) == 0 || len(full["tiles"]) == 0 {
			t.Fatal("recovery lost the complete native state and initial projection")
		}
	}
}
