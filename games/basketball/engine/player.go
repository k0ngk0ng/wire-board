package engine

import (
	"errors"
	"fmt"
)

type Heading uint8

const (
	HeadingUp Heading = iota
	HeadingUpRight
	HeadingRight
	HeadingDownRight
	HeadingDown
	HeadingDownLeft
	HeadingLeft
	HeadingUpLeft
	HeadingNone
)

type ActionRequest struct {
	Action  Action `json:"action"`
	Pending bool   `json:"pending"`
}

type Player struct {
	Body           Body           `json:"body"`
	Action         Action         `json:"action"`
	Entered        bool           `json:"entered"`
	Request        ActionRequest  `json:"request"`
	Heading        Heading        `json:"heading"`
	FacingLeft     bool           `json:"facingLeft"`
	Pose           Pose           `json:"pose"`
	Animation      AnimationState `json:"animation"`
	Airborne       bool           `json:"airborne"`
	NewYorkPassage uint8          `json:"newYorkPassage,omitempty"`
}

type CharacterProfile struct {
	ID           string
	ShotSkill    uint8
	ScoringBonus bool
	JumpVelocity Fixed
	Gravity      Fixed
	RunSpeed     Fixed
	BrakeRight   Fixed
	BrakeLeft    Fixed
	BrakeDepth   Fixed
	Walk         [9]Velocity
	JumpTravel   [9]Velocity
	Animations   map[Action]AnimationSpec
	RunCharge    AnimationSpec
}

// FlatCourt retains the original New York component API. A transaction-local
// originalMotion binding routes enabled additional courts through the complete
// original motion pipeline; those courts never inherit New York geometry.
type FlatCourt struct {
	passLaunchContext                            func() PassLaunchContext
	originalMotion                               *courtMotionBinding
	motionSurface                                *motionSurfaceRecorder
	MinX, MaxX                                   Fixed
	LeftReset, RightReset                        Fixed
	MinDepth, MaxDepth                           Fixed
	Floor                                        Fixed
	RaisedDepth, RaisedLeftEnd, RaisedRightStart Fixed
	RaisedFloor                                  Fixed
	NewYorkPassages                              bool
}

func NewYorkGroundCourt() FlatCourt {
	return FlatCourt{MinX: 55 * Pixel, MaxX: 472 * Pixel, LeftReset: 56 * Pixel, RightReset: 471 * Pixel, MinDepth: 16 * Pixel, MaxDepth: 88 * Pixel, Floor: 0, RaisedDepth: 72 * Pixel, RaisedLeftEnd: 136 * Pixel, RaisedRightStart: 392 * Pixel, RaisedFloor: 64 * Pixel, NewYorkPassages: true}
}

