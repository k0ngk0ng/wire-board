package session

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// BeginBattleWorldLogic stages the actual frame prefix and any interrupted
// video/audio frames. Returned state still awaits object preparation/callbacks.
func (r *Runtime) BeginBattleWorldLogic(clock scoreclock.Clock, context scoreclock.BattleMainContext, graphics *scoreclock.BattleGraphics, flags *scoreclock.BattleWorldFlags, world *engine.ChuWorldState, input engine.ControllerNormalizationInput, limits scoreclock.Limits) (scoreclock.BattleWorldLogicBegin, error) {
	if graphics == nil || flags == nil || world == nil {
		return scoreclock.BattleWorldLogicBegin{}, errors.New("missing live logical frame owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.BattleWorldLogicBegin{}, err
	}
	stagedGraphics, stagedFlags, stagedWorld := *graphics, *flags, *world
	if graphics.Pending != nil {
		stagedGraphics.Pending = append(make([]scoreclock.BattleWorldTransfer, 0, len(graphics.Pending)), graphics.Pending...)
	}
	result, err := scoreclock.AdvanceBattleWorldLogicBegin(clock, context, &stagedGraphics, &stagedFlags, &stagedWorld, input, battleWorldDevice{next.mixed}, nil, limits)
	if err != nil {
		return scoreclock.BattleWorldLogicBegin{}, err
	}
	*r = *next
	*graphics, *flags, *world = stagedGraphics, stagedFlags, stagedWorld
	return result, nil
}
