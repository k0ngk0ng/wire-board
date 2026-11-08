package stoneage_test

import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStoneageFreeElementsHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "stoneage")
	h.Start(map[string]any{})
	a := httpSuggestion(t, h.People[0])
	l := a["loadout"].(map[string]any)
	for _, elements := range []any{[]int{}, []int{0, 0, 0, 0}, []int{11, 0, 0, 0}, []int{-1, 11, 0, 0}, []int{3, 3, 4}, []float64{2.5, 2.5, 2.5, 2.5}, []int{5, 0, 5, 0}, []int{0, 5, 0, 5}, []int{3, 3, 2, 2}} {
		l["elements"] = elements
		before := gametest.Data(h.People[0].Room())
		h.People[0].Action(a, 400)
		if !reflect.DeepEqual(before, gametest.Data(h.People[0].Room())) {
			t.Fatal("invalid elements changed state")
		}
	}
	l["elements"] = []int{7, 0, 0, 3}
	h.People[0].Action(a, 200)
	h.Restart()
	own := gametest.Data(h.People[0].Room())["self"].(map[string]any)["loadout"].(map[string]any)
	if !reflect.DeepEqual(own["elements"], []any{float64(7), float64(0), float64(0), float64(3)}) {
		t.Fatal("elements lost on restore", own)
	}
	for pass := 0; pass < 12 && httpView(t, h.People[0]).Phase != "planning"; pass++ {
		for _, client := range h.People {
			if action := httpSuggestion(t, client); action != nil {
				client.Action(action, 200)
			}
		}
	}
	players := gametest.Data(h.People[0].Room())["players"].([]any)
	body := players[0].(map[string]any)["body"].(map[string]any)
	if !reflect.DeepEqual(body["elements"], []any{float64(70), float64(0), float64(0), float64(30)}) {
		t.Fatal("element selection not used in battle", body)
	}
	h.Record("native 10-point elements and opposing-pair restriction: validation, lock, restart and actual battle values")
	report(t, "free-elements-http", h.People[0].Room())
}

func TestStoneageNativeChargeAndRestoreE2E(t *testing.T) {
	for _, id := range []int{30, 31} {
		t.Run(map[int]string{30: "charge", 31: "double-charge"}[id], func(t *testing.T) {
			m, c, s := combatFixture(t, id)
			a, b := guards(0), guards(1)
			a.Pet = game.Command{Kind: "skill", ID: id, Target: 2}
			s.Players[1].Body.Status.Stone = 10
			s.Players[1].Pet.Status.Stone = 10
			m.Data = raw(s)
			rounds := id - 29
			history := []game.State{}
			for step := 0; step <= rounds; step++ {
				end := combatRound(t, m, c, a, b)
				history = append(history, end)
				attacks := 0
				for _, e := range end.Events {
					if e.Actor == 1 && (e.Kind == "attack" || e.Kind == "miss") {
						attacks++
						if e.Target != 2 {
							t.Fatal("charge lost its committed target")
						}
					}
				}
				charge := obj(t, end.Players[0].Pet)["charge"]
				if step < rounds {
					if attacks != 0 || charge == nil {
						t.Fatal("charge attacked before its preparation rounds", step, end.Events)
					}
					if charge.(map[string]any)["remaining"] != float64(rounds-step-1) {
						t.Fatal("wrong remaining charge", charge)
					}
					v, e := game.Module().View(gameapi.Match{Schema: 1, Data: raw(end)}, 1)
					if e != nil {
						t.Fatal(e)
					}
					public := obj(t, v)
					text := string(raw(public))
					if strings.Contains(text, "\"charge\":{\"skill\":") && strings.Contains(text, "\"target\":2,\"remaining\"") {
						t.Fatal("opponent can inspect pending charge target")
					}
				} else if attacks != 1 || charge != nil || end.Players[0].Pet.TurnDefense != 100 {
					t.Fatal("charge must release exactly one hit without lowering defense", end.Events, charge)
				}
				if step < rounds {
					m = finishFixture(t, c, end)
					restored, e := game.Module().Restore(gameapi.Context{}, m)
					if e != nil {
						t.Fatal(e)
					}
					m = restored
					c.Now = gameTime(t, m)
					a.Pet = game.Command{Kind: "wait", Target: 1}
				}
			}
			report(t, "native-charge-"+map[int]string{30: "one", 31: "two"}[id], history)
		})
	}
}
func gameTime(t *testing.T, m gameapi.Match) (nowTime time.Time) {
	t.Helper()
	var s game.State
	json.Unmarshal(m.Data, &s)
	return time.UnixMilli(s.Players[0].Deadline - 1000)
}

func TestStoneageElitePoolAndPetPreviewE2E(t *testing.T) {
	for n := 0; n < 100; n++ {
		m, _ := start(t, 2)
		var s game.State
		json.Unmarshal(m.Data, &s)
		if len(s.Pool.Rides) != 4 {
			t.Fatal("strong ride pool must draw four", len(s.Pool.Rides))
		}
		for _, p := range append(append([]game.PetDefinition{}, s.Pool.Pets...), s.Pool.Rides...) {
			sum := 0
			for _, w := range p.Weights {
				sum += w
			}
			if sum < 100 {
				t.Fatal("weak source species in elite pool", p.Name, sum)
			}
			stats, ok := obj(t, p)["stats"].(map[string]any)
			if !ok || stats["hp"].(float64) <= 0 || stats["attack"].(float64) <= 0 || stats["defense"].(float64) <= 0 || stats["speed"].(float64) <= 0 {
				t.Fatal("missing four-dimensional candidate preview", p)
			}
		}
		for _, rows := range [][]game.Equipment{s.Pool.Weapons, s.Pool.Hats, s.Pool.Clothes, s.Pool.Accessories} {
			for _, g := range rows {
				v := obj(t, g)
				if !strings.HasPrefix(g.Name, "合成") || v["tier"] != v["maxTier"] || v["tier"] == nil {
					t.Fatal("non-maximum crafted gear appeared", g)
				}
				if g.Slot != "accessory" && v["tier"] != float64(20) {
					t.Fatal("weapon/armor must be tier 20", g)
				}
			}
		}
	}
	report(t, "elite-pools", map[string]any{"pools": 100, "passed": true})
}
