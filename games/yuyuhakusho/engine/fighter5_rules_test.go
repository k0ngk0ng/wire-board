package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestFighter5WorldRuleFrames(t *testing.T) {
	raw, err := os.ReadFile("testdata/fighter5-projectile-data.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Source string `json:"source_sha256"`
		Frames []chuWorldAnimationRule
	}
	if err := json.Unmarshal(raw, &data); err != nil || data.Source != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || len(data.Frames) != 33 {
		t.Fatal("unbound source5 semantic frames", err)
	}
	library := sessionLibrary(t)
	for _, frame := range data.Frames {
		if got, ok := library.Animations[frame.Key]; !ok || !reflect.DeepEqual(got, frame.Data) {
			t.Fatal("source5 typed frame omitted or altered", frame.Key)
		}
	}
}
