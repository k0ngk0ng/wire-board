package engine

import "errors"

type HoopRequest struct {
	Action  HoopAction `json:"action"`
	Pending bool       `json:"pending"`
}

type HoopContext struct {
	Index         uint8
	DetachedCount uint8
	MountFlags    [6]uint8
	ContactLatch  uint8
	HasClimber    bool
	Owner         *Player
	OwnerComputer bool
	OwnerSpecial  bool
	HighThrowArc  bool
}

type HoopStep struct {
	Hoop              Hoop
	MountFlags        [6]uint8
	ContactLatch      uint8
	ThrowPlan         *HoopThrowPlan
	ClearOwnerSpecial bool
	// Fixed DEA9 also publishes the actual mount slot to the board stage.
	ChangeSelectedBoard bool
	SelectedBoard       uint8
}

func NewYorkHoops() [6]Hoop {
	var hoops [6]Hoop
	durability := [3]int8{8, 6, 5}
	for i := range hoops {
		x := Fixed(65) * Pixel
		if i%2 == 1 {
			x = 463 * Pixel
		}
		hoops[i] = Hoop{Position: Position{X: x, Depth: 43 * Pixel, Height: Fixed(80+(i/2)*96) * Pixel}, Action: HoopReady, Entered: true, CanReceive: true, Durability: durability[i/2], Animation: AnimationState{HasNextAction: true}, Request: HoopRequest{Pending: true}}
	}
	return hoops
}

// ScoreLock is not a timer. The original clears every hoop's duplicate-score
// lock when any player holds the ball, and retains it throughout free flight.
func ResetHoopScoreLocks(hoops [6]Hoop, ballHeld bool) [6]Hoop {
	if ballHeld {
		for i := range hoops {
			hoops[i].ScoreLock = 0
		}
	}
	return hoops
}

func hoopAnimation(action HoopAction) (AnimationSpec, error) {
	f := func(pose, ticks uint8) AnimationFrame {
		return AnimationFrame{Pose: Pose{Frame: pose & 127, Mirrored: pose&128 != 0}, Ticks: ticks}
	}
	switch action {
	case HoopInactive:
		return AnimationSpec{Frames: []AnimationFrame{f(5, 255)}, End: RepeatForever}, nil
	case HoopFalling, HoopCarried, HoopKnocked:
		return AnimationSpec{Frames: []AnimationFrame{f(0, 255)}, End: RepeatForever}, nil
	case HoopThrown:
		return AnimationSpec{Frames: []AnimationFrame{f(129, 255)}, End: RepeatForever}, nil
	case HoopScored:
		return AnimationSpec{Frames: []AnimationFrame{f(2, 3), f(1, 4), f(129, 3), f(0, 3), f(1, 2)}, End: RepeatRemaining}, nil
	case HoopJostled:
		return AnimationSpec{Frames: []AnimationFrame{f(1, 2), f(129, 2), f(1, 3), f(0, 3), f(129, 4), f(0, 4)}, End: RepeatRemaining}, nil
	default:
		return AnimationSpec{}, errors.New("hoop animation not reconstructed")
	}
}

func animateHoop(h *Hoop) error {
	spec, err := hoopAnimation(h.Action)
	if err != nil {
		return err
	}
	step, err := AdvanceAnimation(spec, h.Animation, h.FacingLeft)
	if err != nil {
		return err
	}
	h.Animation, h.Pose = step.State, step.Pose
	if step.ChangeAction {
		h.Action, h.Entered = HoopAction(step.Action), false
	}
	return nil
}

func advanceLooseHoop(h *Hoop, court FlatCourt) error {
	if court.originalMotion != nil {
		return court.originalMotion.hoop(h)
	}
	p := Player{Body: Body{Position: h.Position, Velocity: h.Velocity, Gravity: h.Gravity}, Airborne: h.Airborne}
	if err := advanceOnFlatCourt(&p, court); err != nil {
		return err
	}
	h.Position, h.Velocity, h.Gravity, h.Airborne = p.Body.Position, p.Body.Velocity, p.Body.Gravity, p.Airborne
	return nil
}

