package pokepath_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func challengeElapsed(t *testing.T, state *engine.State) int64 {
	t.Helper()
	body, err := json.Marshal(engine.Project(state).Progression)
	if err != nil {
		t.Fatal(err)
	}
	var clock struct {
		Elapsed int64 `json:"challengeElapsedMS"`
	}
	if err := json.Unmarshal(body, &clock); err != nil {
		t.Fatal(err)
	}
	return clock.Elapsed
}

func challengeAdvance(t *testing.T, state *engine.State, ms int64) {
	t.Helper()
	if err := engine.Advance(state, ms); err != nil {
		t.Fatal(err)
	}
}

func TestNativeProgressionChallengeTimerFollowsRealTimeAcrossWaves(t *testing.T) {
	state := progressionChallengeFixture(t)
	starter := state.Team[0].ID
	progressionAct(t, state, 2, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50}`)})
	challengeAdvance(t, state, 1703)
	if got := challengeElapsed(t, state); got != 0 {
		t.Fatalf("preparation consumed %dms", got)
	}
	progressionAct(t, state, 1, engine.Action{Type: "team", Mode: "team", PokemonID: starter})
	progressionAct(t, state, 1, engine.Action{Type: "deploy", TileID: 1, PokemonID: starter})
	progressionAct(t, state, 2, engine.Action{Type: "speed", Value: 2.5})
	progressionAct(t, state, 0, engine.Action{Type: "wave"})
	started := state.Clock
	challengeAdvance(t, state, 1047)
	if got := challengeElapsed(t, state); got != 1047 || state.Tick <= state.Clock {
		t.Fatalf("clock used physics speed or discarded millisecond remainder: %d", got)
	}
	progressionAct(t, state, 1, engine.Action{Type: "pause"})
	challengeAdvance(t, state, 7000)
	if got := challengeElapsed(t, state); got != 1047 {
		t.Fatalf("pause changed the clock: %d", got)
	}
	progressionAct(t, state, 0, engine.Action{Type: "resume"})
	for step := 0; step < 200 && state.WaveActive; step++ {
		challengeAdvance(t, state, 500)
	}
	if state.WaveActive || state.Wave != 2 || state.Health <= 0 {
		t.Fatal("first real challenge wave did not finish")
	}
	snapshot, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := engine.Unmarshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	challengeAdvance(t, resumed, 703)
	want := state.Clock - started + 703
	if got := challengeElapsed(t, resumed); got != want {
		t.Fatalf("between-wave time or checkpoint was lost: %d, want %d", got, want)
	}
	progressionAct(t, resumed, 2, engine.Action{Type: "wave"})
	challengeAdvance(t, resumed, 111)
	if got := challengeElapsed(t, resumed); got != want+111 {
		t.Fatalf("next wave restarted the challenge timer: %d, want %d", got, want+111)
	}
}

func TestNativeProgressionChallengeTimerFreezesDefeatAndResumesRetry(t *testing.T) {
	state := progressionChallengeFixture(t)
	progressionAct(t, state, 0, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"mirror":true}`)})
	state.Health = 1
	progressionAct(t, state, 1, engine.Action{Type: "wave"})
	for step := 0; step < 400 && state.Health > 0; step++ {
		challengeAdvance(t, state, 500)
	}
	elapsed := challengeElapsed(t, state)
	if state.Phase != "defeat" || elapsed == 0 || elapsed != state.Clock {
		t.Fatalf("first-wave clock anchored at zero was not retained at defeat: now %d, readout %d, phase %s", state.Clock, elapsed, state.Phase)
	}
	challengeAdvance(t, state, 10000)
	if got := challengeElapsed(t, state); got != elapsed {
		t.Fatalf("the defeat screen consumed time: %d", got)
	}
	initial := state.Epoch
	progressionAct(t, state, 1, engine.Action{Type: "retry", Mode: "restart"})
	challengeAdvance(t, state, 593)
	if state.Epoch <= initial || !state.Progression.Challenge.Active || challengeElapsed(t, state) != elapsed+593 {
		t.Fatal("an ordinary restart left the challenge or discarded its time")
	}
}