func KunioProfile() CharacterProfile {
	frame := func(pose, ticks uint8) AnimationFrame { return AnimationFrame{Pose: Pose{Frame: pose}, Ticks: ticks} }
	return CharacterProfile{
		ID: "kunio", ShotSkill: 6, JumpVelocity: 2176, Gravity: 256,
		RunSpeed: 1344, BrakeRight: 832, BrakeLeft: 544, BrakeDepth: 192,
		RunCharge:  AnimationSpec{Frames: []AnimationFrame{frame(32, 2), frame(35, 2)}, End: RepeatForever},
		Walk:       [9]Velocity{{Depth: 512}, {X: 832, Depth: 448}, {X: 960}, {X: 832, Depth: -448}, {Depth: -512}, {X: -832, Depth: -448}, {X: -960}, {X: -832, Depth: 448}, {}},
		JumpTravel: [9]Velocity{{Depth: 576}, {X: 896, Depth: 512}, {X: 1024}, {X: 896, Depth: -768}, {Depth: -576}, {X: -896, Depth: -768}, {X: -1024}, {X: -896, Depth: 512}, {}},
		Animations: map[Action]AnimationSpec{
			ActionIdle:               {Frames: []AnimationFrame{frame(PoseStanding, 255)}, End: RepeatForever},
			ActionWalk:               {Frames: []AnimationFrame{frame(PoseWalkFirst, 3), frame(PoseStanding, 2), frame(PoseWalkSecond, 3), frame(PoseStanding, 2)}, End: RepeatForever},
			ActionRun:                {Frames: []AnimationFrame{frame(PoseRunning, 2), frame(PoseStanding, 2)}, End: RepeatForever},
			ActionBrake:              {Frames: []AnimationFrame{frame(PoseStanding, 255)}, End: RepeatForever},
			ActionJump:               {Frames: []AnimationFrame{frame(PoseCrouched, 2), frame(PoseAirborne, 255), frame(PoseAirborne, 255)}, End: RepeatForever},
			ActionRunningJump:        {Frames: []AnimationFrame{frame(PoseCrouched, 2), frame(PoseAirborne, 255), frame(PoseAirborne, 255)}, End: RepeatForever},
			ActionLand:               {Frames: []AnimationFrame{frame(PoseCrouched, 4)}, End: RepeatRemaining},
			ActionAttackA:            {Frames: []AnimationFrame{frame(PoseAttackAWindup, 2), frame(PoseAttackAExtended, 4)}, End: RepeatRemaining},
			ActionAttackB:            {Frames: []AnimationFrame{frame(PoseAttackBFirst, 1), frame(PoseAttackBSecond, 1), frame(PoseAttackBThird, 1)}, End: RepeatUntilCount, RepeatLimit: 2},
			ActionDodge:              {Frames: []AnimationFrame{{Pose: Pose{Frame: 50}, Ticks: 2}, {Pose: Pose{Frame: 7, Mirrored: true}, Ticks: 2}}, End: RepeatRemaining},
			ActionDribbleIdle:        {Frames: []AnimationFrame{frame(6, 2), frame(7, 2)}, End: RepeatForever},
			ActionDribbleWalk:        {Frames: []AnimationFrame{frame(8, 2), frame(7, 2), frame(9, 2), frame(7, 2)}, End: RepeatForever},
			ActionDribbleRun:         {Frames: []AnimationFrame{frame(10, 2), frame(7, 2)}, End: RepeatForever},
			ActionDribbleBrake:       {Frames: []AnimationFrame{frame(6, 255)}, End: RepeatForever},
			ActionCarrierJump:        {Frames: []AnimationFrame{frame(PoseCrouched, 2), frame(PoseAirborne, 255), frame(PoseAirborne, 255)}, End: RepeatForever},
			ActionCarrierRunningJump: {Frames: []AnimationFrame{frame(PoseCrouched, 2), frame(PoseAirborne, 255), frame(PoseAirborne, 255)}, End: RepeatForever},
			ActionCarrierLand:        {Frames: []AnimationFrame{frame(PoseCrouched, 4)}, End: RepeatRemaining},
			ActionPass:               {Frames: []AnimationFrame{frame(43, 4), frame(20, 1), frame(20, 6)}, End: RepeatRemaining},
			ActionAirPass:            {Frames: []AnimationFrame{frame(43, 4), frame(20, 1), frame(20, 255), frame(20, 255)}, End: RepeatForever},
			ActionAimShot:            {Frames: []AnimationFrame{frame(11, 4)}, End: RepeatRemaining},
			ActionAimAirShot:         {Frames: []AnimationFrame{frame(11, 255)}, End: RepeatForever},
			ActionShoot:              {Frames: []AnimationFrame{frame(12, 8)}, End: RepeatRemaining},
			ActionAirShoot:           {Frames: []AnimationFrame{frame(12, 5), frame(PoseAirborne, 255)}, End: RepeatForever},
			ActionCatchBall:          {Frames: []AnimationFrame{frame(13, 5)}, End: RepeatRemaining},
			ActionReceiveBall:        {Frames: []AnimationFrame{frame(13, 5)}, End: RepeatRemaining},
			ActionCatchAir:           {Frames: []AnimationFrame{frame(PoseAirborne, 255), frame(PoseAirborne, 255)}, End: RepeatForever},
			ActionPickUpHoop:         {Frames: []AnimationFrame{frame(PoseCrouched, 4)}, End: RepeatRemaining},
			ActionHoopCarryIdle:      {Frames: []AnimationFrame{frame(PoseStanding, 255)}, End: RepeatForever},
			ActionHoopCarryRun:       {Frames: []AnimationFrame{frame(PoseRunning, 2), frame(PoseStanding, 2)}, End: RepeatForever},
			ActionHoopAim:            {Frames: []AnimationFrame{frame(11, 4)}, End: RepeatRemaining},
			ActionHoopThrow:          {Frames: []AnimationFrame{frame(12, 8)}, End: RepeatRemaining},
		},
	}
}

func freeAction(action Action) bool {
	switch action {
	case ActionIdle, ActionWalk, ActionRun, ActionBrake, ActionJump, ActionRunningJump, ActionLand, ActionFalling, ActionAttackA, ActionAttackB:
		return true
	}
	return false
}

// These are the identified free-player transitions in the original legality
// table. Other actions are explicitly unsupported, not redirected to idle.
func freeTransition(from, to Action) bool {
	switch to {
	case ActionIdle:
		return from == ActionWalk
	case ActionWalk:
		return from == ActionIdle
	case ActionRun:
		return from == ActionIdle || from == ActionWalk || from == ActionBrake
	case ActionBrake:
		return from == ActionRun
	case ActionJump:
		return from == ActionIdle || from == ActionWalk || from == ActionBrake || from == ActionJump || from == ActionRunningJump
	case ActionRunningJump:
		return from == ActionRun
	case ActionAttackA, ActionAttackB:
		return from == ActionIdle || from == ActionWalk || from == ActionRun || from == ActionBrake
	}
	return false
}

