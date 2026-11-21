package engine

func botAvoidFriendlyFire(state *State, seat int, input *Input) {
	if seat < 0 || seat >= len(state.Players) {
		return
	}
	player := state.Players[seat]
	selected := player.Weapon
	if input.Select != "" && weaponIndex(&player, input.Select) >= 0 {
		selected = input.Select
	}
	weapon, ok := WeaponByID(selected)
	if !ok || weapon.Slot != 1 && weapon.Slot != 2 && selected != "knife" || !input.Fire && !(selected == "knife" && input.Secondary) {
		return
	}
	rangeUnits := weapon.Range
	if selected == "knife" && input.Secondary {
		rangeUnits = 32 * Unit
	}
	start := Eye(player)
	end := combatAddRay(start, Direction(input.Yaw+player.RecoilYaw, input.Pitch+player.RecoilPitch), rangeUnits)
	first, closest := -1, NormalUnit+1
	for index, other := range state.Players {
		if index == seat || !other.Alive {
			continue
		}
		target := HistoryPlayer{Seat: other.Seat, X: other.X, Y: other.Y, Z: other.Z, Yaw: other.Yaw, Weapon: other.Weapon, Alive: other.Alive, Life: other.Life, Crouch: other.Crouch}
		fraction, _, hit := modelHitFraction(target, other.Character, start, end)
		if hit && fraction < closest {
			first, closest = index, fraction
		}
	}
	if first >= 0 && state.Players[first].Team == player.Team {
		input.Fire = false
		if selected == "knife" {
			input.Secondary = false
		}
	}
}
