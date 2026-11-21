package engine

import (
	"encoding/json"
	"testing"
)

func objectiveFixture(mode string) *State {
	floor := Hull{Root: 0, Nodes: []Node{{Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}}
	world := Map{ID: "fixture", Mode: mode, Planes: []Plane{{Z: NormalUnit}, {Z: NormalUnit, Dist: 36 * Unit}}, Nodes: []Node{{Plane: 0, Front: ContentsEmpty, Back: ContentsSolid}}, Hulls: []Hull{floor, floor}, Root: 0, SpawnsT: []Spawn{{X: -200 * Unit, Z: 36 * Unit}}, SpawnsCT: []Spawn{{X: 200 * Unit, Z: 36 * Unit}}, Bombsites: []Bounds{{Min: Vec3{-100 * Unit, -100 * Unit, -100 * Unit}, Max: Vec3{100 * Unit, 100 * Unit, 100 * Unit}}}, RescueZones: []Bounds{{Min: Vec3{180 * Unit, -100 * Unit, -100 * Unit}, Max: Vec3{300 * Unit, 100 * Unit, 100 * Unit}}}, Hostages: []Spawn{{X: 20 * Unit, Z: 36 * Unit}}}
	return &State{Options: Options{Map: "fixture", RoundSeconds: 120, MaxRounds: 2}, World: world, MapID: "fixture", Round: 1, Phase: "live", PhaseEndsAt: 120 * FPS, Players: []Player{{Seat: 0, Team: "t", Alive: true, Life: 1, Health: 100, Grounded: true, Weapon: "c4", Z: 36 * Unit, Money: 800}, {Seat: 1, Team: "ct", Alive: true, Life: 1, Health: 100, Grounded: true, X: 200 * Unit, Z: 36 * Unit, Money: 800}}, Bomb: Bomb{Carrier: 0, Defuser: -1}, Seed: 1}
}

func objectiveFrames(state *State, count int64) {
	for range count {
		state.Frame++
		StepObjectives(state)
	}
}

func TestBombPlantInterruptAndDefuseE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Players[0].Input.Fire = true
	objectiveFrames(state, 150)
	if state.Bomb.Planted {
		t.Fatal("early plant")
	}
	state.Players[0].Input.Fire = false
	objectiveFrames(state, 1)
	state.Players[0].Input.Fire = true
	objectiveFrames(state, 299)
	if state.Bomb.Planted {
		t.Fatal("interrupted progress retained")
	}
	objectiveFrames(state, 2)
	if !state.Bomb.Planted || state.Bomb.Carrier != -1 || state.Bomb.ExplodesAt-state.Frame < 3400 {
		t.Fatalf("invalid plant %+v", state.Bomb)
	}
	ct := &state.Players[1]
	ct.X, ct.Y, ct.Z, ct.Defuser = state.Bomb.X, state.Bomb.Y, state.Bomb.Z+36*Unit, true
	ct.Input.Use = true
	objectiveFrames(state, 499)
	if state.Phase != "live" {
		t.Fatal("early defuse")
	}
	objectiveFrames(state, 2)
	if state.Phase != "round_end" || state.LastWinner != "ct" || state.TeamScores != [2]int{0, 1} {
		t.Fatalf("defuse failed %s %s %v", state.Phase, state.LastWinner, state.TeamScores)
	}
}

func TestBombDeadlineAndEliminationPriorityE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Bomb = Bomb{Carrier: -1, Planted: true, ExplodesAt: 400, Defuser: -1}
	state.Players[0].Alive = false
	objectiveFrames(state, 1)
	if state.Phase != "live" {
		t.Fatal("T elimination erased planted bomb")
	}
	state.Frame = 399
	objectiveFrames(state, 1)
	if state.Phase != "round_end" || state.LastWinner != "t" || state.TeamScores[0] != 1 {
		t.Fatalf("deadline failed %+v", state)
	}
}

func TestRoundRewardsAndNextLifeE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Players[0].Alive = false
	state.Players[0].Pending = []Input{{Seq: 1, Life: 1, Fire: true}}
	RoundWon(state, "ct", "elimination")
	money := state.Players[1].Money
	RoundWon(state, "ct", "elimination")
	if state.TeamScores[1] != 1 || state.Players[1].Money != money {
		t.Fatal("duplicate settlement")
	}
	state.Frame = state.PhaseEndsAt
	StepObjectives(state)
	if state.Round != 2 || state.Phase != "freeze" || state.Players[0].Life != 2 || !state.Players[0].Alive || len(state.Players[0].Pending) != 0 {
		t.Fatal("round did not restart cleanly")
	}
	state.Frame = state.PhaseEndsAt
	StepObjectives(state)
	RoundWon(state, "t", "elimination")
	state.Frame = state.PhaseEndsAt
	StepObjectives(state)
	if !state.Finished || state.Phase != "finished" || state.TeamScores != [2]int{1, 1} {
		t.Fatal("match did not finish with tie")
	}
}

