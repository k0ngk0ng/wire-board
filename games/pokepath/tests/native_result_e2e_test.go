package pokepath_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func completeResultWave(t *testing.T, state *engine.State) {
	t.Helper()
	progressionAct(t, state, 1, engine.Action{Type: "wave"})
	if len(state.Enemies) == 0 {
		t.Fatal("the real wave did not spawn")
	}
	state.Enemies = []engine.Enemy{}
	challengeAdvance(t, state, 100)
	if state.Phase != "route_complete" {
		t.Fatalf("expected a real route result, got %s", state.Phase)
	}
}

func TestNativeProgressionRouteResultConfirmationKeepsDeploymentAndRecord(t *testing.T) {
	state := progressionNew(t)
	id := state.Team[0].ID
	progressionAct(t, state, 1, engine.Action{Type: "deploy", PokemonID: id, TileID: 1})
	state.Wave, state.RouteWaves[state.Route] = 100, 100
	state.Health = 4
	completeResultWave(t, state)
	if state.Stars != 1 || state.Records[0] != 100 || state.Team[0].Dex.Challenges != 7 || state.Team[0].Dex.Bosses != 1 {
		t.Fatal("ordinary final result did not settle stars and deployed partner records")
	}
	for _, action := range []engine.Action{{Type: "wave"}, {Type: "route", Route: 0}, {Type: "upgrade", PokemonID: id}} {
		action.Epoch = state.Epoch
		draftRejectAction(t, state, action)
	}
	snapshot, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = engine.Unmarshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	gold, epoch, tower := state.Gold, state.Epoch, state.Towers[0]
	progressionAct(t, state, 2, engine.Action{Type: "continueRoute"})
	if state.Phase != "prepare" || state.Wave != 1 || state.Health != 14 || state.AutoWave || state.Gold != gold || state.Records[0] != 100 || state.Epoch <= epoch || len(state.Towers) != 1 || state.Towers[0].ID != tower.ID || state.Team[0].TileID != tower.TileID || state.Team[0].Dex.Challenges != 7 {
		t.Fatal("continuing a saved result unexpectedly changed ownership, collection or deployment")
	}
	draftRejectAction(t, state, engine.Action{Type: "continueRoute", Epoch: epoch})
	draftRejectAction(t, state, engine.Action{Type: "continueRoute", Epoch: state.Epoch})
	progressionAct(t, state, 1, engine.Action{Type: "wave"})
	if !state.WaveActive || state.Wave != 1 || len(state.Enemies) == 0 {
		t.Fatal("the original result did not lead back to a playable route")
	}
}

func TestNativeProgressionChallengeResultConfirmsAndClearsChallengeOnly(t *testing.T) {
	state := progressionChallengeFixture(t)
	id := state.Team[0].ID
	progressionAct(t, state, 0, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"mirror":true,"levelCap":50}`)})
	progressionAct(t, state, 1, engine.Action{Type: "team", Mode: "team", PokemonID: id})
	progressionAct(t, state, 1, engine.Action{Type: "deploy", PokemonID: id, TileID: 1})
	state.Wave, state.RouteWaves[state.Route] = 100, 100
	completeResultWave(t, state)
	if !state.Progression.Challenge.Active || state.Ribbons != 3 || state.Team[0].Dex.Challenges != 2 {
		t.Fatal("the final challenge did not settle its rewards before confirmation")
	}
	progressionAct(t, state, 3, engine.Action{Type: "continueRoute"})
	owner := progressionFind(state, "charmander")
	if state.Progression.Challenge.Active || state.Progression.ChallengeStartMS != nil || state.Ribbons != 3 || len(state.Team) != 0 || len(state.Box) != 30 || len(state.Towers) != 0 || state.Phase != "prepare" || owner == nil || owner.ID != id || owner.Dex.Challenges != 2 {
		t.Fatal("confirmation lost the collectible roster, rewards, or challenge record")
	}
}

func TestNativeProgressionDefeatRetryClearsOldWaveLocks(t *testing.T) {
	state := progressionNew(t)
	id := state.Team[0].ID
	progressionAct(t, state, 0, engine.Action{Type: "deploy", PokemonID: id, TileID: 1})
	state.Phase, state.Health, state.Wave = "defeat", 0, 24
	draftRejectAction(t, state, engine.Action{Type: "retry", Mode: "retry", Epoch: state.Epoch})
	state.Wave, state.Stars = 49, 450
	state.World.MovementLocked = true
	state.World.Pressure = true
	state.World.WaveElapsedMS = 3900
	state.WaveGold = 11
	epoch, gold := state.Epoch, state.Gold
	tower := state.Towers[0].ID
	progressionAct(t, state, 1, engine.Action{Type: "retry", Mode: "retry"})
	if state.Wave != 25 || state.Health != 10 || state.Phase != "prepare" || state.Epoch <= epoch || state.Gold != gold || state.World.MovementLocked || state.World.Pressure || state.World.WaveElapsedMS != 0 || state.WaveGold != 0 || len(state.Towers) != 1 || state.Towers[0].ID != tower {
		t.Fatal("retry did not restore the previous checkpoint and remove its stale battle locks")
	}
	draftRejectAction(t, state, engine.Action{Type: "retry", Mode: "retry", Epoch: epoch})
	progressionAct(t, state, 2, engine.Action{Type: "retire", PokemonID: id})
}

func TestNativePlatformResultHandsDecisionToHumans(t *testing.T) {
	module, ctx, match := nativeStart(t, 4)
	state := progressionNew(t)
	state.Wave, state.RouteWaves[state.Route] = 100, 100
	completeResultWave(t, state)
	var object map[string]json.RawMessage
	if err := json.Unmarshal(match.Data, &object); err != nil {
		t.Fatal(err)
	}
	snapshot, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	object["snapshot"] = snapshot
	match.Data, err = json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Automated = []bool{true, false, true, true}
	if action, err := module.BotAction(ctx, match); err != nil || len(action) != 0 {
		t.Fatalf("a bot took the victory confirmation away from a human: %s %v", action, err)
	}
	ctx.Automated = []bool{true, true, true, true}
	body, err := module.BotAction(ctx, match)
	var action engine.Action
	if err != nil || json.Unmarshal(body, &action) != nil || action.Type != "continueRoute" {
		t.Fatalf("the all-bot room could not continue: %s, %v", body, err)
	}
	next, err := module.Apply(ctx, match, body)
	if err != nil {
		t.Fatal(err)
	}
	for actor := 0; actor < 4; actor++ {
		before := nativeObserve(t, module, match, actor)
		after := nativeObserve(t, module, next, actor)
		if before.Wave != 100 || after.Wave != 1 || after.Gold != before.Gold || reflect.DeepEqual(before.Snapshot, after.Snapshot) {
			t.Fatal("the continuation did not affect every seat exactly once")
		}
	}
}
