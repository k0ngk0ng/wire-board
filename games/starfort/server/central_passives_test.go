package starfort

// Failures are enumerated in EVOLUTION.md before implementation. These isolated
// geometry/repair cases complement production Module and HTTP complete matches.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"reflect"
	"testing"
	"time"
)

func centralCase(t *testing.T, n int) *State {
	t.Helper()
	m := Module()
	o, e := m.Configure(n, json.RawMessage(`{"mode":"endless"}`))
	if e != nil {
		t.Fatal(e)
	}
	v, e := m.Start(gameapi.Context{Actor: 0, Players: n, Now: time.Unix(1900000000, 0)}, o)
	if e != nil {
		t.Fatal(e)
	}
	s, e := decode(v)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCentralMapFixedFromStart(t *testing.T) {
	for n := 1; n <= 4; n++ {
		s := centralCase(t, n)
		m := s.World.Map
		if s.World.Layout != "central" || len(m.Routes) != n || m.Base.X != float64(m.Width)/2 || m.Base.Y != float64(m.Height)/2 {
			t.Fatalf("players %d: not centered/fixed: %+v", n, m)
		}
		if err := validateMap(m, false); err != nil {
			t.Fatal(err)
		}
		for _, r := range m.Routes {
			if !r.Active || r.Points[len(r.Points)-1] != m.Base || pathDistance(r.Points) < 600 {
				t.Fatal("invalid lane", r)
			}
		}
		for _, p := range m.Pads {
			if math.Hypot(p.X-m.Base.X, p.Y-m.Base.Y) < 120 || roadDistance(m, Point{p.X, p.Y}) < 45 {
				t.Fatal("pad obstructs core/road", p)
			}
		}
		original := cloneMap(m)
		for wave := 1; wave <= 45; wave++ {
			s.Wave = wave
			s.CompletedWaves = wave - 1
			s.World.ShortcutWave = wave
			s.worldPrepare(s.ServerAt + int64(wave)*1000)
			s.World.BoonChoice = nil
			if s.World.Choice != nil || !reflect.DeepEqual(original, s.World.Map) {
				t.Fatalf("map changed at wave %d, players %d", wave, n)
			}
		}
	}
}
func craterCase(t *testing.T, workers int) (*State, int) {
	t.Helper()
	s := centralCase(t, 1)
	s.SkillRules = 0 // archived focus/ring skill cases
	s.Phase = "battle"
	s.ElapsedMS = 200
	s.Tick = 1
	route := s.World.Map.Routes[0]
	p := pathPosition(route.Points, 250)
	target := Enemy{ID: 1, Kind: "armored", Route: route.ID, Progress: 250, X: p.X, Y: p.Y, HP: 1e6, MaxHP: 1e6, Speed: 40, Leak: 1}
	tower := Tower{ID: 1, Kind: "cannon", Owner: 0, Level: 1}
	if !s.makeCrater(&tower, target) {
		t.Fatal("no crater")
	}
	s.Enemies = []Enemy{target}
	s.EnemySerial = workers + 1
	s.TowerSerial = 2
	for i := 0; i < workers; i++ {
		e := target
		e.ID = i + 2
		e.Progress = 245
		if !s.blockAtCrater(&e, 260) {
			t.Fatal("following enemy not blocked")
		}
		s.Enemies = append(s.Enemies, e)
	}
	if s.blockAtCrater(&s.Enemies[0], 260) {
		t.Fatal("impact target fell into its own crater")
	}
	return s, s.Craters[0].ID
}
func TestCraterRepairCountsLivingWorkers(t *testing.T) {
	for _, workers := range []int{1, 2, 3, 6} {
		s, id := craterCase(t, workers)
		steps := 0
		for s.Craters[0].Remaining > 0 && steps < 40 {
			s.ElapsedMS += StepMS
			s.Tick++
			s.repairCraters()
			steps++
		}
		if steps != 30/workers {
			t.Fatalf("%d workers took %d steps", workers, steps)
		}
		for _, e := range s.Enemies {
			if e.Repairing == id {
				t.Fatal("filled crater still holds worker")
			}
		}
	}
	s, _ := craterCase(t, 3)
	s.Enemies[1].HP = 0
	s.ElapsedMS += StepMS
	s.repairCraters()
	if math.Abs(s.Craters[0].Remaining-5.6) > .0001 {
		t.Fatal("dead worker contributed", s.Craters[0])
	}
}
func TestCraterCooldownOverlapAndRestore(t *testing.T) {
	s, _ := craterCase(t, 1)
	r := s.World.Map.Routes[0]
	p := pathPosition(r.Points, 250)
	e := s.Enemies[0]
	e.X = p.X
	e.Y = p.Y
	tower := Tower{ID: 2, Kind: "cannon", Owner: 0, Level: 1}
	if s.makeCrater(&tower, e) || len(s.Craters) != 1 {
		t.Fatal("overlapping crater stacked")
	}
	e.Progress = 400
	p = pathPosition(r.Points, e.Progress)
	e.X = p.X
	e.Y = p.Y
	if !s.makeCrater(&tower, e) || tower.PassiveReadyAt != s.ElapsedMS+6000 {
		t.Fatal("missing skill/cooldown")
	}
	e.Progress = 550
	p = pathPosition(r.Points, e.Progress)
	e.X = p.X
	e.Y = p.Y
	if s.makeCrater(&tower, e) {
		t.Fatal("crater ignored cooldown")
	}
	s.ElapsedMS += StepMS
	s.Tick++
	s.repairCraters()
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Craters, restored.Craters) || s.Enemies[1].Repairing != restored.Enemies[1].Repairing {
		t.Fatal("repair lost on restore")
	}
}
func skillCase(t *testing.T, kind string) *State {
	s := centralCase(t, 1)
	s.SkillRules = 0 // archived focus/ring skill cases
	s.Phase = "battle"
	s.ElapsedMS = 200
	s.Tick = 1
	p := s.World.Map.Pads[0]
	s.Towers = []Tower{{ID: 1, Pad: 0, Owner: 0, Kind: kind, Level: 1, Spent: 150, Priority: "first"}}
	s.TowerSerial = 1
	for i := 0; i < 4; i++ {
		s.Enemies = append(s.Enemies, Enemy{ID: i + 1, Kind: "slime", Route: s.World.Map.Routes[0].ID, Progress: 200 - float64(i), X: p.X + 15 + float64(i)*12, Y: p.Y + 15, HP: 10000, MaxHP: 10000, Speed: 50, Leak: 1})
	}
	s.EnemySerial = 4
	return s
}
func TestArrowFocusIsSingleTargetAndResetsOnSwitch(t *testing.T) {
	s := skillCase(t, "arrow")
	for i := 0; i < 3; i++ {
		s.ElapsedMS = 200 + int64(i)*600
		if !s.fireTower(0) {
			t.Fatal("did not fire")
		}
	}
	if math.Abs(s.Enemies[0].HP-(10000-22*3.8)) > .0001 {
		t.Fatal("missing focus damage", s.Enemies[0].HP)
	}
	for _, e := range s.Enemies[1:] {
		if e.HP != 10000 {
			t.Fatal("arrow hit another target")
		}
	}
	s = skillCase(t, "arrow")
	s.fireTower(0)
	s.ElapsedMS += 600
	s.fireTower(0)
	s.Enemies[0].HP = 0
	s.ElapsedMS += 600
	s.fireTower(0)
	if s.Enemies[1].HP != 9978 || s.Towers[0].FocusHits != 1 {
		t.Fatal("focus carried to different target")
	}
}
func TestFrostAndArcanePassivesAreBounded(t *testing.T) {
	s := skillCase(t, "frost")
	s.fireTower(0)
	for _, e := range s.Enemies {
		if e.SlowUntil != s.Tick+15 {
			t.Fatal("frost ring missing", e)
		}
	}
	if s.Towers[0].PassiveReadyAt != 8200 {
		t.Fatal("frost cooldown")
	}
	s = skillCase(t, "arcane")
	s.fireTower(0)
	if s.Enemies[0].HP != 9952 || s.Enemies[1].HP != 9976 || s.Enemies[2].HP != 9976 || s.Enemies[3].HP != 10000 {
		t.Fatal("chain targets/damage", s.Enemies)
	}
	s.ElapsedMS += 1200
	s.fireTower(0)
	if s.Enemies[1].HP != 9976 || s.Enemies[2].HP != 9976 {
		t.Fatal("chain ignored cooldown")
	}
}