func TestHostageCallFollowAndRescueE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.Hostages = []Hostage{{ID: 1, X: 20 * Unit, Z: 36 * Unit, Health: 100, Following: -1}}
	ct := &state.Players[1]
	ct.X = 40 * Unit
	ct.Yaw = 32768
	ct.Input.Use = true
	objectiveFrames(state, 1)
	if state.Hostages[0].Following != 1 {
		t.Fatal("CT could not call hostage")
	}
	ct.Previous.Use = true
	ct.Input.Use = false
	ct.X = 260 * Unit
	objectiveFrames(state, 200)
	if !state.Hostages[0].Rescued || state.Phase != "round_end" || state.LastWinner != "ct" {
		t.Fatalf("hostage did not reach rescue %+v", state.Hostages)
	}
}

func TestObjectiveTimeoutsE2E(t *testing.T) {
	for _, mode := range []string{"bomb", "hostage"} {
		state := objectiveFixture(mode)
		state.Frame = state.PhaseEndsAt
		StepObjectives(state)
		want := "ct"
		if mode == "hostage" {
			want = "t"
		}
		if state.LastWinner != want || state.Phase != "round_end" {
			t.Fatalf("%s timeout awarded %s", mode, state.LastWinner)
		}
	}
}

func TestRoundEconomySourceRewardsE2E(t *testing.T) {
	cases := []struct {
		mode, reason, team string
		hostages, rescued  int
		planted            bool
		reward             int
	}{
		{"bomb", "elimination", "t", 0, 0, false, 3250},
		{"bomb", "bomb_exploded", "t", 0, 0, true, 3500},
		{"bomb", "bomb_defused", "ct", 0, 0, true, 3250},
		{"bomb", "time", "ct", 0, 0, false, 3250},
		{"hostage", "elimination", "t", 4, 0, false, 3600},
		{"hostage", "time", "t", 4, 0, false, 3850},
		{"hostage", "hostages_rescued", "ct", 4, 4, false, 5500},
	}
	for _, sample := range cases {
		state := objectiveFixture(sample.mode)
		state.Bomb.Planted = sample.planted
		state.Hostages = make([]Hostage, sample.hostages)
		for i := range state.Hostages {
			state.Hostages[i] = Hostage{ID: i + 1, Health: 100, Rescued: i < sample.rescued, Following: -1}
		}
		RoundWon(state, sample.team, sample.reason)
		winner := state.Players[teamIndex(sample.team)].Money
		if winner != 800+sample.reward {
			t.Fatalf("%s/%s reward=%d want=%d", sample.mode, sample.reason, winner-800, sample.reward)
		}
	}
	state := objectiveFixture("bomb")
	RoundWon(state, "ct", "elimination")
	state.Phase = "live"
	RoundWon(state, "ct", "elimination")
	state.Phase = "live"
	before := state.Players[1].Money
	RoundWon(state, "t", "elimination")
	if state.Players[1].Money-before != 1500 || state.LoserBonusBase != 1500 {
		t.Fatal("breaking losing streak did not reset shared base to 1500")
	}
}

func TestHostageRepeatedUseAndDeadRemainderE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.Hostages = []Hostage{{ID: 1, X: 20 * Unit, Z: 36 * Unit, Health: 100, Following: -1}}
	state.Players[1].X = 40 * Unit
	state.Players[1].Yaw = 32768
	state.Players[1].Input.Use = true
	objectiveFrames(state, 1)
	firstMoney := state.Players[1].Money
	state.Players[1].Previous.Use = false
	objectiveFrames(state, 1)
	state.Players[1].Previous.Use = false
	objectiveFrames(state, 1)
	if state.Players[1].Money != firstMoney {
		t.Fatal("hostage use repeated reward")
	}
	state.Hostages = []Hostage{{ID: 1, Rescued: true}, {ID: 2, Rescued: true}, {ID: 3, Health: 0}, {ID: 4, Health: 0}}
	objectiveFrames(state, 1)
	if state.LastWinner != "ct" {
		t.Fatal("rescued half with dead remainder did not finish")
	}
	other := objectiveFixture("hostage")
	other.Hostages = []Hostage{{ID: 1, Health: 0}, {ID: 2, Health: 0}}
	objectiveFrames(other, 1)
	if other.Phase != "live" {
		t.Fatal("killing hostages awarded CT win")
	}
}

