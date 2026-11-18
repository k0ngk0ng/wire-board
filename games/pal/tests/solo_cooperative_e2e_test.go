package pal_test

// Current single-player contract: the original coop icon/HP cost must work
// without peer approval, a saved selection must remain uncommitted, changing
// the icon must not spend HP, and root cancel must let the same human revise
// the previous member's command. All actions are ordinary original inputs.
import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestSoloOriginalCooperativeMagicAndBacktrackingSave(t *testing.T) {
	for _, mode := range []string{"cooperative", "revise_previous_role"} {
		t.Run(mode, func(t *testing.T) {
			m := originalBattleModule(t)
			c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0)}
			g, err := m.Start(c, raw(map[string]any{}))
			if err != nil {
				t.Fatal(err)
			}
			tick := func() {
				t.Helper()
				c.Now = time.UnixMilli(g.SystemDeadline + 200)
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				if v := view(t, m, g, 0); v.Cooperation != nil || v.Phase == "cooperation" {
					t.Fatal("single player was asked for peer approval")
				}
			}
			until := func(check func(pal.View) bool) {
				t.Helper()
				for i := 0; i < 256; i++ {
					if check(view(t, m, g, 0)) {
						return
					}
					tick()
				}
				t.Fatal("original solo command did not reach target")
			}
			input := func(direction int, keys uint32) {
				t.Helper()
				v := view(t, m, g, 0)
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: direction, Keys: keys})
			}
			root := func(v pal.View) bool {
				return v.Battle != nil && v.Battle.State == 1 && v.Battle.Current == 0 && v.Seq == v.AppliedSeq
			}
			until(root)
			initial := view(t, m, g, 0)
			if mode == "revise_previous_role" {
				input(4, 1024) // Original defend, then revise the preceding role.
				until(func(v pal.View) bool { return v.Battle.Current == 1 && v.Battle.State == 1 })
				input(4, 1)
				until(root)
				if !reflect.DeepEqual(initial.Party, view(t, m, g, 0).Party) {
					t.Fatal("revising an unexecuted command spent resources")
				}
			} else {
				input(3, 32)
				until(func(v pal.View) bool { return v.Battle.Selected == 2 && v.Seq == v.AppliedSeq })
			}
			before := view(t, m, g, 0)
			g, err = m.SaveControl(c, g, true)
			if err != nil {
				t.Fatal(err)
			}
			c.Now = c.Now.Add(time.Hour)
			m = originalBattleModule(t)
			g, err = m.ResumeSaved(c, g)
			if err != nil {
				t.Fatal(err)
			}
			after := view(t, m, g, 0)
			if !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Battle), raw(after.Battle)) || before.Cash != after.Cash || before.Epoch == after.Epoch {
				t.Fatal("saved original solo command changed")
			}
			if mode == "cooperative" {
				input(2, 16) // Leave the coop icon without committing it.
				until(func(v pal.View) bool { return v.Battle.Selected == 0 && v.Seq == v.AppliedSeq })
				if !reflect.DeepEqual(initial.Party, view(t, m, g, 0).Party) {
					t.Fatal("uncommitted coop selection consumed resources")
				}
				input(3, 32)
				until(func(v pal.View) bool { return v.Battle.Selected == 2 && v.Seq == v.AppliedSeq })
				input(4, 2)
			}
			spentHP := false
			for step := 0; step < 1024; step++ {
				v := view(t, m, g, 0)
				for i, p := range v.Party {
					spentHP = spentHP || p.HP < initial.Party[i].HP
					if p.Controller != 0 {
						t.Fatal("another controller owns a solo member")
					}
				}
				if v.Cash == 3 {
					if mode == "cooperative" && !spentHP {
						t.Fatal("original coop HP cost was not observed")
					}
					report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "ordinary original solo three-role combat, no peer approval, selected command saved in fresh runtime, uncommitted selection/revision preserves resources and original three-cash victory; isolated battle fixture, not campaign", "mode": mode, "saved": true, "noPeerApproval": true, "observedHPCost": spentHP, "cash": v.Cash}, "", "  ")
					if err := os.WriteFile(campaignEvidencePath("solo-"+mode+".json", 3), report, 0600); err != nil {
						t.Fatal(err)
					}
					return
				}
				if b := v.Battle; mode != "cooperative" && b != nil && b.Phase == 0 && b.State != 0 && v.Seq == v.AppliedSeq {
					input(4, 2)
				}
				tick()
			}
			t.Fatal("original solo battle did not finish")
		})
	}
}
