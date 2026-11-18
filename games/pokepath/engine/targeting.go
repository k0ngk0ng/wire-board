package engine

import (
	"cmp"
	"slices"
)

func pokemonTargetModes(state *State, pokemon *Pokemon) []string {
	species := WorldSpecies(state, pokemon)
	intrinsic := CatalogData().Species[pokemon.Species]
	copied := CatalogData().Species[state.Progression.DNA[pokemon.ID]]
	if slices.Contains([]string{"quickClaw", "spindaCocktail", "silphScope"}, pokemon.Item) || intrinsic.ID == 53 && pokemon.Item != "ringTarget" || copied.ID == 53 || slices.Contains([]string{"area", "aura", "bombardment", "orbital", "allies"}, species.AttackType) {
		return []string{}
	}
	modes := []string{"first", "last", "highHP", "lowHP", "highArmor", "noArmor", "faster", "slower", "poisoned", "notPoisoned", "burned", "notBurned", "stuned", "notStuned", "slowed", "notSlowed", "cursed", "curseable", "nightmared", "random"}
	if slices.Contains([]string{"illuminate", "illuminateBuff", "frisk", "vigilantFrisk", "scrappy", "moongeistBeam", "grimNeigh"}, species.AbilityID) {
		modes = append(modes, "invisible")
	}
	return modes
}

func forcedTargetMode(state *State, pokemon *Pokemon) string {
	if pokemon.Item == "silphScope" {
		return "invisible"
	}
	if pokemon.Item == "spindaCocktail" {
		return "random"
	}
	if pokemon.Item == "quickClaw" && WorldSpecies(state, pokemon).AbilityID != "contrary" {
		return "faster"
	}
	return pokemon.Target
}

func selectionMode(state *State, pokemon *Pokemon, profile AttackProfile, candidates []*Enemy) string {
	if pokemon.Item == "silphScope" && WorldSpecies(state, pokemon).ID == 53 && !slices.ContainsFunc(candidates, func(enemy *Enemy) bool { return enemy.Invisible }) {
		return "random"
	}
	if profile.RandomTarget && pokemon.Item != "silphScope" {
		return "random"
	}
	return forcedTargetMode(state, pokemon)
}

func selectTargets(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile) []*Enemy {
	candidates := targetsInRange(state, tower, pokemon, profile)
	orderTargets(state, candidates, selectionMode(state, pokemon, profile, candidates))
	return candidates
}

func selectAttackTargets(state *State, tower *Tower, pokemon *Pokemon, profile AttackProfile) []*Enemy {
	candidates := targetsInRange(state, tower, pokemon, profile)
	mode := selectionMode(state, pokemon, profile, candidates)
	if mode != "random" || len(candidates) < 2 {
		orderTargets(state, candidates, mode)
		return candidates
	}
	remaining := slices.Clone(candidates)
	orderTargets(state, candidates, mode)
	primary := candidates[0]
	orderTargets(state, remaining, mode)
	index := slices.Index(remaining, primary)
	copy(remaining[1:index+1], remaining[:index])
	remaining[0] = primary
	return remaining
}

func priorityFlag(value bool) int {
	if value {
		return 1
	}
	return 0
}

func targetConditionMode(mode string) (string, bool) {
	switch mode {
	case "poisoned", "notPoisoned":
		return "poison", mode == "poisoned"
	case "burned", "notBurned":
		return "burn", mode == "burned"
	case "stuned", "notStuned":
		return "stun", mode == "stuned"
	case "slowed", "notSlowed":
		return "slow", mode == "slowed"
	case "nightmared":
		return "nightmare", true
	case "curseable":
		return "curse", false
	}
	return "", false
}

func targetMeasure(enemy *Enemy, mode string) float64 {
	switch mode {
	case "first", "invisible":
		return enemy.Distance
	case "last":
		return -enemy.Distance
	case "highHP":
		return enemy.HP
	case "lowHP":
		return -enemy.HP
	case "highArmor":
		return enemy.Armor
	case "noArmor":
		return float64(priorityFlag(enemy.Armor <= 0))
	case "faster":
		return EffectiveEnemySpeedForDamage(&enemy.EnemyEffectState)
	case "slower":
		return -EffectiveEnemySpeedForDamage(&enemy.EnemyEffectState)
	}
	return 0
}

func orderTargets(state *State, enemies []*Enemy, mode string) {
	if len(enemies) < 2 || mode == "cursed" {
		return
	}
	if mode == "random" {
		for tail := len(enemies) - 1; tail > 0; tail-- {
			selected := NextInt(state, tail+1)
			enemies[tail], enemies[selected] = enemies[selected], enemies[tail]
		}
		return
	}
	if condition, want := targetConditionMode(mode); condition != "" {
		priority := func(enemy *Enemy) int {
			return priorityFlag(HasCondition(&enemy.EnemyEffectState, condition) == want)
		}
		slices.SortStableFunc(enemies, func(a, b *Enemy) int { return cmp.Compare(priority(b), priority(a)) })
		if mode == "curseable" {
			if start := slices.IndexFunc(enemies, func(enemy *Enemy) bool { return priority(enemy) == 0 }); start >= 0 {
				slices.Reverse(enemies[start:])
			}
		}
		return
	}
	slices.SortStableFunc(enemies, func(a, b *Enemy) int {
		if mode == "invisible" && a.Invisible != b.Invisible {
			return cmp.Compare(priorityFlag(b.Invisible), priorityFlag(a.Invisible))
		}
		return cmp.Compare(targetMeasure(b, mode), targetMeasure(a, mode))
	})
}