func TestUseDoesNotStopHostageBehindPlayerE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.Hostages = []Hostage{{ID: 1, X: 20 * Unit, Z: 0, Health: 100, Following: 1, Touched: true}}
	state.Players[1].X = 40 * Unit
	state.Players[1].Yaw = 0
	state.Players[1].Input.Use = true
	objectiveFrames(state, 1)
	if state.Hostages[0].Following != 1 {
		t.Fatal("forward use action toggled hostage behind player")
	}
}

func TestBotRescueDestinationIsReachableAndInsideE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World = MapByID("cs_assault")
	state.Players[1].X, state.Players[1].Y, state.Players[1].Z = -188416, 696320, 95240
	state.Hostages = []Hostage{{ID: 3, X: -188416, Y: 696320, Z: 86016, Health: 100, Following: 1, Touched: true}}
	goal, mode, valid := botDestination(state, 1)
	if !valid || mode != 2 {
		t.Fatal("escort destination missing")
	}
	path, _ := botBuildPath(&state.World, Position(state.Players[1]), goal, mode)
	if len(path) == 0 {
		t.Fatal("rescue route missing")
	}
	end := state.World.Nav[path[len(path)-1]]
	inside := false
	for _, zone := range state.World.RescueZones {
		point := Vec3{end.X, end.Y, end.Z - 36*Unit}
		if objectiveInBounds(point, zone) && point.X >= zone.Min.X+64*Unit && point.X <= zone.Max.X-64*Unit && point.Y >= zone.Min.Y+64*Unit && point.Y <= zone.Max.Y-64*Unit {
			inside = true
		}
	}
	if !inside {
		t.Fatalf("NPC reachable endpoint lies outside safe rescue interior goal=%v end=%v", goal, end)
	}
}

func TestLegacyHostageRescueSphereBoundaryE2E(t *testing.T) {
	for _, sample := range []struct {
		x       int
		rescued bool
	}{{256*Unit - 1, true}, {256 * Unit, false}, {256*Unit + 1, false}} {
		state := objectiveFixture("hostage")
		state.World.RescueLegacy = true
		state.World.SpawnsCT = []Spawn{{}}
		state.World.RescueZones = []Bounds{{Min: Vec3{-256 * Unit, -256 * Unit, -256 * Unit}, Max: Vec3{256 * Unit, 256 * Unit, 256 * Unit}}}
		state.Hostages = []Hostage{{ID: 1, X: sample.x, Health: 100, Following: 1, Touched: true}}
		state.Players[1].X = sample.x
		objectiveFrames(state, 1)
		if state.Hostages[0].Rescued != sample.rescued {
			t.Fatalf("legacy rescue boundary x=%d rescued=%t", sample.x, state.Hostages[0].Rescued)
		}
	}
}

func TestBotUsesVisibleOpponentsE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Players[0].Weapon = "glock18"
	state.Players[0].Inventory = []WeaponState{{ID: "knife", Ammo: 1}, {ID: "glock18", Ammo: 20, Reserve: 40}}
	state.Players[0].Ammo = 20
	state.Bomb.Carrier = -1
	state.Players[0].X = -100 * Unit
	state.Players[1].X = 200 * Unit
	state.World.Planes = []Plane{{X: NormalUnit, Dist: 0}, {X: NormalUnit, Dist: 100 * Unit}}
	state.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	state.World.Root = 0
	input := BotInput(state, 0)
	if input.Fire {
		t.Fatal("bot fired through opaque wall")
	}
	state.World.Root = ContentsEmpty
	state.World.Nodes = nil
	state.Frame += FPS
	fired := false
	for range FPS {
		input = BotInput(state, 0)
		if input.Fire {
			fired = true
			break
		}
		state.Frame++
	}
	if !fired || input.Life != state.Players[0].Life || input.Seq <= state.Players[0].ReceivedSeq {
		t.Fatalf("bot failed visible engagement %+v", input)
	}
}

func TestBotSmokeOcclusionAndExpiryE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Frame = 200
	state.Bomb.Carrier = -1
	state.Players[0].X = -300 * Unit
	state.Players[1].X = 300 * Unit
	state.Players[0].Weapon = "ak47"
	state.Players[0].Ammo = 30
	state.Players[0].Inventory = []WeaponState{{ID: "knife", Ammo: 1}, {ID: "ak47", Ammo: 30, Reserve: 90}}
	state.Grenades = []Grenade{{ID: 1, Kind: "smokegrenade", Active: true, ExplodesAt: 100, ExpiresAt: 1900}}
	input := BotInput(state, 0)
	if input.Fire {
		t.Fatal("bot acquired target through active smoke")
	}
	state.Frame = 1900
	input = BotInput(state, 0)
	if !input.Fire {
		t.Fatal("bot did not reacquire after smoke expired")
	}
}

