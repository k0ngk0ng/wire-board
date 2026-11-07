package starfort_test

// Failure inventory, recorded before implementation:
// - New branch controls and Apply disagree; forged branches, another owner or
//   spectators upgrade; restart changes the selected skill or old-save rules.
// - Cold weather adds no ice slow, multiplies the wrong factor, or affects old
//   matches; freeze excludes elites/bosses, never expires or advances on pause.
// - Cannon power lacks extra AoE; speed craters stop elite/boss units or never
//   stop ordinary units; probability/cooldowns are lost in saved RNG state.
// - Arcane power double-counts armor/shield bonuses; magic exceeds tower range,
//   lets silenced healers/banners/bosses act, repeats or inherits summon status.
// - Volley knockback misses elites/bosses, moves before impact or leaves the road.
// - Fifteen-wave refresh duplicates rewards/tasks, resets a paid active convoy,
//   misses the next cycle or repeats after restore/view/pause.
// - Bot choices become illegal or ordinary 1/2/4-seat campaigns no longer finish.
// Public Module + real HTTP/sessions/SQLite tests retain replay and summaries.
// Combat boundary fixtures are labelled; natural campaigns grant no resources.

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func specializationCase(t *testing.T, kind, branch string, enemies ...string) *rulesTable {
	t.Helper()
	b := newRulesTable(t, 1, map[string]any{"mode": "endless"})
	s := cadenceState(b)
	if err := fixtureCadence(&s, 4, ""); err != nil {
		t.Fatal(err)
	}
	for _, tower := range s.Towers {
		if tower.Kind == kind {
			tower.Branch = branch
			tower.Cooldown = 100
			s.Towers = []starfort.Tower{tower}
			break
		}
	}
	s.SkillRNG = 5 // Saved deterministic first roll is 5%, inside all proc windows.
	lead := s.Enemies[0]
	s.Enemies = nil
	for i, k := range enemies {
		e := lead
		e.ID = i + 1
		e.Kind = k
		e.Level = 4
		e.Speed = 10
		e.Shield = 0
		e.Phase = 1
		s.Enemies = append(s.Enemies, e)
	}
	s.EnemySerial = len(s.Enemies)
	s.WaveInfo.Total, s.WaveInfo.Spawned = len(s.Enemies), len(s.Enemies)
	cadenceSave(b, s)
	b.record(map[string]any{"op": "specialization_combat_fixture", "scope": "fixed combat boundary, production module actions/timers", "match": b.s})
	return b
}

func specializationRaw(b *rulesTable) map[string]any {
	b.t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(b.s.Data, &raw); err != nil {
		b.t.Fatal(err)
	}
	return raw
}

func specializationEvents(b *rulesTable, skill string) int {
	n := 0
	for _, e := range cadenceState(b).Events {
		if e.Skill == skill {
			n++
		}
	}
	return n
}

func specializationFarEnemy(b *rulesTable) {
	b.t.Helper()
	s := cadenceState(b)
	pad := s.World.Map.Pads[s.Towers[0].Pad]
	route := s.World.Map.Routes[0]
	best, distance := 0., 0.
	for p := 20.; p < fixturePathLength(route.Points)-50; p += 20 {
		x, y := fixturePathPosition(route.Points, p)
		d := math.Hypot(x-pad.X, y-pad.Y)
		if d > distance {
			best, distance = p, d
		}
	}
	if distance <= b.view(0).Towers[0].EffectiveRange+30 {
		b.t.Fatal("fixture has no out-of-range road")
	}
	e := s.Enemies[0]
	e.ID = len(s.Enemies) + 1
	e.Kind = "slime"
	e.HP = e.MaxHP
	e.Progress = best
	e.X, e.Y = fixturePathPosition(route.Points, best)
	s.Enemies = append(s.Enemies, e)
	s.EnemySerial = len(s.Enemies)
	s.WaveInfo.Total, s.WaveInfo.Spawned = len(s.Enemies), len(s.Enemies)
	cadenceSave(b, s)
}

