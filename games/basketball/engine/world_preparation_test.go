package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type worldPreparationFixture struct {
	Frame         int
	Before, After WorldPreparationState
	SoundIDs      []uint8
	Context       struct {
		Actions                     json.RawMessage
		Kinds                       [7]uint8
		Suppressed                  [7]bool
		AttackGrades                [4]uint8
		HumanTactics, HumanCommands [2]uint8
		HumanStatus                 [2]int8
		BackboardStrength           uint8
		ScoringBonuses              [4]bool
	}
}

func worldPreparationFixtures(t *testing.T) []worldPreparationFixture {
	t.Helper()
	f, err := os.Open("testdata/world-preparation.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []worldPreparationFixture
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1997 {
		t.Fatal("missing complete preparation phases", len(rows))
	}
	return rows
}

func (r worldPreparationFixture) context(t *testing.T) WorldPreparationContext {
	t.Helper()
	var action worldActionFixture
	if err := json.Unmarshal(r.Context.Actions, &action.Context); err != nil {
		t.Fatal(err)
	}
	c := WorldPreparationContext{Actions: action.context(t), Kinds: r.Context.Kinds, Suppressed: r.Context.Suppressed, AttackGrades: r.Context.AttackGrades, HumanTactics: r.Context.HumanTactics, HumanCommands: r.Context.HumanCommands, HumanStatus: r.Context.HumanStatus, BackboardStrength: r.Context.BackboardStrength}
	for i := range c.Actions.Profiles {
		c.Actions.Profiles[i].ScoringBonus = r.Context.ScoringBonuses[i]
	}
	return c
}

func TestWorldPreparationEveryOriginalCompleteStage(t *testing.T) {
	rows := worldPreparationFixtures(t)
	sounds := 0
	for i, r := range rows {
		got, err := StepWorldPreparation(r.Before, r.context(t))
		if err != nil {
			t.Fatalf("record %d frame %d: %v", i, r.Frame, err)
		}
		if got.State != r.After {
			a, b := reflect.ValueOf(got.State), reflect.ValueOf(r.After)
			for n := 0; n < a.NumField(); n++ {
				if !reflect.DeepEqual(a.Field(n).Interface(), b.Field(n).Interface()) {
					t.Errorf("record %d frame %d field %s\ngot %+v\nwant %+v", i, r.Frame, a.Type().Field(n).Name, a.Field(n).Interface(), b.Field(n).Interface())
				}
			}
			t.FailNow()
		}
		if len(got.SoundIDs) != len(r.SoundIDs) {
			t.Fatalf("frame %d sounds %v want %v", r.Frame, got.SoundIDs, r.SoundIDs)
		}
		for j, v := range got.SoundIDs {
			if v != r.SoundIDs[j] {
				t.Fatalf("frame %d sound order %v want %v", r.Frame, got.SoundIDs, r.SoundIDs)
			}
		}
		sounds += len(got.SoundIDs)
	}
	t.Logf("matched all %d independent original preparation stages and %d ordered sound requests; not a single-initial-state full-game replay", len(rows), sounds)
}

func TestWorldPreparationErrorsAreAtomic(t *testing.T) {
	r := worldPreparationFixtures(t)[1]
	for _, change := range []func(*WorldPreparationContext){
		func(c *WorldPreparationContext) { c.Actions.CourtID = 6 },
		func(c *WorldPreparationContext) { c.Kinds[0] = 255 },
		func(c *WorldPreparationContext) { c.HumanCommands[0] = 1 },
	} {
		c := r.context(t)
		change(&c)
		got, err := StepWorldPreparation(r.Before, c)
		if err == nil || got.State != r.Before || len(got.SoundIDs) != 0 {
			t.Fatal("failed preparation left partial state or sound", got, err)
		}
	}
}

func TestWorldPreparationStateRoundTrip(t *testing.T) {
	r := worldPreparationFixtures(t)[300]
	data, err := json.Marshal(r.Before)
	if err != nil {
		t.Fatal(err)
	}
	var restored WorldPreparationState
	if err = json.Unmarshal(data, &restored); err != nil || restored != r.Before {
		t.Fatal("state round trip", err)
	}
	a, err := StepWorldPreparation(r.Before, r.context(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := StepWorldPreparation(restored, r.context(t))
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("restoration changed phase", err)
	}
}

func TestWorldPreparationBoardAdapterPreservesOwnerIdentity(t *testing.T) {
	// The board stage never assigns hoop owners. Even an opaque owner byte
	// must not be normalized through the component's lossy boolean view.
	s := WorldPreparationState{Actions: WorldActionState{Holder: 6}}
	for i := range s.Actions.Hoops {
		s.Actions.Hoops[i].Owner = uint8(i + 4)
	}
	before := s.Actions.hoopOwners()
	_, err := prepareOriginalBoards(&s, WorldPreparationContext{Actions: WorldActionContext{CourtID: 1}})
	if err != nil || s.Actions.hoopOwners() != before {
		t.Fatal("board adapter invented owner assignment", s.Actions.hoopOwners(), before, err)
	}
}
