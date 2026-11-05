package engine

import "errors"

// WorldObject stores the recovered fields of one logical slot, not a RAM
// image. Context-dependent aliases share their actual owner: Body.VelocityX
// also carries the status-effect phase, Aux48 carries a long/high-word/byte
// view, and Body.Clock.Duration is the generic FX delay.
type WorldObject struct {
	ObjectID                               uint16
	Header                                 MatchObjectHeader
	Body                                   FighterBodyState
	Controls                               FighterControls
	Tag                                    uint8
	GraphicsSet, TilesBase, MovementSource uint32
	SkipGraphics                           uint8
	GraphicOffsetX, GraphicOffsetY         int16
	GraphicCache                           [4]uint32
	GraphicParameter                       uint16
	AnimationSet, ShapeSet                 uint32
	ScreenX, ScreenY                       int16
	Aux0A, Aux0E                           uint16
	ScreenOffsetX, ScreenOffsetY           int16
	Aux18, Aux48                           int32
	Energy                                 int32
	DisplayedHealth, DisplayedEnergy       int16
	Power                                  int16
	GuardMask                              uint8
	AttackValue, StunDamage, EffectKind    uint8
	Reaction, HitStopTicks, Shake          uint8
	StatusDelay, AttackDelay, ContactFlags uint8
	TargetBit, InputChannel                uint8
	TargetMask                             uint32
	Residual                               int32
	StunTicks, HUDOffset, AttackScale      uint16
	ComboCount, ChargeTicks                uint8
	InputMode                              int8
	StreamPosition                         uint32
	AIBehavior                             uint8
	AITargetCursor, AICountdown, AIElapsed uint16
	AIFollowTarget                         uint16
	DragonBank                             uint16 `json:",omitempty"`
}

// ApplyObjectClearPrefix applies an already resolved 0x3556 clear request to
// owned fields. The ID is slot metadata and survives. Zero means no clear;
// callers obtain the default/deletion size from the actual callback outcome.
func ApplyObjectClearPrefix(object WorldObject, count uint16) (WorldObject, error) {
	if count > 256 || count&15 != 0 {
		return object, errors.New("object clear prefix is outside the logical slot")
	}
	if count >= 16 {
		object.Header = MatchObjectHeader{}
		object.ScreenX, object.ScreenY = 0, 0
		object.Aux0A, object.Aux0E = 0, 0
	}
	if count >= 32 {
		object.Body.VelocityX, object.Body.VerticalVelocity, object.GraphicsSet = 0, 0, 0
		object.Aux18 = 0
	}
	if count >= 48 {
		object.TilesBase, object.Body.Attributes, object.Body.Layer, object.SkipGraphics = 0, 0, 0, 0
		object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicCache[0] = 0, 0, 0
	}
	if count >= 64 {
		object.GraphicCache[1], object.GraphicCache[2], object.GraphicCache[3] = 0, 0, 0
		object.GraphicParameter, object.Body.Depth = 0, 0
	}
	if count >= 80 {
		object.Body.X, object.Body.Y, object.Aux48, object.Body.Gravity = 0, 0, 0, 0
	}
	if count >= 96 {
		object.AnimationSet, object.ScreenOffsetX, object.ScreenOffsetY, object.Tag = 0, 0, 0, 0
		object.Body.Clock.FrameIndex, object.Body.Clock.Duration, object.Body.Clock.Ticks, object.Body.Clock.Flags = 0, 0, 0, 0
		object.Body.ActionKind, object.Body.Pose = 0, 0
	}
	if count >= 112 {
		object.Body.Clock.Completed, object.Body.Facing, object.GuardMask, object.Body.Auxiliary64 = 0, 0, 0, 0
		object.AttackValue, object.StunDamage, object.EffectKind, object.Reaction = 0, 0, 0, 0
		object.HitStopTicks, object.Shake, object.Body.Request, object.Body.EnabledStages = 0, 0, 0, 0
		object.Body.ActionFlags, object.Body.Team, object.Body.BaseTeam = 0, 0, 0
	}
	if count >= 128 {
		object.TargetBit, object.Controls.RawHeld, object.Controls.RetainTicks, object.InputChannel, object.Body.Auxiliary77 = 0, 0, 0, 0, 0
		object.Controls.Held &= 0xff00
		object.Controls.Pressed &= 0xff00
		object.Controls.Changed &= 0xff00
		object.Body.Health, object.DisplayedHealth, object.Power, object.Body.Auxiliary7E = 0, 0, 0, 0
	}
	if count >= 144 {
		object.Body.Auxiliary80, object.StunTicks, object.StatusDelay, object.Body.ExtraJump = 0, 0, 0, 0
		object.AttackDelay, object.ContactFlags, object.HUDOffset, object.ShapeSet = 0, 0, 0, 0
	}
	if count >= 160 {
		object.MovementSource, object.TargetMask, object.Residual, object.Body.Ground = 0, 0, 0, 0
	}
	if count >= 176 {
		clear(object.Body.Commands[:8])
	}
	if count >= 192 {
		clear(object.Body.Commands[8:])
		object.Energy, object.DisplayedEnergy = 0, 0
	}
	if count >= 208 {
		object.Controls.Held, object.Controls.Pressed, object.Controls.Changed = 0, 0, 0
		object.AttackScale, object.ComboCount, object.ChargeTicks, object.Body.TransitionDelay = 0, 0, 0, 0
		object.InputMode, object.StreamPosition, object.AIBehavior = 0, 0, 0
	}
	if count >= 224 {
		object.AITargetCursor, object.AICountdown, object.AIElapsed, object.AIFollowTarget = 0, 0, 0, 0
	}
	if count >= 256 {
		object.DragonBank = 0
	}
	return object, nil
}

