package pal_test

// Test first: escort dialogue/return boat fails; automatic original party
// reorder assigns another human's role; the third reader vanishes; a saved
// partial dialogue, sailing script or reordered field repeats/loses progress;
// stale controls act after resuming or the new party cannot move together.
// Coordinates below are walkthrough input only, never production bot data.
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

type returnIslandProof struct {
	Balcony, Downstairs                            bool
	Aunt, Boat, Reordered                          bool
	AuntSaved, BoatSaved, SailingSaved, OrderSaved bool
	Readings, Saves                                int
	LastPrompt                                     uint64
}

func TestOriginalCampaignEscortReturnBoatAndPartyOrderSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			t.Parallel()
			originalReturnIsland(t, players)
		})
	}
}

func originalReturnIsland(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalFirstParty(t, players)
	return runOriginalReturnIsland(t, m, c, g, players, returnIslandProof{})
}

func runOriginalReturnIsland(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof returnIslandProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"return-island-failure-state.json": g.Data, "return-island-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	owners := func(v pal.View) {
		t.Helper()
		if len(v.Party) != 2 {
			t.Fatal("escort invented/lost an original party role", v.Party)
		}
		seen := map[int]bool{}
		for _, p := range v.Party {
			expected := 0
			if p.Role == 1 && players > 1 {
				expected = 1
			}
			if p.Role < 0 || p.Role > 1 || seen[p.Role] || p.Controller != expected {
				t.Fatal("original reorder changed role owners", v.Party)
			}
			seen[p.Role] = true
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
					t.Fatal("escort partial reading changed")
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
		if before.Scene != v.Scene || before.Map != v.Map || before.X != v.X || before.Y != v.Y || before.Leader != v.Leader || before.Cash != v.Cash || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Field), raw(v.Field)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) || !bytes.Equal(raw(before.Draws), raw(v.Draws)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) {
			t.Fatal("escort continuation changed original progress")
		}
		c.Actor = v.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("escort resumed stale input")
		}
		proof.Saves++
	}
	for step := 0; step < 2400; step++ {
		v := view(t, m, g, 0)
		v = view(t, m, g, v.Leader)
		owners(v)
		if v.Party[0].Role == 1 {
			proof.Reordered = true
		}
		if proof.Reordered && !proof.OrderSaved {
			save(v.Dialogue != nil)
			proof.OrderSaved = true
			v = view(t, m, g, 0)
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Aunt = proof.Aunt || strings.Contains(d.Text, "快去快回")
				proof.Boat = proof.Boat || strings.Contains(d.Text, "这位姑娘也要去仙灵岛")
				t.Log("escort return reading", proof.Readings, v.Scene, d.ID)
			}
			if proof.Aunt && !proof.AuntSaved {
				save(true)
				proof.AuntSaved = true
				v = view(t, m, g, 0)
				d = v.Dialogue
			}
			if proof.Boat && !proof.BoatSaved {
				save(true)
				proof.BoatSaved = true
				v = view(t, m, g, 0)
				d = v.Dialogue
			}
			if len(d.Read) != players {
				t.Fatal("escort lost original human reading roster")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if proof.Boat && v.Scene == 6 && v.Phase == "script" && !proof.SailingSaved {
			save(false)
			proof.SailingSaved = true
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			stage := 0
			if proof.Aunt {
				stage++
			}
			if proof.Boat {
				stage++
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("escort return repeats position", place)
			}
			if v.Scene == 15 {
				if !proof.AuntSaved || !proof.BoatSaved || !proof.SailingSaved || !proof.OrderSaved || !proof.Reordered || v.Party[0].Role != 1 || v.Party[1].Role != 0 {
					t.Fatal("original escort boat/reorder incomplete", proof, v.Party)
				}
				origin := [2]int{v.X, v.Y}
				direction := -1
				for d, delta := range [][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}} {
					if v.Field.Cells[(8+delta[1])*17+8+delta[0]] == 0 {
						direction = d
						break
					}
				}
				if direction < 0 {
					t.Fatal("reordered original party has no safe step")
				}
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: []uint32{4, 8, 16, 32}[direction]})
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
				c.Now = time.UnixMilli(g.SystemDeadline + 200)
				var err error
				g, err = m.OnTimer(c, g)
				if err != nil {
					t.Fatal(err)
				}
				v = view(t, m, g, 0)
				owners(v)
				if v.Field == nil || v.Field.Party != 2 || origin == [2]int{v.X, v.Y} {
					t.Fatal("original reordered party did not move together")
				}
				save(false)
				if err := os.WriteFile(campaignEvidencePath("campaign-return-island-state.json", players), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": players, "scope": "original post-rescue aunt/escort dialogue, return sailing and island on-enter party reorder; role owners, original reading roster, saved scripts and ordinary two-member movement; not later palace/home events or full campaign", "scene": v.Scene, "map": v.Map, "cash": v.Cash, "party": v.Party, "leader": v.Leader, "readings": proof.Readings, "saves": proof.Saves, "reorderedPartySaved": proof.OrderSaved, "sailingSaved": proof.SailingSaved, "originalReaders": players, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-return-island.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			walker.allowTouch = v.Scene == 6
			x, y, reach := 1152, 384, 32
			switch v.Scene {
			case 2:
			case 4:
				x, y = 1088, 1648
				if v.X == 1296 && v.Y == 1368 {
					proof.Balcony = true
				}
				if proof.Balcony && v.X >= 1200 && v.Y >= 1456 {
					proof.Downstairs = true
				}
				if !proof.Balcony {
					x, y, reach = 1296, 1368, 0
				} else if !proof.Downstairs {
					x, y, reach = 1168, 1368, 0
				}
				if proof.Aunt {
					x, y = 976, 1704
				}
			case 5:
				x, y = 1824, 1328
			case 6:
				if !proof.Boat {
					g = campaignWalkTo(t, m, &c, g, v, 1152, 1376, 124)
				} else {
					g = walker.walk(t, m, &c, g, v, 1184, 1424, 32)
				}
			default:
				t.Fatal("escort return left original route", v.Scene, v.X, v.Y)
			}
			if v.Scene != 6 {
				g = walker.walk(t, m, &c, g, v, x, y, reach)
			}
		} else if v.Battle != nil {
			t.Fatal("unexpected battle before return ferry")
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("escort return position", step, v.Scene, v.X, v.Y, v.Phase)
		}
	}
	t.Fatal("original escort return ferry did not finish", proof)
	return m, c, g
}
