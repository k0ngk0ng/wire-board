package starfort_test

// LATEST.md records the complete failure inventory before implementation.
// Exercise public Module actions, views, timers, saved Match and real HTTP/SQLite.
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

func preLatestModule() gameapi.Module {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, raw)
		if err != nil {
			return match, err
		}
		var s starfort.State
		_ = json.Unmarshal(match.Data, &s)
		s.RevisionRules = 1
		s.BaseMaxHP = 30
		return s.Envelope()
	}
	return m
}
func latestState(s starfort.State) starfort.State {
	s = revPatch(s, map[string]any{"revisionRules": 2})
	s.BaseMaxHP = 30 + s.Wave/10
	return s
}
func latestBoundary(t *testing.T, k, b string, l int, kinds ...string) *rulesTable {
	r := revBoundary(t, k, b, l, kinds...)
	cadenceSave(r, latestState(cadenceState(r)))
	return r
}
func TestStarfortLatestVersionAndHealth(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, prePublicModule())
	if b.s.Schema != 13 || specializationRawView(b)["revisionRules"] != float64(2) {
		t.Fatal("new schema13/revision2 required", b.s.Schema)
	}
	s := cadenceState(b)
	if s.BaseHP != 30 || s.BaseMaxHP != 30 {
		t.Fatal("initial health")
	}
	for _, wave := range []int{9, 10, 19, 20, 29, 30} {
		s = cadenceState(b)
		s.World.Objectives = nil
		s.World.Choice, s.World.BoonChoice = nil, nil
		s.BaseHP = 21
		if err := fixtureClearedWave(b.context(0), &s, wave-1, true); err != nil {
			t.Fatal(err)
		}
		cadenceSave(b, s)
		want := 30 + wave/10
		if s.Wave != wave || s.BaseMaxHP != want || s.BaseHP != 21 {
			t.Fatalf("wave%d health %d/%d", s.Wave, s.BaseHP, s.BaseMaxHP)
		}
		original := string(b.s.Data)
		b.view(0)
		b.validate()
		if string(b.s.Data) != original {
			t.Fatal("view migrated health")
		}
	}
	old := newModuleRulesTable(t, 1, map[string]any{}, preLatestModule())
	if old.s.Schema != 12 {
		t.Fatal("old schema")
	}
	archived := string(old.s.Data)
	old.view(0)
	old.validate()
	if string(old.s.Data) != archived {
		t.Fatal("old migrated")
	}
	bad := b.s
	bad.Schema = 12
	if _, err := b.m.View(bad, 0); err == nil {
		t.Fatal("schema mismatch accepted")
	}
	s = cadenceState(b)
	s.BaseMaxHP++
	if _, err := s.Envelope(); err == nil {
		t.Fatal("invalid health cap accepted")
	}
}
func TestStarfortLatestPerRouteRewards(t *testing.T) {
	for _, peaks := range [][]float64{{.5, .7, .71}, {.50001, .70001, .2}, {.9, .9, .9}} {
		b := latestBoundary(t, "arrow", "", 1, "slime")
		s := cadenceState(b)
		s.Towers = nil
		s.Enemies = nil
		s.Wave = 20
		s.BaseMaxHP = 32
		s.Options.Mode = "endless"
		s.WaveInfo.Total = 1
		s.WaveInfo.Spawned = 1
		s.World.Objectives = nil
		s.World.Map.Routes = nil
		values := map[string]float64{}
		want := 0
		for i, p := range peaks {
			r := starfort.Route{ID: fmt.Sprint("test-", i), Active: true, Sector: 1, Points: append([]starfort.Point{}, s.World.Map.Path...)}
			s.World.Map.Routes = append(s.World.Map.Routes, r)
			values[r.ID] = p
			if p <= .5 {
				want += 200
			} else if p <= .7 {
				want += 100
			}
		}
		// A fourth active route that never spawned must not grant free gold.
		s.World.Map.Routes = append(s.World.Map.Routes, starfort.Route{ID: "empty", Active: true, Sector: 1, Points: append([]starfort.Point{}, s.World.Map.Path...)})
		s = revPatch(s, map[string]any{"revisionRules": 2, "geology": map[string]any{"wave": 20, "kind": "none", "routePeaks": values}})
		cadenceSave(b, s)
		before := s.Players[0].Gold
		b.timer(time.Second)
		after := cadenceState(b)
		if after.Geology.Bonus != want || after.Players[0].Gold-before != 100+want {
			t.Fatal("per-road reward", after.Geology.Bonus, want, after.Players[0].Gold-before)
		}
		gold := after.Players[0].Gold
		b.view(0)
		b.timer(time.Second)
		if cadenceState(b).Players[0].Gold != gold {
			t.Fatal("double payout")
		}
	}
}
func TestStarfortLatestDisastersAndTasks(t *testing.T) {
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, prePublicModule())
	counts := map[string]int{}
	tasks := 0
	merchantCosts := map[int]bool{}
	maxQuakes := 0
	for wave := 1; wave <= 400; wave++ {
		s := cadenceState(b)
		s.World.Objectives = nil
		s.World.TaskAttempts = map[string]int{"merchant": 3}
		if err := fixtureClearedWave(b.context(0), &s, wave, true); err != nil {
			t.Fatal(err)
		}
		if s.World.Event.ID == "fog" && (!strings.Contains(s.World.Event.Description, "击退增加 20") || !strings.Contains(s.World.Event.Description, "30%")) {
			t.Fatal("fog description", s.World.Event)
		}
		if s.World.Event.ID == "starlight" && !strings.Contains(s.World.Event.Description, "增加 15 个百分点") {
			t.Fatal("starlight description", s.World.Event)
		}
		if len(s.World.Objectives) > 1 {
			t.Fatal("multiple task rolls")
		}
		for _, o := range s.World.Objectives {
			tasks++
			if o.Kind == "merchant" {
				merchantCosts[o.Cost] = true
				if o.Cost != 266 {
					t.Fatal("merchant growth", o.Cost)
				}
			}
		}
		s.World.PadReward = nil
		s.World.Choice = nil
		s.World.BoonChoice = nil
		cadenceSave(b, s)
		b.action(0, map[string]any{"type": "ready"}, true)
		s = cadenceState(b)
		g := s.Geology
		counts[g.Kind]++
		if g.Kind == "earthquake" {
			maxQuakes = max(maxQuakes, len(g.Zones))
			if len(g.Zones) < 1 || len(g.Zones) > 5 {
				t.Fatal("quake count")
			}
			for i, z := range g.Zones {
				for _, q := range g.Zones[:i] {
					if math.Hypot(z.X-q.X, z.Y-q.Y) < z.Radius+q.Radius {
						t.Fatal("overlapping quakes")
					}
				}
			}
		}
		before := string(b.s.Data)
		b.view(0)
		b.validate()
		if string(b.s.Data) != before {
			t.Fatal("reroll")
		}
	}
	if counts["none"] < 85 || counts["none"] > 155 || tasks < 245 || tasks > 315 || maxQuakes != 5 || len(merchantCosts) != 1 {
		t.Fatal("shared70", counts, tasks, maxQuakes, merchantCosts)
	}
	t.Log("shared70", counts, "tasks", tasks)
}
func TestStarfortLatestWetAndQuake(t *testing.T) {
	for _, branch := range []string{"freeze", "speed"} {
		b := latestBoundary(t, "frost", branch, 4, "slime")
		s := cadenceState(b)
		s.Enemies[0].WetUntil = 25
		s.SkillRNG = 5
		cadenceSave(b, s)
		b.timer(time.Second)
		e := cadenceState(b).Enemies[0]
		if branch == "freeze" && e.FrozenUntil != 5+23 {
			t.Fatal("wet +2s", e.FrozenUntil)
		}
		if branch == "speed" {
			weatherNear(t, "wet slow +50pp", e.SlowFactor, .38)
		}
	}
	b := latestBoundary(t, "cannon", "speed", 5, "slime")
	s := cadenceState(b)
	pos := s.Enemies[0]
	s = revPatch(s, map[string]any{"revisionRules": 2, "geology": map[string]any{"wave": s.Wave, "kind": "earthquake", "triggered": true, "zones": []any{map[string]any{"id": 1, "kind": "earthquake", "x": pos.X, "y": pos.Y, "radius": 75, "createdMS": 0}}, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if len(s.Craters) != 1 {
		t.Fatal("crater missing")
	}
	weatherNear(t, "quake work80", s.Craters[0].Total, 6*1.8)
}
func TestStarfortLatestFloodRecovery(t *testing.T) {
	b := latestBoundary(t, "cannon", "power", 4, "slime")
	s := cadenceState(b)
	s.Tick = 5
	s.ElapsedMS = 1000
	s.Towers[0].FloodedAtMS = 1000
	cadenceSave(b, s)
	base := 48 * 2.95
	for _, ms := range []int64{1000, 6000, 11000} {
		s = cadenceState(b)
		s.ElapsedMS = ms
		s.Tick = ms / 200
		cadenceSave(b, s)
		weatherNear(t, "flood linear", b.view(0).Towers[0].EffectiveDamage, base*(.7+.3*float64(ms-1000)/10000))
	}
}
func TestStarfortLatestQuakeRecoveryFloor(t *testing.T) {
	b := latestBoundary(t, "arrow", "", 1, "slime")
	s := cadenceState(b)
	s.Towers = nil
	// 30 affected towers on the same road: ten percent would be 3, capped at 2.
	pad := s.World.Map.Pads[0]
	for i := 0; i < 30; i++ {
		p := pad
		p.ID = i
		p.Route = "main"
		s.World.Map.Pads = append(s.World.Map.Pads[:min(i, len(s.World.Map.Pads))], p)
		s.Towers = append(s.Towers, starfort.Tower{ID: i + 1, Pad: i, Kind: "arrow", Level: 1, Owner: 0, Priority: "first", Spent: 90, Cooldown: 10000})
	}
	s.TowerSerial = 30
	s = revPatch(s, map[string]any{"revisionRules": 2, "geology": map[string]any{"wave": s.Wave, "kind": "earthquake", "triggerMS": 200, "zones": []any{map[string]any{"id": 1, "kind": "earthquake", "x": pad.X, "y": pad.Y, "radius": 200, "createdMS": 200}}, "routePeaks": map[string]float64{}}})
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if len(s.Geology.Disabled) != 15 {
		t.Fatal("half towers", len(s.Geology.Disabled))
	}
	for _, d := range s.Geology.Disabled {
		if d.Floor != 2 {
			t.Fatal("cap floor", d)
		}
	}
	// Three recoveries remove exactly one per road, never the retained floor.
	for i := 0; i < 15; i++ {
		b.timer(time.Second)
	}
	s = cadenceState(b)
	if len(s.Geology.Disabled) != 12 {
		t.Fatal("recoveries", len(s.Geology.Disabled))
	}
}
func TestStarfortLatestShopAndWeather(t *testing.T) {
	b := latestBoundary(t, "arcane", "magic", 4, "slime")
	s := cadenceState(b)
	s.Phase = "prepare"
	s.Wave = 20
	s.BaseMaxHP = 32
	s.CompletedWaves = 20
	s.World.PreparedWave = 20
	s.World.TaskOffersWave = 20
	s.World.Objectives = nil
	s.Enemies = nil
	s.World.Shop = &starfort.MerchantShop{Wave: 20}
	s.Players[0].Gold = 699
	cadenceSave(b, s)
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, false)
	s.Players[0].Gold = 700
	cadenceSave(b, s)
	q := specializationRawView(b)["controls"].(map[string]any)["shop"].(map[string]any)
	if q["shieldCost"] != float64(700) || q["shieldEnabled"] != true {
		t.Fatal("quote", q)
	}
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, true)
	if cadenceState(b).Players[0].Gold != 0 {
		t.Fatal("debit")
	}
	b.action(0, map[string]any{"type": "shop", "choice": "shield"}, false)
	b.action(0, map[string]any{"type": "use_invulnerability"}, true)
	for _, tc := range []struct {
		k, branch, weather string
		damage, rate       float64
		chance             int
	}{{"arrow", "heart", "fog", 22 * 2.95 * .5 * 1.3, 1, 3}, {"arcane", "power", "starlight", 48 * 2.95 * 1.2, .6, 0}, {"arcane", "magic", "starlight", 48 * 2.95 * .8, .6, 35}, {"arcane", "magic", "calm", 48 * 2.95 * .8, .6, 20}} {
		b := latestBoundary(t, tc.k, tc.branch, 4, "slime")
		s := cadenceState(b)
		s.World.Event = starfort.WaveEvent{ID: tc.weather, Label: "验收"}
		cadenceSave(b, s)
		v := b.view(0).Towers[0]
		weatherNear(t, "weather damage", v.EffectiveDamage, tc.damage)
		interval := 600.
		if tc.k == "arcane" {
			interval = 1200
		}
		if v.EffectiveInterval != int(interval/(1.12*tc.rate)) {
			t.Fatal("rate", v.EffectiveInterval)
		}
		if tc.chance > 0 && int(growthPassive(b)["triggerChance"].(float64)) != tc.chance {
			t.Fatal("chance", growthPassive(b))
		}
	}
	b = latestBoundary(t, "arrow", "volley", 4, "slime")
	s = cadenceState(b)
	s.World.Event = starfort.WaveEvent{ID: "fog", Label: "薄雾"}
	s.SkillRNG = 59
	cadenceSave(b, s)
	b.timer(time.Second)
	found := false
	for _, e := range cadenceState(b).Events {
		if e.Type == "knockback" {
			weatherNear(t, "fog push20", e.Amount, 70)
			found = true
		}
	}
	if !found {
		t.Fatal("push missing")
	}
}
func TestStarfortLatestHTTP(t *testing.T) {
	m := prePublicModule()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		x, e := start(ctx, raw)
		if e != nil {
			return x, e
		}
		var s starfort.State
		_ = json.Unmarshal(x.Data, &s)
		s.Wave = 20
		s.BaseMaxHP = 32
		s.BaseHP = 22
		s.NextAt = ctx.Now.Add(5 * time.Minute).UnixMilli()
		s.World.Objectives = nil
		s.World.PadRewards = []starfort.PadReward{{Owner: 0, Kind: "add", Count: 1}}
		s.InvulnerabilityPacks = 1
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	a.act(map[string]any{"type": "use_invulnerability"}, 200)
	before := data(a.room())
	tb.restart()
	after := data(a.room())
	if after["revisionRules"] != float64(2) || after["baseMaxHP"] != float64(32) || after["baseHP"] != float64(22) || after["invulnerableUntil"] != before["invulnerableUntil"] {
		t.Fatal("restart lost state")
	}
	peer := tb.people[1]
	peer.command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, "bot invests", func() bool { return len(data(a.room())["towers"].([]any)) > 0 })
	peer.command("autoplay", map[string]any{"enabled": false}, 200)
	if peer.room()["seats"].([]any)[1].(map[string]any)["autoPlay"] == true {
		t.Fatal("takeover")
	}
	tb.record("schema13 health, invulnerability and mixed manual/autoplay survive SQLite restart")
}
func TestStarfortLatestNatural(t *testing.T) { revisionNatural(t, prePublicModule()) }

