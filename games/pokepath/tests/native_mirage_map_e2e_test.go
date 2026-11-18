package pokepath_test

import (
	"bytes"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pokepath/engine"
)

func TestNativeMirageMapOpenUsesTheOriginalChanceAndPermanentDiscoveryBoundaryE2E(t *testing.T) {
	state := waScenario(t)
	if engine.Project(state).Progression.SecretMaps[30] {
		t.Fatal("undiscovered Mirage Island was initially visible")
	}
	state.Random = 17
	waApply(t, state, 0, engine.Action{Type: "mapOpen"})
	if !engine.Project(state).Progression.SecretMaps[30] || state.Progression.SecretMaps[30] {
		t.Fatal("successful map-open roll was missing or permanently discovered the island before entry")
	}
	before, _ := engine.Marshal(state)
	if err := engine.Apply(state, 0, engine.Action{Type: "route", Route: 20, Epoch: state.Epoch}); err == nil {
		t.Fatal("temporary Mirage visibility unlocked a different secret route")
	}
	after, _ := engine.Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("denied island entry altered its opportunity or random state")
	}
	restored, err := engine.Unmarshal(before)
	if err != nil {
		t.Fatal(err)
	}
	if !engine.ProjectTransient(restored).Progression.SecretMaps[30] || restored.Progression.SecretMaps[30] {
		t.Fatal("save restoration lost the shared temporary map offer or prematurely made discovery permanent")
	}
	state.Random = 13000
	waApply(t, state, 1, engine.Action{Type: "mapOpen"})
	if engine.Project(state).Progression.SecretMaps[30] || state.Progression.SecretMaps[30] {
		t.Fatal("a failed later map opening retained an earlier undiscovered offer")
	}
	if err := engine.Apply(state, 0, engine.Action{Type: "route", Route: 30, Epoch: state.Epoch}); err == nil {
		t.Fatal("hidden undiscovered island accepted entry without a current offer")
	}
	state.Random = 17
	waApply(t, state, 2, engine.Action{Type: "mapOpen"})
	waApply(t, state, 3, engine.Action{Type: "route", Route: 30})
	if state.Route != 30 || !state.Progression.SecretMaps[30] {
		t.Fatal("legal offered island entry failed to establish permanent discovery")
	}
	random := state.Random
	waApply(t, state, 0, engine.Action{Type: "mapOpen"})
	if state.Random != random || !engine.Project(state).Progression.SecretMaps[30] {
		t.Fatal("permanently discovered island was rerolled or hidden")
	}
	waSnapshotContinuation(t, state, 40)
	waEvidence(t, "mirage-map-offer", state, map[string]any{"chance": .01, "minimumEntryStars": 0, "offerBeforeStars": true, "permanentOnlyAfterEntry": true, "sharedPlayers": 4})
}

func TestNativeMirageMapProjectionDoesNotRollAndRejectedOpenDoesNotMutateE2E(t *testing.T) {
	for _, blocked := range []string{"paused", "challenge", "actor", "epoch"} {
		state := waScenario(t)
		state.Random = 17
		actor, epoch := 0, state.Epoch
		switch blocked {
		case "paused":
			state.Paused = true
		case "challenge":
			state.Progression.Challenge.Active = true
		case "actor":
			actor = 4
		case "epoch":
			epoch--
		}
		before, err := engine.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		for range 3 {
			engine.Project(state)
			engine.ProjectTransient(state)
		}
		if err := engine.Apply(state, actor, engine.Action{Type: "mapOpen", Epoch: epoch}); err == nil {
			t.Fatal("invalid map opening was accepted", blocked)
		}
		after, _ := engine.Marshal(state)
		if !bytes.Equal(before, after) {
			t.Fatal("projection or invalid map request consumed random state or changed an offer", blocked)
		}
	}
}

func TestNativeMirageMapCanOpenDuringCombatWithoutUnlockingAnotherSecretRouteE2E(t *testing.T) {
	state := waScenario(t)
	waBattle(t, state)
	state.Random, state.Stars = 17, 2400
	waApply(t, state, 0, engine.Action{Type: "mapOpen"})
	if !engine.Project(state).Progression.SecretMaps[30] {
		t.Fatal("combat unnecessarily prevented the original map preview chance")
	}
	for _, id := range []int{30, 20} {
		if err := engine.Apply(state, 0, engine.Action{Type: "route", Route: id, Epoch: state.Epoch}); err == nil {
			t.Fatal("combat preview allowed an illegal route change", id)
		}
	}
	state.Enemies = []engine.Enemy{}
	waAdvance(t, state, 20)
	if err := engine.Apply(state, 0, engine.Action{Type: "route", Route: 20, Epoch: state.Epoch}); err == nil {
		t.Fatal("Mirage Island offer unlocked a different undiscovered secret route")
	}
}