func (object WorldObject) AsAnimation() AnimationActorState {
	return AnimationActorState{ObjectID: object.ObjectID, AnimationSet: object.AnimationSet, ShapeSet: object.ShapeSet,
		Pose: object.Body.Pose, Clock: object.Body.Clock, Facing: object.Body.Facing, ActionFlags: object.Body.ActionFlags,
		EnabledStages: object.Body.EnabledStages, Attributes: object.Body.Attributes, GraphicOffsetX: object.GraphicOffsetX,
		GraphicOffsetY: object.GraphicOffsetY, WorldX: object.Body.X, WorldY: object.Body.Y, GraphicParameter: object.GraphicParameter,
		Auxiliary94: object.TargetMask, Auxiliary64: object.Body.Auxiliary64, Value65: object.AttackValue,
		Value66: object.StunDamage, Value67: object.EffectKind, Value68: object.Reaction}
}

func (object WorldObject) MergeAnimation(actor AnimationActorState) (WorldObject, error) {
	if actor.ObjectID != object.ObjectID {
		return object, errors.New("animation object identity mismatch")
	}
	object.AnimationSet, object.ShapeSet = actor.AnimationSet, actor.ShapeSet
	object.Body.Pose, object.Body.Clock, object.Body.Facing = actor.Pose, actor.Clock, actor.Facing
	object.Body.ActionFlags, object.Body.EnabledStages, object.Body.Attributes = actor.ActionFlags, actor.EnabledStages, actor.Attributes
	object.GraphicOffsetX, object.GraphicOffsetY = actor.GraphicOffsetX, actor.GraphicOffsetY
	object.Body.X, object.Body.Y, object.GraphicParameter = actor.WorldX, actor.WorldY, actor.GraphicParameter
	object.TargetMask, object.Body.Auxiliary64 = actor.Auxiliary94, actor.Auxiliary64
	object.AttackValue, object.StunDamage, object.EffectKind, object.Reaction = actor.Value65, actor.Value66, actor.Value67, actor.Value68
	return object, nil
}

func (object WorldObject) AsChuChild() ChuSpecialChild {
	return ChuSpecialChild{Behavior: object.Header.Behavior, Size: object.Header.Size, Stage: object.Header.Stage,
		VelocityX: object.Body.VelocityX, VerticalVelocity: object.Body.VerticalVelocity, Attributes: object.Body.Attributes,
		Depth: object.Body.Depth, Layer: object.Body.Layer, Pose: object.Body.Pose, Facing: object.Body.Facing,
		Team: object.Body.Team, TargetBit: object.TargetBit, Auxiliary76: object.InputChannel,
		X: object.Body.X, Y: object.Body.Y, Ground: object.Body.Ground, TargetMask: object.TargetMask, Power: object.Power}
}