func initializeAction(p *Player) {
	p.Entered = true
	p.Animation.Cursor, p.Animation.Elapsed, p.Animation.Loops = 0, 0, 0
}

func faceHeading(p *Player) {
	switch p.Heading {
	case HeadingUpRight, HeadingRight, HeadingDownRight:
		p.FacingLeft = false
	case HeadingDownLeft, HeadingLeft, HeadingUpLeft:
		p.FacingLeft = true
	}
}

func animatePlayer(p *Player, profile CharacterProfile, action Action) error {
	spec, ok := profile.Animations[action]
	if !ok {
		return fmt.Errorf("animation for action %d has not been reconstructed", action)
	}
	step, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if err != nil {
		return err
	}
	p.Animation, p.Pose = step.State, step.Pose
	if step.ChangeAction {
		p.Action, p.Entered = step.Action, false
	}
	return nil
}

func stopVertical(p *Player)   { p.Body.Velocity.Height = 0; p.Body.Gravity = 0 }
func stopHorizontal(p *Player) { p.Body.Velocity.X = 0; p.Body.Velocity.Depth = 0 }

func advanceOnFlatCourt(p *Player, court FlatCourt) error {
	if court.originalMotion != nil {
		_, err := court.originalMotion.player(p, nil)
		return err
	}
	court.recordMotionSurface(p.Body.Position)
	previous := p.Body.Position
	p.Body = IntegrateMotion(p.Body)
	b := &p.Body
	if b.Position.X < court.MinX {
		b.Position.X = court.LeftReset + (b.Position.X & 255)
		stopHorizontal(p)
	}
	if b.Position.X >= court.MaxX {
		b.Position.X = court.RightReset + (b.Position.X & 255)
		stopHorizontal(p)
	}
	if b.Position.Depth < court.MinDepth {
		b.Position.Depth = court.MinDepth + (b.Position.Depth & 255)
	}
	if b.Position.Depth >= court.MaxDepth {
		b.Position.Depth = court.MaxDepth - Pixel + (b.Position.Depth & 255)
	}
	if court.RaisedDepth > 0 && b.Position.Depth >= court.RaisedDepth && (b.Position.X < court.RaisedLeftEnd || b.Position.X >= court.RaisedRightStart) {
		return errors.New("raised New York terrain has not been reconstructed")
	}
	if b.Position.Height < court.Floor && b.Velocity.Height < 0 {
		// The original landing correction restores the prior floor point
		// ($883D..$8853) before the landing action begins on the next update.
		b.Position.X, b.Position.Depth = previous.X, previous.Depth
		b.Position.Height = court.Floor
		stopVertical(p)
		p.Airborne = false
	} else if b.Position.Height>>8 == court.Floor>>8 {
		// Ground contact compares whole pixels. The fractional remainder is
		// cleared before the next descending update performs full landing.
		if b.Velocity.Height < Pixel {
			b.Position.Height = court.Floor
		}
		if b.Velocity.Height == 0 && b.Gravity == 0 {
			p.Airborne = false
		}
	} else if b.Position.Height > court.Floor {
		p.Airborne = true
	}
	return nil
}

// StepFreePlayer restores one original world action phase for an unattached,
// unpossessed player on flat ground. It is a component of the future world step,
// not a stand-in for ball possession, combat, platforms or computer strategy.
func StepFreePlayer(original Player, profile CharacterProfile, court FlatCourt) (Player, error) {
	return stepFreePlayer(original, profile, court, FreePlayerContext{})
}

func StepComputerFreePlayer(original Player, profile CharacterProfile, court FlatCourt) (Player, error) {
	return stepFreePlayer(original, profile, court, FreePlayerContext{Computer: true})
}

