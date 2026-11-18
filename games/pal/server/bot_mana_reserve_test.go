package pal

// Failure cases before changing the ordinary combat policy: cheap repeated
// offensive spells exhaust the public mana budget needed for healing; falling
// back confirms a human's selected magic/coop/misc icon; healing loses priority
// to conservation; high-mana opening combat loses its existing shortcut;
// another owner, pending input, animations or a human submenu receive input.
import (
	"fmt"
	"testing"
)

func TestBattleBotReservesPublicManaForRecovery(t *testing.T) {
	for _, roles := range []int{1, 2, 3} {
		for _, mp := range []int{0, 1, 49, 50, 51, 100} {
			t.Run(fmt.Sprintf("roles-%d/mp-%d", roles, mp), func(t *testing.T) {
				v := View{Phase: "battle", Epoch: 7, ControlAt: 1000, Party: make([]PartyMember, roles), Battle: &BattleView{State: 1, Current: roles - 1}}
				for i := range v.Party {
					v.Party[i] = PartyMember{Role: i, Controller: 0, HP: 100, MaxHP: 100, MP: mp, MaxMP: 100}
				}
				for selected := 0; selected < 4; selected++ {
					v.Battle.Selected = selected
					a, ok := battleBotAction(v, 0, nil)
					want := uint32(8)
					if selected == 1 {
						want = 2
					}
					if mp <= 50 {
						want = 2
						if selected != 0 {
							want = 16
						}
					}
					if !ok || a.Keys != want {
						t.Fatal("mana reserve selected the wrong original icon/action", selected, mp, a)
					}
				}
				v.Battle.Selected = 0
				if mp > 0 {
					v.Party[0].HP = 20
					a, ok := battleBotAction(v, 0, nil)
					if !ok || a.Keys != 8 {
						t.Fatal("mana conservation displaced ordinary healing", a)
					}
					v.Party[0].HP = 100
				}
				v.Battle.Menu = 1
				if a, ok := battleBotAction(v, 0, nil); !ok || a.Keys != 1 {
					t.Fatal("fallback confirmed an unfinished human spell", a)
				}
				v.Battle.Menu = 0
				v.Seq = 1
				if a, ok := battleBotAction(v, 0, nil); ok {
					t.Fatal("fallback ran ahead of accepted input", a)
				}
				v.Seq = 0
				if a, ok := battleBotAction(v, 1, nil); ok {
					t.Fatal("fallback controlled another owner", a)
				}
				v.Battle.State = 0
				if a, ok := battleBotAction(v, 0, nil); ok {
					t.Fatal("fallback controlled a battle animation", a)
				}
			})
		}
	}
}
