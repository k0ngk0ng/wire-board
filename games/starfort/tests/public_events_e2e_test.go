package starfort_test

// Failure inventory is frozen in PUBLIC_EVENTS.md before implementation.
// Only public Module actions, views, timers and serialized saves are exercised.
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

func prePublicModule() gameapi.Module {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		v, e := start(ctx, raw)
		if e != nil {
			return v, e
		}
		var s starfort.State
		_ = json.Unmarshal(v.Data, &s)
		s.RevisionRules = 2
		return s.Envelope()
	}
	return m
}
func publicBoundary(t *testing.T, kind, branch string, level int, kinds ...string) *rulesTable {
	b := latestBoundary(t, kind, branch, level, kinds...)
	s := cadenceState(b)
	s.RevisionRules = 3
	cadenceSave(b, s)
	return b
}
func publicLedger(s starfort.State, kind string, earned []int, settled bool) starfort.State {
	return revPatch(s, map[string]any{"revisionRules": 3, "publicEvent": map[string]any{"wave": s.Wave, "kind": kind, "startedMS": s.ElapsedMS, "earned": earned, "settled": settled}})
}
func TestStarfortPublicVersionAndLegacy(t *testing.T) {
	b := newModuleRulesTable(t, 2, map[string]any{}, starfort.Module())
	if b.s.Schema != 14 || specializationRawView(b)["revisionRules"] != float64(3) {
		t.Fatal("schema14/revision3 required", b.s.Schema)
	}
	old := newModuleRulesTable(t, 2, map[string]any{}, prePublicModule())
	if old.s.Schema != 13 {
		t.Fatal(old.s.Schema)
	}
	before := string(old.s.Data)
	old.validate()
	old.view(0)
	if before != string(old.s.Data) {
		t.Fatal("legacy mutated")
	}
	bad := b.s
	bad.Schema = 13
	if _, e := b.m.View(bad, 0); e == nil {
		t.Fatal("schema mismatch accepted")
	}
}
func TestStarfortPublicSharedProbabilityAndStableRestore(t *testing.T) {
	counts := map[string]int{}
	b := newModuleRulesTable(t, 1, map[string]any{"mode": "endless"}, starfort.Module())
	for wave := 1; wave <= 500; wave++ {
		s := cadenceState(b)
		s.World.Objectives = nil
		s.World.Choice = nil
		s.World.BoonChoice = nil
		if e := fixtureClearedWave(b.context(0), &s, wave, true); e != nil {
			t.Fatal(e)
		}
		cadenceSave(b, s)
		b.action(0, map[string]any{"type": "ready"}, true)
		v := specializationRawView(b)
		event, ok := v["publicEvent"].(map[string]any)
		if !ok {
			t.Fatal("missing ledger")
		}
		counts[event["kind"].(string)]++
		before := string(b.s.Data)
		b.validate()
		b.view(0)
		if before != string(b.s.Data) {
			t.Fatal("view rerolled")
		}
	}
	if counts["tax"] < 45 || counts["grant"] < 45 || counts["none"] < 300 || counts["none"] > 400 {
		t.Fatal("single shared30% roll", counts)
	}
	t.Logf("500 waves: %v", counts)
}
func TestStarfortPublicSettlementAndDebt(t *testing.T) {
	for _, kind := range []string{"tax", "grant", "none"} {
		b := publicBoundary(t, "arrow", "", 1, "slime")
		s := cadenceState(b)
		s.Towers = nil
		s.Enemies = nil
		s.WaveInfo.Spawned = s.WaveInfo.Total
		s.World.Objectives = nil
		s.Geology = nil
		s.Players[0].Gold = 3
		s = publicLedger(s, kind, []int{107}, false)
		cadenceSave(b, s)
		base := 40 + s.Wave*3
		want := 3 + base
		income := 107 + base
		if kind == "tax" {
			want -= income * 20 / 100
		}
		if kind == "grant" {
			want += income * 30 / 100
		}
		b.timer(time.Second)
		after := cadenceState(b)
		if after.Players[0].Gold != want {
			t.Fatal(kind, after.Players[0].Gold, want)
		}
		data := string(b.s.Data)
		b.validate()
		b.view(0)
		if data != string(b.s.Data) {
			t.Fatal("restore changed settlement")
		}
		b.timer(time.Second)
		if cadenceState(b).Players[0].Gold != want {
			t.Fatal("double settlement")
		}
	}
}
func TestStarfortPublicTaskBorrowingAndEarlyCompletion(t *testing.T) {
	for _, kind := range []string{"mine", "convoy", "merchant"} {
		b := newModuleRulesTable(t, 1, map[string]any{}, starfort.Module())
		s := cadenceState(b)
		s.Players[0].Gold = 0
		s.World.Choice = nil
		s.World.BoonChoice = nil
		r := s.World.Map.Routes[0]
		p := fixturePathLength(r.Points) * .45
		x, y := fixturePathPosition(r.Points, p)
		s.World.Objectives = []starfort.Objective{{ID: "borrow", Kind: kind, Label: kind, Status: "available", HP: 5, MaxHP: 5, Route: r.ID, Sector: 1, Progress: p, X: x, Y: y, Cost: 200, Reward: 220, Owner: -1}}
		cadenceSave(b, s)
		v := specializationRawView(b)
		controls := v["controls"].(map[string]any)["objectives"].([]any)
		if !controls[0].(map[string]any)["enabled"].(bool) {
			t.Fatal("zero gold cannot borrow")
		}
		b.action(0, map[string]any{"type": "objective", "objectiveId": "borrow"}, true)
		s = cadenceState(b)
		if s.Players[0].Gold != -200 {
			t.Fatal("debt", s.Players[0].Gold)
		}
		s.World.Objectives = append(s.World.Objectives, starfort.Objective{ID: "again", Kind: "beacon", Label: "again", Status: "available", HP: 7, MaxHP: 7, Route: r.ID, Sector: 1, Progress: p, X: x, Y: y, Cost: 80, Reward: 80, Owner: -1})
		cadenceSave(b, s)
		b.action(0, map[string]any{"type": "objective", "objectiveId": "again"}, false)
		b.validate()
		s = cadenceState(b)
		s.World.Objectives = s.World.Objectives[:1]
		s.Phase = "battle"
		s.NextAt = 0
		s.WaveInfo.Spawned = s.WaveInfo.Total
		s.Enemies = nil
		s = publicLedger(s, "tax", []int{0}, false)
		cadenceSave(b, s)
		b.timer(time.Second)
		s = cadenceState(b)
		if kind == "merchant" {
			if s.World.Objectives[0].Status != "active" || s.World.Shop != nil || s.Phase != "battle" {
				t.Fatal("merchant completed before arrival")
			}
			continue
		}
		// Task 220 is excluded from tax; ordinary clear43 is taxed by8.
		if s.Players[0].Gold != 55 || s.Phase != "prepare" {
			t.Fatal(kind, "early completion/task excluded", s.Players[0].Gold, s.Phase)
		}
	}
}
func TestStarfortPublicBossBonusAndSkillGrowth(t *testing.T) {
	b := publicBoundary(t, "arcane", "power", 4, "boss")
	s := cadenceState(b)
	s.Enemies[0].HP = 1
	s.Enemies[0].Shield = 0
	s.Enemies[0].MaxHP = 1
	s.Enemies[0].Reward = 20
	s.SkillRNG = 5
	s.World.Objectives = nil
	s = publicLedger(s, "none", []int{0}, false)
	gold := s.Players[0].Gold
	cadenceSave(b, s)
	b.timer(time.Second)
	s = cadenceState(b)
	if s.Kills < 1 || s.Players[0].Gold-gold < 30 {
		t.Fatal("boss bonus missing", s.Players[0].Gold-gold)
	}
	found := false
	for _, e := range s.Events {
		if e.Type == "death" && e.Kind == "boss" {
			found = true
			if e.Amount != 30 {
				t.Fatal("boss award", e.Amount)
			}
		}
	}
	if !found {
		t.Fatal("no boss death")
	}
	for _, tc := range []struct {
		kind, branch string
		level        int
		description  string
	}{{"arrow", "heart", 4, "6%"}, {"frost", "speed", 4, "15%"}, {"arrow", "volley", 4, "3 秒"}} {
		q := publicBoundary(t, tc.kind, tc.branch, tc.level, "slime", "slime")
		v := specializationRawView(q)
		towers := v["towers"].([]any)
		raw := fmt.Sprint(towers[0])
		if !strings.Contains(raw, tc.description) {
			t.Fatal(tc, raw)
		}
	}
	// Boundary values retain actual level ordering, not branch-relative levels.
	q := publicBoundary(t, "frost", "speed", 4, "slime")
	s = cadenceState(q)
	s.World.Event = starfort.WaveEvent{ID: "calm", Label: "晴空", Wave: s.Wave}
	cadenceSave(q, s)
	q.timer(time.Second)
	s = cadenceState(q)
	if math.Abs(s.Enemies[0].SlowFactor-.85) > 1e-9 {
		t.Fatal("ice level4 slow", s.Enemies[0].SlowFactor)
	}
}

func TestStarfortPublicHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		x, e := start(ctx, raw)
		if e != nil {
			return x, e
		}
		var s starfort.State
		_ = json.Unmarshal(x.Data, &s)
		s.Players[0].Gold = -200
		s.NextAt = ctx.Now.Add(5 * time.Minute).UnixMilli()
		s.World.Objectives = nil
		s.World.PadRewards = []starfort.PadReward{{Owner: 0, Kind: "add", Count: 1}}
		s.RewardChoosing = []int{0}
		s.RewardRemainingMS = 12345
		s.NextAt = ctx.Now.UnixMilli() + 12345
		s = publicLedger(s, "tax", []int{101, 202}, true)
		return s.Envelope()
	}
	tb := newHost(t, 2, m)
	tb.start(nil)
	a := tb.people[0]
	before := data(a.room())
	tb.restart()
	after := data(a.room())
	if after["rewardRemainingMS"] != float64(12345) || fmt.Sprint(after["rewardChoosing"]) != "[0]" || after["revisionRules"] != float64(3) || after["players"].([]any)[0].(map[string]any)["gold"] != float64(-200) || string(rulesJSON(before["publicEvent"])) != string(rulesJSON(after["publicEvent"])) {
		t.Fatal("HTTP SQLite restart lost debt/ledger")
	}
	peer := tb.people[1]
	peer.command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, "bot invests", func() bool { return len(data(a.room())["towers"].([]any)) > 0 })
	peer.command("autoplay", map[string]any{"enabled": false}, 200)
	if peer.room()["seats"].([]any)[1].(map[string]any)["autoPlay"] == true {
		t.Fatal("takeover")
	}
	tb.record("schema14 debt and public ledger persist across authenticated HTTP SQLite restart and mixed autoplay/manual takeover")
}
func TestStarfortPublicNatural(t *testing.T) { revisionNatural(t, starfort.Module()) }

