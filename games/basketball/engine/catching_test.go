package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAutomaticCatchesAgainstOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/catches.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario string
		Frame    int
		Before   struct {
			Candidate, Holder, ExcludedCatcher, PreviousHolder, LastTouch, BallAction uint8
			Climbers                                                                  [2]uint8
			HoopOwners                                                                [6]uint8
			Actors                                                                    [4]struct{ Action, Control, Flags, Attachment, Air uint8 }
		}
		After struct {
			Holder, LastTouch, BallAction uint8
			Actions                       [4]uint8
		}
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2392 {
		t.Fatal(len(rows))
	}
	bound := 0
	for _, r := range rows {
		candidate := int8(-1)
		if r.Before.Candidate < 128 {
			candidate = int8(r.Before.Candidate)
		}
		ctx := AutomaticCatchContext{Candidate: candidate, Holder: int8(r.Before.Holder), ExcludedCatcher: int8(r.Before.ExcludedCatcher), PreviousHolder: int8(r.Before.PreviousHolder), LastTouch: int8(r.Before.LastTouch), BallAction: BallAction(r.Before.BallAction & 127)}
		for i, v := range r.Before.Climbers {
			ctx.AuxiliaryOwners[i] = int8(v)
		}
		for i, v := range r.Before.HoopOwners {
			ctx.HoopOwners[i] = int8(v)
		}
		var actions [4]uint8
		for i, p := range r.Before.Actors {
			ctx.Players[i] = CatchPlayer{Action: Action(p.Action & 127), Control: p.Control, Flags: p.Flags, Attached: p.Attachment&128 != 0, Airborne: p.Air&128 != 0}
			actions[i] = p.Action
		}
		got, err := ResolveAutomaticCatch(ctx)
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		if got.ChangePlayer {
			actions[got.Actor] = uint8(got.PlayerAction)
		}
		ball := r.Before.BallAction
		if got.Ball.Change {
			ball = uint8(got.Ball.Action)
		}
		if actions != r.After.Actions || ball != r.After.BallAction || got.Holder != int8(r.After.Holder) || got.LastTouch != int8(r.After.LastTouch) {
			t.Fatalf("%s frame %d: got %+v actions %v ball %d want %+v", r.Scenario, r.Frame, got, actions, ball, r.After)
		}
		if got.Holder != ctx.Holder {
			bound++
		}
	}
	if bound != 20 {
		t.Fatal("unexpected catch bindings", bound)
	}
	t.Log("matched 2392 original catch phases including 20 ownership transfers")
}

func ordinaryCatchContext() AutomaticCatchContext {
	c := AutomaticCatchContext{Candidate: 0, Holder: 6, ExcludedCatcher: -1, PreviousHolder: 6, LastTouch: -1, BallAction: BallPass, AuxiliaryOwners: [2]int8{-1, -1}, HoopOwners: [6]int8{-1, -1, -1, -1, -1, -1}}
	c.Players[0] = CatchPlayer{Action: ActionIdle, Flags: 16}
	return c
}

func TestCatchEligibilityAndActionSelection(t *testing.T) {
	for _, tc := range []struct {
		action Action
		air    bool
		want   Action
		ball   BallAction
	}{{ActionIdle, false, ActionReceiveBall, BallHeld}, {ActionRun, false, ActionDribbleRun, BallDribble}, {ActionJump, true, ActionCatchAir, BallHeld}} {
		c := ordinaryCatchContext()
		c.Players[0].Action = tc.action
		c.Players[0].Airborne = tc.air
		got, err := ResolveAutomaticCatch(c)
		if err != nil || !got.Caught || got.Holder != 0 || got.PlayerAction != tc.want || got.Ball.Action != tc.ball {
			t.Fatal(tc, got, err)
		}
	}
	for _, change := range []func(*AutomaticCatchContext){func(c *AutomaticCatchContext) { c.ExcludedCatcher = 0 }, func(c *AutomaticCatchContext) { c.Players[0].Control = 0xc0 }, func(c *AutomaticCatchContext) { c.Players[0].Action = ActionAttackA }, func(c *AutomaticCatchContext) { c.Players[0].Flags = 0 }, func(c *AutomaticCatchContext) { c.Holder = 1 }, func(c *AutomaticCatchContext) { c.BallAction = BallRest }} {
		c := ordinaryCatchContext()
		change(&c)
		got, err := ResolveAutomaticCatch(c)
		if err != nil || got.Caught || got.ChangePlayer || got.Ball.Change || got.Holder != c.Holder {
			t.Fatal(c, got, err)
		}
	}
}

func TestAttachedAndOccupiedHandCatchRules(t *testing.T) {
	c := ordinaryCatchContext()
	c.Players[0].Attached = true
	got, err := ResolveAutomaticCatch(c)
	if err != nil || !got.Caught || got.PlayerAction != ActionReceiveBall {
		t.Fatal(got, err)
	}
	c.HoopOwners[2] = 0
	got, err = ResolveAutomaticCatch(c)
	if err != nil || got.Caught || got.ChangePlayer {
		t.Fatal("attached hoop owner caught ball", got, err)
	}
	c = ordinaryCatchContext()
	c.Players[0].Action = ActionHoopCarryIdle
	got, err = ResolveAutomaticCatch(c)
	if err != nil || got.Caught || got.Holder != 6 || got.PlayerAction != Action(0x48) || got.Ball.Action != BallKnockback {
		t.Fatal("pass impact became possession", got, err)
	}
	c.BallAction = BallShot
	got, err = ResolveAutomaticCatch(c)
	if err != nil || got.ChangePlayer || got.Ball.Change {
		t.Fatal(got, err)
	}
}
