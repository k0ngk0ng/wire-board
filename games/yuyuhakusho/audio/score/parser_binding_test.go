package score_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func TestParserMetadataSemanticBindingAndScalarTypes(t *testing.T) {
	c := catalog(t)
	_, original := parserMetadata(t, c)
	for _, name := range []string{"empty-work", "wrong-pitch-family", "wrong-pitch-form", "null-loop-first", "null-loop-taken", "null-duration-additions"} {
		t.Run(name, func(t *testing.T) {
			var document map[string]any
			if err := json.Unmarshal(original, &document); err != nil {
				t.Fatal(err)
			}
			changed := false
			for _, record := range document["records"].([]any) {
				for _, variant := range record.(map[string]any)["variants"].([]any) {
					v := variant.(map[string]any)
					if name == "empty-work" {
						v["work"] = []any{}
						changed = true
						break
					}
					for _, value := range v["work"].([]any) {
						w := value.(map[string]any)
						kind := w["kind"]
						switch {
						case name == "wrong-pitch-family" && kind == "pitch-resolve-site" && w["family"] == "dac":
							w["family"] = "fm"
							changed = true
						case name == "wrong-pitch-form" && kind == "pitch-resolve-site" && w["form"] == "dac":
							w["form"] = "chromatic"
							changed = true
						case name == "null-loop-first" && kind == "loop":
							w["first"] = nil
							changed = true
						case name == "null-loop-taken" && kind == "loop":
							w["taken"] = nil
							changed = true
						case name == "null-duration-additions" && kind == "duration-scale":
							w["additions"] = nil
							changed = true
						}
						if changed {
							break
						}
					}
					if changed {
						break
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("negative input not reached")
			}
			raw, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = score.DecodeParserMetadata(c, bytes.NewReader(raw)); err == nil {
				t.Fatal("semantically invalid parser work accepted")
			}
		})
	}
}
