package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestOriginalAuxiliaryCarrierMovementInstructions(t *testing.T) {
	f, err := os.Open("testdata/auxiliary-movement.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Before, After                              WorldActionState
		Computer, EnhancementEnabled               bool
		Team, RunGrade, JumpGrade, JumpEnhancement uint8
		Sounds                                     []uint8
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4672 {
		t.Fatal("missing original movement calls", len(rows))
	}
	for i, r := range rows {
		profile := KunioProfile()
		profile.RunSpeed, _ = RunSpeedForGrade(r.RunGrade)
		profile.JumpVelocity, _ = JumpVelocityForGrade(r.JumpGrade)
		support := RiderSupport{Body: r.Before.Actors[1].Player.Body, Pose: r.Before.Actors[1].Player.Pose, Kind: 0}
		ctx := AuxiliaryCarrierContext{Actor: 0, Computer: r.Computer, HasBall: r.Before.Holder == 0, Owners: [2]int8{r.Before.Auxiliary[0].Owner, r.Before.Auxiliary[1].Owner}, Attachment: r.Before.PlayerLinks[0], Support: &support, CarriedPlayer: &r.Before.CarriedPlayer, EnhancementEnabled: r.EnhancementEnabled, Team: r.Team, JumpEnhancement: &r.JumpEnhancement}
		if (r.Before.Actors[0].Player.Action == 0x24 || r.Before.Actors[0].Player.Action == 0x25) && !r.Before.Actors[0].Player.Entered && r.Before.PlayerLinks[0]&128 != 0 {
			world := WorldActionStep{State: r.Before}
			binding, e := originalWorldMotionCourt(&world, WorldActionContext{CourtID: 1, Teams: [2]uint8{r.Team, 1}}, 0)
			if e != nil {
				t.Fatal(e)
			}
			bound, e := StepAuxiliaryCarrier(r.Before.Actors[0].Player, ctx, profile, binding)
			if e != nil || bound.Player != r.After.Actors[0].Player {
				t.Fatalf("row%d jump detached after motion instead of before: %s err%v", i, firstWorldLoopDifference(reflect.ValueOf(bound.Player), reflect.ValueOf(r.After.Actors[0].Player), "player"), e)
			}
		}
		got, err := StepAuxiliaryCarrier(r.Before.Actors[0].Player, ctx, profile, NewYorkGroundCourt())
		ball := ApplyBallCommand(r.Before.Ball, got.Ball)
		aim := r.Before.Actors[0].AimWindow
		if got.ChangeAimWindow {
			aim = got.AimWindow
		}
		cycle := r.Before.Actors[0].ShotCycle
		if got.ChangeShotCycle {
			cycle = got.ShotCycle
		}
		special := r.Before.Actors[0].SpecialShot
		if got.ClearSpecialShot {
			special = 0
		}
		link := r.Before.PlayerLinks[0]
		if got.Revisit {
			link = 0
		}
		aux := r.Before.Auxiliary
		if got.ChangeAuxiliary {
			aux[got.Object-4].Action, aux[got.Object-4].Entered = got.Action, false
		}
		if err != nil || ball != r.After.Ball || got.Player != r.After.Actors[0].Player || aim != r.After.Actors[0].AimWindow || cycle != r.After.Actors[0].ShotCycle || special != r.After.Actors[0].SpecialShot || link != r.After.PlayerLinks[0] || !slices.Equal(got.SoundIDs, r.Sounds) || aux != r.After.Auxiliary {
			t.Fatalf("row%d action%x entered%v heading%d computer%v: %s cycle%d/%d special%d/%d link%d/%d sounds%v/%v err%v", i, r.Before.Actors[0].Player.Action, r.Before.Actors[0].Player.Entered, r.Before.Actors[0].Player.Heading, r.Computer, firstWorldLoopDifference(reflect.ValueOf(got.Player), reflect.ValueOf(r.After.Actors[0].Player), "player"), cycle, r.After.Actors[0].ShotCycle, special, r.After.Actors[0].SpecialShot, link, r.After.PlayerLinks[0], got.SoundIDs, r.Sounds, err)
		}
	}
	t.Logf("matched %d original group20 movement/held falling/attached throw calls", len(rows))
}
