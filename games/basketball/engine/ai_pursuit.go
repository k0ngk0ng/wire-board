package engine

import (
	"errors"
	"fmt"
)

// The bit-8 column of the original bank-5 action flags at $9321. This is
// gameplay data, independent of the runtime ROM or an instruction interpreter.
var originalOpponentSearchFlags = [2]uint64{0xffbf7f3fbfbfdfbf, 0x3ffff3f00bf}

func originalOpponentSearchable(action Action) bool {
	return action < 128 && originalOpponentSearchFlags[action/64]&(uint64(1)<<(action%64)) != 0
}

// PursuitAlias74 preserves two original bytes with no established gameplay
// meaning. Explicit context distinguishes supplied zero from unknown memory.
// Its X coordinate is the current hoop9 height, not a configuration constant.
type PursuitAlias74 struct {
	ActionByte uint8 `json:"actionByte"`
	DepthByte  uint8 `json:"depthByte"`
}

// PursuitReservedBytes contains only reads whose original bytes have no
// established gameplay meaning. Supply from initial state, not later oracles.
// A nil context means unknown, unlike an explicitly supplied zero byte.
type PursuitReservedBytes struct {
	Depth88  *uint8 `json:"depth88,omitempty"`
	XHigh88  *uint8 `json:"xHigh88,omitempty"`
	Action73 uint8  `json:"action73"`
	Depth73  uint8  `json:"depth73"`
	Depth75  uint8  `json:"depth75"`
}

type originalOpponentProbe struct {
	Action   Action
	Position Position
}

// This table of canonical sources preserves B100's action+1 indexing quirk.
// Each read is independent: rejected actions never require depth or X state.
// Unsupported source bytes remain errors rather than simulated RAM defaults.
func originalPursuitAction(v AIControlView, index uint8) (Action, error) {
	if index < 4 {
		return v.PlayerActions[index] & 127, nil
	}
	switch index {
	case 7:
		return Action(v.Hoops[0].Action) & 127, nil
	case 31:
		aux := v.Auxiliary
		if v.PursuitWorld != nil {
			aux = &v.PursuitWorld.Auxiliary
		}
		if aux == nil {
			return 0, errors.New("original pursuit index31 requires auxiliary state")
		}
		if !aux[1].Animation.HasNextAction {
			return 127, nil
		}
		return aux[1].Animation.NextAction & 127, nil
	case 39, 40:
		if v.PlayerControls == nil {
			return 0, fmt.Errorf("original pursuit index%d requires player controls", index)
		}
		return Action(v.PlayerControls[index-39] & 127), nil
	case 48:
		return Action(uint8(v.Hoops[2].Owner) & 127), nil
	case 73:
		if v.PursuitReserved == nil {
			return 0, errors.New("original pursuit index73 requires explicit reserved bytes")
		}
		return Action(v.PursuitReserved.Action73 & 127), nil
	case 74:
		if v.PursuitAlias74 == nil {
			return 0, errors.New("original pursuit index74 requires explicit alias bytes")
		}
		return Action(v.PursuitAlias74.ActionByte & 127), nil
	case 14, 23, 55, 75, 76, 77, 79, 87, 88:
		if v.PursuitWorld == nil {
			return 0, fmt.Errorf("original pursuit index%d requires canonical world state", index)
		}
		w := v.PursuitWorld
		switch index {
		case 14:
			return w.Actors[1].Player.Request.Action & 127, nil
		case 23:
			return Action(w.Hoops[3].Request.Action) & 127, nil
		case 55:
			return Action(uint8(w.Actors[1].Dodge.InitialXHigh) & 127), nil
		case 75:
			return Action(uint8(w.SelectedHoop) & 127), nil
		case 76:
			return Action(uint8(w.ShotOriginX>>8) & 127), nil
		case 77:
			return Action(uint8(w.ShotOriginX>>16) & 127), nil
		case 79:
			return Action(uint8(w.ExcludedCatcher) & 127), nil
		case 87:
			return Action(uint8(w.CarriedPlayer) & 127), nil
		case 88:
			return Action(w.PoseSource & 127), nil
		}
	}
	return 0, fmt.Errorf("original pursuit index%d action source not reconstructed", index)
}

func originalPursuitDepth(v AIControlView, index uint8) (uint8, error) {
	if index < 4 {
		return uint8(v.Players[index].Depth >> 8), nil
	}
	switch index {
	case 7:
		return uint8(v.Hoops[0].Position.Depth >> 8), nil
	case 14:
		return uint8(v.Players[1].Height), nil
	case 23:
		return uint8(v.Hoops[3].Position.Height), nil
	case 31:
		aux := v.Auxiliary
		if v.PursuitWorld != nil {
			aux = &v.PursuitWorld.Auxiliary
		}
		if aux == nil {
			return 0, errors.New("original pursuit index31 requires auxiliary state")
		}
		return uint8(aux[1].Body.Position.Height >> 8), nil
	case 39, 40:
		return uint8(v.Players[index-39].Height >> 16), nil
	case 48:
		return uint8(v.Hoops[2].Position.Height >> 16), nil
	case 73, 75:
		if v.PursuitReserved == nil {
			return 0, fmt.Errorf("original pursuit index%d requires explicit reserved depth", index)
		}
		if index == 73 {
			return v.PursuitReserved.Depth73, nil
		}
		return v.PursuitReserved.Depth75, nil
	case 88:
		if v.PursuitReserved == nil || v.PursuitReserved.Depth88 == nil {
			return 0, errors.New("original pursuit index88 requires explicit depth byte")
		}
		return *v.PursuitReserved.Depth88, nil
	case 74:
		if v.PursuitAlias74 == nil {
			return 0, errors.New("original pursuit index74 requires explicit alias bytes")
		}
		return v.PursuitAlias74.DepthByte, nil
	}
	return 0, fmt.Errorf("original pursuit index%d depth source not reconstructed", index)
}

