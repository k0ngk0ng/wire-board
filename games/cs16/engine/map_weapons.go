package engine

var armouryWeapons = [...]struct {
	ID      string
	Reserve int
}{
	{"mp5navy", 60}, {"tmp", 60}, {"p90", 50}, {"mac10", 60},
	{"ak47", 60}, {"sg552", 60}, {"m4a1", 60}, {"aug", 60},
	{"scout", 30}, {"g3sg1", 30}, {"awp", 20}, {"m3", 24},
	{"xm1014", 24}, {"m249", 60},
}

func InitMapWeapons(state *State) {
	kept := state.Drops[:0]
	for _, drop := range state.Drops {
		if drop.SpawnID <= 0 {
			kept = append(kept, drop)
		}
	}
	state.Drops = kept
	seen := make(map[int]bool, len(state.World.WorldWeapons))
	for _, spawn := range state.World.WorldWeapons {
		if spawn.ID <= 0 || seen[spawn.ID] || spawn.Item < 0 || spawn.Item >= len(armouryWeapons) {
			continue
		}
		definition := armouryWeapons[spawn.Item]
		if spawn.Weapon != "" && spawn.Weapon != definition.ID {
			continue
		}
		weapon, ok := WeaponByID(definition.ID)
		if !ok || weapon.Slot != 1 {
			continue
		}
		seen[spawn.ID] = true
		point := Vec3{spawn.X, spawn.Y, spawn.Z}
		if len(state.World.Planes) > 0 {
			floor := TraceWorld(state, point, Vec3{point.X, point.Y, point.Z - 4096*Unit}, -1)
			if !floor.StartSolid && floor.Fraction < NormalUnit && floor.Normal.Z > 0 {
				point = floor.End
			}
		}
		state.NextID++
		state.Drops = append(state.Drops, Drop{ID: state.NextID, X: point.X, Y: point.Y, Z: point.Z, Yaw: spawn.Yaw, Weapon: WeaponState{ID: weapon.ID, Ammo: weapon.Ammo, Reserve: definition.Reserve}, Frame: state.Frame, SpawnID: spawn.ID, Remaining: max(1, spawn.Count)})
	}
}

func mapWeaponHasPrimary(player *Player) bool {
	for _, item := range player.Inventory {
		weapon, ok := WeaponByID(item.ID)
		if ok && weapon.Slot == 1 {
			return true
		}
	}
	return false
}

func mapWeaponSeatEligible(state *State, seat int) bool {
	return !state.Finished && (state.Phase == "freeze" || state.Phase == "live") && seat >= 0 && seat < len(state.Players) && state.Players[seat].Alive && !mapWeaponHasPrimary(&state.Players[seat])
}

func TouchMapWeapons(state *State, seat int) bool {
	if !mapWeaponSeatEligible(state, seat) {
		return false
	}
	p := &state.Players[seat]
	bottom, top := -36*Unit, 36*Unit
	if p.Crouch {
		bottom, top = -18*Unit, 32*Unit
	}
	for index, drop := range state.Drops {
		if drop.SpawnID <= 0 || drop.Remaining <= 0 || p.X+16*Unit < drop.X-16*Unit || p.X-16*Unit > drop.X+16*Unit || p.Y+16*Unit < drop.Y-16*Unit || p.Y-16*Unit > drop.Y+16*Unit || p.Z+top < drop.Z || p.Z+bottom > drop.Z+16*Unit {
			continue
		}
		weapon, ok := WeaponByID(drop.Weapon.ID)
		if !ok || weapon.Slot != 1 {
			continue
		}
		shared := 0
		for _, owned := range p.Inventory {
			definition, valid := WeaponByID(owned.ID)
			if valid && definition.AmmoType == weapon.AmmoType {
				shared = max(shared, owned.Reserve)
			}
		}
		item := WeaponState{ID: weapon.ID, Ammo: weapon.Ammo, Reserve: min(weapon.Reserve, shared+max(0, drop.Weapon.Reserve))}
		resetWeaponAccuracy(&item, false)
		p.Inventory = append(p.Inventory, item)
		syncSharedAmmo(p, weapon.AmmoType, item.Reserve)
		CancelGrenade(p)
		p.Weapon = weapon.ID
		p.NextAttack = state.Frame + weaponDeployFrames(weapon.ID)
		SyncWeapon(p)
		state.Drops[index].Remaining--
		remaining := state.Drops[index].Remaining
		if remaining <= 0 {
			state.Drops = append(state.Drops[:index], state.Drops[index+1:]...)
		}
		Emit(state, Event{Kind: "pickup", Seat: seat, Target: drop.SpawnID, Weapon: weapon.ID, X: drop.X, Y: drop.Y, Z: drop.Z, Value: remaining})
		return true
	}
	return false
}

func MapWeaponGoal(state *State, seat int) (Vec3, bool) {
	if !mapWeaponSeatEligible(state, seat) {
		return Vec3{}, false
	}
	p := &state.Players[seat]
	bestDistance := int(^uint(0) >> 1)
	best, found := Vec3{}, false
	for _, drop := range state.Drops {
		if drop.SpawnID <= 0 || drop.Remaining <= 0 {
			continue
		}
		weapon, ok := WeaponByID(drop.Weapon.ID)
		if !ok || weapon.Slot != 1 {
			continue
		}
		point := Vec3{drop.X, drop.Y, drop.Z + 36*Unit}
		distance := Distance(Position(*p), point)
		if distance < bestDistance {
			bestDistance, best, found = distance, point, true
		}
	}
	return best, found
}

func trimDynamicDrops(state *State) {
	ordinary := 0
	for _, drop := range state.Drops {
		if drop.SpawnID <= 0 {
			ordinary++
		}
	}
	excess := ordinary - 64
	if excess <= 0 {
		return
	}
	kept := state.Drops[:0]
	for _, drop := range state.Drops {
		if drop.SpawnID <= 0 && excess > 0 {
			excess--
			continue
		}
		kept = append(kept, drop)
	}
	state.Drops = kept
}
