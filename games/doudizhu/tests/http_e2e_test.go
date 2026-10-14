package doudizhu_test

// Real HTTP cases are written before platform registration. The helper owns
// only sessions, rooms and trace files; all rules are the production module.
import (
	"fmt"
	"reflect"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestLegacyDoudizhuRoomsAndPrivateViewsHTTP(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, legacyModule(), "doudizhu")
			h.AssertUnpublished()
			h.Start(map[string]any{"rounds": 1})
			for p, c := range h.People {
				d := gametest.Data(c.Room())
				size := 17
				if n == 4 {
					size = 25
				}
				if len(d["hand"].([]any)) != size {
					t.Fatal("wrong private hand", p, d)
				}
				if _, ok := d["hands"]; ok {
					t.Fatal("all hands leaked")
				}
				if len(d["bottom"].([]any)) != 0 {
					t.Fatal("bottom leaked before auction")
				}
			}
			watched := gametest.Data(h.Spectator.Room())
			if len(watched["hand"].([]any)) != 0 {
				t.Fatal("spectator hand leaked")
			}
			h.Record("private auction views")
			old := h.People[0].Room()
			// Upgrade the running server to the real four-player-only module.
			// Existing three-player matches must still restore and accept actions.
			h.Config.Modules = []gameapi.Module{game.Module()}
			h.Restart()
			if !reflect.DeepEqual(gametest.Data(old), gametest.Data(h.People[0].Room())) {
				t.Fatal("restart changed game")
			}
			r := h.People[0].Room()
			actor := int(gametest.Data(r)["turn"].(float64))
			a := h.People[actor]
			wrong := h.People[(actor+1)%n]
			wrong.Action(map[string]any{"type": "bid", "window": gametest.Data(r)["window"], "bid": 3}, 400)
			a.Action(map[string]any{"type": "bid", "window": gametest.Data(r)["window"], "bid": 3}, 200)
			h.Record("landlord takes public bottom")
			d := gametest.Data(a.Room())
			size, bottom := 20, 3
			if n == 4 {
				size, bottom = 33, 8
			}
			if len(d["hand"].([]any)) != size || len(d["bottom"].([]any)) != bottom {
				t.Fatal("wrong landlord hand")
			}
			control := d["controls"].(map[string]any)
			move := control["moves"].([]any)[0].(map[string]any)
			req := a.ActionRequest(map[string]any{"type": "play", "window": d["window"], "cards": move["cards"]})
			a.Request("POST", "/api/rooms/"+r["id"].(string), req, 200)
			saved := gametest.Data(a.Room())
			a.Request("POST", "/api/rooms/"+r["id"].(string), req, 200)
			if !reflect.DeepEqual(saved, gametest.Data(a.Room())) {
				t.Fatal("nonce replay played cards twice")
			}
			h.Record("duplicate command is idempotent")
		})
	}
}
func TestLegacyDoudizhuNaturalCompleteGamesHTTP(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, legacyModule(), "doudizhu")
			h.Start(map[string]any{"rounds": 1})
			steps := 0
			for ; steps < 500; steps++ {
				r := h.People[0].Room()
				if r["status"] == "finished" {
					break
				}
				d := gametest.Data(r)
				actor := int(d["turn"].(float64))
				p := h.People[actor]
				v := gametest.Data(p.Room())
				c := v["controls"].(map[string]any)
				action := map[string]any{"window": v["window"]}
				switch v["phase"] {
				case "bid":
					action["type"] = "bid"
					action["bid"] = 3
				case "play":
					moves := c["moves"].([]any)
					if len(moves) == 0 {
						action["type"] = "pass"
					} else {
						best := moves[0].(map[string]any)
						for _, candidate := range moves {
							m := candidate.(map[string]any)
							if len(m["cards"].([]any)) > len(best["cards"].([]any)) {
								best = m
							}
						}
						action["type"] = "play"
						action["cards"] = best["cards"]
					}
				default:
					t.Fatal("unexpected HTTP phase", v["phase"])
				}
				p.Action(action, 200)
			}
			if steps == 500 {
				t.Fatal("natural HTTP game stalled")
			}
			r := h.People[0].Room()
			d := gametest.Data(r)
			rr := d["roundResults"].([]any)[0].(map[string]any)
			first := int(rr["firstOut"].(float64))
			landlord := int(rr["landlord"].(float64))
			expected := n - 1
			if first == landlord {
				expected = 1
			}
			if len(rr["winners"].([]any)) != expected {
				t.Fatal("wrong coalition winner count", rr)
			}
			sum := float64(0)
			for _, score := range rr["scores"].([]any) {
				sum += score.(float64)
			}
			if sum != 0 {
				t.Fatal("HTTP score conservation failed")
			}
			h.Record("natural game finished")
			h.Restart()
			if !reflect.DeepEqual(d, gametest.Data(h.People[0].Room())) {
				t.Fatal("finished game changed after restore")
			}
			h.People[0].Command("rematch", nil, 200)
			if h.People[0].Room()["status"] != "waiting" {
				t.Fatal("rematch did not return to common waiting room")
			}
			h.Record("common rematch")
		})
	}
}