// MergeChuChild applies only fields owned by the parent's child initializer.
// In particular animation timing, graphics/cache and combat flags survive.
func (object WorldObject) MergeChuChild(child ChuSpecialChild) WorldObject {
	object.Header = MatchObjectHeader{Behavior: child.Behavior, Size: child.Size, Stage: child.Stage}
	object.Body.VelocityX, object.Body.VerticalVelocity = child.VelocityX, child.VerticalVelocity
	object.Body.Attributes, object.Body.Depth, object.Body.Layer = child.Attributes, child.Depth, child.Layer
	object.Body.Pose, object.Body.Facing, object.Body.Team = child.Pose, child.Facing, child.Team
	object.TargetBit, object.InputChannel = child.TargetBit, child.Auxiliary76
	object.Body.X, object.Body.Y, object.Body.Ground, object.TargetMask, object.Power = child.X, child.Y, child.Ground, child.TargetMask, child.Power
	return object
}

func (object WorldObject) AsChuFighter(children [3]WorldObject) (ChuActorFighter, error) {
	for i, child := range children {
		if child.ObjectID != object.ObjectID+uint16(i+1)*256 {
			return ChuActorFighter{}, errors.New("fighter child identity mismatch")
		}
	}
	f := ChuActorFighter{ObjectID: object.ObjectID, Size: object.Header.Size, Body: object.Body, Controls: object.Controls, Energy: object.Energy,
		InputMode: object.InputMode, InputChannel: object.InputChannel, StreamPosition: object.StreamPosition, AIBehavior: object.AIBehavior,
		AITargetCursor: object.AITargetCursor, AICountdown: object.AICountdown, AIElapsed: object.AIElapsed, AIFollowTarget: object.AIFollowTarget,
		ScreenX: object.ScreenX, ScreenY: object.ScreenY, ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY,
		DisplayedHealth: object.DisplayedHealth, DisplayedEnergy: object.DisplayedEnergy, HUDOffset: object.HUDOffset,
		AnimationSet: object.AnimationSet, ShapeSet: object.ShapeSet, GraphicOffsetX: object.GraphicOffsetX, GraphicOffsetY: object.GraphicOffsetY,
		GraphicParameter: object.GraphicParameter, ChargeTicks: object.ChargeTicks, GuardMask: object.GuardMask, ContactFlags: object.ContactFlags,
		TargetMask: object.TargetMask, AttackValue: object.AttackValue, StunDamage: object.StunDamage, EffectKind: object.EffectKind, Reaction: object.Reaction,
		AttackScale: object.AttackScale, HitStopTicks: object.HitStopTicks, Shake: object.Shake, StatusDelay: object.StatusDelay, Residual: object.Residual,
		StunTicks: object.StunTicks, TargetBit: object.TargetBit, Power: object.Power, AlternateVelocity: object.Aux48, MovementSource: object.MovementSource}
	for i, child := range children {
		f.Children[i], f.ChildAttackFlags[i] = child.AsChuChild(), child.Body.Auxiliary64
		f.ChildExtras[i] = Fighter4ChildExtra{Direction: child.Aux48, Held: uint8(child.Controls.Held), Pressed: uint8(child.Controls.Pressed), ContactFlags: child.ContactFlags, DragonBank: child.DragonBank}
	}
	return f, nil
}

