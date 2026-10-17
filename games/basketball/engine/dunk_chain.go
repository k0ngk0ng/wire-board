package engine

import "errors"

// DunkContext reads the selected hanging target, independent of the ordinary
// shot target. TargetOwner preserves the original raised-hoop high-bit flag.
type DunkContext struct {
	Actor, TargetObject, TargetOwner uint8
	TargetDurability                 int8
	AuxiliaryOwners                  [2]int8
	CarriedPlayer                    int8
	DetachedCount, Attachment        uint8
	Ball                             Ball
	BallHolder                       int8
	AuxiliaryRelease, Computer       bool
}
type DunkObjectCommand struct{ Object, Action uint8 }
type DunkStep struct {
	Player           Player
	Ball             Ball
	TargetDurability int8
	AuxiliaryRelease bool
	SetActiveDunker  bool
	ActiveDunker     uint8
	Revisit          bool
	Objects          []DunkObjectCommand
}

// setCourtHalfPixel retains the original coordinate fraction and court-half
// component while replacing its within-half whole-pixel coordinate.
func setCourtHalfPixel(value Fixed, pixel uint8) Fixed {
	return (value &^ (255 * Pixel)) + Fixed(pixel)*Pixel
}

// StepDunkPlayer restores bank-6 $8E9D (actions 58/59/5B). Basket contacts determine
// scoring later; this step only performs the hang, release and dismount.
func StepDunkPlayer(original Player, ctx DunkContext, court FlatCourt) (DunkStep, error) {
	initial := DunkStep{Player: original, Ball: ctx.Ball, TargetDurability: ctx.TargetDurability, AuxiliaryRelease: ctx.AuxiliaryRelease}
	result := initial
	if !originalDunkAction(original.Action) || ctx.Actor >= 4 || ctx.TargetObject >= 13 {
		return initial, errors.New("invalid original dunk actor or target")
	}
	mount := ctx.TargetOwner & 127
	if mount < 7 || mount > 12 {
		return initial, errors.New("dunk target has no reconstructed hoop mount")
	}
	p := &result.Player
	b := &result.Ball
	isAux := ctx.AuxiliaryOwners[0] == int8(ctx.TargetObject) || ctx.AuxiliaryOwners[1] == int8(ctx.TargetObject)
	if !p.Entered {
		result.SetActiveDunker, result.ActiveDunker = true, ctx.Actor
		initializeAction(p)
		result.AuxiliaryRelease = false
		p.FacingLeft = ctx.TargetOwner&1 != 0
		p.Pose.Mirrored = p.FacingLeft
		stopHorizontal(p)
		stopVertical(p)
		if ctx.Attachment&128 != 0 {
			result.Revisit = true
		}
		if ctx.CarriedPlayer == int8(ctx.Actor) {
			result.Objects = append(result.Objects, DunkObjectCommand{Object: 4, Action: 12})
		}
		tier := (mount - 7) / 2
		playerX, ballX := uint8(77), uint8(69)
		if mount&1 == 0 {
			playerX, ballX = 195, 203
		}
		p.Body.Position.X = setCourtHalfPixel(p.Body.Position.X, playerX)
		p.Body.Position.Depth = 43*Pixel + (p.Body.Position.Depth & 255)
		height := Fixed(74+96*int(tier)) * Pixel
		if ctx.TargetOwner&128 != 0 {
			height += 32 * Pixel
		}
		p.Body.Position.Height = height + (p.Body.Position.Height & 255)
		b.Body.Position.X = setCourtHalfPixel(b.Body.Position.X, ballX)
		if ctx.DetachedCount < 4 && result.TargetDurability >= 0 {
			result.TargetDurability -= 2
		}
		p.Pose = Pose{Frame: 58, Mirrored: p.Body.Position.X>>16 == 0}
	}
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 44}, Ticks: 5}, {Pose: Pose{Frame: 45}, Ticks: 6}, {Pose: Pose{Frame: 4}, Ticks: 255}, {Pose: Pose{Frame: 4}, Ticks: 255}, {Pose: Pose{Frame: 4}, Ticks: 255}}, End: RepeatRemaining}
	if original.Action == 0x59 {
		spec.Frames[0] = AnimationFrame{Pose: Pose{Frame: 44, Mirrored: true}, Ticks: 3}
		spec.Frames[1] = AnimationFrame{Pose: Pose{Frame: 44, Mirrored: true}, Ticks: 2}
	} else if original.Action == 0x5b {
		spec.Frames[0].Pose.Frame = 51
	}
	animation, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if err != nil {
		return initial, err
	}
	p.Animation, p.Pose = animation.State, animation.Pose
	if animation.ChangeAction {
		p.Action, p.Entered = animation.Action, false
	}
	if p.Animation.Cursor != 0 && p.Animation.Cursor != 2 && p.Animation.Elapsed == 0 {
		b.Action, b.Entered = 10, false
		if p.Action != 0x59 {
			p.Body.Position.Height -= 16 * Pixel
		}
		if isAux {
			speed := -2 * Pixel
			if b.Body.Position.X>>16 == 0 {
				speed = 2 * Pixel
			}
			b.Body.Velocity.X = speed + (b.Body.Velocity.X & 255)
			b.Body.Velocity.Height = 4 * Pixel
			b.Body.Gravity = Pixel / 4
			result.AuxiliaryRelease = true
		} else {
			side := b.Body.Position.X >> 16
			if side < 0 || side > 1 {
				return initial, errors.New("released dunk ball outside court halves")
			}
			x := uint8(69)
			if side != 0 {
				x = 203
			}
			b.Body.Position.X = setCourtHalfPixel(b.Body.Position.X, x)
			b.Body.Position.Depth = 45*Pixel + (b.Body.Position.Depth & 255)
		}
	}
	if p.Animation.Cursor == 2 && p.Animation.Elapsed == 0 {
		p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 15*Pixel/4)
		p.Body.Gravity = signed16(p.Body.Gravity + 15*Pixel/8)
		action := uint8(3)
		if result.AuxiliaryRelease {
			action = 9
		}
		result.Objects = append(result.Objects, DunkObjectCommand{Object: ctx.TargetObject, Action: action})
	}
	ballCommand, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: ctx.Computer, HasBall: ctx.BallHolder == int8(ctx.Actor)}, court)
	if err != nil {
		return initial, err
	}
	result.Ball = ApplyBallCommand(result.Ball, ballCommand)
	if !p.Airborne {
		p.Action, p.Entered = ActionLand, false
		if result.TargetDurability < 0 && ctx.DetachedCount < 4 && isAux {
			object := uint8(4)
			if ctx.AuxiliaryOwners[0] != int8(ctx.TargetObject) {
				object = 5
			}
			result.Objects = append(result.Objects, DunkObjectCommand{Object: object, Action: 12})
		}
	}
	return result, nil
}

