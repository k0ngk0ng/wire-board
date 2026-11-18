package pal_test

// Test first. Original return paths or palace triggers fail; the reordered
// party cannot fight; Ling'er leaving/rejoining steals a surviving role's
// controller; unassigned humans disappear from shared reading; saving in a
// battle/departure/death/rejoin repeats events or changes native state; old
// controls survive; the funeral never reaches the original field endpoint.
// Route coordinates and future story markers stay in this E2E only.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type homecomingProof struct {
	Home, Separated, Grandma, Died, Farewell, Invited, Rejoined                  bool
	HomeSaved, SeparateSaved, GrandmaSaved, DeathSaved, RejoinSaved, CombatSaved bool
	InBattle                                                                     bool
	Readings, Saves, Battles                                                     int
	LastPrompt                                                                   uint64
}

func TestOriginalCampaignPalaceHomecomingAndFarewellSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			originalHomecoming(t, players)
		})
	}
}

func originalHomecoming(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalReturnIsland(t, players)
	return runOriginalHomecoming(t, m, c, g, players, homecomingProof{})
}

func runOriginalHomecoming(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof homecomingProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, b := range map[string][]byte{"homecoming-failure-state.json": g.Data, "homecoming-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), b, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	owners := func(v pal.View) {
		t.Helper()
		if len(v.Party) < 1 || len(v.Party) > 2 {
			t.Fatal("homecoming invented/lost original party", v.Party)
		}
		seen := map[int]bool{}
		for _, p := range v.Party {
			expected := 0
			if p.Role == 1 && players > 1 {
				expected = 1
			}
			if p.Role < 0 || p.Role > 1 || seen[p.Role] || p.Controller != expected {
				t.Fatal("homecoming role owner changed", v.Party)
			}
			seen[p.Role] = true
		}
		if !seen[0] {
			t.Fatal("homecoming lost surviving Li Xiaoyao")
		}
	}
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("homecoming partial reading changed")
				}
			}
		}
		before := v
		var err error
		g, err = m.SaveControl(c, g, true)
		if err != nil {
			t.Fatal(err)
		}
		c.Now = c.Now.Add(24 * time.Hour)
		m, _ = nativeModule(t)
		g, err = m.ResumeSaved(c, g)
		if err != nil {
			t.Fatal(err)
		}
		v = view(t, m, g, 0)
		if before.Scene != v.Scene || before.Map != v.Map || before.X != v.X || before.Y != v.Y || before.Leader != v.Leader || before.Cash != v.Cash || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Battle), raw(v.Battle)) || !bytes.Equal(raw(before.Field), raw(v.Field)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) || !bytes.Equal(raw(before.Draws), raw(v.Draws)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) {
			t.Fatal("homecoming continuation changed original event/party/drawing")
		}
		c.Actor = v.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("homecoming resumed stale input")
		}
		proof.Saves++
	}
	for step := 0; step < 6000; step++ {
		v := view(t, m, g, 0)
		v = view(t, m, g, v.Leader)
		owners(v)
		if v.Cash < 550 {
			t.Fatal("homecoming restarted after defeat", v.Scene, v.Cash)
		}
		if len(v.Party) == 1 {
			proof.Separated = true
			if !proof.SeparateSaved {
				save(v.Dialogue != nil)
				proof.SeparateSaved = true
				v = view(t, m, g, v.Leader)
			}
		} else if proof.Separated {
			proof.Rejoined = true
			if !proof.RejoinSaved {
				save(v.Dialogue != nil)
				proof.RejoinSaved = true
				v = view(t, m, g, v.Leader)
			}
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Home = proof.Home || strings.Contains(d.Text, "好惨") || strings.Contains(d.Text, "全都遭到苗人的毒手")
				proof.Grandma = proof.Grandma || strings.Contains(d.Text, "姥姥！！")
				proof.Died = proof.Died || strings.Contains(d.Text, "姥姥便断气")
				proof.Farewell = proof.Farewell || strings.Contains(d.Text, "就此拜别")
				proof.Invited = proof.Invited || strings.Contains(d.Text, "如先住在我家")
				t.Log("homecoming reading", proof.Readings, v.Scene, d.ID, len(v.Party))
			}
			if proof.Home && !proof.HomeSaved {
				save(true)
				proof.HomeSaved = true
				v = view(t, m, g, v.Leader)
				d = v.Dialogue
			}
			if proof.Grandma && !proof.GrandmaSaved {
				save(true)
				proof.GrandmaSaved = true
				v = view(t, m, g, v.Leader)
				d = v.Dialogue
			}
			if proof.Died && !proof.DeathSaved {
				save(true)
				proof.DeathSaved = true
				v = view(t, m, g, v.Leader)
				d = v.Dialogue
			}
			if len(d.Read) != players {
				t.Fatal("homecoming party changes lost original readers")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Battle != nil {
			if !proof.InBattle {
				proof.Battles++
				proof.InBattle = true
			}
			if !proof.CombatSaved {
				save(false)
				proof.CombatSaved = true
			}
			for _, actor := range g.Actors {
				c.Actor = actor
				a, err := m.BotAction(c, g)
				if err != nil {
					t.Fatal(err)
				}
				if len(a) > 0 {
					g, err = m.Apply(campaignBotContext(c), g, a)
					if err != nil {
						t.Fatal(err)
					}
				}
			}
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			proof.InBattle = false
			stage := 0
			if proof.Home {
				stage++
			}
			if proof.Separated {
				stage++
			}
			if proof.Died {
				stage++
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("homecoming repeats position", place)
			}
			if v.Scene == 20 && proof.Invited {
				if !proof.HomeSaved || !proof.SeparateSaved || !proof.GrandmaSaved || !proof.DeathSaved || !proof.RejoinSaved || !proof.Farewell || !proof.Rejoined || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 1 {
					t.Fatal("original homecoming/farewell incomplete", proof, v.Party)
				}
				save(false)
				if err := os.WriteFile(campaignEvidencePath("campaign-homecoming-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": players, "scope": "actual original return forest/opened path, palace homecoming, Ling'er separation/grandmother farewell and original rejoining; original owners/readers and fresh-runtime event saves; not full campaign or full autoplay", "scene": v.Scene, "map": v.Map, "cash": v.Cash, "party": v.Party, "leader": v.Leader, "readings": proof.Readings, "saves": proof.Saves, "battles": proof.Battles, "separationSaved": proof.SeparateSaved, "rejoinSaved": proof.RejoinSaved, "deathReadingSaved": proof.DeathSaved, "originalReaders": players, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-homecoming.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			walker.allowTouch = v.Scene == 16
			x, y, reach := 1472, 448, 32
			switch v.Scene {
			case 15:
			case 16:
				x, y = 1824, 944
			case 19:
				x, y = 1872, 488
			case 17:
				x, y = 1856, 1040
			case 20:
				x, y = 1088, 592
			case 21:
				x, y, reach = 784, 1384, 32
				if proof.Separated {
					x, y, reach = 1120, 1232, 64
				}
			default:
				t.Fatal("homecoming left original route", v.Scene, v.X, v.Y)
			}
			g = walker.walk(t, m, &c, g, v, x, y, reach)
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("homecoming position", step, v.Scene, v.X, v.Y, v.Phase, len(v.Party))
		}
	}
	t.Fatal("original palace homecoming/farewell did not finish", proof)
	return m, c, g
}
