package engine

import "math"

func worldShapeAuras(state *State, receiver *Pokemon, profile *AttackProfile) error {
	shape := combatRangeType(WorldSpecies(state, receiver), *receiver, *profile)
	applied := map[string]bool{}
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner == nil || WorldSpecies(state, owner).AbilityID != "hugePower" {
			continue
		}
		aura, err := BuildAttackProfile(contextFor(state, tower, owner, nil, .5))
		if err != nil {
			return err
		}
		if aura.Aura.Shape != shape || applied[aura.Aura.Shape] {
			continue
		}
		profile.Power = math.Ceil(profile.Power*aura.Aura.PowerMultiplier) + aura.Aura.PowerFlat
		applied[aura.Aura.Shape] = true
	}
	return nil
}
