// Package session owns one canonical audio checkpoint. It borrows World queue,
// pause and main clocks only for each actual call, never persisting copies.
package session

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type Config struct {
	PCMOrigin uint64
	Chips     chips.Config
}
type State struct {
	Version    uint8
	ResourceID string
	Root       rootcontrol.State
	Score      score.State
	Scheduler  engine.SoundSchedulerState
	Mixed      mixed.State
}
type Runtime struct {
	library   *resourcebridge.Library
	config    Config
	root      rootcontrol.State
	score     *score.Driver
	scheduler engine.SoundSchedulerState
	mixed     *mixed.Runtime
}

func New(library *resourcebridge.Library, config Config) (*Runtime, error) {
	if library == nil {
		return nil, errors.New("missing audio session resources")
	}
	pcm, err := library.PCMConfig(config.PCMOrigin, 64)
	if err != nil {
		return nil, err
	}
	device, err := mixed.New(mixed.Config{PCM: pcm, Chips: config.Chips}, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
	if err != nil {
		return nil, err
	}
	driver, err := score.New(library.Catalog())
	if err != nil {
		return nil, err
	}
	return &Runtime{library: library, config: config, root: rootcontrol.InitialState(), score: driver, mixed: device}, nil
}
func Restore(library *resourcebridge.Library, config Config, state State) (*Runtime, error) {
	if library == nil || state.Version != 1 || state.ResourceID != library.Identity() {
		return nil, errors.New("audio session resource/checkpoint identity differs")
	}
	root, err := rootcontrol.RestoreState(state.Root)
	if err != nil {
		return nil, err
	}
	driver, err := score.Restore(library.Catalog(), state.Score)
	if err != nil {
		return nil, err
	}
	pcm, err := library.PCMConfig(config.PCMOrigin, 64)
	if err != nil {
		return nil, err
	}
	device, err := mixed.Restore(mixed.Config{PCM: pcm, Chips: config.Chips}, state.Mixed)
	if err != nil {
		return nil, err
	}
	return &Runtime{library: library, config: config, root: root, score: driver, scheduler: state.Scheduler, mixed: device}, nil
}
func (r *Runtime) Snapshot() State {
	root := r.root
	if root.PauseRestore != nil {
		levels := *root.PauseRestore
		root.PauseRestore = &levels
	}
	return State{Version: 1, ResourceID: r.library.Identity(), Root: root, Score: r.score.Snapshot(), Scheduler: r.scheduler, Mixed: r.mixed.Snapshot()}
}
func (r *Runtime) fork() (*Runtime, error) {
	root, err := rootcontrol.RestoreState(r.root)
	if err != nil {
		return nil, err
	}
	driver, err := score.Restore(r.library.Catalog(), r.score.Snapshot())
	if err != nil {
		return nil, err
	}
	device, err := r.mixed.Fork()
	if err != nil {
		return nil, err
	}
	return &Runtime{library: r.library, config: r.config, root: root, score: driver, scheduler: r.scheduler, mixed: device}, nil
}
func (r *Runtime) Interrupt(clock scoreclock.Clock, queue *[4]uint8, pause *uint8, priorities engine.SoundPriorityTable, limits scoreclock.Limits) (scoreclock.SoundIRQResult, error) {
	return r.advance(clock, queue, pause, priorities, limits, false)
}
func (r *Runtime) Catchup(clock scoreclock.Clock, queue *[4]uint8, pause *uint8, priorities engine.SoundPriorityTable, limits scoreclock.Limits) (scoreclock.SoundIRQResult, error) {
	return r.advance(clock, queue, pause, priorities, limits, true)
}
func (r *Runtime) advance(clock scoreclock.Clock, queue *[4]uint8, pause *uint8, priorities engine.SoundPriorityTable, limits scoreclock.Limits, catchup bool) (scoreclock.SoundIRQResult, error) {
	if queue == nil || pause == nil {
		return scoreclock.SoundIRQResult{}, errors.New("missing external World audio owners")
	}
	next, err := r.fork()
	if err != nil {
		return scoreclock.SoundIRQResult{}, err
	}
	queued, paused := *queue, *pause
	owners := rootcontrol.RuleOwners{Score: next.score, Queue: &queued, AudioPause: &paused}
	input := rootcontrol.RuleInput{Priorities: priorities}
	var result scoreclock.SoundIRQResult
	if catchup {
		result, err = scoreclock.AdvanceSoundCatchup(clock, &next.scheduler, &next.root, owners, input, next.library, next.mixed, next.mixed, limits)
	} else {
		result, err = scoreclock.AdvanceSoundInterrupt(clock, &next.scheduler, &next.root, owners, input, next.library, next.mixed, next.mixed, limits)
	}
	if err != nil {
		return scoreclock.SoundIRQResult{}, err
	}
	*r = *next
	*queue, *pause = queued, paused
	return result, nil
}

// AdvancePCM retains the real contention result for the external main owner;
// it neither chooses a world barrier nor silently charges/discards that delay.
func (r *Runtime) AdvancePCM(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	next, err := r.fork()
	if err != nil {
		return pcmcommands.RunResult{}, err
	}
	out, err := next.mixed.RunUntil(at, dma)
	if err != nil {
		return pcmcommands.RunResult{}, err
	}
	*r = *next
	return out, nil
}

// FrameBoundary requires the actual PCM frontier/hold to have reached its
// reference boundary. It is explicit and never invents a hidden RunUntil.
func (r *Runtime) FrameBoundary() (mixed.Frame, error) {
	next, err := r.fork()
	if err != nil {
		return mixed.Frame{}, err
	}
	out, err := next.mixed.FrameBoundary()
	if err != nil {
		return mixed.Frame{}, err
	}
	*r = *next
	return out, nil
}