// StepFreePlayerAction executes one original E165/E168 actor call. Revisit
// reports the jump handler clearing the processed flag; the world scheduler
// decides when the actor runs again.
func StepFreePlayerAction(original Player, ctx FreePlayerContext, profile CharacterProfile, court FlatCourt) (FreePlayerStep, error) {
	initial := FreePlayerStep{Player: original, ShotCycle: ctx.ShotCycle}
	p := original
	shotCycle := ctx.ShotCycle
	var sounds []uint8
	computer := ctx.Computer
	if p.Heading > HeadingNone || !freeAction(p.Action) || p.Request.Pending && !freeAction(p.Request.Action) && p.Request.Action != ActionCatchBall {
		return initial, errors.New("player action/context has not been reconstructed")
	}
	if p.Request.Pending && freeTransition(p.Action, p.Request.Action) {
		p.Action, p.Entered = p.Request.Action, false
		p.Request.Pending = false
	}
	action := p.Action
	if action == ActionFalling {
		next, err := stepFreeFalling(p, ctx, court)
		if err != nil {
			return initial, err
		}
		return FreePlayerStep{Player: next, ShotCycle: shotCycle}, nil
	}
	enteredNow := !p.Entered
	if enteredNow {
		initializeAction(&p)
		if (action == ActionJump || action == ActionRunningJump) && ctx.JumpPhase < 0 {
			p.Animation.Cursor = 1
			launchFreeJump(&p, action, profile, ctx.JumpPhase)
			return FreePlayerStep{Player: p, ShotCycle: shotCycle, Revisit: true, SoundIDs: []uint8{23}}, nil
		}
		switch action {
		case ActionIdle:
			stopHorizontal(&p)
			if !p.Airborne {
				stopVertical(&p)
			}
		case ActionLand:
			stopHorizontal(&p)
			stopVertical(&p)
			p.Animation.NextAction = ActionIdle
			p.Animation.HasNextAction = true
		case ActionRun:
			shotCycle = 8
			switch p.Heading {
			case HeadingUpRight, HeadingRight, HeadingDownRight:
				p.Body.Velocity.X = profile.RunSpeed
			case HeadingDownLeft, HeadingLeft, HeadingUpLeft:
				p.Body.Velocity.X = -profile.RunSpeed
			}
		case ActionAttackA, ActionAttackB:
			p.Body.Velocity.Depth = 0
			p.Animation.NextAction = ActionIdle
			p.Animation.HasNextAction = true
			if action == ActionAttackB {
				faceHeading(&p)
			}
		}
	}
	if action == ActionWalk && !computer || action == ActionRun || action == ActionJump || action == ActionRunningJump {
		faceHeading(&p)
	}
	if action == ActionRun && computer && p.Heading != HeadingNone {
		setRunHeading(&p, profile.RunSpeed)
	}
	if action == ActionRun && ctx.HasBall && !computer && p.Animation.Cursor == 0 && p.Animation.Elapsed == 0 {
		shotCycle--
		if shotCycle == 0 {
			p.Animation.Cursor = 3
		} else if shotCycle < 0 {
			shotCycle = -1
		}
	}
	var animationErr error
	if action == ActionRun && p.Animation.Cursor >= 3 {
		animationErr = animateFreeRunCharge(&p)
	} else {
		animationErr = animatePlayer(&p, profile, action)
	}
	if animationErr != nil {
		return initial, animationErr
	}
	if action == ActionWalk {
		v := profile.Walk[p.Heading]
		p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
	}
	if action == ActionRun {
		p.Body.Velocity.Depth = 0
		switch p.Heading {
		case HeadingUp, HeadingUpRight, HeadingUpLeft:
			p.Body.Velocity.Depth = 512
		case HeadingDown, HeadingDownRight, HeadingDownLeft:
			p.Body.Velocity.Depth = -512
		}
	}
	if action == ActionBrake {
		if p.Body.Velocity.X > 0 {
			p.Body.Velocity.X = max(0, p.Body.Velocity.X-profile.BrakeRight)
		}
		if p.Body.Velocity.X < 0 {
			p.Body.Velocity.X = min(0, p.Body.Velocity.X+profile.BrakeLeft)
		}
		if p.Body.Velocity.Depth > 0 {
			p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth - profile.BrakeDepth)
		} else if p.Body.Velocity.Depth < 0 {
			p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth + profile.BrakeDepth)
		}
	}
	if action == ActionAttackA || action == ActionAttackB {
		if p.Body.Velocity.X > 0 {
			p.Body.Velocity.X = max(0, p.Body.Velocity.X-128)
		}
		if p.Body.Velocity.X < 0 {
			p.Body.Velocity.X = min(0, p.Body.Velocity.X+128)
		}
	}
	ball, err := advancePlayerOnCourt(&p, playerTerrainContext{Computer: computer, HasBall: ctx.HasBall}, court)
	if err != nil {
		return initial, err
	}
	if (action == ActionIdle || action == ActionWalk || action == ActionRun) && p.Airborne {
		p.Action, p.Entered = ActionFalling, false
	}
	if action == ActionBrake && p.Body.Velocity.X == 0 {
		p.Action, p.Entered = ActionIdle, false
	}
	if (action == ActionJump || action == ActionRunningJump) && p.Animation.Cursor != 0 {
		if p.Animation.Elapsed == 0 {
			sounds = append(sounds, 23)
			launchFreeJump(&p, action, profile, ctx.JumpPhase)
		}
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
		}
	}
	return FreePlayerStep{Player: p, Ball: ball, ShotCycle: shotCycle, SoundIDs: sounds, Revisit: enteredNow && (action == ActionJump || action == ActionRunningJump)}, nil
}
