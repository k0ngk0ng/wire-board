package engine

import "fmt"

type WorldActionActor struct {
	Player        Player      `json:"player"`
	ShotCycle     int8        `json:"shotCycle"`
	AimWindow     uint8       `json:"aimWindow"`
	JumpPhase     int8        `json:"jumpPhase"`
	Dodge         DodgeMemory `json:"dodge"`
	Endurance     uint8       `json:"endurance"`
	LinkedActor   int8        `json:"linkedActor"`
	SpecialShot   uint8       `json:"specialShot"`
	AttackVariant uint8       `json:"attackVariant"`
}

// Owner preserves the numbered/raised mount or player reference. Component
// booleans are derived for each call, never stored as a second ownership truth.
type WorldActionHoop struct {
	Position   Position
	Velocity   Velocity
	Gravity    Fixed
	Action     HoopAction
	Entered    bool
	Owner      uint8
	ScoreLock  uint8
	Durability int8
	Pose       Pose
	FacingLeft bool
	Animation  AnimationState
	Airborne   bool
	Request    HoopRequest
}

func (h WorldActionHoop) component() Hoop {
	result := Hoop{Position: h.Position, Velocity: h.Velocity, Gravity: h.Gravity, Action: h.Action, Entered: h.Entered, ScoreLock: h.ScoreLock, Durability: h.Durability, Pose: h.Pose, FacingLeft: h.FacingLeft, Animation: h.Animation, Airborne: h.Airborne, Request: h.Request, Carried: h.Owner < 4, CanReceive: h.Owner&15 >= 7 && h.Owner&15 != 15}
	if result.Carried {
		result.CarrierSeat = h.Owner
	}
	return result
}

func (h *WorldActionHoop) applyComponent(next Hoop, index uint8) {
	owner := h.Owner
	if next.Carried {
		owner = next.CarrierSeat
	} else if !next.CanReceive {
		owner = 15
	} else if owner&15 < 7 || owner&15 == 15 {
		owner = index + 7
	}
	*h = WorldActionHoop{Position: next.Position, Velocity: next.Velocity, Gravity: next.Gravity, Action: next.Action, Entered: next.Entered, Owner: owner, ScoreLock: next.ScoreLock, Durability: next.Durability, Pose: next.Pose, FacingLeft: next.FacingLeft, Animation: next.Animation, Airborne: next.Airborne, Request: next.Request}
}

// Fixed $DFBD identifies the actual mount by height and court half. The
// raised flag is part of ownership; a relocated hoop need not use its own ID.
func originalHoopMountOwner(position Position) uint8 {
	low := uint8(position.Height >> 8)
	owner := uint8(7)
	if uint8(position.Height>>16) != 0 {
		owner = 11
		if low >= 32 {
			owner = 139
		}
	} else if low >= 192 {
		owner = 137
	} else if low >= 144 {
		owner = 9
	} else if low >= 96 {
		owner = 135
	}
	return owner + uint8(position.X>>16)
}

// WorldActionState owns all state written by the action phase. Auxiliary
// attachment flags live only in Auxiliary; ObjectLink on auxiliary 5 is also
// the authoritative assisted-actor reference ($0570), never a second field.
type WorldActionState struct {
	// CaptureCounter is the original box escape countdown ($056C).
	// Preserve the first development snapshot key for existing replay evidence.
	CaptureCounter uint8               `json:"captureObject,omitempty"`
	Actors         [4]WorldActionActor `json:"actors"`
	Auxiliary      [2]AuxiliaryState   `json:"auxiliary"`
	Ball           Ball                `json:"ball"`
	Hoops          [6]WorldActionHoop  `json:"hoops"`
	PlayerLinks    [4]uint8            `json:"playerLinks"`
	BallLink       uint8               `json:"ballLink"`
	// Original $055A: special target flag when the ball owns itself.
	UnownedSpecialShot uint8 `json:"unownedSpecialShot,omitempty"`
	// PoseSource is the original shared follow-position source ($056A).
	// It persists when no handler writes it; it is not the current ball owner.
	PoseSource                                        uint8 `json:"poseSource"`
	Holder, ExcludedCatcher, LastTouch, SpecialHolder int8
	Random                                            RandomState
	MountFlags                                        [6]uint8
	DetachedCount, ContactLatch                       uint8
	SelectedHoop                                      int8
	LaunchTarget                                      Position
	LaunchArcHeight                                   Fixed
	BallLanding                                       Position
	ShotOriginX                                       Fixed
	LastShooter                                       int8
	DunkTargetObject                                  uint8
	AuxiliaryRelease                                  bool
	CarriedPlayer                                     int8
}

