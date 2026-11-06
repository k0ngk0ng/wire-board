package session

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type battleAux struct{ mixed *mixed.Runtime }

func (a battleAux) PCMFrontier() uint64 { return a.mixed.Snapshot().PCM.Time }
func (a battleAux) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	return a.mixed.RunUntil(at, dma)
}

// BeginBattleSampleIRQ stages the real RAM sampling prefix, controller scan,
// auxiliary progress and whole sound driver together. Its returned context is
// borrowed by the main owner, never copied into the audio checkpoint. The IRQ
// epilogue remains pending; this method does not resume World execution.
func (r *Runtime) BeginBattleSampleIRQ(clock scoreclock.Clock, context scoreclock.BattleSampleContext, physical scoreclock.BattleSampleInput, queue *[4]uint8, pause *uint8, priorities engine.SoundPriorityTable, limits scoreclock.Limits) (scoreclock.BattleSampleResult, error) {
	if queue == nil || pause == nil {
		return scoreclock.BattleSampleResult{}, errors.New("missing World sampling IRQ owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.BattleSampleResult{}, err
	}
	queued, paused := *queue, *pause
	owners := rootcontrol.RuleOwners{Score: next.score, Queue: &queued, AudioPause: &paused}
	result, err := scoreclock.AdvanceBattleSampleIRQ(clock, context, physical, battleAux{next.mixed}, nil, &next.scheduler, &next.root, owners, rootcontrol.RuleInput{Priorities: priorities}, next.library, next.mixed, next.mixed, limits)
	if err != nil {
		return scoreclock.BattleSampleResult{}, err
	}
	*r = *next
	*queue, *pause = queued, paused
	return result, nil
}
