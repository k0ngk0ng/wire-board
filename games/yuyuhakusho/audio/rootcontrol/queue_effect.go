package rootcontrol

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func (r *ruleTick) applyQueueReplace(op score.Operation) error {
	if r.hooks.QueueReplace == nil {
		r.owners.Queue[0] = op.Writes[0].SoundID
		return nil
	}
	active, called, violated := true, false, false
	apply := func() error {
		if !active || called {
			violated = true
			return fmt.Errorf("%w: queue effect repeated or expired", ErrRuleUnsupported)
		}
		called = true
		r.owners.Queue[0] = op.Writes[0].SoundID
		return nil
	}
	err := r.hooks.QueueReplace(op, apply)
	active = false
	if err != nil {
		return err
	}
	if !called || violated {
		return fmt.Errorf("%w: queue effect was not applied exactly once", ErrRuleUnsupported)
	}
	return nil
}
