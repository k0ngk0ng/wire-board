package engine

// Before implementation: Kind10 action3 captures only a whole-height-zero
// player without any of six active incoming attachments. Fractional height,
// entered bit, attachment bit6, prop targets, fallback impulse, no capture
// sound/damage, the fixed 056C value4 (even emitter5), missing context and later
// phase rollback must agree with the original function. Four real runtime
// failures must advance and remain deterministic after JSON restore.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestKind10OriginalContactCaptureAndFallback(t *testing.T) {
	var rows []struct {
		Source, Target       int8
		Court                uint8
		AuxiliaryKinds       [2]uint8
		Attachments          [4]uint8
		Before, After        CombatState
		Sounds               []uint8
		ChangeCaptureCounter bool  `json:"changeCaptureObject"`
		CaptureCounter       uint8 `json:"captureObject"`
	}
	readPropAttackFixture(t, "testdata/kind10-contact.json.gz", &rows)
	if len(rows) != 1162 {
		t.Fatal("source matrix incomplete", len(rows))
	}
	captures := 0
	for i, row := range rows {
		c := oneCombatCandidate(row.Source, row.Target)
		c.CourtID = row.Court
		c.AuxiliaryKinds = row.AuxiliaryKinds
		c.PlayerAttachments = &row.Attachments
		got, e := StepOriginalCombat(row.Before, c)
		if e != nil || got.State != row.After || len(got.Sounds) != len(row.Sounds) || got.ChangeCaptureCounter != row.ChangeCaptureCounter {
			t.Fatalf("case%d got%+v want%+v error%v", i, got, row, e)
		}
		for j, sound := range got.Sounds {
			if sound.ID != row.Sounds[j] {
				t.Fatal(i, got.Sounds, row.Sounds)
			}
		}
		if row.ChangeCaptureCounter {
			captures++
			if got.CaptureCounter != row.CaptureCounter || got.CaptureCounter != 4 {
				t.Fatal("capture reference differs", i, got)
			}
		}
	}
	if captures != 32 {
		t.Fatal("capture branch coverage", captures)
	}
}

func TestKind10MissingCaptureContextAndRollback(t *testing.T) {
	s := CombatState{SpecialHolder: -1}
	s.Auxiliary[0].Action = 3
	c := oneCombatCandidate(4, 0)
	c.AuxiliaryKinds = [2]uint8{10, 11}
	got, e := StepOriginalCombat(s, c)
	if e == nil || got.State != s || got.ChangeCaptureCounter || len(got.Sounds) != 0 {
		t.Fatal("unknown incoming links guessed", got, e)
	}
	c.PlayerAttachments = &[4]uint8{}
	c.Candidates[5] = 13
	got, e = StepOriginalCombat(s, c)
	if e == nil || got.State != s || got.ChangeCaptureCounter || len(got.Sounds) != 0 {
		t.Fatal("later failure leaked capture", got, e)
	}
}

func TestKind10RecordedRuntimeFailures(t *testing.T) {
	var rows []struct {
		Name, Failure string
		State         RuntimeState
	}
	readPropAttackFixture(t, "testdata/kind10-runtime-failures.json.gz", &rows)
	if len(rows) != 4 {
		t.Fatal("missing real failures", len(rows))
	}
	for _, row := range rows {
		t.Run(row.Name, func(t *testing.T) {
			s := row.State
			for tick := 0; tick < 24; tick++ {
				raw, e := json.Marshal(s)
				if e != nil {
					t.Fatal(e)
				}
				var restored RuntimeState
				if e = json.Unmarshal(raw, &restored); e != nil {
					t.Fatal(e)
				}
				got, e := StepVideo(s, [4]Buttons{})
				if e != nil {
					t.Fatal(tick, e)
				}
				again, e := StepVideo(restored, [4]Buttons{})
				if e != nil || !reflect.DeepEqual(got, again) {
					t.Fatal("restore changed first-difference continuation", tick, e)
				}
				s = got.State
			}
		})
	}
}