func (object WorldObject) MergeChuFighter(fighter ChuActorFighter) (WorldObject, error) {
	if fighter.ObjectID != object.ObjectID {
		return object, errors.New("fighter object identity mismatch")
	}
	object.Header.Size, object.Body, object.Controls, object.Energy = fighter.Size, fighter.Body, fighter.Controls, fighter.Energy
	object.InputMode, object.InputChannel, object.StreamPosition, object.AIBehavior = fighter.InputMode, fighter.InputChannel, fighter.StreamPosition, fighter.AIBehavior
	object.AITargetCursor, object.AICountdown, object.AIElapsed, object.AIFollowTarget = fighter.AITargetCursor, fighter.AICountdown, fighter.AIElapsed, fighter.AIFollowTarget
	object.ScreenX, object.ScreenY, object.ScreenOffsetX, object.ScreenOffsetY = fighter.ScreenX, fighter.ScreenY, fighter.ScreenOffsetX, fighter.ScreenOffsetY
	object.DisplayedHealth, object.DisplayedEnergy, object.HUDOffset = fighter.DisplayedHealth, fighter.DisplayedEnergy, fighter.HUDOffset
	object.AnimationSet, object.ShapeSet, object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicParameter = fighter.AnimationSet, fighter.ShapeSet, fighter.GraphicOffsetX, fighter.GraphicOffsetY, fighter.GraphicParameter
	object.ChargeTicks, object.GuardMask, object.ContactFlags, object.TargetMask = fighter.ChargeTicks, fighter.GuardMask, fighter.ContactFlags, fighter.TargetMask
	object.AttackValue, object.StunDamage, object.EffectKind, object.Reaction, object.AttackScale = fighter.AttackValue, fighter.StunDamage, fighter.EffectKind, fighter.Reaction, fighter.AttackScale
	object.HitStopTicks, object.Shake, object.StatusDelay, object.Residual = fighter.HitStopTicks, fighter.Shake, fighter.StatusDelay, fighter.Residual
	object.StunTicks, object.TargetBit, object.Power, object.Aux48 = fighter.StunTicks, fighter.TargetBit, fighter.Power, fighter.AlternateVelocity
	object.MovementSource = fighter.MovementSource
	return object, nil
}

func (object WorldObject) AsEffectSlot() EffectSlot {
	return EffectSlot{Behavior: object.Header.Behavior, Size: object.Header.Size, Stage: object.Header.Stage, VelocityX: object.Body.VelocityX,
		Attributes: object.Body.Attributes, Layer: object.Body.Layer, Depth: object.Body.Depth, X: object.Body.X, Y: object.Body.Y, Type: object.Tag}
}

func (object WorldObject) MergeEffectSlot(slot EffectSlot) WorldObject {
	object.Header = MatchObjectHeader{Behavior: slot.Behavior, Size: slot.Size, Stage: slot.Stage}
	object.Body.VelocityX, object.Body.Attributes, object.Body.Layer, object.Body.Depth = slot.VelocityX, slot.Attributes, slot.Layer, slot.Depth
	object.Body.X, object.Body.Y, object.Tag = slot.X, slot.Y, slot.Type
	return object
}

func (object WorldObject) AsReactionExtra() ReactionEffectExtra {
	return ReactionEffectExtra{Owner: uint16(uint32(object.Aux48) >> 16), OffsetY: object.ScreenOffsetY, Phase: object.Body.Clock.Duration}
}

func (object WorldObject) MergeReactionExtra(extra ReactionEffectExtra) WorldObject {
	object.Aux48 = worldObjectHighWord(object.Aux48, extra.Owner)
	object.ScreenOffsetY, object.Body.Clock.Duration = extra.OffsetY, extra.Phase
	return object
}

func (object WorldObject) AsCollisionActor() CollisionActor {
	return CollisionActor{AttackContactActor: AttackContactActor{X: object.Body.X, Y: object.Body.Y, Ground: object.Body.Ground,
		VelocityX: object.Body.VelocityX, VelocityY: object.Body.VerticalVelocity, Gravity: object.Body.Gravity, Attributes: object.Body.Attributes,
		Depth: object.Body.Depth, Layer: object.Body.Layer, CharacterID: object.Tag, FrameIndex: object.Body.Clock.FrameIndex,
		FrameDuration: object.Body.Clock.Duration, FrameTicks: object.Body.Clock.Ticks, FrameFlags: object.Body.Clock.Flags,
		ActionKind: object.Body.ActionKind, Pose: object.Body.Pose, Completed: object.Body.Clock.Completed, Facing: object.Body.Facing,
		GuardMask: object.GuardMask, AttackFlags: object.Body.Auxiliary64, AttackValue: object.AttackValue, StunDamage: object.StunDamage,
		EffectKind: object.EffectKind, Reaction: object.Reaction, HitStopTicks: object.HitStopTicks, Shake: object.Shake,
		ActionFlags: object.Body.EnabledStages, CancelTicks: object.Body.ActionFlags, Team: object.Body.Team, TargetBit: object.TargetBit,
		RetainedHeld: uint8(object.Controls.Held), InputRetention: object.Controls.RetainTicks, Auxiliary77: object.Body.Auxiliary77,
		Health: object.Body.Health, StunValue: object.Body.Auxiliary7E, StunTicks: object.StunTicks, StatusDelay: object.StatusDelay,
		AttackDelay: object.AttackDelay, ContactFlags: object.ContactFlags, TargetMask: object.TargetMask, Residual: object.Residual,
		AuxiliaryB8: object.Energy, ComboCount: object.ComboCount}, ThrowDelay: object.Body.TransitionDelay}
}

