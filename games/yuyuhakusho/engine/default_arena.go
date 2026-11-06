package engine

import (
	"errors"
	"math/bits"
)

type DefaultArenaCamera struct {
	Used                                uint16
	Left, Right, X, Y, TargetX, TargetY int16
	SamplesX, SamplesY                  [16]int32
}

// StepDefaultArenaCamera consumes the preceding actor round's position samples.
// Both sums wrap as words before unsigned division; widening or using signed
// averages changes extreme inputs. With no samples all camera state is held.
func StepDefaultArenaCamera(state DefaultArenaCamera) (DefaultArenaCamera, error) {
	return StepDefaultArenaCameraObserved(state, nil)
}

// StepDefaultArenaCameraObserved exposes work without retaining its observer.
func StepDefaultArenaCameraObserved(state DefaultArenaCamera, hook ArenaWorkHook) (DefaultArenaCamera, error) {
	work := arenaWorkEmitter{hook: hook}
	next, err := stepDefaultArenaCameraWork(state, &work)
	if work.err != nil {
		return state, work.err
	}
	return next, err
}

func stepDefaultArenaCameraWork(state DefaultArenaCamera, work *arenaWorkEmitter) (DefaultArenaCamera, error) {
	if state.Used > 64 || state.Used&3 != 0 {
		return state, errors.New("invalid arena camera sample offset")
	}
	work.spend("camera-entry", 12)
	work.branch("camera-empty", state.Used == 0, true)
	if state.Used == 0 {
		work.spend("camera-return", 16)
		return state, nil
	}
	work.spend("camera-samples", 4, 8, 8, 4, 4)
	count := state.Used / 4
	var x, y uint16
	for i := int(count) - 1; i >= 0; i-- {
		work.spend("camera-sample", 14, 14, 4)
		x += uint16(state.SamplesX[i] >> 16)
		y += uint16(state.SamplesY[i] >> 16)
		work.branch("camera-sample-loop", i != 0, false)
	}
	work.spend("camera-count", 10)
	work.division(x, count)
	work.spend("camera-target-x", 8)
	work.division(y, count)
	work.spend("camera-target-y", 8, 10, 16, 12, 16)
	tx := int16(x/count) - 160
	ty := (int16(y/count) - 192) >> 2
	work.branch("camera-left-bound", tx >= 744, false)
	if tx < 744 {
		work.spend("camera-left-clamp", 16, 12)
		tx = 744
	} else {
		work.spend("camera-right-bound", 16)
		work.branch("camera-right-branch", tx <= 984, false)
		if tx > 984 {
			work.spend("camera-right-clamp", 16)
			tx = 984
		}
	}
	work.spend("camera-top-bound", 4)
	work.branch("camera-top-branch", ty >= 0, false)
	if ty < 0 {
		work.spend("camera-top-clamp", 4)
		ty = 0
	}
	work.spend("camera-bottom-bound", 8)
	work.branch("camera-bottom-branch", ty <= 32, false)
	if ty > 32 {
		work.spend("camera-bottom-clamp", 12)
		ty = 32
	}
	work.spend("camera-smooth", 12, 12, 12, 12, 16, 12, 8, 12, 8, 12, 12, 12, 10, 16)
	work.spend("camera-return", 16)
	state.Used, state.TargetX, state.TargetY = 0, tx, ty
	state.X += (tx - state.X) >> 3
	state.Left = state.X + 16
	state.Right = state.Left + 288
	state.Y += (ty - state.Y) >> 2
	return state, nil
}

type ArenaTimedFrame struct {
	Parameter uint16 `json:"parameter"`
	Ticks     uint8  `json:"ticks"`
}
type ArenaRuntimeLibrary struct {
	WaveX         []int8            `json:"wave_x"`
	WaveY         []int8            `json:"wave_y"`
	Tails         map[int16][]int16 `json:"tails"`
	ColorTargets  [][]uint16        `json:"color_targets"`
	FadeAdjust    [][]int16         `json:"fade_adjust"`
	SparkSequence []ArenaTimedFrame `json:"spark_sequence"`
}
type DefaultArenaView struct {
	PauseFlags, SlowFlag uint8
	FrameCounter         uint8 // The F589 low byte of Clock.LogicFrame.
	Shake, Pan           int16
	// E600..E7FF is shared with the HUD. It is a read-only view supplied by
	// the world, not a second independently maintained copy of those cells.
	HUDPrefix [256]int16
}
type DefaultArenaState struct {
	Stage            uint16
	Offsets          [3]int16
	Drifts           [2]int32
	Phases           [2]uint16
	Camera           DefaultArenaCamera
	Mode             uint8
	Vertical         [2]int16
	RowsA, RowsB     [256]int16
	PanA, PanB       [224]int16
	SourceA, SourceB int32
}

