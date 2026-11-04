package engine

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ChuWorldMatch owns the shared match configuration and counters. Round is
// the start/bracket round at A07e; RoundIndex is the distinct A06a label index.
// Controller selection codes are 1..4 for humans and 0 for the original AI.
type ChuWorldMatch struct {
	Arena, Rule, Layout, Round, RoundPair              uint16
	RoundIndex                                         int16
	Grouping                                           uint16
	Characters, Controllers, Colors, BaseTeams         [4]uint8
	Options                                            [3]uint8
	ModeFlag, Disabled, OptionA097                     uint8
	EligibleChannels, TeamResult                       uint8
	OpeningTicks, ResultTicks, AttributeMask, Selected uint16
	WinnerCode                                         int16
	TeamTitles                                         [2]uint16
}

// ChuWorldState is the single rule world. Component views are built at each
// callback and merged immediately, rather than kept as independent worlds.
// Cells covers E600..EC00: fighter HUD = [1:513], arena rows = [256:768].
// LossesByCharacter[60:64] is also the recording end-marker storage.
type ChuWorldState struct {
	Objects                                           WorldObjects
	Clock                                             WorldClockState
	Controllers                                       [8]ControllerFrame
	RandomSeed                                        uint32
	Palette                                           ArenaPaletteState
	Camera                                            DefaultArenaCamera
	Cells                                             [768]uint16
	Buffers                                           [4]AnimationShapeBuffer
	Throws                                            ThrowContactBuffer
	Registration                                      FighterRegistrationState
	PendingSound                                      uint8
	SoundQueue                                        [4]uint8
	SlowGate                                          uint8
	SlowTicks                                         uint16
	EffectCursor                                      uint16
	EffectTileCursor                                  uint8
	OccupiedLanes                                     uint8
	ComboFlags                                        uint8
	Stream                                            []byte
	Stopped                                           uint8
	HeadToHead                                        [16]uint8
	WinsByCharacter, LossesByCharacter                [64]uint8
	Difficulty                                        uint16
	ResetEnabled                                      uint8
	VideoMode, SceneMode, SceneRequest, InterruptMode uint16
	ScrollMode                                        uint8
	Vertical                                          [2]int16
	PanA, PanB                                        [224]int16
	ScrollSources                                     [2]int32
	ResourceList                                      uint32
	ResourceBusy                                      uint8
	Match                                             ChuWorldMatch
	MatchComplete                                     bool
}

type ChuWorldView struct {
	StreamBase uint32
}

type ChuWorldLibrary struct {
	Fighters            FighterSpecLibrary
	Commands            CommandLibrary
	Animations          AnimationLibrary
	Attachments         ChuAttachmentLibrary
	Effects             EffectMotionLibrary
	AttachedEffects     AttachedEffectLibrary
	MatchStart          MatchStartLibrary
	Arena               ArenaRuntimeLibrary
	Fighter0Projectiles Fighter0ProjectileLibrary
}

// Events follow callback address order. A callback records its entry header
// and saved stride; draw/HUD/start events are semantic renderer operations.
// Physical sprite caches, VDP commands and F518 DMA cursors are not rules.
type ChuWorldEvent struct {
	Kind         string
	ObjectID     uint16
	Header       MatchObjectHeader
	Stride       uint16
	ClearedBytes uint16
	HUD          FighterBarTransfer
	Start        MatchStartEvent
	Draw         *WorldDraw `json:",omitempty"`
}

// StepChuWorldObjects recovers one ready-match 3592 pass. A future slot created
// by a preceding callback executes immediately in this pass. On any failure,
// the entire caller state and event list are preserved atomically.
func StepChuWorldObjects(state ChuWorldState, view ChuWorldView, library ChuWorldLibrary) (ChuWorldState, []ChuWorldEvent, error) {
	original := state
	if state.MatchComplete {
		return state, nil, nil
	}
	for id := range state.Objects {
		if _, err := LookupWorldObject(state.Objects, id); err != nil {
			return original, nil, err
		}
	}
	state.Objects = cloneWorldObjects(state.Objects)
	state.Stream = append([]byte(nil), state.Stream...)
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Registration.Owners = append([]uint16(nil), state.Registration.Owners...)
	var events []ChuWorldEvent
	var hudCursor uint16 // Logical per-pass transfer cursor, not a physical DMA address.
	for address := uint32(0x8000); address < 0xa000; {
		id := uint16(address)
		object, err := LookupWorldObject(state.Objects, id)
		if err != nil {
			return original, nil, err
		}
		if object.Header.Behavior == 0 {
			address += 64
			continue
		}
		stride := object.Header.Size
		if stride == 0 || stride&63 != 0 || stride > 256 || address+uint32(stride) > 0xa000 {
			return original, nil, fmt.Errorf("invalid object stride %d at %04x", stride, id)
		}
		events = append(events, ChuWorldEvent{Kind: "callback", ObjectID: id, Header: object.Header, Stride: stride})
		more, err := stepChuWorldObject(&state, object, view, library, &hudCursor)
		if err != nil {
			return original, nil, fmt.Errorf("object %04x callback %x stage %d: %w", id, object.Header.Behavior, object.Header.Stage, err)
		}
		for i := range more {
			if more[i].Kind == "draw" {
				request := state.Objects[more[i].ObjectID].DrawRequest()
				more[i].Draw = &request
			}
		}
		events = append(events, more...)
		if state.MatchComplete {
			break
		}
		address += uint32(stride)
	}
	return state, events, nil
}