func TestBotObjectiveKnowledgeDoesNotReadHiddenCoordinatesE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Frame = 200
	state.Players[1].X = -300 * Unit
	state.World.Planes = []Plane{{X: NormalUnit}, {X: NormalUnit, Dist: 100 * Unit}}
	state.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	state.World.Root = 0
	state.Bomb = Bomb{Planted: true, Carrier: -1, Defuser: -1, X: 300 * Unit, Y: 100 * Unit, ExplodesAt: 4500}
	first, _, _ := botDestination(state, 1)
	state.Bomb.X, state.Bomb.Y = 700*Unit, -100*Unit
	second, _, _ := botDestination(state, 1)
	if first != second {
		t.Fatalf("hidden bomb changed CT search target %v -> %v", first, second)
	}
	state.World.Mode = "hostage"
	state.World.Hostages = []Spawn{{X: 200 * Unit, Z: 0}}
	state.Hostages = []Hostage{{ID: 1, X: 300 * Unit, Z: 0, Health: 100, Following: 1}}
	state.Players[0].X = -300 * Unit
	first, _ = botHostageDestination(state, &state.Players[0])
	state.Hostages[0].X = 700 * Unit
	second, _ = botHostageDestination(state, &state.Players[0])
	if first != second || first.X != 200*Unit {
		t.Fatalf("T chased unseen rescued-hostage movement %v -> %v", first, second)
	}
}

func TestBombCannotBeDefusedThroughWallE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.World.Planes = []Plane{{X: NormalUnit}, {X: NormalUnit, Dist: 100 * Unit}}
	state.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	state.World.Root = 0
	state.Bomb = Bomb{Planted: true, Carrier: -1, Defuser: -1, X: 50 * Unit, ExplodesAt: 4500}
	state.Players[1].X = -5 * Unit
	state.Players[1].Input.Use = true
	objectiveFrames(state, 1100)
	if state.Phase != "live" || state.Bomb.Defuser >= 0 {
		t.Fatal("use action defused an occluded bomb through wall")
	}
}

func TestBotFriendlyBlockerAndPureDecisionE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Bomb.Carrier = -1
	state.Players[0].X = -200 * Unit
	state.Players[1].Alive = false
	state.Players = append(state.Players, Player{Seat: 2, Team: "t", Alive: true, Life: 1, Health: 100, X: -172 * Unit, Z: 36 * Unit})
	before, _ := json.Marshal(state)
	input := BotInput(state, 0)
	after, _ := json.Marshal(state)
	if string(before) != string(after) {
		t.Fatal("bot decision mutated authoritative state")
	}
	if input.MoveX == 0 || input.MoveY > 0 {
		t.Fatalf("bot continued into friendly blocker %+v", input)
	}
}

func TestBotDecisionBucketAtServerCadenceE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Frame = 12
	state.Bomb.Carrier = -1
	p := &state.Players[0]
	p.X = -100 * Unit
	p.Weapon = "glock18"
	p.Inventory = []WeaponState{{ID: "knife", Ammo: 1}, {ID: "glock18", Ammo: 20, Reserve: 40}}
	p.Ammo = 20
	p.ReceivedSeq, p.AppliedSeq = 1, 1
	p.Input = Input{Seq: 1, Life: 1, ViewFrame: 9, QueuedFrame: 9, Fire: true, Yaw: 0, Select: "glock18"}
	state.World.Planes = []Plane{{X: NormalUnit}, {X: NormalUnit, Dist: 100 * Unit}}
	state.World.Nodes = []Node{{Plane: 0, Front: 1, Back: ContentsEmpty}, {Plane: 1, Front: ContentsEmpty, Back: ContentsSolid}}
	state.World.Root = 0
	input := BotInput(state, 0)
	if input.Fire {
		t.Fatal("33Hz sample crossing decision boundary reused stale attack")
	}
}

