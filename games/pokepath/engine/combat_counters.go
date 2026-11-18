package engine

import (
	"math"
)

func advanceTowerClock(state *State, tower *Tower) {
	stepDexTower(state, tower)
	tower.DeployedMS += battleStepWholeMS(state)
	tower.CooldownMS -= battleStepMS(state)
	tower.StatsRefreshMS -= battleStepMS(state)
	if tower.StatsRefreshMS <= 0 {
		tower.PublishedIncense = tower.IncenseSeconds
		tower.StatsRefreshMS = 100
	}
	if state.WaveActive {
		tower.WaveActiveMS += battleStepWholeMS(state)
		if owner := pokemonFor(state, tower.PokemonID); owner != nil && owner.Item == "fullIncense" {
			tower.IncenseTimerMS += battleStepMS(state)
			for tower.IncenseTimerMS >= 1000 {
				tower.IncenseTimerMS -= 1000
				tower.IncenseSeconds++
			}
		}
	}
	if tower.CriticalStreak > 0 {
		tower.SinceCriticalMS = min(6000, tower.SinceCriticalMS+battleStepWholeMS(state))
		if tower.SinceCriticalMS >= 6000 {
			tower.CriticalStreak = 0
		}
	}
}

func resetWaveCombatCounters(state *State, ended bool) {
	for index := range state.Towers {
		tower := &state.Towers[index]
		owner := pokemonFor(state, tower.PokemonID)
		if owner == nil {
			continue
		}
		ability := WorldSpecies(state, owner).AbilityID
		tower.WaveActiveMS = 0
		tower.IncenseTimerMS, tower.IncenseSeconds, tower.PublishedIncense = 0, 0, 0
		if !ended {
			tower.CriticalStreak, tower.SinceCriticalMS = 0, 0
			delete(tower.Stacks, "speedBoost")
			delete(tower.Stacks, "focusEnergy")
		}
		if ability == "laundryGoRound" || !ended && ability == "meteorMash" {
			tower.AttackCount = 0
		}
		if ability == "shiftGear" {
			tower.Stacks["orbitalSpeed"] = 0
			if ended {
				count := 0
				for _, projectile := range state.Projectiles {
					if projectile.PokemonID == owner.ID && projectile.Kind == "orbit" {
						count++
					}
				}
				current := 0
				for projectileIndex := range state.Projectiles {
					projectile := &state.Projectiles[projectileIndex]
					if projectile.PokemonID == owner.ID && projectile.Kind == "orbit" && count > 0 {
						projectile.Angle = float64(current) * math.Pi * 2 / float64(count)
						projectile.X = tower.X + math.Cos(projectile.Angle)*projectile.OrbitRadius
						projectile.Y = tower.Y + math.Sin(projectile.Angle)*projectile.OrbitRadius
						current++
					}
				}
			}
		}
	}
}
