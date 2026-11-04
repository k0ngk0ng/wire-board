package engine

import "errors"

// MatchOverlayObject names the fields owned by the original 64-byte overlay
// objects. Screen coordinates are words; these objects do not use world motion.
type MatchOverlayObject struct {
	Header                         MatchObjectHeader
	ScreenX, ScreenY               int16
	Counter, Timer, Data           uint16
	GraphicsSet, TilesBase         uint32
	Attributes                     uint16
	Layer, SkipGraphics            uint8
	GraphicOffsetX, GraphicOffsetY int16
	GraphicCache                   [4]uint32
	Parameter, Depth               uint16
}

type MatchOverlayState struct {
	ObjectID   uint16
	Object     MatchOverlayObject
	Children   [2]MatchOverlayObject // The intro manager's following +40/+80 slots.
	SoundQueue [4]uint8
	ModeFlag   uint8 // Shared A096; the demo object can change it to 3.
	SlowFlag   uint8 // Shared F55A; visible demo text holds it at 1.
}

type MatchOverlayEnvironment struct {
	PauseFlags uint8
	WinnerCode int16
	Characters [4]uint8 // Original selected character codes 1..11.
	TeamTitles [2]uint16
	RoundIndex uint16
	Blink      uint8
}

type MatchOverlayOutcome struct {
	Draw         bool
	ClearedBytes uint16
}

