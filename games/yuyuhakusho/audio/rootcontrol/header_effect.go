package rootcontrol

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"

func (r *ruleTick) headerEffect(op score.Operation, effect func() error) error {
	if r.hooks.HeaderEffect == nil {
		return effect()
	}
	active, called, violated := true, false, false
	apply := func() error {
		if !active || called {
			violated = true
			return ErrRuleUnsupported
		}
		called = true
		return effect()
	}
	err := r.hooks.HeaderEffect(op, apply)
	active = false
	if err != nil {
		return err
	}
	if !called || violated {
		return ErrRuleUnsupported
	}
	return nil
}