func (object WorldObject) MergeCollisionActor(collision CollisionActor) WorldObject {
	a := collision.AttackContactActor
	object.Body.X, object.Body.Y, object.Body.Ground = a.X, a.Y, a.Ground
	object.Body.VelocityX, object.Body.VerticalVelocity, object.Body.Gravity = a.VelocityX, a.VelocityY, a.Gravity
	object.Body.Attributes, object.Body.Depth, object.Body.Layer, object.Tag = a.Attributes, a.Depth, a.Layer, a.CharacterID
	object.Body.Clock = AnimationClock{FrameIndex: a.FrameIndex, Duration: a.FrameDuration, Ticks: a.FrameTicks, Flags: a.FrameFlags, Completed: a.Completed}
	object.Body.ActionKind, object.Body.Pose, object.Body.Facing = a.ActionKind, a.Pose, a.Facing
	object.GuardMask, object.Body.Auxiliary64, object.AttackValue, object.StunDamage = a.GuardMask, a.AttackFlags, a.AttackValue, a.StunDamage
	object.EffectKind, object.Reaction, object.HitStopTicks, object.Shake = a.EffectKind, a.Reaction, a.HitStopTicks, a.Shake
	object.Body.EnabledStages, object.Body.ActionFlags, object.Body.Team, object.TargetBit = a.ActionFlags, a.CancelTicks, a.Team, a.TargetBit
	object.Controls.Held = object.Controls.Held&0xff00 | Buttons(a.RetainedHeld)
	object.Controls.RetainTicks, object.Body.Auxiliary77 = a.InputRetention, a.Auxiliary77
	object.Body.Health, object.Body.Auxiliary7E, object.StunTicks, object.StatusDelay = a.Health, a.StunValue, a.StunTicks, a.StatusDelay
	object.AttackDelay, object.ContactFlags, object.TargetMask, object.Residual = a.AttackDelay, a.ContactFlags, a.TargetMask, a.Residual
	object.Energy, object.ComboCount, object.Body.TransitionDelay = a.AuxiliaryB8, a.ComboCount, collision.ThrowDelay
	return object
}

func (object WorldObject) AsChuProjectile() ChuProjectileState {
	return ChuProjectileState{Animation: object.AsAnimation(), Behavior: object.Header.Behavior, GraphicSet: object.GraphicsSet, TileBase: object.TilesBase,
		Size: object.Header.Size, Stage: object.Header.Stage, GraphicCache: object.GraphicCache, VelocityX: object.Body.VelocityX,
		VerticalVelocity: object.Body.VerticalVelocity, Ground: object.Body.Ground, Power: uint16(object.Power), TrailPhase: uint8(uint32(object.Aux48) >> 24),
		Shake: object.Shake, ContactFlags: object.ContactFlags, Layer: object.Body.Layer, Team: object.Body.Team, TargetBit: object.TargetBit,
		Health: object.Body.Health, Depth: object.Body.Depth, ScreenX: object.ScreenX, ScreenY: object.ScreenY,
		ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY}
}

