package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type aiRecordedAction struct {
	At         int64  `json:"at"`
	Actor      int    `json:"actor"`
	Action     Action `json:"action"`
	GoldBefore int64  `json:"goldBefore"`
	GoldAfter  int64  `json:"goldAfter"`
}

type aiRecordedStage struct {
	At     int64     `json:"at"`
	Wave   int       `json:"wave"`
	Record int       `json:"record"`
	Gold   int64     `json:"gold"`
	Health int       `json:"health"`
	Team   []Pokemon `json:"team"`
}

func aiSaveArtifact(t *testing.T, filename string, value any) {
	t.Helper()
	directory := filepath.Join("..", "..", "..", ".local", "pokepath", "verification", "native-ai")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, filename), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAIProposalIsReadOnlyAndSharedResourcesRemainLegal(t *testing.T) {
	state, err := New(Options{Starter: "charmander"}, 4, 123456, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := Marshal(state)
	first := Propose(state)
	second := Propose(state)
	after, _ := Marshal(state)
	if !bytes.Equal(before, after) || !reflect.DeepEqual(first, second) {
		t.Fatal("proposal changed the shared state or random stream")
	}
	trace := []aiRecordedAction{}
	for controller := 0; controller < 40; controller++ {
		action := Propose(state)
		if action == nil {
			break
		}
		gold := state.Gold
		if err := Apply(state, controller%4, *action); err != nil {
			t.Fatalf("controller %d action %+v: %v", controller%4, *action, err)
		}
		if state.Gold < 0 || action.Epoch != state.Epoch {
			t.Fatal("shared resource overdraft or stale bot action")
		}
		trace = append(trace, aiRecordedAction{Actor: controller % 4, Action: *action, GoldBefore: gold, GoldAfter: state.Gold})
	}
	if len(state.Team)+len(state.Box) < 2 {
		t.Fatal("the bot did not grow from the actual initial economy")
	}
	aiSaveArtifact(t, "shared-resources.json", map[string]any{"command": "go test ./games/pokepath/engine -run '^TestAI' -count=1 -v", "initialGold": 50, "initialLevel": 1, "actions": trace, "final": Project(state)})
}

func aiRunCampaign(t *testing.T, starter string, players int, seed uint64, target int, limit int64) *State {
	t.Helper()
	state, err := New(Options{Starter: starter}, players, seed, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	trace := []aiRecordedAction{}
	stages := []aiRecordedStage{}
	defer func() {
		aiSaveArtifact(t, fmt.Sprintf("%s-%d-%d-%d.json", starter, players, seed, target), map[string]any{"command": "go test ./games/pokepath/engine -run '^TestAI' -count=1 -v", "starter": starter, "players": players, "seed": seed, "initialGold": 50, "initialLevel": 1, "botCadenceMS": 100, "actions": trace, "stages": stages, "state": state, "final": Project(state), "passed": state.Records[0] >= target})
	}()
	previous := ""
	lastProgress := int64(0)
	for elapsed := int64(0); elapsed < limit && state.Records[0] < target; elapsed += 100 {
		if elapsed%120000 == 0 {
			aiSaveArtifact(t, fmt.Sprintf("progress-%s-%d-%d.json", starter, players, seed), map[string]any{"elapsed": elapsed, "wave": state.Wave, "record": state.Records[0], "gold": state.Gold, "health": state.Health, "phase": state.Phase, "team": state.Team, "state": state, "lastActions": trace[max(0, len(trace)-8):]})
		}
		action := Propose(state)
		if action != nil {
			gold := state.Gold
			actor := int(elapsed/100) % players
			if err := Apply(state, actor, *action); err != nil {
				t.Fatalf("at %d wave %d record %d action %+v: %v", elapsed, state.Wave, state.Records[0], *action, err)
			}
			trace = append(trace, aiRecordedAction{At: elapsed, Actor: actor, Action: *action, GoldBefore: gold, GoldAfter: state.Gold})
		}
		if state.Gold < 0 {
			t.Fatal("negative shared gold")
		}
		if err := Advance(state, 100); err != nil {
			t.Fatal(err)
		}
		stage := fmt.Sprintf("%d/%d/%d/%s/%d", state.Wave, state.Records[0], state.Health, state.Phase, state.TotalKills)
		if stage != previous {
			previous = stage
			lastProgress = elapsed
			if len(stages) == 0 || stages[len(stages)-1].Wave != state.Wave || stages[len(stages)-1].Record != state.Records[0] || stages[len(stages)-1].Health != state.Health {
				stages = append(stages, aiRecordedStage{At: elapsed, Wave: state.Wave, Record: state.Records[0], Gold: state.Gold, Health: state.Health, Team: append([]Pokemon{}, state.Team...)})
			}
		}
		if elapsed-lastProgress > 600000 {
			t.Fatalf("no progress for 10 minutes: wave=%d record=%d phase=%s gold=%d health=%d", state.Wave, state.Records[0], state.Phase, state.Gold, state.Health)
		}
	}
	if state.Records[0] < target {
		t.Fatalf("real campaign stopped at record %d/%d wave=%d gold=%d health=%d team=%+v", state.Records[0], target, state.Wave, state.Gold, state.Health, state.Team)
	}
	return state
}

func TestAIAllStartersAndSharedPlayerCountsE2E(t *testing.T) {
	for index, starter := range []string{"charmander", "treecko", "froaki"} {
		for _, players := range []int{1, 2, 4} {
			t.Run(fmt.Sprintf("%s-%d", starter, players), func(t *testing.T) {
				aiRunCampaign(t, starter, players, uint64(42000+index*10+players), 10, 3600000)
			})
		}
	}
}

func TestAIMovementLockKeepsOrdinaryActionsLegal(t *testing.T) {
	state, err := New(Options{Starter: "charmander"}, 2, 32567, 1730000000000)
	if err != nil {
		t.Fatal(err)
	}
	pokemon, err := newPokemon(state, "bronzor", 1)
	if err != nil {
		t.Fatal(err)
	}
	state.Team = append(state.Team, pokemon)
	if err := Apply(state, 0, Action{Type: "speed", Epoch: state.Epoch, Value: 2.5}); err != nil {
		t.Fatal(err)
	}
	if err := Apply(state, 0, Action{Type: "wave", Epoch: state.Epoch}); err != nil {
		t.Fatal(err)
	}
	route, _ := routeForState(state)
	for _, tile := range route.Placements {
		if !terrainAllowed(pokemon, CatalogData().Species[pokemon.Species], tile.Terrain) {
			continue
		}
		if err := Apply(state, 1, Action{Type: "deploy", Epoch: state.Epoch, PokemonID: pokemon.ID, TileID: tile.ID}); err != nil {
			t.Fatal(err)
		}
		break
	}
	if !state.World.MovementLocked {
		t.Fatal("heavy metal did not lock the wave")
	}
	before, _ := Marshal(state)
	action := Propose(state)
	after, _ := Marshal(state)
	if !bytes.Equal(before, after) {
		t.Fatal("locked proposal changed the shared state")
	}
	if action != nil {
		if action.Type == "deploy" || action.Type == "team" || action.Type == "equip" || action.Type == "buyItem" {
			t.Fatalf("locked bot proposed an unavailable operation: %+v", action)
		}
		if err := Apply(state, 0, *action); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAICompleteFirstHundredWavesFromActualInitialStateE2E(t *testing.T) {
	state := aiRunCampaign(t, "charmander", 1, 123456, 100, 28800000)
	if state.Records[0] != 100 || state.TotalKills < 100 || state.Health <= 0 {
		t.Fatal("the actual 100-wave route was not completed")
	}
}
