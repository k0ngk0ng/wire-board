package stoneage

import "fmt"

func (s *State) advanceCharge(actor int, c Command) bool {
	u := s.unit(actor)
	if u.Charge == nil {
		u.Charge = &ChargeState{Skill: c.ID, Target: c.Target, Remaining: c.ID - 29}
	}
	if u.Charge.Remaining > 0 {
		u.Charge.Remaining--
		name := "突击"
		if c.ID == 31 {
			name = "双重突击"
		}
		s.event("wait", actor, actor, fmt.Sprintf("%s：蓄力中（剩余 %d 回合）", name, u.Charge.Remaining), 0, 0, false)
		return false
	}
	u.Charge = nil
	return true
}