func (object WorldObject) MergeChuProjectile(projectile ChuProjectileState) (WorldObject, error) {
	merged, err := object.MergeAnimation(projectile.Animation)
	if err != nil {
		return object, err
	}
	merged.Header = MatchObjectHeader{Behavior: projectile.Behavior, Size: projectile.Size, Stage: projectile.Stage}
	merged.GraphicsSet, merged.TilesBase, merged.GraphicCache = projectile.GraphicSet, projectile.TileBase, projectile.GraphicCache
	merged.Body.VelocityX, merged.Body.VerticalVelocity, merged.Body.Ground, merged.Power = projectile.VelocityX, projectile.VerticalVelocity, projectile.Ground, int16(projectile.Power)
	merged.Aux48 = int32(uint32(merged.Aux48)&0x00ffffff | uint32(projectile.TrailPhase)<<24)
	merged.Shake, merged.ContactFlags, merged.Body.Layer, merged.Body.Team, merged.TargetBit = projectile.Shake, projectile.ContactFlags, projectile.Layer, projectile.Team, projectile.TargetBit
	merged.Body.Health, merged.Body.Depth = projectile.Health, projectile.Depth
	merged.ScreenX, merged.ScreenY, merged.ScreenOffsetX, merged.ScreenOffsetY = projectile.ScreenX, projectile.ScreenY, projectile.ScreenOffsetX, projectile.ScreenOffsetY
	return merged, nil
}

func (object WorldObject) AsChuAttachment(kind ChuAttachmentKind) ChuAttachmentState {
	return ChuAttachmentState{Kind: kind, Actor: object.AsAnimation(), Behavior: object.Header.Behavior, Size: object.Header.Size, Stage: object.Header.Stage,
		GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase, GraphicCache: object.GraphicCache, Power: object.Power, Health: object.Body.Health,
		ScreenX: object.ScreenX, ScreenY: object.ScreenY, ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY,
		SourceOffsetX: int16(object.Body.VelocityX >> 16), SourceOffsetY: int16(object.Body.VerticalVelocity >> 16)}
}

func (object WorldObject) MergeChuAttachment(attachment ChuAttachmentState) (WorldObject, error) {
	merged, err := object.MergeAnimation(attachment.Actor)
	if err != nil {
		return object, err
	}
	merged.Header = MatchObjectHeader{Behavior: attachment.Behavior, Size: attachment.Size, Stage: attachment.Stage}
	merged.GraphicsSet, merged.TilesBase, merged.GraphicCache = attachment.GraphicsSet, attachment.TilesBase, attachment.GraphicCache
	merged.Power, merged.Body.Health = attachment.Power, attachment.Health
	merged.ScreenX, merged.ScreenY, merged.ScreenOffsetX, merged.ScreenOffsetY = attachment.ScreenX, attachment.ScreenY, attachment.ScreenOffsetX, attachment.ScreenOffsetY
	merged.Body.VelocityX = worldObjectHighWord(merged.Body.VelocityX, uint16(attachment.SourceOffsetX))
	merged.Body.VerticalVelocity = worldObjectHighWord(merged.Body.VerticalVelocity, uint16(attachment.SourceOffsetY))
	return merged, nil
}

func (object WorldObject) AsEffectMotion() EffectMotionState {
	return EffectMotionState{ObjectID: object.ObjectID, Slot: object.AsEffectSlot(), VerticalVelocity: object.Body.VerticalVelocity,
		GraphicSet: object.GraphicsSet, TileBase: object.TilesBase, Sequence: object.AnimationSet, GraphicCache: object.GraphicCache,
		GraphicOffsetX: object.GraphicOffsetX, GraphicOffsetY: object.GraphicOffsetY, GraphicParameter: object.GraphicParameter, SkipTiles: object.SkipGraphics,
		FrameIndex: object.Body.Clock.FrameIndex, Delay: object.Body.Clock.Duration, Ticks: object.Body.Clock.Ticks, Flags: object.Body.Clock.Flags,
		FollowOwner: uint16(uint32(object.Aux48) >> 16), ScreenX: object.ScreenX, ScreenY: object.ScreenY,
		ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY}
}