func TestBotLeavesCrouchedNavigationE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Planes = append(state.World.Planes, Plane{Z: NormalUnit, Dist: 18 * Unit})
	state.World.Hulls[1] = Hull{Root: 0, Nodes: []Node{{Plane: 2, Front: ContentsEmpty, Back: ContentsSolid}}}
	state.World.Nav = []Vec3{{0, 0, 18 * Unit}, {50 * Unit, 0, 18 * Unit}, {100 * Unit, 0, 36 * Unit}}
	state.World.NavFlags = []int{botCrouch, botCrouch, 0}
	state.World.NavEdges = [][]int{{1}, {0, 2}, {1}}
	state.Players[1].X, state.Players[1].Z, state.Players[1].Crouch = 0, 18*Unit, true
	input := botNavigate(state, 1, Vec3{100 * Unit, 0, 36 * Unit}, true, Input{})
	if input.Crouch || input.MoveY <= 0 {
		t.Fatalf("bot retained unnecessary crouch %+v", input)
	}
}

func TestBotPlansAcrossClosedDoorE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Planes = append(state.World.Planes, Plane{Z: NormalUnit, Dist: 18 * Unit})
	state.World.Hulls[1] = Hull{Root: 0, Nodes: []Node{{Plane: 2, Front: ContentsEmpty, Back: ContentsSolid}}}
	state.World.Nav = []Vec3{{-100 * Unit, 0, 36 * Unit}, {100 * Unit, 0, 36 * Unit}, {-200 * Unit, 0, 18 * Unit}}
	state.World.NavFlags = []int{0, 0, botCrouch}
	state.World.NavEdges = [][]int{{1, 2}, {0}, {0}}
	state.World.Brushes = []Brush{{ID: 1, Kind: "func_door", Bounds: Bounds{Min: Vec3{-4 * Unit, -40 * Unit, 0}, Max: Vec3{4 * Unit, 40 * Unit, 96 * Unit}}, Health: 1}}
	state.Brushes = []BrushState{{ID: 1, Health: 1}}
	state.Players[1].X, state.Players[1].Z, state.Players[1].Crouch = -100*Unit, 18*Unit, true
	waypoint, _, _ := botWaypointWithRules(state, Position(state.Players[1]), state.World.Nav[1], false, true)
	if waypoint.X <= 0 {
		t.Fatalf("closed door planner selected stationary start %v", waypoint)
	}
	if state.Brushes[0].Open {
		t.Fatal("planner opened door without a normal input")
	}
}

func TestBotUsesAirDuckOnJumpConnectionE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Nav = []Vec3{{0, 0, 36 * Unit}, {100 * Unit, 0, 81 * Unit}}
	state.World.NavFlags = []int{0, botCrouch}
	state.World.NavEdges = [][]int{{1}, {0}}
	state.World.NavConnections = [][]int{{botJump}, {0}}
	state.Players[1].X = 0
	first := botNavigate(state, 1, state.World.Nav[1], false, Input{})
	if !first.Jump || first.Crouch {
		t.Fatalf("jump connection crouched before launch %+v", first)
	}
	state.Frame = 1
	state.Players[1].Grounded = false
	state.Players[1].Z = 40 * Unit
	second := botNavigate(state, 1, state.World.Nav[1], false, Input{})
	if !second.Crouch {
		t.Fatalf("jump connection did not shrink hull in air %+v", second)
	}
}

func TestHostageRouteAvoidsJumpShortcutE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Nav = []Vec3{{0, 0, 36 * Unit}, {100 * Unit, 0, 81 * Unit}, {0, 160 * Unit, 36 * Unit}, {200 * Unit, 0, 36 * Unit}}
	state.World.NavFlags = []int{0, botCrouch, 0, 0}
	state.World.NavEdges = [][]int{{1, 2}, {3}, {3}, {}}
	state.World.NavConnections = [][]int{{botJump, 0}, {0}, {0}, {}}
	state.World.Brushes = []Brush{{ID: 1, Kind: "func_wall", Bounds: Bounds{Min: Vec3{80 * Unit, -40 * Unit, 0}, Max: Vec3{120 * Unit, 40 * Unit, 96 * Unit}}}}
	waypoint, flags, connection := botWaypointWithRules(state, Vec3{0, 0, 36 * Unit}, state.World.Nav[3], true, true)
	if flags&botCrouch != 0 || connection&botJump != 0 || waypoint.Y <= 0 {
		t.Fatalf("escort planner chose jump shortcut point=%v flags=%d conn=%d", waypoint, flags, connection)
	}
}

