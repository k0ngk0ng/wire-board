package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func readPropAttackFixture(t *testing.T, path string, value any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err := json.NewDecoder(z).Decode(value); err != nil {
		t.Fatal(err)
	}
}

func TestOriginalPropAttack(t *testing.T) {
	var rows []struct {
		Name                     string
		Source, Target           int8
		Court                    uint8
		AuxiliaryKinds           [2]uint8
		Before, After            CombatState
		Sounds                   []uint8
		UnsupportedDoubleRelease bool
	}
	readPropAttackFixture(t, "testdata/prop-attack-isolated.json.gz", &rows)
	if len(rows) != 1596 {
		t.Fatal("missing original prop responses", len(rows))
	}
	for _, r := range rows {
		ctx := oneCombatCandidate(r.Source, r.Target)
		ctx.CourtID, ctx.AuxiliaryKinds = r.Court, r.AuxiliaryKinds
		got, err := StepOriginalCombat(r.Before, ctx)
		if r.UnsupportedDoubleRelease {
			if err == nil || got.State != r.Before || len(got.Sounds) != 0 {
				t.Fatal("uncovered double-release alias must reject atomically", r.Name, got, err)
			}
			continue
		}
		if err != nil || got.State != r.After || len(got.Sounds) != len(r.Sounds) {
			t.Fatalf("%s before=%+v\ngot=%+v\nwant=%+v\nerror=%v", r.Name, r.Before, got, r.After, err)
		}
		for i, s := range got.Sounds {
			if s.ID != r.Sounds[i] || s.Source != uint8(r.Source) || s.Target != uint8(r.Target) {
				t.Fatal(r.Name, got.Sounds, r.Sounds)
			}
		}
	}
}

func TestPropAttackHTTPRuntimeRegression(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/prop-attack-http-before.json.gz", &s)
	for tick := 0; tick < 24; tick++ {
		encoded, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		var restored RuntimeState
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		got, err := StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatalf("unchanged HTTP seed at video %d: %v", s.VideoFrames, err)
		}
		again, err := StepVideo(restored, [4]Buttons{})
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatal("restore changed contact result", tick, err)
		}
		s = got.State
	}
}

func TestPropAttackInvalidLaterCandidateIsAtomic(t *testing.T) {
	s := CombatState{SpecialHolder: -1}
	s.Auxiliary[0].Action = 13
	ctx := oneCombatCandidate(4, 1)
	ctx.AuxiliaryKinds = [2]uint8{12, 11}
	ctx.CourtID = 1
	ctx.Candidates[5] = 13
	got, err := StepOriginalCombat(s, ctx)
	if err == nil || got.State != s || len(got.Sounds) != 0 {
		t.Fatal(got, err)
	}
}

func TestOriginalPropRebound(t *testing.T) {
	var rows []struct {
		Case, Tick                        int
		Actor                             uint8
		Before, After                     AuxiliaryState
		SpecialHolder, AfterSpecialHolder int8
	}
	readPropAttackFixture(t, "testdata/prop-rebound-isolated.json.gz", &rows)
	if len(rows) != 1688 {
		t.Fatal("missing original rebound frames", len(rows))
	}
	for _, r := range rows {
		got, err := StepAuxiliaryRest(r.Before, AuxiliaryContext{Object: r.Actor, Court: 1, SpecialHolder: r.SpecialHolder})
		holder := r.SpecialHolder
		if got.ClearSpecialHolder {
			holder = -1
		}
		if err != nil || got.State != r.After || holder != r.AfterSpecialHolder || got.ChangeLinkedPose {
			t.Fatalf("case %d tick %d before=%+v\ngot=%+v\nwant=%+v\nholder=%d want=%d err=%v", r.Case, r.Tick, r.Before, got, r.After, holder, r.AfterSpecialHolder, err)
		}
	}
}

func TestOriginalPropReboundGeometry(t *testing.T) {
	for _, kind := range []uint8{11, 12} {
		for frame := uint8(0); frame < 2; frame++ {
			got, err := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: 7, Frame: frame})
			base, baseErr := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: 12, Frame: frame})
			if err != nil || baseErr != nil || got.Shape != base.Shape || got.AttackFlags != 0 || got.ReceiveFlags != 0 {
				t.Fatal(kind, frame, got, base, err, baseErr)
			}
		}
	}
}
