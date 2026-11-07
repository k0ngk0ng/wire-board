package starfort_test

// Legacy layout regressions use captured v1.36.1 geometry; current central
// layout and skills are verified separately through production starts.

// Failure cases enumerated before objective implementation: accepting twice
// double-charges gold; unauthorized/battle actions activate objectives; an
// escort is a checkbox instead of moving; its position changes on reconnect or
// pause; enemies cannot damage the defended location; any unrelated base leak
// fails a distant objective; destruction still grants rewards; convoy delivery
// pays every tick or overheals the base; unfinished escorts silently win when
// the match ends. Module snapshots isolate those spatial/transaction boundaries;
// complete natural worlds and room persistence are covered by world E2E.

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

type objectiveSnapshot struct {
	ID, Kind, Status, Route   string
	HP, MaxHP, X, Y, Progress float64
}

func objectivePublic(b *rulesTable) []objectiveSnapshot {
	b.t.Helper()
	var v struct {
		World struct{ Objectives []objectiveSnapshot }
	}
	if err := json.Unmarshal(b.s.Data, &v); err != nil {
		b.t.Fatal(err)
	}
	return v.World.Objectives
}
func addObjectiveFixture(b *rulesTable, kind, status string, hp float64) {
	b.t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(b.s.Data, &raw); err != nil {
		b.t.Fatal(err)
	}
	v := combatPublic(b)
	r := v.Map.Routes[0]
	start := r.Points[0]
	cost, reward := 120, 90
	if kind == "beacon" {
		cost, reward = 160, 1
	}
	if kind == "convoy" {
		cost, reward = 100, 60
	}
	raw["world"].(map[string]any)["objectives"] = []any{map[string]any{"id": kind + "-fixture", "kind": kind, "label": "测试支线", "description": "守护实际位置", "sector": 1, "x": math.Max(0, start.X), "y": math.Max(0, start.Y), "route": r.ID, "progress": 0., "hp": hp, "maxHP": hp, "cost": cost, "reward": reward, "status": status, "startWave": 1, "endWave": 1, "baseAtStart": 30}}
	b.s.Data = rulesJSON(raw)
	b.record(map[string]any{"op": "objective_fixture", "match": b.s})
	b.validate()
}
func TestStarfortWorldObjectiveConvoyTravelAndDelivery(t *testing.T) {
	b := newLegacyRulesTable(t, 1, map[string]any{})
	addObjectiveFixture(b, "convoy", "available", 100)
	var raw map[string]any
	_ = json.Unmarshal(b.s.Data, &raw)
	raw["baseHP"] = 26
	b.s.Data = rulesJSON(raw)
	b.action(0, map[string]any{"type": "objective", "objectiveId": "convoy-fixture"}, true)
	b.action(0, map[string]any{"type": "objective", "objectiveId": "convoy-fixture"}, false)
	if b.view(0).Players[0].Gold != 200 {
		t.Fatal("convoy payment duplicated or omitted")
	}
	b.allReady()
	start := objectivePublic(b)[0]
	b.timer(time.Second)
	current := objectivePublic(b)[0]
	if current.Progress <= start.Progress || current.X == start.X && current.Y == start.Y || current.Route != start.Route {
		t.Fatalf("convoy did not move on persistent route: %+v", current)
	}
	// Decode/re-encode a saved match and compare exactly one server tick.
	restored := b.s
	_ = json.Unmarshal(rulesJSON(b.s), &restored)
	ctx := b.context(-1)
	ctx.Now = ctx.Now.Add(time.Second)
	a, ea := b.m.OnTimer(ctx, b.s)
	c, ec := b.m.OnTimer(ctx, restored)
	if ea != nil || ec != nil || string(rulesJSON(a)) != string(rulesJSON(c)) {
		t.Fatalf("escort reconnect continuity: %v/%v", ea, ec)
	}
	for sec := 0; sec < 60 && objectivePublic(b)[0].Status == "active"; sec++ {
		b.timer(time.Second)
	}
	done := objectivePublic(b)[0]
	if done.Status != "completed" || b.view(0).BaseHP != 30 || b.view(0).Players[0].Gold != 260 {
		t.Fatalf("delivery reward/repair failed: %+v hp=%d gold=%d", done, b.view(0).BaseHP, b.view(0).Players[0].Gold)
	}
	gold := b.view(0).Players[0].Gold
	b.timer(time.Second)
	if b.view(0).Players[0].Gold != gold {
		t.Fatal("delivery repaid on next tick")
	}
}
func TestStarfortWorldObjectiveLocalDamageAndFailure(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "slime"}, {"kind": "slime"}}, false)
	addObjectiveFixture(b, "mine", "active", 2)
	gold := b.view(0).Players[0].Gold
	b.timer(time.Second)
	o := objectivePublic(b)[0]
	if o.Status != "failed" || math.Abs(o.HP) > 1e-9 || b.view(0).Players[0].Gold != gold {
		t.Fatalf("local objective destruction failed or rewarded: %+v", o)
	}
	b.timer(time.Second)
	if objectivePublic(b)[0].Status != "failed" {
		t.Fatal("destroyed objective revived")
	}
}
func TestStarfortWorldObjectiveMineIgnoresDistantBaseLeak(t *testing.T) {
	b := combatFixture(t, []map[string]any{{"kind": "slime"}}, false)
	addObjectiveFixture(b, "mine", "active", 120)
	var raw map[string]any
	_ = json.Unmarshal(b.s.Data, &raw)
	raw["baseHP"] = 29
	enemies := raw["enemies"].([]any)
	e := enemies[0].(map[string]any)
	// Enemy ends its own path away from the objective and leaks into the base.
	v := combatPublic(b)
	route := v.Map.Routes[0]
	length := 0.
	for i := 1; i < len(route.Points); i++ {
		a, z := route.Points[i-1], route.Points[i]
		length += math.Hypot(z.X-a.X, z.Y-a.Y)
	}
	e["progress"] = length - 1
	e["x"] = route.Points[len(route.Points)-1].X - 1
	e["y"] = route.Points[len(route.Points)-1].Y
	b.s.Data = rulesJSON(raw)
	gold := b.view(0).Players[0].Gold
	b.timer(time.Second)
	o := objectivePublic(b)[0]
	if o.Status != "completed" || o.HP != 120 || b.view(0).Players[0].Gold-gold != 90+43 {
		t.Fatalf("distant leak incorrectly failed objective: %+v gold=%d", o, b.view(0).Players[0].Gold-gold)
	}
}

