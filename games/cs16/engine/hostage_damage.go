package engine

func hostageDamageStateFor(state *State, index int) *HostageDamageState {
	id := state.Hostages[index].ID
	for i := range state.HostageDamageStates {
		if state.HostageDamageStates[i].ID == id {
			return &state.HostageDamageStates[i]
		}
	}
	state.HostageDamageStates = append(state.HostageDamageStates, HostageDamageState{ID: id})
	return &state.HostageDamageStates[len(state.HostageDamageStates)-1]
}

func hostageTraceUnits(damage int64, group int) int64 {
	switch group {
	case 1:
		return damage * 3
	case 2, 3:
		return damage * 3 / 2
	case 6, 7:
		return damage * 3 / 4
	case 8:
		return 0
	}
	return damage
}

func hostageHitUnits(damage int64, group int) int64 {
	switch group {
	case 0, 3:
		return damage * 7 / 4
	case 1:
		return damage * 5 / 2
	case 2:
		return damage * 3 / 2
	case 4, 5:
		return damage * 3 / 4
	case 6, 7:
		return damage * 3 / 5
	}
	return damage * 3 / 2
}

func damageHostageTraceUnits(state *State, attacker, index int, damage int64, group int) {
	damageHostageMultiUnits(state, attacker, index, hostageTraceUnits(damage, group), group)
}

func damageHostageMultiUnits(state *State, attacker, index int, damage int64, group int) {
	if index < 0 || index >= len(state.Hostages) {
		return
	}
	hostageDamageStateFor(state, index).Hitgroup = group
	damageHostageUnits(state, attacker, index, hostageHitUnits(damage, group))
}

func damageHostageUnits(state *State, attacker, index int, damage int64) {
	if index < 0 || index >= len(state.Hostages) || damage <= 0 {
		return
	}
	h := &state.Hostages[index]
	if h.Rescued || h.Health <= 0 {
		return
	}
	private := hostageDamageStateFor(state, index)
	health := int64(h.Health)*damageUnit - int64(private.HealthRemainder)
	damage = min(damage, health)
	health -= damage
	h.Health = int((health + damageUnit - 1) / damageUnit)
	private.HealthRemainder = int(int64(h.Health)*damageUnit - health)
	penalty := 20 * int(damage/damageUnit)
	if h.Health == 0 {
		h.Following = -1
		penalty += 500
	}
	if attacker >= 0 && attacker < len(state.Players) {
		state.Players[attacker].Money = max(0, state.Players[attacker].Money-penalty)
	}
	Emit(state, Event{Kind: "hostage_hit", Seat: attacker, Target: h.ID, Value: int(damage / damageUnit), Hitgroup: private.Hitgroup, X: h.X, Y: h.Y, Z: h.Z})
}
