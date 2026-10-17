package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalSpecialBallContactOracles(t *testing.T) {
	for _, corpus := range []struct {
		path    string
		count   int
		natural bool
	}{{"testdata/special-contact-isolated.json", 570, false}, {"testdata/special-contact-natural.json", 6, true}} {
		var rows []struct {
			Name, Scenario string
			Frame          int
			Before, After  InteractionState
			Context        InteractionContext
			Sounds         []uint8
		}
		raw, err := os.ReadFile(corpus.path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != corpus.count {
			t.Fatal("missing original contact cases", corpus.path, len(rows))
		}
		hits, reflections, catches := 0, 0, 0
		for _, r := range rows {
			got, err := StepOriginalBallContact(r.Before, r.Context)
			if err != nil || got.State != r.After || len(got.Sounds) != len(r.Sounds) {
				t.Fatalf("%s %s/%s frame%d\nbefore %+v\nctx %+v\ngot %+v\nwant %+v sounds %v error %v", corpus.path, r.Name, r.Scenario, r.Frame, r.Before, r.Context, got, r.After, r.Sounds, err)
			}
			for i, sound := range got.Sounds {
				if sound.ID != r.Sounds[i] || sound.Source != 6 || sound.Target != uint8(r.Context.Candidates[6]) {
					t.Fatal("wrong special contact sound", sound, r)
				}
			}
			if len(r.Sounds) > 0 {
				hits++
			}
			if r.Before.SelectedHoop != r.After.SelectedHoop {
				reflections++
			}
			if r.Before.Holder != r.After.Holder {
				catches++
			}
		}
		if corpus.natural {
			if hits != 5 || catches != 1 {
				t.Fatal(hits, catches)
			}
		} else if hits == 0 || reflections == 0 || catches == 0 {
			t.Fatal("isolated branch coverage missing", hits, reflections, catches)
		}
		t.Logf("%s natural=%v matched %d cases: hits%d reflections%d catches%d", corpus.path, corpus.natural, len(rows), hits, reflections, catches)
	}
}

func TestSpecialBallContactFailureIsAtomic(t *testing.T) {
	s := InteractionState{Holder: 6, SelectedHoop: 8, Ball: Ball{Action: 17, Animation: AnimationState{Loops: 9}}}
	s.Players[0].Endurance = 44
	ctx := InteractionContext{CourtID: 255, Candidates: [7]int8{-1, -1, -1, -1, -1, -1, 0}}
	got, err := StepOriginalBallContact(s, ctx)
	if err == nil || got.State != s || len(got.Sounds) != 0 {
		t.Fatal(got, err)
	}
	ctx.CourtID = 1
	// Auxiliary candidates 4/5 now have their original response. Keep the
	// rollback test on an actual invalid object instead of rejecting them.
	ctx.Candidates[6] = 13
	got, err = StepOriginalBallContact(s, ctx)
	if err == nil || got.State != s {
		t.Fatal("invalid ball-contact candidate silently accepted", got, err)
	}
}
