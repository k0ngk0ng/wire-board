package starfort_test

// Failure scenarios specified before the scheduler change:
// 200 ms rounding discards level/speed improvements; towers in one timer batch
// receive the same visual timestamp; array order overrides actual due order;
// a dead target absorbs a later shot; idle/out-of-range towers bank a burst;
// saving loses fractional cooldown; offline timers fast-forward combat;
// healing, splash hits and short-lived spawns cannot reconstruct visible HP;
// changing the attack clock changes movement, frost or ability durations.
// All assertions cross the production Module/serialized Match boundary. The
// deterministic, opt-in fixture is also reused by the real HTTP browser host.

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
)

type cadenceEvent struct {
	ID, TowerID, TargetID int
	Type, Kind            string
	AtMS                  *int64
	Damage                []starfort.DamageDelta
	Target                *struct {
		starfort.Enemy
		Health, MaxHealth float64
	}
}

// fixtureCadence keeps the real map and legal tower pads. A nearly stationary
// high-HP enemy makes event interval/count measurements independent of range.
func fixtureCadence(s *starfort.State, level int, branch string) error {
	route := s.World.Map.Routes[0]
	best := math.Inf(1)
	progress := 0.
	var selected []int
	for d := 20.; d < fixturePathLength(route.Points)-20; d += 5 {
		x, y := fixturePathPosition(route.Points, d)
		pads := append([]starfort.Pad{}, s.World.Map.Pads...)
		sort.Slice(pads, func(i, j int) bool {
			return math.Hypot(pads[i].X-x, pads[i].Y-y) < math.Hypot(pads[j].X-x, pads[j].Y-y)
		})
		distance := math.Hypot(pads[3].X-x, pads[3].Y-y)
		if distance < best {
			best, progress = distance, d
			selected = []int{pads[0].ID, pads[1].ID, pads[2].ID, pads[3].ID}
		}
	}
	if best > 170 {
		return fmt.Errorf("no four real pads share a target: %g", best)
	}
	x, y := fixturePathPosition(route.Points, progress)
	s.Phase, s.NextAt = "battle", 0
	s.WaveInfo = starfort.WaveInfo{Title: "攻击间隔验收", Total: 1, Spawned: 1}
	s.World.Event.ID = "calm"
	s.EnemySerial, s.TowerSerial = 1, 4
	s.Enemies = []starfort.Enemy{{ID: 1, Kind: "shield", X: x, Y: y, Progress: progress, HP: 1000000, MaxHP: 1000000, Shield: 800000, Speed: .001, Reward: 10, Leak: 1, Route: route.ID}}
	s.Towers = nil
	for i, kind := range []string{"arrow", "cannon", "frost", "arcane"} {
		s.Towers = append(s.Towers, starfort.Tower{ID: i + 1, Pad: selected[i], Owner: 0, Kind: kind, Level: level, Branch: branch, Spent: 1000, Priority: "first"})
	}
	for i := range s.Players {
		s.Players[i].Gold = 100000
	}
	return nil
}

func cadenceState(b *rulesTable) starfort.State {
	b.t.Helper()
	var s starfort.State
	if err := json.Unmarshal(b.s.Data, &s); err != nil {
		b.t.Fatal(err)
	}
	return s
}

func cadenceSave(b *rulesTable, s starfort.State) {
	b.t.Helper()
	var err error
	b.s, err = s.Envelope()
	if err != nil {
		b.t.Fatal(err)
	}
	b.record(map[string]any{"op": "cadence_fixture", "match": b.s})
	b.validate()
}

func cadenceTable(t *testing.T, level int, branch string) *rulesTable {
	t.Helper()
	b := newLegacyRulesTable(t, 1, map[string]any{})
	s := cadenceState(b)
	if err := fixtureCadence(&s, level, branch); err != nil {
		t.Fatal(err)
	}
	cadenceSave(b, s)
	return b
}

func cadenceEvents(b *rulesTable, after int) []cadenceEvent {
	b.t.Helper()
	var events []cadenceEvent
	for _, raw := range b.view(0).Events {
		var e cadenceEvent
		if err := json.Unmarshal(raw, &e); err != nil {
			b.t.Fatal(err)
		}
		if e.ID > after {
			events = append(events, e)
		}
	}
	return events
}

