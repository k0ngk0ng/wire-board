package pokepath_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func draftOwned(state *engine.State) []engine.Pokemon {
	return append(slices.Clone(state.Team), state.Box...)
}

func draftAcquire(t *testing.T, state *engine.State, key string) {
	t.Helper()
	if slices.ContainsFunc(draftOwned(state), func(pokemon engine.Pokemon) bool { return pokemon.Species == key }) {
		return
	}
	pool := slices.Clone(state.Progression.EggPool)
	if !slices.Contains(pool, key) {
		t.Fatalf("expected %s in the uncollected pool", key)
	}
	state.Progression.EggPool = []string{key}
	progressionAct(t, state, 0, engine.Action{Type: "egg"})
	state.Progression.EggPool = slices.DeleteFunc(pool, func(value string) bool { return value == key })
}

func draftShuffle(ids []int, seed uint32) []int {
	cursor := &engine.State{Random: seed}
	result := slices.Clone(ids)
	for index := len(result) - 1; index > 0; index-- {
		other := engine.NextInt(cursor, index+1)
		result[index], result[other] = result[other], result[index]
	}
	return result
}

func draftChooseSeed(t *testing.T, pool []int, matches func([]int) bool) uint32 {
	t.Helper()
	for seed := uint32(1); seed < 100000; seed++ {
		if matches(draftShuffle(pool, seed)) {
			return seed
		}
	}
	t.Fatal("failed to construct original precedence fixture")
	return 0
}

func draftRejectAtomically(t *testing.T, state *engine.State, kind string, token int) {
	t.Helper()
	draftRejectAction(t, state, engine.Action{Type: kind, TargetID: token, Pick: 0, Epoch: state.Epoch})
}

func draftRejectAction(t *testing.T, state *engine.State, action engine.Action) {
	t.Helper()
	before, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Apply(state, 3, action); err == nil {
		t.Fatalf("%s accepted the illegal draft receipt %d", action.Type, action.TargetID)
	}
	after, err := engine.Marshal(state)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("a rejected draft operation mutated the world", err)
	}
}

