package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSmallMapsEntityModeAndTenSeatStartE2E(t *testing.T) {
	for _, id := range []string{"fy_iceworld", "cs_bloodstrike"} {
		world := MapByID(id)
		if world.ID != id || world.Mode != "elimination" || len(world.SpawnsT) < 5 || len(world.SpawnsCT) < 5 {
			t.Fatalf("missing proven ten-seat elimination map %s", id)
		}
		if len(world.Bombsites) != 0 || len(world.Hostages) != 0 {
			t.Fatalf("invented objective for %s", id)
		}
		state, err := New(Options{Map: id, RoundSeconds: 60, MaxRounds: 2, FriendlyFire: true}, 10, 20261122)
		if err != nil {
			t.Fatal(err)
		}
		teams := map[string]int{}
		for _, p := range state.Players {
			teams[p.Team]++
			if !p.Alive || p.Life != 1 || p.Health != 100 || p.Money != 800 {
				t.Fatalf("invalid original start for %s seat%d", id, p.Seat)
			}
			for _, item := range p.Inventory {
				if item.ID == "c4" {
					t.Fatal("elimination map gave C4")
				}
			}
			if PointContents(world.Hulls[0], Position(p)) == ContentsSolid {
				t.Fatalf("%s spawned inside solid", id)
			}
		}
		if teams["t"] != 5 || teams["ct"] != 5 || state.Bomb.Carrier != -1 || len(state.Hostages) != 0 || !state.Options.FriendlyFire {
			t.Fatalf("invalid 5v5 state for %s", id)
		}
	}
}

func TestEliminationClockDoesNotInventWinnerE2E(t *testing.T) {
	state := objectiveFixture("elimination")
	state.Bomb = Bomb{Carrier: -1, Defuser: -1}
	state.Phase = "freeze"
	state.PhaseEndsAt = 10
	state.Frame = 9
	objectiveFrames(state, 1)
	if state.Phase != "live" || state.PhaseEndsAt != 0 {
		t.Fatal("elimination retained an objective deadline")
	}
	objectiveFrames(state, 121*FPS)
	if state.Phase != "live" || state.Finished || state.TeamScores != [2]int{} {
		t.Fatal("time alone invented an elimination result")
	}
	state.Players[0].Alive = false
	objectiveFrames(state, 1)
	if state.LastWinner != "ct" || state.TeamScores != [2]int{0, 1} {
		t.Fatal("actual elimination did not end round")
	}
}

func TestSmallMapsOriginalWeaponAndBuyFactsE2E(t *testing.T) {
	ice := MapByID("fy_iceworld")
	blood := MapByID("cs_bloodstrike")
	if len(ice.WorldWeapons) != 24 || len(ice.BuyZones) != 1 || len(blood.WorldWeapons) != 0 || len(blood.BuyZones) != 0 {
		t.Fatal("BSP source facts were not preserved")
	}
	counts := map[string]int{}
	for _, spawn := range ice.WorldWeapons {
		if spawn.ID <= 0 || spawn.Count != 1 {
			t.Fatal("armoury identity or count changed")
		}
		counts[spawn.Weapon]++
	}
	for _, id := range []string{"mp5navy", "tmp", "p90", "mac10", "ak47", "sg552", "m4a1", "aug", "awp", "m3", "xm1014", "m249"} {
		if counts[id] != 2 {
			t.Fatalf("%s expected two original armoury entities", id)
		}
	}
	for _, id := range []string{"fy_iceworld", "cs_bloodstrike"} {
		state, err := New(Options{Map: id, MaxRounds: 2}, 10, 8)
		if err != nil {
			t.Fatal(err)
		}
		for seat := range state.Players {
			if CanBuy(state, seat) != (id == "cs_bloodstrike") {
				t.Fatalf("%s spawn purchase eligibility changed", id)
			}
		}
		if id == "cs_bloodstrike" {
			money := state.Players[1].Money
			if Buy(state, 1, "defuser") == nil || state.Players[1].Money != money {
				t.Fatal("elimination sold a defuser without a bomb objective")
			}
		}
	}
}

func TestEliminationBotsUsePublicReachableGoalsE2E(t *testing.T) {
	state := objectiveFixture("elimination")
	state.World.Bombsites = nil
	state.World.Hostages = nil
	state.World.SpawnsT = []Spawn{{X: -200 * Unit, Z: 36 * Unit}}
	state.World.SpawnsCT = []Spawn{{X: 200 * Unit, Z: 36 * Unit}}
	state.World.Nav = []Vec3{{-200 * Unit, 0, 36 * Unit}, {0, 100 * Unit, 36 * Unit}, {200 * Unit, 0, 36 * Unit}, {5000 * Unit, 5000 * Unit, 36 * Unit}}
	state.World.NavEdges = [][]int{{1}, {0, 2}, {1}, nil}
	goal, _, ok := botDestination(state, 0)
	if !ok || goal.X != -200*Unit && goal.X != 200*Unit {
		t.Fatalf("missing public elimination patrol goal %v", goal)
	}
	before := goal
	state.Players[1].X = 9999 * Unit
	state.Players[1].Y = -9999 * Unit
	goal, _, ok = botDestination(state, 0)
	if !ok || goal != before {
		t.Fatal("hidden enemy movement changed static patrol target")
	}
	state.Frame = 1
	StepBotNavigation(state)
	if len(state.Players[0].BotPath) == 0 {
		t.Fatal("elimination did not create a reachable private path")
	}
	for _, node := range state.Players[0].BotPath {
		if node == 3 {
			t.Fatal("bot chose isolated NAV component")
		}
	}
}

