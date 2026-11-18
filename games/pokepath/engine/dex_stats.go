package engine

import "errors"

type DexTowerStats struct {
	Damage     float64 `json:"damage"`
	HighestHit float64 `json:"highestHit"`
	Defeated   int     `json:"defeated"`
	Deployed   int     `json:"deployed"`
	Active     int     `json:"active"`
	Waves      int     `json:"waves"`
	Bosses     int     `json:"bosses"`
	Challenges int     `json:"challenges"`
}

type DexEnemyStats struct {
	Defeated int     `json:"defeated"`
	Escaped  int     `json:"escaped"`
	Damage   float64 `json:"damage"`
}

func updateDexEnemy(state *State, key string, defeated, escaped int, damage float64) {
	if key == "" || defeated == 0 && escaped == 0 && damage <= 0 {
		return
	}
	if state.Progression.Profile.EnemyStats == nil {
		state.Progression.Profile.EnemyStats = make(map[string]DexEnemyStats)
	}
	record := state.Progression.Profile.EnemyStats[key]
	record.Defeated += defeated
	record.Escaped += escaped
	record.Damage += max(0, damage)
	state.Progression.Profile.EnemyStats[key] = record
}

func validateDexEnemyStats(records map[string]DexEnemyStats) error {
	catalog := CatalogData().Enemies
	if len(records) > len(catalog) {
		return errors.New("敌人图鉴超过资料总量")
	}
	for key, record := range records {
		if _, ok := catalog[key]; !ok || record.Defeated < 0 || record.Escaped < 0 || record.Damage < 0 || !finiteNumber(record.Damage) {
			return errors.New("敌人图鉴统计无效")
		}
	}
	return nil
}

func damageTrackedEnemy(state *State, enemy *Enemy, sourceID int, amount float64, kind string, armorPierce float64) {
	_, credit := applyDamageWithCredit(&enemy.EnemyEffectState, amount, kind, armorPierce)
	updateDexEnemy(state, enemy.Key, 0, 0, credit)
	recordDexTowerDamage(state, sourceID, credit, enemy.LastResolvedDamage)
}

func recordDexTowerDamage(state *State, id int, credit, resolved float64) {
	if (credit <= 0 && resolved <= 0) || id <= 0 {
		return
	}
	if owner := ownedPokemonFor(state, id); owner != nil {
		owner.Dex.Damage += credit
		owner.Dex.HighestHit = max(owner.Dex.HighestHit, resolved)
	}
}

func validateDexTowerStats(record DexTowerStats) bool {
	return finiteNumber(record.Damage) && finiteNumber(record.HighestHit) && record.Damage >= 0 && record.HighestHit >= 0 && record.Defeated >= 0 && record.Deployed >= 0 && record.Active >= 0 && record.Waves >= 0 && record.Bosses >= 0 && record.Challenges >= 0
}

func recordDexDeploy(tower *Tower, owner *Pokemon) {
	tower.DexCountdownMS = 1550
	owner.Dex.Deployed++
}

func stepDexTower(state *State, tower *Tower) {
	tower.DexCountdownMS -= battleStepMS(state) / state.Speed
	if state.WaveActive && tower.DexCountdownMS <= 0 {
		tower.DexCountdownMS += 1550
		if owner := ownedPokemonFor(state, tower.PokemonID); owner != nil {
			owner.Dex.Active++
		}
	}
}

func recordDexCompletedWave(state *State, isBossWave bool) {
	for _, tower := range state.Towers {
		if owner := ownedPokemonFor(state, tower.PokemonID); owner != nil {
			owner.Dex.Waves++
			if isBossWave {
				owner.Dex.Bosses++
			}
		}
	}
}