func TestStarfortPublicRewardChoiceClock(t *testing.T) {
	b := newModuleRulesTable(t, 2, map[string]any{}, starfort.Module())
	s := cadenceState(b)
	s.World.Choice = nil
	s.World.BoonChoice = nil
	s.World.Objectives = nil
	s.World.PadRewards = []starfort.PadReward{{Owner: 0, Kind: "add", Count: 1}, {Owner: 1, Kind: "add", Count: 1}}
	cadenceSave(b, s)
	remaining := s.NextAt - b.now.UnixMilli()
	b.action(0, map[string]any{"type": "reward_selection", "choice": "open"}, true)
	b.action(1, map[string]any{"type": "reward_selection", "choice": "open"}, true)
	for i := 0; i < 4; i++ {
		b.timer(10 * time.Second)
		q := cadenceState(b)
		v := specializationRawView(b)
		if q.Phase != "prepare" || q.NextAt-b.now.UnixMilli() != remaining || v["rewardRemainingMS"] != float64(remaining) {
			t.Fatal("reward time consumed", q.NextAt, b.now, v)
		}
	}
	b.validate()
	b.view(0)
	b.action(0, map[string]any{"type": "reward_selection", "choice": "close"}, true)
	b.timer(5 * time.Second)
	if cadenceState(b).NextAt-b.now.UnixMilli() != remaining {
		t.Fatal("other chooser lost hold")
	}
	b.action(1, map[string]any{"type": "reward_selection", "choice": "close"}, true)
	if cadenceState(b).NextAt-b.now.UnixMilli() != remaining {
		t.Fatal("closing reset budget")
	}
	b.timer(time.Duration(remaining) * time.Millisecond)
	if cadenceState(b).Phase != "battle" {
		t.Fatal("did not resume countdown")
	}
}

// PUBLIC_EVENTS.md: settlement must use each player's earned income, even
// after spending, and retain independent rounding and negative balances.
func TestStarfortPublicPerPlayerIncomeSettlement(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want []int
	}{{"tax", []int{16, -1}}, {"grant", []int{91, 121}}} {
		b := newModuleRulesTable(t, 2, map[string]any{"mode": "endless"}, starfort.Module())
		s := cadenceState(b)
		s.Phase = "battle"
		s.NextAt = 0
		s.World.Objectives = nil
		s.World.Choice = nil
		s.World.BoonChoice = nil
		s.WaveInfo.Total = 1
		s.WaveInfo.Spawned = 1
		s.Players[0].Gold = 3
		s.Players[1].Gold = 5
		s = publicLedger(s, tc.kind, []int{107, 203}, false)
		cadenceSave(b, s)
		b.timer(time.Second)
		after := cadenceState(b)
		for i, want := range tc.want {
			if after.Players[i].Gold != want {
				t.Fatalf("%s seat%d gold%d want%d", tc.kind, i, after.Players[i].Gold, want)
			}
		}
		b.validate()
		b.view(0)
		b.timer(time.Second)
		for i, want := range tc.want {
			if cadenceState(b).Players[i].Gold != want {
				t.Fatal("duplicate settlement")
			}
		}
	}
}
