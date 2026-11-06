package session

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// PrepareBattleWorldIRQLogic commits only after the complete World IRQ prefix,
// transfer wrapper and interrupted frames reach the explicit logical-World (or
// busy epilogue) boundary. Earlier stage failures never expose partial frames.
func (r *Runtime) PrepareBattleWorldIRQLogic(clock scoreclock.Clock, context scoreclock.BattleMainContext, input scoreclock.BattleWorldInput, graphics *scoreclock.BattleGraphics, flags *scoreclock.BattleWorldFlags, world *engine.ChuWorldState, limits scoreclock.Limits) (scoreclock.BattleWorldFlow, error) {
	if graphics == nil || flags == nil || world == nil {
		return scoreclock.BattleWorldFlow{}, errors.New("missing combined World IRQ owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.BattleWorldFlow{}, err
	}
	stagedGraphics, stagedFlags, stagedWorld := *graphics, *flags, *world
	if graphics.Pending != nil {
		stagedGraphics.Pending = append(make([]scoreclock.BattleWorldTransfer, 0, len(graphics.Pending)), graphics.Pending...)
	}
	result, err := scoreclock.AdvanceBattleWorldIRQToLogic(clock, context, input, &stagedGraphics, &stagedFlags, &stagedWorld, battleWorldDevice{next.mixed}, nil, limits)
	if err != nil {
		return scoreclock.BattleWorldFlow{}, err
	}
	*r = *next
	*graphics, *flags, *world = stagedGraphics, stagedFlags, stagedWorld
	return result, nil
}
