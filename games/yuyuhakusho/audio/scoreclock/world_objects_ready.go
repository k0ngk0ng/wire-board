package scoreclock

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"

type BattleWorldObjectsReady struct {
	BattleWorldLogicBegin
	BusyReturned bool
}

// AdvanceBattleWorldIRQToObjects uses one service through both the actual IRQ
// prefix/transfer wrapper and the complete logical prelude. It returns each new
// audio frame once. Object preparation, callbacks and outer IRQ tail remain
// explicit pending work; a busy wrapper does not run another logical frame.
func AdvanceBattleWorldIRQToObjects(clock Clock, context BattleMainContext, physical BattleWorldInput, graphics *BattleGraphics, flags *BattleWorldFlags, world *engine.ChuWorldState, input engine.ControllerNormalizationInput, device BattleWorldClockDevice, observer BattleWorldObserver, limits Limits) (BattleWorldObjectsReady, error) {
	if graphics == nil || flags == nil || world == nil || device == nil || len(context.DMAWindows) > 1 {
		return BattleWorldObjectsReady{}, ErrBattleWorldProfile
	}
	start := uint32(context.IRQ.Video.Line) * 3420
	for _, v := range context.DMAWindows {
		if v.Begin != 0 || v.End > start || v.Remaining != 0 || v.Mode > 1 {
			return BattleWorldObjectsReady{}, ErrBattleWorldProfile
		}
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleWorldObjectsReady{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	w, err := newWorldClockEntry(s, context, graphics, flags, world, device, observer, 4)
	if err != nil {
		return BattleWorldObjectsReady{}, err
	}
	w.p.cachedDisplay = &graphics.CachedDisplay
	if _, err := w.p.prepareWorld(physical, flags, &world.Clock.AudioPause, device); err != nil {
		return BattleWorldObjectsReady{}, err
	}
	dispatch, err := w.run()
	if err != nil {
		return BattleWorldObjectsReady{}, err
	}
	if dispatch.BusyReturned {
		return BattleWorldObjectsReady{BattleWorldLogicBegin: BattleWorldLogicBegin{Clock: dispatch.Clock, Context: dispatch.Context, Operations: dispatch.Operations, Frames: dispatch.Frames}, BusyReturned: true}, nil
	}
	logic, err := w.beginLogic(input)
	if err != nil {
		return BattleWorldObjectsReady{}, err
	}
	return BattleWorldObjectsReady{BattleWorldLogicBegin: logic}, nil
}