// StepMatchOverlay recovers intro, demo, pause/round/bracket labels and result
// callbacks to sprite submission or their silent return. Result objects persist
// at their terminal stage until the match manager clears the object region.
func StepMatchOverlay(state MatchOverlayState, world MatchOverlayEnvironment) (MatchOverlayState, MatchOverlayOutcome, error) {
	original := state
	kind := state.Object.Header.Behavior
	paused := world.PauseFlags&0x80 != 0 || state.SlowFlag != 0
	outcome := MatchOverlayOutcome{}
	object := &state.Object
	switch kind {
	case 0xfe78:
		if object.Header.Stage == 0 {
			object.ScreenX, object.ScreenY = 0x120, 0xf0
			initializeMatchGraphic(object, false)
			object.Depth, object.Parameter, object.Attributes = 256, 0x4c, 0xe050
			object.Header.Stage += 2
		}
		outcome.Draw = world.PauseFlags&0x80 != 0
	case 0xfeb2:
		switch object.Header.Stage {
		case 0:
			object.ScreenX, object.ScreenY = 0x120, 0xf0
			initializeMatchGraphic(object, false)
			object.Depth, object.Attributes = 256, 0xe050
			object.Data &= 0x00ff
			object.Header.Stage += 2
			fallthrough
		case 2:
			if int8(state.ModeFlag) <= 1 {
				break
			}
			object.Header.Stage += 2
			beginMatchDemo(&state, true)
		case 4:
			beginMatchDemo(&state, true)
		case 6, 10:
			object.Timer--
			if int16(object.Timer) < 0 {
				object.Header.Stage += 2
			}
		case 8:
			beginMatchDemo(&state, false)
		case 12:
			state.ModeFlag = 3
			object.Header.Stage += 2
		case 14:
		default:
			return original, outcome, errors.New("demo overlay stage is unavailable")
		}
		if uint8(object.Data>>8) != 0 {
			state.SlowFlag = 1
			outcome.Draw = true
		}
	case 0x10088:
		if object.Header.Stage == 0 {
			initializeMatchGraphic(object, false)
			object.Depth, object.Layer, object.ScreenX, object.ScreenY, object.Attributes = 256, 4, 0x198, 0xb8, 0xe068
			object.Header.Stage += 2
		}
		if int8(state.ModeFlag) < 0 {
			object.Parameter, outcome.Draw = 0x52, true
			break
		}
		if state.ModeFlag == 1 {
			if world.Blink&0x20 != 0 {
				break
			}
			if world.Blink&0x40 != 0 {
				object.Parameter, outcome.Draw = 0x50, true
				break
			}
		}
		// The final table value D6 refers to a zero graphic-table entry. Until
		// a valid caller/render path is established, it is reference-only.
		if world.RoundIndex > 10 {
			return original, outcome, errors.New("round label has no bounded nonzero graphic")
		}
		object.Parameter, outcome.Draw = 0xc0+world.RoundIndex*2, true
	case 0x10116, 0x10172:
		if object.Header.Stage == 0 {
			if object.Timer&1 != 0 || object.Timer > 14 {
				return original, outcome, errors.New("bracket label table offset is unavailable")
			}
			index := object.Timer / 2
			initializeMatchGraphic(object, false)
			object.Depth, object.Layer, object.ScreenY = 256, 4, 0xb8
			if kind == 0x10116 {
				object.Attributes = 0xe068
				object.ScreenX = [8]int16{0x190, 0x188, 0x188, 0x170, 0x170, 0x170, 0x1a0, 0x180}[index]
				object.Parameter = [8]uint16{0x84, 0x98, 0x82, 0x8a, 0x8e, 0x92, 0x80, 0x80}[index]
			} else {
				object.Attributes = [8]uint16{0xe070, 0xe070, 0xe070, 0xe074, 0xe074, 0xe074, 0xe070, 0xe070}[index]
				object.ScreenX = [8]int16{0x1a8, 0x1a8, 0x1a8, 0x1a8, 0x1a8, 0x1a8, 0x198, 0x190}[index]
				object.Parameter = [8]uint16{0x9a, 0x9c, 0x9e, 0xa0, 0x9a, 0x9c, 0x96, 0x80}[index]
			}
			object.Header.Stage += 2
		}
		outcome.Draw = true
	case 0x101f2:
		if paused {
			return state, outcome, nil
		}
		switch object.Header.Stage {
		case 0:
			object.Timer = 72
			object.Header.Stage += 2
			fallthrough
		case 2:
			object.Timer--
			if int16(object.Timer) < 0 {
				object.Header.Stage += 2
			}
		case 4, 8:
			second := object.Header.Stage == 8
			sound, first := uint8(0x58), uint16(2)
			if second {
				sound, first = 0x59, 6
			}
			overlaySound(&state.SoundQueue, sound)
			for i := range state.Children {
				child := &state.Children[i]
				child.Header = MatchObjectHeader{Behavior: 0x10330, Size: 64}
				child.ScreenX, child.ScreenY = int16(0x100+i*0x40), 0xd0
				child.Parameter, child.Attributes, child.Timer = first+uint16(i*2), 0xe25c+uint16(i*0x80), 80
				child.Data &= 0x00ff // CLR/MOVE.B at +1a retains its following byte.
				if second {
					child.Data |= 0x0100
				}
			}
			object.Header.Stage += 2
			// Original falls through to the wait, which sees its new first child.
		case 6, 10:
			if state.Children[0].Header.Behavior == 0 {
				object.Header.Stage += 2
			}
		case 12:
			return clearMatchOverlay(state, false)
		default:
			return original, outcome, errors.New("intro stage is unavailable")
		}
	case 0x10330:
		if !paused {
			switch object.Header.Stage {
			case 0:
				initializeMatchGraphic(object, true)
				object.Depth = 0
				object.Header.Stage += 2
				fallthrough
			case 2:
				growMatchOverlay(object)
			case 4:
				object.Header.Stage += 2
				fallthrough
			case 6:
				object.Timer--
				if int16(object.Timer) < 0 {
					object.Header.Stage += 2
				}
			case 8:
				if uint8(object.Data>>8) != 0 {
					return clearMatchOverlay(state, true)
				}
				object.Header.Stage += 2
				fallthrough
			case 10:
				object.Depth -= 32
				if int16(object.Depth) <= 0 {
					object.Depth = 0
					object.Header.Stage += 2
				}
			case 12:
				return clearMatchOverlay(state, true)
			default:
				return original, outcome, errors.New("intro part stage is unavailable")
			}
		}
		outcome.Draw = true
	case 0x10436, 0x104ac:
		if !paused {
			switch object.Header.Stage {
			case 0:
				sound, x, parameter := uint8(0x5c), int16(0x120), uint16(0x48)
				if kind == 0x104ac {
					sound, x, parameter = 0x5d, 0x108, 0x4a
				}
				overlaySound(&state.SoundQueue, sound)
				object.ScreenX, object.ScreenY = x, 0xe0
				initializeMatchGraphic(object, false)
				object.Depth, object.Parameter, object.Attributes = 0, parameter, 0xe050
				object.Header.Stage += 2
				fallthrough
			case 2:
				growMatchOverlay(object)
			case 4:
				object.Header.Stage += 2
			case 6:
			default:
				return original, outcome, errors.New("result title stage is unavailable")
			}
		}
		outcome.Draw = true
	case 0x10522, 0x10616, 0x106e8:
		if !paused {
			switch object.Header.Stage {
			case 0:
				object.Parameter, object.Timer = 0, 96
				object.Header.Stage += 2
				fallthrough
			case 2:
				object.Timer--
				if int16(object.Timer) < 0 {
					object.Header.Stage += 2
				}
			case 4:
				if kind == 0x10616 {
					if err := chooseMatchTeamTitle(&state, world); err != nil {
						return original, outcome, err
					}
				}
				x, attribute := int16(0x138), uint16(0xe058)
				if kind == 0x10616 {
					x = 0x128
				} else if kind == 0x106e8 {
					x, attribute = 0x148, 0xe05c
				}
				object.ScreenX, object.ScreenY = x, 0xe0
				initializeMatchGraphic(object, false)
				object.Depth, object.Attributes = 0, attribute
				if kind == 0x10522 {
					if err := chooseMatchCharacterTitle(&state, world); err != nil {
						return original, outcome, err
					}
				} else if kind == 0x106e8 {
					object.Parameter = 0x2c
				}
				object.Header.Stage += 2
				fallthrough
			case 6:
				growMatchOverlay(object)
			case 8:
				if kind == 0x10522 {
					object.Counter = 0
				} else if kind == 0x106e8 {
					object.Timer = 24
				}
				object.Header.Stage += 2
				fallthrough
			case 10:
				if kind == 0x10522 {
					object.Counter++
					if object.Counter == 32 {
						if object.Data == 9 {
							overlaySound(&state.SoundQueue, 0x6e)
						} else if object.Data == 11 {
							overlaySound(&state.SoundQueue, 0x6d)
						}
					}
				} else if kind == 0x106e8 {
					object.Timer--
					if object.Timer == 0 {
						overlaySound(&state.SoundQueue, 0x66)
					}
				}
			default:
				return original, outcome, errors.New("result name stage is unavailable")
			}
		}
		outcome.Draw = object.Parameter != 0
	default:
		return original, outcome, errors.New("match overlay callback is unavailable")
	}
	return state, outcome, nil
}

