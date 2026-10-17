package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Three exact fixed-seed failures, retained before implementation. Never rewrite
// positions or requests to bypass missing handlers; verify JSON continuation.
func TestRuntimeCharacter23AttackBeforeContinues(t *testing.T) {
	for _, name := range []string{"handler-28", "handler-31", "handler-27", "variant27"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/attack-" + name + "-runtime-before.json")
			if e != nil {
				t.Fatal(e)
			}
			var state RuntimeState
			if e = json.Unmarshal(raw, &state); e != nil {
				t.Fatal(e)
			}
			for tick := 0; tick < 24; tick++ {
				next, e := StepVideo(state, [4]Buttons{})
				if e != nil {
					t.Fatal(tick, e)
				}
				packed, e := json.Marshal(state)
				if e != nil {
					t.Fatal(e)
				}
				var restored RuntimeState
				if e = json.Unmarshal(packed, &restored); e != nil {
					t.Fatal(e)
				}
				replay, e := StepVideo(restored, [4]Buttons{})
				if e != nil || !reflect.DeepEqual(replay, next) {
					t.Fatal("restore differs", tick, e)
				}
				state = next.State
			}
		})
	}
}
