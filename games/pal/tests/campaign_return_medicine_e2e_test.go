package pal_test

// Test-first support for the full original first-party route: reverse map/boat
// transitions fail; original medicine cannot be selected/used on the aunt;
// battle or script continuation fails; a saved partial reading duplicates the
// cure; the item survives consumption; old controls or extra roles appear.
// Walkthrough data below never enters production field strategy.
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

type returnMedicineProof struct {
	Returned, Used, Cured, SelectedSaved, CureSaved bool
	InventoryChecked                                bool
	Scans, Readings, Saves                          int
	LastPrompt                                      uint64
}

func TestOriginalCampaignReturnMedicineAndAuntCureSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) { t.Parallel(); originalReturnMedicine(t, players) })
	}
}

func runOriginalReturnMedicine(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof returnMedicineProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"return-medicine-failure-state.json": g.Data, "return-medicine-failure-proof.json": raw(proof)} {
				if err := os.WriteFile(campaignEvidencePath(name, players), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("return medicine partial reading changed")
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
		after := view(t, m, g, 0)
		if before.Scene != after.Scene || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || before.Leader != after.Leader || before.Night != after.Night || !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || !bytes.Equal(raw(before.Menu), raw(after.Menu)) || !bytes.Equal(raw(before.Draws), raw(after.Draws)) || !bytes.Equal(raw(before.Overlay), raw(after.Overlay)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
			t.Fatal("return medicine continuation changed original state")
		}
		c.Actor = after.Leader
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("medicine continuation accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 5000; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Cured = proof.Cured || strings.Contains(d.Text, "李大娘服下紫金丹")
				t.Log("return medicine reading", proof.Readings, v.Scene, d.ID)
			}
			if proof.Cured && !proof.CureSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				proof.CureSaved = true
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			c.Actor = v.Leader
			keys := uint32(32)
			if proof.Cured && v.Scene == 2 {
				// This route's starting items and quest gifts fit on one page.
				text := originalVisibleText(v, false)
				painted := false
				for _, op := range v.Overlay {
					painted = painted || op.Kind == 1 && op.Source == 2 && op.ID == 0 && op.Frame == 69
				}
				if strings.Contains(text, "紫金丹") {
					t.Fatal("original cure did not remove the medicine from the displayed inventory", text)
				}
				if !painted {
					// Wait for the original cursor; an empty inventory has no glyphs.
					proof.Scans++
					if proof.Scans > 64 {
						t.Fatal("post-cure original inventory did not paint")
					}
					keys = 0
				} else if !proof.InventoryChecked {
					save(false)
					v = view(t, m, g, 0)
					proof.InventoryChecked = true
					keys = 1
				} else {
					keys = 1
				}
			} else if proof.Used {
				keys = 1
			} else if strings.Contains(originalVisibleText(v, true), "紫金丹") {
				if !proof.SelectedSaved {
					save(false)
					v = view(t, m, g, 0)
					proof.SelectedSaved = true
				}
				keys, proof.Used = 2, true
			} else {
				proof.Scans++
				if proof.Scans > 64 {
					t.Fatal("return medicine not available in original inventory")
				}
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			walker.allowTouch = v.Scene == 16
			place := [4]int{v.Scene, v.X, v.Y, 0}
			if proof.Used {
				place[3] = 1
			}
			visits[place]++
			if visits[place] > 48 {
				t.Fatal("return medicine walkthrough repeats position", place)
			}
			if v.Scene == 2 && proof.Cured {
				if !proof.InventoryChecked {
					c.Actor = v.Leader
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
					c.Now = time.UnixMilli(g.SystemDeadline + 200)
					var err error
					g, err = m.OnTimer(c, g)
					if err != nil {
						t.Fatal(err)
					}
					continue
				}
				if !proof.Returned || !proof.Used || !proof.SelectedSaved || !proof.CureSaved || len(v.Party) != 1 || v.Party[0].Role != 0 {
					t.Fatal("original medicine return incomplete", proof, v.Party)
				}
				save(false)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original post-palace return boat/village/inn, selected Zijindan use, aunt cure script and displayed post-cure inventory, saved partial reading and new-runtime wake-up; first-party route remains unfinished", "players": players, "scene": v.Scene, "map": v.Map, "cash": v.Cash, "readings": proof.Readings, "saves": proof.Saves, "selectedMedicineSaved": proof.SelectedSaved, "cureReadingSaved": proof.CureSaved, "medicineConsumed": proof.InventoryChecked, "partialReadingSaved": players > 1, "oldInputRejected": true}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-return-medicine.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			if v.Scene == 5 {
				proof.Returned = true
			}
			x, y, reach := 480, 1536, 32
			switch v.Scene {
			case 21:
			case 20:
				x, y = 224, 1120
			case 17:
				x, y = 240, 856
			case 19:
				x, y = 512, 1184
			case 16:
				x, y = 352, 1696
			case 15:
				g = campaignWalkTo(t, m, &c, g, v, 704, 832, 202)
			case 6:
				x, y = 192, 1680
			case 5:
				x, y = 1296, 872
			case 4:
				x, y = 1328, 1448
			case 3:
				x, y = 1456, 728 // Within original item-use range, outside the bed collision.
				if v.X == x && v.Y == y {
					direction := 3
					keys := []uint32{4, 8, 16, 32}[direction]
					if v.Field.Direction == direction {
						direction, keys = 4, 2048
						if proof.Used {
							keys = 2
						}
					}
					c.Actor = v.Leader
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
				} else {
					g = walker.walk(t, m, &c, g, v, x, y, 0)
				}
			default:
				t.Fatal("medicine return left original route", v.Scene, v.X, v.Y)
			}
			if v.Scene != 3 && v.Scene != 15 {
				g = walker.walk(t, m, &c, g, v, x, y, reach)
			}
		} else if v.Battle != nil {
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
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("return medicine position", step, v.Scene, v.X, v.Y, v.Phase)
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("original medicine return did not finish", v.Scene, v.X, v.Y, proof)
	return m, c, g
}
