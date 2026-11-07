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
	return r.WorldFrameWithHooks(world, held, rules, WorldFrameHooks{Sound: hook})
}

type WorldFrameHooks struct {
	Sound     WorldHook
	Work      func(*Runtime, engine.WorldWorkOperation, engine.WorldSoundOwners) error
	Prefix    func(*Runtime, engine.MainWorkOperation, engine.WorldFrameOwners) error
	Normalize func(*Runtime, engine.WorldFrameOwners, engine.MainWorkHook) error
	Owners    func(*Runtime, *engine.ChuWorldState) error
}

// WorldFrameWithHooks stages audio effects at both requests and default-arena
// work boundaries. The caller must stage its own main clock; this API does not
// yet own the complete world's temporal checkpoint or IRQ acceptance policy.
func (r *Runtime) WorldFrameWithHooks(world engine.ChuWorldState, held [8]engine.Buttons, rules engine.ChuWorldLibrary, hooks WorldFrameHooks) (engine.ChuWorldState, []engine.ChuWorldEvent, engine.WorldFrameOutcome, error) {
	if hooks.Sound == nil && hooks.Work == nil && hooks.Prefix == nil && hooks.Normalize == nil && hooks.Owners == nil {
		return engine.StepChuSessionFrame(world, held, rules)
	}
	next, err := r.fork()
	if err != nil {
		return world, nil, engine.WorldFrameOutcome{}, err
	}
	var observers engine.ChuWorldHooks
	if hooks.Owners != nil {
		observers.Owners = func(state *engine.ChuWorldState) error { return hooks.Owners(next, state) }
	}
	if hooks.Prefix != nil || hooks.Normalize != nil {
		observers.Frame = &engine.WorldFrameWorkHooks{}
		if hooks.Prefix != nil {
			observers.Frame.Work = func(op engine.MainWorkOperation, owners engine.WorldFrameOwners) error {
				return hooks.Prefix(next, op, owners)
			}
		}
		if hooks.Normalize != nil {
			observers.Frame.Normalize = func(owners engine.WorldFrameOwners, work engine.MainWorkHook) error {
				return hooks.Normalize(next, owners, work)
			}
		}
	}
	if hooks.Sound != nil {
		observers.Sound = func(op engine.WorldSoundOperation, owners engine.WorldSoundOwners, apply func() error) error {
			return hooks.Sound(next, op, owners, apply)
		}
	}
	if hooks.Work != nil {
		observers.Work = func(op engine.WorldWorkOperation, owners engine.WorldSoundOwners) error {
			return hooks.Work(next, op, owners)
		}
	}
	state, events, outcome, err := engine.StepChuSessionFrameWithHooks(world, held, rules, observers)
	if err != nil {
		return world, nil, engine.WorldFrameOutcome{}, err
	}
	*r = *next
	return state, events, outcome, nil
}