func TestStarfortSpecializationBranchesHTTP(t *testing.T) {
	for _, tc := range []struct{ kind, branch string }{{"frost", "freeze"}, {"frost", "speed"}, {"cannon", "power"}, {"cannon", "speed"}, {"arcane", "power"}, {"arcane", "magic"}, {"arrow", "volley"}, {"arrow", "heart"}} {
		t.Run(tc.kind+"-"+tc.branch, func(t *testing.T) {
			m := starfort.Module()
			start := m.Start
			m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
				match, err := start(ctx, raw)
				if err != nil {
					return match, err
				}
				var s starfort.State
				_ = json.Unmarshal(match.Data, &s)
				s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: tc.kind, Level: 3, Spent: 500, Priority: "first"}}
				s.TowerSerial = 1
				s.Players[0].Gold = 10000
				return s.Envelope()
			}
			tb := newHost(t, 2, m)
			tb.start(nil)
			a := tb.people[0]
			controls := data(a.room())["controls"].(map[string]any)["upgrades"].([]any)[0].(map[string]any)
			found := false
			for _, v := range controls["branches"].([]any) {
				if v.(map[string]any)["id"] == tc.branch {
					found = true
				}
			}
			if !found {
				t.Fatalf("branch %s missing in public controls: %v", tc.branch, controls)
			}
			tb.people[1].act(map[string]any{"type": "upgrade", "towerId": 1, "branch": tc.branch}, 400)
			a.act(map[string]any{"type": "upgrade", "towerId": 1, "branch": "forged"}, 400)
			a.act(map[string]any{"type": "upgrade", "towerId": 1, "branch": tc.branch}, 200)
			for _, p := range tb.people {
				p.act(map[string]any{"type": "pause"}, 200)
			}
			tb.record("new specialization selected, paid and paused")
			tb.restart()
			v := data(a.room())["towers"].([]any)[0].(map[string]any)
			if v["branch"] != tc.branch || v["level"] != float64(4) {
				t.Fatal("restored specialization differs", v)
			}
		})
	}
}

func TestStarfortSpecializationKnockback(t *testing.T) {
	b := specializationCase(t, "arrow", "volley", "slime", "elite", "boss")
	s := cadenceState(b)
	s.SkillRNG = 10
	cadenceSave(b, s)
	b.timer(time.Second)
	if specializationEvents(b, "knockback") == 0 {
		t.Fatal("volley lacks independent 15% knockback")
	}
	for i, e := range cadenceState(b).Enemies {
		if e.Progress >= s.Enemies[i].Progress-40 || e.Progress < 0 {
			t.Fatal("knockback missed or left road", e.Kind, e.Progress)
		}
	}
}

func TestStarfortSpecializationColdAndFreeze(t *testing.T) {
	t.Run("cold", func(t *testing.T) {
		b := specializationCase(t, "frost", "speed", "slime")
		s := cadenceState(b)
		s.World.Event.ID = "frost"
		cadenceSave(b, s)
		before := s.Enemies[0].Progress
		b.timer(time.Second)
		if got := cadenceState(b).Enemies[0].Progress - before; math.Abs(got-3.4) > 1e-7 {
			t.Fatalf("cold ice should leave 40%% speed, including weather: %g", got)
		}
	})
	t.Run("freeze-range-and-expire", func(t *testing.T) {
		b := specializationCase(t, "frost", "freeze", "slime", "elite", "boss")
		specializationFarEnemy(b)
		before := cadenceState(b)
		b.timer(time.Second)
		after := cadenceState(b)
		if specializationEvents(b, "freeze") == 0 {
			t.Fatal("in-range freeze never triggered")
		}
		for i, e := range after.Enemies[:3] {
			if e.Progress != before.Enemies[i].Progress {
				t.Fatal("frozen enemy moved", e.Kind)
			}
		}
		if after.Enemies[3].Progress <= before.Enemies[3].Progress {
			t.Fatal("out-of-range enemy frozen")
		}
		raw := specializationRaw(b)
		for _, v := range raw["enemies"].([]any)[:3] {
			if v.(map[string]any)["frozenUntil"] == nil {
				t.Fatal("freeze omitted from save")
			}
		}
		after.Towers = nil
		cadenceSave(b, after)
		b.timer(time.Second)
		b.timer(time.Second)
		for i, e := range cadenceState(b).Enemies {
			if e.Progress <= before.Enemies[i].Progress {
				t.Fatal("freeze did not expire", e.Kind)
			}
		}
	})
}

func TestStarfortSpecializationCannon(t *testing.T) {
	t.Run("blast", func(t *testing.T) {
		b := specializationCase(t, "cannon", "power", "slime", "elite", "boss")
		b.timer(time.Second)
		if specializationEvents(b, "blast") == 0 {
			t.Fatal("power branch lacks explosive AoE")
		}
		for _, e := range cadenceState(b).Enemies {
			if e.HP >= 1000000 {
				t.Fatal("blast missed enemy", e.Kind)
			}
		}
	})
	t.Run("crater-heavy-immunity", func(t *testing.T) {
		b := specializationCase(t, "cannon", "speed", "slime", "elite", "boss", "slime")
		b.timer(time.Second)
		s := cadenceState(b)
		if len(s.Craters) == 0 {
			t.Fatal("speed branch did not create crater")
		}
		for _, e := range s.Enemies {
			if (e.Kind == "elite" || e.Kind == "boss") && e.Repairing != 0 {
				t.Fatal("heavy unit blocked by speed crater", e.Kind)
			}
		}
		if s.Enemies[3].Repairing == 0 {
			t.Fatal("ordinary following enemy ignored crater")
		}
	})
}

