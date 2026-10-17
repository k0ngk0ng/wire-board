package engine

import "fmt"

// ActionLaunched is bank-6 $9E3D's $3E branch: a court device or an attached
// source throws the actor along an original target-height trajectory.
const ActionLaunched Action = 0x3e

type LaunchedPlayerContext struct {
	Actor, Attachment  uint8
	Source             Player
	SourceKind         uint8
	Random             RandomState
	Computer, HasBall  bool
	Team, Character    uint8
	EnhancementEnabled bool
	Equipment          uint8
	HolderReleasing    bool
	Holder             int8
	HolderSpecialShot  uint8
	SpecialShot        uint8
	ShotCycle          int8
	HeldButtons        Buttons
	HighArc            bool
	AuxiliaryOwners    [2]int8
}

type LaunchedPlayerPreparation struct {
	Player                 Player
	Attachment             uint8
	ChangeRandom           bool
	Random                 RandomState
	ChangePoseSource       bool
	PoseSource             uint8
	ChangeSpecialShot      bool
	ClearHolderSpecialShot bool
	SpecialShot            uint8
	LaunchPlan             *HoopThrowPlan
	SoundIDs               []uint8
}

type LaunchedPlayerCompletion struct {
	Player Player
	Ball   BallCommand
}

// PrepareLaunchedPlayer stops at original $9E99, before animation and motion.
// The world must commit these writes before the motion binding loads its state.
func PrepareLaunchedPlayer(original Player, ctx LaunchedPlayerContext, launchContext ...PassLaunchContext) (LaunchedPlayerPreparation, error) {
	initial := LaunchedPlayerPreparation{Player: original, Attachment: ctx.Attachment}
	if original.Action != ActionLaunched || ctx.Actor >= 4 {
		return initial, fmt.Errorf("launched player action/actor outside restored handler")
	}
	result := initial
	if original.Entered {
		return result, nil
	}
	p := &result.Player
	initializeAction(p)
	result.SoundIDs = []uint8{6}
	source := ctx.Source
	result.PoseSource = ctx.Attachment & 15
	if ctx.Attachment&128 == 0 {
		// F9C3 returns its last ROR carry: the OLD register's low bit. AND #1
		// preserves that carry and ROR A places it in the facing byte's high bit.
		p.FacingLeft = ctx.Random&1 != 0
		result.ChangeRandom = true
		result.Random, _ = NextRandom(ctx.Random)
		source = *p
		result.PoseSource = ctx.Actor
	} else if result.PoseSource >= 13 {
		return initial, fmt.Errorf("launched source object %d outside world", result.PoseSource)
	}
	result.ChangePoseSource = true
	result.Attachment = 64
	height := Fixed(88) * Pixel
	if uint8(source.Body.Position.Height>>16) != 0 || uint8(source.Body.Position.Height>>8) >= 152 {
		height = 280 * Pixel
	} else if uint8(source.Body.Position.Height>>8) >= 56 {
		height = 184 * Pixel
	}
	x := Fixed(460) * Pixel
	if source.FacingLeft {
		x = 68 * Pixel
	}
	plan := HoopThrowPlan{Target: Position{X: x, Depth: 47 * Pixel, Height: height}, ArcHeight: 24 * Pixel}
	if ctx.HighArc {
		plan.ArcHeight = 48 * Pixel
	}
	override := false
	if ctx.EnhancementEnabled && ctx.Team == 0 {
		if ctx.Character >= 4 {
			return initial, fmt.Errorf("launched equipment character %d outside catalog", ctx.Character)
		}
		if !ctx.HolderReleasing && ctx.SpecialShot&128 != 0 {
			override = true
		} else if ctx.Equipment < 32 && originalEquipmentShotModes[ctx.Equipment] == ctx.Equipment {
			heights := [4]Fixed{272, 176, 104, 160}
			slot := ctx.Equipment >> 2
			if slot >= 4 {
				return initial, fmt.Errorf("launched equipment height %d outside restored catalog", slot)
			}
			plan.Target.Height = heights[slot] * Pixel
		} else if ctx.HolderReleasing && ctx.Character == 0 {
			// $E0F3 falls through to $E0F5 only for equipment slot zero.
			// This consumes the current holder's flag, not this actor's.
			if ctx.Holder < 0 || ctx.Holder >= 4 {
				return initial, fmt.Errorf("launched equipment holder %d outside player state", ctx.Holder)
			}
			if ctx.HolderSpecialShot != 0 {
				plan.Target.Height = 272 * Pixel
				result.ClearHolderSpecialShot = true
			}
		} else {
			override = ctx.Computer && uint8(ctx.ShotCycle)&8 != 0 || !ctx.Computer && ctx.HeldButtons&ButtonUp != 0
		}
	} else {
		override = ctx.Computer && uint8(ctx.ShotCycle)&8 != 0 || !ctx.Computer && ctx.HeldButtons&ButtonUp != 0
	}
	if override {
		plan.Target.Height = 240 * Pixel
		result.ChangeSpecialShot = true
		result.SpecialShot = 0
	}
	body, e := LaunchPass(p.Body.Position, plan.Target, plan.ArcHeight, launchContext...)
	if e != nil {
		return initial, e
	}
	p.Body = body
	result.LaunchPlan = &plan
	return result, nil
}

// CompleteLaunchedPlayer covers $9E99..$9EC0. The original auxiliary-owner
// pose suppresses animation advancement; landing uses the request's low bits
// even when its pending flag has already been consumed.
func CompleteLaunchedPlayer(original Player, ctx LaunchedPlayerContext, court FlatCourt) (LaunchedPlayerCompletion, error) {
	initial := LaunchedPlayerCompletion{Player: original}
	if original.Action != ActionLaunched || ctx.Actor >= 4 {
		return initial, fmt.Errorf("launched player completion outside restored handler")
	}
	p := original
	if ctx.AuxiliaryOwners[0] == int8(ctx.Actor) || ctx.AuxiliaryOwners[1] == int8(ctx.Actor) {
		p.Pose = Pose{Frame: 18, Mirrored: p.FacingLeft}
	} else {
		spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 5}, Ticks: 2}, {Pose: Pose{Frame: 4}, Ticks: 255}, {Pose: Pose{Frame: 4}, Ticks: 255}}, End: RepeatForever}
		step, e := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
		if e != nil {
			return initial, e
		}
		p.Animation, p.Pose = step.State, step.Pose
	}
	var ball BallCommand
	if ctx.Attachment&128 != 0 && court.originalMotion == nil {
		if ctx.SourceKind >= 16 {
			return initial, fmt.Errorf("launched attached source kind %d outside original table", ctx.SourceKind)
		}
		attachRiderPosition(&p, RiderSupport{Body: ctx.Source.Body, Pose: ctx.Source.Pose, Kind: ctx.SourceKind})
	} else {
		var e error
		ball, e = advancePlayerOnCourt(&p, playerTerrainContext{HasBall: ctx.HasBall, Computer: ctx.Computer}, court)
		if e != nil {
			return initial, e
		}
	}
	if !p.Airborne {
		p.Action, p.Entered = p.Request.Action, false
	}
	return LaunchedPlayerCompletion{Player: p, Ball: ball}, nil
}
