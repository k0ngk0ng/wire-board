package shengji_test

import (
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/shengji/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
)

func TestShanghaiNaturalRoomsHTTP(t *testing.T) {
	for _, decks := range []int{1, 2} {
		t.Run(fmt.Sprint(decks), func(t *testing.T) {
			h := gametest.NewHTTP(t, 4, game.Module(), "shengji")
			h.AssertUnpublished()
			h.Start(map[string]any{"decks": decks, "rounds": 2})
			n := 25
			if decks == 1 {
				n = 12
			}
			for _, c := range h.People {
				if len(gametest.Data(c.Room())["hand"].([]any)) != n {
					t.Fatal("wrong dealt count")
				}
			}
			steps := 0
			checkedBury := false
			for ; steps < 1800; steps++ {
				room := h.People[0].Room()
				if room["status"] == "finished" {
					break
				}
				actors := room["game"].(map[string]any)["actors"].([]any)
				if len(actors) == 0 {
					t.Fatal("stalled")
				}
				p := int(actors[0].(float64))
				c := h.People[p]
				d := gametest.Data(c.Room())
				ctrl := d["controls"].(map[string]any)
				a := map[string]any{"window": d["window"]}
				switch d["phase"] {
				case "declare":
					ds := ctrl["declarations"].([]any)
					if len(ds) > 0 && d["declaration"] == nil {
						a["type"] = "declare"
						a["cards"] = ds[len(ds)-1].(map[string]any)["cards"]
					} else {
						a["type"] = "pass"
					}
				case "bury":
					count := int(ctrl["buryCount"].(float64))
					hand := d["hand"].([]any)
					if len(hand) != n+count {
						t.Fatal("dealer failed to take bottom")
					}
					if !checkedBury {
						wrong := h.People[(p+1)%4]
						wrong.Action(map[string]any{"type": "bury", "window": d["window"], "cards": hand[:count]}, 400)
						c.Action(map[string]any{"type": "bury", "window": d["window"], "cards": hand[:count-1]}, 400)
						h.Record("dealer privately takes bottom")
						h.Restart()
						if !reflect.DeepEqual(d, gametest.Data(c.Room())) {
							t.Fatal("restore changed private hand")
						}
						checkedBury = true
					}
					a["type"] = "bury"
					a["cards"] = hand[:count]
				case "play":
					moves := ctrl["moves"].([]any)
					if len(moves) == 0 {
						t.Fatal("no legal follow")
					}
					a["type"] = "play"
					a["cards"] = moves[0].(map[string]any)["cards"]
				case "round_end":
					a["type"] = "ready"
				default:
					t.Fatal("unexpected phase", d["phase"])
				}
				c.Action(a, 200)
				v := gametest.Data(h.Spectator.Room())
				if len(v["hand"].([]any)) > 0 {
					t.Fatal("spectator hand leak")
				}
				if v["phase"] == "play" || v["phase"] == "bury" {
					if len(v["bottom"].([]any)) > 0 {
						t.Fatal("bottom leaked")
					}
				}
			}
			if steps == 1800 {
				t.Fatal("natural Shanghai HTTP match stalled")
			}
			h.Record("two natural hands and final result")
			r := h.People[0].Room()
			if r["status"] != "finished" {
				t.Fatal("not finished")
			}
			h.Restart()
			if !reflect.DeepEqual(gametest.Data(r), gametest.Data(h.People[0].Room())) {
				t.Fatal("result restore")
			}
			h.People[0].Command("rematch", nil, 200)
			if h.People[0].Room()["status"] != "waiting" {
				t.Fatal("rematch")
			}
			h.Record("common waiting room after rematch")
		})
	}
}
