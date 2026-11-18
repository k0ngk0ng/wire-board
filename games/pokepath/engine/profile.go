package engine

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

type ProfileRecord struct {
	Value    float64 `json:"value"`
	RouteTag string  `json:"routeTag"`
}

type TrainerProgress struct {
	Portrait            int                      `json:"portrait"`
	HighestPokemonLevel int                      `json:"highestPokemonLevel"`
	HighestHit          float64                  `json:"highestHit"`
	DefeatedEnemies     int                      `json:"defeatedEnemies"`
	DefeatedSpecies     map[string]bool          `json:"defeatedSpecies"`
	EnemyStats          map[string]DexEnemyStats `json:"enemyStats,omitempty"`
	Applied             [5]int                   `json:"applied"`
	WavesCompleted      int                      `json:"wavesCompleted"`
	Resets              int                      `json:"resets"`
	StolenGold          int64                    `json:"stolenGold"`
	MaxGoldPerWave      ProfileRecord            `json:"maxGoldPerWave"`
	MaxGoldPerTime      ProfileRecord            `json:"maxGoldPerTime"`
}

type TrainerStats struct {
	TimePlayed          int64         `json:"timePlayed"`
	Stars               int           `json:"stars"`
	MaxStars            int           `json:"maxStars"`
	PokemonOwned        int           `json:"pokemonOwned"`
	PokemonTotal        int           `json:"pokemonTotal"`
	ShinyAmount         int           `json:"shinyAmount"`
	HighestPokemonLevel int           `json:"highestPokemonLevel"`
	TotalPokemonLevel   int           `json:"totalPokemonLevel"`
	TotalGold           int64         `json:"totalGold"`
	ItemAmount          int           `json:"itemAmount"`
	ItemTotal           int           `json:"itemTotal"`
	WavesCompleted      int           `json:"wavesCompleted"`
	HighestHit          float64       `json:"highestHit"`
	DefeatedEnemies     int           `json:"defeatedEnemies"`
	DefeatedSpecies     int           `json:"defeatedSpecies"`
	EnemyTotal          int           `json:"enemyTotal"`
	AppliedStuns        int           `json:"appliedStuns"`
	AppliedSlows        int           `json:"appliedSlows"`
	AppliedBurns        int           `json:"appliedBurns"`
	AppliedPoisons      int           `json:"appliedPoisons"`
	AppliedCurses       int           `json:"appliedCurses"`
	Resets              int           `json:"resets"`
	StolenGold          int64         `json:"stolenGold"`
	MaxGoldPerWave      ProfileRecord `json:"maxGoldPerWave"`
	MaxGoldPerTime      ProfileRecord `json:"maxGoldPerTime"`
}

func projectTrainerStats(state *State) TrainerStats {
	profile := state.Progression.Profile
	result := TrainerStats{TimePlayed: state.Clock / 60000, Stars: min(4000, state.Stars), MaxStars: 4000, PokemonOwned: len(state.Team) + len(state.Box), PokemonTotal: len(CatalogData().Collection), TotalGold: state.Progression.TotalGold, HighestPokemonLevel: max(1, profile.HighestPokemonLevel), HighestHit: profile.HighestHit, DefeatedEnemies: profile.DefeatedEnemies, DefeatedSpecies: min(345, len(profile.DefeatedSpecies)), EnemyTotal: 345, AppliedStuns: profile.Applied[0], AppliedSlows: profile.Applied[1], AppliedBurns: profile.Applied[2], AppliedPoisons: profile.Applied[3], AppliedCurses: profile.Applied[4], WavesCompleted: profile.WavesCompleted, Resets: profile.Resets, StolenGold: profile.StolenGold, MaxGoldPerWave: profile.MaxGoldPerWave, MaxGoldPerTime: profile.MaxGoldPerTime}
	for _, group := range [][]Pokemon{state.Team, state.Box} {
		for _, pokemon := range group {
			result.HighestPokemonLevel = max(result.HighestPokemonLevel, pokemon.Level)
			result.TotalPokemonLevel += pokemon.Level
			if pokemon.Shiny {
				result.ShinyAmount++
			}
		}
	}
	removed := []string{"dragoninite", "metagrossite", "froslassite", "garchompite", "gyaradosite", "magearnite", "heracronite", "aggronite"}
	for key, item := range CatalogData().Items {
		if item.Equipable && !slices.Contains(removed, key) {
			result.ItemTotal++
			if progressionOwnsItem(state, key) {
				result.ItemAmount++
			}
		}
	}
	return result
}

