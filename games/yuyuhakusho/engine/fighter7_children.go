package engine

import "errors"

// These contexts are temporary views of existing canonical owners. In
// particular Link's +10/+14 are the second child's target coordinates, and
// EffectExtra.Delay is the generic FX clock's existing +5a byte.
type Fighter7ChildParent struct {
	ObjectID uint16
	Pose     uint8
}

type Fighter7ChildLink struct {
	ObjectID                           uint16
	VelocityX, VerticalVelocity, Aux48 int32
}

type Fighter7HealingTarget struct {
	ObjectID, Size uint16
	Health         int16
	X, Y           int32
}

type Fighter7ChildSubmission struct {
	Buffers      [4]AnimationShapeBuffer
	PendingSound uint8
	SoundQueue   [4]uint8
}

type Fighter7ChildEffectExtra struct {
	VerticalVelocity int32
	Delay            uint8
}

type Fighter7FirstChildContext struct {
	Parent     Fighter7ChildParent
	Link       Fighter7ChildLink
	Submission Fighter7ChildSubmission
}

type Fighter7SecondChildContext struct {
	Parent       Fighter7ChildParent
	Fighters     [4]Fighter7HealingTarget
	Pool         EffectPool
	EffectExtras [16]Fighter7ChildEffectExtra
	Submission   Fighter7ChildSubmission
}

// StepFighter7FirstChild recovers136e6 through its own draw or completed clear.
// The main entry itself selects the137f4 hit-stop continuation. It writes its
// sibling's target before the ground clamp and even if that sibling is empty.
func StepFighter7FirstChild(object WorldObject, context Fighter7FirstChildContext, environment ChuProjectileEnvironment, library AnimationLibrary) (WorldObject, Fighter7FirstChildContext, ChuProjectileOutcome, error) {
	return stepFighter7FirstChildWithSound(object, context, environment, library, nil)
}

func stepFighter7FirstChildWithSound(object WorldObject, context Fighter7FirstChildContext, environment ChuProjectileEnvironment, library AnimationLibrary, soundPort *worldSoundPort) (WorldObject, Fighter7FirstChildContext, ChuProjectileOutcome, error) {
	original, oldContext := object, context
	fail := func(err error) (WorldObject, Fighter7FirstChildContext, ChuProjectileOutcome, error) {
		return original, oldContext, ChuProjectileOutcome{}, err
	}
	if object.Header.Behavior != 0x136e6 {
		return fail(errors.New("source7 first child callback is unavailable"))
	}
	initializeFighter7Child(&object, environment.PaletteAttributes, 0x3e)
	object.Body.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return object, context, ChuProjectileOutcome{Draw: true}, nil
	}
	if object.HitStopTicks != 0 {
		stepFighter7ChildHitStop(&object)
	} else {
		object.Shake = 0
		drift := StepResidualMotion(ResidualMotion{Position: object.Body.X, Residual: object.Residual, Facing: object.Body.Facing})
		object.Body.X, object.Residual = drift.Position, drift.Residual
		object.Body.Clock = StepAnimationClock(object.Body.Clock)
		common := true
		switch object.Body.ActionKind {
		case 0:
			if object.Body.Clock.Completed == 1 {
				resetFighter7Child(&object, 0x41, 2)
				common = false
			} else {
				if context.Parent.ObjectID != object.ObjectID-0x100 {
					return fail(errors.New("source7 first child parent identity mismatch"))
				}
				if context.Parent.Pose != 0xa7 {
					resetFighter7Child(&object, 0x3f, 1)
					common = false
				}
			}
		case 1:
			if object.Body.Clock.Completed == 1 {
				next, out, err := clearFighter7Child(object)
				if err != nil {
					return fail(err)
				}
				return next, context, out, nil
			}
		case 2:
			if context.Parent.ObjectID != object.ObjectID-0x100 {
				return fail(errors.New("source7 first child parent identity mismatch"))
			}
			if context.Parent.Pose != 0xa7 {
				resetFighter7Child(&object, 0x3f, 1)
				common = false
			} else if object.Controls.Held&ButtonUp != 0 && object.Controls.Pressed&ButtonA != 0 {
				resetFighter7Child(&object, 0x43, 0)
				common = false
			} else {
				if object.Controls.Held&ButtonUp != 0 {
					object.Body.Y -= 0x20000
				}
				if object.Controls.Held&ButtonDown != 0 {
					object.Body.Y += 0x20000
				}
				if object.Controls.Held&ButtonLeft != 0 {
					object.Body.X -= 0x20000
					object.Body.Facing = 1
				}
				if object.Controls.Held&ButtonRight != 0 {
					object.Body.X += 0x20000
					object.Body.Facing = 0
				}
			}
		default:
			return fail(errors.New("source7 first child action is unavailable"))
		}
		if common {
			if context.Link.ObjectID != object.ObjectID+0x100 {
				return fail(errors.New("source7 first child sibling identity mismatch"))
			}
			context.Link.Aux48 = int32(uint32(context.Link.Aux48)&0x00ffffff | uint32(object.Body.Facing)<<24)
			context.Link.VelocityX, context.Link.VerticalVelocity = object.Body.X, object.Body.Y
			if fighter7ChildOutside(&object, environment) {
				next, out, err := clearFighter7Child(object)
				if err != nil {
					return fail(err)
				}
				return next, context, out, nil
			}
		}
	}
	var err error
	object, context.Submission, err = submitFighter7Child(object, context.Submission, environment, library, soundPort)
	if err != nil {
		return fail(err)
	}
	return object, context, ChuProjectileOutcome{Draw: true}, nil
}