func TestNativeProgressionDraftOriginalPrecedenceAndSharedRestart(t *testing.T) {
	state := progressionChallengeFixture(t)
	draftAcquire(t, state, "spoink")
	draftAcquire(t, state, "meowth")
	state.Stars = 2000
	state.Records[21] = 100
	progressionAct(t, state, 0, engine.Action{Type: "route", Route: 21})
	owned := append(slices.Clone(state.Box), state.Team...)
	ids := make([]int, 0, len(owned))
	facts := map[int]engine.Species{}
	for _, pokemon := range owned {
		ids = append(ids, pokemon.ID)
		facts[pokemon.ID] = engine.ProgressionSpecies(state, &pokemon)
	}
	frisk := func(id int) bool { return slices.Contains([]int{36, 4, 3, 92, 163}, facts[id].ID) }
	mountain := func(id int) bool { return slices.Contains(facts[id].Terrains, 4) }
	firstOthers := func(order []int) (int, int) {
		specialIndex := slices.IndexFunc(order, frisk)
		if specialIndex < 0 {
			return 0, 0
		}
		special := order[specialIndex]
		terrainIndex := slices.IndexFunc(order, func(id int) bool { return id != special && mountain(id) })
		if terrainIndex < 0 {
			return 0, 0
		}
		return special, order[terrainIndex]
	}
	state.Random = draftChooseSeed(t, ids, func(order []int) bool {
		special, terrain := firstOthers(order)
		return facts[special].ID == 3 && facts[order[0]].ID == 18 && slices.Index(order, special) < slices.Index(order, terrain)
	})
	firstOrder := draftShuffle(ids, state.Random)
	special, terrain := firstOthers(firstOrder)
	progressionAct(t, state, 2, engine.Action{Type: "challenge", Settings: json.RawMessage(`{"slotLimit":6,"draft":true}`)})
	draft := state.Progression.Draft
	if len(draft.Offers) != 3 || draft.Offers[0] != special || draft.Offers[1] != terrain || draft.Offers[2] != firstOrder[0] {
		t.Fatalf("the map's extra mountain guarantee was swallowed by the opening guarantee: want %v, got %v", []int{special, terrain, firstOrder[0]}, draft.Offers)
	}
	if !slices.Equal(draft.Pool, firstOrder) {
		t.Fatalf("draft pool did not retain the original shuffle order: %v", draft.Pool)
	}
	draftRejectAction(t, state, engine.Action{Type: "upgrade", PokemonID: draft.Offers[0], Epoch: state.Epoch})
	state.Random = draftChooseSeed(t, draft.Pool, func(order []int) bool { return facts[order[0]].ID == 18 })
	nextOrder := draftShuffle(draft.Pool, state.Random)
	startToken := draft.Token
	progressionAct(t, state, 1, engine.Action{Type: "draftReroll", TargetID: startToken})
	draft = state.Progression.Draft
	if !slices.Equal(draft.Pool, nextOrder) || !draft.CanReroll || draft.Round != 0 || draft.Token != startToken+1 {
		t.Fatalf("the reroll did not consume the saved order or refund the free reroll: %+v", draft)
	}
	draftRejectAtomically(t, state, "draftRestart", startToken)
	state.Random = draftChooseSeed(t, draft.Pool, func(order []int) bool {
		for _, id := range order[:16] {
			if slices.Contains([]int{18, 84, 105}, facts[id].ID) {
				return false
			}
		}
		return true
	})
	progressionAct(t, state, 3, engine.Action{Type: "draftReroll", TargetID: draft.Token})
	draft = state.Progression.Draft
	if draft.CanReroll {
		t.Fatalf("ordinary reroll unexpectedly granted an extra use: %+v", draft)
	}
	draftRejectAtomically(t, state, "draftReroll", draft.Token)
	picked := draft.Offers[0]
	progressionAct(t, state, 0, engine.Action{Type: "draftPick", Pick: 0, TargetID: draft.Token})
	draft = state.Progression.Draft
	if !draft.CanReroll || draft.Round != 1 || len(state.Team) != 1 || state.Team[0].ID != picked || slices.Contains(draft.Pool, picked) {
		t.Fatalf("wrong transition after selection: %+v", draft)
	}
	snapshot, err := engine.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := engine.Unmarshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	restart := engine.Action{Type: "draftRestart", TargetID: draft.Token}
	progressionAct(t, state, 2, restart)
	progressionAct(t, restored, 2, restart)
	final, _ := engine.Marshal(state)
	replayed, _ := engine.Marshal(restored)
	if !bytes.Equal(final, replayed) || len(state.Team) != 0 || len(state.Box) != len(owned) || !slices.Contains(state.Progression.Draft.Pool, picked) || state.Progression.Draft.Round != 0 || !state.Progression.Draft.CanReroll || state.Progression.Draft.RestartCount != 1 {
		t.Fatalf("restart did not rebuild the same shared pool after restoring the snapshot: %+v", state.Progression.Draft)
	}
	draftRejectAtomically(t, state, "draftRestart", draft.Token)
	for state.Progression.Draft.Active {
		progressionAct(t, state, 1, engine.Action{Type: "draftPick", Pick: 0, TargetID: state.Progression.Draft.Token})
	}
	draftRejectAtomically(t, state, "draftRestart", state.Progression.Draft.Token)
}

func TestNativeProgressionChallengeReplacesRunningWave(t *testing.T) {
	state := progressionChallengeFixture(t)
	pokemonID := state.Team[0].ID
	progressionAct(t, state, 0, engine.Action{Type: "deploy", PokemonID: pokemonID, TileID: 1})
	progressionAct(t, state, 0, engine.Action{Type: "wave"})
	if err := engine.Advance(state, 1250); err != nil {
		t.Fatal(err)
	}
	if !state.WaveActive || len(state.Enemies) == 0 || state.World.WaveElapsedMS == 0 {
		t.Fatal("expected a running wave before the challenge")
	}
	request := engine.Action{Type: "challenge", Settings: json.RawMessage(`{"levelCap":50}`), Epoch: state.Epoch}
	progressionAct(t, state, 1, engine.Action{Type: "pause"})
	if engine.Project(state).Progression.CanChallenge || engine.Apply(state, 3, request) == nil {
		t.Fatal("a paused board opened a new challenge")
	}
	progressionAct(t, state, 1, engine.Action{Type: "resume"})
	if !engine.Project(state).Progression.CanChallenge {
		t.Fatal("original challenge is available during a running, unpaused wave")
	}
	gold := state.Gold
	owned := len(draftOwned(state))
	progressionAct(t, state, 3, request)
	if state.WaveActive || state.Wave != 1 || state.Health != 14 || state.Gold != gold || len(draftOwned(state)) != owned || len(state.Team) != 0 || len(state.Towers) != 0 || len(state.Enemies) != 0 || len(state.Projectiles) != 0 || state.World.WaveElapsedMS != 0 || state.WaveGold != 0 || state.TotalDamage != 0 || !state.Progression.Challenge.Active {
		t.Fatalf("challenge did not replace the running wave: %s wave %d, gold %d, elapsed %d", state.Phase, state.Wave, state.Gold, state.World.WaveElapsedMS)
	}
	draftRejectAction(t, state, engine.Action{Type: "wave", Epoch: request.Epoch})
}