func TestStarfortCadenceExactIntervals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		level  int
		branch string
	}{{"base", 1, ""}, {"level2", 2, ""}, {"power", 4, "power"}, {"speed", 4, "speed"}, {"minimum200", 201, "speed"}} {
		t.Run(tc.name, func(t *testing.T) {
			b := cadenceTable(t, tc.level, tc.branch)
			intervals := map[int]int{}
			for _, tower := range b.view(0).Towers {
				intervals[tower.ID] = tower.EffectiveInterval
			}
			if tc.level == 1 && !reflect.DeepEqual(intervals, map[int]int{1: 600, 2: 1600, 3: 1000, 4: 1200}) {
				t.Fatalf("base balance changed: %v", intervals)
			}
			times := map[int][]int64{}
			events := []cadenceEvent{}
			last := 0
			for sec := 0; sec < 12; sec++ {
				b.timer(time.Second)
				batch := cadenceEvents(b, last)
				for _, e := range batch {
					last = e.ID
					if e.AtMS == nil {
						t.Fatalf("event %d lacks exact atMS: %+v", e.ID, e)
					}
					if *e.AtMS > b.view(0).ElapsedMS {
						t.Fatalf("future event: %+v", e)
					}
					if e.Type == "shot" {
						times[e.TowerID] = append(times[e.TowerID], *e.AtMS)
					}
				}
				events = append(events, batch...)
				// Every timer already serializes the complete Match. An explicit
				// round trip also rejects reliance on a process-local due clock.
				var restored = b.s
				if err := json.Unmarshal(rulesJSON(b.s), &restored); err != nil {
					t.Fatal(err)
				}
				b.s = restored
			}
			for id, interval := range intervals {
				want := int((12000-200)/int64(interval)) + 1
				if len(times[id]) != want {
					t.Fatalf("tower%d count=%d want%d interval%d times%v", id, len(times[id]), want, interval, times[id])
				}
				for i, at := range times[id] {
					if at != 200+int64(i*interval) {
						t.Fatalf("tower%d shot%d at%d expected%d", id, i, at, 200+i*interval)
					}
				}
			}
			if err := os.WriteFile(filepath.Join(b.dir, "cadence.json"), rulesJSON(map[string]any{"intervalsMS": intervals, "shotTimesMS": times, "events": events}), 0644); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStarfortCadenceOrdersAndRetargets(t *testing.T) {
	b := cadenceTable(t, 2, "")
	s := cadenceState(b)
	s.Towers = []starfort.Tower{s.Towers[0], s.Towers[3]}
	s.Towers[0].Cooldown, s.Towers[1].Cooldown = 175, 25
	s.Enemies[0].Kind, s.Enemies[0].HP, s.Enemies[0].MaxHP, s.Enemies[0].Shield = "slime", 1, 1, 0
	next := s.Enemies[0]
	next.ID, next.HP, next.MaxHP, next.Progress = 2, 1000000, 1000000, next.Progress-1
	s.Enemies = append(s.Enemies, next)
	s.EnemySerial = 2
	cadenceSave(b, s)
	b.timer(time.Second)
	shots := []cadenceEvent{}
	for _, e := range cadenceEvents(b, 0) {
		if e.Type == "shot" {
			shots = append(shots, e)
		}
	}
	if len(shots) < 2 || shots[0].TowerID != 4 || shots[0].TargetID != 1 || shots[1].TowerID != 1 || shots[1].TargetID != 2 || shots[0].AtMS == nil || *shots[0].AtMS != 25 || *shots[1].AtMS != 175 {
		t.Fatalf("due order/retarget wrong: %+v", shots)
	}
}

func TestStarfortCadenceIdleAndOffline(t *testing.T) {
	b := cadenceTable(t, 2, "")
	s := cadenceState(b)
	s.Towers = s.Towers[:1]
	target := s.Enemies[0]
	s.Enemies[0].Progress = 0
	s.Enemies[0].X, s.Enemies[0].Y = fixturePathPosition(s.World.Map.Routes[0].Points, 0)
	cadenceSave(b, s)
	for i := 0; i < 10; i++ {
		b.timer(time.Second)
	}
	for _, e := range cadenceEvents(b, 0) {
		if e.Type == "shot" {
			t.Fatal("out of range target fired")
		}
	}
	s = cadenceState(b)
	if s.Towers[0].Cooldown != 0 {
		t.Fatalf("idle cooldown accumulated: %d", s.Towers[0].Cooldown)
	}
	s.Enemies[0] = target
	cadenceSave(b, s)
	b.timer(24 * time.Hour)
	if b.view(0).ElapsedMS != 11000 {
		t.Fatal("offline time advanced more than one batch")
	}
	var shots []int64
	for _, e := range cadenceEvents(b, 0) {
		if e.Type == "shot" && e.AtMS != nil {
			shots = append(shots, *e.AtMS)
		}
	}
	if !reflect.DeepEqual(shots, []int64{10200, 10776}) {
		t.Fatalf("idle banked attacks or lost remainder: %v", shots)
	}
}

func TestStarfortCadenceHealthTimeline(t *testing.T) {
	b := cadenceTable(t, 1, "")
	s := cadenceState(b)
	s.Towers = []starfort.Tower{s.Towers[1]}
	s.Enemies[0].Shield = 10
	s.Enemies[0].HP = 30
	s.Enemies[0].MaxHP = 100
	second := s.Enemies[0]
	second.ID, second.HP, second.Shield = 2, 100, 0
	s.Enemies = append(s.Enemies, second)
	s.EnemySerial = 2
	cadenceSave(b, s)
	b.timer(time.Second)
	var shot, death *cadenceEvent
	hits := map[int]float64{}
	for _, e := range cadenceEvents(b, 0) {
		if e.Type != "shot" && e.Type != "hit" && e.Type != "death" {
			continue
		}
		if e.Target == nil || e.Target.ID != e.TargetID || e.AtMS == nil || *e.AtMS != 200 {
			t.Fatalf("incomplete health timeline: %+v", e)
		}
		if e.Type == "shot" {
			copy := e
			shot = &copy
		}
		if e.Type == "death" {
			copy := e
			death = &copy
		}
		if e.Type == "hit" {
			hits[e.TargetID] = e.Target.Health
			for _, d := range e.Damage {
				hits[d.ID] = 100 - d.Amount
			}
		}
	}
	if shot == nil || death == nil || shot.Target.Health != 40 || death.Target.Health != 0 || hits[1] != 0 || hits[2] != 52 {
		t.Fatalf("shot prestate/splash/death disagree: shot%+v death%+v hits%v", shot, death, hits)
	}
}

func TestStarfortCadenceHealingTime(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "healer"}, {"kind": "slime", "hp": 50.}}, false)
	b.timer(time.Second)
	b.timer(time.Second)
	found := false
	for _, e := range cadenceEvents(b, 0) {
		if e.Type != "heal" {
			continue
		}
		found = true
		if e.AtMS == nil || *e.AtMS != 2000 || e.TargetID != 2 || e.Target == nil || e.Target.Health != 53 {
			t.Fatalf("healing clock/poststate wrong: %+v", e)
		}
	}
	if !found {
		t.Fatal("healing event missing")
	}
}