func (object WorldObject) MergeEffectMotion(effect EffectMotionState) (WorldObject, error) {
	if effect.ObjectID != object.ObjectID {
		return object, errors.New("effect object identity mismatch")
	}
	object = object.MergeEffectSlot(effect.Slot)
	object.Body.VerticalVelocity, object.GraphicsSet, object.TilesBase, object.AnimationSet = effect.VerticalVelocity, effect.GraphicSet, effect.TileBase, effect.Sequence
	object.GraphicCache, object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicParameter = effect.GraphicCache, effect.GraphicOffsetX, effect.GraphicOffsetY, effect.GraphicParameter
	object.SkipGraphics = effect.SkipTiles
	object.Body.Clock.FrameIndex, object.Body.Clock.Duration, object.Body.Clock.Ticks, object.Body.Clock.Flags = effect.FrameIndex, effect.Delay, effect.Ticks, effect.Flags
	object.Aux48 = worldObjectHighWord(object.Aux48, effect.FollowOwner)
	object.ScreenX, object.ScreenY, object.ScreenOffsetX, object.ScreenOffsetY = effect.ScreenX, effect.ScreenY, effect.ScreenOffsetX, effect.ScreenOffsetY
	return object, nil
}

func (object WorldObject) AsAttachedEffect(kind AttachedEffectKind) AttachedEffectState {
	return AttachedEffectState{Kind: kind, ObjectID: object.ObjectID, Slot: object.AsEffectSlot(), GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase,
		SkipTiles: object.SkipGraphics, GraphicOffsetX: object.GraphicOffsetX, GraphicOffsetY: object.GraphicOffsetY, GraphicCache: object.GraphicCache,
		Parameter: object.GraphicParameter, Aux48: uint16(uint32(object.Aux48) >> 16), Sequence: object.AnimationSet,
		ScreenX: object.ScreenX, ScreenY: object.ScreenY, OffsetX: object.ScreenOffsetX, OffsetY: object.ScreenOffsetY,
		Index: object.Body.Clock.FrameIndex, Delay: object.Body.Clock.Duration, Ticks: object.Body.Clock.Ticks, Flags: object.Body.Clock.Flags}
}

func (object WorldObject) MergeAttachedEffect(effect AttachedEffectState) (WorldObject, error) {
	if effect.ObjectID != object.ObjectID {
		return object, errors.New("attached effect object identity mismatch")
	}
	object = object.MergeEffectSlot(effect.Slot)
	object.GraphicsSet, object.TilesBase, object.SkipGraphics = effect.GraphicsSet, effect.TilesBase, effect.SkipTiles
	object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicCache, object.GraphicParameter = effect.GraphicOffsetX, effect.GraphicOffsetY, effect.GraphicCache, effect.Parameter
	object.Aux48, object.AnimationSet = worldObjectHighWord(object.Aux48, effect.Aux48), effect.Sequence
	object.ScreenX, object.ScreenY, object.ScreenOffsetX, object.ScreenOffsetY = effect.ScreenX, effect.ScreenY, effect.OffsetX, effect.OffsetY
	object.Body.Clock.FrameIndex, object.Body.Clock.Duration, object.Body.Clock.Ticks, object.Body.Clock.Flags = effect.Index, effect.Delay, effect.Ticks, effect.Flags
	return object, nil
}

func (object WorldObject) AsFighterInitialization() FighterInitializationState {
	return FighterInitializationState{Body: object.Body, CharacterID: object.Tag, GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase,
		AnimationSet: object.AnimationSet, ShapeSet: object.ShapeSet, MovementSource: object.MovementSource, SkipGraphics: object.SkipGraphics,
		GraphicOffsetX: object.GraphicOffsetX, GraphicOffsetY: object.GraphicOffsetY, GraphicCache: object.GraphicCache, GraphicParameter: object.GraphicParameter,
		DisplayedHealth: object.DisplayedHealth, Energy: object.Energy, DisplayedEnergy: object.DisplayedEnergy, AttackScale: object.AttackScale}
}

func (object WorldObject) MergeFighterInitialization(state FighterInitializationState) WorldObject {
	object.Body, object.Tag = state.Body, state.CharacterID
	object.GraphicsSet, object.TilesBase, object.AnimationSet, object.ShapeSet, object.MovementSource = state.GraphicsSet, state.TilesBase, state.AnimationSet, state.ShapeSet, state.MovementSource
	object.SkipGraphics, object.GraphicOffsetX, object.GraphicOffsetY, object.GraphicCache, object.GraphicParameter = state.SkipGraphics, state.GraphicOffsetX, state.GraphicOffsetY, state.GraphicCache, state.GraphicParameter
	object.DisplayedHealth, object.Energy, object.DisplayedEnergy, object.AttackScale = state.DisplayedHealth, state.Energy, state.DisplayedEnergy, state.AttackScale
	return object
}

