package engine

func brushReceivesDamage(state *State, index int) bool {
	if index < 0 || index >= len(state.World.Brushes) {
		return false
	}
	brush := state.World.Brushes[index]
	if brush.Kind != "func_breakable" && brush.Kind != "breakable" || brush.Flags&1 != 0 {
		return false
	}
	dynamic := brushDynamic(state, brush.ID)
	return dynamic != nil && !dynamic.Open && dynamic.Health > 0
}

func brushDamageStateFor(state *State, id int) *BrushDamageState {
	for index := range state.BrushDamageStates {
		if state.BrushDamageStates[index].ID == id {
			return &state.BrushDamageStates[index]
		}
	}
	state.BrushDamageStates = append(state.BrushDamageStates, BrushDamageState{ID: id})
	return &state.BrushDamageStates[len(state.BrushDamageStates)-1]
}

func damageBreakableUnits(state *State, attacker, index int, damage int64, weapon string) bool {
	if damage <= 0 || !brushReceivesDamage(state, index) {
		return false
	}
	brush := state.World.Brushes[index]
	if brush.Material == 7 {
		return false
	}
	dynamic := brushDynamic(state, brush.ID)
	private := brushDamageStateFor(state, brush.ID)
	health := int64(dynamic.Health)*damageUnit - int64(private.HealthRemainder)
	health = max(0, health-min(health, damage))
	dynamic.Health = int((health + damageUnit - 1) / damageUnit)
	private.HealthRemainder = int(int64(dynamic.Health)*damageUnit - health)
	if health > 0 {
		return false
	}
	dynamic.Open = true
	point := brushCenter(brush)
	Emit(state, Event{Kind: "break", Seat: attacker, Target: brush.ID, Weapon: weapon, X: point.X, Y: point.Y, Z: point.Z})
	if brush.Target != "" {
		var activator *Player
		if attacker >= 0 && attacker < len(state.Players) {
			activator = &state.Players[attacker]
		}
		for target, other := range state.World.Brushes {
			if target != index && other.TargetName == brush.Target {
				activateBrush(state, target, activator)
			}
		}
	}
	return true
}

func damageBlastBrushes(state *State, attacker int, center, source Vec3, damage, radius int, weapon string) {
	if !blastWaterAllowed(state, center, 0) {
		return
	}
	for index, brush := range state.World.Brushes {
		if !brushReceivesDamage(state, index) || brush.Material == 7 {
			continue
		}
		units := blastDamageUnits(source, brushCenter(brush), damage, radius)
		damageBreakableUnits(state, attacker, index, units, weapon)
	}
}
