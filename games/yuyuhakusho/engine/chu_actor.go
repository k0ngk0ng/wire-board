package engine

import "errors"

// ChuActorFighter owns one selected fighter's intrinsic state. Energy is the
// complete fixed-point resource; source controls, AI counters and HUD display
// resources each have one owner rather than duplicated component snapshots.
type ChuActorFighter struct {
	ObjectID, Size                                         uint16
	Body                                                   FighterBodyState
	Controls                                               FighterControls
	Energy                                                 int32
	InputMode                                              int8
	InputChannel                                           uint8
	StreamPosition                                         uint32
	AIBehavior                                             uint8
	AITargetCursor, AICountdown, AIElapsed, AIFollowTarget uint16
	ScreenX, ScreenY, ScreenOffsetX, ScreenOffsetY         int16
	DisplayedHealth, DisplayedEnergy                       int16
	HUDOffset                                              uint16
	AnimationSet, ShapeSet                                 uint32
	GraphicOffsetX, GraphicOffsetY                         int16
	GraphicParameter                                       uint16
	ChargeTicks, GuardMask, ContactFlags                   uint8
	TargetMask                                             uint32
	AttackValue, StunDamage, EffectKind, Reaction          uint8
	AttackScale                                            uint16
	HitStopTicks, Shake, StatusDelay                       uint8
	Residual                                               int32
	StunTicks                                              uint16
	TargetBit                                              uint8
	Power                                                  int16
	AlternateVelocity                                      int32
	Children                                               [3]ChuSpecialChild
	ChildAttackFlags                                       [3]uint8
}

// ChuActorWorld is the shared state passed to the next object callback. The
// world scheduler must not reinitialize these counters or RNG per fighter.
type ChuActorWorld struct {
	Pool                   EffectPool
	EffectExtras           [16]ReactionEffectExtra
	OtherEnergy            map[uint16]int16
	OccupiedLanes          uint8
	Buffers                [4]AnimationShapeBuffer
	Throws                 ThrowContactBuffer
	PendingSound           uint8
	SoundQueue             [4]uint8
	RandomSeed             uint32
	GlobalF568, GlobalF56A uint16
	Stream                 []byte
	Stopped                uint8
	EndMarkers             [4]uint8
	Registration           FighterRegistrationState
	SampleUsed             uint16
	SamplesX, SamplesY     [16]int32
	HUDCells               []uint16
	HUDCursor              uint16
}

// ChuActorView contains immutable external inputs and visible other-fighter
// facts for this callback. Self geometry is rebuilt from Fighter before AI.
type ChuActorView struct {
	Pause, Slow, Disabled, Blink  uint8
	Difficulty                    uint16
	CameraX, CameraY, CameraShake int16
	StreamBase                    uint32
	Controllers                   [8]ControllerFrame
	Fighters                      [4]ChuAIFighter
	ArenaLeft, ArenaRight         int16
	BattleState                   int16
	Parameters                    ChuActionParameters
}

type ChuActorState struct {
	Fighter ChuActorFighter
	World   ChuActorWorld
}

type ChuActorEmissions struct {
	DrawRequested bool
	ObjectID      uint16
	HUDTransfers  []FighterBarTransfer
}

// StepChuActor recovers the complete rule part of active callback0x12682,
// ending immediately before0x1580 sprite emission. Initialization, child
// callbacks, collision-manager timing and the world loop remain caller stages.
func StepChuActor(state ChuActorState, view ChuActorView, commands CommandLibrary, animations AnimationLibrary) (ChuActorState, ChuActorEmissions, error) {
	return StepFighterActor(state, 9, view, commands, animations)
}