func originalPursuitX(v AIControlView, index uint8) (uint16, error) {
	if index < 4 {
		return uint16(v.Players[index].X >> 8), nil
	}
	switch index {
	case 7:
		return uint16(v.Hoops[0].Position.X >> 8), nil
	case 14:
		p := v.Players[1]
		return uint16(uint8(p.X>>16)) | uint16(uint8(p.Depth))<<8, nil
	case 23:
		p := v.Hoops[3].Position
		return uint16(uint8(p.X>>16)) | uint16(uint8(p.Depth))<<8, nil
	case 31:
		aux := v.Auxiliary
		if v.PursuitWorld != nil {
			aux = &v.PursuitWorld.Auxiliary
		}
		if aux == nil {
			return 0, errors.New("original pursuit index31 requires auxiliary state")
		}
		return uint16(aux[1].Body.Position.Depth), nil
	case 39, 40:
		p := v.Players[index-39]
		return uint16(uint8(p.Depth>>8)) | uint16(uint8(p.Height))<<8, nil
	case 48:
		p := v.Hoops[2].Position
		return uint16(uint8(p.Depth>>8)) | uint16(uint8(p.Height))<<8, nil
	case 55:
		return uint16(v.Players[3].Height), nil
	case 88:
		if v.PursuitReserved == nil || v.PursuitReserved.XHigh88 == nil {
			return 0, errors.New("original pursuit index88 requires explicit X high byte")
		}
		return uint16(uint8(v.Hoops[3].Position.Height>>16)) | uint16(*v.PursuitReserved.XHigh88)<<8, nil
	case 73, 74, 75, 76, 77:
		return uint16(v.Hoops[index-72].Position.Height >> 8), nil
	}
	return 0, fmt.Errorf("original pursuit index%d X source not reconstructed", index)
}

func originalPursuitProbe(v AIControlView, index uint8) (originalOpponentProbe, error) {
	a, err := originalPursuitAction(v, index)
	if err != nil {
		return originalOpponentProbe{}, err
	}
	d, err := originalPursuitDepth(v, index)
	if err != nil {
		return originalOpponentProbe{}, err
	}
	x, err := originalPursuitX(v, index)
	if err != nil {
		return originalOpponentProbe{}, err
	}
	return originalOpponentProbe{Action: a, Position: Position{X: Fixed(x) * Pixel, Depth: Fixed(d) * Pixel}}, nil
}

func opponentNearOriginalBall(view AIControlView) (int8, error) {
	candidate := (view.Actor & 2) ^ 2
	for visit := 0; visit < 2; visit++ {
		action, err := originalPursuitAction(view, candidate)
		if err != nil {
			return -1, err
		}
		if !originalOpponentSearchable(action) {
			candidate = uint8(action) + 1
			continue
		}
		depth, err := originalPursuitDepth(view, candidate)
		if err != nil {
			return -1, err
		}
		dz := uint8(depth - uint8(view.BallPosition.Depth>>8))
		if dz >= 16 && dz < 240 {
			candidate++
			continue
		}
		x, err := originalPursuitX(view, candidate)
		if err != nil {
			return -1, err
		}
		dx := int16(x - uint16(view.BallPosition.X>>8))
		if dx >= -32 && dx < 32 {
			return int8(candidate), nil
		}
		candidate++
	}
	return -1, nil
}

func advanceOriginalPursuit(result *AIControlDecision, view AIControlView) error {
	if !view.TeamHasHuman {
		target, err := opponentNearOriginalBall(view)
		if err != nil {
			return err
		}
		if target >= 0 {
			if view.PursuitOptions == nil {
				return errors.New("original character pursuit options are missing")
			}
			result.Control.TrackedObject = target
			var value byte
			result.Random, value = NextRandom(result.Random)
			choice := view.PursuitOptions[(value>>1)&3]
			result.Control.Destination, result.Control.PlanParameter = choice.Destination, choice.Parameter
			probe, err := originalPursuitProbe(view, uint8(target))
			if err != nil {
				return err
			}
			position := probe.Position
			// $AE5A passes the opponent's depth as the X-alignment distance.
			approach := steerOriginalPoint(result, view.Position, position, uint8(position.Depth>>8), 4, uint8(choice.Destination))
			if approach.Reached {
				result.Control.Mode, result.Control.Entered = 7, false
			} else if approach.DepthSeparated && approach.XDistance < 256 {
				chooseAIAction(result, ActionWalk)
				result.FacingLeft = view.Position.X>>8 >= position.X>>8
			} else {
				chooseAIAction(result, ActionRun)
			}
			return nil
		}
	}
	return reactOriginalBall(result, view)
}