func TestStarfortSpecializationArcane(t *testing.T) {
	for _, kind := range []string{"shield", "elite", "boss"} {
		t.Run("shield-"+kind, func(t *testing.T) {
			b := specializationCase(t, "arcane", "power", kind)
			s := cadenceState(b)
			s.Enemies[0].Armor = .9
			s.Enemies[0].Shield = 1000000
			cadenceSave(b, s)
			damage := b.view(0).Towers[0].EffectiveDamage
			b.timer(time.Second)
			after := cadenceState(b)
			factor := 1.5
			if kind == "shield" {
				factor *= 1.15
			}
			if got := 1000000 - after.Enemies[0].Shield; math.Abs(got-damage*factor) > 1e-7 {
				t.Fatalf("shield damage %g want %g", got, damage*factor)
			}
		})
	}
	t.Run("meteor-silence", func(t *testing.T) {
		b := specializationCase(t, "arcane", "magic", "healer", "banner", "boss", "engineer")
		specializationFarEnemy(b)
		s := cadenceState(b)
		s.Enemies[0].HP = 500000
		s.Enemies[2].HP = 400000
		for i := range s.Enemies {
			s.Enemies[i].AbilityTick = 0
		}
		cadenceSave(b, s)
		b.timer(time.Second)
		after := cadenceState(b)
		if specializationEvents(b, "meteor") == 0 {
			t.Fatal("missing meteor")
		}
		if len(after.Enemies) != 5 || after.Enemies[2].Phase != 1 {
			t.Fatal("silenced boss summoned", after.Enemies)
		}
		for i, e := range after.Enemies {
			if math.Abs(e.Progress-s.Enemies[i].Progress-10) > 1e-7 {
				t.Fatal("silenced banner still accelerated", e.Kind)
			}
		}
		if after.Enemies[4].HP != s.Enemies[4].HP {
			t.Fatal("meteor exceeded tower range")
		}
		for _, v := range specializationRaw(b)["enemies"].([]any)[:4] {
			if v.(map[string]any)["silencedUntil"] == nil {
				t.Fatal("silence omitted from save")
			}
		}
		after.Towers = nil
		cadenceSave(b, after)
		b.timer(time.Second)
		if cadenceState(b).Enemies[0].HP != after.Enemies[0].HP {
			t.Fatal("silenced healer healed")
		}
		b.timer(time.Second)
		b.timer(time.Second)
		if len(cadenceState(b).Enemies) != 7 {
			t.Fatal("boss did not resume after silence expired")
		}
	})
}

func TestStarfortSpecializationObjectiveRefresh(t *testing.T) {
	b := newRulesTable(t, 1, map[string]any{"mode": "endless"})
	s := cadenceState(b)
	if err := fixtureWorldStage(b.context(0), &s, 4); err != nil {
		t.Fatal(err)
	}
	for i := range s.World.Objectives {
		o := &s.World.Objectives[i]
		o.Status = "completed"
		o.HP = 20
		if o.Kind == "convoy" {
			o.Status = "active"
			o.Progress = 100
			o.HP = 33
			o.StartWave = 16
			o.EndWave = 0
		}
	}
	s.World.Choice = nil
	s.World.BoonChoice = nil
	if err := fixtureClearedWave(b.context(0), &s, 30, false); err != nil {
		t.Fatal(err)
	}
	cadenceSave(b, s)
	b.timer(time.Second)
	after := cadenceState(b)
	for _, o := range after.World.Objectives {
		if o.Kind == "convoy" {
			if o.Status != "active" || o.HP != 33 || o.Progress < 100 {
				t.Fatal("refresh reset paid convoy", o)
			}
		} else if o.Status != "available" || o.HP != o.MaxHP {
			t.Fatal("objective not refreshed at 30", o)
		}
	}
	if specializationRaw(b)["world"].(map[string]any)["objectiveRefreshWave"] != float64(30) {
		t.Fatal("refresh cycle not persisted")
	}
	gold := after.Players[0].Gold
	before := string(b.s.Data)
	for i := 0; i < 4; i++ {
		b.view(0)
	}
	if string(b.s.Data) != before {
		t.Fatal("view repeated refresh")
	}
	var restored gameapi.Match
	_ = json.Unmarshal(rulesJSON(b.s), &restored)
	b.s = restored
	b.validate()
	if after.World.BoonChoice != nil {
		b.action(0, map[string]any{"type": "boon_vote", "choice": "power"}, true)
	}
	b.action(0, map[string]any{"type": "pause"}, true)
	b.timer(20 * time.Second)
	if cadenceState(b).Players[0].Gold != gold {
		t.Fatal("refresh/pause minted rewards")
	}
}

func TestStarfortSpecializationRefreshHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, raw)
		if err != nil {
			return match, err
		}
		var s starfort.State
		_ = json.Unmarshal(match.Data, &s)
		if err = fixtureWorldStage(ctx, &s, 4); err != nil {
			return match, err
		}
		for i := range s.World.Objectives {
			o := &s.World.Objectives[i]
			o.Status = "completed"
			o.HP = 20
			if o.Kind == "convoy" {
				o.Status = "active"
				o.HP = 33
				o.Progress = 100
				o.StartWave = 16
				o.EndWave = 0
			}
		}
		if err = fixtureClearedWave(ctx, &s, 30, false); err != nil {
			return match, err
		}
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(map[string]any{"mode": "endless"})
	a := tb.people[0]
	deadline := time.Now().Add(6 * time.Second)
	for data(a.room())["wave"].(float64) != 31 {
		if time.Now().After(deadline) {
			t.Fatal("production HTTP timer did not refresh after wave 30")
		}
		time.Sleep(20 * time.Millisecond)
	}
	tb.record("fixture skipped earlier combat; real host timer refreshed cycle 30")
	if worldData(t, a.room())["objectiveRefreshWave"] != float64(30) {
		t.Fatal("refresh cycle missing from HTTP projection")
	}
	voteAll(tb, "boonChoice", "boon_vote", 0)
	var mine string
	for _, raw := range worldData(t, a.room())["objectives"].([]any) {
		o := raw.(map[string]any)
		if o["kind"] == "mine" {
			mine = o["id"].(string)
			if o["status"] != "available" {
				t.Fatal("completed mine did not refresh")
			}
		}
		if o["kind"] == "convoy" && (o["status"] != "active" || o["hp"] != float64(33)) {
			t.Fatal("refresh replaced paid convoy", o)
		}
	}
	before := gold(a.room(), 0)
	a.act(map[string]any{"type": "objective", "objectiveId": mine}, 200)
	if before-gold(a.room(), 0) != 120 {
		t.Fatal("refreshed task did not charge multiplayer fee")
	}
	tb.people[1].act(map[string]any{"type": "objective", "objectiveId": mine}, 400)
	for _, p := range tb.people {
		p.act(map[string]any{"type": "pause"}, 200)
	}
	tb.record("refreshed task accepted once and paid; convoy preserved")
	tb.restart()
	if gold(a.room(), 0) != before-120 {
		t.Fatal("restart duplicated task fee or reward")
	}
}

func TestStarfortSpecializationLegacySave(t *testing.T) {
	b := newRulesTable(t, 1, map[string]any{})
	raw := specializationRaw(b)
	delete(raw, "specializationRules")
	b.s.Data = rulesJSON(raw)
	s := cadenceState(b)
	s.Players[0].Gold = 10000
	s.Towers = []starfort.Tower{{ID: 1, Pad: 0, Owner: 0, Kind: "frost", Level: 3, Spent: 500, Priority: "first"}}
	s.TowerSerial = 1
	cadenceSave(b, s)
	b.action(0, map[string]any{"type": "upgrade", "towerId": 1, "branch": "freeze"}, false)
	b.action(0, map[string]any{"type": "upgrade", "towerId": 1, "branch": "power"}, true)
	before := b.view(0)
	var restored gameapi.Match
	_ = json.Unmarshal(rulesJSON(b.s), &restored)
	b.s = restored
	if !reflect.DeepEqual(before, b.view(0)) {
		t.Fatal("legacy save changed on restore")
	}
}

func TestStarfortSpecializationNaturalCampaign(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			b := newRulesTable(t, n, map[string]any{"waves": 20})
			for sec := 0; sec < 16000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			if b.view(0).Phase != "won" {
				t.Fatal("natural 20-wave campaign failed", b.view(0).CompletedWaves)
			}
		})
	}
}
