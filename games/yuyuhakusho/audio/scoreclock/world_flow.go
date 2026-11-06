package scoreclock

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"

type BattleWorldFlow struct {
	Prefix   BattleWorldEntry
	Dispatch BattleWorldDispatch
}

// AdvanceBattleWorldIRQToLogic carries the actual RAM prefix, transfer wrapper,
// interrupted video/audio frames and logical-World dispatch on one service and
// operation budget. The caller still supplies detached canonical owners.
func AdvanceBattleWorldIRQToLogic(clock Clock, context BattleMainContext, input BattleWorldInput, graphics *BattleGraphics, flags *BattleWorldFlags, world *engine.ChuWorldState, device BattleWorldClockDevice, observer BattleWorldObserver, limits Limits) (BattleWorldFlow, error) {
	if graphics == nil || flags == nil || world == nil || device == nil || len(context.DMAWindows) > 1 {
		return BattleWorldFlow{}, ErrBattleWorldProfile
	}
	start := uint32(context.IRQ.Video.Line) * 3420
	for _, v := range context.DMAWindows {
		if v.Begin != 0 || v.End > start || v.Remaining != 0 || v.Mode > 1 {
			return BattleWorldFlow{}, ErrBattleWorldProfile
		}
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleWorldFlow{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	w, err := newWorldClockEntry(s, context, graphics, flags, world, device, observer, 4)
	if err != nil {
		return BattleWorldFlow{}, err
	}
	w.p.cachedDisplay = &graphics.CachedDisplay
	prefix, err := w.p.prepareWorld(input, flags, &world.Clock.AudioPause, device)
	if err != nil {
		return BattleWorldFlow{}, err
	}
	dispatch, err := w.run()
	if err != nil {
		return BattleWorldFlow{}, err
	}
	return BattleWorldFlow{Prefix: prefix, Dispatch: dispatch}, nil
}
