package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type observedDexStats struct {
	Defeated int     `json:"defeated"`
	Escaped  int     `json:"escaped"`
	Damage   float64 `json:"damage"`
}

func observeDexStats(t *testing.T, state *State) map[string]observedDexStats {
	t.Helper()
	encoded, err := json.Marshal(Project(state).Progression)
	if err != nil {
		t.Fatal(err)
	}
	var view struct {
		EnemyStats map[string]observedDexStats `json:"enemyStats"`
	}
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	if view.EnemyStats == nil {
		t.Fatal("missing enumerable enemy statistics in the shared projection")
	}
	return view.EnemyStats
}

func TestDexStatisticsTrackHitsDefeatsAndEscapesInOneWave(t *testing.T) {
	state := coreNew(t, 4)
	coreApply(t, state, 3, Action{Type: "deploy", PokemonID: state.Team[0].ID, TileID: coreTile(t, state)})
	coreApply(t, state, 1, Action{Type: "wave"})
	state.Enemies = []Enemy{}
	tower := state.Towers[0]
	for index := 0; index < 2; index++ {
		id, err := SpawnEnemyAt(state, "rattata", Point{X: tower.X + 30 + float64(index)*15, Y: tower.Y}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.Entered, enemy.BaseSpeed = true, 0
		enemy.HP, enemy.MaxHP, enemy.Armor, enemy.MaxArmor = 4, 4, 2, 2
		enemy.Shield = 3
	}
	path := pathsForState(state)[0]
	id, err := SpawnEnemyAt(state, "ledyba", Point{X: -500, Y: path[len(path)-1].Y}, 0, len(path)-1)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, id).BaseSpeed = 0
	if err := Advance(state, 20000); err != nil {
		t.Fatal(err)
	}
	if state.TotalKills != 2 || state.Health != 13 || state.Gold <= 50 {
		t.Fatalf("the fixture did not play two real kills and one escape: kills=%d hp=%d gold=%d", state.TotalKills, state.Health, state.Gold)
	}
	want := map[string]observedDexStats{"rattata": {Defeated: 2, Damage: 12}, "ledyba": {Escaped: 1}}
	if got := observeDexStats(t, state); !reflect.DeepEqual(got, want) {
		t.Fatalf("records must cap overkill at health plus armor, exclude shield capacity, and distinguish escape: %#v", got)
	}
	if got := observeDexTower(t, state); got.Damage != 12 || got.HighestHit < 9 || got.Defeated != 2 || got.Deployed != 1 || got.Waves != 1 || got.Active == 0 {
		t.Fatalf("lifetime tower statistics must credit two real kills, a capped damage sum, and the completed wave: %+v", got)
	}
	data, err := Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	route, ok := routeFor(1)
	if !ok {
		t.Fatal("second route unavailable")
	}
	restored.Stars = route.UnlockStars
	coreApply(t, restored, 2, Action{Type: "route", Route: 1})
	if got := observeDexStats(t, restored); !reflect.DeepEqual(got, want) {
		t.Fatalf("switching routes after restoring changed lifetime data: %#v", got)
	}
	if got, expected := observeDexTower(t, restored), observeDexTower(t, state); got != expected {
		t.Fatalf("switching routes after restore cleared the owned tower lifetime record: %+v", got)
	}
}

type observedDexTowerStats struct {
	Damage     float64 `json:"damage"`
	HighestHit float64 `json:"highestHit"`
	Defeated   int     `json:"defeated"`
	Deployed   int     `json:"deployed"`
	Active     int     `json:"active"`
	Waves      int     `json:"waves"`
	Bosses     int     `json:"bosses"`
	Challenges int     `json:"challenges"`
}

func observeDexTower(t *testing.T, state *State) observedDexTowerStats {
	t.Helper()
	encoded, err := json.Marshal(Project(state).Team[0])
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Dex observedDexTowerStats `json:"dex"`
	}
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	return value.Dex
}

func TestDexTowerActiveTimeUsesWallTimeAndFinalWaveCounts(t *testing.T) {
	state := coreNew(t, 2)
	id := state.Team[0].ID
	coreApply(t, state, 0, Action{Type: "deploy", PokemonID: id, TileID: coreTile(t, state)})
	coreApply(t, state, 1, Action{Type: "speed", Value: 2.5})
	coreApply(t, state, 1, Action{Type: "wave"})
	state.Enemies = []Enemy{}
	enemyID, err := SpawnEnemyAt(state, "rattata", Point{X: 950, Y: 950}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	enemyFor(state, enemyID).BaseSpeed = 0
	if err := Advance(state, 1540); err != nil {
		t.Fatal(err)
	}
	if got := observeDexTower(t, state); got.Active != 0 {
		t.Fatalf("wall timer was advanced too quickly at 2.5×: %+v", got)
	}
	if err := Advance(state, 30); err != nil {
		t.Fatal(err)
	}
	if got := observeDexTower(t, state); got.Active != 1 {
		t.Fatalf("first 1.55 s wall interval was not recorded: %+v", got)
	}
	coreApply(t, state, 0, Action{Type: "retire", PokemonID: id})
	coreApply(t, state, 1, Action{Type: "deploy", PokemonID: id, TileID: coreTile(t, state)})
	state.Enemies = []Enemy{}
	state.Wave = 100
	if err := Advance(state, 20); err != nil {
		t.Fatal(err)
	}
	if got := observeDexTower(t, state); got.Deployed != 2 || got.Active != 1 || got.Waves != 1 || got.Bosses != 1 || got.Challenges != 7 {
		t.Fatalf("re-deployment and completed boss wave did not preserve the cumulative record: %+v", got)
	}
}

func TestDexStatisticsCountStatusAndWeatherForTheActualTarget(t *testing.T) {
	state := coreNew(t, 2)
	coreApply(t, state, 0, Action{Type: "wave"})
	state.Enemies = []Enemy{}
	state.Weather = "sandstorm"
	for index, key := range []string{"rattata", "ledyba"} {
		id, err := SpawnEnemyAt(state, key, Point{X: 200 + float64(index)*30, Y: 200}, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		enemy := enemyFor(state, id)
		enemy.Entered, enemy.BaseSpeed = true, 0
		enemy.HP, enemy.MaxHP = 100, 100
		if index == 0 {
			enemy.Conditions = []Condition{{Kind: "burn", DurationMS: 5000, RemainingMS: 5000, Magnitude: .05, Stacks: 1, SourcePokemonID: state.Team[0].ID}}
		}
	}
	if err := Advance(state, 1250); err != nil {
		t.Fatal(err)
	}
	want := map[string]observedDexStats{"rattata": {Damage: 6}, "ledyba": {Damage: 1}}
	if got := observeDexStats(t, state); !reflect.DeepEqual(got, want) {
		t.Fatalf("indirect damage was omitted or assigned to the wrong enemy: %#v", got)
	}
}