// StepDefaultArena recovers the complete default callback 20918 including
// camera consumption, wave/parallax rows and feedback copies. It does not
// encode VDP commands or draw pixels. Row banks remain presentation data.
func StepDefaultArena(state DefaultArenaState, view DefaultArenaView, library ArenaRuntimeLibrary) (DefaultArenaState, error) {
	return StepDefaultArenaObserved(state, view, library, nil)
}

// StepDefaultArenaObserved emits internal work boundaries in the actual rule
// path. Work can interleave sound, whose owners are independent of these pure
// camera/scroll values. Any work error returns the original complete state.
func StepDefaultArenaObserved(state DefaultArenaState, view DefaultArenaView, library ArenaRuntimeLibrary, hook ArenaWorkHook) (DefaultArenaState, error) {
	work := arenaWorkEmitter{hook: hook}
	next, err := stepDefaultArenaWork(state, view, library, &work)
	if work.err != nil {
		return state, work.err
	}
	return next, err
}
func stepDefaultArenaWork(state DefaultArenaState, view DefaultArenaView, library ArenaRuntimeLibrary, work *arenaWorkEmitter) (DefaultArenaState, error) {
	work.spend("arena-pause", 12)
	work.branch("arena-pause-branch", view.PauseFlags&128 != 0, false)
	if view.PauseFlags&128 != 0 {
		work.spend("arena-return", 16)
		return state, nil
	}
	work.spend("arena-slow", 12)
	work.branch("arena-slow-branch", view.SlowFlag != 0, false)
	if view.SlowFlag != 0 {
		work.spend("arena-return", 16)
		return state, nil
	}
	work.spend("arena-body", 18, 12)
	work.branch("arena-initialized", state.Stage != 0, false)
	before := state
	if state.Stage == 0 {
		work.spend("arena-initialize", 16, 24, 24, 16, 16, 16)
		state.Mode = 3
		state.Drifts = [2]int32{}
		state.Phases = [2]uint16{}
		state.Stage += 2
	}
	var err error
	work.spend("arena-camera", 18)
	state.Camera, err = stepDefaultArenaCameraWork(state.Camera, work)
	if err != nil {
		return before, err
	}
	if len(library.WaveX) != 1024 {
		return before, errors.New("arena waveform is unavailable")
	}
	tail, ok := library.Tails[state.Camera.X]
	if !ok || len(tail) != 56 {
		return before, errors.New("arena parallax tail is unavailable")
	}
	work.spend("arena-vertical", 12, 12, 4, 12, 12, 12, 12, 4, 4, 8, 4, 16, 8, 4, 16, 12)
	work.branch("arena-drift-held", view.Pan != 0, false)
	dy := state.Camera.Y - view.Shake
	state.Vertical = [2]int16{dy + 8, dy + 8}
	state.SourceA, state.SourceB = int32(dy*2-0x1800), int32(dy*2-0x1600)
	if view.Pan == 0 {
		work.spend("arena-drift", 32, 32, 20, 20)
		state.Drifts[0] += 0x2000
		state.Drifts[1] += 0x4000
		state.Phases[0] += 8
		state.Phases[1] += 8
	}
	work.spend("arena-offsets", 12, 4, 8, 10, 4, 8, 4, 12, 12, 10, 8, 4, 12, 12, 12, 8, 4, 12)
	x := state.Camera.X
	state.Offsets[0] = -((x >> 1) + (x >> 2) - 0x228)
	state.Offsets[1] = -((x >> 2) - 0x78)
	state.Offsets[2] = -((x >> 3) - 0x0c)
	work.spend("arena-near-setup", 8, 12, 4, 4, 4, 8)
	work.rows("arena-near-fill", 16, 12)
	for i := 80; i < 208; i++ {
		state.RowsA[i] = state.Offsets[1]
	}
	work.spend("arena-front-setup", 12, 4, 4, 4, 8)
	work.rows("arena-front-fill", 4, 12)
	for i := 208; i < 240; i++ {
		state.RowsA[i] = state.Offsets[0]
	}
	work.spend("arena-far-setup", 8, 12, 4, 4, 4, 8)
	work.rows("arena-far-fill", 8, 12)
	for i := 16; i < 80; i++ {
		state.RowsB[i] = state.Offsets[1]
	}
	for group := 0; group < 2; group++ {
		work.spend("arena-wave-setup", 12, 12, 12, 8, 8, 12, 8)
		base := state.Offsets[2] + int16(state.Drifts[group]>>16)
		phase := state.Phases[group]
		for i := 0; i < 16; i++ {
			shiftCost := uint16(14)
			if group != 0 {
				shiftCost = 12
			}
			work.spend("arena-wave-row", 4, 14, 4, shiftCost, 4, 8, 4)
			work.loop("arena-wave-loop", i < 15)
			wave := int16(library.WaveX[(phase&0x7fe)/2])
			if group == 0 {
				wave >>= 4
			} else {
				wave >>= 3
			}
			state.RowsB[80+group*16+i] = base + wave
			phase += 0x300
		}
	}
	work.spend("arena-lower-setup", 12, 12, 4, 4, 4, 12)
	work.rows("arena-lower-fill", 9, 12)
	for i := 112; i < 184; i++ {
		state.RowsB[i] = state.Offsets[2] + int16(state.Drifts[1]>>16)
	}
	work.spend("arena-tail-setup", 12, 8, 4, 24, 16, 12, 8, 8)
	work.rows("arena-tail-copy", 7, 20)
	copy(state.RowsB[184:240], tail)
	work.spend("arena-body-return", 16, 10)
	work.spend("arena-pan", 12)
	work.branch("arena-pan-branch", view.Pan == 0, false)
	if view.Pan != 0 {
		// The original copies 224 rows from the signed camera/feedback
		// selection. Negative selections can begin in the current HUD bank.
		if dy < -256 || dy > 32 {
			return before, errors.New("arena feedback row view is unavailable")
		}
		work.spend("arena-pan-setup", 16, 16, 12, 12, 16, 16, 12, 8)
		get := func(index int) int16 {
			if index < 256 {
				return view.HUDPrefix[index]
			}
			if index < 512 {
				return state.RowsA[index-256]
			}
			return state.RowsB[index-512]
		}
		for i := 0; i < 224; i++ {
			work.spend("arena-pan-row", 8, 4, 8, 8, 4, 8)
			if i&3 == 3 {
				work.loop("arena-pan-loop", i < 223)
			}
			state.PanA[i] = get(256+int(dy)+i) + view.Pan
			state.PanB[i] = get(512+int(dy)+i) + view.Pan
		}
		state.SourceA, state.SourceB = -0xa000, -0x9e00
	}
	work.spend("arena-return", 16)
	return state, nil
}