// StepFighter7SecondChild recovers1391e, including the reachable13a08
// continuation, following the first child and healing all four nearby parents.
// Health alone is merged from Fighters; the parent pose is a disjoint view.
func StepFighter7SecondChild(object WorldObject, context Fighter7SecondChildContext, environment ChuProjectileEnvironment, library AnimationLibrary) (WorldObject, Fighter7SecondChildContext, ChuProjectileOutcome, error) {
	return stepFighter7SecondChildWithSound(object, context, environment, library, nil)
}

func stepFighter7SecondChildWithSound(object WorldObject, context Fighter7SecondChildContext, environment ChuProjectileEnvironment, library AnimationLibrary, soundPort *worldSoundPort) (WorldObject, Fighter7SecondChildContext, ChuProjectileOutcome, error) {
	original, oldContext := object, context
	fail := func(err error) (WorldObject, Fighter7SecondChildContext, ChuProjectileOutcome, error) {
		return original, oldContext, ChuProjectileOutcome{}, err
	}
	if object.Header.Behavior != 0x1391e {
		return fail(errors.New("source7 second child callback is unavailable"))
	}
	initializeFighter7Child(&object, environment.PaletteAttributes, 0x3c)
	object.Body.Health = 255
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return object, context, ChuProjectileOutcome{Draw: true}, nil
	}
	if object.HitStopTicks != 0 {
		stepFighter7ChildHitStop(&object)
	} else {
		object.Shake = 0
		object.Body.Clock = StepAnimationClock(object.Body.Clock)
		common := true
		switch object.Body.ActionKind {
		case 0, 3:
			if object.Body.Clock.Completed == 1 {
				resetFighter7Child(&object, 0x40, 2)
				common = false
			} else {
				if context.Parent.ObjectID != object.ObjectID-0x200 {
					return fail(errors.New("source7 second child parent identity mismatch"))
				}
				if context.Parent.Pose != 0xa7 {
					resetFighter7Child(&object, 0x3d, 1)
					common = false
				}
			}
		case 1:
			if object.Body.Clock.Completed == 1 {
				next, out, err := clearFighter7Child(object)
				if err != nil {
					return fail(err)
				}
				return next, context, out, nil
			}
		case 2:
			if context.Parent.ObjectID != object.ObjectID-0x200 {
				return fail(errors.New("source7 second child parent identity mismatch"))
			}
			if context.Parent.Pose != 0xa7 {
				resetFighter7Child(&object, 0x3d, 1)
				common = false
			} else if object.Controls.Held&ButtonB != 0 {
				if err := spawnFighter7HealingEffects(object, &context); err != nil {
					return fail(err)
				}
				resetFighter7Child(&object, 0x45, 3)
				for i := range context.Fighters {
					target := &context.Fighters[i]
					if target.ObjectID != uint16(0x8400+i*0x400) {
						return fail(errors.New("source7 healing requires all four fixed parents"))
					}
					if target.Size == 0 || target.Health < 0 {
						continue
					}
					dx := int16(target.X>>16) - int16(object.Body.X>>16)
					if dx < 0 {
						dx = -dx
					}
					if dx > 24 {
						continue
					}
					dy := int16(target.Y>>16) - 32 - int16(object.Body.Y>>16)
					if dy < 0 {
						dy = -dy
					}
					if dy > 32 {
						continue
					}
					target.Health += 16
					if target.Health >= 223 {
						target.Health = 223
					}
				}
				common = false
			} else if object.Controls.Held&(ButtonUp|ButtonDown) == 0 && object.Controls.Pressed&ButtonA != 0 {
				resetFighter7Child(&object, 0x42, 0)
				common = false
			} else if object.Controls.Held&ButtonDown != 0 && object.Controls.Pressed&ButtonA != 0 {
				resetFighter7Child(&object, 0x44, 0)
				common = false
			} else {
				object.Body.Facing = uint8(uint32(object.Aux48) >> 24)
				object.Body.X += (object.Body.VelocityX - object.Body.X) >> 2
				object.Body.Y += (object.Body.VerticalVelocity - object.Body.Y) >> 3
			}
		default:
			return fail(errors.New("source7 second child action is unavailable"))
		}
		if common && fighter7ChildOutside(&object, environment) {
			next, out, err := clearFighter7Child(object)
			if err != nil {
				return fail(err)
			}
			return next, context, out, nil
		}
	}
	var err error
	object, context.Submission, err = submitFighter7Child(object, context.Submission, environment, library, soundPort)
	if err != nil {
		return fail(err)
	}
	return object, context, ChuProjectileOutcome{Draw: true}, nil
}

