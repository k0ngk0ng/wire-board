package pal

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestRetainedAudioCatalogUsesSavedTimingAndIdentity(t *testing.T) {
	const legacyID = "0000000000000000000000000000000000000000000000000000000000000001"
	legacy := audioCatalog{Sounds: map[int][2]int64{98: {44100, 22050}}, Music: map[int][2]int64{31: {44100, 22050}}, CD: map[int][2]int64{}}
	if _, exists := audioCatalogs[legacyID]; exists {
		t.Fatal("isolated timing fixture collides with a published catalog")
	}
	audioCatalogs[legacyID] = legacy
	t.Cleanup(func() { delete(audioCatalogs, legacyID) })
	s := state{ServerAt: 1100, Audio: &audioMemory{}, Public: View{Audio: &AudioView{AssetID: legacyID, Sounds: []AudioSound{}}}}
	commands := []engine.AudioCommand{{Seq: 1, Kind: 1, ID: 31}, {Seq: 2, Kind: 2, ID: 98}}
	if err := syncNativeAudio(&s, &engine.AudioJournal{Serial: 2, Commands: commands}, 1000, nil); err != nil {
		t.Fatal(err)
	}
	if !validAudio(s) || s.Public.Audio.AssetID != legacyID || s.Public.Audio.Music == nil || len(s.Public.Audio.Sounds) != 1 {
		t.Fatal("retained audio version was rejected or replaced", s.Public.Audio)
	}
	pruneAudio(&s, 2000)
	if s.Public.Audio.Music == nil || len(s.Public.Audio.Sounds) != 1 {
		t.Fatal("saved two-second audio used the current shorter sound duration")
	}
	pruneAudio(&s, 3000)
	if s.Public.Audio.Music != nil || len(s.Public.Audio.Sounds) != 0 || s.Audio.LastSound != 0 {
		t.Fatal("saved two-second audio used the current longer music duration")
	}
	current := state{ServerAt: 1100}
	if err := syncNativeAudio(&current, &engine.AudioJournal{Serial: 2, Commands: commands}, 1000, nil); err != nil || current.Public.Audio.AssetID != audioAssetID {
		t.Fatal("new audio state did not select the current catalog", err)
	}
	unknown := s
	unknown.Public.Audio = cloneAudio(s.Public.Audio)
	unknown.Public.Audio.AssetID = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	before, _ := json.Marshal(unknown)
	if validAudio(unknown) {
		t.Fatal("unknown saved audio version accepted")
	}
	if err := syncNativeAudio(&unknown, &engine.AudioJournal{Serial: 3, Commands: append(commands, engine.AudioCommand{Seq: 3, Kind: 2, ID: 98})}, 1000, nil); err == nil {
		t.Fatal("unknown audio version silently used current timing")
	}
	after, _ := json.Marshal(unknown)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("rejected audio version changed the original snapshot")
	}
}
