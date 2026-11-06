package rootcontrol

// ruleEffect preserves the exact enriched observation while leasing its sole
// owner mutation synchronously. Failed detached owners are discarded together.
func (r *ruleTick) ruleEffect(op RuleOperation, hook func(RuleOperation, func() error) error, effect func() error) error {
	op.Version, op.Sequence, op.Phase = 1, r.sequence, r.phase
	if err := r.emit(op); err != nil {
		return err
	}
	if hook == nil {
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
	err := hook(op, apply)
	active = false
	if err != nil {
		return err
	}
	if !called || violated {
		return ErrRuleUnsupported
	}
	return nil
}
