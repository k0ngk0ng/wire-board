package session

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type battleWorldDevice struct{ *mixed.Runtime }

func (a battleWorldDevice) PCMFrontier() uint64 { return a.Snapshot().PCM.Time }
func (a battleWorldDevice) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	return a.Runtime.RunUntil(at, dma)
}

// BeginBattleWorldIRQ stages the real transfer gate, bus, video and main work
// on the same actual audio owners. The wrapper/DMA/World call remains pending.
// The returned main context and flags belong to the enclosing main transaction.
func (r *Runtime) BeginBattleWorldIRQ(clock scoreclock.Clock, context scoreclock.BattleSampleContext, input scoreclock.BattleWorldInput, flags *scoreclock.BattleWorldFlags, pause *uint8, limits scoreclock.Limits) (scoreclock.BattleWorldEntry, error) {
	if flags == nil || pause == nil {
		return scoreclock.BattleWorldEntry{}, errors.New("missing World IRQ owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.BattleWorldEntry{}, err
	}
	stagedFlags, stagedPause := *flags, *pause
	result, err := scoreclock.PrepareBattleWorldIRQ(clock, context, input, &stagedFlags, &stagedPause, battleWorldDevice{next.mixed}, nil, limits)
	if err != nil {
		return scoreclock.BattleWorldEntry{}, err
	}
	*r = *next
	*flags, *pause = stagedFlags, stagedPause
	return result, nil
}
