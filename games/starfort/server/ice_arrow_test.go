package starfort

// Pre-implementation failure inventory: crossing rolls every tick; slow expires
// or stacks to zero; paused time expires walls; saves reroll skills; wall/skill
// history grows without bounds; ordinary high-level, elite or boss enemies are
// executed; AoE damages unrelated targets; branch switching/forgery is accepted;
// legacy saves silently acquire new rules. Module/HTTP campaigns cover integration.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"reflect"
	"testing"
	"time"
)

func iceArrowCase(t *testing.T, kind string) *State {
	s := skillCase(t, kind)
	s.SkillRules = 2
	s.SkillRNG = 5
	s.NextAt = 0
	s.World.Event.ID = "calm"
	s.WaveInfo.Total = len(s.Enemies)
	s.WaveInfo.Spawned = len(s.Enemies)
	for i := range s.Enemies {
		s.Enemies[i].Level = 4
	}
	return s
}
func TestIceWallCrossingPermanentSlowAndRestore(t *testing.T) {
	s := iceArrowCase(t, "frost")
	s.fireTower(0)
	if len(s.IceWalls) != 1 {
		t.Fatal("missing wall")
	}
	wall := s.IceWalls[0]
	if s.Enemies[1].Frostbound {
		t.Fatal("freezes before crossing")
	}
	s.SkillRNG = 2
	s.crossIceWalls(&s.Enemies[1], wall.Progress-1, wall.Progress+1)
	if !s.Enemies[1].Frostbound {
		t.Fatal("crossing did not apply permanent slow")
	}
	s.Enemies[1].Progress = wall.Progress + 1
	s.Enemies[1].SlowUntil = 0
	s.Enemies[1].Speed = 10
	s.Towers = nil
	s.ElapsedMS = 50200
	s.Tick = 251
	m, e := s.Envelope()
	if e != nil {
		t.Fatal(e)
	}
	restored, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	if !restored.Enemies[1].Frostbound {
		t.Fatal("permanent slow lost on save")
	}
	before := restored.Enemies[1].Progress
	restored.step(restored.ServerAt)
	if math.Abs(restored.Enemies[1].Progress-before-1.4) > 1e-8 {
		t.Fatal("slow expired or stacked", restored.Enemies[1].Progress-before)
	}
	if len(restored.IceWalls) != 0 {
		t.Fatal("expired walls retained")
	}
}
func TestIceWallMissDoesNotRerollAndCooldownIsBounded(t *testing.T) {
	s := iceArrowCase(t, "frost")
	s.fireTower(0)
	wall := s.IceWalls[0]
	s.SkillRNG = 1
	s.crossIceWalls(&s.Enemies[1], wall.Progress-1, wall.Progress+1)
	if s.Enemies[1].Frostbound {
		t.Fatal("miss fixture unexpectedly proc'd")
	}
	rng := s.SkillRNG
	s.crossIceWalls(&s.Enemies[1], wall.Progress+1, wall.Progress+3)
	if s.SkillRNG != rng {
		t.Fatal("rerolled after passing wall")
	}
	s.fireTower(0)
	if len(s.IceWalls) != 1 || s.Towers[0].PassiveReadyAt != 8200 {
		t.Fatal("wall ignores cooldown")
	}
	s.ElapsedMS = wall.ExpiresMS + 1
	s.crossIceWalls(&s.Enemies[2], wall.Progress-1, wall.Progress+1)
	if s.Enemies[2].Frostbound {
		t.Fatal("expired wall still affects enemies")
	}
}
func TestHeartArrowThresholdAndEliteBossImmunity(t *testing.T) {
	for _, tc := range []struct {
		kind    string
		level   int
		execute bool
	}{{"slime", 8, true}, {"shield", 8, true}, {"slime", 9, false}, {"elite", 1, false}, {"boss", 1, false}} {
		t.Run(tc.kind+string(rune('0'+tc.level)), func(t *testing.T) {
			s := iceArrowCase(t, "arrow")
			s.Towers[0].Level = 4
			s.Towers[0].Branch = "heart"
			s.Enemies = s.Enemies[:1]
			e := &s.Enemies[0]
			e.Kind = tc.kind
			e.Level = tc.level
			e.Shield = 500
			e.Armor = .7
			before := e.HP + e.Shield
			damage, _, _ := s.towerStats(s.Towers[0])
			s.fireTower(0)
			if tc.execute {
				if e.HP != 0 || e.Shield != 0 {
					t.Fatal("eligible target survived")
				}
			} else {
				if e.HP <= 0 || math.Abs(before-e.HP-e.Shield-damage*3*.3) > 1e-7 {
					t.Fatal("ineligible enemy executed or wrong bonus", e)
				}
			}
		})
	}
}
func TestArrowRainIsProbabilisticAndLocal(t *testing.T) {
	for _, seed := range []uint64{1, 2} {
		s := iceArrowCase(t, "arrow")
		s.Towers[0].Level = 4
		s.Towers[0].Branch = "volley"
		s.SkillRNG = seed
		s.Enemies[3].X += 300
		damage, _, _ := s.towerStats(s.Towers[0])
		s.fireTower(0)
		if math.Abs(s.Enemies[0].HP-(10000-damage)) > 1e-8 {
			t.Fatal("primary damage")
		}
		want := 10000.
		if seed == 2 {
			want -= damage * .7
		}
		if math.Abs(s.Enemies[1].HP-want) > 1e-8 || s.Enemies[3].HP != 10000 {
			t.Fatal("AOE chance or radius")
		}
	}
}
func TestNewSkillRandomStateSurvivesRestore(t *testing.T) {
	s := iceArrowCase(t, "arrow")
	s.Towers[0].Level = 4
	s.Towers[0].Branch = "volley"
	s.SkillRNG = 1
	s.fireTower(0)
	m, e := s.Envelope()
	if e != nil {
		t.Fatal(e)
	}
	r, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	s.ElapsedMS += 600
	s.Tick += 3
	r.ElapsedMS += 600
	r.Tick += 3
	s.fireTower(0)
	r.fireTower(0)
	if !reflect.DeepEqual(s.Enemies, r.Enemies) || s.SkillRNG != r.SkillRNG {
		t.Fatal("restore rerolled skill")
	}
}
func TestArrowBranchAuthorityAndLegacyRules(t *testing.T) {
	s := iceArrowCase(t, "arrow")
	s.Towers[0].Level = 3
	s.Players[0].Gold = 10000
	m, _ := s.Envelope()
	ctx := gameapi.Context{Actor: 0, Players: 1, Now: time.UnixMilli(s.ServerAt)}
	for _, branch := range []string{"power", "speed", "invalid"} {
		raw, _ := json.Marshal(Action{Type: "upgrade", Epoch: s.Epoch, TowerID: 1, Branch: branch})
		if _, e := Module().Apply(ctx, m, raw); e == nil {
			t.Fatal("accepted invalid skill branch", branch)
		}
	}
	for _, branch := range []string{"volley", "heart"} {
		raw, _ := json.Marshal(Action{Type: "upgrade", Epoch: s.Epoch, TowerID: 1, Branch: branch})
		next, e := Module().Apply(ctx, m, raw)
		if e != nil {
			t.Fatal(e)
		}
		state, _ := decode(next)
		if state.Towers[0].Branch != branch {
			t.Fatal("branch lost")
		}
		if _, e = Module().Apply(ctx, next, raw); e == nil {
			t.Fatal("changed branch twice")
		}
	}
	s.SkillRules = 0
	s.SkillRNG = 0
	s.Towers[0].Level = 4
	s.Towers[0].Branch = "power"
	m, e := s.Envelope()
	if e != nil {
		t.Fatal(e)
	}
	r, e := decode(m)
	if e != nil || r.SkillRules != 0 || r.passiveView(r.Towers[0]).Name != "追猎重矢" {
		t.Fatal("legacy rules changed", e)
	}
}
func TestIceWallLimitsAndSaveValidation(t *testing.T) {
	s := iceArrowCase(t, "frost")
	for i := 0; i < 40; i++ {
		tower := s.Towers[0]
		tower.ID = i + 1
		tower.PassiveReadyAt = 0
		s.makeIceWall(&tower, s.Enemies[0])
	}
	if len(s.IceWalls) != 32 {
		t.Fatal("unbounded walls", len(s.IceWalls))
	}
	s.TowerSerial = 40
	m, e := s.Envelope()
	if e != nil {
		t.Fatal(e)
	}
	r, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	r.IceWalls[0].Route = "missing"
	if _, e = r.Envelope(); e == nil {
		t.Fatal("invalid wall route accepted")
	}
	if m.Schema != 4 {
		t.Fatal("new skill schema not explicit")
	}
	var public map[string]json.RawMessage
	v, _ := view(m, 0)
	raw, _ := json.Marshal(v)
	json.Unmarshal(raw, &public)
	if _, ok := public["skillRng"]; ok {
		t.Fatal("private random state on wire")
	}
}
func TestNewEnemiesDoNotInheritPermanentFrostWithoutCrossing(t *testing.T) {
	s := iceArrowCase(t, "arrow")
	s.Enemies = s.Enemies[:1]
	s.Enemies[0].Kind = "boss"
	s.Enemies[0].HP = 4000
	s.Enemies[0].Phase = 1
	s.Enemies[0].Frostbound = true
	s.enemyAbilities()
	for _, e := range s.Enemies[1:] {
		if e.Frostbound || e.Level != 4 {
			t.Fatal("summoned guard inherited frost or lost level")
		}
	}
	s = iceArrowCase(t, "arrow")
	s.Enemies = s.Enemies[:1]
	s.Towers = nil
	s.Enemies[0].Kind = "splitter"
	s.Enemies[0].HP = 0
	s.Enemies[0].Frostbound = true
	s.step(s.ServerAt)
	for _, e := range s.Enemies {
		if e.Frostbound || e.Level != 4 {
			t.Fatal("split child inherited frost or lost level")
		}
	}
}
func TestIceSkillsSendOnlyRenderingState(t *testing.T) {
	s := iceArrowCase(t, "frost")
	s.fireTower(0)
	w := s.IceWalls[0]
	s.SkillRNG = 2
	s.crossIceWalls(&s.Enemies[1], w.Progress-1, w.Progress+1)
	found := false
	for _, event := range s.Events {
		if event.Type == "frostbound" {
			found = true
			if event.Target != nil || event.IceWallID != w.ID {
				t.Fatal("status event repeats enemy state or loses wall identity")
			}
		}
	}
	if !found {
		t.Fatal("no frost event")
	}
	m, err := s.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := view(m, 0)
	raw, _ := json.Marshal(v)
	var output struct {
		Walls []map[string]json.RawMessage `json:"iceWalls"`
	}
	json.Unmarshal(raw, &output)
	if len(output.Walls) != 1 {
		t.Fatal("missing current wall")
	}
	for _, key := range []string{"route", "progress", "owner", "towerId"} {
		if _, ok := output.Walls[0][key]; ok {
			t.Fatal("save-only wall field on wire", key)
		}
	}
	for i := range s.Events {
		if s.Events[i].Type == "frostbound" {
			s.Events[i].TargetID = s.EnemySerial + 1
		}
	}
	if _, err = s.Envelope(); err == nil {
		t.Fatal("unknown enemy in compact frost event accepted")
	}
}