// StepFighterActor shares the original complete active-rule callback between
// sources0/2/3/8/9/10; drawing, source identity and other callbacks remain caller-owned.
func StepFighterActor(state ChuActorState, characterID uint8, view ChuActorView, commands CommandLibrary, animations AnimationLibrary) (ChuActorState, ChuActorEmissions, error) {
	original := state
	if characterID != 0 && characterID != 2 && characterID != 3 && characterID != 8 && characterID != 9 && characterID != 10 {
		return original, ChuActorEmissions{}, errors.New("unsupported complete fighter actor source")
	}
	self, ok := chuAIIndex(state.Fighter.ObjectID)
	if !ok {
		return original, ChuActorEmissions{}, errors.New("unsupported Chu actor object")
	}
	registration, err := RegisterFighter(state.World.Registration, state.Fighter.ObjectID)
	if err != nil {
		return original, ChuActorEmissions{}, err
	}
	state.World.Registration = registration
	if view.Pause&0x80 == 0 && view.Slow == 0 {
		f, w := &state.Fighter, &state.World
		view.Fighters[self] = ChuAIFighter{Size: f.Size, Team: f.Body.Team, Health: f.Body.Health, X: int16(f.Body.X >> 16), Y: int16(f.Body.Y >> 16), Ground: int16(f.Body.Ground >> 16),
			FrameFlags: f.Body.Clock.Flags, AttackFlags: f.Body.Auxiliary64, ChildX: int16(f.Children[0].X >> 16), ChildY: int16(f.Children[0].Y >> 16),
			ChildGround: int16(f.Children[0].Ground >> 16), ChildAttackFlags: f.ChildAttackFlags[0], ChildAttackValue: view.Fighters[self].ChildAttackValue}
		input := ControllerFrame{}
		if f.InputChannel < 8 {
			input = view.Controllers[f.InputChannel]
		}
		inputState, inputErr := StepFighterInput(ChuInputState{Envelope: FighterInputEnvelopeState{Controls: f.Controls, Mode: f.InputMode, Channel: f.InputChannel,
			Disabled: view.Disabled, Stopped: w.Stopped, Position: f.StreamPosition, StreamBase: view.StreamBase, Stream: w.Stream, Input: input, EndMarkers: w.EndMarkers},
			AI: ChuAIState{ObjectID: f.ObjectID, Actor: ChuAIActor{CharacterID: characterID, Facing: f.Body.Facing, Pose: f.Body.Pose, Energy: int16(f.Energy >> 16), Controls: f.Controls,
				Request: f.Body.Request, Behavior: f.AIBehavior, TargetCursor: f.AITargetCursor, Countdown: f.AICountdown, Elapsed: f.AIElapsed, FollowTarget: f.AIFollowTarget},
				Fighters: view.Fighters, Difficulty: view.Difficulty, RandomSeed: w.RandomSeed, ArenaLeft: view.ArenaLeft, ArenaRight: view.ArenaRight}})
		if inputErr != nil {
			return original, ChuActorEmissions{}, inputErr
		}
		e, ai := inputState.Envelope, inputState.AI
		f.Controls, f.InputMode, f.InputChannel, f.StreamPosition = e.Controls, e.Mode, e.Channel, e.Position
		f.Body.Request, f.AIBehavior = ai.Actor.Request, ai.Actor.Behavior
		f.AITargetCursor, f.AICountdown, f.AIElapsed, f.AIFollowTarget = ai.Actor.TargetCursor, ai.Actor.Countdown, ai.Actor.Elapsed, ai.Actor.FollowTarget
		w.Stream, w.Stopped, w.EndMarkers, w.RandomSeed = e.Stream, e.Stopped, e.EndMarkers, ai.RandomSeed
		action, actionErr := state.actorAction(view)
		if actionErr != nil {
			return original, ChuActorEmissions{}, actionErr
		}
		action, actionErr = StepFighterAction(action, characterID, view.Parameters, commands, animations)
		if actionErr != nil {
			return original, ChuActorEmissions{}, actionErr
		}
		state.acceptActorAction(action)
		f = &state.Fighter
		projection := ProjectFighter(ProjectionState{WorldX: f.Body.X, WorldY: f.Body.Y, ScreenX: f.ScreenX, ScreenY: f.ScreenY,
			OffsetX: f.ScreenOffsetX, OffsetY: f.ScreenOffsetY, CameraX: view.CameraX, CameraY: view.CameraY, Shake: view.CameraShake})
		f.ScreenX, f.ScreenY, f.ScreenOffsetX, f.ScreenOffsetY = projection.ScreenX, projection.ScreenY, projection.OffsetX, projection.OffsetY
	}
	if state.Fighter.Body.Health != -1024 {
		bounds, boundsErr := StepFighterBounds(FighterBoundsState{X: state.Fighter.Body.X, Y: state.Fighter.Body.Y, Left: view.ArenaLeft, Right: view.ArenaRight,
			Used: state.World.SampleUsed, XSamples: state.World.SamplesX, YSamples: state.World.SamplesY})
		if boundsErr != nil {
			return original, ChuActorEmissions{}, boundsErr
		}
		state.Fighter.Body.X, state.Fighter.Body.Y = bounds.X, bounds.Y
		state.World.SampleUsed, state.World.SamplesX, state.World.SamplesY = bounds.Used, bounds.XSamples, bounds.YSamples
	}
	f, w := &state.Fighter, &state.World
	hud, transfers, hudErr := StepFighterHUD(FighterHUDState{Health: f.Body.Health, DisplayedHealth: f.DisplayedHealth, Energy: int16(f.Energy >> 16), DisplayedEnergy: f.DisplayedEnergy,
		Offset: f.HUDOffset, TargetBit: f.TargetBit, Blink: view.Blink, Cells: w.HUDCells, TransferCursor: w.HUDCursor})
	if hudErr != nil {
		return original, ChuActorEmissions{}, hudErr
	}
	f.DisplayedHealth, f.DisplayedEnergy, w.HUDCells, w.HUDCursor = hud.DisplayedHealth, hud.DisplayedEnergy, hud.Cells, hud.TransferCursor
	return state, ChuActorEmissions{DrawRequested: true, ObjectID: f.ObjectID, HUDTransfers: transfers}, nil
}

