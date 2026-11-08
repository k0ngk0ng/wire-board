package pvz_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func rosterAct(t *testing.T, s *engine.State, actor int, a engine.Action) error {
	t.Helper()
	a.Epoch = s.Epoch
	a.Seq = s.Players[actor].Seq + 1
	return engine.Apply(s, actor, a)
}
func rosterRecord(t *testing.T, s engine.State, label string) {
	t.Helper()
	out := filepath.Join("..", "..", "..", ".local/pvz/open-roster")
	if e := os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": label, "digest": engine.Digest(s), "state": s}, "", "  ")
	if e := os.WriteFile(filepath.Join(out, fmt.Sprintf("%x.json", []byte(t.Name()))), raw, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestPVZOpenRosterNaturalAccessAndQuickbarE2E(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := economyStart(t, "day", n)
			if s.RulesVersion != 4 {
				t.Fatal("new match did not open its complete card pool", s.RulesVersion)
			}
			if containsBotCard(s.Players[0].Deck, "starfruit") {
				t.Fatal("fixture must use a non-quickbar card")
			}
			before := s.Players[0].Money
			if e := rosterAct(t, &s, 0, engine.Action{Type: "plant", Kind: "starfruit", Row: 0, Col: 2}); e != nil {
				t.Fatal("catalog plant unavailable", e)
			}
			if s.Players[0].Money != before-125 {
				t.Fatal("non-quickbar card bypassed price")
			}
			if e := engine.Step(&s, 200); e != nil {
				t.Fatal(e)
			}
			if e := rosterAct(t, &s, 0, engine.Action{Type: "plant", Kind: "starfruit", Row: 1, Col: 2}); e != nil {
				t.Fatal(e)
			}
			cooldown, money := s.Players[0].Cooldowns["starfruit"], s.Players[0].Money
			if e := rosterAct(t, &s, 0, engine.Action{Type: "deck", Kinds: []string{"sunflower", "cabbagepult"}, ImitateKind: "wallnut"}); e != nil {
				t.Fatal("battle quickbar update rejected", e)
			}
			if s.Players[0].Money != money || s.Players[0].Cooldowns["starfruit"] != cooldown {
				t.Fatal("quickbar update reset price/cooldown")
			}
			digest := engine.Digest(s)
			if rosterAct(t, &s, 0, engine.Action{Type: "plant", Kind: "starfruit", Row: 2, Col: 2}) == nil || engine.Digest(s) != digest {
				t.Fatal("quickbar update bypassed cooldown")
			}
			if rosterAct(t, &s, 0, engine.Action{Type: "deck", Kinds: []string{"sunflower"}, ImitateKind: "gatlingpea"}) == nil || engine.Digest(s) != digest {
				t.Fatal("non-quickbar imitater accepted an upgrade target")
			}
			if rosterAct(t, &s, 0, engine.Action{Type: "plant", Kind: "unknown", Row: 2, Col: 2}) == nil || engine.Digest(s) != digest {
				t.Fatal("unknown unit altered state")
			}
			rosterRecord(t, s, "production module start, natural money, ordinary actions; quickbar is not the available card pool")
		})
	}
}

func TestPVZOpenRosterBotUsesNonQuickbarCounterE2E(t *testing.T) {
	s := economyStart(t, "day", 2)
	if e := engine.Step(&s, 200+1800); e != nil {
		t.Fatal(e)
	}
	if e := rosterAct(t, &s, 0, engine.Action{Type: "deck", Kinds: []string{"sunflower"}}); e != nil {
		t.Fatal(e)
	}
	if e := rosterAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "balloon", Row: 0}); e != nil {
		t.Fatal(e)
	}
	visible := engine.ProjectState(s, 0)
	a := fullBotProposal(t, &visible, 0)
	if a.Kind != "blover" {
		t.Fatal("bot failed to counter a visible balloon outside its quickbar", a)
	}
	if e := engine.Apply(&s, 0, *a); e != nil {
		t.Fatal(e)
	}
	if e := engine.Step(&s, 40); e != nil {
		t.Fatal(e)
	}
	for _, z := range s.Zombies {
		if z.Kind == "balloon" && z.HP > 0 {
			t.Fatal("ordinary blover counter did not clear the balloon")
		}
	}
	rosterRecord(t, s, "production start, natural income and 90s unlock, pure projected bot proposal and ordinary counter Apply")
}

