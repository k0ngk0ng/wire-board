package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestFighter7WorldRuleFrames(t *testing.T) {
	raw, err := os.ReadFile("../data/fighter7-children-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct{ Frames []chuWorldAnimationRule }
	if err := json.Unmarshal(raw, &source); err != nil || len(source.Frames) != 94 {
		t.Fatal("missing source7 typed rules", err)
	}
	library := sessionLibrary(t)
	for _, frame := range source.Frames {
		if got, ok := library.Animations[frame.Key]; !ok || !reflect.DeepEqual(got, frame.Data) {
			t.Fatal("source7 typed frame omitted or altered", frame.Key)
		}
	}
}