func beginMatchDemo(state *MatchOverlayState, first bool) {
	parameter, sound, timer := uint16(0x46), uint8(0x5b), uint16(96)
	if first {
		parameter, sound, timer = 0x44, 0x5a, 64
		state.Object.Data = state.Object.Data&0x00ff | 0x0100
	}
	state.Object.Parameter, state.Object.Timer = parameter, timer
	overlaySound(&state.SoundQueue, sound)
	state.Object.Header.Stage += 2
	state.Object.Timer--
	if int16(state.Object.Timer) < 0 {
		state.Object.Header.Stage += 2
	}
}

func growMatchOverlay(object *MatchOverlayObject) {
	object.Depth += 32
	if int16(object.Depth) >= 256 {
		object.Depth = 256
		object.Header.Stage += 2
	}
}

func initializeMatchGraphic(object *MatchOverlayObject, intro bool) {
	object.TilesBase, object.GraphicsSet = 0x24b9e0, 0xda1da
	if intro {
		object.TilesBase, object.GraphicsSet = 0x25d260, 0xdaa4a
	}
	object.GraphicOffsetX, object.GraphicOffsetY = 0, 0
	object.SkipGraphics, object.Layer = 0, 0
	object.GraphicCache = [4]uint32{}
	object.Depth = 256
}

func chooseMatchCharacterTitle(state *MatchOverlayState, world MatchOverlayEnvironment) error {
	if world.WinnerCode < 1 || world.WinnerCode > 4 {
		return errors.New("result winner character slot is unavailable")
	}
	character := world.Characters[world.WinnerCode-1]
	if character < 1 || character > 11 {
		return errors.New("result character title is unavailable")
	}
	state.Object.Data = uint16(character)
	state.Object.Parameter = uint16(character) * 2
	sounds := [11]uint8{0x6b, 0x69, 0x68, 0x67, 0x6a, 0x71, 0x70, 0x72, 0x6c, 0x6f, 0x6c}
	overlaySound(&state.SoundQueue, sounds[character-1])
	return nil
}

func chooseMatchTeamTitle(state *MatchOverlayState, world MatchOverlayEnvironment) error {
	index := 0
	if uint16(world.WinnerCode)>>2 != 0 {
		index = 1
	}
	title := world.TeamTitles[index]
	if title < 1 || title > 8 {
		return errors.New("result team title is unavailable")
	}
	state.Object.Parameter = 0x1c + (title-1)*2
	overlaySound(&state.SoundQueue, uint8(0x5e+title-1))
	return nil
}

func overlaySound(queue *[4]uint8, request uint8) {
	for i, value := range queue {
		if value < 128 || value == request {
			queue[i] = request
			break
		}
	}
}