func worldObjectHighWord(value int32, high uint16) int32 {
	return int32(uint32(high)<<16 | uint32(value)&0xffff)
}

// WorldObjects may omit completely empty slots. Cleared/inactive slots with
// meaningful retained fields remain present. Reads validate metadata identity.
type WorldObjects map[uint16]WorldObject

func LookupWorldObject(objects WorldObjects, id uint16) (WorldObject, error) {
	if id < 0x8000 || id >= 0xa000 || id&63 != 0 {
		return WorldObject{}, errors.New("world object ID is outside logical pool slots")
	}
	object, ok := objects[id]
	if !ok {
		return WorldObject{ObjectID: id}, nil
	}
	if object.ObjectID != id {
		return WorldObject{}, errors.New("stored world object identity mismatch")
	}
	return object, nil
}

func BuildChuActorFighter(objects WorldObjects, id uint16) (ChuActorFighter, error) {
	object, err := LookupWorldObject(objects, id)
	if err != nil {
		return ChuActorFighter{}, err
	}
	children := [3]WorldObject{}
	for i := range children {
		children[i], err = LookupWorldObject(objects, id+uint16(i+1)*256)
		if err != nil {
			return ChuActorFighter{}, err
		}
	}
	return object.AsChuFighter(children)
}

func MergeWorldChuActor(objects WorldObjects, fighter ChuActorFighter) (WorldObjects, error) {
	object, err := LookupWorldObject(objects, fighter.ObjectID)
	if err != nil {
		return objects, err
	}
	merged, err := object.MergeChuFighter(fighter)
	if err != nil {
		return objects, err
	}
	children := [3]WorldObject{}
	for i := range children {
		children[i], err = LookupWorldObject(objects, fighter.ObjectID+uint16(i+1)*256)
		if err != nil {
			return objects, err
		}
		children[i] = children[i].MergeChuChild(fighter.Children[i])
		extra := fighter.ChildExtras[i]
		children[i].Aux48, children[i].ContactFlags, children[i].DragonBank = extra.Direction, extra.ContactFlags, extra.DragonBank
		children[i].Controls.Held = children[i].Controls.Held&0xff00 | Buttons(extra.Held)
		children[i].Controls.Pressed = children[i].Controls.Pressed&0xff00 | Buttons(extra.Pressed)
	}
	result := cloneWorldObjects(objects)
	result[fighter.ObjectID] = merged
	for i, child := range children {
		// ChildAttackFlags are a read-only view used by AI, not fields the
		// parent initializer is allowed to write back.
		result[fighter.ObjectID+uint16(i+1)*256] = child
	}
	return result, nil
}

func BuildWorldEffectPool(objects WorldObjects, cursor uint16) (EffectPool, error) {
	pool := EffectPool{Cursor: cursor}
	for i := range pool.Slots {
		object, err := LookupWorldObject(objects, 0x9400+uint16(i)*128)
		if err != nil {
			return EffectPool{}, err
		}
		pool.Slots[i] = object.AsEffectSlot()
	}
	return pool, nil
}

func MergeWorldEffectPool(objects WorldObjects, pool EffectPool) (WorldObjects, error) {
	merged := [16]WorldObject{}
	for i, slot := range pool.Slots {
		object, err := LookupWorldObject(objects, 0x9400+uint16(i)*128)
		if err != nil {
			return objects, err
		}
		merged[i] = object.MergeEffectSlot(slot)
	}
	result := cloneWorldObjects(objects)
	for _, object := range merged {
		result[object.ObjectID] = object
	}
	return result, nil
}

func cloneWorldObjects(objects WorldObjects) WorldObjects {
	result := make(WorldObjects, len(objects))
	for id, object := range objects {
		result[id] = object
	}
	return result
}
