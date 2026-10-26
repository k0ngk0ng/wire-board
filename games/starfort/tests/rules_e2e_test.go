package starfort_test

// Failure scenarios, enumerated before implementation:
// invalid options/actors/epochs mutate state; simultaneous builds double-charge;
// another seat manages a tower; gifts create gold or accept invalid recipients;
// failed upgrades/sales corrupt ownership; zero gold disappears; readiness fails
// to launch; one player's pause/resume freezes everyone; paused timers progress;
// repeated player actions advance simulation; old deadlines fast-forward an
// offline match; serialization loses enemies, IDs, cooldowns, or wave progress;
// enemies or event history grow without bound; bot actions stall; cooperative
// wins exclude teammates; losses invent a winner; endless ends at a campaign
// boundary; natural 1/2/4-player matches cannot clear 10/20 normal waves.
// These tests exercise only the Module boundary, with natural gameplay and a
// virtual server clock. No state fixtures or implementation internals are used.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type rulesPlayer struct {
	Gold       int  `json:"gold"`
	Ready      bool `json:"ready"`
	PauseVote  bool `json:"pauseVote"`
	ResumeVote bool `json:"resumeVote"`
}
type rulesTower struct {
	ID                int     `json:"id"`
	Pad               int     `json:"pad"`
	Owner             int     `json:"owner"`
	Kind              string  `json:"kind"`
	Level             int     `json:"level"`
	Branch            string  `json:"branch"`
	EffectiveRange    float64 `json:"effectiveRange"`
	EffectiveDamage   float64 `json:"effectiveDamage"`
	EffectiveInterval int     `json:"effectiveInterval"`
}
type rulesView struct {
	World struct {
		Event struct {
			ID string `json:"id"`
		} `json:"event"`
	} `json:"world"`
	Phase          string            `json:"phase"`
	Epoch          int               `json:"epoch"`
	Wave           int               `json:"wave"`
	CompletedWaves int               `json:"completedWaves"`
	BaseHP         int               `json:"baseHP"`
	BaseMaxHP      int               `json:"baseMaxHP"`
	Kills          int               `json:"kills"`
	Tick           int               `json:"tick"`
	ElapsedMS      int64             `json:"elapsedMS"`
	ServerAt       int64             `json:"serverAt"`
	NextAt         int64             `json:"nextAt"`
	Players        []rulesPlayer     `json:"players"`
	Towers         []rulesTower      `json:"towers"`
	Enemies        []json.RawMessage `json:"enemies"`
	Events         []json.RawMessage `json:"events"`
	Map            struct {
		Pads []struct {
			ID int `json:"id"`
		} `json:"pads"`
	} `json:"map"`
	Catalog []struct {
		Kind     string  `json:"kind"`
		Cost     int     `json:"cost"`
		Range    float64 `json:"range"`
		Damage   float64 `json:"damage"`
		Interval int     `json:"interval"`
	} `json:"catalog"`
	Controls struct {
		CanAct    bool `json:"canAct"`
		CanReady  bool `json:"canReady"`
		CanPause  bool `json:"canPause"`
		CanResume bool `json:"canResume"`
		Upgrades  []struct {
			TowerID     int  `json:"towerId"`
			Cost        int  `json:"cost"`
			Enabled     bool `json:"enabled"`
			NeedsBranch bool `json:"needsBranch"`
		} `json:"upgrades"`
		Sells []struct {
			TowerID int  `json:"towerId"`
			Refund  int  `json:"refund"`
			Enabled bool `json:"enabled"`
		} `json:"sells"`
	} `json:"controls"`
}

type rulesTable struct {
	t     *testing.T
	m     gameapi.Module
	s     gameapi.Match
	n     int
	now   time.Time
	trace *os.File
	dir   string
	steps int
}

func rulesJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func newRulesTable(t *testing.T, n int, options any) *rulesTable {
	t.Helper()
	return newModuleRulesTable(t, n, options, starfort.Module())
}
func newModuleRulesTable(t *testing.T, n int, options any, m gameapi.Module) *rulesTable {
	t.Helper()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := m.Configure(n, rulesJSON(options))
	if err != nil {
		t.Fatal(err)
	}
	b := &rulesTable{t: t, m: m, n: n, now: time.Unix(1900000000, 0).UTC()}
	b.s, err = m.Start(b.context(-1), raw)
	if err != nil {
		t.Fatal(err)
	}
	b.dir = filepath.Join("..", "..", "..", ".local", "starfort", "rules", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.MkdirAll(b.dir, 0755); err != nil {
		t.Fatal(err)
	}
	b.trace, err = os.Create(filepath.Join(b.dir, "replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	b.record(map[string]any{"op": "start", "players": n, "options": json.RawMessage(raw), "at": b.now.UnixMilli(), "match": b.s})
	t.Cleanup(func() {
		b.record(map[string]any{"op": "final", "match": b.s})
		_ = b.trace.Close()
		summary := map[string]any{"test": t.Name(), "passed": !t.Failed(), "steps": b.steps, "view": b.view(0), "result": b.s.Result, "command": ". scripts/env.sh && go test ./games/starfort/tests -run '^" + t.Name() + "$' -count=1 -v"}
		if err := os.WriteFile(filepath.Join(b.dir, "summary.json"), rulesJSON(summary), 0644); err != nil {
			t.Error(err)
		}
	})
	b.validate()
	return b
}
func (b *rulesTable) context(actor int) gameapi.Context {
	return gameapi.Context{Actor: actor, Players: b.n, Now: b.now}
}
func (b *rulesTable) record(v any) {
	b.t.Helper()
	if _, err := fmt.Fprintln(b.trace, string(rulesJSON(v))); err != nil {
		b.t.Fatal(err)
	}
}
func (b *rulesTable) view(actor int) rulesView {
	b.t.Helper()
	v, err := b.m.View(b.s, actor)
	if err != nil {
		b.t.Fatal(err)
	}
	var out rulesView
	if err = json.Unmarshal(rulesJSON(v), &out); err != nil {
		b.t.Fatal(err)
	}
	return out
}
func (b *rulesTable) validate() {
	b.t.Helper()
	if err := b.m.ValidateMatch(b.s, b.n); err != nil {
		b.t.Fatal(err)
	}
	v := b.view(0)
	if len(v.Players) != b.n || v.BaseHP < 0 || v.BaseHP > v.BaseMaxHP {
		b.t.Fatalf("invalid public resources: %+v", v)
	}
	for _, p := range v.Players {
		if p.Gold < 0 {
			b.t.Fatal("negative gold")
		}
	}
	if len(v.Enemies) > 96 || len(v.Events) > starfort.MaxEvents {
		b.t.Fatalf("unbounded runtime state: enemies=%d events=%d", len(v.Enemies), len(v.Events))
	}
}
func (b *rulesTable) action(actor int, a map[string]any, success bool) {
	b.t.Helper()
	if _, exists := a["epoch"]; !exists {
		a["epoch"] = b.view(actor).Epoch
	}
	before := string(rulesJSON(b.s))
	next, err := b.m.Apply(b.context(actor), b.s, rulesJSON(a))
	if string(rulesJSON(b.s)) != before {
		b.t.Fatal("Apply mutated its input")
	}
	if success && err != nil {
		b.t.Fatalf("%+v rejected: %v", a, err)
	}
	if !success && err == nil {
		b.t.Fatalf("invalid action accepted: %+v", a)
	}
	if err != nil {
		if string(rulesJSON(next)) != before {
			b.t.Fatal("failed action returned changed snapshot")
		}
	} else {
		b.s = next
		b.validate()
	}
	b.steps++
	b.record(map[string]any{"op": "action", "actor": actor, "at": b.now.UnixMilli(), "action": a, "accepted": err == nil, "sha256": fmt.Sprintf("%x", sha256.Sum256(rulesJSON(b.s)))})
}
func (b *rulesTable) timer(advance time.Duration) {
	b.t.Helper()
	b.now = b.now.Add(advance)
	if b.s.SystemDeadline == 0 || b.s.SystemDeadline > b.now.UnixMilli() {
		return
	}
	before := string(rulesJSON(b.s))
	next, err := b.m.OnTimer(b.context(-1), b.s)
	if string(rulesJSON(b.s)) != before {
		b.t.Fatal("OnTimer mutated input")
	}
	if err != nil {
		b.t.Fatalf("due timer rejected at %s: %v", b.now, err)
	}
	b.s = next
	b.steps++
	b.validate()
	b.record(map[string]any{"op": "timer", "at": b.now.UnixMilli(), "sha256": fmt.Sprintf("%x", sha256.Sum256(rulesJSON(b.s)))})
}

func TestStarfortRulesTowersAndSharedRewards(t *testing.T) {
	// Failure scenarios: a tower archetype cannot build; branch selection is
	// ignored; upgrades skip payment; bot chooses illegal specialization;
	// kill rewards depend on last-hit owner; elite/shield/boss waves never spawn.
	// Refined before behavior fixes: frost never slows, cannon only hits one
	// enemy, arcane never reaches armored/shield targets, one branch is unused.
	// Upgraded towers must expose effective stats: a catalog-only range ring
	// understates targeting range and cannot describe specialization to players.
	b := newRulesTable(t, 4, map[string]any{"waves": 20})
	kinds := []string{"arrow", "cannon", "frost", "arcane"}
	for actor, kind := range kinds {
		v := b.view(actor)
		b.action(actor, map[string]any{"type": "build", "pad": v.Map.Pads[actor].ID, "tower": kind}, true)
		tower := b.view(actor).Towers[actor]
		for _, spec := range v.Catalog {
			if spec.Kind == kind && (tower.EffectiveRange != spec.Range || tower.EffectiveDamage != spec.Damage || tower.EffectiveInterval != spec.Interval) {
				t.Errorf("base effective stats differ from catalog for %s: %+v", kind, tower)
			}
		}
		b.action(actor, map[string]any{"type": "upgrade", "towerId": tower.ID, "branch": "invalid"}, false)
	}
	seenEnemies := map[string]bool{}
	seenBranches := map[string]bool{}
	sharedReward := false
	frostSlow, cannonSplash, arcaneArmor, arcaneShield := false, false, false, false
	for sec := 0; sec < 8000 && b.s.Result == nil; sec++ {
		b.botPass()
		before := b.view(0)
		if b.s.Result == nil {
			b.timer(time.Second)
		}
		after := b.view(0)
		if after.Kills > before.Kills || after.CompletedWaves > before.CompletedWaves {
			gain := after.Players[0].Gold - before.Players[0].Gold
			for actor := 1; actor < b.n; actor++ {
				if after.Players[actor].Gold-before.Players[actor].Gold != gain {
					t.Fatal("shared reward differs by seat")
				}
			}
			if gain > 0 {
				sharedReward = true
			}
		}
		for _, raw := range after.Enemies {
			var enemy struct {
				Kind      string `json:"kind"`
				SlowUntil int    `json:"slowUntil"`
			}
			if err := json.Unmarshal(raw, &enemy); err != nil {
				t.Fatal(err)
			}
			seenEnemies[enemy.Kind] = true
			frostSlow = frostSlow || enemy.SlowUntil > after.Tick
		}
		shotKind, shotOwner, shotTick, hits := "", -1, -1, 0
		for _, raw := range after.Events {
			var event struct {
				Type   string                 `json:"type"`
				Kind   string                 `json:"kind"`
				Damage []starfort.DamageDelta `json:"damage"`
				Owner  int                    `json:"owner"`
				Tick   int                    `json:"tick"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				t.Fatal(err)
			}
			if event.Type == "shot" {
				shotKind, shotOwner, shotTick, hits = event.Kind, event.Owner, event.Tick, 0
			}
			if event.Type == "hit" && event.Owner == shotOwner && event.Tick == shotTick {
				hits += 1 + len(event.Damage)
				cannonSplash = cannonSplash || (shotKind == "cannon" && hits > 1)
				arcaneArmor = arcaneArmor || (shotKind == "arcane" && event.Kind == "armored")
				arcaneShield = arcaneShield || (shotKind == "arcane" && event.Kind == "shield")
			}
		}
		for _, tower := range after.Towers {
			if tower.EffectiveRange <= 0 || tower.EffectiveDamage <= 0 || tower.EffectiveInterval <= 0 {
				t.Errorf("effective tower stats absent: %+v", tower)
			}
			for _, spec := range after.Catalog {
				// Weather changes the baseline; upgrades must still extend it.
				baseline := spec.Range
				if after.World.Event.ID == "fog" {
					baseline *= .8
				}
				if spec.Kind == tower.Kind && tower.Level > 1 && tower.EffectiveRange <= baseline {
					t.Errorf("upgrade did not expand effective range: %+v", tower)
				}
			}
			if tower.Branch != "" {
				seenBranches[tower.Branch] = true
			}
			if tower.Level >= 4 && tower.Branch == "" {
				t.Fatal("advanced tower skipped specialization")
			}
		}
	}
	if !sharedReward {
		t.Fatal("no shared reward observed")
	}
	for _, kind := range []string{"slime", "runner", "armored", "swarm", "shield", "elite", "boss"} {
		if !seenEnemies[kind] {
			t.Errorf("missing enemy category %s; saw %v", kind, seenEnemies)
		}
	}
	if !seenBranches["power"] || !seenBranches["speed"] {
		t.Errorf("missing branch specialization: %v", seenBranches)
	}
	if !frostSlow || !cannonSplash || !arcaneArmor || !arcaneShield {
		t.Errorf("tower effects missing: slow=%v splash=%v arcaneArmor=%v arcaneShield=%v", frostSlow, cannonSplash, arcaneArmor, arcaneShield)
	}
	if b.view(0).Phase != "won" {
		t.Fatal("mixed tower party failed normal campaign")
	}
}
func (b *rulesTable) allReady() {
	b.t.Helper()
	for i := 0; i < b.n; i++ {
		if !b.view(i).Players[i].Ready {
			b.action(i, map[string]any{"type": "ready"}, true)
		}
	}
}
func (b *rulesTable) botPass() {
	b.t.Helper()
	// Match the host scheduler: exactly one action per room per second, choosing
	// the first non-idle seat in the module's rotating Actors list. Idle seats
	// must not generate wait snapshots or starve a later seat that can act.
	if b.s.Result != nil || len(b.s.Actors) == 0 {
		return
	}
	for _, actor := range b.s.Actors {
		raw, err := b.m.BotAction(b.context(actor), b.s)
		if err != nil {
			b.t.Fatalf("bot %d: %v", actor, err)
		}
		if len(raw) == 0 {
			continue
		}
		var a map[string]any
		if err = json.Unmarshal(raw, &a); err != nil {
			b.t.Fatal(err)
		}
		if a == nil || a["type"] == "wait" {
			b.t.Fatal("idle bot must return nil rather than generate wait snapshots")
		}
		b.action(actor, a, true)
		return
	}
}

func TestStarfortRulesConfiguration(t *testing.T) {
	// Corruption boundary scenarios, listed before decode refinements: unknown
	// phase, huge player gold, duplicated building pad, negative cooldown; public
	// callbacks must reject each corrupted snapshot and preserve their input.
	// Context.Players disagreement must not permit acting or stale admission.
	m := starfort.Module()
	for _, n := range []int{0, 5} {
		if _, err := m.Configure(n, nil); err == nil {
			t.Errorf("accepted %d players", n)
		}
	}
	for _, raw := range []string{`{"mode":"unknown"}`, `{"waves":0}`, `{"waves":30}`, `{"difficulty":"nightmare"}`, `{"prepSeconds":0}`, `{"unknown":true}`, `{} {}`, `null`} {
		if _, err := m.Configure(2, json.RawMessage(raw)); err == nil {
			t.Errorf("accepted invalid options %s", raw)
		}
	}
	for _, n := range []int{1, 2, 4} {
		if _, err := m.Configure(n, nil); err != nil {
			t.Error(err)
		}
	}
	b := newRulesTable(t, 2, map[string]any{})
	for i := 0; i < 2; i++ {
		b.action(i, map[string]any{"type": "build", "pad": b.view(i).Map.Pads[i].ID, "tower": "arrow"}, true)
	}
	ctx := b.context(0)
	ctx.Players = 4
	action := rulesJSON(map[string]any{"type": "wait", "epoch": b.view(0).Epoch})
	if next, err := m.Apply(ctx, b.s, action); err == nil || string(rulesJSON(next)) != string(rulesJSON(b.s)) {
		t.Error("actor context roster mismatch accepted")
	}
	if m.AcceptStale(ctx, b.s, action) {
		t.Error("stale admission accepted wrong roster size")
	}
	for _, scenario := range []string{"badPhase", "hugeGold", "duplicatePad", "badCooldown"} {
		var state map[string]any
		if err := json.Unmarshal(b.s.Data, &state); err != nil {
			t.Fatal(err)
		}
		switch scenario {
		case "badPhase":
			state["phase"] = "bogus"
		case "hugeGold":
			state["players"].([]any)[0].(map[string]any)["gold"] = 1e30
		case "duplicatePad":
			state["towers"].([]any)[1].(map[string]any)["pad"] = state["towers"].([]any)[0].(map[string]any)["pad"]
		case "badCooldown":
			state["towers"].([]any)[0].(map[string]any)["cooldown"] = -1
		}
		corrupt := b.s
		corrupt.Data = rulesJSON(state)
		before := string(rulesJSON(corrupt))
		if _, err := m.View(corrupt, 0); err == nil {
			t.Errorf("View accepted %s", scenario)
		}
		if _, err := m.Apply(b.context(0), corrupt, action); err == nil {
			t.Errorf("Apply accepted %s", scenario)
		}
		if _, err := m.OnTimer(b.context(-1), corrupt); err == nil {
			t.Errorf("OnTimer accepted %s", scenario)
		}
		if _, err := m.BotAction(b.context(0), corrupt); err == nil {
			t.Errorf("BotAction accepted %s", scenario)
		}
		if m.AcceptStale(b.context(0), corrupt, action) {
			t.Errorf("AcceptStale accepted %s", scenario)
		}
		if string(rulesJSON(corrupt)) != before {
			t.Errorf("callbacks mutated %s", scenario)
		}
	}
}

func TestStarfortRulesGovernanceAndEconomy(t *testing.T) {
	// Refinement before idle-bot change: a seat unable to afford any investment
	// must return nil,nil, allowing the host to scan another seat without writes.
	b := newRulesTable(t, 2, map[string]any{})
	v := b.view(0)
	if v.Phase != "prepare" || v.Wave != 1 || v.BaseHP <= 0 || !v.Controls.CanAct {
		t.Fatal("invalid start")
	}
	if b.view(-1).Controls.CanAct {
		t.Fatal("spectator can act")
	}
	pad := v.Map.Pads[0].ID
	for _, actor := range []int{-1, 2, 99} {
		b.action(actor, map[string]any{"type": "build", "pad": pad, "tower": "arrow"}, false)
	}
	for _, a := range []map[string]any{
		{"type": "build", "pad": -1, "tower": "arrow"}, {"type": "build", "pad": 999, "tower": "arrow"}, {"type": "build", "pad": pad, "tower": "unknown"},
		{"type": "gift", "target": 1, "amount": -1}, {"type": "gift", "target": 1, "amount": 0}, {"type": "gift", "target": 0, "amount": 1}, {"type": "gift", "target": 9, "amount": 1}, {"type": "gift", "target": 1, "amount": v.Players[0].Gold + 1},
		{"type": "upgrade", "towerId": 999}, {"type": "sell", "towerId": 999}, {"type": "unknown"}, {"type": "build", "pad": pad, "tower": "arrow", "epoch": v.Epoch - 1},
	} {
		b.action(0, a, false)
	}
	before := v.Players[0].Gold
	b.action(0, map[string]any{"type": "build", "pad": pad, "tower": "arrow"}, true)
	id := b.view(0).Towers[0].ID
	b.action(1, map[string]any{"type": "build", "pad": pad, "tower": "cannon"}, false)
	b.action(0, map[string]any{"type": "build", "pad": pad, "tower": "arrow"}, false)
	b.action(1, map[string]any{"type": "upgrade", "towerId": id}, false)
	b.action(1, map[string]any{"type": "sell", "towerId": id}, false)
	if b.view(0).Players[0].Gold >= before {
		t.Fatal("building did not spend gold")
	}
	gold := b.view(0).Players[0].Gold
	total := gold + b.view(0).Players[1].Gold
	b.action(0, map[string]any{"type": "gift", "target": 1, "amount": gold}, true)
	v = b.view(0)
	if v.Players[0].Gold != 0 || v.Players[1].Gold != total {
		t.Fatal("gift does not conserve gold/zero balance")
	}
	b.action(0, map[string]any{"type": "build", "pad": v.Map.Pads[1].ID, "tower": "arrow"}, false)
	refund := 0
	for _, s := range v.Controls.Sells {
		if s.TowerID == id && s.Enabled {
			refund = s.Refund
		}
	}
	if refund <= 0 {
		t.Fatal("sell not offered")
	}
	b.action(0, map[string]any{"type": "sell", "towerId": id}, true)
	if len(b.view(0).Towers) != 0 || b.view(0).Players[0].Gold != refund {
		t.Fatal("sale refund/cleanup incorrect")
	}
	oldEpoch := b.view(0).Epoch
	b.allReady()
	if b.view(0).Phase != "battle" {
		t.Fatal("unanimous ready did not launch")
	}
	if raw, err := b.m.BotAction(b.context(0), b.s); err != nil || len(raw) != 0 {
		t.Fatalf("idle zero-tower seat must yield without action, got %s / %v", raw, err)
	}
	if raw, err := b.m.BotAction(b.context(1), b.s); err != nil || len(raw) == 0 {
		t.Fatalf("funded teammate must still have an action, got %s / %v", raw, err)
	}
	b.action(0, map[string]any{"type": "build", "pad": pad, "tower": "arrow", "epoch": oldEpoch}, false)
	b.action(0, map[string]any{"type": "pause"}, false)
}

func TestStarfortRulesPauseClockAndSnapshot(t *testing.T) {
	// A human teammate must be able to pause/resume with autoplay seats: bots
	// must honor an existing vote before spending gold or changing readiness.
	// Fully paused bots with no resume vote must produce no action or writes.
	b := newRulesTable(t, 2, map[string]any{})
	b.action(0, map[string]any{"type": "pause"}, true)
	if b.view(0).Phase == "paused" {
		t.Fatal("minority paused match")
	}
	botVote := func(want string) {
		t.Helper()
		raw, err := b.m.BotAction(b.context(1), b.s)
		if err != nil {
			t.Fatal(err)
		}
		var action map[string]any
		if err = json.Unmarshal(raw, &action); err != nil {
			t.Fatal(err)
		}
		if action["type"] != want {
			t.Fatalf("bot must follow %s vote before investment/readiness, got %s", want, raw)
		}
		b.action(1, action, true)
	}
	botVote("pause")
	if b.view(0).Phase != "paused" || b.s.SystemDeadline != 0 {
		t.Fatal("unanimous pause failed")
	}
	for actor := 0; actor < b.n; actor++ {
		if raw, err := b.m.BotAction(b.context(actor), b.s); err != nil || len(raw) != 0 {
			t.Fatalf("paused bot %d must remain idle: %s / %v", actor, raw, err)
		}
	}
	before := string(rulesJSON(b.s))
	b.now = b.now.Add(time.Hour)
	next, err := b.m.OnTimer(b.context(-1), b.s)
	if err == nil && string(rulesJSON(next)) != before {
		t.Fatal("paused timer advanced")
	}
	b.action(0, map[string]any{"type": "resume"}, true)
	if b.view(0).Phase != "paused" {
		t.Fatal("minority resumed match")
	}
	botVote("resume")
	if b.view(0).Phase != "prepare" || b.view(0).NextAt <= b.now.UnixMilli() {
		t.Fatal("resume failed to reset clock")
	}
	b.allReady()
	v := b.view(0)
	for i := 0; i < 12; i++ {
		b.action(i%2, map[string]any{"type": "gift", "target": 1 - i%2, "amount": 1}, true)
	}
	if b.view(0).Tick != v.Tick || b.view(0).ElapsedMS != v.ElapsedMS {
		t.Fatal("actions advanced server clock")
	}
	b.timer(time.Hour)
	if elapsed := b.view(0).ElapsedMS - v.ElapsedMS; elapsed < 0 || elapsed > 1000 {
		t.Fatalf("offline catch-up %dms", elapsed)
	}
	for i := 0; i < 5; i++ {
		b.timer(time.Second)
	}
	var restored gameapi.Match
	if err = json.Unmarshal(rulesJSON(b.s), &restored); err != nil {
		t.Fatal(err)
	}
	ctx := b.context(-1)
	ctx.Now = ctx.Now.Add(time.Second)
	a, ea := b.m.OnTimer(ctx, b.s)
	c, ec := b.m.OnTimer(ctx, restored)
	if ea != nil || ec != nil || !reflect.DeepEqual(a, c) {
		t.Fatalf("snapshot continuity lost: %v / %v", ea, ec)
	}
	if b.m.AcceptStale == nil {
		t.Fatal("simultaneous action protocol absent")
	}
	current := b.view(0)
	if !b.m.AcceptStale(b.context(0), b.s, rulesJSON(map[string]any{"type": "gift", "target": 1, "amount": 1, "epoch": current.Epoch})) {
		t.Fatal("same-epoch simultaneous action denied")
	}
	if b.m.AcceptStale(b.context(0), b.s, rulesJSON(map[string]any{"type": "gift", "target": 1, "amount": 1, "epoch": current.Epoch - 1})) {
		t.Fatal("stale epoch accepted")
	}
}

func TestStarfortRulesNaturalCampaigns(t *testing.T) {
	// Difficulty variants must terminate through gameplay, with normal/easy
	// victory and hard capable of surviving at least the first elite wave.
	type scenario struct {
		players, waves int
		difficulty     string
	}
	scenarios := []scenario{}
	for _, players := range []int{1, 2, 3, 4} {
		for _, waves := range []int{10, 20} {
			scenarios = append(scenarios, scenario{players, waves, "normal"})
		}
	}
	scenarios = append(scenarios, scenario{2, 10, "easy"}, scenario{2, 10, "hard"})
	for _, s := range scenarios {
		players, waves := s.players, s.waves
		t.Run(fmt.Sprintf("%dplayers_%dwaves_%s", players, waves, s.difficulty), func(t *testing.T) {
			b := newRulesTable(t, players, map[string]any{"mode": "campaign", "waves": waves, "difficulty": s.difficulty})
			for sec := 0; sec < 8000 && b.s.Result == nil; sec++ {
				b.botPass()
				if b.s.Result == nil {
					b.timer(time.Second)
				}
			}
			v := b.view(0)
			if s.difficulty == "hard" && v.Phase == "lost" {
				if v.CompletedWaves < 5 || b.s.Result == nil || len(b.s.Result.Winners) != 0 {
					t.Fatalf("hard mode invalid/unplayable: waves=%d result=%+v", v.CompletedWaves, b.s.Result)
				}
				return
			}
			if v.Phase != "won" || v.CompletedWaves != waves || b.s.Result == nil || len(b.s.Result.Winners) != players {
				t.Fatalf("campaign failed: phase=%s completed=%d hp=%d result=%+v", v.Phase, v.CompletedWaves, v.BaseHP, b.s.Result)
			}
			for _, rank := range b.s.Result.Ranks {
				if rank != 1 {
					t.Fatal("teammate excluded from victory")
				}
			}
		})
	}
}

func TestStarfortRulesNaturalLoss(t *testing.T) {
	b := newRulesTable(t, 2, map[string]any{"difficulty": "hard"})
	b.allReady()
	for sec := 0; sec < 2000 && b.s.Result == nil; sec++ {
		if b.view(0).Phase == "prepare" {
			b.allReady()
		}
		if b.s.Result == nil {
			b.timer(time.Second)
		}
	}
	if b.view(0).Phase != "lost" || b.view(0).BaseHP != 0 || b.s.Result == nil || len(b.s.Result.Winners) != 0 {
		t.Fatalf("loss fabricated winner or stalled: %+v", b.s.Result)
	}
}

func TestStarfortRulesNaturalEndless(t *testing.T) {
	b := newRulesTable(t, 4, map[string]any{"mode": "endless", "difficulty": "normal"})
	for sec := 0; sec < 12000 && b.s.Result == nil && b.view(0).CompletedWaves < 22; sec++ {
		b.botPass()
		if b.s.Result == nil {
			b.timer(time.Second)
		}
	}
	v := b.view(0)
	if v.CompletedWaves < 22 || b.s.Result != nil || v.Phase == "won" {
		t.Fatalf("endless failed to exceed 20 waves: phase=%s completed=%d hp=%d", v.Phase, v.CompletedWaves, v.BaseHP)
	}
}
