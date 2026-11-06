package session

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"

// WorldHook receives the detached audio branch and the actual world owners.
// Its synchronous effects commit only if the entire logical frame succeeds.
// No CPU pricing or IRQ acceptance policy is supplied by this API.
type WorldHook func(*Runtime, engine.WorldSoundOperation, engine.WorldSoundOwners, func() error) error

// WorldFrame composes actual world producers and the sound runtime atomically,
// including failures after the last producer in object or palette work. The
// caller supplies the verified temporal policy; nil leaves audio unchanged.
// The hook must not retain the branch, owners, or apply callback.
func (r *Runtime) WorldFrame(world engine.ChuWorldState, held [8]engine.Buttons, rules engine.ChuWorldLibrary, hook WorldHook) (engine.ChuWorldState, []engine.ChuWorldEvent, engine.WorldFrameOutcome, error) {
	if hook == nil {
		return engine.StepChuSessionFrame(world, held, rules)
	}
	next, err := r.fork()
	if err != nil {
		return world, nil, engine.WorldFrameOutcome{}, err
	}
	state, events, outcome, err := engine.StepChuSessionFrameObserved(world, held, rules, func(op engine.WorldSoundOperation, owners engine.WorldSoundOwners, apply func() error) error {
		return hook(next, op, owners, apply)
	})
	if err != nil {
		return world, nil, engine.WorldFrameOutcome{}, err
	}
	*r = *next
	return state, events, outcome, nil
}
