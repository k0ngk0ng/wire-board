package rootcontrol_test

import (
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
)

func TestOrchestratorContractOwnsNoSecondStateOrClock(t *testing.T) {
	owners := reflect.TypeFor[rootcontrol.RuleOwners]()
	if owners.NumField() != 3 {
		t.Fatal("rules orchestrator gained another state owner", owners)
	}
	for i, want := range []string{"Score", "Queue", "AudioPause"} {
		if owners.Field(i).Name != want || owners.Field(i).Type.Kind() != reflect.Pointer {
			t.Fatal("detached owners ceased to be unique references", owners.Field(i))
		}
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[rootcontrol.RuleOperation](), reflect.TypeFor[rootcontrol.RuleOutput]()} {
		for _, forbidden := range []string{"Cycle", "Clock", "PC", "RAM", "State", "Root", "Queue", "Voices", "PCM", "YM", "Cost"} {
			if _, exists := typ.FieldByName(forbidden); exists {
				t.Fatal("rule effect stores another owner or fabricated timing", typ, forbidden)
			}
		}
	}
}
