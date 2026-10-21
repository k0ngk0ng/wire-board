package tests

// Failure scenarios established before implementation: status cycles shrink
// with team size; freshly inflicted poison ticks before its owner's turn;
// poison applied while another shell is flying; poison kills a player who is
// nevertheless granted an action; restore decrements a status a second time.
import (
	"encoding/json"
	fb "github.com/k0ngk0ng/wire-board/games/fortressblue/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
)

func TestStatusesAdvanceOnlyWhenOwnerReceivesTurn(t *testing.T) {
	for _, players := range []int{2, 4, 6, 8} {
		t.Run(string(rune('0'+players)), func(t *testing.T) {
			mod, m, now := liveFixture(t, players, false)
			s, _ := timeline(t, m)
			s.Tanks[0].Statuses = []fb.Status{{Kind: "burn", Turns: 2, Source: 0}, {Kind: "poison", Turns: 3, Damage: 25, Source: players / 2}}
			before := s.Tanks[0].HP
			m.Data, _ = json.Marshal(s)
			for turn := 1; turn <= players; turn++ {
				m = completeSupplyTransport(t, mod, m)
				s, _ = timeline(t, m)
				m = timerAt(t, mod, m, s.Deadline)
				s, _ = timeline(t, m)
				want := before
				cycles := 2
				if turn == players {
					want -= 25
					cycles--
				}
				if s.Tanks[0].HP != want || len(s.Tanks[0].Statuses) != 2 || s.Tanks[0].Statuses[0].Turns != cycles {
					t.Fatalf("after %d/%d turns hp=%d statuses=%+v", turn, players, s.Tanks[0].HP, s.Tanks[0].Statuses)
				}
				restored, e := mod.Restore(gameapi.Context{Players: players, Actor: 0, Now: now}, m)
				if e != nil {
					t.Fatal(e)
				}
				r, _ := timeline(t, restored)
				if r.Tanks[0].HP != want {
					t.Fatal("restore ticked status twice")
				}
			}
		})
	}
}
func TestPoisonWaitsUntilTargetTurnAndSkipsKilledActor(t *testing.T) {
	mod, m, now := liveFixture(t, 4, false)
	s, _ := timeline(t, m)
	s.Tanks[1].Statuses = []fb.Status{{Kind: "poison", Turns: 3, Damage: 25, Source: 2}}
	s.Tanks[1].HP = 20
	m.Data, _ = json.Marshal(s)
	n := fireFixture(t, mod, m, now)
	_, w := timeline(t, n)
	if w.Resolution == nil {
		t.Fatal("missing flight")
	}
	n = timerAt(t, mod, n, w.Resolution.ImpactAt-1)
	s, _ = timeline(t, n)
	if s.Tanks[1].HP != 20 {
		t.Fatal("poison ticked during unrelated flight")
	}
	for i := 0; i < 20; i++ {
		_, w = timeline(t, n)
		if w.Resolution == nil {
			break
		}
		n = timerAt(t, mod, n, n.SystemDeadline)
	}
	s, _ = timeline(t, n)
	if s.Current != 2 || s.Tanks[1].HP != 20 {
		t.Fatal("poison ticked on other player's turn")
	}
	n = timerAt(t, mod, n, s.Deadline)
	s, _ = timeline(t, n)
	if s.Tanks[1].Alive || s.Tanks[1].HP != 0 || s.Current == 1 {
		t.Fatal("poison victim was offered an action")
	}
	for _, seat := range n.Actors {
		if seat == 1 {
			t.Fatal("dead status target actor")
		}
	}
}
