package monopoly_test

// Failure scenarios before UI controls: spectator or eliminated action hints;
// roll prompts on other seats; unaffordable auctions offered; mortgage/build
// hints disagree with even-building rules; hints spend reserved auction funds;
// viewing hints mutates the game or exposes private state.
import (
	"reflect"
	"testing"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
)

func controls(c *client) map[string]any { return data(c.room())["controls"].(map[string]any) }
func propertyControl(c *client, id string) map[string]any {
	return controls(c)["properties"].(map[string]any)[id].(map[string]any)
}
func TestMonopolyControlsHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { s.Phase = "roll"; group(s, 0, []int{6, 8, 9}, []int{0, 0, 0}) })
	a, b := tb.people[0], tb.people[1]
	if controls(a)["roll"] != true || controls(b)["roll"] == true || controls(tb.spectator)["active"] == true {
		t.Fatal("wrong action roles")
	}
	if propertyControl(a, "6")["build"] != true || propertyControl(a, "6")["mortgage"] != true {
		t.Fatal("legal asset actions missing")
	}
	before := a.room()
	_ = controls(a)
	_ = controls(tb.spectator)
	if !reflect.DeepEqual(before, a.room()) {
		t.Fatal("view mutated state")
	}
	a.act(map[string]any{"type": "build", "property": 6}, 200)
	if propertyControl(a, "6")["build"] == true || propertyControl(a, "8")["mortgage"] == true || propertyControl(a, "6")["sell"] != true {
		t.Fatal("hints ignored even building/group mortgage rules")
	}
	tb.record("server-authored controls agree with actions")
}

func TestMonopolyControlsAuctionAndDebtHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { s.Phase = "purchase"; s.Players[0].Position = 6; s.Players[2].Cash = 0 })
	a, c := tb.people[0], tb.people[2]
	a.act(map[string]any{"type": "auction"}, 200)
	if controls(a)["bid"] != true || controls(c)["bid"] == true || controls(c)["auctionPass"] == true {
		t.Fatal("unfunded player offered a useless response")
	}
	tb.record("auction eligibility controls")
}