type WorldActionContext struct {
	// Owner FF's source is the last generated background column, before actions.
	BackgroundLaunchHeight *Fixed `json:"-"`
	// Original third equipment slot ($06E6 + 3*character), explicitly sourced.
	JumpEnhancements *[4]uint8 `json:"-"`
	// Transient original $0634..$0637 marks after input; nil means unavailable.
	PickupFlags *[4]uint8 `json:"-"`
	// Original 04E0..04E3 actor-indexed input, refreshed for each action phase.
	ActorHeldButtons [4]Buttons
	// Original 04E4..04E7 actor-indexed edges. Nil means not provided.
	ActorPressedButtons                  *[4]Buttons `json:"-"`
	CameraShake                          uint8
	CourtID                              uint8
	Profiles                             [4]CharacterProfile
	Controls, Characters, RecoveryGrades [4]uint8
	Teams                                [2]uint8
	Equipment                            uint8
	CachedZones                          [4]CourtZone
	ShotEquipment                        [4]uint8
	BallShotInputButtons                 Buttons
	EnhancementEnabled                   bool
	Enhancements                         [4]uint8
	// Original arc lookup also permits object indices 0..6. These bytes
	// precede the mutable six hoop mount flags, and are not duplicate mounts.
	OtherObjectArcFlags [7]uint8
}

type WorldActionStep struct {
	MotionSurfaceWrites []MotionSurfaceWrite
	State               WorldActionState
	Calls               []uint8
	SoundIDs            []uint8
	// Preparation owns the persistent selector; preserve action-stage writes
	// as a command so the continuous driver can commit the last write.
	ChangeSelectedBoard bool
	SelectedBoard       uint8
	ChangeCameraShake   bool
	CameraShake         uint8
}

func (s WorldActionState) actionLinks() [7]uint8 {
	return [7]uint8{s.PlayerLinks[0], s.PlayerLinks[1], s.PlayerLinks[2], s.PlayerLinks[3], s.Auxiliary[0].Attachment, s.Auxiliary[1].Attachment, s.BallLink}
}
func (s *WorldActionState) setActionLinks(links [7]uint8) {
	copy(s.PlayerLinks[:], links[:4])
	s.Auxiliary[0].Attachment, s.Auxiliary[1].Attachment, s.BallLink = links[4], links[5], links[6]
}
func (s WorldActionState) hoopOwners() [6]int8 {
	var owners [6]int8
	for i, h := range s.Hoops {
		owners[i] = int8(h.Owner)
	}
	return owners
}
func (s *WorldActionState) setObjectPose(object uint8, pose Pose) error {
	switch {
	case object < 4:
		s.Actors[object].Player.Pose = pose
	case object < 6:
		s.Auxiliary[object-4].Pose = pose
	case object == 6:
		s.Ball.Pose = pose
	case object < 13:
		s.Hoops[object-7].Pose = pose
	default:
		return fmt.Errorf("invalid pose object %d", object)
	}
	return nil
}
func (s *WorldActionState) applyHurtObject(c HurtObjectCommand) error {
	if c.Object >= 13 {
		return fmt.Errorf("invalid hurt object %d", c.Object)
	}
	if c.ClearAttachment {
		if c.Object >= 7 {
			return fmt.Errorf("hoop attachment clear is not reconstructed")
		}
		links := s.actionLinks()
		links[c.Object] = 0
		s.setActionLinks(links)
	}
	if c.SetAction {
		switch {
		case c.Object < 4:
			s.Actors[c.Object].Player.Action, s.Actors[c.Object].Player.Entered = Action(c.Action), false
		case c.Object < 6:
			s.Auxiliary[c.Object-4].Action, s.Auxiliary[c.Object-4].Entered = c.Action, false
		case c.Object == 6:
			s.Ball.Action, s.Ball.Entered = BallAction(c.Action), false
		default:
			s.Hoops[c.Object-7].Action, s.Hoops[c.Object-7].Entered = HoopAction(c.Action), false
		}
	}
	if c.SetFacing {
		switch {
		case c.Object < 4:
			s.Actors[c.Object].Player.FacingLeft = c.FacingLeft
		case c.Object < 6:
			s.Auxiliary[c.Object-4].FacingLeft = c.FacingLeft
		case c.Object == 6:
			s.Ball.FacingLeft = c.FacingLeft
		default:
			s.Hoops[c.Object-7].FacingLeft = c.FacingLeft
		}
	}
	if c.SetOwner {
		switch {
		case c.Object >= 4 && c.Object < 6:
			s.Auxiliary[c.Object-4].Owner = c.Owner
		case c.Object == 6:
			s.Holder = c.Owner
		case c.Object >= 7:
			h := &s.Hoops[c.Object-7]
			if c.Owner >= 4 {
				return fmt.Errorf("hurt hoop owner %d not reconstructed", c.Owner)
			}
			h.Owner = 15
			if c.Owner >= 0 {
				h.Owner = uint8(c.Owner)
			}
		default:
			return fmt.Errorf("cannot assign ownership to player %d", c.Object)
		}
	}
	return nil
}