// StepFighter7Pan recovers15094's word countdown and globalF570 write. Its
// high word is a temporary interpretation of canonical VY, not a new timer.
func StepFighter7Pan(object WorldObject, panAmplitude uint16, environment ChuProjectileEnvironment) (WorldObject, uint16, ChuProjectileOutcome, error) {
	original, oldAmplitude := object, panAmplitude
	if object.Header.Behavior != 0x15094 {
		return original, oldAmplitude, ChuProjectileOutcome{}, errors.New("source7 pan callback is unavailable")
	}
	if object.Header.Stage == 0 {
		object.Body.VerticalVelocity = replaceChuSpecialHighWord(object.Body.VerticalVelocity, 0x1000)
		object.Header.Stage += 2
	}
	if environment.PauseFlags&0x80 != 0 || environment.SlowFlag != 0 {
		return object, panAmplitude, ChuProjectileOutcome{}, nil
	}
	remaining := uint16(uint32(object.Body.VerticalVelocity)>>16) - 0x10
	object.Body.VerticalVelocity = replaceChuSpecialHighWord(object.Body.VerticalVelocity, remaining)
	panAmplitude = remaining
	if remaining == 0 {
		next, out, err := clearFighter7Child(object)
		if err != nil {
			return original, oldAmplitude, ChuProjectileOutcome{}, err
		}
		return next, panAmplitude, out, nil
	}
	return object, panAmplitude, ChuProjectileOutcome{}, nil
}

func initializeFighter7Child(object *WorldObject, attributes uint16, pose uint8) {
	if object.Header.Stage != 0 {
		return
	}
	object.Header.Stage += 2
	object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicParameter = 0, 0, 0
	object.GraphicCache = [4]uint32{}
	object.GraphicsSet, object.TilesBase = 0xd8f1a, 0x240000
	object.AnimationSet, object.ShapeSet = 0xd92ca, 0xd9a3a
	object.Body.Attributes |= attributes
	object.Body.Pose, object.Body.Clock.Duration, object.Body.Clock.Ticks = pose, 8, 255
}