// Regression scenario before the position fix: a road along the southern map
// edge receives a +Y landmark offset and places the beacon outside the board.
func TestStarfortWorldObjectiveLocationsBounded(t *testing.T) {
	b := newLegacyRulesTable(t, 1, map[string]any{"waves": 20})
	seen := 0
	for sec := 0; sec < 8000 && b.s.Result == nil; sec++ {
		b.botPass()
		if b.s.Result == nil {
			b.timer(time.Second)
		}
		value, err := b.m.View(b.s, 0)
		if err != nil {
			t.Fatal(err)
		}
		var v struct {
			Map   struct{ Width, Height float64 }
			World struct{ Objectives []objectiveSnapshot }
		}
		if err = json.Unmarshal(rulesJSON(value), &v); err != nil {
			t.Fatal(err)
		}
		for _, o := range v.World.Objectives {
			if o.X < 0 || o.Y < 0 || o.X > v.Map.Width || o.Y > v.Map.Height {
				t.Fatalf("objective %s outside %gx%g: (%g,%g)", o.ID, v.Map.Width, v.Map.Height, o.X, o.Y)
			}
		}
		seen = max(seen, len(v.World.Objectives))
		if seen == 3 {
			return
		}
	}
	t.Fatalf("did not reach all objective sectors: %d", seen)
}

// Mine economy failures, enumerated before the cost fix: successful solo
// mining loses money; multiplayer costs unexpectedly change; the displayed
// price differs from the deducted amount. Compare identical wave completions
// with/without the objective so ordinary kill/wave rewards cannot hide a loss.
func TestStarfortWorldMineInvestmentProfit(t *testing.T) {
	for _, players := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			b := newLegacyRulesTable(t, players, map[string]any{"waves": 20})
			worldUntil(t, b, 6)
			for _, kind := range []string{"route", "boon"} {
				v := worldRead(t, b)
				choice := v.World.Choice
				if kind == "boon" {
					choice = v.World.BoonChoice
				}
				for actor := 0; actor < players; actor++ {
					b.action(actor, map[string]any{"type": kind + "_vote", "choice": choice.Options[0].ID}, true)
				}
			}
			var view struct {
				World struct {
					Objectives []struct {
						ID, Kind, Description string
						Cost, Reward          int
					}
				}
			}
			public, err := b.m.View(b.s, 0)
			if err != nil {
				t.Fatal(err)
			}
			_ = json.Unmarshal(rulesJSON(public), &view)
			mine := view.World.Objectives[0]
			if mine.Kind != "mine" {
				t.Fatal("missing actual generated mine")
			}
			for donor := 1; donor < players && b.view(0).Players[0].Gold < mine.Cost; donor++ {
				v := b.view(0)
				amount := min(mine.Cost-v.Players[0].Gold, v.Players[donor].Gold)
				if amount > 0 {
					b.action(donor, map[string]any{"type": "gift", "target": 0, "amount": amount}, true)
				}
			}
			original, at := b.s, b.now
			beforeGold := b.view(0).Players[0].Gold
			clear := func() {
				b.allReady()
				var raw map[string]any
				_ = json.Unmarshal(b.s.Data, &raw)
				raw["enemies"] = []any{}
				info := raw["waveInfo"].(map[string]any)
				info["spawned"] = info["total"]
				b.s.Data = rulesJSON(raw)
				b.record(map[string]any{"op": "objective_wave_completion_fixture", "match": b.s})
				b.timer(time.Second)
			}
			clear()
			baseline := b.view(0).Players
			b.s, b.now = original, at
			b.action(0, map[string]any{"type": "objective", "objectiveId": mine.ID}, true)
			charged := beforeGold - b.view(0).Players[0].Gold
			clear()
			completed := objectivePublic(b)[0]
			if completed.Status != "completed" {
				t.Fatalf("mine not completed: %+v", completed)
			}
			after := b.view(0).Players
			net := 0
			for actor := range after {
				net += after[actor].Gold - baseline[actor].Gold
			}
			if net <= 0 {
				t.Fatalf("successful %d-player mine loses gold: charged=%d rewardEach=%d totalNet=%d", players, charged, mine.Reward, net)
			}
			expected := min(120, 60*players)
			if mine.Cost != expected || charged != expected || mine.Reward != 90 || !strings.Contains(mine.Description, fmt.Sprintf("支付 %d 金币", expected)) || !strings.Contains(mine.Description, fmt.Sprintf("每人获得 %d 金币", mine.Reward)) {
				t.Fatalf("price/reward/description drift: %+v charged=%d", mine, charged)
			}
			if players == 1 && after[0].Gold-(baseline[0].Gold-beforeGold) <= beforeGold {
				t.Fatal("solo mine lost money after excluding normal wave income")
			}
			worldArtifact(t, b, map[string]any{"players": players, "mineCost": mine.Cost, "rewardEach": mine.Reward, "netProfit": net, "normalWaveIncomeExcluded": true})
		})
	}
}