// ArenaPaletteState retains all three actual color banks. F500 Flags is
// distinct from F501 Status; both low bits and signed completion matter.
type ArenaPaletteState struct {
	Flags, Status               uint8
	Mode, Step, Target          uint16
	Current, Primary, Secondary [64]uint16
}

// AdvancePaletteFade recovers 2300. Completion clears Status.bit7; zero-step
// cleanup only clears Flags.bit0 when Status.bit0 was previously set.
func AdvancePaletteFade(state ArenaPaletteState) ArenaPaletteState {
	if state.Step == 0 {
		old := state.Status
		state.Status &^= 1
		if old&1 != 0 {
			state.Mode = 0
			state.Flags &^= 1
		}
		return state
	}
	value := state.Mode + state.Step
	if value < state.Target {
		state.Mode = value
	} else {
		state.Mode = state.Target
		state.Step = 0
		state.Status &^= 128
	}
	return state
}

// ApplyPaletteFade recovers 233c, including byte-only mode selection and
// source odd/reserved color bits. The frame caller owns the F500.bit0 gate.
func ApplyPaletteFade(state ArenaPaletteState, library ArenaRuntimeLibrary) (ArenaPaletteState, error) {
	if state.Status&2 != 0 {
		return state, nil
	}
	a, b := state.Primary, state.Secondary
	if state.Status&1 != 0 {
		a, b = b, a
	}
	// MOVE.B from the word's first address reads its high byte on 68000.
	mode := uint8(state.Mode >> 8)
	if mode == 0 {
		state.Current = b
		return state, nil
	}
	if mode == uint8(state.Target>>8) {
		state.Current = a
		return state, nil
	}
	row := int(mode&14) / 2
	if len(library.FadeAdjust) != 8 || len(library.FadeAdjust[row]) != 15 {
		return state, errors.New("palette interpolation table is unavailable")
	}
	for i, source := range a {
		if source == state.Current[i] {
			continue
		}
		target := b[i]
		for component := 0; component < 3; component++ {
			delta := int(target&14) - int(source&14)
			if delta != 0 {
				source += uint16(library.FadeAdjust[row][delta/2+7])
			}
			if component < 2 {
				source = bits.RotateLeft16(source, -4)
				target = bits.RotateLeft16(target, -4)
			}
		}
		state.Current[i] = bits.RotateLeft16(source, 8)
	}
	return state, nil
}