// Persisted repair geometry and worker identity need validation before the
// new fields can be trusted by the timer or renderer.
func TestCraterSnapshotRejectsInvalidState(t *testing.T) {
	for _, change := range []func(*State){
		func(s *State) { s.Craters[0].Remaining = 7 },
		func(s *State) { s.Craters[0].Route = "missing" },
		func(s *State) { s.Craters[0].X += 40 },
		func(s *State) { s.Craters[0].Workers = []int{9999} },
		func(s *State) { s.Enemies[1].Repairing = 9999 },
	} {
		s, _ := craterCase(t, 1)
		change(s)
		raw, _ := json.Marshal(s)
		if _, err := decode(gameapi.Match{Schema: 3, Data: raw}); err == nil {
			t.Fatal("invalid crater state accepted")
		}
	}
}
func TestCentralAndLegacySnapshotVersions(t *testing.T) {
	s := centralCase(t, 1)
	m, e := s.Envelope()
	if e != nil || m.Schema != 4 {
		t.Fatal("new central skills need schema 4", m.Schema, e)
	}
	if _, e = decode(m); e != nil {
		t.Fatal(e)
	}
	s.World.Layout = ""
	s.SkillRules = 0
	m, e = s.Envelope()
	if e != nil || m.Schema != 2 {
		t.Fatal("legacy layout changed schema", e)
	}
	if _, e = decode(m); e != nil {
		t.Fatal(e)
	}
	m.Schema = 3
	if _, e = decode(m); e == nil {
		t.Fatal("schema 3 accepted a missing layout")
	}
}

func TestBasicFrostPreservesLongerPassiveSlow(t *testing.T) {
	s := skillCase(t, "frost")
	s.Enemies[0].SlowUntil = s.Tick + 15
	s.hit(0, 1, s.Towers[0])
	if s.Enemies[0].SlowUntil != s.Tick+15 {
		t.Fatal("basic hit shortened passive slow")
	}
}

func TestCentralCatalogIncludesPassiveBeforeBuilding(t *testing.T) {
	s := centralCase(t, 1)
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	v, err := view(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	var output struct {
		Catalog []struct {
			Passive *PassiveView `json:"passive"`
		} `json:"catalog"`
	}
	if err = json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Catalog) != 4 {
		t.Fatal("catalog missing")
	}
	for _, item := range output.Catalog {
		if item.Passive == nil || item.Passive.Name == "" {
			t.Fatal("passive missing before build")
		}
	}
}
