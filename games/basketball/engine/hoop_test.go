package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedHoop struct {
	observedBall
	Link, Cooldown, Durability, Detached, ContactLatch, FirstClimber, SecondClimber                                  int32
	MountFlags                                                                                                       [6]uint8
	OwnerAction, OwnerPose, OwnerFacing, OwnerX, OwnerDepth, OwnerHeight, OwnerComputer, OwnerSpecial, SelectedFlags int32
}

func hoopFromObservation(r observedHoop) Hoop {
	b := ballFromObservation(r.observedBall)
	h := Hoop{Position: b.Body.Position, Velocity: b.Body.Velocity, Gravity: b.Body.Gravity, Action: HoopAction(r.Action & 127), Entered: r.Action&128 != 0, Pose: b.Pose, FacingLeft: b.FacingLeft, Animation: b.Animation, Airborne: b.Airborne, Request: HoopRequest{Action: HoopAction(r.Request & 127), Pending: r.Request&128 == 0}, CanReceive: r.Link&15 >= 7 && r.Link&15 != 15, ScoreLock: uint8(r.Cooldown), Durability: int8(r.Durability), Carried: r.Link < 4}
	if h.Carried {
		h.CarrierSeat = uint8(r.Link)
	}
	return h
}

func TestHoopActionsAgainstOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/hoop.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame, Actor  int
		Before, After observedHoop
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12702 {
		t.Fatal("unexpected original hoop call count", len(rows))
	}
	count := 0
	for _, r := range rows {
		ctx := HoopContext{Index: uint8(r.Actor - 7), DetachedCount: uint8(r.Before.Detached), MountFlags: r.Before.MountFlags, ContactLatch: uint8(r.Before.ContactLatch), HasClimber: int(r.Before.FirstClimber) == r.Actor || int(r.Before.SecondClimber) == r.Actor}
		if r.Before.Link < 4 {
			ctx.Owner = &Player{Action: Action(r.Before.OwnerAction & 127), Pose: Pose{Frame: uint8(r.Before.OwnerPose & 127), Mirrored: r.Before.OwnerPose&128 != 0}, FacingLeft: r.Before.OwnerFacing&128 != 0, Body: Body{Position: Position{X: Fixed(r.Before.OwnerX), Depth: Fixed(r.Before.OwnerDepth), Height: Fixed(r.Before.OwnerHeight)}}}
			ctx.OwnerComputer = r.Before.OwnerComputer&128 != 0
			ctx.OwnerSpecial = r.Before.OwnerSpecial != 0
			ctx.HighThrowArc = r.Before.SelectedFlags&2 != 0
		}
		got, err := StepHoopAction(hoopFromObservation(r.Before), ctx, NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("%s frame %d hoop %d: %v", r.Scenario, r.Frame, r.Actor, err)
		}
		want := hoopFromObservation(r.After)
		if got.Hoop != want || got.MountFlags != r.After.MountFlags || got.ContactLatch != uint8(r.After.ContactLatch) {
			t.Fatalf("%s frame %d hoop %d\nbefore %+v\ngot %+v\nwant %+v flags %v latch %d", r.Scenario, r.Frame, r.Actor, hoopFromObservation(r.Before), got, want, r.After.MountFlags, r.After.ContactLatch)
		}
		if r.Before.Action == 6 {
			wantPlan := HoopThrowPlan{Target: Position{X: Fixed(r.After.TargetX) * Pixel, Depth: Fixed(r.After.TargetDepth) * Pixel, Height: Fixed(r.After.TargetHeight) * Pixel}, ArcHeight: Fixed(r.After.ArcHeight) * Pixel}
			if got.ThrowPlan == nil || *got.ThrowPlan != wantPlan {
				t.Fatal("throw target differs from original", r.Frame, got.ThrowPlan, wantPlan)
			}
		}
		count++
	}
	if count != 12702 {
		t.Fatal("unexpected supported hoop observation count", count)
	}
	t.Logf("matched all %d original hoop calls, including carried, thrown and knocked hoops", count)
}