func TestPVZOpenRosterNonQuickZombieCadenceE2E(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := economyStart(t, "day", n)
			if e := engine.Step(&s, 200); e != nil {
				t.Fatal(e)
			}
			if containsBotCard(s.Players[1].Deck, "flag") {
				t.Fatal("fixture needs a non-quickbar attacker")
			}
			if e := rosterAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "flag", Row: 0}); e != nil {
				t.Fatal("catalog attacker unavailable", e)
			}
			actor := 1
			if n == 4 {
				actor = 3
			}
			before := engine.Digest(s)
			if rosterAct(t, &s, actor, engine.Action{Type: "spawn", Kind: "backupdancer", Row: 1}) == nil || engine.Digest(s) != before {
				t.Fatal("outside quickbar or teammate bypassed deployment cadence")
			}
			if e := rosterAct(t, &s, actor, engine.Action{Type: "deck", Kinds: []string{"backupdancer"}, ImitateKind: "peashooter"}); e != nil {
				t.Fatal(e)
			}
			before = engine.Digest(s)
			if rosterAct(t, &s, actor, engine.Action{Type: "spawn", Kind: "backupdancer", Row: 1}) == nil || engine.Digest(s) != before {
				t.Fatal("editing quickbar cleared shared cadence")
			}
			if e := engine.Step(&s, 59); e != nil {
				t.Fatal(e)
			}
			if rosterAct(t, &s, actor, engine.Action{Type: "spawn", Kind: "backupdancer", Row: 1}) == nil {
				t.Fatal("cadence ended early")
			}
			if e := engine.Step(&s, 1); e != nil {
				t.Fatal(e)
			}
			if e := rosterAct(t, &s, actor, engine.Action{Type: "spawn", Kind: "backupdancer", Row: 1}); e != nil {
				t.Fatal(e)
			}
			rosterRecord(t, s, "natural resources, ordinary paid deployments and battle quickbar changes; team cadence applies to all catalog cards")
		})
	}
}

func TestPVZOpenRosterUnlockAndLegacyRestoreE2E(t *testing.T) {
	s := economyStart(t, "day", 2)
	if e := engine.Step(&s, 200); e != nil {
		t.Fatal(e)
	}
	before := engine.Digest(s)
	if rosterAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "balloon", Row: 0}) == nil || engine.Digest(s) != before {
		t.Fatal("open pool bypassed aerial unlock")
	}
	if e := engine.Step(&s, 1800); e != nil {
		t.Fatal(e)
	}
	if e := rosterAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "balloon", Row: 0}); e != nil {
		t.Fatal(e)
	}
	rosterRecord(t, s, "normal game clock; flying remains unavailable until 90 seconds")
	m := pvz.Module()
	ctx := gameapi.Context{Players: 2, Actor: -1, Now: time.Unix(1800000000, 0)}
	match, e := m.Start(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	var saved struct {
		Runtime   engine.State  `json:"runtime"`
		ServerAt  int64         `json:"serverAt"`
		Remainder int64         `json:"remainder"`
		Revision  uint64        `json:"revision"`
		Applied   []pvz.Applied `json:"applied"`
	}
	if e = json.Unmarshal(match.Data, &saved); e != nil {
		t.Fatal(e)
	}
	saved.Runtime.RulesVersion = 3
	match.Data, _ = json.Marshal(saved)
	old := engine.Digest(saved.Runtime)
	ctx.Now = ctx.Now.Add(time.Hour)
	match, e = m.Restore(ctx, match)
	if e != nil {
		t.Fatal(e)
	}
	restored := fullBotRuntime(t, match)
	if engine.Digest(restored) != old {
		t.Fatal("restore upgraded an old card pool")
	}
	if rosterAct(t, &restored, 0, engine.Action{Type: "plant", Kind: "starfruit", Row: 0, Col: 2}) == nil {
		t.Fatal("old restricted card pool was silently opened")
	}
	if e = engine.Step(&restored, int(restored.PhaseEndTick-restored.Tick)); e != nil {
		t.Fatal(e)
	}
	if rosterAct(t, &restored, 0, engine.Action{Type: "deck", Kinds: []string{"sunflower"}}) == nil {
		t.Fatal("old battle deck restriction changed")
	}
}

