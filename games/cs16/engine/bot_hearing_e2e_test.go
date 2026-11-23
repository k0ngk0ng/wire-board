package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type botMemoryAudit struct {
	HeardPoint  Vec3  `json:"heardPoint"`
	HeardFrame  int64 `json:"heardFrame"`
	HeardUntil  int64 `json:"heardUntil"`
	SoundCursor int64 `json:"soundCursor"`
	EscapePoint Vec3  `json:"escapePoint"`
	EscapeUntil int64 `json:"escapeUntil"`
}

type botTrajectoryAudit struct {
	Frame int64 `json:"frame"`
	Point Vec3  `json:"point"`
	Yaw   int   `json:"yaw"`
	MoveX int   `json:"moveX"`
	MoveY int   `json:"moveY"`
}

func botHearingMemory(t *testing.T, s *State, seat int) botMemoryAudit {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Memories []botMemoryAudit `json:"botMemories"`
	}
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if seat >= len(decoded.Memories) {
		return botMemoryAudit{}
	}
	return decoded.Memories[seat]
}

func botHearingReport(t *testing.T, name string, value any) {
	t.Helper()
	path := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "bot-hearing")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, name+".json"), append(raw, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func botHearingFixture() *State {
	s := objectiveFixture("hostage")
	s.Frame = 100
	s.Bomb.Carrier = -1
	s.Hostages = []Hostage{{ID: 1, X: 20 * Unit, Z: 0, Health: 100, Following: -1}}
	s.Players[0].Yaw = 32768
	s.Players[1].X, s.Players[1].Y = 300*Unit, 100*Unit
	for seat := range s.Players {
		p := &s.Players[seat]
		p.Health, p.Weapon, p.Character = 10000, "glock18", "urban"
		p.Inventory = []WeaponState{{ID: "knife", Ammo: 1}, {ID: "glock18", Ammo: 20, Reserve: 40}}
		SyncWeapon(p)
	}
	s.World.Brushes = []Brush{{ID: 1, Kind: "func_wall", Health: 1, Bounds: Bounds{Min: Vec3{100 * Unit, -2000 * Unit, 0}, Max: Vec3{120 * Unit, 2000 * Unit, 256 * Unit}}}}
	s.Brushes = []BrushState{{ID: 1, Health: 1}}
	return s
}

func botHearingSubmit(t *testing.T, s *State, seat int, input Input, frames int64) {
	t.Helper()
	input.Seq, input.Life, input.ViewFrame = s.Players[seat].ReceivedSeq+1, s.Players[seat].Life, s.Frame
	if err := QueueInput(s, seat, input); err != nil {
		t.Fatal(err)
	}
	Advance(s, s.Frame+frames)
}

func TestBotGuardDoesNotRotateWithoutTargetE2E(t *testing.T) {
	s := botHearingFixture()
	trace := []botTrajectoryAudit{}
	turn := 0
	previous := s.Players[0].Yaw
	defer func() {
		botHearingReport(t, "guard", map[string]any{"trace": trace, "turn": turn, "passed": !t.Failed()})
	}()
	for range 100 {
		input := BotInput(s, 0)
		botHearingSubmit(t, s, 0, input, 3)
		p := s.Players[0]
		turn += Abs(botAngleDifference(p.Yaw, previous))
		previous = p.Yaw
		trace = append(trace, botTrajectoryAudit{Frame: s.Frame, Point: Position(p), Yaw: p.Yaw, MoveX: input.MoveX, MoveY: input.MoveY})
	}
	if turn > 32768 {
		t.Fatalf("unguarded spinning: %d angle units in 3 seconds", turn)
	}
}

func TestBotGuardCoincidentObjectiveHasStableHeadingE2E(t *testing.T) {
	s := botHearingFixture()
	s.Hostages[0].X = 0
	turn, previous := 0, s.Players[0].Yaw
	trace := []botTrajectoryAudit{}
	defer func() {
		botHearingReport(t, "guard-coincident", map[string]any{"trace": trace, "turn": turn, "passed": !t.Failed()})
	}()
	for range 100 {
		input := BotInput(s, 0)
		botHearingSubmit(t, s, 0, input, 3)
		p := s.Players[0]
		turn += Abs(botAngleDifference(p.Yaw, previous))
		previous = p.Yaw
		trace = append(trace, botTrajectoryAudit{Frame: s.Frame, Point: Position(p), Yaw: p.Yaw, MoveX: input.MoveX, MoveY: input.MoveY})
	}
	if turn > 32768 {
		t.Fatal("coincident guard heading was relative and kept accumulating", turn)
	}
}

func TestBotHearsActualRunningButNotSilentMovementE2E(t *testing.T) {
	results := []map[string]any{}
	defer func() { botHearingReport(t, "actual-footsteps", results) }()
	for _, mode := range []string{"run", "walk", "crouch", "stand"} {
		s := botHearingFixture()
		StepBotNavigation(s)
		for range 6 {
			input := Input{Yaw: 16384, MoveY: 1000, Walk: mode == "walk", Crouch: mode == "crouch"}
			if mode == "stand" {
				input.MoveY = 0
			}
			botHearingSubmit(t, s, 1, input, 10)
		}
		steps := 0
		for _, event := range s.Events {
			if event.Kind == "step" && event.Seat == 1 {
				steps++
			}
		}
		m := botHearingMemory(t, s, 0)
		input := BotInput(s, 0)
		results = append(results, map[string]any{"mode": mode, "steps": steps, "memory": m, "input": input})
		if mode == "run" {
			if steps == 0 || m.HeardFrame <= 100 || m.HeardUntil <= s.Frame {
				t.Errorf("running steps were not heard: %d %+v", steps, m)
			}
			if input.Fire {
				t.Error("heard footsteps caused blind fire through wall")
			}
			if input.MoveX == 0 && input.MoveY == 0 {
				t.Error("continuing footsteps restarted the stop-and-listen delay")
			}
		} else if steps != 0 || m.HeardFrame != 0 {
			t.Errorf("silent %s revealed a source: steps %d memory %+v", mode, steps, m)
		}
	}
}

func TestBotHearingSoundRangeBoundariesE2E(t *testing.T) {
	results := []map[string]any{}
	defer func() { botHearingReport(t, "sound-boundaries", results) }()
	for _, sound := range []struct {
		kind, weapon  string
		value, radius int
	}{{"step", "", 0, 1250}, {"land", "", 0, 512}, {"knife_air", "knife", 0, 1250}, {"knife_hit", "knife", 0, 1250}, {"knife_wall", "knife", 0, 1250}, {"shot", "ak47", 0, 2048}, {"shot", "usp", 1, 500}} {
		for _, axis := range []string{"x", "z"} {
			for _, adjacent := range []int{-1, 0, 1} {
				s := botHearingFixture()
				StepBotNavigation(s)
				point := Position(s.Players[0])
				if axis == "x" {
					point.X += sound.radius*Unit + adjacent
				} else {
					point.Z += sound.radius*Unit + adjacent
				}
				Emit(s, Event{Kind: sound.kind, Seat: 1, Weapon: sound.weapon, Value: sound.value, X: point.X, Y: point.Y, Z: point.Z, Life: 1})
				StepBotNavigation(s)
				m := botHearingMemory(t, s, 0)
				want := adjacent <= 0
				passed := (m.HeardFrame == s.Frame) == want
				results = append(results, map[string]any{"kind": sound.kind, "weapon": sound.weapon, "axis": axis, "adjacent": adjacent, "heard": m.HeardFrame == s.Frame, "expected": want, "passed": passed})
				if !passed {
					t.Errorf("%s/%s %s radius %d adjacent %d heard %t", sound.kind, sound.weapon, axis, sound.radius, adjacent, m.HeardFrame == s.Frame)
				}
				if want && m.HeardPoint != point {
					t.Error("audible event point lost its original height", m.HeardPoint, point)
				}
			}
		}
	}
}

func TestBotHearingRejectsIneligibleEventsE2E(t *testing.T) {
	for _, name := range []string{"self", "friendly", "dead", "old-life", "stale", "future", "purchase", "knife-shot", "c4-shot"} {
		s := botHearingFixture()
		StepBotNavigation(s)
		event := Event{Kind: "step", Seat: 1, X: 300 * Unit, Z: 36 * Unit, Life: 1}
		switch name {
		case "self":
			event.Seat = 0
		case "friendly":
			s.Players[1].Team = "t"
		case "dead":
			s.Players[1].Alive = false
		case "old-life":
			event.Life = 2
		case "purchase":
			event.Kind = "buy"
		case "knife-shot":
			event.Kind, event.Weapon = "shot", "knife"
		case "c4-shot":
			event.Kind, event.Weapon = "shot", "c4"
		}
		Emit(s, event)
		if name == "stale" {
			s.Events[len(s.Events)-1].Frame = s.Frame - 51
		}
		if name == "future" {
			s.Events[len(s.Events)-1].Frame = s.Frame + 1
		}
		StepBotNavigation(s)
		if m := botHearingMemory(t, s, 0); m.HeardFrame != 0 {
			t.Errorf("ineligible %s created memory %+v", name, m)
		}
	}
}

func TestBotHearingSnapshotExpiryRestoreAndDeathE2E(t *testing.T) {
	s := botHearingFixture()
	StepBotNavigation(s)
	point := Vec3{300 * Unit, 100 * Unit, 36 * Unit}
	Emit(s, Event{Kind: "step", Seat: 1, X: point.X, Y: point.Y, Z: point.Z, Life: 1})
	StepBotNavigation(s)
	first := botHearingMemory(t, s, 0)
	if first.HeardFrame != 100 {
		t.Fatal("fixture did not hear source", first)
	}
	before, _ := json.Marshal(s)
	input := BotInput(s, 0)
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("BotInput mutated authoritative memory")
	}
	s.Players[1].X, s.Players[1].Y = 700*Unit, -200*Unit
	StepBotNavigation(s)
	if botHearingMemory(t, s, 0).HeardPoint != point || BotInput(s, 0) != input {
		t.Fatal("silent hidden movement changed heard snapshot or input")
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	restored.World = s.World
	restored.Frame += 10
	StepBotNavigation(&restored)
	m := botHearingMemory(t, &restored, 0)
	if m.HeardFrame != first.HeardFrame || m.HeardUntil != first.HeardUntil || m.SoundCursor != s.EventID {
		t.Fatal("restore replayed or lost sound", first, m)
	}
	restored.Frame = first.HeardUntil
	StepBotNavigation(&restored)
	if m = botHearingMemory(t, &restored, 0); m.HeardUntil > restored.Frame {
		t.Fatal("expired event renewed sound memory", m)
	}
	for _, cause := range []string{"listener-death", "source-death", "round", "life", "source-life"} {
		var current State
		if err := json.Unmarshal(raw, &current); err != nil {
			t.Fatal(err)
		}
		current.World = s.World
		current.Frame += 10
		switch cause {
		case "listener-death":
			current.Players[0].Alive = false
		case "source-death":
			current.Players[1].Alive = false
		case "round":
			current.Round++
		case "life":
			current.Players[0].Life++
		case "source-life":
			current.Players[1].Life++
		}
		StepBotNavigation(&current)
		if m := botHearingMemory(t, &current, 0); m.HeardUntil > current.Frame {
			t.Errorf("%s retained old memory %+v", cause, m)
		}
	}
	var legacy map[string]json.RawMessage
	if err := json.Unmarshal(raw, &legacy); err != nil {
		t.Fatal(err)
	}
	delete(legacy, "botMemories")
	legacyRaw, _ := json.Marshal(legacy)
	var oldState State
	if err := json.Unmarshal(legacyRaw, &oldState); err != nil {
		t.Fatal(err)
	}
	oldState.World, oldState.Frame = s.World, 200
	StepBotNavigation(&oldState)
	if m := botHearingMemory(t, &oldState, 0); m.HeardFrame != 0 {
		t.Fatal("legacy state replayed stale sound", m)
	}
	botHearingReport(t, "memory-lifecycle", map[string]any{"initial": first, "restored": m, "originalPoint": point, "pureInput": true})
}

func TestBotVisibleReactionKeepsTargetWithoutInstantFireE2E(t *testing.T) {
	s := objectiveFixture("bomb")
	s.Frame = 100
	s.Bomb.Carrier = -1
	s.Players[0].Yaw = 0
	s.World.Bombsites = []Bounds{{Min: Vec3{-1000 * Unit, -10 * Unit, 0}, Max: Vec3{-900 * Unit, 10 * Unit, 72 * Unit}}}
	for seat := range s.Players {
		p := &s.Players[seat]
		p.Weapon = "glock18"
		p.Inventory = []WeaponState{{ID: "glock18", Ammo: 20, Reserve: 40}}
		SyncWeapon(p)
	}
	StepBotNavigation(s)
	input := BotInput(s, 0)
	if input.Fire {
		t.Fatal("visual acquisition fired before reaction time")
	}
	if Abs(botAngleDifference(input.Yaw, 0)) > 4096 {
		t.Fatal("reaction wait turned away to navigation goal", input)
	}
}

func TestBotBlockedRouteUsesDeterministicRetreatE2E(t *testing.T) {
	s := objectiveFixture("bomb")
	s.Frame = 100
	s.Bomb.Carrier = -1
	s.Players[0].Yaw = 0
	s.Players[1].X = 9000 * Unit
	s.World.Bombsites = []Bounds{{Min: Vec3{380 * Unit, -20 * Unit, 0}, Max: Vec3{420 * Unit, 20 * Unit, 72 * Unit}}}
	s.World.Brushes = []Brush{{ID: 1, Kind: "func_wall", Health: 1, Bounds: Bounds{Min: Vec3{60 * Unit, -500 * Unit, 0}, Max: Vec3{80 * Unit, 500 * Unit, 128 * Unit}}}, {ID: 2, Kind: "func_wall", Health: 1, Bounds: Bounds{Min: Vec3{-500 * Unit, -60 * Unit, 0}, Max: Vec3{60 * Unit, -32 * Unit, 128 * Unit}}}, {ID: 3, Kind: "func_wall", Health: 1, Bounds: Bounds{Min: Vec3{-500 * Unit, 32 * Unit, 0}, Max: Vec3{60 * Unit, 60 * Unit, 128 * Unit}}}}
	s.Brushes = []BrushState{{ID: 1, Health: 1}, {ID: 2, Health: 1}, {ID: 3, Health: 1}}
	trace := []botTrajectoryAudit{}
	escaped := false
	defer func() {
		botHearingReport(t, "blocked-route", map[string]any{"trace": trace, "retreated": escaped, "passed": !t.Failed()})
	}()
	for range 150 {
		input := BotInput(s, 0)
		botHearingSubmit(t, s, 0, input, 3)
		p := s.Players[0]
		if p.X < -32*Unit {
			escaped = true
		}
		trace = append(trace, botTrajectoryAudit{Frame: s.Frame, Point: Position(p), Yaw: p.Yaw, MoveX: input.MoveX, MoveY: input.MoveY})
	}
	if !escaped {
		t.Fatal("blocked route never retreated through the available passage")
	}
}

func TestBotHearingOriginalMapsTenSeatTrajectoriesE2E(t *testing.T) {
	for _, mapID := range []string{"de_dust2", "de_inferno", "de_aztec", "cs_assault", "fy_iceworld", "cs_bloodstrike"} {
		t.Run(mapID, func(t *testing.T) {
			s, err := New(Options{Map: mapID, RoundSeconds: 90, MaxRounds: 2}, 10, 20261123)
			if err != nil {
				t.Fatal(err)
			}
			traces := make([][]botTrajectoryAudit, 10)
			counts := map[string]int{}
			cursor := int64(0)
			defer func() {
				botHearingReport(t, "original-"+mapID, map[string]any{"traces": traces, "events": counts, "frames": s.Frame, "passed": !t.Failed()})
			}()
			for s.Frame < 1200 && !s.Finished {
				for seat := range s.Players {
					if item := BotPurchase(s, seat); item != "" {
						if err := Buy(s, seat, item); err != nil {
							t.Fatal(err)
						}
						continue
					}
					if !s.Players[seat].Alive {
						continue
					}
					if err := QueueInput(s, seat, BotInput(s, seat)); err != nil {
						t.Fatal(err)
					}
				}
				Advance(s, s.Frame+3)
				for _, event := range s.Events {
					if event.ID > cursor {
						counts[event.Kind]++
						cursor = event.ID
					}
				}
				if s.Frame%30 != 0 || s.Phase != "live" {
					continue
				}
				for seat, p := range s.Players {
					if !p.Alive {
						continue
					}
					traces[seat] = append(traces[seat], botTrajectoryAudit{Frame: s.Frame, Point: Position(p), Yaw: p.Yaw, MoveX: p.Input.MoveX, MoveY: p.Input.MoveY})
				}
			}
			if counts["step"] == 0 {
				t.Error("original map did not exercise real footsteps")
			}
			for seat, trace := range traces {
				for start := 0; start+13 < len(trace); start++ {
					end := start + 13
					if trace[end].Frame-trace[start].Frame > 420 {
						continue
					}
					minX, maxX, minY, maxY := trace[start].Point.X, trace[start].Point.X, trace[start].Point.Y, trace[start].Point.Y
					turn := 0
					for i := start + 1; i <= end; i++ {
						minX, maxX = min(minX, trace[i].Point.X), max(maxX, trace[i].Point.X)
						minY, maxY = min(minY, trace[i].Point.Y), max(maxY, trace[i].Point.Y)
						turn += Abs(botAngleDifference(trace[i].Yaw, trace[i-1].Yaw))
					}
					if maxX-minX < 32*Unit && maxY-minY < 32*Unit && turn > 65536 {
						t.Errorf("seat %d rotated %d within a stationary 4s window at frame %d", seat, turn, trace[start].Frame)
						break
					}
				}
			}
		})
	}
}