func TestStarfortCadenceSimultaneousAndLegacy(t *testing.T) {
	b := cadenceTable(t, 2, "")
	s := cadenceState(b)
	// Equal due times use persistent tower IDs, independent of save ordering.
	s.Towers = []starfort.Tower{s.Towers[3], s.Towers[0]}
	s.Towers[0].Cooldown, s.Towers[1].Cooldown = 25, 25
	cadenceSave(b, s)
	b.timer(time.Second)
	shots := []cadenceEvent{}
	for _, e := range cadenceEvents(b, 0) {
		if e.Type == "shot" {
			shots = append(shots, e)
		}
	}
	if len(shots) < 2 || shots[0].TowerID != 1 || shots[1].TowerID != 4 || *shots[0].AtMS != 25 || *shots[1].AtMS != 25 {
		t.Fatalf("simultaneous shots depend on array order: %+v", shots)
	}
	var old map[string]any
	if err := json.Unmarshal(b.s.Data, &old); err != nil {
		t.Fatal(err)
	}
	for _, event := range old["events"].([]any) {
		fields := event.(map[string]any)
		delete(fields, "atMS")
		delete(fields, "towerId")
		delete(fields, "target")
	}
	b.s.Data = rulesJSON(old)
	b.validate()
	last := cadenceState(b).EventSerial
	b.timer(time.Second)
	newShots := map[int]int64{}
	for _, e := range cadenceEvents(b, last) {
		if e.Type == "shot" {
			if e.AtMS == nil {
				t.Fatal("legacy restore lost new event clock")
			}
			if _, exists := newShots[e.TowerID]; !exists {
				newShots[e.TowerID] = *e.AtMS
			}
		}
	}
	if newShots[1] != 1177 || newShots[4] != 1178 {
		t.Fatalf("legacy cooldown lost: %v", newShots)
	}
}

