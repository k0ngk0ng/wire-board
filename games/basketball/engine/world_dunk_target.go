package engine

// 056E is shared scratch, not a persistent hoop attachment. Other actors can
// overwrite it while a dunk is already entered. 8E9D only reads mount geometry
// on entry; its later target action/durability references use the live byte.
// 0577+0..6 overlaps existing actor jump/attack bytes; never allocate a second
// independently writable durability array for these aliases.
func originalDunkTargetBytes(s *WorldActionState, c WorldActionContext, target uint8) (uint8, int8) {
	var owner uint8
	switch {
	case target < 4:
		owner = c.Controls[target]
	case target < 6:
		owner = uint8(s.Auxiliary[target-4].Owner)
	case target == 6:
		owner = uint8(s.Holder)
	default:
		owner = s.Hoops[target-7].Owner
	}
	switch {
	case target < 3:
		return owner, s.Actors[target+1].JumpPhase
	case target < 7:
		return owner, int8(s.Actors[target-3].AttackVariant)
	default:
		return owner, s.Hoops[target-7].Durability
	}
}
func setOriginalDunkTargetDurability(s *WorldActionState, target uint8, value int8) {
	switch {
	case target < 3:
		s.Actors[target+1].JumpPhase = value
	case target < 7:
		s.Actors[target-3].AttackVariant = uint8(value)
	default:
		s.Hoops[target-7].Durability = value
	}
}
