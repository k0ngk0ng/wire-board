package pal

// Preserve surviving roles before allocating newcomers. Party order alone
// cannot identify a free seat after a scripted departure or reordering.
func partyControllers(roles []int, previous []PartyMember, players int) []int {
	owners := make([]int, len(roles))
	loads := make([]int, players)
	for slot, role := range roles {
		owners[slot] = -1
		for _, member := range previous {
			if member.Role == role {
				owners[slot] = member.Controller
				loads[member.Controller]++
				break
			}
		}
	}
	for slot, owner := range owners {
		if owner >= 0 {
			continue
		}
		seat := 0
		for candidate := 1; candidate < players; candidate++ {
			if loads[candidate] < loads[seat] {
				seat = candidate
			}
		}
		owners[slot] = seat
		loads[seat]++
	}
	return owners
}
