package pvz_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func economyStart(t *testing.T, scene string, n int) engine.State {
	t.Helper()
	ctx := gameapi.Context{Players: n, Actor: -1, Now: time.Unix(1800000000, 0)}
	raw, _ := json.Marshal(engine.Options{Scene: scene, RoundSeconds: 180, PrepSeconds: 10})
	m, e := pvz.Module().Start(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	return fullBotRuntime(t, m)
}

func TestPVZEconomyNewMatchAndTeamSupplyE2E(t *testing.T) {
	for _, q := range []struct {
		scene             string
		opening, interval int
	}{{"day", 500, 4}, {"night", 300, 8}, {"pool", 600, 4}, {"fog", 400, 8}, {"roof", 700, 3}, {"roofnight", 700, 3}} {
		for _, n := range []int{2, 4} {
			t.Run(fmt.Sprintf("%s-%d", q.scene, n), func(t *testing.T) {
				s := economyStart(t, q.scene, n)
				if s.RulesVersion != 3 {
					t.Fatal("new match did not select the balanced economy profile", s.RulesVersion)
				}
				money := balanceMoney(s)
				if money != [2]int{q.opening, 250} {
					t.Fatal("wrong scene or team opening funds", money)
				}
				if e := engine.Step(&s, 800); e != nil {
					t.Fatal(e)
				} // 10s preparation + 30s battle.
				money = balanceMoney(s)
				if money[0] != q.opening+40/q.interval*25 || money[1] != 250+15*25 {
					t.Fatal("team supply was missing, multiplied by seats or changed zombie income", money)
				}
				min, max := 1000000, 0
				for _, p := range s.Players {
					if p.Side == "plants" {
						if p.Money < min {
							min = p.Money
						}
						if p.Money > max {
							max = p.Money
						}
					}
				}
				if max-min > 1 {
					t.Fatal("unequal shared supply", s.Players)
				}
				// A new round has its own opening balance and income clock.
				for s.Round == 1 || s.Phase != "prepare" {
					if e := engine.Step(&s, 1); e != nil {
						t.Fatal(e)
					}
				}
				money = balanceMoney(s)
				if money != [2]int{250, q.opening} {
					t.Fatal("swapped opening funds differ", money)
				}
			})
		}
	}
}

func TestPVZEconomyProducerAndFormationE2E(t *testing.T) {
	for _, q := range []struct{ kind, scene string }{{"sunflower", "day"}, {"sunshroom", "night"}} {
		t.Run(q.kind, func(t *testing.T) {
			s := economyStart(t, q.scene, 2)
			a := engine.Action{Type: "plant", Kind: q.kind, Row: 0, Col: 0, Epoch: s.Epoch, Seq: 1}
			if e := engine.Apply(&s, 0, a); e != nil {
				t.Fatal(e)
			}
			first := s.Plants[0].ReadyAt
			if first < 60 || first > 120 {
				t.Fatal("first sun must arrive 3–6 seconds after planting", first)
			}
			if e := engine.Step(&s, int(first)); e != nil {
				t.Fatal(e)
			}
			next := s.Plants[0].ReadyAt - s.Tick
			if next < 360 || next > 400 {
				t.Fatal("production repeat must be 18–20 seconds", next)
			}
		})
	}
	for _, scene := range []string{"day", "night", "pool", "fog", "roof", "roofnight"} {
		for _, n := range []int{2, 4} {
			t.Run(fmt.Sprintf("formation-%s-%d", scene, n), func(t *testing.T) {
				s := economyStart(t, scene, n)
				limit := 800
				if s.Map.Roof {
					limit = 1100
				}
				for frame := 0; frame < limit; frame++ {
					if s.Tick%4 == 0 {
						for actor, p := range s.Players {
							if p.Side != "plants" {
								continue
							}
							visible := engine.ProjectState(s, actor)
							a := balanceFormation(visible, actor)
							if a != nil {
								if e := engine.Apply(&s, actor, *a); e != nil {
									t.Fatal(e)
								}
							}
						}
					}
					if e := engine.Step(&s, 1); e != nil {
						t.Fatal(e)
					}
				}
				rows := map[int]bool{}
				producers := 0
				for _, p := range s.Plants {
					if balanceShooter(p.Kind) && !p.Asleep {
						rows[p.Row] = true
					}
					if p.Kind == "sunflower" || p.Kind == "sunshroom" {
						producers++
					}
				}
				if len(rows) != s.RowCount() || producers < 2 {
					t.Fatal("natural opening cannot form basic fire on every lane and two producers", len(rows), producers, s.Players)
				}
			})
		}
	}
}

func TestPVZEconomyLegacySnapshotRemainsFrozenE2E(t *testing.T) {
	for _, scene := range []string{"day", "night", "roofnight"} {
		t.Run(scene, func(t *testing.T) {
			s, e := engine.New(engine.Options{Scene: scene, PrepSeconds: 10}, 2, "economy-legacy")
			if e != nil {
				t.Fatal(e)
			}
			s.RulesVersion = 2
			raw, _ := json.Marshal(s)
			var restored engine.State
			if e = json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			if e = engine.Step(&restored, 800); e != nil {
				t.Fatal(e)
			}
			want := 300
			if !s.Map.Night {
				want += 200
			}
			if restored.Players[0].Money != want || restored.RulesVersion != 2 {
				t.Fatal("old snapshot was silently given the new economy", restored.Players)
			}
		})
	}
}

func TestPVZEconomyStarterDeckAnswersE2E(t *testing.T) {
	for _, scene := range []string{"day", "night", "pool", "fog", "roof", "roofnight"} {
		t.Run(scene, func(t *testing.T) {
			s := economyStart(t, scene, 2)
			has := func(actor int, kind string) bool {
				for _, k := range s.Players[actor].Deck {
					if k == kind {
						return true
					}
				}
				return false
			}
			if s.Map.Roof && !has(0, "cherrybomb") {
				t.Fatal("roof starter has no affordable wave-clear answer")
			}
			if !has(1, "peahead") || !has(1, "gatlinghead") {
				t.Fatal("zombie starter has no ranged answer to a mushroom formation")
			}
			if len(s.Players[0].Deck) > 10 || len(s.Players[1].Deck) > 10 {
				t.Fatal("starter deck exceeds public limit")
			}
		})
	}
}
