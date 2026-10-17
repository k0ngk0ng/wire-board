package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedAttack struct {
	observedPlayer
	Control, RunGrade, Variant, Enhancement uint8
	EnhancementEnabled                      bool
	Actions, Owners, Facings                [13]uint8
	Attachments                             [4]uint8
}

func TestOriginalPlayerAttackTrace(t *testing.T) {
	var rows []struct {
		Frame         int
		Actor         uint8
		Before, After observedAttack
	}
	raw, err := os.ReadFile("testdata/player-attack-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 499 {
		t.Fatal("missing original attack calls", len(rows))
	}
	counts := map[Action]int{}
	for _, r := range rows {
		b, a := r.Before, r.After
		s := AttackState{Player: playerFromObservation(b.observedPlayer), AttackVariant: b.Variant, JumpPhase: int8(b.Boost)}
		ctx := AttackContext{Team: uint8(b.Team), Character: uint8(b.Character), Computer: b.Control&128 != 0, Attachment: uint8(b.Attachment), HasBall: b.Holder == int32(r.Actor), EnhancementEnabled: b.EnhancementEnabled, Enhancement: b.Enhancement}
		profile := KunioProfile()
		profile.RunSpeed = 4*Pixel + Fixed(b.RunGrade)*Pixel/4
		got, err := StepPlayerAttack(s, ctx, profile, NewYorkGroundCourt())
		want := AttackState{Player: playerFromObservation(a.observedPlayer), AttackVariant: a.Variant, JumpPhase: int8(a.Boost)}
		if err != nil || got.State != want || got.Revisit != (a.Attachment&64 == 0) {
			t.Fatalf("frame %d actor %d before %+v\ngot %+v\nwant %+v err=%v", r.Frame, r.Actor, b, got, want, err)
		}
		if b.Owners != a.Owners || got.Ball.Change || got.Ball.ChangeRequest {
			t.Fatal("unexpected object mutation", r.Frame, got)
		}
		for i := 0; i < 13; i++ {
			if i != int(r.Actor) && (b.Actions[i] != a.Actions[i] || b.Facings[i] != a.Facings[i]) {
				t.Fatal("unrepresented other-object action", r.Frame, i)
			}
		}
		counts[s.Player.Action]++
	}
	t.Logf("matched all %d original attack calls: %v", len(rows), counts)
}
func TestPlayerAttackDistinctProfilesAndUnknownHandlers(t *testing.T) {
	p := Player{Action: ActionAttackA, Heading: HeadingNone, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}, Animation: AnimationState{HasNextAction: true}}
	kunio, err := StepPlayerAttack(AttackState{Player: p}, AttackContext{Team: 0, Character: 0}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || kunio.State.AttackVariant != 0 {
		t.Fatal(kunio, err)
	}
	rick, err := StepPlayerAttack(AttackState{Player: p}, AttackContext{Team: 0, Character: 1}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || rick.State.AttackVariant != 3 || rick.State.Player.Animation.Elapsed == kunio.State.Player.Animation.Elapsed {
		t.Fatal(rick, err)
	}
	p.Action = 0x69
	s := AttackState{Player: p}
	got, err := StepPlayerAttack(s, AttackContext{}, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got.State != s {
		t.Fatal("unknown action accepted", got, err)
	}
}