// Action entry sounds precede the motion hook; preserve that original order
// when a court device also emits a request during the same action call.
func appendPlayerActionSounds(result *WorldActionStep, start int, sounds []uint8) {
	if len(sounds) == 0 {
		return
	}
	motion := append([]uint8(nil), result.SoundIDs[start:]...)
	result.SoundIDs = append(result.SoundIDs[:start], sounds...)
	result.SoundIDs = append(result.SoundIDs, motion...)
}

func advanceWorldPlayer(result *WorldActionStep, actor uint8, ctx WorldActionContext, court FlatCourt) error {
	motionSoundStart := len(result.SoundIDs)
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	resolved, err := ResolvePlayerRequest(*p)
	if err != nil {
		return err
	}
	*p = resolved
	computer := ctx.Controls[actor]&128 != 0
	profile := ctx.Profiles[actor]
	if s.PlayerLinks[actor]&128 != 0 && p.Action != 0x58 && p.Action != 0x68 && p.Action != ActionRiding && p.Action != ActionReceiveBall && p.Action != ActionLaunched && !capturedAction(p.Action) && !worldAuxiliaryPlayerAction(*s, actor) {
		return fmt.Errorf("attached player action is not reconstructed")
	}
	heldHoop := -1
	for i, h := range s.Hoops {
		if h.Owner == actor {
			heldHoop = i
			break
		}
	}
	carrierContext := CarrierContext{HasBall: s.Holder == int8(actor), Computer: computer, BallAction: s.Ball.Action, Teammate: s.Actors[actor^1].Player.Body.Position, SkipJumpWindup: a.JumpPhase < 0}
	carrierContext.ShotRelease = ShotReleaseContext{Team: ctx.Teams[actor/2], Character: ctx.Characters[actor], Zone: ctx.CachedZones[actor], EnhancementEnabled: ctx.EnhancementEnabled, Equipment: ctx.ShotEquipment[actor], Random: s.Random}
	carrierContext.CarryingPlayer = s.CarriedPlayer == int8(actor)
	if s.PlayerLinks[actor]&128 != 0 && p.Action == ActionReceiveBall {
		parent := s.PlayerLinks[actor] & 15
		if parent >= 4 {
			return fmt.Errorf("attached receiver support %d requires its original kind", parent)
		}
		support := s.Actors[parent].Player
		carrierContext.AttachedSupport = &RiderSupport{Body: support.Body, Pose: support.Pose, Kind: ctx.Teams[parent/2]}
	}
	carrierContext.DunkSearch = DunkSearchContext{BallPosition: s.Ball.Body.Position, MountFlags: s.MountFlags}
	for i, h := range s.Hoops {
		carrierContext.DunkSearch.HoopOwners[i] = h.Owner
	}
	applyCarrier := func(step CarrierStep) {
		a.Player, a.ShotCycle = step.Carrier.Player, step.Carrier.ShotCycle
		s.Ball = ApplyBallCommand(s.Ball, step.Ball)
		if step.BindBall {
			s.Holder = int8(actor)
		}
		if step.Revisit {
			s.PlayerLinks[actor] = 0
		}
		if step.ChangeAimWindow {
			a.AimWindow = step.AimWindow
		}
		if step.ChangeSpecialShot {
			a.SpecialShot = step.SpecialShot
		}
		if step.ChangeRandom {
			s.Random = step.Random
		}
		if step.ChangeDunkTarget {
			s.DunkTargetObject = step.DunkTargetObject
		}
	}
	switch {
	case p.Action == 0x68:
		if err := advanceWorldAuxiliaryRunningPlayer(result, actor, ctx, court); err != nil {
			return err
		}
	case capturedAction(p.Action):
		if err := advanceWorldCapturedPlayer(result, actor, ctx, court); err != nil {
			return err
		}
	case p.Action == 0x4f:
		if err := advanceWorldAuxiliarySetDown(result, actor, ctx, court); err != nil {
			return err
		}
	case p.Action == ActionLaunched:
		if err := advanceWorldLaunchedPlayer(result, actor, ctx, court); err != nil {
			return err
		}
	case p.Action == ActionRiding:
		parent := s.PlayerLinks[actor] & 15
		if parent >= 4 {
			return fmt.Errorf("rider support object %d requires its original kind", parent)
		}
		support := s.Actors[parent].Player
		step, err := StepRiderPlayer(RiderPlayerState{Player: *p, JumpPhase: a.JumpPhase}, RiderPlayerContext{
			Actor: actor, Attachment: s.PlayerLinks[actor], Computer: computer, HasBall: carrierContext.HasBall,
			Support:         RiderSupport{Body: support.Body, Pose: support.Pose, Kind: ctx.Teams[parent/2]},
			AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner},
		}, profile, court)
		if err != nil {
			return err
		}
		a.Player, a.JumpPhase, s.PlayerLinks[actor] = step.State.Player, step.State.JumpPhase, step.Attachment
		s.Ball = ApplyBallCommand(s.Ball, step.Ball)
		if step.ChangeSupportLink {
			s.Actors[step.SupportObject].LinkedActor = int8(actor)
		}
	case p.Action == 0x58:
		if s.DunkTargetObject < 7 || s.DunkTargetObject >= 13 {
			return fmt.Errorf("auxiliary dunk target %d not reconstructed", s.DunkTargetObject)
		}
		target := &s.Hoops[s.DunkTargetObject-7]
		step, err := StepDunkPlayer(*p, DunkContext{Actor: actor, TargetObject: s.DunkTargetObject, TargetOwner: target.Owner, TargetDurability: target.Durability, AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, CarriedPlayer: s.CarriedPlayer, DetachedCount: s.DetachedCount, Attachment: s.PlayerLinks[actor], Ball: s.Ball, BallHolder: s.Holder, AuxiliaryRelease: s.AuxiliaryRelease, Computer: computer}, court)
		if err != nil {
			return err
		}
		a.Player, s.Ball, target.Durability, s.AuxiliaryRelease = step.Player, step.Ball, step.TargetDurability, step.AuxiliaryRelease
		if step.SetActiveDunker {
			s.LastShooter = int8(step.ActiveDunker)
		}
		if step.Revisit {
			s.PlayerLinks[actor] = 0
		}
		for _, command := range step.Objects {
			if err := s.applyHurtObject(HurtObjectCommand{Object: command.Object, SetAction: true, Action: command.Action}); err != nil {
				return err
			}
		}
	case p.Action >= 0x60 && p.Action <= 0x69:
		step, err := StepPlayerAttack(AttackState{Player: *p, AttackVariant: a.AttackVariant, JumpPhase: a.JumpPhase}, AttackContext{Team: ctx.Teams[actor/2], Character: ctx.Characters[actor], Computer: computer, HasBall: carrierContext.HasBall, Attachment: s.PlayerLinks[actor], EnhancementEnabled: ctx.EnhancementEnabled, Enhancement: ctx.Enhancements[actor]}, profile, court)
		if err != nil {
			return err
		}
		a.Player, a.AttackVariant, a.JumpPhase = step.State.Player, step.State.AttackVariant, step.State.JumpPhase
		s.Ball = ApplyBallCommand(s.Ball, step.Ball)
		if step.Revisit {
			s.PlayerLinks[actor] = 0
		}
		appendPlayerActionSounds(result, motionSoundStart, step.SoundIDs)
	case p.Action >= 0x47 && p.Action <= 0x4a:
		step, err := StepHurtPlayer(HurtState{Player: *p, Endurance: a.Endurance}, HurtContext{Actor: actor, Computer: computer, RecoveryGrade: ctx.RecoveryGrades[actor], Random: s.Random, BallHolder: s.Holder, BallVelocityX: s.Ball.Body.Velocity.X, AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, HoopOwners: s.hoopOwners(), SpecialHolder: s.SpecialHolder, AssistedActor: s.Auxiliary[1].ObjectLink, LinkedActor: a.LinkedActor, Attachments: s.PlayerLinks}, court)
		if err != nil {
			return err
		}
		a.Player, a.Endurance, s.Random = step.State.Player, step.State.Endurance, step.Random
		for _, command := range step.Objects {
			if err := s.applyHurtObject(command); err != nil {
				return err
			}
		}
		s.Ball = ApplyBallCommand(s.Ball, step.Ball)
		if step.ReleaseBall {
			s.Holder = 6
		}
		if step.ChangeBallVelocity {
			s.Ball.Body.Velocity.X = step.BallVelocityX
		}
		if step.ClearSpecialHolder {
			s.SpecialHolder = -1
		}
	case worldAuxiliaryPlayerAction(*s, actor):
		if err := advanceWorldAuxiliaryCarrier(result, actor, ctx, court); err != nil {
			return err
		}
	case p.Action == ActionDodge:
		step, err := StepCarrierDodge(Carrier{Player: *p, ShotCycle: a.ShotCycle}, a.Dodge, carrierContext, profile, court)
		if err != nil {
			return err
		}
		applyCarrier(step.CarrierStep)
		a.Dodge = step.Memory
	case p.Action == ActionFalling:
		// $978D scans $053D..$0546; SpecialHolder ($056B) is not an
		// ownership slot in that scan. A boxed runner can fall normally.
		otherHeld := heldHoop >= 0 || s.Auxiliary[0].Owner == int8(actor) || s.Auxiliary[1].Owner == int8(actor) || s.CarriedPlayer == int8(actor)
		step, err := StepFallingPlayer(*p, FallingContext{Computer: computer, HasBall: carrierContext.HasBall, HasOtherHeldObject: otherHeld, Attached: s.PlayerLinks[actor]&128 != 0}, court)
		if err != nil {
			return err
		}
		a.Player = step.Player
		s.Ball = ApplyBallCommand(s.Ball, step.Ball)
	case p.Action == ActionPickUp && heldHoop < 0 || p.Action != ActionPickUp && ordinaryCarrierAction(p.Action):
		origin := p.Body.Position.X &^ 255
		step, err := StepCarrierAction(Carrier{Player: *p, ShotCycle: a.ShotCycle}, carrierContext, profile, court)
		if err != nil {
			return err
		}
		applyCarrier(step)
		if step.ShotReleased {
			s.Ball.FacingLeft = a.Player.FacingLeft
			s.ShotOriginX = origin
			s.LastShooter = int8(actor)
		}
	case freeAction(p.Action):
		// Ordinary movement handlers can follow an accepted stale request
		// while this actor still owns the ball. They retain their action IDs;
		// shared run and terrain branches consume ownership explicitly.
		if p.Action == ActionRun && !p.Entered || p.Request.Pending && p.Request.Action == ActionRun && freeTransition(p.Action, p.Request.Action) {
			a.ShotCycle = 8
		}
		step, err := StepFreePlayerAction(*p, FreePlayerContext{Computer: computer, HasBall: carrierContext.HasBall, JumpPhase: a.JumpPhase, ShotCycle: a.ShotCycle}, profile, court)
		if err != nil {
			return err
		}
		a.Player, a.ShotCycle = step.Player, step.ShotCycle
		s.Ball = ApplyBallCommand(s.Ball, step.Ball)
		if step.Revisit {
			s.PlayerLinks[actor] = 0
		}
	default:
		step, err := StepHoopCarrierAction(HoopCarrier{Player: *p, ShotCycle: a.ShotCycle, AimWindow: a.AimWindow}, HoopCarrierContext{Computer: computer, HeldHoop: heldHoop}, profile, court)
		if err != nil {
			return err
		}
		a.Player, a.ShotCycle, a.AimWindow = step.State.Player, step.State.ShotCycle, step.State.AimWindow
		if step.Hoop.ChangeAction {
			s.Hoops[step.Hoop.Index].Action, s.Hoops[step.Hoop.Index].Entered = step.Hoop.Action, false
		}
		if step.Hoop.ChangeFacing {
			s.Hoops[step.Hoop.Index].FacingLeft = step.Hoop.FacingLeft
		}
	}
	if court.originalMotion != nil && court.originalMotion.resetJumpPhase {
		a.JumpPhase = 0
	}
	return nil
}

