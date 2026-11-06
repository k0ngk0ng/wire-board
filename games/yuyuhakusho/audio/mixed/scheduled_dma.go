package mixed

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"

func (r *Runtime) DMAWaiting() bool { return r.pcm.DMAWaiting() }

func (r *Runtime) RunDMAUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	var result pcmcommands.RunResult
	err := r.atomic(func(next *Runtime) error {
		var err error
		result, err = next.pcm.RunDMAUntil(at, dma)
		if err != nil {
			return err
		}
		return next.consume(next.pcm.DrainLedger())
	})
	if err != nil {
		return pcmcommands.RunResult{}, err
	}
	return result, nil
}

func (r *Runtime) AdvanceDMA(end uint64, remaining uint32) error {
	return r.atomic(func(next *Runtime) error { return next.pcm.AdvanceDMA(end, remaining) })
}
