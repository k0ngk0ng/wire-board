package engine

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

// Failure inventory was fixed before implementation in SHOES27_SCORE_VERIFICATION.md.
func TestShoes27OpeningProfilesReachOriginalRimConsumer(t *testing.T) {
	var rows []struct {
		Shooter, Char, Item, Mode, Team, Height, Side int
		Contact                                       string
		Before, After                                 []byte
	}
	readPropAttackFixture(t, "testdata/shoes27-score.json.gz", &rows)
	if len(rows) != 6912 {
		t.Fatal("incomplete original shoes27 rim calls", len(rows))
	}
	base := coldOpeningForTest(t)
	for i, r := range rows {
		s := base
		s.Transition.Mode = uint8(r.Mode)
		for actor := 0; actor < 4; actor++ {
			s.Transition.Characters[actor] = r.Before[0x3e3+actor]
		}
		s.Transition.Teams = [2]uint8{r.Before[0x586], r.Before[0x587]}
		for char := 0; char < 4; char++ {
			for slot := 0; slot < 3; slot++ {
				s.Equipment[char][slot] = int8(r.Before[0x6e4+3*char+slot])
			}
		}
		_, c, e := WorldFromOriginalOpening(s, OpeningWorldOptions{})
		if e != nil {
			t.Fatal(i, e)
		}
		b, w := boardContactRAM(r.Before), boardContactRAM(r.After)
		ctx := boardContactContextRAM(r.Before).Rim
		ctx.PreviousBallAction = b.PreviousBallAction
		ctx.ScoringBonus = c.Actions.Profiles[r.Shooter].ScoringBonus
		got, e := ResolveRimContact(RimState{Ball: b.Ball, Hoops: b.Hoops, Random: b.Random, Scores: b.Scores, ScoreSoundDelay: b.ScoreSoundDelay}, ctx)
		want := RimState{Ball: w.Ball, Hoops: w.Hoops, Random: w.Random, Scores: w.Scores, ScoreSoundDelay: w.ScoreSoundDelay}
		if e != nil || got.State != want {
			t.Fatalf("call%d shooter%d char%d item%d mode%d team%d height%d side%d %s: got%+v want%+v err%v", i, r.Shooter, r.Char, r.Item, r.Mode, r.Team, r.Height, r.Side, r.Contact, got.State, want, e)
		}
	}
}

func TestShoes27OwnedCampaignOpeningAndRestore(t *testing.T) {
	p, e := campaign.New(0, 0)
	if e != nil {
		t.Fatal(e)
	}
	// Both victories over opponent6 restore the original reward pair27/3.
	p.Period, p.WinBits = 4, [2]uint8{32, 32}
	code, e := campaign.EncodePassword(p)
	if e != nil {
		t.Fatal(e)
	}
	p, e = campaign.DecodePassword(code)
	if e != nil {
		t.Fatal(e)
	}
	index := -1
	for i, v := range p.Inventory {
		if v == 27 {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("password did not restore original reward27", code, p.Inventory)
	}
	p, e = campaign.Equip(p, 3, uint8(index))
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewCampaignRuntime(p, CampaignMatchSelection{Opponent: 1, Characters: [4]uint8{3, 0, 3, 0}, HumanTactics: [2]uint8{4, 4}, Seed: 0x12345678})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2000 && s.Playing == nil; i++ {
		step, err := StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatal(err)
		}
		s = step.State
	}
	if s.Playing == nil {
		t.Fatal("equipped campaign did not finish opening")
	}
	var restored RuntimeState
	b, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &restored); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRuntimeState(restored); e != nil {
		t.Fatal("restore rejected", e)
	}
	for actor := 0; actor < 4; actor++ {
		if got := restored.Playing.Context.Actions.Profiles[actor].ScoringBonus; got != (actor == 0) {
			t.Fatal("owned shoe did not follow eligible actor/character", actor, got)
		}
		var bad RuntimeState
		if e = json.Unmarshal(b, &bad); e != nil {
			t.Fatal(e)
		}
		bad.Playing.Context.Actions.Profiles[actor].ScoringBonus = !bad.Playing.Context.Actions.Profiles[actor].ScoringBonus
		if e = ValidateRuntimeState(bad); e == nil {
			t.Fatal("accepted tampered scoring bonus", actor)
		}
	}
	t.Logf("original password%s restored owned27, equipped character3, completed opening and JSON validation", code)
	// Normal versus configuration carries no equipment; abnormal source-mode128
	// fixture loadouts above do not expose an equipment selector to versus users.
	v, e := NewRuntime(DefaultRuntimeConfig())
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2000 && v.Playing == nil; i++ {
		step, err := StepVideo(v, [4]Buttons{})
		if err != nil {
			t.Fatal(err)
		}
		v = step.State
	}
	if v.Playing == nil {
		t.Fatal("versus opening missing")
	}
	for i, p := range v.Playing.Context.Actions.Profiles {
		if p.ScoringBonus {
			t.Fatal(fmt.Sprint("empty versus bonus", i))
		}
	}
}