func TestEscortKeepsForwardOnTraversableSlopeE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.Frame = 150
	state.World.Planes = []Plane{{X: -NormalUnit / 2, Z: NormalUnit, Dist: 36 * Unit}}
	state.World.Hulls = []Hull{{Root: 0, Nodes: []Node{{Plane: 0, Front: ContentsEmpty, Back: ContentsSolid}}}}
	state.World.Nav = []Vec3{{0, 0, 36 * Unit}, {100 * Unit, 0, 86 * Unit}}
	state.World.NavFlags = []int{0, 0}
	state.World.NavEdges = [][]int{{1}, {0}}
	state.Players[1].X, state.Players[1].Y, state.Players[1].Z = 0, 0, 36*Unit
	input := botNavigate(state, 1, state.World.Nav[1], true, Input{})
	if input.MoveX != 0 || input.Jump || input.MoveY <= 0 {
		t.Fatalf("escort mistook walkable rise for wall %+v", input)
	}
}

func TestAuthorityBotPathMemoryAndRestoreE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Frame = 1
	state.World.Nav = []Vec3{{-200 * Unit, 0, 36 * Unit}, {-100 * Unit, 0, 36 * Unit}, {0, 0, 36 * Unit}}
	state.World.NavFlags = []int{0, 0, 0}
	state.World.NavEdges = [][]int{{1}, {0, 2}, {1}}
	state.Players[1].X = -200 * Unit
	original := state.Players[1]
	StepBotNavigation(state)
	p := &state.Players[1]
	if len(p.BotPath) != 2 || p.BotPath[0] != 1 || p.BotTarget != 0 {
		t.Fatalf("wrong initial committed path %+v", p.BotPath)
	}
	if p.Input != original.Input || p.X != original.X || p.Y != original.Y || p.Z != original.Z || p.Money != original.Money || state.TeamScores != [2]int{} {
		t.Fatal("private route planning changed manual gameplay")
	}
	p.X = -112 * Unit
	state.Frame++
	StepBotNavigation(state)
	if len(p.BotPath) != 2 || p.BotPath[0] != 1 {
		t.Fatal("precise waypoint was accepted before entering safe radius")
	}
	p.X = -100 * Unit
	state.Frame++
	StepBotNavigation(state)
	if len(p.BotPath) != 1 || p.BotPath[0] != 2 || p.BotTarget != 1 {
		t.Fatalf("arrival did not advance private route %v", p.BotPath)
	}
	p.X = -110 * Unit
	state.Frame++
	StepBotNavigation(state)
	if len(p.BotPath) != 1 || p.BotPath[0] != 2 {
		t.Fatal("nearest-point boundary prepended an already reached waypoint")
	}
	before, _ := json.Marshal(state)
	var restored State
	if json.Unmarshal(before, &restored) != nil {
		t.Fatal("restore failed")
	}
	restored.World = state.World
	StepBotNavigation(&restored)
	StepBotNavigation(&restored)
	after, _ := json.Marshal(&restored)
	if string(before) != string(after) {
		t.Fatal("restored private path was not idempotent")
	}
	input := botNavigate(state, 1, p.BotGoal, false, Input{})
	if input.Yaw > 8192 && input.Yaw < 57344 {
		t.Fatalf("committed route steered back toward old waypoint %+v", input)
	}
}

func TestBotArrivalBrakesAndCancelsLateralVelocityE2E(t *testing.T) {
	state := objectiveFixture("bomb")
	state.Players[1].X, state.Players[1].Y = 0, 0
	state.Players[1].VX, state.Players[1].VY = 250*Unit, 150*Unit
	state.World.Nav = []Vec3{{0, 0, 36 * Unit}, {20 * Unit, 0, 36 * Unit}}
	state.World.NavEdges = [][]int{{1}, {0}}
	state.World.NavFlags = []int{0, 0}
	input := botNavigate(state, 1, state.World.Nav[1], false, Input{})
	if input.MoveY >= 0 || input.MoveX <= 0 {
		t.Fatalf("arrival did not brake and counter lateral inertia %+v", input)
	}
	if input.MoveX > 1000 || input.MoveY < -1000 {
		t.Fatal("steering escaped validated input range")
	}
}

