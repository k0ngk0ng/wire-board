package assets

// Failure inventory before implementation: missing76 delivery, invalid sample
// bounds/duration/period/rate, mismatched resource type, and accepting a loop
// descriptor independently from the manifest used by the frontend.
import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMusic76DeliveryRevision(t *testing.T) {
	revision, e := MusicRevision(76)
	if e != nil || len(revision) != 64 {
		t.Fatal("missing verified76 revision", revision, e)
	}
}
func TestMusic76RejectsInvalidLoopMetadata(t *testing.T) {
	saved := playbackManifest
	t.Cleanup(func() { playbackManifest = saved })
	valid := map[string]any{"file": "music-076-intro-loop.mp3", "mime_type": "audio/mpeg", "distribution_sha256": strings.Repeat("a", 64), "loop": true, "sample_rate": 44100, "loop_start_sample": 5465255, "loop_end_sample": 32515748, "loop_samples": 27050493, "decoded_samples": 32515748}
	for _, field := range []string{"sample_rate", "loop_start_sample", "loop_end_sample", "loop_samples", "decoded_samples", "file", "mime_type", "loop"} {
		bad := map[string]any{}
		for k, v := range valid {
			bad[k] = v
		}
		switch field {
		case "file":
			bad[field] = "preview.wav"
		case "mime_type":
			bad[field] = "audio/wav"
		case "loop":
			bad[field] = false
		default:
			bad[field] = 0
		}
		playbackManifest, _ = json.Marshal(map[string]any{"music": map[string]any{"76": bad}})
		if _, e := MusicRevision(76); e == nil {
			t.Errorf("accepted invalid76 %s", field)
		}
	}
	playbackManifest, _ = json.Marshal(map[string]any{"music": map[string]any{"76": valid}})
	if _, e := MusicRevision(76); e != nil {
		t.Fatal(e)
	}
}