func advanceWorldBall(s *WorldActionState, ctx WorldActionContext) error {
	return advanceWorldBallOnCourt(s, ctx, NewYorkBallCourt())
}

func advanceWorldBallOnCourt(s *WorldActionState, ctx WorldActionContext, court FlatCourt) error {
	_, err := advanceWorldBallStep(s, ctx, court)
	return err
}

func advanceWorldBallStep(s *WorldActionState, ctx WorldActionContext, court FlatCourt) ([]uint8, error) {
	var sounds []uint8
	var err error
	s.Ball, err = ResolveBallRequest(s.Ball)
	if err != nil {
		return nil, err
	}
	switch s.Ball.Action {
	case BallRest:
		s.Ball, err = StepRestingBall(s.Ball, RestBallContext{CameraShake: ctx.CameraShake}, court)
	case BallRolling:
		var step LooseBallStep
		step, err = StepRollingBall(s.Ball, court)
		if err == nil {
			s.Ball = step.Ball
			if step.ClearExcludedCatcher {
				s.ExcludedCatcher = -1
			}
		}
	case BallDribble, BallHeld:
		if s.Holder < 0 || s.Holder >= 4 {
			return nil, fmt.Errorf("held ball has invalid player owner %d", s.Holder)
		}
		clearExclusion := s.Ball.Action == BallHeld && !s.Ball.Entered
		s.Ball, err = StepHeldBallOnCourt(s.Ball, s.Actors[s.Holder].Player, court)
		if err == nil {
			// $B54F / $B5C3 write on every update. The held overhead
			// override temporarily sets bit 7; $EC82 clears it again.
			s.PoseSource = uint8(s.Holder)
		}
		if err == nil && clearExclusion {
			s.ExcludedCatcher = -1
		}
	case BallKnockback, BallDropped:
		var step LooseBallStep
		if s.Ball.Action == BallKnockback {
			step, err = StepKnockedBackBall(s.Ball, court)
		} else {
			step, err = StepDroppedBall(s.Ball, DroppedBallContext{CourtID: ctx.CourtID}, court)
		}
		if err == nil {
			s.Ball = step.Ball
			if step.ClearExcludedCatcher {
				s.ExcludedCatcher = -1
			}
		}
	case BallStuck:
		if s.SelectedHoop < 7 || s.SelectedHoop >= 13 {
			return nil, fmt.Errorf("stuck ball selected hoop %d is invalid", s.SelectedHoop)
		}
		var step StuckBallStep
		step, err = StepStuckBall(s.Ball, s.Hoops[s.SelectedHoop-7].Position.Height, court)
		if err == nil {
			s.Ball = step.Ball
			if step.ReleaseHolder {
				s.Holder = 6
			}
		}
	case BallAction(10):
		step, e := StepDunkReleasedBall(s.Ball, court)
		err = e
		if err == nil {
			s.Ball = step.Ball
			if step.ReleaseHolder {
				s.Holder = 6
			}
		}
	case BallBounce:
		s.Ball, err = StepBouncingBall(s.Ball, court)
	case BallDeflected:
		s.Ball, err = StepDeflectedBall(s.Ball, court)
	case BallPass:
		initializing := !s.Ball.Entered
		receiver := s.LaunchTarget
		arc := s.LaunchArcHeight
		if initializing {
			if s.Holder < 0 || s.Holder >= 4 {
				return nil, fmt.Errorf("pass launch has invalid owner")
			}
			receiver = s.Actors[s.Holder^1].Player.Body.Position
			receiver.X, receiver.Depth, receiver.Height = receiver.X&^255, receiver.Depth&^255, receiver.Height&^255
			distance := absPass(int32(receiver.X/Pixel - s.Ball.Body.Position.X/Pixel))
			arcs := [8]Fixed{0, 4, 8, 16, 48, 64, 80, 96}
			if distance/64 >= 8 {
				return nil, fmt.Errorf("pass arc distance outside restored domain")
			}
			arc = arcs[distance/64] * Pixel
		}
		s.Ball, err = stepPassedBallOnCourt(s.Ball, receiver, court)
		if err == nil && initializing {
			// $B604 writes before ownership is released; later pass updates
			// leave the shared source untouched.
			s.PoseSource = uint8(s.Holder)
			s.ExcludedCatcher = s.Holder
			s.Holder = 6
			s.BallLanding = Position{X: receiver.X, Depth: receiver.Depth}
			s.LaunchTarget = receiver
			s.LaunchArcHeight = arc
		}
		// Original pass tail B64F clears the previous passer exclusion when
		// court motion grounds the ball, including a launch that lands at once.
		if err == nil && !s.Ball.Airborne {
			s.ExcludedCatcher = -1
		}
	case BallShot, 13, 20, 21, 23:
		holder := s.Holder
		shotContext := OriginalShotBallContext{Holder: holder, UnownedSpecialShot: s.UnownedSpecialShot, EnhancementEnabled: ctx.EnhancementEnabled, Equipment: 255, BallInputButtons: ctx.BallShotInputButtons, MountFlags: s.MountFlags, SelectedHoop: s.SelectedHoop, Target: s.LaunchTarget, ArcHeight: s.LaunchArcHeight, Landing: s.BallLanding}
		if holder >= 0 && holder < 4 {
			shooter := s.Actors[holder]
			shotContext.ShooterHeight, shotContext.ShooterAction = shooter.Player.Body.Position.Height, shooter.Player.Action
			shotContext.Team, shotContext.Character = ctx.Teams[holder/2], ctx.Characters[holder]
			shotContext.Equipment, shotContext.SpecialShot = ctx.ShotEquipment[holder], shooter.SpecialShot
		}
		var step OriginalShotBallStep
		step, err = StepOriginalShotBall(s.Ball, shotContext, court)
		if err == nil {
			s.Ball, s.SelectedHoop, s.LaunchTarget, s.LaunchArcHeight, s.BallLanding = step.Ball, step.SelectedHoop, step.Target, step.ArcHeight, step.Landing
			if step.ChangeSpecialShot {
				s.Actors[holder].SpecialShot = step.SpecialShot
			}
			if step.ClearUnownedSpecialShot {
				s.UnownedSpecialShot = 0
			}
			if step.ReleaseHolder {
				s.Holder = 6
			}
			sounds = append(sounds, step.SoundIDs...)
		}

	default:
		return nil, fmt.Errorf("ball action %d not reconstructed", s.Ball.Action)
	}
	return sounds, err
}

