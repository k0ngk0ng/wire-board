package session

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// PrepareBattleWorldObjects is one rollback boundary from the RAM World IRQ
// through actual DMA/VINT/audio frames and input/clock/RNG/feedback work.
func (r *Runtime) PrepareBattleWorldObjects(clock scoreclock.Clock, context scoreclock.BattleMainContext, physical scoreclock.BattleWorldInput, graphics *scoreclock.BattleGraphics, flags *scoreclock.BattleWorldFlags, world *engine.ChuWorldState, input engine.ControllerNormalizationInput, limits scoreclock.Limits) (scoreclock.BattleWorldObjectsReady, error) {
	if graphics == nil || flags == nil || world == nil {
		return scoreclock.BattleWorldObjectsReady{}, errors.New("missing combined pre-object World owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.BattleWorldObjectsReady{}, err
	}
	stagedGraphics, stagedFlags, stagedWorld := *graphics, *flags, *world
	if graphics.Pending != nil {
		stagedGraphics.Pending = append(make([]scoreclock.BattleWorldTransfer, 0, len(graphics.Pending)), graphics.Pending...)
	}
	result, err := scoreclock.AdvanceBattleWorldIRQToObjects(clock, context, physical, &stagedGraphics, &stagedFlags, &stagedWorld, input, battleWorldDevice{next.mixed}, nil, limits)
	if err != nil {
		return scoreclock.BattleWorldObjectsReady{}, err
	}
	*r = *next
	*graphics, *flags, *world = stagedGraphics, stagedFlags, stagedWorld
	return result, nil
}