func originalDunkAction(action Action) bool {
	return action == 0x58 || action == 0x59 || action == 0x5b
}

type DunkBallStep struct {
	Ball          Ball
	ReleaseHolder bool
}

// StepDunkReleasedBall is $B8CB (ball action 10). Entry adds to existing
// vertical motion, releases possession, and lets normal court physics land it.
func StepDunkReleasedBall(original Ball, court FlatCourt) (DunkBallStep, error) {
	initial := DunkBallStep{Ball: original}
	result := initial
	b := &result.Ball
	if b.Action != 10 {
		return initial, errors.New("ball is not in dunk release")
	}
	if !b.Entered {
		b.Entered = true
		b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
		result.ReleaseHolder = true
		b.Body.Velocity.Height = signed16(b.Body.Velocity.Height + Pixel/16)
		b.Body.Gravity = signed16(b.Body.Gravity + Pixel/2)
	}
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 1}, Ticks: 2}, {Pose: Pose{Frame: 2}, Ticks: 2}, {Pose: Pose{Frame: 3}, Ticks: 2}, {Pose: Pose{Frame: 0}, Ticks: 2}}, End: RepeatForever}
	animation, err := AdvanceAnimation(spec, b.Animation, b.FacingLeft)
	if err != nil {
		return initial, err
	}
	b.Animation, b.Pose = animation.State, animation.Pose
	if err = advanceBallOnCourt(b, court); err != nil {
		return initial, err
	}
	if !b.Airborne {
		b.Action, b.Entered = BallBounce, false
	}
	return result, nil
}
