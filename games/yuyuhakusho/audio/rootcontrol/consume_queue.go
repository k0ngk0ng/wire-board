package rootcontrol

import "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"

func (r *ruleTick) consumeQueue(priorities engine.SoundPriorityTable) error {
	present := false
	if r.hooks.QueueTest == nil {
		present = *r.owners.Queue != ([4]uint8{})
	} else {
		active, called, violated := true, false, false
		sampled := false
		read := func() bool {
			if !active || called {
				violated = true
				return false
			}
			called = true
			sampled = *r.owners.Queue != ([4]uint8{})
			return sampled
		}
		value, err := r.hooks.QueueTest(read)
		active = false
		if err != nil {
			return err
		}
		if !called || violated || value != sampled {
			return ErrRuleUnsupported
		}
		present = value
	}
	if !present {
		return nil
	}
	return engine.ConsumeSoundQueueObserved(engine.SoundQueueOwners{Priority: &r.root.Priority, Selected: &r.root.Selected, Queue: r.owners.Queue}, priorities, r.hooks.QueueOperation)
}
