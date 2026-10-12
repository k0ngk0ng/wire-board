package monopoly_test

// Presentation failure cases, specified before implementation: wrong direction
// across GO/back-three; card-chain teleport loses intermediate squares; a third
// double walks instead of going directly to jail; replay uses private RNG;
// refresh/restart loses event IDs; spectators see a different public movement.
import (
	"encoding/json"
	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
	"reflect"
	"testing"
)

func TestMonopolyMotionHTTP(t *testing.T) {
	for _, tc := range []struct {
		name            string
		position        int
		doubles         int
		key             byte
		wantKinds       []string
		from, to, steps int
	}{
		{"cross_go", 38, 0, 5, []string{"roll", "move"}, 38, 1, 3},
		{"third_double", 15, 2, 0, []string{"roll", "jump"}, 15, 10, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newTable(t, 2, func(s *monopoly.State) {
				s.Phase = "roll"
				s.Players[0].Position = tc.position
				s.Doubles = tc.doubles
				dice(s, tc.key)
			})
			c := tb.people[0]
			c.act(map[string]any{"type": "roll"}, 200)
			r := c.room()
			v := data(r)
			events, ok := v["motion"].([]any)
			if !ok || len(events) != 2 {
				t.Fatalf("missing public motion: %v", v["motion"])
			}
			kinds := []string{}
			for i, e := range events {
				a := e.(map[string]any)
				kinds = append(kinds, a["kind"].(string))
				if a["id"] != float64(i+1) || a["player"] != float64(0) {
					t.Fatal(a)
				}
			}
			if !reflect.DeepEqual(kinds, tc.wantKinds) {
				t.Fatal(kinds)
			}
			move := events[1].(map[string]any)
			if move["from"] != float64(tc.from) || move["to"] != float64(tc.to) || move["steps"] != float64(tc.steps) {
				t.Fatal(move)
			}
			if !reflect.DeepEqual(v["motion"], data(tb.people[1].room())["motion"]) {
				t.Fatal("public motion differs")
			}
			raw, _ := json.Marshal(v["motion"])
			tb.restart()
			after, _ := json.Marshal(data(c.room())["motion"])
			if string(raw) != string(after) {
				t.Fatal("restart changed motion")
			}
			tb.record("public animation trace survives restart")
		})
	}
}

func TestMonopolyCardMotionChainHTTP(t *testing.T) {
	tb := newTable(t, 2, func(s *monopoly.State) { cardFixture(s, 0, 4) })
	tb.people[0].act(map[string]any{"type": "roll"}, 200)
	events := data(tb.people[0].room())["motion"].([]any)
	if len(events) != 3 {
		t.Fatal(events)
	}
	backwards := events[2].(map[string]any)
	if backwards["from"] != float64(7) || backwards["to"] != float64(4) || backwards["steps"] != float64(-3) {
		t.Fatal(backwards)
	}
	tb.record("dice, forward movement and card back-three remain ordered")
}
