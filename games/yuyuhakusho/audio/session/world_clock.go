package session

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// PrepareBattleWorldLogic stages the real World wrapper and its interrupted
// video/audio frames together. The returned boundary is still before logical
// World work or the outer busy-IRQ epilogue. Their caller owns the main context;
// no second clock, world, graphics queue or output history enters audio.State.
func (r *Runtime) PrepareBattleWorldLogic(clock scoreclock.Clock, context scoreclock.BattleMainContext, graphics *scoreclock.BattleGraphics, flags *scoreclock.BattleWorldFlags, world *engine.ChuWorldState, limits scoreclock.Limits) (scoreclock.BattleWorldDispatch, error) {
	if graphics == nil || flags == nil || world == nil {
		return scoreclock.BattleWorldDispatch{}, errors.New("missing joint World clock owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.BattleWorldDispatch{}, err
	}
	stagedGraphics, stagedFlags, stagedWorld := *graphics, *flags, *world
	if graphics.Pending != nil {
		stagedGraphics.Pending = append(make([]scoreclock.BattleWorldTransfer, 0, len(graphics.Pending)), graphics.Pending...)
	}
	result, err := scoreclock.AdvanceBattleWorldWrapper(clock, context, &stagedGraphics, &stagedFlags, &stagedWorld, battleWorldDevice{next.mixed}, nil, limits)
	if err != nil {
		return scoreclock.BattleWorldDispatch{}, err
	}
	*r = *next
	*graphics, *flags, *world = stagedGraphics, stagedFlags, stagedWorld
	return result, nil
}
