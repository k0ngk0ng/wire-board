package cs16

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHostageDamagePrecisionStaysOutOfPublicProjection(t *testing.T) {
	state := observerFixture()
	raw, err := json.Marshal(state.World)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["hostageDamageStates"] = json.RawMessage(`[{"id":1,"healthRemainder":360000,"hitgroup":6}]`)
	raw, err = json.Marshal(fields)
	if err != nil || json.Unmarshal(raw, &state.World) != nil {
		t.Fatal("invalid private damage fixture", err)
	}
	raw, err = json.Marshal(state.World)
	if err != nil || !strings.Contains(string(raw), "hostageDamageStates") {
		t.Fatal("private hostage damage precision was not saved", err)
	}
	for _, actor := range []int{-1, 0, 1} {
		raw, err = json.Marshal(projection(state, actor))
		if err != nil || strings.Contains(string(raw), "hostageDamageStates") || strings.Contains(string(raw), "healthRemainder") {
			t.Fatal("private hostage damage precision leaked into public view", actor, err)
		}
	}
}
