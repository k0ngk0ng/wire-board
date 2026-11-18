package engine

func hasRoomService(state *State) bool {
	for _, tower := range state.Towers {
		if owner := pokemonFor(state, tower.PokemonID); owner != nil && owner.Item == "roomService" {
			return true
		}
	}
	return false
}

func worldHealEvent(state *State, amount int) {
	before := state.Health
	state.Health = min(state.MaxHealth, state.Health+amount)
	state.WaveHeartsHealed += state.Health - before
	state.World.HealScale += amount
	state.World.RoomServicePower += 20
}

func worldDamageHealing(state *State, sourceID int, enemy *Enemy, profile AttackProfile, context AbilityContext, damage float64) error {
	owner := ownedPokemonFor(state, sourceID)
	if owner == nil {
		return nil
	}
	if state.World.HealingDamage == nil {
		state.World.HealingDamage = map[int]float64{}
	}
	previous := state.World.HealingDamage[sourceID]
	if value := state.World.Towers[sourceID]; value != nil {
		previous = max(previous, value.WaveDamage)
	}
	state.World.HealingDamage[sourceID] = previous + damage
	if enemy.InvulnerableMS > 0 || enemy.LastHitEvaded || enemy.PassiveID == "iceBody" && enemy.LastHitCritical {
		return nil
	}
	if state.World.HealingItemUsed == nil {
		state.World.HealingItemUsed = map[string]bool{}
	}
	if state.World.HealingUses == nil {
		state.World.HealingUses = map[int]float64{}
	}
	goalCrossed := profile.HeartDamageGoal > 0 && state.World.HealingDamage[sourceID] > profile.HeartDamageGoal
	eligible := enemy.PassiveID != "healBlock" && (state.Health < state.MaxHealth || state.World.HealScale < 5)
	if eligible {
		if context.ItemID == "leftovers" && !state.World.HealingItemUsed["leftovers"] && profile.HeartChance > 0 && NextRandom(state) < profile.HeartChance {
			worldHealEvent(state, 1)
			state.World.HealingItemUsed["leftovers"] = true
		}
		if goalCrossed && !state.World.HealingItemUsed[context.ItemID] {
			worldHealEvent(state, 1)
			state.World.HealingItemUsed[context.ItemID] = true
		}
	}
	if context.ItemID == "shellBell" && goalCrossed {
		state.World.HealingItemUsed["shellBell"] = true
	}
	if !eligible || context.ItemID == "lifeOrb" {
		return nil
	}
	chanceProfile := profile
	if context.ItemID == "leftovers" || profile.HeartDamageGoal > 0 {
		bare := context
		bare.ItemID = ""
		bare.HealingUses = state.World.HealingUses[sourceID]
		var err error
		chanceProfile, err = BuildAttackProfile(bare)
		if err != nil {
			return err
		}
	}
	if chanceProfile.HeartChance <= 0 || state.World.HealingUses[sourceID] >= 3 {
		return nil
	}
	cost := 1.0
	if context.AbilityID == "bitterBlade" || context.ItemID == "auspiciousArmor" {
		cost = 3
	}
	if context.AbilityID == "takeHeart" || context.AbilityID == "soulHeartReactor" {
		cost = 1.5
	}
	if chanceProfile.HeartLimit > 0 && int(state.World.HealingUses[sourceID]/cost) >= chanceProfile.HeartLimit {
		return nil
	}
	live := context
	live.HealingUses = state.World.HealingUses[sourceID]
	live.HealedHearts = state.World.HealScale
	if live.ItemID == "leftovers" || profile.HeartDamageGoal > 0 {
		live.ItemID = ""
	}
	current, err := BuildAttackProfile(live)
	if err != nil {
		return err
	}
	if current.HeartChance <= 0 || NextRandom(state) >= current.HeartChance {
		return nil
	}
	amount := chanceProfile.HeartAmount
	if profile.HeartInverted {
		if state.Health > 1 {
			lost := min(state.Health-1, WorldOnHeartsLost(state, 1))
			state.Health -= lost
			state.WaveHeartsLost += lost
		}
		state.World.PendingHeal++
		state.World.HealScale += amount
		state.World.RoomServicePower += 20
	} else {
		worldHealEvent(state, amount)
	}
	state.World.HealingUses[sourceID] = min(3, state.World.HealingUses[sourceID]+cost)
	return nil
}

func worldIndirectHealing(state *State, enemy *Enemy, sourceID int, damage float64) error {
	return worldSourceHealing(state, enemy, sourceID, damage, "")
}

func worldSourceHealing(state *State, enemy *Enemy, sourceID int, damage float64, abilityID string) error {
	owner := ownedPokemonFor(state, sourceID)
	if owner == nil {
		return nil
	}
	tower := worldTowerFor(state, sourceID)
	if tower == nil {
		tower = &Tower{PokemonID: sourceID, TileID: -1}
	}
	context := contextFor(state, tower, owner, enemy, .5)
	context.AbilityID = abilityID
	profile, err := BuildAttackProfile(context)
	if err != nil {
		return err
	}
	return worldDamageHealing(state, sourceID, enemy, profile, context, damage)
}