func stepChuWorldObject(state *ChuWorldState, object WorldObject, view ChuWorldView, library ChuWorldLibrary, hudCursor *uint16) ([]ChuWorldEvent, error) {
	id := object.ObjectID
	switch object.Header.Behavior {
	case 0x9ad0:
		manager, err := LookupWorldObject(state.Objects, 0x8040)
		if err != nil || id != 0x8000 || object.Header.Stage != 20 || manager.Header.Behavior != 0xf2a4 {
			return nil, errors.New("main container is outside the proven ready-match wait boundary")
		}
		return nil, nil
	case 0xf2a4:
		if id != 0x8040 {
			return nil, errors.New("match manager identity is unavailable")
		}
		if object.Header.Stage < 4 {
			return stepChuWorldStart(state, object, library.MatchStart)
		}
		return stepChuWorldMatch(state, object)
	case 0x1edac:
		return nil, stepChuWorldCollision(state)
	case 0x20918:
		return nil, stepChuWorldArena(state, object, library.Arena)
	case 0x12668:
		return stepChuWorldFighter(state, object, view, library, hudCursor)
	case 0x1313e:
		return stepChuWorldProjectile(state, object, library.Animations)
	case 0x12c96, 0x134c2:
		return stepFighter0WorldProjectile(state, object, library.Fighter0Projectiles)
	case 0x12dd4:
		return stepFighter2WorldProjectile(state, object, library.Animations)
	case 0x132de, 0x1361e:
		return stepChuWorldAttachment(state, object, library.Attachments)
	case 0x149ac:
		return stepChuWorldEffect(state, object, library.Effects)
	case 0x14930, 0x14ec6:
		return stepChuWorldAttachedEffect(state, object, library.AttachedEffects)
	case 0x14882:
		return stepChuWorldShadow(state, object)
	case 0x150ca, 0x1525e, 0x15330:
		return stepChuWorldSky(state, object, library.Arena)
	case 0xff74:
		return nil, stepChuWorldTraining(state)
	case 0xfe78, 0xfeb2, 0x10088, 0x10116, 0x10172, 0x101f2, 0x10330, 0x10436, 0x104ac, 0x10522, 0x10616, 0x106e8:
		return stepChuWorldOverlay(state, object)
	default:
		return nil, errors.New("object callback is not recovered")
	}
}

func chuWorldPresentation(id uint16, draw bool, cleared uint16) []ChuWorldEvent {
	var events []ChuWorldEvent
	if cleared != 0 {
		events = append(events, ChuWorldEvent{Kind: "clear", ObjectID: id, ClearedBytes: cleared})
	}
	if draw {
		events = append(events, ChuWorldEvent{Kind: "draw", ObjectID: id})
	}
	return events
}

