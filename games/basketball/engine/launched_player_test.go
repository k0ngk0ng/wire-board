package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

type launchedPlayerFixture struct {
	worldActionFixture
	Prepared WorldActionState
	Actor    uint8
	Variant  string
	Held     [4]Buttons
	Sounds   []uint8
}

func launchedFixtures(t *testing.T) []launchedPlayerFixture {
	t.Helper()
	f, e := os.Open("testdata/launched-player.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows []launchedPlayerFixture
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows) != 64 {
		t.Fatal(len(rows))
	}
	return rows
}
func launchedFixtureContext(s WorldActionState, r launchedPlayerFixture, c WorldActionContext) LaunchedPlayerContext {
	actor := r.Actor
	source := actor
	if s.PlayerLinks[actor]&128 != 0 {
		source = s.PlayerLinks[actor] & 15
	}
	a := s.Actors[actor]
	holder := s.Holder
	releasing := holder >= 0 && holder < 4 && (s.Actors[holder].Player.Action == ActionShoot || s.Actors[holder].Player.Action == ActionAirShoot)
	equipment := c.Enhancements[actor]
	if releasing {
		equipment = c.ShotEquipment[actor]
	}
	holderSpecial := uint8(0)
	if holder >= 0 && holder < 4 {
		holderSpecial = s.Actors[holder].SpecialShot
	}
	return LaunchedPlayerContext{Actor: actor, Attachment: s.PlayerLinks[actor], Source: s.Actors[source].Player, Random: s.Random, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Team: c.Teams[actor/2], Character: c.Characters[actor], EnhancementEnabled: c.EnhancementEnabled, Equipment: equipment, HolderReleasing: releasing, Holder: holder, HolderSpecialShot: holderSpecial, SpecialShot: a.SpecialShot, ShotCycle: a.ShotCycle, HeldButtons: r.Held[actor], HighArc: s.MountFlags[s.SelectedHoop-7]&2 != 0, AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}}
}
func applyLaunchedPreparation(s *WorldActionState, actor uint8, p LaunchedPlayerPreparation) {
	s.Actors[actor].Player = p.Player
	s.PlayerLinks[actor] = p.Attachment
	if p.ClearHolderSpecialShot {
		s.Actors[s.Holder].SpecialShot = 0
	}
	if p.ChangeRandom {
		s.Random = p.Random
	}
	if p.ChangePoseSource {
		s.PoseSource = p.PoseSource
	}
	if p.ChangeSpecialShot {
		s.Actors[actor].SpecialShot = p.SpecialShot
	}
	if p.LaunchPlan != nil {
		s.LaunchTarget = p.LaunchPlan.Target
		s.LaunchArcHeight = p.LaunchPlan.ArcHeight
	}
}
func TestOriginalLaunchedPlayerPrepareAndComplete(t *testing.T) {
	for _, r := range launchedFixtures(t) {
		c := r.context(t)
		ctx := launchedFixtureContext(r.Before, r, c)
		pre, e := PrepareLaunchedPlayer(r.Before.Actors[r.Actor].Player, ctx)
		if e != nil {
			t.Fatal(r.Variant, e)
		}
		result := WorldActionStep{State: r.Before}
		applyLaunchedPreparation(&result.State, r.Actor, pre)
		if result.State != r.Prepared {
			t.Fatalf("%s prepared: %s", r.Variant, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.Prepared), "world"))
		}
		result.SoundIDs = append(result.SoundIDs, pre.SoundIDs...)
		court := NewYorkGroundCourt()
		if c.CourtID == 2 {
			court, e = originalWorldMotionCourt(&result, c, r.Actor)
			if e != nil {
				t.Fatal(e)
			}
		}
		ctx.Attachment = pre.Attachment
		done, e := CompleteLaunchedPlayer(pre.Player, ctx, court)
		if e != nil {
			t.Fatal(r.Variant, e)
		}
		result.State.Actors[r.Actor].Player = done.Player
		result.State.Ball = ApplyBallCommand(result.State.Ball, done.Ball)
		if result.State != r.After {
			t.Fatalf("%s final: %s", r.Variant, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatalf("%s sounds got %v want %v", r.Variant, result.SoundIDs, r.Sounds)
		}
	}
}
