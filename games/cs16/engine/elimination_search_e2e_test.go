package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestEliminationFixedSeedNaturalCombatE2E(t *testing.T) {
	for _, seed := range []uint64{1, 2, 3, 16, 20261122} {
		t.Run(strconv.FormatUint(seed, 10), func(t *testing.T) {
			state, err := New(Options{Map: "fy_iceworld", RoundSeconds: 60, MaxRounds: 2}, 10, seed)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			cursor := int64(0)
			type sample struct {
				Frame   int64    `json:"frame"`
				Round   int      `json:"round"`
				Phase   string   `json:"phase"`
				Players []Player `json:"players"`
			}
			samples := []sample{}
			for !state.Finished && state.Frame < 270*FPS {
				if state.Frame%(10*FPS) == 0 {
					players := make([]Player, len(state.Players))
					for seat, player := range state.Players {
						players[seat] = player
						players[seat].Inventory = append([]WeaponState(nil), player.Inventory...)
						players[seat].BotPath = append([]int(nil), player.BotPath...)
						players[seat].Pending = nil
					}
					samples = append(samples, sample{state.Frame, state.Round, state.Phase, players})
				}
				for seat := range state.Players {
					if item := BotPurchase(state, seat); item != "" {
						if err := Buy(state, seat, item); err != nil {
							t.Fatal(err)
						}
						continue
					}
					if state.Players[seat].Alive {
						if err := QueueInput(state, seat, BotInput(state, seat)); err != nil {
							t.Fatal(err)
						}
					}
				}
				Advance(state, state.Frame+3)
				for _, event := range state.Events {
					if event.ID <= cursor {
						continue
					}
					counts[event.Kind]++
					cursor = event.ID
					if event.Kind == "round_end" && event.Weapon != "elimination" {
						t.Fatalf("invented round result %s", event.Weapon)
					}
				}
			}
			directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "elimination-search")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			report := map[string]any{"seed": seed, "passed": state.Finished, "frame": state.Frame, "counts": counts, "samples": samples}
			if !state.Finished {
				report["checkpoint"] = state
			}
			raw, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "seed-"+strconv.FormatUint(seed, 10)+".json"), raw, 0644); err != nil {
				t.Fatal(err)
			}
			if !state.Finished || counts["round_end"] != 2 || counts["shot"] == 0 || counts["pickup"] == 0 {
				t.Fatalf("seed=%d frame=%d round=%d phase=%s score=%v counts=%v", seed, state.Frame, state.Round, state.Phase, state.TeamScores, counts)
			}
			t.Logf("seed=%d frame=%d score=%v counts=%v", seed, state.Frame, state.TeamScores, counts)
		})
	}
}

func eliminationSearchArena() *State {
	state := objectiveFixture("elimination")
	state.World.Bombsites, state.World.Hostages = nil, nil
	state.Bomb = Bomb{Carrier: -1, Defuser: -1}
	state.World.Nav = []Vec3{{-200 * Unit, -200 * Unit, 36 * Unit}, {200 * Unit, -200 * Unit, 36 * Unit}, {-200 * Unit, 200 * Unit, 36 * Unit}, {200 * Unit, 200 * Unit, 36 * Unit}, {5000 * Unit, 5000 * Unit, 36 * Unit}}
	state.World.NavEdges = [][]int{{1, 2}, {0, 3}, {0, 3}, {1, 2}, nil}
	state.World.SpawnsT = []Spawn{{X: -200 * Unit, Y: -200 * Unit, Z: 36 * Unit}, {X: 200 * Unit, Y: -200 * Unit, Z: 36 * Unit}, {X: 5000 * Unit, Y: 5000 * Unit, Z: 36 * Unit}}
	state.World.SpawnsCT = []Spawn{{X: -200 * Unit, Y: 200 * Unit, Z: 36 * Unit}, {X: 200 * Unit, Y: 200 * Unit, Z: 36 * Unit}}
	state.Players[0].X, state.Players[0].Y = -200*Unit, -200*Unit
	state.Players[1].X, state.Players[1].Y = 10000*Unit, 10000*Unit
	InitInventory(&state.Players[0])
	return state
}

func TestEliminationSearchCoverageAndPrivateRestoreE2E(t *testing.T) {
	state := eliminationSearchArena()
	visited := map[int]bool{}
	for state.Frame < 3000 {
		if err := QueueInput(state, 0, BotInput(state, 0)); err != nil {
			t.Fatal(err)
		}
		Advance(state, state.Frame+3)
		for index, point := range state.World.Nav {
			if Distance(Position(state.Players[0]), point) <= 48*Unit {
				visited[index] = true
			}
		}
		for _, node := range state.Players[0].BotPath {
			if node == 4 {
				t.Fatal("search entered an unreachable NAV component")
			}
		}
	}
	if len(visited) != 4 || visited[4] {
		t.Fatalf("search repeated one lane without covering reachable spawn areas: %v", visited)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.World = state.World
	for range 200 {
		input := BotInput(state, 0)
		if input != BotInput(&restored, 0) {
			t.Fatal("private restoration changed the next search input")
		}
		if err := QueueInput(state, 0, input); err != nil {
			t.Fatal(err)
		}
		if err := QueueInput(&restored, 0, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, state.Frame+3)
		Advance(&restored, restored.Frame+3)
	}
	wanted, _ := json.Marshal(state)
	actual, _ := json.Marshal(&restored)
	if string(wanted) != string(actual) {
		t.Fatal("private restoration changed search progression or rules")
	}
}

func TestEliminationSearchDoesNotTrackHiddenEnemyE2E(t *testing.T) {
	state := eliminationSearchArena()
	for range 400 {
		before, _ := json.Marshal(state)
		input := BotInput(state, 0)
		after, _ := json.Marshal(state)
		if string(before) != string(after) {
			t.Fatal("search input mutated authority")
		}
		state.Players[1].X = -10000 * Unit
		other := BotInput(state, 0)
		state.Players[1].X = 10000 * Unit
		if input != other {
			t.Fatal("unseen enemy movement changed public-map search")
		}
		if err := QueueInput(state, 0, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, state.Frame+3)
	}
}