func clearMatchOverlay(state MatchOverlayState, draw bool) (MatchOverlayState, MatchOverlayOutcome, error) {
	before := state
	size := state.Object.Header.Size
	if size == 0 {
		size = 64
	}
	size = size >> 4 << 4
	if size == 0 || size > 64 {
		return before, MatchOverlayOutcome{}, errors.New("overlay clear prefix is outside its object")
	}
	o := &state.Object
	o.Header, o.ScreenX, o.ScreenY = MatchObjectHeader{}, 0, 0
	if size >= 32 {
		o.Counter, o.Timer, o.Data, o.GraphicsSet = 0, 0, 0, 0
	}
	if size >= 48 {
		o.TilesBase, o.Attributes, o.Layer, o.SkipGraphics = 0, 0, 0, 0
		o.GraphicOffsetX, o.GraphicOffsetY, o.GraphicCache[0] = 0, 0, 0
	}
	if size >= 64 {
		o.GraphicCache, o.Parameter, o.Depth = [4]uint32{}, 0, 0
	}
	return state, MatchOverlayOutcome{Draw: draw, ClearedBytes: size}, nil
}

type MatchShadowState struct {
	Object                       MatchOverlayObject
	X, Y                         int32
	Cursor                       uint16
	ScreenOffsetX, ScreenOffsetY int16
	Registration                 FighterRegistrationState
}

type MatchShadowEnvironment struct {
	AttributeMask           uint16
	CameraX, CameraY, Shake int16
}

type MatchShadowOwner struct {
	Depth     uint16
	X, Ground int32
}

// StepMatchShadow recovers 0x14882. It renders just one registered owner per
// call, rotating through the available registration list and clearing Used.
// The original 9c00 object runs after the fighters in the same object round.
func StepMatchShadow(state MatchShadowState, world MatchShadowEnvironment, owners map[uint16]MatchShadowOwner) (MatchShadowState, MatchOverlayOutcome, error) {
	before := state
	if state.Object.Header.Stage == 0 {
		state.Object.GraphicsSet = 0x27f618
		state.Object.SkipGraphics = 1
		state.Object.GraphicOffsetX, state.Object.GraphicOffsetY = 0, 0
		state.Object.Layer, state.Object.Parameter = 0xf4, 0x26
		state.Object.GraphicCache = [4]uint32{}
		state.Object.Attributes = 0x21a0 | world.AttributeMask
		state.ScreenOffsetX, state.ScreenOffsetY, state.Cursor = 0, -3, 0
		state.Registration.Used = 0
		state.Object.Header.Stage += 2
	}
	if state.Registration.Used == 0 {
		return state, MatchOverlayOutcome{}, nil
	}
	if int16(state.Registration.Used) <= int16(state.Cursor) {
		state.Cursor = 0
	}
	index := int(int16(state.Cursor)) / 2
	if state.Cursor&1 != 0 || int16(state.Cursor) < 0 || index >= len(state.Registration.Owners) {
		return before, MatchOverlayOutcome{}, errors.New("shadow registration cursor is unavailable")
	}
	owner, ok := owners[state.Registration.Owners[index]]
	if !ok {
		return before, MatchOverlayOutcome{}, errors.New("shadow owner is unavailable")
	}
	state.Object.Depth, state.X, state.Y = owner.Depth, owner.X, owner.Ground
	state.Cursor += 2
	state.Registration.Used = 0
	projected := ProjectFighter(ProjectionState{WorldX: state.X, WorldY: state.Y, ScreenX: state.Object.ScreenX, ScreenY: state.Object.ScreenY,
		OffsetX: state.ScreenOffsetX, OffsetY: state.ScreenOffsetY, CameraX: world.CameraX, CameraY: world.CameraY, Shake: world.Shake})
	state.Object.ScreenX, state.Object.ScreenY, state.ScreenOffsetX, state.ScreenOffsetY = projected.ScreenX, projected.ScreenY, projected.OffsetX, projected.OffsetY
	return state, MatchOverlayOutcome{Draw: true}, nil
}

type MatchHealthTarget struct {
	Behavior uint32
	Health   int16
}

type MatchTrainingState struct {
	Targets [4]MatchHealthTarget
}

// StepMatchTraining recovers 0xff74's paused training-only health controls.
// It is not an AI shortcut or a generic heal action.
func StepMatchTraining(state MatchTrainingState, pause uint8, held, extended [4]uint8) MatchTrainingState {
	if pause&0x80 == 0 {
		return state
	}
	var base, extra uint8
	for i := range held {
		base, extra = base|held[i], extra|extended[i]
	}
	value := int16(0)
	if extra&1 != 0 {
		value = 223
	} else if extra&2 == 0 {
		return state
	}
	for i, mask := range [4]uint8{4, 1, 2, 8} {
		target := &state.Targets[i]
		if base&mask != 0 && target.Behavior == 0x12668 && (value != 0 || target.Health >= 0) {
			target.Health = value
		}
	}
	return state
}