func (state ChuActorState) actorAction(view ChuActorView) (ChuActionState, error) {
	f, w := state.Fighter, state.World
	resources := make(map[uint16]TailFighterEnergy, len(w.OtherEnergy))
	for id, energy := range w.OtherEnergy {
		index, ok := chuAIIndex(id)
		if !ok || id == f.ObjectID {
			return ChuActionState{}, errors.New("other fighter resource identity is unavailable")
		}
		visible := view.Fighters[index]
		resources[id] = TailFighterEnergy{Size: visible.Size, Team: visible.Team, Health: visible.Health, Energy: energy}
	}
	return ChuActionState{ChuTailState: ChuTailState{Body: f.Body, Controls: f.Controls, ObjectID: f.ObjectID, Energy: int16(f.Energy >> 16), ChildSize: f.Children[0].Size,
		Pool: w.Pool, Throws: w.Throws, OtherFighters: resources, ChargeTicks: f.ChargeTicks, OccupiedLanes: w.OccupiedLanes, BattleState: view.BattleState,
		AnimationSet: f.AnimationSet, ShapeSet: f.ShapeSet, GraphicOffsetX: f.GraphicOffsetX, GraphicOffsetY: f.GraphicOffsetY, GraphicParameter: f.GraphicParameter,
		GuardMask: f.GuardMask, ContactFlags: f.ContactFlags, TargetMask: f.TargetMask, AttackValue: f.AttackValue, StunDamage: f.StunDamage, EffectKind: f.EffectKind, Reaction: f.Reaction,
		AttackScale: f.AttackScale, RandomSeed: w.RandomSeed, Buffers: w.Buffers, PendingSound: w.PendingSound, SoundQueue: w.SoundQueue},
		EnergyFraction: uint16(f.Energy), HitStopTicks: f.HitStopTicks, Shake: f.Shake, ScreenOffsetX: f.ScreenOffsetX, StatusDelay: f.StatusDelay, Residual: f.Residual,
		StunTicks: f.StunTicks, TargetBit: f.TargetBit, Power: f.Power, AlternateVelocity: f.AlternateVelocity, Children: f.Children, EffectExtras: w.EffectExtras,
		GlobalF568: w.GlobalF568, GlobalF56A: w.GlobalF56A, Left: view.ArenaLeft, Right: view.ArenaRight}, nil
}

func (state *ChuActorState) acceptActorAction(a ChuActionState) {
	f, w := &state.Fighter, &state.World
	f.Body, f.Controls = a.Body, a.Controls
	f.Energy = int32(uint32(uint16(a.Energy))<<16 | uint32(a.EnergyFraction))
	f.ChargeTicks, f.GuardMask, f.ContactFlags, f.TargetMask = a.ChargeTicks, a.GuardMask, a.ContactFlags, a.TargetMask
	f.AnimationSet, f.ShapeSet, f.GraphicOffsetX, f.GraphicOffsetY, f.GraphicParameter = a.AnimationSet, a.ShapeSet, a.GraphicOffsetX, a.GraphicOffsetY, a.GraphicParameter
	f.AttackValue, f.StunDamage, f.EffectKind, f.Reaction, f.AttackScale = a.AttackValue, a.StunDamage, a.EffectKind, a.Reaction, a.AttackScale
	f.HitStopTicks, f.Shake, f.ScreenOffsetX, f.StatusDelay, f.Residual = a.HitStopTicks, a.Shake, a.ScreenOffsetX, a.StatusDelay, a.Residual
	f.StunTicks, f.TargetBit, f.Power, f.AlternateVelocity, f.Children = a.StunTicks, a.TargetBit, a.Power, a.AlternateVelocity, a.Children
	w.Pool, w.EffectExtras, w.Throws = a.Pool, a.EffectExtras, a.Throws
	w.OtherEnergy = make(map[uint16]int16, len(a.OtherFighters))
	for id, resource := range a.OtherFighters {
		w.OtherEnergy[id] = resource.Energy
	}
	w.OccupiedLanes, w.Buffers, w.PendingSound, w.SoundQueue, w.RandomSeed = a.OccupiedLanes, a.Buffers, a.PendingSound, a.SoundQueue, a.RandomSeed
	w.GlobalF568, w.GlobalF56A = a.GlobalF568, a.GlobalF56A
}
