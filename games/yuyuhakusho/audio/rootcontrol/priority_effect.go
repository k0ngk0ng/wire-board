package rootcontrol

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func (r *ruleTick) applyPriorityClear(op score.Operation) error {
	if r.hooks.PriorityClear == nil {
		r.root.Priority = 0
		return nil
	}
	active, called, violated := true, false, false
	apply := func() error {
		if !active || called {
			violated = true
			return fmt.Errorf("%w: priority effect repeated or expired", ErrRuleUnsupported)
		}
		called = true
		r.root.Priority = 0
		return nil
	}
	err := r.hooks.PriorityClear(op, apply)
	active = false
	if err != nil {
		return err
	}
	if !called || violated {
		return fmt.Errorf("%w: priority effect was not applied exactly once", ErrRuleUnsupported)
	}
	return nil
}
