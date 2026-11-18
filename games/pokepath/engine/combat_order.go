package engine

import (
	"cmp"
	"slices"
)

func advanceOrderedTowers(state *State, lastStep bool) error {
	type update struct {
		id       int
		y        float64
		deferred bool
	}
	order := make([]update, 0, len(state.Towers))
	owners := make(map[int]bool, len(state.Towers))
	for _, tower := range state.Towers {
		owner := pokemonFor(state, tower.PokemonID)
		y := tower.Y - 12
		if value := state.World.Towers[tower.PokemonID]; value != nil && value.OrbitPosition != nil && owner != nil && WorldSpecies(state, owner).AbilityID == "aerialAce" {
			y = value.OrbitPosition.Y
		}
		order = append(order, update{id: tower.ID, y: y, deferred: owner != nil && owner.Species == "spectrier"})
		owners[tower.ID] = true
	}
	slices.SortStableFunc(order, func(a, b update) int {
		if a.deferred != b.deferred {
			if a.deferred {
				return 1
			}
			return -1
		}
		return cmp.Compare(a.y, b.y)
	})
	for _, current := range order {
		for index := range state.Towers {
			tower := &state.Towers[index]
			if tower.ID != current.id {
				continue
			}
			advanceTowerClock(state, tower)
			if err := syncOrbitalFor(state, tower.PokemonID); err != nil {
				return err
			}
			if err := worldAdvanceDeployedTower(state, tower, lastStep); err != nil {
				return err
			}
			break
		}
		if state.WaveActive {
			if err := advanceTowersWhere(state, func(id int) bool { return id == current.id }); err != nil {
				return err
			}
		}
		if err := advanceAttackModesWhere(state, func(id int) bool { return id == current.id }); err != nil {
			return err
		}
		if err := advanceProjectilesWhere(state, func(projectile Projectile) bool {
			return projectile.TowerID == current.id && !isBeamKind(projectile.Kind)
		}); err != nil {
			return err
		}
		if err := advanceProjectilesWhere(state, func(projectile Projectile) bool {
			return projectile.TowerID == current.id && isBeamKind(projectile.Kind)
		}); err != nil {
			return err
		}
		if len(state.Towers) > 0 && current.id == state.Towers[0].ID {
			if err := worldAdvanceLinks(state, battleStepMS(state)); err != nil {
				return err
			}
		}
	}
	if len(order) == 0 {
		state.World.Links = nil
	}
	if err := advanceAttackModesWhere(state, func(id int) bool { return !owners[id] }); err != nil {
		return err
	}
	return advanceProjectilesWhere(state, func(projectile Projectile) bool { return !owners[projectile.TowerID] })
}