func TestNativeProgressionSurrenderLeavesTheRunningWave(t *testing.T) {
	state := progressionChallengeFixture(t)
	starter := state.Team[0].ID
	normalPower := state.Team[0].Stats.Power
	state.Inventory = append(state.Inventory, engine.OwnedItem{ID: "leftovers"})
	progressionAct(t, state, 2, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"mirror":true,"levelCap":50,"toughEnemies":50}`)})
	progressionAct(t, state, 1, engine.Action{Type: "team", Mode: "team", PokemonID: starter})
	progressionAct(t, state, 1, engine.Action{Type: "equip", PokemonID: starter, ItemID: "leftovers"})
	progressionAct(t, state, 1, engine.Action{Type: "deploy", PokemonID: starter, TileID: 1})
	progressionAct(t, state, 2, engine.Action{Type: "wave"})
	progressionAct(t, state, 2, engine.Action{Type: "auto"})
	challengeAdvance(t, state, 900)
	before := slices.Clone(state.Enemies)
	if len(before) == 0 || !before[0].PathMirrored || state.World.WaveElapsedMS == 0 || len(state.Towers) == 0 {
		t.Fatal("fixture did not reach a real mirrored battle")
	}
	path := slices.Clone(engine.EnemyPath(state, &before[0]))
	gold, health, waveClock, epoch := state.Gold, state.Health, state.World.WaveElapsedMS, state.Epoch
	progressionAct(t, state, 1, engine.Action{Type: "pause"})
	draftRejectAction(t, state, engine.Action{Type: "surrender", Epoch: state.Epoch})
	progressionAct(t, state, 1, engine.Action{Type: "resume"})
	progressionAct(t, state, 3, engine.Action{Type: "surrender"})
	if !state.WaveActive || state.Phase != "battle" || !state.AutoWave || state.Wave != 1 || state.Gold != gold || state.Health != health || len(state.Enemies) != len(before) || state.World.WaveElapsedMS != waveClock || state.Epoch <= epoch || state.Progression.Challenge.Active || challengeElapsed(t, state) != 0 {
		t.Fatal("surrender cleared or advanced the running wave, or left a stale control boundary")
	}
	if len(state.Team) != 0 || len(state.Box) != 30 || len(state.Towers) != 0 || len(state.Projectiles) != 0 || state.Inventory[0].EquippedBy != 0 {
		t.Fatal("surrender did not recall all partners and their in-flight attacks and detach equipment")
	}
	member := slices.IndexFunc(state.Box, func(pokemon engine.Pokemon) bool { return pokemon.ID == starter })
	if member < 0 || state.Box[member].Item != "" || state.Box[member].TileID >= 0 || state.Box[member].Stats.Power != normalPower {
		t.Fatal("surrender did not restore the original owned partner")
	}
	for index, enemy := range state.Enemies {
		if enemy.ID != before[index].ID || enemy.HP != before[index].HP || enemy.X != before[index].X || enemy.Y != before[index].Y || enemy.Waypoint != before[index].Waypoint {
			t.Fatal("surrender altered already-spawned enemies")
		}
	}
	if !reflect.DeepEqual(path, engine.EnemyPath(state, &state.Enemies[0])) {
		t.Fatal("surrender rerouted the existing enemies")
	}
	challengeAdvance(t, state, 175)
	if state.Enemies[0].X == before[0].X && state.Enemies[0].Y == before[0].Y {
		t.Fatal("the old enemies stopped after surrender")
	}
	draftRejectAction(t, state, engine.Action{Type: "wave", Epoch: epoch})
}

func TestNativePlatformChallengeTimerSurvivesFrozenSave(t *testing.T) {
	module, ctx, match := nativeStart(t, 4)
	state := progressionChallengeFixture(t)
	progressionAct(t, state, 0, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"noItems":true}`)})
	progressionAct(t, state, 1, engine.Action{Type: "wave"})
	var encoded map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &encoded); err != nil {
		t.Fatal(err)
	}
	snapshot, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	encoded["snapshot"] = snapshot
	match.Data, err = json.Marshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	read := func(viewer int) int64 {
		current, err := engine.Unmarshal(nativeObserve(t, module, match, viewer).Snapshot)
		if err != nil {
			t.Fatal(err)
		}
		return challengeElapsed(t, current)
	}
	ctx.Now = ctx.Now.Add(975 * time.Millisecond)
	match, err = module.SaveControl(ctx, match, true)
	if err != nil {
		t.Fatal(err)
	}
	for viewer := 0; viewer < 4; viewer++ {
		if got := read(viewer); got != 975 {
			t.Fatalf("seat %d did not see the authoritative frozen 975ms, got %d", viewer+1, got)
		}
	}
	ctx.Now = ctx.Now.Add(48 * time.Hour)
	match, err = module.ResumeSaved(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(2); got != 975 {
		t.Fatalf("offline time was counted by the resumed challenge: %d", got)
	}
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(125 * time.Millisecond)
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if got := read(3); got != 1100 {
		t.Fatalf("resuming did not retain 975ms and advance exactly 125ms: %d", got)
	}
}

func TestNativeProgressionChallengeVictoryFreezesTimeAndAwardsRibbons(t *testing.T) {
	state := progressionChallengeFixture(t)
	progressionAct(t, state, 2, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50}`)})
	state.Wave, state.RouteWaves[state.Route] = 100, 100
	progressionAct(t, state, 3, engine.Action{Type: "wave"})
	challengeAdvance(t, state, 712)
	state.Enemies = nil
	challengeAdvance(t, state, 100)
	elapsed := challengeElapsed(t, state)
	if state.Phase != "route_complete" || elapsed < 712 || elapsed > 812 || state.Ribbons != 2 {
		t.Fatalf("the completed challenge lost its result or terminal time: phase %s, time %d, ribbons %d", state.Phase, elapsed, state.Ribbons)
	}
	before, _ := engine.Marshal(state)
	challengeAdvance(t, state, 50000)
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) || challengeElapsed(t, state) != elapsed {
		t.Fatal("the victory screen changed the final time or awarded the ribbons again")
	}
}