func TestSmallMapsWorldWeaponsRefreshThroughRoundE2E(t *testing.T) {
	state, err := New(Options{Map: "fy_iceworld", RoundSeconds: 60, MaxRounds: 2}, 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Drops) != 24 {
		t.Fatal("original round did not place 24 armoury entities")
	}
	for _, drop := range state.Drops {
		if drop.SpawnID <= 0 || drop.Remaining != 1 {
			t.Fatal("map weapon identity missing")
		}
	}
	Advance(state, state.PhaseEndsAt+1)
	for seat := range state.Players {
		if state.Players[seat].Team == "t" {
			state.Players[seat].Alive = false
		}
	}
	Advance(state, state.Frame+1)
	if state.Phase != "round_end" {
		t.Fatal("elimination did not settle")
	}
	Advance(state, state.PhaseEndsAt)
	if state.Round != 2 || len(state.Drops) != 24 || state.Phase != "freeze" {
		t.Fatal("next round did not restore original armoury")
	}
}

func TestEliminationFinalApproachTouchesOriginalGunE2E(t *testing.T) {
	state := objectiveFixture("elimination")
	state.Bomb = Bomb{Carrier: -1, Defuser: -1}
	state.Frame = 1
	state.World.Nav = []Vec3{{-200 * Unit, 0, 36 * Unit}}
	state.World.NavEdges = [][]int{nil}
	state.World.WorldWeapons = []MapWeaponSpawn{{ID: 42, Item: 4, Weapon: "ak47", X: -72 * Unit, Count: 1}}
	InitInventory(&state.Players[0])
	InitMapWeapons(state)
	for range 100 {
		p := &state.Players[0]
		input := Input{Seq: p.ReceivedSeq + 1, Life: p.Life, ViewFrame: state.Frame, Yaw: p.Yaw}
		if goal, found := MapWeaponGoal(state, 0); found {
			input = botNavigate(state, 0, goal, false, input)
		}
		if err := QueueInput(state, 0, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, state.Frame+1)
	}
	if state.Players[0].Weapon != "ak47" || len(state.Drops) != 0 {
		t.Fatal("bot stayed at NAV point instead of walking the clear final pickup segment")
	}
}

func TestOriginalIceworldFinalPickupSegmentE2E(t *testing.T) {
	state, err := New(Options{Map: "fy_iceworld", MaxRounds: 2}, 2, 17)
	if err != nil {
		t.Fatal(err)
	}
	state.Phase = "live"
	state.Frame = 1000
	p := &state.Players[0]
	p.X, p.Y, p.Z, p.Grounded = -6074, -326721, -56312, true
	state.Drops = []Drop{{ID: 51, SpawnID: 51, Remaining: 1, X: 0, Y: -294912, Z: -65528, Weapon: WeaponState{ID: "tmp", Ammo: 30, Reserve: 60}}}
	for range 100 {
		p = &state.Players[0]
		input := Input{Seq: p.ReceivedSeq + 1, Life: p.Life, ViewFrame: state.Frame, Yaw: p.Yaw}
		if goal, found := MapWeaponGoal(state, 0); found {
			input = botNavigate(state, 0, goal, false, input)
		}
		if err := QueueInput(state, 0, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, state.Frame+1)
	}
	if state.Players[0].Weapon != "tmp" {
		t.Fatal("original clear gun segment was trapped at nearest NAV endpoint")
	}
}

func TestSmallMapsTenSeatNaturalCombatE2E(t *testing.T) {
	for _, id := range []string{"fy_iceworld", "cs_bloodstrike"} {
		t.Run(id, func(t *testing.T) {
			state, err := New(Options{Map: id, RoundSeconds: 60, MaxRounds: 2}, 10, 20261122)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			cursor := int64(0)
			for !state.Finished && state.Frame < 180*FPS {
				if state.Frame%3 == 0 {
					for seat := range state.Players {
						if item := BotPurchase(state, seat); item != "" {
							if err := Buy(state, seat, item); err != nil {
								t.Fatal(err)
							}
							continue
						}
						if err := QueueInput(state, seat, BotInput(state, seat)); err != nil {
							t.Fatal(err)
						}
					}
				}
				Advance(state, state.Frame+1)
				for _, event := range state.Events {
					if event.ID > cursor {
						counts[event.Kind]++
						cursor = event.ID
						if event.Kind == "round_end" && event.Weapon != "elimination" {
							t.Fatalf("%s invented round result %s", id, event.Weapon)
						}
					}
				}
			}
			if !state.Finished || state.Round != 2 || counts["shot"] < 10 || counts["kill"] < 2 || id == "fy_iceworld" && counts["pickup"] < 5 {
				directory := filepath.Join("..", "..", "..", ".local/cs16/verification/small-maps/checkpoints")
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(directory, id+".json"), raw, 0644); err != nil {
					t.Fatal(err)
				}
				t.Fatalf("%s natural combat incomplete frame=%d phase=%s events=%v", id, state.Frame, state.Phase, counts)
			}
			if counts["bomb_planted"] != 0 || counts["hostage_rescued"] != 0 {
				t.Fatal("elimination acquired nonexistent objective")
			}
			t.Logf("map=%s seats=10 frames=%d score=%v events=%v", id, state.Frame, state.TeamScores, counts)
		})
	}
}