type ArenaSkyKind uint8

const (
	ArenaSun ArenaSkyKind = iota
	ArenaMoon
	ArenaSpark
)

type ArenaSkyState struct {
	Kind                           ArenaSkyKind
	ObjectID                       uint16
	Behavior                       uint32
	Size, Stage                    uint16
	ScreenX, ScreenY               int16
	ScreenXFraction                uint16
	GraphicsSet, TilesBase         uint32
	Attributes                     uint16
	Layer                          uint8
	GraphicOffsetX, GraphicOffsetY int16
	Cache                          [4]uint32
	Parameter, Depth               uint16
	X, Y                           int32
	Phase, Sequence                uint32
	Index, Ticks, Flags            uint8
}
type ArenaSkyWorld struct {
	PauseFlags, SlowFlag uint8
	CameraY, Pan         int16
	PaletteStatus        uint8
	Primary              [64]uint16
	RandomSeed           uint32
	Next                 MatchObjectHeader
}
type ArenaSkyOutcome struct {
	Draw         bool
	ClearedBytes uint16
	Spawned      bool
}

// StepArenaSky recovers 150ca,1525e and the directly spawned15330 rule stages
// through draw1580 dispatch or completed3556 deletion. It never emits sprites.
func StepArenaSky(state ArenaSkyState, world ArenaSkyWorld, library ArenaRuntimeLibrary) (ArenaSkyState, ArenaSkyWorld, ArenaSkyOutcome, error) {
	before, oldWorld := state, world
	out := ArenaSkyOutcome{}
	fail := func(message string) (ArenaSkyState, ArenaSkyWorld, ArenaSkyOutcome, error) {
		return before, oldWorld, ArenaSkyOutcome{}, errors.New(message)
	}
	if state.Kind > ArenaSpark {
		return fail("unsupported arena sky object")
	}
	active := world.PauseFlags&128 == 0 && world.SlowFlag == 0
	initialize := func(attributes uint16, layer uint8) {
		state.GraphicsSet, state.TilesBase = 0xda16a, 0x24b620
		state.Attributes, state.Layer = attributes, layer
		state.GraphicOffsetX, state.GraphicOffsetY = 0, 0
		state.Cache = [4]uint32{}
		state.Parameter, state.Depth = 0, 256
		state.Stage += 2
	}
	if state.Kind == ArenaSun {
		if !active {
			out.Draw = true
			return state, world, out, nil
		}
		if state.Stage == 0 {
			initialize(0x45fc, 248)
			state.Phase = 0x03000000
		}
		state.Phase = state.Phase&0xffff | uint32(uint16(state.Phase>>16)&1023)<<16
	} else if state.Kind == ArenaMoon {
		if state.Stage == 0 {
			initialize(0x45f8, 248)
			state.Phase = state.Phase&0xffff | 0x01000000
		}
		if !active {
			out.Draw = true
			return state, world, out, nil
		}
		phase := uint16(state.Phase >> 16)
		if phase > 1023 {
			return fail("persistent moon phase is outside decoded waveform")
		}
		if world.Pan == 0 && phase >= 384 && phase <= 640 {
			var random uint32
			world.RandomSeed, random = NextRandom(world.RandomSeed)
			if random&255 == 0 && world.Next.Size == 0 {
				world.Next = MatchObjectHeader{Behavior: 0x15330, Size: 128}
				out.Spawned = true
			}
		}
	} else {
		if state.Stage == 0 {
			initialize(0x45f4, 252)
			state.Sequence = 0x15432
			state.Index, state.Ticks, state.Flags = 255, 1, 0
			var random uint32
			world.RandomSeed, random = NextRandom(world.RandomSeed)
			state.X = int32(uint32((random&127)+224)<<16 | uint32(uint16(state.X)))
			world.RandomSeed, random = NextRandom(world.RandomSeed)
			state.Y = int32(uint32((random&31)+128)<<16 | uint32(uint16(state.Y)))
		}
		if !active {
			out.Draw = true
			return state, world, out, nil
		}
		state.Ticks--
		if state.Ticks == 0 {
			state.Index++
			if state.Sequence != 0x15432 || int(state.Index) >= len(library.SparkSequence) {
				return fail("arena particle sequence is unavailable")
			}
			frame := library.SparkSequence[state.Index]
			state.Parameter, state.Ticks = frame.Parameter, frame.Ticks
			if state.Ticks == 0 {
				size := state.Size
				if size == 0 {
					size = 64
				}
				cleared := size >> 4 << 4
				if cleared == 0 || cleared > 128 {
					return fail("arena particle deletion is outside its slot")
				}
				clearArenaSky(&state, cleared)
				out.ClearedBytes = cleared
				return state, world, out, nil
			}
		}
		state.X -= 0x18000
		state.Y += 0x18000
		state.ScreenX, state.ScreenXFraction, state.ScreenY = int16(state.X>>16), uint16(state.X), int16(state.Y>>16)-world.CameraY
		out.Draw = true
		return state, world, out, nil
	}
	if len(library.WaveX) != 1024 || len(library.WaveY) != 1024 {
		return fail("arena orbit waveform is unavailable")
	}
	phase := uint16(state.Phase >> 16)
	state.ScreenX = (int16(library.WaveX[phase]) + 288) & 511
	state.ScreenX += world.Pan
	state.ScreenY = (int16(library.WaveY[phase]) - world.CameraY + 240) & 511
	if world.Pan == 0 {
		if state.Kind == ArenaSun && world.PaletteStatus == 0 && state.Phase&0xfffff == 0 {
			index := 3
			if phase <= 896 && phase > 640 {
				index = 0
			} else if phase <= 640 && phase > 384 {
				index = 1
			} else if phase <= 384 && phase > 256 {
				index = 2
			}
			if len(library.ColorTargets) != 4 || len(library.ColorTargets[index]) != 16 {
				return fail("arena palette target is unavailable")
			}
			for i, target := range library.ColorTargets[index] {
				color := world.Primary[i] & 0xeee
				target &= 0xeee
				for _, component := range []struct{ mask, step uint16 }{{0xe00, 0x200}, {0xe0, 0x20}, {14, 2}} {
					a, b := target&component.mask, color&component.mask
					if a > b {
						color += component.step
					} else if a < b {
						color -= component.step
					}
				}
				world.Primary[i] = color
			}
		}
		state.Phase -= 0x2000
		if state.Kind == ArenaMoon {
			state.Phase &= 0x3ffffff
		}
	}
	out.Draw = true
	return state, world, out, nil
}

func clearArenaSky(state *ArenaSkyState, prefix uint16) {
	if prefix > 0 {
		state.Behavior, state.Size, state.Stage = 0, 0, 0
	}
	if prefix > 8 {
		state.ScreenX, state.ScreenXFraction, state.ScreenY = 0, 0, 0
	}
	if prefix > 28 {
		state.GraphicsSet = 0
	}
	if prefix > 32 {
		state.TilesBase, state.Attributes, state.Layer = 0, 0, 0
	}
	if prefix > 40 {
		state.GraphicOffsetX, state.GraphicOffsetY = 0, 0
	}
	for i := range state.Cache {
		if int(prefix) > 44+i*4 {
			state.Cache[i] = 0
		}
	}
	if prefix > 60 {
		state.Parameter, state.Depth = 0, 0
	}
	if prefix > 64 {
		state.X, state.Y, state.Phase = 0, 0, 0
	}
	if prefix > 80 {
		state.Sequence, state.Index, state.Ticks, state.Flags = 0, 0, 0, 0
	}
}
