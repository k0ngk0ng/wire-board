package score_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func instrumentBytes(t *testing.T) []byte {
	t.Helper()
	f, err := os.Open("../../data/audio-score-instrument-work.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func instrumentCatalog(t *testing.T) (*score.Catalog, *score.Catalog) {
	t.Helper()
	legacy := catalog(t)
	bound, err := score.BindInstrumentWork(legacy, bytes.NewReader(instrumentBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	return legacy, bound
}

func TestInstrumentWorkFullBindingAndCanonicalTone(t *testing.T) {
	legacy, bound := instrumentCatalog(t)
	var document struct {
		Profiles  []score.InstrumentWork
		Selectors []struct {
			Key struct {
				ScoreID uint8 `json:"score_id"`
				VoiceID int   `json:"voice_id"`
				Section string
				Index   int `json:"event_index"`
			}
			Reference string
		}
		Fallback string `json:"fallback_reference"`
	}
	if err := json.Unmarshal(instrumentBytes(t), &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Profiles) != 106 || len(document.Selectors) != 960 {
		t.Fatal("resource closure differs")
	}
	for _, work := range document.Profiles {
		got, ok := bound.Instrument(work.ID)
		if !ok || got != work {
			t.Fatal("immutable instrument profile differs")
		}
		if _, ok = legacy.Instrument(work.ID); ok {
			t.Fatal("binding mutated original catalog")
		}
	}
	if _, ok := bound.Instrument("missing"); ok {
		t.Fatal("unknown instrument invented")
	}
	for _, sid := range []uint8{0x8d, 0x95} {
		d, err := score.New(bound)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = d.Advance(score.Input{SoundID: sid}); err != nil {
			t.Fatal(err)
		}
		for _, v := range d.Snapshot().Voices {
			if v.Owned && len(v.Channel) >= 2 && v.Channel[:2] == "fm" {
				work, ok := bound.Instrument(v.Tone)
				if !ok || work.ID == work.PatchID {
					t.Fatal("Tone is not the single semantic instrument reference", v.Tone)
				}
			}
		}
		state := d.Snapshot()
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var saved score.State
		if err = json.Unmarshal(raw, &saved); err != nil {
			t.Fatal(err)
		}
		resumed, err := score.Restore(bound, saved)
		if err != nil || !reflect.DeepEqual(state, resumed.Snapshot()) {
			t.Fatal("bound snapshot lost choice", err)
		}
		if _, err = score.Restore(legacy, state); err == nil {
			t.Fatal("bound references restored into a legacy resource set")
		}
		old, err := score.New(legacy)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = old.Advance(score.Input{SoundID: sid}); err != nil {
			t.Fatal(err)
		}
		if _, err = score.Restore(bound, old.Snapshot()); err == nil {
			t.Fatal("legacy tone silently resolved to a timed variant")
		}
	}
	aliases := map[string]map[uint8]bool{}
	for _, work := range document.Profiles {
		if aliases[work.PatchID] == nil {
			aliases[work.PatchID] = map[uint8]bool{}
		}
		aliases[work.PatchID][work.RecordsSkipped] = true
	}
	if !aliases["patch-ff7d20a3582bd2f75b87"][8] || !aliases["patch-ff7d20a3582bd2f75b87"][13] || !aliases["patch-4c6a6d97d872d62c7799"][8] || !aliases["patch-4c6a6d97d872d62c7799"][14] {
		t.Fatal("known same-sound/different-work choices merged")
	}
}

func TestInstrumentWorkBoundCatalogOriginalTrajectories(t *testing.T) {
	_, bound := instrumentCatalog(t)
	for _, name := range []string{"isolated-8d", "isolated-95", "higher-priority-replaces", "music-replaces-primary", "primary-before-first-music", "music82-hidden-patch"} {
		t.Run(name, func(t *testing.T) {
			frames, intents := replayOriginal(t, filepath.Join("testdata", name+".jsonl.gz"), bound)
			if frames == 0 || intents == 0 {
				t.Fatal("empty original replay")
			}
			t.Logf("%d original frames %d intents with semantic instrument refs", frames, intents)
		})
	}
}

func TestInstrumentWorkRejectsIncompleteMismatchedAndMutableMetadata(t *testing.T) {
	legacy := catalog(t)
	raw := instrumentBytes(t)
	var original map[string]any
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct {
		name  string
		apply func(map[string]any)
	}{
		{"catalog", func(d map[string]any) { d["catalog_json_sha256"] = "other" }},
		{"missing-selector", func(d map[string]any) { v := d["selectors"].([]any); d["selectors"] = v[:len(v)-1] }},
		{"duplicate-selector", func(d map[string]any) { v := d["selectors"].([]any); v[1] = v[0] }},
		{"missing-profile", func(d map[string]any) { d["profiles"] = d["profiles"].([]any)[1:] }},
		{"work-identity", func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["records_skipped"] = 255 }},
		{"fallback", func(d map[string]any) { d["fallback_reference"] = "missing" }},
		{"source-index", func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["source_index"] = 0 }},
	} {
		t.Run(change.name, func(t *testing.T) {
			var d map[string]any
			if err := json.Unmarshal(raw, &d); err != nil {
				t.Fatal(err)
			}
			change.apply(d)
			payload, _ := json.Marshal(d)
			got, err := score.BindInstrumentWork(legacy, bytes.NewReader(payload))
			if err == nil || got != nil {
				t.Fatal("invalid binding returned partial catalog")
			}
		})
	}
	bound, err := score.BindInstrumentWork(legacy, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 'x'
	var document struct{ Profiles []score.InstrumentWork }
	clean := instrumentBytes(t)
	if err = json.Unmarshal(clean, &document); err != nil {
		t.Fatal(err)
	}
	want := document.Profiles[0]
	got, ok := bound.Instrument(want.ID)
	if !ok || got != want {
		t.Fatal("binding retained mutable input")
	}
	got.PatchID = "changed"
	again, _ := bound.Instrument(want.ID)
	if again != want {
		t.Fatal("lookup exposes mutable profile")
	}
}