func resetFighter7Child(object *WorldObject, pose, action uint8) {
	object.Body.Pose, object.Body.ActionKind = pose, action
	object.Body.Clock.FrameIndex, object.Body.Clock.Duration, object.Body.Clock.Ticks = 0, 8, 255
	object.Body.Clock.Completed, object.Body.Auxiliary64, object.Body.ActionFlags = 0, 0, 0
}

func stepFighter7ChildHitStop(object *WorldObject) {
	object.HitStopTicks--
	if object.Shake > 1 {
		object.Shake--
	}
	shake := int16(object.Shake)
	if object.HitStopTicks&1 != 0 {
		shake = -shake
	}
	object.ScreenOffsetX += shake
}

func fighter7ChildOutside(object *WorldObject, environment ChuProjectileEnvironment) bool {
	if int16(object.Body.Ground>>16) <= int16(object.Body.Y>>16) {
		object.Body.Y = object.Body.Ground
	}
	x, y := int16(object.Body.X>>16), int16(object.Body.Y>>16)
	return y < 0 || x > environment.ArenaRight+32 || x < environment.ArenaLeft-32
}

func submitFighter7Child(object WorldObject, submission Fighter7ChildSubmission, environment ChuProjectileEnvironment, library AnimationLibrary, soundPort *worldSoundPort) (WorldObject, Fighter7ChildSubmission, error) {
	result, err := submitAnimationRecordWithSound(AnimationSubmissionState{Actor: object.AsAnimation(), Buffers: submission.Buffers, PendingSound: submission.PendingSound, SoundQueue: submission.SoundQueue}, library, soundPort)
	if err != nil {
		return object, submission, err
	}
	object, err = object.MergeAnimation(result.Actor)
	if err != nil {
		return object, submission, err
	}
	submission.Buffers, submission.PendingSound, submission.SoundQueue = result.Buffers, result.PendingSound, result.SoundQueue
	p := ProjectFighter(ProjectionState{WorldX: object.Body.X, WorldY: object.Body.Y, ScreenX: object.ScreenX, ScreenY: object.ScreenY, OffsetX: object.ScreenOffsetX, OffsetY: object.ScreenOffsetY, CameraX: environment.CameraX, CameraY: environment.CameraY, Shake: environment.Shake})
	object.ScreenX, object.ScreenY, object.ScreenOffsetX, object.ScreenOffsetY = p.ScreenX, p.ScreenY, p.OffsetX, p.OffsetY
	return object, submission, nil
}

func spawnFighter7HealingEffects(object WorldObject, context *Fighter7SecondChildContext) error {
	x, y := int16(object.Body.X>>16), int16(object.Body.Y>>16)-32
	for i, definition := range [...]struct {
		X  int16
		VY int32
	}{{-8, 0xe000}, {0, 0x10000}, {-12, 0x12000}, {8, 0x6000}, {4, 0x10000}, {12, 0x12000}, {-2, 0xa000}, {2, 0xc000}} {
		pool, index, err := AllocateEffect(context.Pool)
		if err != nil {
			return err
		}
		context.Pool = pool
		if index < 0 {
			continue
		}
		slot := &context.Pool.Slots[index]
		slot.Type = 0x24
		slot.X = replaceChuSpecialHighWord(slot.X, uint16(x+definition.X))
		slot.Y = replaceChuSpecialHighWord(slot.Y, uint16(y))
		slot.Attributes, slot.Depth, slot.Layer = object.Body.Attributes&0x9800|0x4000, object.Body.Depth, object.Body.Layer-4
		context.EffectExtras[index] = Fighter7ChildEffectExtra{VerticalVelocity: definition.VY, Delay: uint8(i * 4)}
	}
	return nil
}

func clearFighter7Child(object WorldObject) (WorldObject, ChuProjectileOutcome, error) {
	size := object.Header.Size
	if size == 0 {
		size = 64
	}
	count := size / 16 * 16
	if count == 0 || count > 256 {
		return object, ChuProjectileOutcome{}, errors.New("source7 child clear exceeds logical slot")
	}
	next, err := ApplyObjectClearPrefix(object, count)
	if err != nil {
		return object, ChuProjectileOutcome{}, err
	}
	return next, ChuProjectileOutcome{Deleted: true, ClearedBytes: count}, nil
}
