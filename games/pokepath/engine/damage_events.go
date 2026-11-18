package engine

func recordPokemonDamage(state *State, sourceID int, actual, resolved float64) {
	if owner := ownedPokemonFor(state, sourceID); owner != nil {
		state.TotalDamage += actual
		state.TotalOverdamage += resolved
		owner.Damage += actual
		owner.Overdamage += resolved
	}
}

func resetPokemonDamage(state *State, owner *Pokemon) {
	owner.Damage, owner.Overdamage = 0, 0
	delete(state.World.HealingDamage, owner.ID)
	if tower := state.World.Towers[owner.ID]; tower != nil {
		tower.WaveDamage = 0
	}
}

func resetWaveDamage(state *State) {
	state.TotalDamage, state.TotalOverdamage = 0, 0
	for index := range state.Team {
		resetPokemonDamage(state, &state.Team[index])
	}
}

func damageEventFromEnemy(enemy *Enemy, sourceID int, kind string, critical bool) Event {
	if kind == "direct" || kind == "critical" || kind == "special" {
		kind = "physical"
	}
	effect := &enemy.EnemyEffectState
	return Event{Type: "hit", Source: sourceID, Target: enemy.ID, X: enemy.X, Y: enemy.Y, Key: enemy.Key, Critical: critical, HPDamage: effect.LastHPDamage, ArmorDamage: effect.LastArmorDamage, ShieldDamage: effect.LastShieldDamage, AttemptedDamage: effect.LastIncomingDamage, ResolvedDamage: effect.LastResolvedDamage, DisplayHPDamage: max(0, effect.LastResolvedDamage-effect.LastArmorDamage-effect.LastShieldDamage), DisplayArmorDamage: effect.LastArmorDamage, SourceKind: kind, Evaded: effect.LastHitEvaded, Amount: effect.LastHPDamage + effect.LastArmorDamage + effect.LastShieldDamage}
}
