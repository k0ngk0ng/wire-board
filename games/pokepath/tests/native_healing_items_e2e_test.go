package pokepath_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func healingEquip(t *testing.T, state *engine.State, owner int, key string) {
	t.Helper()
	found := false
	for _, item := range state.Inventory {
		found = found || item.ID == key
	}
	if !found {
		for index, item := range state.Progression.ShopStock {
			if item == key {
				state.Progression.ShopStock[index] = ""
			}
		}
		state.Inventory = append(state.Inventory, engine.OwnedItem{ID: key})
	}
	waApply(t, state, 0, engine.Action{Type: "equip", PokemonID: owner, ItemID: key})
}

func healingHit(t *testing.T, state *engine.State, owner, target int) {
	t.Helper()
	for index := range state.Towers {
		tower := &state.Towers[index]
		tower.CooldownMS = 100000
		if tower.PokemonID == owner {
			tower.CooldownMS = 0
			enemy := persistentAreaEnemy(t, state, target)
			enemy.X, enemy.Y = tower.X+20, tower.Y
		}
	}
	state.Random = 17
	waAdvance(t, state, 20)
}

func TestNativeHealingDamageThresholdsAreStrictAndResetPerWaveE2E(t *testing.T) {
	for _, scenario := range []struct {
		species, item string
		goal          float64
	}{{"clefairy", "clefairyDoll", 25000}, {"charmander", "shellBell", 50000}} {
		t.Run(scenario.item, func(t *testing.T) {
			state := waScenario(t)
			owner := waDeploy(t, state, scenario.species, 1)
			healingEquip(t, state, owner, scenario.item)
			waBattle(t, state)
			state.Health = 10
			target := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
			power := state.Team[0].Stats.Power
			state.World.Towers[owner].WaveDamage = scenario.goal - power
			healingHit(t, state, owner, target)
			if state.Health != 10 {
				t.Fatal("damage-goal item healed at equality instead of strict greater-than")
			}
			waSnapshotContinuation(t, state, 40)
			healingHit(t, state, owner, target)
			if state.Health != 11 {
				t.Fatal("crossing the real wave damage goal did not restore one heart", state.Health)
			}
			healingHit(t, state, owner, target)
			if state.Health != 11 {
				t.Fatal("damage-goal item restored a second heart in the same wave")
			}
			state.Enemies = []engine.Enemy{}
			waAdvance(t, state, 20)
			waBattle(t, state)
			state.World.Towers[owner].WaveDamage = scenario.goal + 1
			target = waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
			healingHit(t, state, owner, target)
			if state.Health != 12 {
				t.Fatal("new wave failed to reset the damage-healing claim")
			}
			waEvidence(t, "healing-"+scenario.item, state, map[string]any{"strictGoal": scenario.goal, "globalClaimsPerWave": 1, "resetNextWave": true})
		})
	}
}

func TestNativeHealingShellBellSharesOneClaimWithMagicianE2E(t *testing.T) {
	state := waScenario(t)
	first := waDeploy(t, state, "charmander", 1)
	second := waDeploy(t, state, "fennekin", 1)
	healingEquip(t, state, first, "shellBell")
	healingEquip(t, state, second, "shellBell")
	waBattle(t, state)
	state.Health = 10
	target := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	state.World.Towers[first].WaveDamage = 50001
	state.World.Towers[second].WaveDamage = 50001
	healingHit(t, state, first, target)
	healingHit(t, state, second, target)
	if state.Health != 11 {
		t.Fatal("two legal holders consumed more than the one global shell-bell claim", state.Health)
	}
	waEvidence(t, "healing-shell-global", state, map[string]any{"legalCopyAbility": "magician", "sharedItemClaims": 1})
}

func TestNativeHealingPropCaseLosesHeartsThenRestoresPendingWaveHealsE2E(t *testing.T) {
	for _, health := range []int{5, 1} {
		state := waScenario(t)
		owner := waDeploy(t, state, "chikorita", 1)
		healingEquip(t, state, owner, "propCase")
		waBattle(t, state)
		state.Health = health
		target := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
		for index := 0; index < 3; index++ {
			healingHit(t, state, owner, target)
		}
		want := max(1, health-3)
		if state.Health != want {
			t.Fatal("prop case restored hearts immediately or became lethal", health, state.Health, want)
		}
		waSnapshotContinuation(t, state, 40)
		healingHit(t, state, owner, target)
		if state.Health != want {
			t.Fatal("prop-case proc cap was not attributed to the owned partner")
		}
		state.Enemies = []engine.Enemy{}
		waAdvance(t, state, 20)
		if state.Health != min(state.MaxHealth, want+3) {
			t.Fatal("wave end lost or duplicated the three pending prop-case heals", state.Health)
		}
		waEvidence(t, "healing-prop-case", state, map[string]any{"initialHealth": health, "nonlethal": true, "pendingRestored": 3})
	}
}

func TestNativeHealingProcCapsBelongToEachOwnedPartnerE2E(t *testing.T) {
	state := waScenario(t)
	first := waDeploy(t, state, "chikorita", 1)
	second := waDeploy(t, state, "cottonee", 1)
	waBattle(t, state)
	state.Health = 1
	target := waEnemy(t, state, engine.Point{X: state.Towers[0].X + 20, Y: state.Towers[0].Y})
	for index := 0; index < 3; index++ {
		healingHit(t, state, first, target)
		healingHit(t, state, second, target)
	}
	if state.Health != 7 {
		t.Fatal("one healer consumed the other healer's independent three-proc limit", state.Health)
	}
	healingHit(t, state, first, target)
	healingHit(t, state, second, target)
	if state.Health != 7 {
		t.Fatal("owned healing proc cap was not enforced after six total heals")
	}
	waEvidence(t, "healing-owned-caps", state, map[string]any{"owners": 2, "healsEach": 3, "totalHeals": 6})
}

func TestNativeHealingRoomServiceIsGlobalAndCountsFullHealthEventsE2E(t *testing.T) {
	state := waScenario(t)
	room := waDeploy(t, state, "turtwig", 1)
	healer := waDeploy(t, state, "chikorita", 1)
	attacker := waDeploy(t, state, "charmander", 1)
	healingEquip(t, state, room, "roomService")
	healingEquip(t, state, healer, "bigRoot")
	waBattle(t, state)
	state.Health = state.MaxHealth
	target := waEnemy(t, state, engine.Point{X: state.Towers[1].X + 20, Y: state.Towers[1].Y})
	healingHit(t, state, healer, target)
	if state.Health != state.MaxHealth {
		t.Fatal("full-health healing event changed maximum health")
	}
	assertPower := func(bonus float64) {
		t.Helper()
		view := engine.Project(state)
		for _, pokemon := range view.Team {
			if pokemon.ID == attacker {
				if pokemon.CombatStats == nil || pokemon.CombatStats.Power != pokemon.Stats.Power+bonus {
					t.Fatal("room service was local, omitted a full-health event, or counted two hearts as two events", pokemon.CombatStats, bonus)
				}
				return
			}
		}
		t.Fatal("missing room-service recipient")
	}
	assertPower(20)
	roomTile := state.Towers[0].TileID
	waApply(t, state, 0, engine.Action{Type: "retire", PokemonID: room})
	assertPower(0)
	waApply(t, state, 0, engine.Action{Type: "deploy", PokemonID: room, TileID: roomTile})
	assertPower(20)
	waSnapshotContinuation(t, state, 200)
	waEvidence(t, "healing-room-service", state, map[string]any{"globalPowerPerEvent": 20, "fullHealthEventCounts": true, "bigRootHeartAmount": 2})
}
