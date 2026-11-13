package starfort_test

// REVISION.md freezes failure cases and ambiguous numeric interpretation first.
// Public Module, serialized Match, timer, real HTTP/SQLite and natural bots.
import (
	"encoding/json"
	"fmt"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"strings"
	"testing"
	"time"
)

func revPatch(s starfort.State, patch map[string]any) starfort.State {
	v := geoRaw(s)
	v["revisionRules"] = 1
	v["geologyRules"] = 1
	for k, x := range patch {
		v[k] = x
	}
	var out starfort.State
	_ = json.Unmarshal(rulesJSON(v), &out)
	return out
}
func revBoundary(t *testing.T, kind, branch string, level int, kinds ...string) *rulesTable {
	fixtureBranch := branch
	if fixtureBranch == "" {
		fixtureBranch = "power"
		if kind == "arrow" {
			fixtureBranch = "volley"
		}
	}
	b := v3Boundary(t, kind, fixtureBranch, max(4, level), kinds...)
	s := cadenceState(b)
	s = revPatch(s, nil)
	s.Towers[0].Level = level
	s.Towers[0].Branch = branch
	s.SkillRNG = 5
	r := s.World.Map.Routes[0]
	pad := s.World.Map.Pads[s.Towers[0].Pad]
	best, d := math.Inf(1), 0.
	for p := 0.; p < fixturePathLength(r.Points); p++ {
		x, y := fixturePathPosition(r.Points, p)
		if z := math.Hypot(x-pad.X, y-pad.Y); z < best {
			best, d = z, p
		}
	}
	for i := range s.Enemies {
		e := &s.Enemies[i]
		e.Progress = d + float64(i)*6
		e.X, e.Y = fixturePathPosition(r.Points, e.Progress)
		e.Speed = .001
		e.WeatherCheckAt = 10000
	}
	cadenceSave(b, s)
	return b
}
func TestStarfortRevisionVersion(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
	if b.s.Schema != 12 || specializationRawView(b)["revisionRules"] != float64(1) {
		t.Fatal("new schema12/revision1 required")
	}
	s := cadenceState(b)
	v := geoRaw(s)
	delete(v, "revisionRules")
	s = starfort.State{}
	_ = json.Unmarshal(rulesJSON(v), &s)
	cadenceSave(b, s)
	if b.s.Schema != 11 {
		t.Fatal("archive migrated")
	}
	before := string(b.s.Data)
	b.view(0)
	b.validate()
	if string(b.s.Data) != before {
		t.Fatal("view changed archive")
	}
	bad := b.s
	bad.Schema = 12
	if _, e := b.m.View(bad, 0); e == nil {
		t.Fatal("mismatch accepted")
	}
}
func TestStarfortRevisionStatsAndGrowth(t *testing.T) {
	for _, tc := range []struct {
		k, b            string
		l               int
		damage, rangePX float64
		interval        int
		quotes          []string
	}{{"arrow", "", 1, 22, 120, 600, nil}, {"cannon", "", 1, 48, 200, 1600, nil}, {"frost", "freeze", 4, 10 * 2.95 * .6, 193, 1785, []string{"16%", "2.5", "9.9"}}, {"frost", "speed", 4, 10 * 2.95 * .4, 193, 1116, []string{"12%", "2.0"}}, {"cannon", "power", 4, 48 * 2.95, 218, 1428, []string{"12%", "110", "22%"}}, {"cannon", "speed", 5, 48 * 3.6, 224, 1724, []string{"14%", "19.5", "6"}}, {"arrow", "volley", 4, 22 * 2.95 * .5, 138, 535, []string{"6%", "140", "22%", "50", "5 秒"}}, {"arrow", "heart", 4, 22 * 2.95 * .5, 138, 535, []string{"3%", "7%"}}, {"arcane", "magic", 4, 48 * 2.95, 213, 1071, []string{"20%", "220", "32%", "5 秒"}}} {
		t.Run(tc.k+tc.b, func(t *testing.T) {
			b := revBoundary(t, tc.k, tc.b, tc.l, "slime")
			v := b.view(0).Towers[0]
			weatherNear(t, "damage", v.EffectiveDamage, tc.damage)
			weatherNear(t, "range", v.EffectiveRange, tc.rangePX)
			if v.EffectiveInterval != tc.interval {
				t.Fatal("interval", v.EffectiveInterval, tc.interval)
			}
			if tc.b != "" {
				p := growthPassive(b)
				for _, q := range tc.quotes {
					if !strings.Contains(p["description"].(string), q) {
						t.Fatal("description", q, p)
					}
				}
			}
		})
	}
}
func TestStarfortRevisionArmorShield(t *testing.T) {
	for _, kind := range []string{"arrow", "arcane"} {
		b := revBoundary(t, kind, "", 1, "shield")
		s := cadenceState(b)
		s.Enemies[0].Armor = .55
		s.Enemies[0].Shield = 800000
		cadenceSave(b, s)
		b.timer(time.Second)
		e := cadenceState(b).Enemies[0]
		if kind == "arcane" {
			weatherNear(t, "mage hp bypass", 1000000-e.HP, 48*1.2)
			weatherNear(t, "shield retained", e.Shield, 800000)
		} else {
			weatherNear(t, "arrow armor80", 800000-e.Shield, 22*.8)
		}
	}
}
func TestStarfortRevisionUpgradePrice(t *testing.T) {
	for _, tc := range []struct {
		level  int
		factor float64
	}{{8, 1}, {9, 1.1}, {10, 1.1}, {18, 1.1}, {19, 1.1 * 1.2}, {20, 1.15}, {29, 1.15 * 1.3}} {
		b := revBoundary(t, "cannon", "power", tc.level, "slime")
		s := cadenceState(b)
		s.Players[0].Gold = 100000
		cadenceSave(b, s)
		base := math.Round(140 * (.7 + .38*float64(tc.level-1)))
		want := int(math.Round(base * tc.factor))
		if b.view(0).Controls.Upgrades[0].Cost != want {
			t.Fatal("quote", tc, b.view(0).Controls.Upgrades[0].Cost, want)
		}
		b.action(0, map[string]any{"type": "upgrade", "towerId": s.Towers[0].ID}, true)
		if cadenceState(b).Players[0].Gold != 100000-want {
			t.Fatal("debit")
		}
	}
}
func TestStarfortRevisionDisastersAndTasks(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	counts := map[string]int{}
	tasks := 0
	seen := map[string]bool{}
	for wave := 1; wave <= 300; wave++ {
		s := cadenceState(b)
		s.World.Objectives = nil
		if e := fixtureClearedWave(b.context(0), &s, wave, true); e != nil {
			t.Fatal(e)
		}
		if len(s.World.Objectives) > 1 {
			t.Fatal("multiple task rolls")
		}
		for _, o := range s.World.Objectives {
			tasks++
			seen[o.Kind] = true
			if o.Kind == "mine" && o.HP != 10 {
				t.Fatal("mine hp", o)
			}
			if o.Kind == "beacon" && o.HP != 7 {
				t.Fatal("beacon hp", o)
			}
		}
		s.Phase = "prepare"
		s.World.PadReward = nil
		cadenceSave(b, s)
		b.action(0, map[string]any{"type": "ready"}, true)
		s = cadenceState(b)
		g := geoRaw(s)["geology"].(map[string]any)
		kind := g["kind"].(string)
		counts[kind]++
		zs, _ := g["zones"].([]any)
		if kind == "flood" {
			if len(zs) < 2 || len(zs) > 6 {
				t.Fatal("flood count", len(zs))
			}
			for i, raw := range zs {
				z := raw.(map[string]any)
				life := z["expiresMS"].(float64) - z["createdMS"].(float64)
				if life < 15000 || life > 40000 {
					t.Fatal("flood life", life)
				}
				for _, prior := range zs[:i] {
					q := prior.(map[string]any)
					if z["x"].(float64) < q["x"].(float64)+q["width"].(float64) && z["x"].(float64)+z["width"].(float64) > q["x"].(float64) && z["y"].(float64) < q["y"].(float64)+q["height"].(float64) && z["y"].(float64)+z["height"].(float64) > q["y"].(float64) {
						t.Fatal("overlap")
					}
				}
			}
		}
		if kind == "typhoon" {
			if len(zs) != 20 {
				t.Fatal("wind plan count")
			}
			for _, raw := range zs {
				z := raw.(map[string]any)
				if z["speed"].(float64) < 20 || z["speed"].(float64) > 70 || z["radius"].(float64) < 30 || z["radius"].(float64) > 175 {
					t.Fatal("wind bounds")
				}
			}
		}
		before := string(b.s.Data)
		b.view(0)
		b.validate()
		if before != string(b.s.Data) {
			t.Fatal("reroll")
		}
	}
	if counts["none"] < 110 || counts["none"] > 190 || tasks < 110 || tasks > 190 || len(seen) != 4 {
		t.Fatal("shared50", counts, tasks, seen)
	}
	t.Log(counts, tasks)
}
func TestStarfortRevisionWetAndCrater(t *testing.T) {
	b := revBoundary(t, "frost", "freeze", 4, "slime")
	s := cadenceState(b)
	s = revPatch(s, map[string]any{"enemies": []any{func() map[string]any {
		e := geoRaw(s)["enemies"].([]any)[0].(map[string]any)
		e["wetUntil"] = 25
		return e
	}()}})
	s.SkillRNG = 5
	cadenceSave(b, s)
	b.timer(time.Second)
	e := cadenceState(b).Enemies[0]
	if e.FrozenUntil-e.RecoveryUntil >= 0 || e.FrozenUntil != 5+18 {
		t.Fatal("wet freeze 3.5 seconds rounded", e)
	}
	b = revBoundary(t, "frost", "speed", 4, "slime")
	s = cadenceState(b)
	enemy := geoRaw(s)["enemies"].([]any)[0].(map[string]any)
	enemy["wetUntil"] = 25
	s = revPatch(s, map[string]any{"enemies": []any{enemy}})
	cadenceSave(b, s)
	b.timer(time.Second)
	weatherNear(t, "wet slow", cadenceState(b).Enemies[0].SlowFactor, .68)
	b = revBoundary(t, "cannon", "speed", 5, "slime")
	s = cadenceState(b)
	s.SkillRNG = 5
	pos := s.Enemies[0]
	s = revPatch(s, map[string]any{"geology": map[string]any{"wave": s.Wave, "kind": "earthquake", "triggered": true, "zones": []any{map[string]any{"id": 1, "kind": "earthquake", "x": pos.X, "y": pos.Y, "radius": 75, "createdMS": 0}}, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if len(s.Craters) != 1 {
		t.Fatal("crater missing")
	}
	weatherNear(t, "quake work", s.Craters[0].Total, 6*1.3)
}
func TestStarfortRevisionPowerAndShop(t *testing.T) {
	b := revBoundary(t, "arrow", "heart", 4, "slime")
	s := cadenceState(b)
	s.Phase = "prepare"
	s.Wave = 20
	s.CompletedWaves = 20
	s.World.PreparedWave = 20
	s.World.TaskOffersWave = 20
	s.World.Objectives = nil
	s.Enemies = nil
	s.World.Shop = &starfort.MerchantShop{Wave: 20, DamageTowers: []int{}, RangeTowers: []int{}}
	s.BaseHP = 26
	s.Players[0].Gold = 5000
	cadenceSave(b, s)
	raw := specializationRawView(b)["controls"].(map[string]any)["shop"].(map[string]any)
	if raw["unitCost"] != float64(200) {
		t.Fatal("wave shop cost", raw)
	}
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, true)
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, false)
	b.action(0, map[string]any{"type": "use_invulnerability"}, true)
	b.action(0, map[string]any{"type": "use_invulnerability"}, false)
	s = cadenceState(b)
	if geoRaw(s)["invulnerableUntil"] != float64(50) {
		t.Fatal("10sec deadline")
	}
	b.action(0, map[string]any{"type": "shop", "choice": "repair", "amount": 2}, true)
	if cadenceState(b).Players[0].Gold != 3600 {
		t.Fatal("price/repair debit")
	}
}
func TestStarfortRevisionDeferredRewards(t *testing.T) {
	b := newModuleRulesTable(t, 2, map[string]any{}, starfort.Module())
	s := cadenceState(b)
	s.World.Objectives = nil
	s = revPatch(s, map[string]any{"world": func() map[string]any {
		w := geoRaw(s)["world"].(map[string]any)
		w["padRewards"] = []any{map[string]any{"owner": 0, "kind": "add", "count": 2}, map[string]any{"owner": 1, "kind": "highground", "count": 1}}
		return w
	}()})
	cadenceSave(b, s)
	before := len(s.World.Map.Pads)
	b.action(0, map[string]any{"type": "ready"}, true)
	b.action(1, map[string]any{"type": "ready"}, true)
	if b.view(0).Phase != "battle" {
		t.Fatal("reward blocked wave")
	}
	road := s.World.Map.Routes[0].Points[1]
	b.action(0, map[string]any{"type": "pad_reward", "x": road.X, "y": road.Y}, false) // a road point must be rejected
	b.action(1, map[string]any{"type": "pad_reward", "pad": 1}, true)
	b.action(0, map[string]any{"type": "pad_reward", "x": 60, "y": 60}, true)
	if len(cadenceState(b).World.Map.Pads) != before+1 {
		t.Fatal("deferred claim")
	}
	b.validate()
}
func TestStarfortRevisionHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		match, e := start(ctx, raw)
		if e != nil {
			return match, e
		}
		var s starfort.State
		_ = json.Unmarshal(match.Data, &s)
		s.NextAt = ctx.Now.Add(5 * time.Minute).UnixMilli()
		s.World.Objectives = nil
		s = revPatch(s, map[string]any{"invulnerabilityPacks": 1})
		s.World.PadRewards = []starfort.PadReward{{Owner: 0, Kind: "add", Count: 2}, {Owner: 0, Kind: "highground", Count: 1}}
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	a.act(map[string]any{"type": "use_invulnerability"}, 200)
	before := data(a.room())
	tb.restart()
	after := data(a.room())
	if after["revisionRules"] != float64(1) || after["invulnerableUntil"] != before["invulnerableUntil"] {
		t.Fatal("SQLite lost new state")
	}
	if string(rulesJSON(before["world"].(map[string]any)["padRewards"])) != string(rulesJSON(after["world"].(map[string]any)["padRewards"])) {
		t.Fatal("SQLite lost deferred rewards")
	}
	peer := tb.people[1]
	peer.command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, "current bot invests alongside manual deferred rewards", func() bool { return len(data(a.room())["towers"].([]any)) > 0 })
	peer.command("autoplay", map[string]any{"enabled": false}, 200)
	if peer.room()["seats"].([]any)[1].(map[string]any)["autoPlay"] == true {
		t.Fatal("manual takeover failed")
	}
	if data(a.room())["invulnerableUntil"] != before["invulnerableUntil"] {
		t.Fatal("takeover changed deadline")
	}
	tb.record("schema12 revision1 invulnerability/rewards survive HTTP SQLite restart; mixed autoplay/manual takeover preserves deadlines")
}
func TestStarfortRevisionNatural(t *testing.T) {
	for _, tc := range []struct {
		n, waves   int
		difficulty string
	}{{1, 20, "normal"}, {2, 20, "normal"}, {3, 20, "normal"}, {4, 20, "normal"}, {2, 10, "hard"}, {2, 10, "easy"}} {
		t.Run(fmt.Sprintf("%dp-%dw-%s", tc.n, tc.waves, tc.difficulty), func(t *testing.T) {
			b := newModuleRulesTable(t, tc.n, map[string]any{"waves": tc.waves, "difficulty": tc.difficulty}, starfort.Module())
			for sec := 0; sec < 22000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			if b.view(0).Phase != "won" {
				t.Fatal("natural failed", b.view(0).Phase, b.view(0).Wave)
			}
		})
	}
}