// StepHoopAction restores the observed basket actions. Carrying/throwing takes
// the holder's current world state as input, without replacing that player or
// its original AI. Unobserved lob/linked-body contexts remain explicit errors.
func StepHoopAction(original Hoop, ctx HoopContext, court FlatCourt) (HoopStep, error) {
	result := HoopStep{Hoop: original, MountFlags: ctx.MountFlags, ContactLatch: ctx.ContactLatch}
	fail := func(err error) (HoopStep, error) {
		return HoopStep{Hoop: original, MountFlags: ctx.MountFlags, ContactLatch: ctx.ContactLatch}, err
	}
	if ctx.Index >= 6 || original.Action > HoopKnocked || original.Action == HoopLobbed {
		return fail(errors.New("hoop action/attachment not reconstructed"))
	}
	if (original.Action == HoopCarried || original.Action == HoopThrown && !original.Entered) && ctx.Owner == nil {
		return fail(errors.New("carried/thrown hoop has no owner state"))
	}
	h := &result.Hoop
	action := h.Action
	if !h.Entered {
		h.Entered = true
		h.Animation.Cursor, h.Animation.Elapsed, h.Animation.Loops = 0, 0, 0
		switch action {
		case HoopInactive:
			h.Velocity = Velocity{}
			h.Gravity = 0
			h.CanReceive = false
			h.Carried = false
			h.CarrierSeat = 0
		case HoopReady:
			h.Velocity = Velocity{}
			h.Gravity = 0
			h.CanReceive = true
		case HoopScored:
			result.ContactLatch = 0
			h.Animation.NextAction, h.Animation.HasNextAction = Action(HoopReady), true
			if h.Durability >= 0 {
				h.Durability--
				if h.Durability < 0 && ctx.DetachedCount < 4 {
					h.Action, h.Entered = HoopFalling, false
				}
			}
		case HoopJostled:
			h.Animation.NextAction, h.Animation.HasNextAction = Action(HoopReady), true
			if h.Durability < 0 && ctx.DetachedCount < 4 {
				h.Animation.NextAction = Action(HoopFalling)
			}
		case HoopFalling:
			h.Velocity.Height, h.Gravity = 3008, 288
			h.Velocity.X = -2 * Pixel
			if h.Position.X>>16 == 0 {
				h.Velocity.X = 2 * Pixel
			}
			row := uint8(h.Position.Height>>8) >> 4
			slots := [8]uint8{4, 4, 0, 0, 0, 2, 2, 2}
			masks := [16]uint8{2, 2, 1, 1, 2, 2, 1, 1, 3, 3, 2, 2, 1, 1, 3, 3}
			slot := int(slots[row/2]) + int(h.Position.X>>16)
			if slot < 0 || slot >= 6 {
				return fail(errors.New("falling hoop mount outside court"))
			}
			result.MountFlags[slot] &= masks[row]
			result.ChangeSelectedBoard, result.SelectedBoard = true, uint8(slot)
		case HoopThrown:
			h.FacingLeft = ctx.Owner.FacingLeft
			// Release starts at the previous carried position. Reattaching to
			// the owner's newly moved pose here would shift the launch point.
			plan, clear := chooseHoopThrow(*ctx.Owner, ctx.OwnerComputer, ctx.OwnerSpecial, ctx.MountFlags, ctx.HighThrowArc)
			body, err := court.launchPass(h.Position, plan.Target, plan.ArcHeight)
			if err != nil {
				return fail(err)
			}
			h.Position, h.Velocity, h.Gravity = body.Position, body.Velocity, body.Gravity
			h.CanReceive, h.Carried, h.CarrierSeat = false, false, 0
			result.ThrowPlan = &plan
			result.ClearOwnerSpecial = clear
		case HoopKnocked:
			h.Velocity.Height = signed16(h.Velocity.Height + 3264)
			h.Gravity = signed16(h.Gravity + 480)
			h.CanReceive, h.Carried, h.CarrierSeat = false, false, 0
		}
	}
	if action == HoopReady {
		if !(ctx.HasClimber && h.Pose.Frame == 2 && !h.Pose.Mirrored) {
			h.Pose = Pose{}
		}
		return result, nil
	}
	if err := animateHoop(h); err != nil {
		return fail(err)
	}
	switch action {
	case HoopCarried:
		// The original draws with the previous facing, then updates facing
		// and follows the owner's current pose before integrating motion.
		h.FacingLeft = ctx.Owner.FacingLeft
		if err := positionCarriedHoop(h, *ctx.Owner); err != nil {
			return fail(err)
		}
		if err := advanceLooseHoop(h, court); err != nil {
			return fail(err)
		}
	case HoopJostled:
		delta := Pixel
		if h.Animation.Cursor&1 != 0 {
			delta = -Pixel
		}
		h.Position.Height = signed24(h.Position.Height + delta)
	case HoopFalling, HoopInactive, HoopThrown, HoopKnocked:
		if err := advanceLooseHoop(h, court); err != nil {
			return fail(err)
		}
		if action != HoopInactive && !h.Airborne {
			h.Action, h.Entered = HoopInactive, false
		}
	}
	return result, nil
}