func TestPVZOpenRosterEveryCardOrdinaryApplyE2E(t *testing.T) {
	for _, c := range engine.GetCatalog().Plants {
		t.Run("plant-"+c.Kind, func(t *testing.T) {
			scene := "day"
			if c.Mushroom || c.Kind == "gravebuster" {
				scene = "night"
			}
			if c.WaterOnly {
				scene = "pool"
				if c.Mushroom {
					scene = "fog"
				}
			}
			if c.RoofOnly {
				scene = "roof"
			}
			s := fullBotFixture(t, scene)
			s.RulesVersion = 4
			if c.UpgradeFrom != "" {
				if c.Kind == "cobcannon" {
					fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "kernelpult", Row: 1, Col: 2})
					fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "kernelpult", Row: 1, Col: 3})
				} else {
					row := 1
					if c.Kind == "cattail" {
						row = 2
					}
					fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: c.UpgradeFrom, Row: row, Col: 2})
				}
			}
			if c.Kind == "coffeebean" {
				fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "puffshroom", Row: 1, Col: 2})
			}
			s.Players[0].Deck = []string{"sunflower"}
			if c.Kind == "sunflower" {
				s.Players[0].Deck = []string{"peashooter"}
			}
			s.Players[0].ImitaterKind = "peashooter"
			found := false
			for row := 0; row < s.RowCount() && !found; row++ {
				for col := 0; col < 9; col++ {
					copy := engine.Clone(s)
					if rosterAct(t, &copy, 0, engine.Action{Type: "plant", Kind: c.Kind, Row: row, Col: col}) == nil {
						s = copy
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatal("catalog plant cannot be used outside quickbar", c.Kind)
			}
			rosterRecord(t, s, "controlled single-unit coverage, sufficient money and required bases; actual ordinary Apply with target absent from quickbar")
		})
	}
	for _, c := range engine.GetCatalog().Zombies {
		t.Run("zombie-"+c.Kind, func(t *testing.T) {
			scene := "day"
			if len(c.Scenes) > 0 {
				scene = c.Scenes[0]
			}
			if c.WaterOnly {
				scene = "pool"
			}
			s := fullBotFixture(t, scene)
			s.RulesVersion = 4
			if e := engine.Step(&s, 2400); e != nil {
				t.Fatal(e)
			}
			if c.RequiresIce {
				s.Terrain = append(s.Terrain, engine.Terrain{ID: s.NextID + 1, Kind: "ice", Owner: 1, Row: 1, FromX: 0, ToX: 10000, Until: s.Tick + 1000})
				s.NextID++
			}
			if c.Target == "cell" {
				fullBotFixtureAct(t, &s, 0, engine.Action{Type: "plant", Kind: "sunflower", Row: 1, Col: 2})
			}
			s.Players[1].Deck = []string{"zombie"}
			if c.Kind == "zombie" {
				s.Players[1].Deck = []string{"conehead"}
			}
			found := false
			for row := 0; row < s.RowCount() && !found; row++ {
				cols := 1
				if c.Target == "cell" {
					cols = 9
				}
				for col := 0; col < cols; col++ {
					copy := engine.Clone(s)
					if rosterAct(t, &copy, 1, engine.Action{Type: "spawn", Kind: c.Kind, Row: row, Col: col}) == nil {
						s = copy
						found = true
						break
					}
				}
			}
			if !found {
				t.Fatal("catalog zombie cannot be used outside quickbar", c.Kind)
			}
			rosterRecord(t, s, "controlled single-unit coverage with sufficient money, normal unlock clock and required scene/ice/target; ordinary Apply")
		})
	}
}