// StepWorldActions executes exactly the original unified action stage. Input,
// AI, owned-ball input and contact phases have already run. Each handler is
// called once per scheduler visit; any error discards every partial mutation.
func StepWorldActions(original WorldActionState, ctx WorldActionContext) (WorldActionStep, error) {
	initial := WorldActionStep{State: original}
	result := initial
	if ctx.CourtID > 5 {
		return initial, fmt.Errorf("world action court %d not reconstructed", ctx.CourtID)
	}
	for actor, profile := range ctx.Profiles {
		if profile.Animations == nil {
			return initial, fmt.Errorf("missing actor %d animation profile", actor)
		}
	}
	scheduler := ActionScheduler{}
	for {
		object, links, ok := scheduler.Next(result.State.actionLinks())
		result.State.setActionLinks(links)
		if !ok {
			break
		}
		if len(result.Calls) >= 256 {
			return initial, fmt.Errorf("world action dependency loop exceeded restored domain")
		}
		result.Calls = append(result.Calls, object)
		s := &result.State
		var err error
		court := NewYorkGroundCourt()
		if object >= 4 && object <= 6 {
			court = NewYorkBallCourt()
		}
		if ctx.CourtID == 0 || ctx.CourtID == 2 || ctx.CourtID == 3 || ctx.CourtID == 4 || ctx.CourtID == 5 || object == 6 {
			court, err = originalWorldMotionCourt(&result, ctx, object)
			if err != nil {
				return initial, err
			}
		} else {
			court = court.withMotionSurfaceWrites(object, &result.MotionSurfaceWrites)
		}
		court.passLaunchContext = func() PassLaunchContext {
			return PassLaunchContext{SecondPlayerX: result.State.Actors[1].Player.Body.Position.X}
		}
		switch {
		case object < 4:
			err = advanceWorldPlayer(&result, object, ctx, court)
		case object < 6:
			err = advanceWorldAuxiliary(&result, object, ctx, court)
		case object == 6:
			s.Ball, err = ResolveBallRequest(s.Ball)
			if err == nil {
				if s.Ball.Action == 12 || s.Ball.Action == 14 || s.Ball.Action == 15 {
					err = advanceWorldTeamSpecialBall(&result, ctx, court)
				} else {
					var sounds []uint8
					sounds, err = advanceWorldBallStep(s, ctx, court)
					result.SoundIDs = append(result.SoundIDs, sounds...)
				}
			}
		default:
			if s.Hoops[object-7].Action == HoopReady && !s.Hoops[object-7].Entered {
				s.Hoops[object-7].Owner = originalHoopMountOwner(s.Hoops[object-7].Position)
			}
			h := s.Hoops[object-7].component()
			if h.Request.Pending && h.Request.Action != 0 {
				return initial, fmt.Errorf("object %d hoop request transition not reconstructed", object)
			}
			hc := HoopContext{Index: object - 7, DetachedCount: s.DetachedCount, MountFlags: s.MountFlags, ContactLatch: s.ContactLatch, HasClimber: s.Auxiliary[0].Owner == int8(object) || s.Auxiliary[1].Owner == int8(object)}
			if h.Carried {
				if h.CarrierSeat >= 4 {
					return initial, fmt.Errorf("invalid hoop owner %d", h.CarrierSeat)
				}
				hc.Owner = &s.Actors[h.CarrierSeat].Player
				hc.OwnerComputer = ctx.Controls[h.CarrierSeat]&128 != 0
				hc.OwnerSpecial = s.Actors[h.CarrierSeat].SpecialShot != 0
				if s.SelectedHoop >= 7 && s.SelectedHoop < 13 {
					hc.HighThrowArc = s.MountFlags[s.SelectedHoop-7]&2 != 0
				} else if s.SelectedHoop >= 0 && s.SelectedHoop < 7 {
					hc.HighThrowArc = ctx.OtherObjectArcFlags[s.SelectedHoop]&2 != 0
				} else if h.Action == HoopThrown && !h.Entered {
					return initial, fmt.Errorf("hoop throw arc selector %d not reconstructed", s.SelectedHoop)
				}
			}
			var step HoopStep
			step, err = StepHoopAction(h, hc, court)
			if err == nil {
				// Follow-source writes belong to the dispatched action, not
				// whichever next action its animation selected this update.
				switch {
				case h.Action == HoopCarried || h.Action == HoopScored && h.Carried:
					// $DEF5 / $DE28 followed by $EC7B ($EC82 strips bit 7).
					s.PoseSource = s.Hoops[object-7].Owner & 127
				case h.Action == HoopThrown && !h.Entered:
					// $DF11 runs only at launch, before clearing the owner.
					s.PoseSource = s.Hoops[object-7].Owner
				}
				s.Hoops[object-7].applyComponent(step.Hoop, object-7)
				s.MountFlags, s.ContactLatch = step.MountFlags, step.ContactLatch
				if step.ChangeSelectedBoard {
					result.ChangeSelectedBoard, result.SelectedBoard = true, step.SelectedBoard
				}
				if step.ClearOwnerSpecial {
					s.Actors[h.CarrierSeat].SpecialShot = 0
				}
				if step.ThrowPlan != nil {
					s.LaunchTarget, s.LaunchArcHeight = step.ThrowPlan.Target, step.ThrowPlan.ArcHeight
				}
			}
		}
		if err != nil {
			return initial, fmt.Errorf("object %d: %w", object, err)
		}
	}
	return result, nil
}
