package engine

import "errors"

type CatchPlayer struct {
	Action             Action
	Control, Flags     uint8
	Attached, Airborne bool
}

type AutomaticCatchContext struct {
	Candidate                                          int8
	Holder, ExcludedCatcher, PreviousHolder, LastTouch int8
	BallAction                                         BallAction
	Players                                            [4]CatchPlayer
	AuxiliaryOwners                                    [2]int8
	HoopOwners                                         [6]int8
}

type CatchKind uint8

const (
	NoAutomaticCatch CatchKind = iota
	RunningCatch
	AttachedCatch
	AirCatch
	GroundCatch
	OccupiedHandsPassImpact
)

type CatchResolution struct {
	Holder, LastTouch int8
	Actor             int8
	Caught            bool
	Kind              CatchKind
	ChangePlayer      bool
	PlayerAction      Action
	Ball              BallCommand
}

// ResolveAutomaticCatch runs after attack/interactions have updated actors.
// Candidate selection comes from actual frame-box contact, not a fixed radius.
// The emitted state changes are applied before player/ball action updates.
func ResolveAutomaticCatch(ctx AutomaticCatchContext) (CatchResolution, error) {
	result := CatchResolution{Holder: ctx.Holder, LastTouch: ctx.LastTouch, Actor: -1}
	if ctx.Candidate < 0 {
		return result, nil
	}
	if ctx.Candidate >= 4 {
		return result, errors.New("auxiliary-object contact requires the interaction resolver")
	}
	if ctx.Holder != 6 || ctx.BallAction == BallRest {
		return result, nil
	}
	if ctx.BallAction >= 13 && ctx.BallAction != 21 {
		return result, errors.New("special-ball contact requires the interaction resolver")
	}
	actor := ctx.Candidate
	p := ctx.Players[actor]
	if p.Control == 0xc0 || p.Action >= 0x60 || p.Flags&16 == 0 || actor == ctx.ExcludedCatcher {
		return result, nil
	}
	if p.Attached {
		if actor == ctx.PreviousHolder {
			return result, nil
		}
		for _, owner := range ctx.AuxiliaryOwners {
			if owner == actor {
				return result, nil
			}
		}
		for _, owner := range ctx.HoopOwners {
			if owner == actor {
				return result, nil
			}
		}
	}
	kind := RunningCatch
	if p.Action != ActionRun {
		kind = AttachedCatch
		if !p.Attached {
			kind = AirCatch
			if !p.Airborne {
				kind = GroundCatch
				group := p.Action & 0x70
				if group != 0 && group != 0x40 {
					if ctx.BallAction != BallPass {
						return result, nil
					}
					kind = OccupiedHandsPassImpact
				}
			}
		}
	}
	result.Actor, result.Kind, result.LastTouch = actor, kind, actor
	result.ChangePlayer = true
	result.Ball = BallCommand{Change: true, Action: BallHeld}
	switch kind {
	case RunningCatch:
		result.PlayerAction = ActionDribbleRun
		result.Ball.Action = BallDribble
	case AttachedCatch, GroundCatch:
		result.PlayerAction = ActionReceiveBall
	case AirCatch:
		result.PlayerAction = ActionCatchAir
	case OccupiedHandsPassImpact:
		result.PlayerAction = Action(0x48)
		result.Ball.Action = BallKnockback
	}
	if kind != OccupiedHandsPassImpact {
		result.Holder = actor
		result.Caught = true
	}
	return result, nil
}