func TestStarfortLatestInwardPads(t *testing.T) {
	for seed := int64(1); seed <= 9; seed++ {
		ctx := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(100, seed)}
		m, err := starfort.Module().Start(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		var s starfort.State
		_ = json.Unmarshal(m.Data, &s)
		r := s.World.Map.Routes[0]
		length := fixturePathLength(r.Points)
		high := 0
		if len(s.World.Map.Pads) != 6 {
			t.Fatal("six initial pads")
		}
		for i, p := range s.World.Map.Pads {
			if p.Terrain == "highground" {
				high++
			}
			x, y := fixturePathPosition(r.Points, length*float64(i+1)/7)
			qx, qy := fixturePathPosition(r.Points, length*float64(i+1)/7+1)
			// Dot against the inward normal; displacement along the road cannot hide
			// placement on the outward bank. Ignore straight radial segments (tie).
			nx, ny := -(qy - y), qx-x
			sign := nx*(s.World.Map.Base.X-x) + ny*(s.World.Map.Base.Y-y)
			if math.Abs(sign) > 1 && (nx*(p.X-x)+ny*(p.Y-y))*sign < 0 {
				t.Fatal("outward pad", seed, i, p)
			}
			for j := 1; j < len(r.Points); j++ {
				a, b := r.Points[j-1], r.Points[j]
				dx, dy := b.X-a.X, b.Y-a.Y
				f := math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/(dx*dx+dy*dy)))
				if math.Hypot(p.X-a.X-f*dx, p.Y-a.Y-f*dy) < 48 {
					t.Fatal("pad on road")
				}
			}
			for _, q := range s.World.Map.Pads[:i] {
				if math.Hypot(p.X-q.X, p.Y-q.Y) < 60 {
					t.Fatal("pad overlap")
				}
			}
		}
		if high != 1 {
			t.Fatal("one highground")
		}
	}
}
