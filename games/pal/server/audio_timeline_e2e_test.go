package pal

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalNativeAudioRequestsThroughAuthorityTimeline(t *testing.T) {
	if os.Getenv("PAL_AUDIO_E2E") != "1" {
		t.Skip("explicit real native audio request trace")
	}
	data, err := os.ReadFile("../../../.local/pal/verification/audio-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var trace struct {
		Commands []engine.AudioCommand `json:"commands"`
	}
	if err = json.Unmarshal(data, &trace); err != nil {
		t.Fatal(err)
	}
	if len(trace.Commands) != 14 {
		t.Fatal("unexpected original native API trace")
	}
	s := state{ServerAt: 10000}
	checks := []any{}
	for i, command := range trace.Commands {
		journal := &engine.AudioJournal{Serial: command.Seq, Commands: trace.Commands[:i+1]}
		if err = syncNativeAudio(&s, journal, 10000, nil); err != nil {
			t.Fatal(err)
		}
		if command.Kind == 1 && command.ID == 0 && s.Public.Audio.Music.Kind != 3 {
			t.Fatal("MIDI stop incorrectly stopped the CD")
		}
		if command.Kind == 3 && command.ID == 12 && s.Public.Audio.Music != nil {
			t.Fatal("unavailable CD played instead of fallback")
		}
		copy, _ := json.Marshal(s.Public.Audio)
		checks = append(checks, json.RawMessage(copy))
	}
	if len(s.Public.Audio.Sounds) != 2 || s.Public.Audio.Music == nil || s.Public.Audio.Music.ID != 26 || !s.Public.Audio.Music.Loop {
		t.Fatal("lost simultaneous effects or fallback music", s.Public.Audio)
	}
	before, _ := json.Marshal(s)
	if err = syncNativeAudio(&s, &engine.AudioJournal{Serial: trace.Commands[len(trace.Commands)-1].Seq, Commands: trace.Commands}, 10000, nil); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(s)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("same native journal replayed audio")
	}
	pruneAudio(&s, 10000+60000)
	if len(s.Public.Audio.Sounds) != 0 || s.Audio.LastSound != 0 || s.Public.Audio.Music.ID != 26 {
		t.Fatal("expired finite voices survived or looped music stopped")
	}
	bad := &engine.AudioJournal{Serial: s.Public.Audio.Serial + 200, Commands: trace.Commands}
	if err = syncNativeAudio(&s, bad, 80000, nil); err == nil {
		t.Fatal("lost native events accepted")
	}
	result, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual native API request trace through authority timeline, overlapping finite voices, MIDI/CD stop separation, fallback, idempotent journal consumption and expiry; independent bridge isolation, not full story or browser playback", "checks": checks, "assetID": audioAssetID}, "", "  ")
	if err = os.WriteFile("../../../.local/pal/verification/audio-timeline.json", append(result, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