func validateTrainerProgress(state *State) error {
	profile := state.Progression.Profile
	if err := validateDexEnemyStats(profile.EnemyStats); err != nil {
		return err
	}
	if profile.Portrait < 0 || profile.Portrait > 79 || profile.HighestPokemonLevel < 0 || profile.HighestPokemonLevel > 100 || !finiteNumber(profile.HighestHit) || profile.HighestHit < 0 || profile.DefeatedEnemies < 0 || profile.WavesCompleted < 0 || profile.Resets < 0 || profile.StolenGold < 0 || len(profile.DefeatedSpecies) > len(CatalogData().Enemies) {
		return errors.New("训练家统计存档无效")
	}
	for key, defeated := range profile.DefeatedSpecies {
		if _, ok := CatalogData().Enemies[key]; !ok || !defeated {
			return errors.New("训练家击败物种记录无效")
		}
	}
	for _, value := range profile.Applied {
		if value < 0 {
			return errors.New("训练家状态施加记录无效")
		}
	}
	for _, record := range []ProfileRecord{profile.MaxGoldPerWave, profile.MaxGoldPerTime} {
		if !finiteNumber(record.Value) || record.Value < 0 || len(record.RouteTag) > 40 || record.Value == 0 && record.RouteTag != "" || record.Value > 0 && record.RouteTag == "" {
			return errors.New("训练家金币纪录无效")
		}
	}
	return nil
}

func trainerProfileAction(state *State, action Action) error {
	if state.Paused || state.Progression.CustomRoute != nil || !finiteNumber(action.Value) || math.Trunc(action.Value) != action.Value {
		return errors.New("当前不能更改训练家资料")
	}
	switch action.Type {
	case "portrait":
		if action.Value < 0 || action.Value > 79 {
			return errors.New("请选择有效训练家头像")
		}
		state.Progression.Profile.Portrait = int(action.Value)
	case "profileRecordReset":
		if action.Value == 19 {
			state.Progression.Profile.MaxGoldPerWave = ProfileRecord{}
		} else if action.Value == 20 {
			state.Progression.Profile.MaxGoldPerTime = ProfileRecord{}
		} else {
			return errors.New("请选择可清空的金币纪录")
		}
	default:
		return errors.New("未知训练家资料操作")
	}
	return nil
}

func trainerConditionCounts(enemy *Enemy) [5]int {
	result := [5]int{}
	for _, condition := range enemy.Conditions {
		index := slices.Index([]string{"stun", "slow", "burn", "poison", "curse"}, condition.Kind)
		if index >= 0 {
			result[index] += max(1, condition.Stacks)
		}
	}
	return result
}

func recordTrainerConditions(state *State, before [5]int, enemy *Enemy) {
	after := trainerConditionCounts(enemy)
	for index, value := range after {
		state.Progression.Profile.Applied[index] += max(0, value-before[index])
	}
}

func recordTrainerDefeat(state *State, enemy *Enemy) {
	profile := &state.Progression.Profile
	if profile.DefeatedSpecies == nil {
		profile.DefeatedSpecies = map[string]bool{}
	}
	profile.DefeatedSpecies[enemy.Key] = true
	profile.DefeatedEnemies++
	updateDexEnemy(state, enemy.Key, 1, 0, 0)
}

func recordTrainerWave(state *State, victory bool, gold int64) {
	profile := &state.Progression.Profile
	if victory {
		profile.WavesCompleted++
	}
	route, _ := routeForState(state)
	tag := "???"
	if !slices.Contains([]int{20, 30, 31, 33, 35}, route.ID) {
		index := route.ID
		if index > 20 {
			index--
		}
		if index > 29 {
			index -= 4
		}
		if index > 30 {
			index--
		}
		tag = fmt.Sprintf("R%d-%d W%d", index/4+1, index%4+1, state.Wave)
	}
	if float64(gold) > profile.MaxGoldPerWave.Value {
		profile.MaxGoldPerWave = ProfileRecord{Value: float64(gold), RouteTag: tag}
	}
	seconds := float64(state.World.WaveElapsedMS) / 1000
	if seconds > 0 {
		perSecond := math.Round(float64(gold)/seconds*100) / 100
		if perSecond > profile.MaxGoldPerTime.Value {
			profile.MaxGoldPerTime = ProfileRecord{Value: perSecond, RouteTag: tag}
		}
	}
}