func TestTenSeatOriginalMapBotMatchesE2E(t *testing.T) {
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault"} {
		t.Run(mapID, func(t *testing.T) {
			state, err := New(Options{Map: mapID, RoundSeconds: 60, MaxRounds: 2}, 10, 20261121)
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			cursor := int64(0)
			limit := int64(2*(3+60+45+5)*FPS + 500)
			for !state.Finished && state.Frame < limit {
				for seat := range state.Players {
					if item := BotPurchase(state, seat); item != "" {
						if err := Buy(state, seat, item); err != nil {
							t.Fatalf("bot buy %s: %v", item, err)
						}
						continue
					}
					input := BotInput(state, seat)
					if err := QueueInput(state, seat, input); err != nil {
						t.Fatalf("bot input seat %d: %v", seat, err)
					}
				}
				Advance(state, state.Frame+1)
				for _, event := range state.Events {
					if event.ID > cursor {
						counts[event.Kind]++
						cursor = event.ID
					}
				}
			}
			if !state.Finished || state.Round != 2 || state.Phase != "finished" {
				t.Fatalf("match stalled at frame %d phase %s", state.Frame, state.Phase)
			}
			if counts["shot"] < 10 || counts["kill"]+counts["bomb_planted"]+counts["hostage_rescued"] == 0 {
				t.Fatalf("bots finished without meaningful combat/objective: %v", counts)
			}
			t.Logf("map=%s seats=10 frames=%d score=%v events=%v", mapID, state.Frame, state.TeamScores, counts)
		})
	}
}

func TestOriginalAssaultHostageEscortRouteE2E(t *testing.T) {
	state, err := New(Options{Map: "cs_assault", RoundSeconds: 300, MaxRounds: 1}, 2, 20261121)
	if err != nil {
		t.Fatal(err)
	}
	rescued := 0
	for !state.Finished && state.Frame < 310*FPS {
		player := &state.Players[1]
		input := Input{Seq: player.ReceivedSeq + 1, Life: player.Life, ViewFrame: state.Frame, Yaw: player.Yaw, Pitch: player.Pitch}
		if state.Phase == "live" {
			goal := Vec3{}
			following := false
			waiting := false
			for _, hostage := range state.Hostages {
				if hostage.Following == 1 && !hostage.Rescued {
					following = true
					if botHorizontalDistance(objectivePosition(player), Vec3{hostage.X, hostage.Y, hostage.Z}) > 180*Unit {
						waiting = true
					}
				}
			}
			if following {
				goal = botRescueDestination(state, 1)
			} else {
				var distance int
				goal, distance = botHostageDestination(state, player)
				if distance < 75*Unit {
					input.Yaw = AngleTo(goal.X-player.X, goal.Y-player.Y)
					input.Pitch = 0
					input.Use = state.Frame%20 < 10
					waiting = true
				}
			}
			if !waiting {
				input = botNavigate(state, 1, goal, true, input)
				botAvoidPlayers(state, 1, &input)
			}
		}
		if err := QueueInput(state, 1, input); err != nil {
			t.Fatal(err)
		}
		Advance(state, state.Frame+1)
		rescued = 0
		for _, hostage := range state.Hostages {
			if hostage.Rescued {
				rescued++
			}
		}
	}
	if rescued != len(state.Hostages) || state.LastWinner != "ct" || !state.Finished {
		t.Fatalf("escort failed rescued=%d phase=%s winner=%s player=%+v hostages=%+v", rescued, state.Phase, state.LastWinner, state.Players[1], state.Hostages)
	}
	t.Logf("original assault four-hostage escort completed frame=%d", state.Frame)
}

func TestOriginalHostageDescendsRampE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World = MapByID("cs_assault")
	state.Players[1].X, state.Players[1].Y, state.Players[1].Z = 47199, 769313, 77824
	state.Hostages = []Hostage{{ID: 4, X: 97250, Y: 757913, Z: 86016, Health: 100, Following: 1, Touched: true}}
	before := Distance(Vec3{97250, 757913, 86016}, Vec3{47199, 769313, 77824 - 36*Unit})
	for range 500 {
		state.Frame++
		stepHostages(state)
	}
	hostage := state.Hostages[0]
	after := Distance(Vec3{hostage.X, hostage.Y, hostage.Z}, Vec3{state.Players[1].X, state.Players[1].Y, state.Players[1].Z - 36*Unit})
	if after >= before-64*Unit {
		t.Fatalf("original hostage could not descend ramp: before=%d after=%d hostage=%+v", before, after, hostage)
	}
}

func TestHostageFreeFallUsesNativeGravityE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Planes = nil
	state.World.Hulls = []Hull{{Root: ContentsEmpty}, {Root: ContentsEmpty}}
	state.Hostages = []Hostage{{ID: 1, Z: 1000 * Unit, Health: 100, Following: 1, Touched: true}}
	state.Players[1].X = 1000 * Unit
	for range 10 {
		state.Frame++
		stepHostages(state)
	}
	fallen := 1000*Unit - state.Hostages[0].Z
	if fallen < 4*Unit-10 || fallen > 4*Unit+10 {
		t.Fatalf("100ms free fall=%d fixed units; expected native 4U", fallen)
	}
}