func TestStarfortCadenceHTTP(t *testing.T) {
	for _, scenario := range []string{"cadence", "cadence-speed"} {
		t.Run(scenario, func(t *testing.T) {
			tb := newHost(t, 1, scenarioModule())
			tb.start(map[string]any{"scenario": scenario})
			client := tb.people[0]
			wait(t, "four seconds of real battle timer", func() bool { return data(client.room())["elapsedMS"].(float64) >= 4000 })
			room := client.room()
			var view struct {
				ElapsedMS int64
				Towers    []rulesTower
				Events    []cadenceEvent
			}
			if err := json.Unmarshal(rulesJSON(data(room)), &view); err != nil {
				t.Fatal(err)
			}
			times := map[int][]int64{}
			for _, event := range view.Events {
				if event.Type != "shot" {
					continue
				}
				if event.AtMS == nil || event.Target == nil || event.Target.Health <= 0 {
					t.Fatalf("HTTP omitted combat clock/prestate: %+v", event)
				}
				times[event.TowerID] = append(times[event.TowerID], *event.AtMS)
			}
			for _, tower := range view.Towers {
				want := int((view.ElapsedMS-200)/int64(tower.EffectiveInterval)) + 1
				if len(times[tower.ID]) != want {
					t.Fatalf("HTTP tower%d shots=%v want%d", tower.ID, times[tower.ID], want)
				}
				for i, at := range times[tower.ID] {
					if at != 200+int64(i*tower.EffectiveInterval) {
						t.Fatalf("HTTP tower%d interval collapsed: %v", tower.ID, times[tower.ID])
					}
				}
			}
			tb.trace = append(tb.trace, map[string]any{"label": "real system timer cadence", "elapsedMS": view.ElapsedMS, "shotTimesMS": times, "room": room, "spectatorRoom": tb.spectator.room()})
		})
	}
}

func TestStarfortCadencePhysicsAndSpawnBoundaries(t *testing.T) {
	t.Run("frost-eight-steps", func(t *testing.T) {
		b := cadenceTable(t, 1, "")
		s := cadenceState(b)
		s.Towers = []starfort.Tower{s.Towers[2]}
		s.Enemies[0].Speed = 10
		start := s.Enemies[0].Progress
		cadenceSave(b, s)
		b.timer(time.Second)
		if got := cadenceState(b).Enemies[0].Progress - start; math.Abs(got-5.5) > 1e-8 {
			t.Fatalf("frost first-second movement changed: %g", got)
		}
		b.action(0, map[string]any{"type": "sell", "towerId": 3}, true)
		b.timer(time.Second)
		if got := cadenceState(b).Enemies[0].Progress - start; math.Abs(got-12.8) > 1e-8 {
			t.Fatalf("frost must slow exactly eight 200ms physics steps: %g", got)
		}
	})
	t.Run("natural-spawns", func(t *testing.T) {
		b := newLegacyRulesTable(t, 1, map[string]any{})
		b.allReady()
		b.timer(time.Second)
		b.timer(time.Second)
		var spawns []int64
		for _, e := range cadenceEvents(b, 0) {
			if e.Type != "spawn" {
				continue
			}
			if e.AtMS == nil || e.Target == nil || e.Target.ID != e.TargetID || e.Target.Health != e.Target.MaxHealth || e.Target.Progress != 0 {
				t.Fatalf("spawn missing initial authority: %+v", e)
			}
			spawns = append(spawns, *e.AtMS)
		}
		if !reflect.DeepEqual(spawns, []int64{200, 1000, 1800}) {
			t.Fatalf("natural spawn cadence changed: %v", spawns)
		}
	})
}