func stepChuWorldOverlay(state *ChuWorldState, object WorldObject) ([]ChuWorldEvent, error) {
	s := MatchOverlayState{ObjectID: object.ObjectID, Object: object.AsMatchOverlay(), ModeFlag: state.Match.ModeFlag, SlowFlag: state.Clock.SlowFlag, SoundQueue: state.SoundQueue}
	if object.Header.Behavior == 0x101f2 {
		for i := range s.Children {
			child, err := LookupWorldObject(state.Objects, object.ObjectID+uint16(i+1)*64)
			if err != nil {
				return nil, err
			}
			s.Children[i] = child.AsMatchOverlay()
		}
	}
	next, out, err := StepMatchOverlay(s, MatchOverlayEnvironment{PauseFlags: state.Clock.PauseFlags, WinnerCode: state.Match.WinnerCode, Characters: state.Match.Characters, TeamTitles: state.Match.TeamTitles, RoundIndex: uint16(state.Match.RoundIndex), Blink: uint8(state.Clock.Frame)})
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object.MergeMatchOverlay(next.Object)
	if s.Object.Header.Behavior == 0x101f2 {
		for i, child := range next.Children {
			id := object.ObjectID + uint16(i+1)*64
			old, _ := LookupWorldObject(state.Objects, id)
			state.Objects[id] = old.MergeMatchOverlay(child)
		}
	}
	state.SoundQueue, state.Match.ModeFlag, state.Clock.SlowFlag = next.SoundQueue, next.ModeFlag, next.SlowFlag
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldMatch(state *ChuWorldState, object WorldObject) ([]ChuWorldEvent, error) {
	m := &state.Match
	s := MatchLifecycleState{Manager: object.Header, PauseFlags: state.Clock.PauseFlags, PauseMask: state.Clock.PauseMask, ModeFlag: m.ModeFlag, Disabled: m.Disabled,
		EligibleChannels: m.EligibleChannels, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue, OpeningTicks: m.OpeningTicks, ResultTicks: m.ResultTicks,
		WinnerCode: m.WinnerCode, Rule: m.Rule, RoundIndex: m.RoundIndex, Grouping: m.Grouping, TeamResult: m.TeamResult, OptionFF14: m.Options[0], OptionFF16: m.Options[2], OptionA097: m.OptionA097,
		Characters: m.Characters, Controllers: m.Controllers, PaletteFlags: state.Palette.Flags, PaletteStatus: state.Palette.Status, PaletteMode: state.Palette.Mode, PaletteStep: state.Palette.Step, PaletteTarget: state.Palette.Target,
		VideoMode: state.VideoMode, CurrentColors: state.Palette.Current, SecondaryColors: state.Palette.Secondary, HeadToHead: state.HeadToHead, WinsByCharacter: state.WinsByCharacter, LossesByCharacter: state.LossesByCharacter}
	for i := range s.Pressed {
		s.Pressed[i] = uint8(state.Controllers[i].Pressed)
		f, _ := LookupWorldObject(state.Objects, uint16(0x8400+i*0x400))
		s.Fighters[i] = MatchFighter{Behavior: f.Header.Behavior, Health: f.Body.Health, BaseTeam: f.Body.BaseTeam}
	}
	for i := range s.Presentation {
		p, _ := LookupWorldObject(state.Objects, uint16(0x8300+i*64))
		s.Presentation[i] = p.Header
	}
	next, out, err := StepMatchLifecycle(s)
	if err != nil {
		return nil, err
	}
	object.Header = next.Manager
	state.Objects[object.ObjectID] = object
	state.Clock.PauseFlags, state.Clock.PauseMask = next.PauseFlags, next.PauseMask
	m.ModeFlag, m.Disabled, m.EligibleChannels = next.ModeFlag, next.Disabled, next.EligibleChannels
	m.OpeningTicks, m.ResultTicks, m.WinnerCode, m.TeamResult = next.OpeningTicks, next.ResultTicks, next.WinnerCode, next.TeamResult
	state.PendingSound, state.SoundQueue = next.PendingSound, next.SoundQueue
	state.Palette.Flags, state.Palette.Status, state.Palette.Mode, state.Palette.Step, state.Palette.Target = next.PaletteFlags, next.PaletteStatus, next.PaletteMode, next.PaletteStep, next.PaletteTarget
	state.Palette.Current, state.Palette.Secondary, state.VideoMode = next.CurrentColors, next.SecondaryColors, next.VideoMode
	state.HeadToHead, state.WinsByCharacter, state.LossesByCharacter = next.HeadToHead, next.WinsByCharacter, next.LossesByCharacter
	for i, header := range next.Presentation {
		id := uint16(0x8300 + i*64)
		p, _ := LookupWorldObject(state.Objects, id)
		p.Header = header
		state.Objects[id] = p
	}
	if out.ClearWorldFrom != 0 {
		for id := range state.Objects {
			if id >= out.ClearWorldFrom {
				state.Objects[id] = WorldObject{ObjectID: id}
			}
		}
		state.MatchComplete = true
		return []ChuWorldEvent{{Kind: "clear-world", ObjectID: out.ClearWorldFrom}}, nil
	}
	return nil, nil
}

func stepChuWorldCollision(state *ChuWorldState) error {
	pool, err := BuildWorldEffectPool(state.Objects, state.EffectCursor)
	if err != nil {
		return err
	}
	s := CollisionManagerState{Actors: make(map[uint16]CollisionActor), Buffers: state.Buffers, Throws: state.Throws, Effects: pool,
		PendingSound: state.PendingSound, SoundQueue: state.SoundQueue, PauseFlags: state.Clock.PauseFlags, SlowTicks: state.SlowTicks, SlowFlag: state.Clock.SlowFlag,
		ComboFlags: state.ComboFlags, ArenaLeft: state.Camera.Left, ArenaRight: state.Camera.Right}
	for id := uint16(0x8400); id < 0x9400; id += 256 {
		o, err := LookupWorldObject(state.Objects, id)
		if err != nil {
			return err
		}
		s.Actors[id] = o.AsCollisionActor()
	}
	for i := range s.EffectParameters {
		o, _ := LookupWorldObject(state.Objects, uint16(0x9400+i*128))
		s.EffectParameters[i] = o.GraphicParameter
	}
	next, err := ResolveCollisionManager(s)
	if err != nil {
		return err
	}
	for id, actor := range next.Actors {
		o, _ := LookupWorldObject(state.Objects, id)
		state.Objects[id] = o.MergeCollisionActor(actor)
	}
	state.Objects, err = MergeWorldEffectPool(state.Objects, next.Effects)
	if err != nil {
		return err
	}
	for i, parameter := range next.EffectParameters {
		id := uint16(0x9400 + i*128)
		o := state.Objects[id]
		o.GraphicParameter = parameter
		state.Objects[id] = o
	}
	state.EffectCursor, state.Buffers, state.Throws = next.Effects.Cursor, next.Buffers, next.Throws
	state.PendingSound, state.SoundQueue = next.PendingSound, next.SoundQueue
	state.SlowTicks, state.Clock.SlowFlag, state.ComboFlags = next.SlowTicks, next.SlowFlag, next.ComboFlags
	return nil
}

func stepChuWorldArena(state *ChuWorldState, object WorldObject, library ArenaRuntimeLibrary) error {
	s := DefaultArenaState{Stage: object.Header.Stage, Offsets: [3]int16{object.ScreenX, object.ScreenY, int16(object.Body.VelocityX >> 16)},
		Drifts: [2]int32{object.Body.VerticalVelocity, object.Aux18}, Phases: [2]uint16{uint16(object.TilesBase >> 16), uint16(object.TilesBase)},
		Camera: state.Camera, Mode: state.ScrollMode, Vertical: state.Vertical, PanA: state.PanA, PanB: state.PanB, SourceA: state.ScrollSources[0], SourceB: state.ScrollSources[1]}
	v := DefaultArenaView{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, Shake: int16(state.Clock.ScreenShake), Pan: int16(state.Clock.PanOffset)}
	for i := range s.RowsA {
		s.RowsA[i], s.RowsB[i], v.HUDPrefix[i] = int16(state.Cells[256+i]), int16(state.Cells[512+i]), int16(state.Cells[i])
	}
	next, err := StepDefaultArena(s, v, library)
	if err != nil {
		return err
	}
	object.Header.Stage, object.ScreenX, object.ScreenY = next.Stage, next.Offsets[0], next.Offsets[1]
	object.Body.VelocityX = worldObjectHighWord(object.Body.VelocityX, uint16(next.Offsets[2]))
	object.Body.VerticalVelocity, object.Aux18 = next.Drifts[0], next.Drifts[1]
	object.TilesBase = uint32(next.Phases[0])<<16 | uint32(next.Phases[1])
	state.Objects[object.ObjectID] = object
	state.Camera, state.ScrollMode, state.Vertical = next.Camera, next.Mode, next.Vertical
	state.PanA, state.PanB, state.ScrollSources = next.PanA, next.PanB, [2]int32{next.SourceA, next.SourceB}
	for i := range next.RowsA {
		state.Cells[256+i], state.Cells[512+i] = uint16(next.RowsA[i]), uint16(next.RowsB[i])
	}
	return nil
}

func stepChuWorldFighter(state *ChuWorldState, object WorldObject, view ChuWorldView, library ChuWorldLibrary, hudCursor *uint16) ([]ChuWorldEvent, error) {
	if _, ok := chuAIIndex(object.ObjectID); !ok || (object.Tag != 0 && object.Tag != 2 && object.Tag != 3 && object.Tag != 9 && object.Tag != 10) {
		return nil, errors.New("complete fighter action source is not recovered")
	}
	switch object.Header.Stage {
	case 0:
		next, err := InitializeFighter(object.AsFighterInitialization(), state.Match.AttributeMask, library.Fighters)
		if err != nil {
			return nil, err
		}
		object = object.MergeFighterInitialization(next)
		object.Header.Stage += 2
		state.Objects[object.ObjectID] = object
		return nil, nil
	case 4:
		count := object.Header.Size >> 4 << 4
		cleared, err := ApplyObjectClearPrefix(object, count)
		if err != nil {
			return nil, err
		}
		state.Objects[object.ObjectID] = cleared
		return chuWorldPresentation(object.ObjectID, false, count), nil
	case 2:
	default:
		return nil, errors.New("fighter callback stage is unavailable")
	}
	fighter, err := BuildChuActorFighter(state.Objects, object.ObjectID)
	if err != nil {
		return nil, err
	}
	pool, err := BuildWorldEffectPool(state.Objects, state.EffectCursor)
	if err != nil {
		return nil, err
	}
	w := ChuActorWorld{Pool: pool, OtherEnergy: make(map[uint16]int16), OccupiedLanes: state.OccupiedLanes, Buffers: state.Buffers, Throws: state.Throws,
		PendingSound: state.PendingSound, SoundQueue: state.SoundQueue, RandomSeed: state.RandomSeed, GlobalF568: state.Clock.DurationRequest, GlobalF56A: state.Clock.AmplitudeRequest,
		Stream: state.Stream, Stopped: state.Stopped, Registration: state.Registration, SampleUsed: state.Camera.Used, SamplesX: state.Camera.SamplesX, SamplesY: state.Camera.SamplesY,
		HUDCells: state.Cells[1:513], HUDCursor: *hudCursor}
	copy(w.EndMarkers[:], state.LossesByCharacter[60:64])
	for i := range w.EffectExtras {
		o, _ := LookupWorldObject(state.Objects, uint16(0x9400+i*128))
		w.EffectExtras[i] = o.AsReactionExtra()
	}
	spec, ok := library.Fighters[object.Tag]
	if !ok || spec.CharacterID != object.Tag {
		return nil, errors.New("fighter movement specification is unavailable")
	}
	v := ChuActorView{Pause: state.Clock.PauseFlags, Slow: state.Clock.SlowFlag, Disabled: state.Match.Disabled, Blink: uint8(state.Clock.Frame), Difficulty: state.Difficulty,
		CameraX: state.Camera.X, CameraY: state.Camera.Y, CameraShake: int16(state.Clock.ScreenShake), StreamBase: view.StreamBase, Controllers: state.Controllers,
		ArenaLeft: state.Camera.Left, ArenaRight: state.Camera.Right, BattleState: int16(state.Match.ResultTicks),
		Parameters: ChuActionParameters{ChuTailParameters: ChuTailParameters{JumpParameters: spec.Movement.JumpParameters, BurstSpeed: spec.Movement.BurstSpeed, EnergyDrain: spec.Movement.EnergyDrain}, WalkSpeed: spec.Movement.WalkSpeed}}
	for i := range v.Fighters {
		id := uint16(0x8400 + i*0x400)
		o, _ := LookupWorldObject(state.Objects, id)
		child, _ := LookupWorldObject(state.Objects, id+256)
		v.Fighters[i] = ChuAIFighter{Size: o.Header.Size, Team: o.Body.Team, Health: o.Body.Health, X: int16(o.Body.X >> 16), Y: int16(o.Body.Y >> 16), Ground: int16(o.Body.Ground >> 16),
			FrameFlags: o.Body.Clock.Flags, AttackFlags: o.Body.Auxiliary64, ChildX: int16(child.Body.X >> 16), ChildY: int16(child.Body.Y >> 16), ChildGround: int16(child.Body.Ground >> 16), ChildAttackFlags: child.Body.Auxiliary64, ChildAttackValue: child.AttackValue}
		if id != object.ObjectID {
			w.OtherEnergy[id] = int16(o.Energy >> 16)
		}
	}
	next, out, err := StepFighterActor(ChuActorState{Fighter: fighter, World: w}, object.Tag, v, library.Commands, library.Animations)
	if err != nil {
		return nil, err
	}
	state.Objects, err = MergeWorldChuActor(state.Objects, next.Fighter)
	if err != nil {
		return nil, err
	}
	w = next.World
	state.Objects, err = MergeWorldEffectPool(state.Objects, w.Pool)
	if err != nil {
		return nil, err
	}
	for i, extra := range w.EffectExtras {
		id := uint16(0x9400 + i*128)
		state.Objects[id] = state.Objects[id].MergeReactionExtra(extra)
	}
	for id, energy := range w.OtherEnergy {
		o, _ := LookupWorldObject(state.Objects, id)
		o.Energy = worldObjectHighWord(o.Energy, uint16(energy))
		state.Objects[id] = o
	}
	state.EffectCursor, state.OccupiedLanes, state.Buffers, state.Throws = w.Pool.Cursor, w.OccupiedLanes, w.Buffers, w.Throws
	state.PendingSound, state.SoundQueue, state.RandomSeed = w.PendingSound, w.SoundQueue, w.RandomSeed
	state.Clock.DurationRequest, state.Clock.AmplitudeRequest = w.GlobalF568, w.GlobalF56A
	state.Stream, state.Stopped, state.Registration = w.Stream, w.Stopped, w.Registration
	copy(state.LossesByCharacter[60:64], w.EndMarkers[:])
	state.Camera.Used, state.Camera.SamplesX, state.Camera.SamplesY = w.SampleUsed, w.SamplesX, w.SamplesY
	copy(state.Cells[1:513], w.HUDCells)
	*hudCursor = w.HUDCursor
	events := make([]ChuWorldEvent, 0, len(out.HUDTransfers)+1)
	for _, transfer := range out.HUDTransfers {
		events = append(events, ChuWorldEvent{Kind: "hud", ObjectID: object.ObjectID, HUD: transfer})
	}
	if out.DrawRequested {
		events = append(events, ChuWorldEvent{Kind: "draw", ObjectID: object.ObjectID})
	}
	return events, nil
}

func stepChuWorldProjectile(state *ChuWorldState, object WorldObject, library AnimationLibrary) ([]ChuWorldEvent, error) {
	return stepWorldProjectile(state, object, func(projectile ChuProjectileState, environment ChuProjectileEnvironment) (ChuProjectileState, ChuProjectileOutcome, error) {
		return StepChuProjectile(projectile, environment, library)
	})
}

func stepFighter0WorldProjectile(state *ChuWorldState, object WorldObject, library Fighter0ProjectileLibrary) ([]ChuWorldEvent, error) {
	kind := Fighter0ScatterProjectile
	if object.Header.Behavior == 0x134c2 {
		kind = Fighter0ReleaseLight
	}
	return stepWorldProjectile(state, object, func(projectile ChuProjectileState, environment ChuProjectileEnvironment) (ChuProjectileState, ChuProjectileOutcome, error) {
		next, out, err := StepFighter0Projectile(Fighter0ProjectileState{Kind: kind, Projectile: projectile}, environment, library)
		return next.Projectile, out, err
	})
}

func stepFighter2WorldProjectile(state *ChuWorldState, object WorldObject, library AnimationLibrary) ([]ChuWorldEvent, error) {
	hitStop := object.HitStopTicks
	events, err := stepWorldProjectile(state, object, func(projectile ChuProjectileState, environment ChuProjectileEnvironment) (ChuProjectileState, ChuProjectileOutcome, error) {
		next, out, err := StepFighter2Projectile(Fighter2ProjectileState{Projectile: projectile, HitStopTicks: hitStop}, environment, library)
		hitStop = next.HitStopTicks
		return next.Projectile, out, err
	})
	if err != nil {
		return nil, err
	}
	// +69 remains owned by the world object, including on partial deletion.
	merged := state.Objects[object.ObjectID]
	merged.HitStopTicks = hitStop
	state.Objects[object.ObjectID] = merged
	return events, nil
}

func stepWorldProjectile(state *ChuWorldState, object WorldObject, step func(ChuProjectileState, ChuProjectileEnvironment) (ChuProjectileState, ChuProjectileOutcome, error)) ([]ChuWorldEvent, error) {
	s := object.AsChuProjectile()
	var err error
	s.Pool, err = BuildWorldEffectPool(state.Objects, state.EffectCursor)
	if err != nil {
		return nil, err
	}
	s.Buffers, s.PendingSound, s.SoundQueue = state.Buffers, state.PendingSound, state.SoundQueue
	next, out, err := step(s, ChuProjectileEnvironment{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, PaletteAttributes: state.Match.AttributeMask,
		ArenaLeft: state.Camera.Left, ArenaRight: state.Camera.Right, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)})
	if err != nil {
		return nil, err
	}
	state.Objects, err = MergeWorldEffectPool(state.Objects, next.Pool)
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object, err = object.MergeChuProjectile(next)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object
	state.EffectCursor, state.Buffers, state.PendingSound, state.SoundQueue = next.Pool.Cursor, next.Buffers, next.PendingSound, next.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldAttachment(state *ChuWorldState, object WorldObject, library ChuAttachmentLibrary) ([]ChuWorldEvent, error) {
	kind, offset := ChuChargeAttachment, uint16(0x200)
	if object.Header.Behavior == 0x1361e {
		kind, offset = ChuReleaseAttachment, 0x300
	}
	parentID := object.ObjectID - offset
	if _, ok := chuAIIndex(parentID); !ok {
		return nil, errors.New("attachment parent identity is unavailable")
	}
	parent, err := LookupWorldObject(state.Objects, parentID)
	if err != nil {
		return nil, err
	}
	s := object.AsChuAttachment(kind)
	s.Buffers, s.PendingSound, s.SoundQueue = state.Buffers, state.PendingSound, state.SoundQueue
	next, out, err := StepChuAttachment(s, ChuAttachmentParent{CharacterID: parent.Tag, FrameIndex: parent.Body.Clock.FrameIndex, Pose: parent.Body.Pose, X: parent.Body.X, Y: parent.Body.Y, Power: parent.Power},
		ChuAttachmentWorld{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, AttributeMask: state.Match.AttributeMask, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}, library)
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object, err = object.MergeChuAttachment(next)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object
	state.Buffers, state.PendingSound, state.SoundQueue = next.Buffers, next.PendingSound, next.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldEffect(state *ChuWorldState, object WorldObject, library EffectMotionLibrary) ([]ChuWorldEvent, error) {
	owners := make(map[uint16]EffectMotionOwner, len(state.Objects))
	for id, o := range state.Objects {
		owners[id] = EffectMotionOwner{X: o.Body.X, Y: o.Body.Y}
	}
	next, world, out, err := StepEffectMotion(object.AsEffectMotion(), EffectMotionWorld{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag,
		PaletteAttributes: state.Match.AttributeMask, TileCursor: state.EffectTileCursor, SoundQueue: state.SoundQueue, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}, owners, library)
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object, err = object.MergeEffectMotion(next)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object
	state.EffectTileCursor, state.SoundQueue = world.TileCursor, world.SoundQueue
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldAttachedEffect(state *ChuWorldState, object WorldObject, library AttachedEffectLibrary) ([]ChuWorldEvent, error) {
	kind := AttachedComboNumber
	var parent AttachedEffectParent
	if object.Header.Behavior == 0x14ec6 {
		kind = AttachedStunEffect
		id := uint16(uint32(object.Aux48) >> 16)
		o, err := LookupWorldObject(state.Objects, id)
		if err != nil {
			return nil, err
		}
		parent = AttachedEffectParent{ObjectID: id, CharacterID: o.Tag, FrameIndex: o.Body.Clock.FrameIndex, Ticks: o.Body.Clock.Ticks, ActionKind: o.Body.ActionKind, X: o.Body.X, Y: o.Body.Y, StunTicks: o.StunTicks}
	}
	next, out, err := StepAttachedEffect(object.AsAttachedEffect(kind), parent, AttachedEffectWorld{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, AttributeMask: state.Match.AttributeMask,
		CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}, library)
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object, err = object.MergeAttachedEffect(next)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldShadow(state *ChuWorldState, object WorldObject) ([]ChuWorldEvent, error) {
	owners := make(map[uint16]MatchShadowOwner, len(state.Objects))
	for id, o := range state.Objects {
		owners[id] = MatchShadowOwner{Depth: o.Body.Depth, X: o.Body.X, Ground: o.Body.Ground}
	}
	s := object.AsMatchShadow()
	s.Registration = state.Registration
	next, out, err := StepMatchShadow(s, MatchShadowEnvironment{AttributeMask: state.Match.AttributeMask, CameraX: state.Camera.X, CameraY: state.Camera.Y, Shake: int16(state.Clock.ScreenShake)}, owners)
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object.MergeMatchShadow(next)
	state.Registration = next.Registration
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldSky(state *ChuWorldState, object WorldObject, library ArenaRuntimeLibrary) ([]ChuWorldEvent, error) {
	kind := ArenaSun
	if object.Header.Behavior == 0x1525e {
		kind = ArenaMoon
	} else if object.Header.Behavior == 0x15330 {
		kind = ArenaSpark
	}
	world := ArenaSkyWorld{PauseFlags: state.Clock.PauseFlags, SlowFlag: state.Clock.SlowFlag, CameraY: state.Camera.Y, Pan: int16(state.Clock.PanOffset), PaletteStatus: state.Palette.Status, Primary: state.Palette.Primary, RandomSeed: state.RandomSeed}
	if kind == ArenaMoon {
		next, err := LookupWorldObject(state.Objects, object.ObjectID+128)
		if err != nil {
			return nil, err
		}
		world.Next = next.Header
	}
	next, world, out, err := StepArenaSky(object.AsArenaSky(kind), world, library)
	if err != nil {
		return nil, err
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
	if err != nil {
		return nil, err
	}
	object, err = object.MergeArenaSky(next)
	if err != nil {
		return nil, err
	}
	state.Objects[object.ObjectID] = object
	if kind == ArenaMoon {
		id := object.ObjectID + 128
		o, _ := LookupWorldObject(state.Objects, id)
		o.Header = world.Next
		state.Objects[id] = o
	}
	state.Palette.Primary, state.RandomSeed = world.Primary, world.RandomSeed
	return chuWorldPresentation(object.ObjectID, out.Draw, out.ClearedBytes), nil
}

func stepChuWorldTraining(state *ChuWorldState) error {
	var s MatchTrainingState
	var held, extended [4]uint8
	for i := range s.Targets {
		o, err := LookupWorldObject(state.Objects, uint16(0x8400+i*0x400))
		if err != nil {
			return err
		}
		s.Targets[i] = MatchHealthTarget{Behavior: o.Header.Behavior, Health: o.Body.Health}
		held[i], extended[i] = uint8(state.Controllers[i].Current), uint8(state.Controllers[i].Current>>8)
	}
	next := StepMatchTraining(s, state.Clock.PauseFlags, held, extended)
	for i, target := range next.Targets {
		id := uint16(0x8400 + i*0x400)
		o, _ := LookupWorldObject(state.Objects, id)
		o.Header.Behavior, o.Body.Health = target.Behavior, target.Health
		state.Objects[id] = o
	}
	return nil
}

func stepChuWorldStart(state *ChuWorldState, object WorldObject, library MatchStartLibrary) ([]ChuWorldEvent, error) {
	m := &state.Match
	s := MatchStartState{Manager: object.Header, PauseFlags: state.Clock.PauseFlags, ModeFlag: m.ModeFlag, ResourceBusy: state.ResourceBusy, ResourceList: state.ResourceList, VideoMode: state.VideoMode,
		Arena: m.Arena, Rule: m.Rule, Layout: m.Layout, Round: m.Round, RoundPair: m.RoundPair, Characters: m.Characters, Controllers: m.Controllers, Colors: m.Colors, BaseTeams: m.BaseTeams,
		Options: m.Options, OptionA097: m.OptionA097, Disabled: m.Disabled, SlowGate: state.SlowGate, SlowFlag: state.Clock.SlowFlag, SlowTicks: state.SlowTicks,
		PendingSound: state.PendingSound, SoundQueue: state.SoundQueue, OpeningTicks: m.OpeningTicks, ResultTicks: m.ResultTicks, WinnerCode: uint16(m.WinnerCode), AttributeMask: m.AttributeMask, Selected: m.Selected,
		Camera: [4]int16{state.Camera.X, state.Camera.Y, state.Camera.TargetX, state.Camera.TargetY}, Bounds: [2]int16{state.Camera.Left, state.Camera.Right}, PrimaryColors: state.Palette.Primary,
		Feedback:      [7]uint16{state.Clock.ScreenShake, state.Clock.DurationRequest, state.Clock.AmplitudeRequest, state.Clock.Amplitude, state.Clock.Decay, state.Clock.PanAmplitude, state.Clock.PanOffset},
		ScrollSources: [2]uint32{uint32(state.ScrollSources[0]), uint32(state.ScrollSources[1])}, ResetWords: [4]uint16{state.Cells[256], state.Cells[512], uint16(state.Vertical[0]), uint16(state.Vertical[1])}}
	ids := [10]uint16{0x8100, 0x8140, 0x8180, 0x81c0, 0x8200, 0x8240, 0x8280, 0x9c00, 0x9c80, 0x9d00}
	for i, id := range ids {
		o, err := LookupWorldObject(state.Objects, id)
		if err != nil {
			return nil, err
		}
		s.Objects[i] = MatchStartObject{Header: o.Header, Parameter: uint16(uint32(o.Aux18) >> 16)}
	}
	for i := range s.Fighters {
		o, err := LookupWorldObject(state.Objects, uint16(0x8400+i*0x400))
		if err != nil {
			return nil, err
		}
		s.Fighters[i] = MatchStartFighter{Header: o.Header, TargetBit: o.TargetBit, Attributes: o.Body.Attributes, X: o.Body.X, Ground: o.Body.Ground, Depth: o.Body.Depth,
			Facing: o.Body.Facing, Layer: o.Body.Layer, CharacterID: o.Tag, Channel: o.InputChannel, BaseTeam: o.Body.BaseTeam, InputMode: uint8(o.InputMode), StreamPosition: o.StreamPosition, HUDOffset: o.HUDOffset}
	}
	next, out, err := StepMatchStart(s, library)
	if err != nil {
		return nil, err
	}
	cells := state.Cells
	cells[256], cells[512] = next.ResetWords[0], next.ResetWords[1]
	cells, err = matchStartCells(cells, out.Events)
	if err != nil {
		return nil, err
	}
	object.Header = next.Manager
	state.Objects[object.ObjectID] = object
	for i, id := range ids {
		o, _ := LookupWorldObject(state.Objects, id)
		o.Header = next.Objects[i].Header
		o.Aux18 = worldObjectHighWord(o.Aux18, next.Objects[i].Parameter)
		state.Objects[id] = o
	}
	for i, f := range next.Fighters {
		id := uint16(0x8400 + i*0x400)
		o, _ := LookupWorldObject(state.Objects, id)
		o.Header, o.TargetBit, o.Body.Attributes, o.Body.X, o.Body.Ground, o.Body.Depth = f.Header, f.TargetBit, f.Attributes, f.X, f.Ground, f.Depth
		o.Body.Facing, o.Body.Layer, o.Tag, o.InputChannel, o.Body.BaseTeam = f.Facing, f.Layer, f.CharacterID, f.Channel, f.BaseTeam
		o.InputMode, o.StreamPosition, o.HUDOffset = int8(f.InputMode), f.StreamPosition, f.HUDOffset
		state.Objects[id] = o
	}
	state.Clock.PauseFlags, state.Match.ModeFlag, state.VideoMode = next.PauseFlags, next.ModeFlag, next.VideoMode
	state.ResourceBusy, state.ResourceList = next.ResourceBusy, next.ResourceList
	m.Disabled, state.SlowGate, state.Clock.SlowFlag, state.SlowTicks = next.Disabled, next.SlowGate, next.SlowFlag, next.SlowTicks
	state.PendingSound, state.SoundQueue = next.PendingSound, next.SoundQueue
	m.OpeningTicks, m.ResultTicks, m.WinnerCode, m.AttributeMask, m.Selected = next.OpeningTicks, next.ResultTicks, int16(next.WinnerCode), next.AttributeMask, next.Selected
	state.Camera.X, state.Camera.Y, state.Camera.TargetX, state.Camera.TargetY, state.Camera.Left, state.Camera.Right = next.Camera[0], next.Camera[1], next.Camera[2], next.Camera[3], next.Bounds[0], next.Bounds[1]
	state.Palette.Primary = next.PrimaryColors
	state.Clock.ScreenShake, state.Clock.DurationRequest, state.Clock.AmplitudeRequest, state.Clock.Amplitude, state.Clock.Decay, state.Clock.PanAmplitude, state.Clock.PanOffset = next.Feedback[0], next.Feedback[1], next.Feedback[2], next.Feedback[3], next.Feedback[4], next.Feedback[5], next.Feedback[6]
	state.ScrollSources = [2]int32{int32(next.ScrollSources[0]), int32(next.ScrollSources[1])}
	state.Cells = cells
	state.Vertical[0], state.Vertical[1] = int16(next.ResetWords[2]), int16(next.ResetWords[3])
	events := make([]ChuWorldEvent, 0, len(out.Events))
	for _, event := range out.Events {
		events = append(events, ChuWorldEvent{Kind: "start", ObjectID: object.ObjectID, Start: event})
	}
	return events, nil
}

// FE68 writes words at E600+byteOffset. This is the shared cells base,
// distinct from the later fighter HUD view that starts at E602.
func matchStartCells(cells [768]uint16, events []MatchStartEvent) ([768]uint16, error) {
	original := cells
	for _, event := range events {
		if event.Kind != "words" {
			continue
		}
		start := int(event.Offset) / 2
		if event.Offset&1 != 0 || start > len(cells) || len(event.Words) > len(cells)-start {
			return original, errors.New("match-start words exceed the shared cell range")
		}
		copy(cells[start:], event.Words)
	}
	return cells, nil
}

type chuWorldAnimationRule struct {
	Key  AnimationFrameKey   `json:"key"`
	Data AnimationRecordData `json:"data"`
}

type chuWorldRules struct {
	Schema      int                     `json:"schema"`
	Fighters    []FighterSpec           `json:"fighters"`
	Commands    CommandLibrary          `json:"commands"`
	Animations  []chuWorldAnimationRule `json:"animations"`
	Attachments struct {
		Sequences []struct {
			ID     uint32               `json:"id"`
			Frames []ChuAttachmentFrame `json:"frames"`
		} `json:"sequences"`
		Animations []chuWorldAnimationRule `json:"animations"`
	} `json:"attachments"`
	Effects         EffectMotionLibrary `json:"effects"`
	AttachedEffects struct {
		Sequences []struct {
			ID     uint32                `json:"id"`
			Frames []AttachedEffectFrame `json:"frames"`
		} `json:"sequences"`
		OrbitX  [1024]int8 `json:"orbit_x"`
		OrbitY  [1024]int8 `json:"orbit_y"`
		OffsetY [11]int16  `json:"offset_y"`
	} `json:"attached_effects"`
	MatchStart          MatchStartLibrary   `json:"match_start"`
	Arena               ArenaRuntimeLibrary `json:"arena"`
	Fighter0Projectiles struct {
		Sequences []struct {
			Identity uint32               `json:"identity"`
			Frames   []ChuAttachmentFrame `json:"frames"`
		} `json:"sequences"`
	} `json:"fighter0_projectiles"`
}

// DecodeChuWorldRules loads immutable semantic data once, before simulation.
// The bundle contains no CPU cases, mutable RAM images or client media.
func DecodeChuWorldRules(raw []byte) (ChuWorldLibrary, error) {
	var rules chuWorldRules
	var empty ChuWorldLibrary
	if err := json.Unmarshal(raw, &rules); err != nil {
		return empty, err
	}
	if (rules.Schema != 1 && rules.Schema != 2) || len(rules.Fighters) != 11 || len(rules.Animations) == 0 || len(rules.Commands) == 0 || len(rules.Effects.Types) != 37 || len(rules.Attachments.Sequences) != 8 || len(rules.Attachments.Animations) == 0 || len(rules.AttachedEffects.Sequences) != 3 || len(rules.Arena.WaveX) != 1024 || len(rules.Arena.WaveY) != 1024 || len(rules.Arena.Tails) == 0 || rules.MatchStart.ArenaBehaviors[0] != 0x20918 {
		return empty, errors.New("incomplete or unsupported Chu world rules")
	}
	library := ChuWorldLibrary{Fighters: make(FighterSpecLibrary), Commands: rules.Commands, Animations: make(AnimationLibrary), Effects: rules.Effects, MatchStart: rules.MatchStart, Arena: rules.Arena,
		Attachments:     ChuAttachmentLibrary{Sequences: make(map[uint32][]ChuAttachmentFrame), Animations: make(AnimationLibrary)},
		AttachedEffects: AttachedEffectLibrary{Sequences: make(map[uint32][]AttachedEffectFrame), OrbitX: rules.AttachedEffects.OrbitX, OrbitY: rules.AttachedEffects.OrbitY, OffsetY: rules.AttachedEffects.OffsetY}}
	for _, fighter := range rules.Fighters {
		if fighter.CharacterID >= 11 {
			return empty, errors.New("invalid world fighter identity")
		}
		if _, found := library.Fighters[fighter.CharacterID]; found {
			return empty, errors.New("duplicate world fighter identity")
		}
		library.Fighters[fighter.CharacterID] = fighter
	}
	for _, animation := range rules.Animations {
		if _, found := library.Animations[animation.Key]; found {
			return empty, errors.New("duplicate world animation identity")
		}
		library.Animations[animation.Key] = animation.Data
	}
	if rules.Schema == 2 {
		if len(rules.Fighter0Projectiles.Sequences) != 8 {
			return empty, errors.New("source0 release sequences are unavailable")
		}
		library.Fighter0Projectiles = Fighter0ProjectileLibrary{Animations: library.Animations, Sequences: make(map[uint32][]ChuAttachmentFrame)}
		for _, sequence := range rules.Fighter0Projectiles.Sequences {
			if sequence.Identity < 0x135ee || sequence.Identity > 0x13618 || (sequence.Identity-0x135ee)%6 != 0 || len(sequence.Frames) != 3 || sequence.Frames[2] != (ChuAttachmentFrame{}) {
				return empty, errors.New("invalid source0 release sequence identity or termination")
			}
			if _, exists := library.Fighter0Projectiles.Sequences[sequence.Identity]; exists {
				return empty, errors.New("duplicate source0 release sequence")
			}
			for _, frame := range sequence.Frames[:2] {
				if frame.Duration == 0 || frame.Parameter&1 != 0 || frame.Parameter > 510 {
					return empty, errors.New("invalid source0 release frame")
				}
			}
			library.Fighter0Projectiles.Sequences[sequence.Identity] = sequence.Frames
		}
	}
	for _, animation := range rules.Attachments.Animations {
		if _, found := library.Attachments.Animations[animation.Key]; found {
			return empty, errors.New("duplicate attachment animation identity")
		}
		library.Attachments.Animations[animation.Key] = animation.Data
	}
	for _, sequence := range rules.Attachments.Sequences {
		if _, found := library.Attachments.Sequences[sequence.ID]; found {
			return empty, errors.New("duplicate attachment sequence identity")
		}
		library.Attachments.Sequences[sequence.ID] = sequence.Frames
	}
	for _, sequence := range rules.AttachedEffects.Sequences {
		if _, found := library.AttachedEffects.Sequences[sequence.ID]; found {
			return empty, errors.New("duplicate status sequence identity")
		}
		library.AttachedEffects.Sequences[sequence.ID] = sequence.Frames
	}
	return library, nil
}