func TestCarriedHoopRequiresOwner(t *testing.T) {
	h := Hoop{Action: HoopCarried, Carried: true, CarrierSeat: 2}
	got, err := StepHoopAction(h, HoopContext{}, NewYorkGroundCourt())
	if err == nil || got.Hoop != h {
		t.Fatal(got, err)
	}
}

func TestHoopScoreLocksAgainstOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/hoop-locks.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Holder        uint8
		Before, After [6]uint8
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2116 {
		t.Fatal("unexpected lock call count", len(rows))
	}
	for i, r := range rows {
		var hoops [6]Hoop
		for n, lock := range r.Before {
			hoops[n].ScoreLock = lock
		}
		got := ResetHoopScoreLocks(hoops, r.Holder != 6)
		for n, h := range got {
			if h.ScoreLock != r.After[n] {
				t.Fatal(i, r, got)
			}
		}
	}
	t.Logf("matched %d original possession-based score-lock updates", len(rows))
}

func TestHoopDurabilityChangesOnlyOnActionEntry(t *testing.T) {
	h := Hoop{Action: HoopScored, CanReceive: true, Durability: 1, Position: Position{X: 65 * Pixel, Depth: 43 * Pixel, Height: 80 * Pixel}}
	ctx := HoopContext{}
	one, err := StepHoopAction(h, ctx, NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	two, err := StepHoopAction(one.Hoop, ctx, NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	if one.Hoop.Durability != 0 || two.Hoop.Durability != 0 {
		t.Fatal(one, two)
	}
	h.Durability = 0
	ctx.DetachedCount = 4
	protected, err := StepHoopAction(h, ctx, NewYorkGroundCourt())
	if err != nil || protected.Hoop.Durability != -1 || protected.Hoop.Action == HoopFalling {
		t.Fatal(protected, err)
	}
}

func TestInitialNewYorkHoopsAgainstOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/hoop.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Actor  int
		Before observedHoop
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	got := NewYorkHoops()
	for i := 0; i < 6; i++ {
		if rows[i].Actor != i+7 || got[i] != hoopFromObservation(rows[i].Before) {
			t.Fatal(i, got[i], hoopFromObservation(rows[i].Before))
		}
	}
}

func TestContinuousHoopGoalRecovery(t *testing.T) {
	data, err := os.ReadFile("testdata/hoop.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame, Actor  int
		Before, After observedHoop
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	var h Hoop
	started := false
	count := 0
	for _, r := range rows {
		if r.Scenario != "shot" || r.Actor != 8 {
			continue
		}
		if !started {
			if r.Before.Action != 2 {
				continue
			}
			h = hoopFromObservation(r.Before)
			started = true
		}
		// Ball possession and climbers belong to other world actors. Hoop
		// position, durability and animation are never reset during recovery.
		var all [6]Hoop
		all[1] = h
		h = ResetHoopScoreLocks(all, r.Before.Holder != 6)[1]
		ctx := HoopContext{Index: 1, DetachedCount: uint8(r.Before.Detached), MountFlags: r.Before.MountFlags, ContactLatch: uint8(r.Before.ContactLatch), HasClimber: r.Before.FirstClimber == 8 || r.Before.SecondClimber == 8}
		step, err := StepHoopAction(h, ctx, NewYorkGroundCourt())
		if err != nil {
			t.Fatal(err)
		}
		h = step.Hoop
		if h != hoopFromObservation(r.After) {
			t.Fatalf("frame %d got %+v want %+v", r.Frame, h, hoopFromObservation(r.After))
		}
		count++
		if h.Action == HoopReady && h.Entered {
			break
		}
	}
	if count != 16 {
		t.Fatal("unexpected goal-recovery length", count)
	}
	t.Log("16 continuous original hoop updates from goal entry through ready")
}
