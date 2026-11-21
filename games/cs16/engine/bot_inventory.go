package engine

func botBestWeapon(player *Player) string {
	best, bestSlot := "", 3
	for _, item := range player.Inventory {
		weapon, ok := WeaponByID(item.ID)
		if !ok || weapon.Slot < 1 || weapon.Slot > 2 || item.Ammo <= 0 && item.Reserve <= 0 {
			continue
		}
		if weapon.Slot < bestSlot {
			best, bestSlot = item.ID, weapon.Slot
		}
	}
	if best != "" {
		return best
	}
	if weaponIndex(player, "knife") >= 0 {
		return "knife"
	}
	return ""
}

func BotWeapon(state *State, seat int, input *Input) {
	if state == nil || input == nil || seat < 0 || seat >= len(state.Players) || state.Finished || state.Phase != "live" || !state.Players[seat].Alive {
		return
	}
	p := &state.Players[seat]
	selected := input.Select
	if selected == "" {
		selected = p.Weapon
	}
	index := weaponIndex(p, selected)
	weapon, known := WeaponByID(selected)
	if index >= 0 && known && weapon.Slot >= 3 {
		input.Reload = false
		return
	}
	requestedReload := input.Reload
	if index < 0 || !known || p.Inventory[index].Ammo <= 0 && p.Inventory[index].Reserve <= 0 {
		selected = botBestWeapon(p)
		input.Select = selected
		input.Fire, input.Secondary, input.Reload = false, false, false
		index = weaponIndex(p, selected)
		weapon, known = WeaponByID(selected)
		if index < 0 || !known {
			return
		}
	}
	if selected != p.Weapon {
		input.Select = selected
		input.Fire, input.Secondary, input.Reload = false, false, false
		return
	}
	item := p.Inventory[index]
	if item.ReloadUntil > 0 || state.Frame < p.NextAttack {
		input.Fire, input.Secondary, input.Reload = false, false, false
		return
	}
	needsReload := item.Reserve > 0 && (item.Ammo <= 0 || requestedReload && item.Ammo < weapon.Ammo)
	input.Reload = needsReload && !p.Input.Reload
	if needsReload || item.Ammo <= 0 {
		input.Fire, input.Secondary = false, false
	}
}
