package guandan_test

// A labelled milestone fixture forces the double-tribute branch. Every
// subsequent choice uses the real HTTP/session/production Apply/View pipeline.
import (
	"encoding/json"
	"reflect"
	"testing"

	game "github.com/k0ngk0ng/wire-board/games/guandan/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func tributeFixture() gameapi.Module {
	module := game.Module()
	start := module.Start
	module.Start = func(ctx gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		m, e := start(ctx, o)
		if e != nil {
			return m, e
		}
		var s game.State
		if e = json.Unmarshal(m.Data, &s); e != nil {
			return m, e
		}
		s.Round = 2
		s.Level = 5
		s.Levels = []int{5, 2}
		s.DealerTeam = 0
		s.OpeningSeat = 1
		s.Turn = 1
		s.Phase = "tribute"
		s.Played = []int{}
		s.FinishOrder = []int{}
		s.Table = []game.Play{}
		s.LastPlay = nil
		s.Revealed = []game.Reveal{}
		s.Results = []game.RoundResult{{Round: 1, Level: 2, Team: 0, Head: 0, PartnerPlace: 2, Promotion: 3, Order: []int{0, 2}, Ranks: []int{1, 3, 2, 3}, DoubleDown: true, Levels: []int{5, 2}}}
		deck := make([]int, 108)
		for i := range deck {
			deck[i] = i
		}
		deck[0], deck[53] = deck[53], deck[0]
		deck[54], deck[107] = deck[107], deck[54]
		s.Hands = make([][]int, 4)
		for p := 0; p < 4; p++ {
			s.Hands[p] = append([]int{}, deck[p*27:(p+1)*27]...)
		}
		s.Exchanges = []game.Exchange{{From: 1, To: -1, Given: -1, Returned: -1}, {From: 3, To: -1, Given: -1, Returned: -1}}
		s.Due = []int64{0, ctx.Now.UnixMilli() + 30000, 0, ctx.Now.UnixMilli() + 30000}
		s.Logs = append(s.Logs, "HTTP 测试夹具：第二副双下贡还；不代表已自然打完第一副")
		m.Data, _ = json.Marshal(s)
		m.Actors = []int{1, 3}
		m.TimeoutActors = []int{1, 3}
		m.Attention = []int{1, 3}
		m.Deadline = s.Due[1]
		return m, nil
	}
	return module
}
func TestJiangsuDoubleTributeAndPrivateReturnHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, tributeFixture(), "guandan")
	h.AssertUnpublished()
	h.Start(map[string]any{"rounds": 3})
	for _, p := range []int{1, 3} {
		d := gametest.Data(h.People[p].Room())
		choices := d["controls"].(map[string]any)["choices"].([]any)
		if len(choices) != 1 {
			t.Fatal("fixture should expose one maximum black joker", choices)
		}
		h.People[p].Action(map[string]any{"type": "tribute", "window": d["window"], "card": choices[0]}, 200)
	}
	h.Record("both black jokers publicly contributed")
	d := gametest.Data(h.People[0].Room())
	if d["phase"] != "return" {
		t.Fatal("did not enter return phase")
	}
	exchanges := d["exchanges"].([]any)
	for _, x := range exchanges {
		v := x.(map[string]any)
		from, to := int(v["from"].(float64)), int(v["to"].(float64))
		if to != (from+3)%4 {
			t.Fatal("equal tributes must transfer clockwise", v)
		}
	}
	for _, p := range []int{0, 2} {
		d := gametest.Data(h.People[p].Room())
		candidate := d["controls"].(map[string]any)["choices"].([]any)[0]
		h.People[p].Action(map[string]any{"type": "return", "window": d["window"], "card": candidate}, 200)
		other := 2 - p
		outsider := gametest.Data(h.People[other].Room())
		spectator := gametest.Data(h.Spectator.Room())
		for _, v := range []map[string]any{outsider, spectator} {
			for _, x := range v["exchanges"].([]any) {
				r := x.(map[string]any)
				if int(r["to"].(float64)) == p && r["returned"].(float64) != -1 {
					t.Fatal("return card leaked to nonparty", r)
				}
			}
			for _, x := range v["events"].([]any) {
				ev := x.(map[string]any)
				if ev["type"] == "return" && int(ev["seat"].(float64)) == p {
					if cs, ok := ev["cards"].([]any); ok && len(cs) > 0 {
						t.Fatal("private return leaked through event", ev)
					}
				}
			}
		}
	}
	d = gametest.Data(h.People[0].Room())
	if d["phase"] != "play" || d["turn"].(float64) != 1 {
		t.Fatal("higher/tied tribute giver did not lead", d["turn"])
	}
	for _, c := range h.People {
		if len(gametest.Data(c.Room())["hand"].([]any)) != 27 {
			t.Fatal("card conservation failed after tribute")
		}
	}
	h.Record("private returns complete, all hands have 27 cards")
	before := gametest.Data(h.People[1].Room())
	h.Restart()
	if !reflect.DeepEqual(before, gametest.Data(h.People[1].Room())) {
		t.Fatal("restore changed private return information")
	}
}
func TestJiangsuNaturalCompleteGameHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, game.Module(), "guandan")
	h.Start(map[string]any{"rounds": 2})
	phases := map[string]int{}
	steps := 0
	for ; steps < 1200; steps++ {
		room := h.People[0].Room()
		if room["status"] == "finished" {
			break
		}
		actors := room["game"].(map[string]any)["actors"].([]any)
		if len(actors) == 0 {
			t.Fatal("no actor")
		}
		p := int(actors[0].(float64))
		c := h.People[p]
		d := gametest.Data(c.Room())
		phase := d["phase"].(string)
		phases[phase]++
		control := d["controls"].(map[string]any)
		a := map[string]any{"window": d["window"]}
		switch phase {
		case "tribute", "return":
			a["type"] = phase
			a["card"] = control["choices"].([]any)[0]
		case "round_end":
			a["type"] = "ready"
		case "play":
			moves := control["moves"].([]any)
			if len(moves) == 0 {
				a["type"] = "pass"
			} else {
				best := moves[0].(map[string]any)
				for _, v := range moves {
					m := v.(map[string]any)
					if len(m["cards"].([]any)) > len(best["cards"].([]any)) {
						best = m
					}
				}
				a["type"] = "play"
				a["cards"] = best["cards"]
				a["interpretation"] = best["key"]
			}
		default:
			t.Fatal("unexpected phase", phase)
		}
		c.Action(a, 200)
	}
	if steps == 1200 {
		t.Fatal("natural HTTP match stalled")
	}
	h.Record("natural Jiangsu match completed")
	r := h.People[0].Room()
	if r["status"] != "finished" {
		t.Fatal(r["status"])
	}
	h.Restart()
	if !reflect.DeepEqual(gametest.Data(r), gametest.Data(h.People[0].Room())) {
		t.Fatal("result changed after restore")
	}
}