func TestStoppedHostageContinuesFallingE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Planes = nil
	state.World.Hulls = []Hull{{Root: ContentsEmpty}, {Root: ContentsEmpty}}
	state.Hostages = []Hostage{{ID: 1, Z: 1000 * Unit, Health: 100, Following: 1, Touched: true}}
	state.Players[1].X = 1000 * Unit
	for range 5 {
		state.Frame++
		stepHostages(state)
	}
	state.Hostages[0].Following = -1
	for range 5 {
		state.Frame++
		stepHostages(state)
	}
	fallen := 1000*Unit - state.Hostages[0].Z
	if fallen < 4*Unit-10 || fallen > 4*Unit+10 {
		t.Fatalf("stopping follow interrupted native gravity: %d", fallen)
	}
}

func TestOriginalHostageWalksDropConnectionE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World = MapByID("cs_assault")
	start, target := state.World.Nav[263], state.World.Nav[117]
	state.Players[1].X, state.Players[1].Y, state.Players[1].Z = target.X, target.Y, target.Z
	state.Hostages = []Hostage{{ID: 1, X: start.X, Y: start.Y, Z: start.Z - 36*Unit, Health: 100, Following: 1, Touched: true}}
	for range 200 {
		state.Frame++
		stepHostages(state)
	}
	hostage := state.Hostages[0]
	if hostage.Y >= 2224*Unit || hostage.Z >= 64*Unit {
		t.Fatalf("NPC fell into warehouse wall before traversing authored drop edge: %+v", hostage)
	}
}

func TestHostageRouteRestoreAndManualIsolationE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World = MapByID("cs_assault")
	state.Players[1].X, state.Players[1].Y, state.Players[1].Z = 47199, 769313, 77824
	state.Hostages = []Hostage{{ID: 4, X: 97250, Y: 757913, Z: 86016, Health: 100, Following: 1, Touched: true}}
	original := state.Players[1]
	state.Frame = 1
	stepHostages(state)
	if len(state.HostageRoutes) != 1 || len(state.HostageRoutes[0].Path) == 0 {
		t.Fatal("following hostage did not retain private navigation route")
	}
	if state.Players[1].Input != original.Input || Position(state.Players[1]) != Position(original) || state.Players[1].Money != original.Money {
		t.Fatal("NPC navigation changed manual player")
	}
	raw, _ := json.Marshal(state)
	var restored State
	if json.Unmarshal(raw, &restored) != nil {
		t.Fatal("NPC restore failed")
	}
	restored.World = state.World
	for range 300 {
		state.Frame++
		restored.Frame++
		stepHostages(state)
		stepHostages(&restored)
	}
	a, _ := json.Marshal(state)
	b, _ := json.Marshal(&restored)
	if string(a) != string(b) {
		t.Fatal("NPC private route restore diverged")
	}
}

func TestMovingLeaderDoesNotReinsertDropWaypointE2E(t *testing.T) {
	state := objectiveFixture("hostage")
	state.World.Planes = nil
	state.World.Nav = []Vec3{{0, 0, 164 * Unit}, {100 * Unit, 0, 36 * Unit}}
	state.World.NavFlags = []int{0, 0}
	state.World.NavEdges = [][]int{{1}, {0}}
	state.Players[1].X, state.Players[1].Z = 100*Unit, 36*Unit
	state.Hostages = []Hostage{{ID: 1, X: 10 * Unit, Z: 80 * Unit, Health: 100, Following: 1, Touched: true}}
	state.HostageRoutes = []HostageRoute{{ID: 1, Following: 1, Goal: Vec3{-500 * Unit, 0, 36 * Unit}, Path: []int{1}, From: 0, PlannedAt: 1, VZ: -100 * Unit}}
	state.Frame = 20
	stepHostages(state)
	if len(state.HostageRoutes[0].Path) != 1 || state.HostageRoutes[0].Path[0] != 1 {
		t.Fatalf("moving leader replanned an unreachable upper point during drop: %+v", state.HostageRoutes[0])
	}
}

func TestTenSeatAssaultServerCadenceE2E(t *testing.T) {
	state, err := New(Options{Map: "cs_assault", RoundSeconds: 60, MaxRounds: 2}, 10, 20261121)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	cursor := int64(0)
	for !state.Finished && state.Frame < 140*FPS {
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
			}
		}
	}
	if !state.Finished || counts["shot"] < 10 || counts["kill"]+counts["hostage_rescued"] == 0 {
		t.Fatalf("33Hz bot match stalled or lacked combat: frame=%d events=%v", state.Frame, counts)
	}
	t.Logf("10-seat 33Hz assault frame=%d scores=%v events=%v", state.Frame, state.TeamScores, counts)
}
