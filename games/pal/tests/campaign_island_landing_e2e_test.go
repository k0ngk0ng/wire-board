package pal_test

// Test first: the original gift is absent or granted twice after resume;
// inventory/partial reading loses state; the boatman retains old dialogue;
// a boat script cannot resume; arrival traps an unsupported rule call; rewards,
// party or controls change on save. Coordinates are test walkthrough input.
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

func TestOriginalCampaignHammerAndIslandLandingSave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) { originalIslandLanding(t, players) })
	}
}

func originalIslandLanding(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalIslandMedicineRequest(t, players)
	hammer, instructions, repeat, boat, landed := false, false, false, false, false
	giftSaved, boatSaved, arrivalSaved := false, false, false
	inventories, scans, saves := 0, 0, 0
	pages := []string{}
	last := uint64(0)
	save := func(partial bool) {
		t.Helper()
		v := view(t, m, g, 0)
		if partial && players > 1 {
			c.Actor = 0
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			v = view(t, m, g, 0)
			for actor, read := range v.Dialogue.Read {
				if read != (actor == 0) {
					t.Fatal("unexpected reader before island save", actor, read)
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
		if before.Scene != after.Scene || before.Map != after.Map || before.X != after.X || before.Y != after.Y || before.Cash != after.Cash || !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Dialogue), raw(after.Dialogue)) || !bytes.Equal(raw(before.Menu), raw(after.Menu)) || !bytes.Equal(raw(before.Overlay), raw(after.Overlay)) || !bytes.Equal(raw(before.Raster), raw(after.Raster)) {
			t.Fatal("island save changed original progress, reading or drawing")
		}
		saves++
	}
	for step := 0; step < 1536; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != last {
				last = d.ID
				pages = append(pages, d.Text)
				t.Log("island departure story", v.Scene, v.X, v.Y, d.Text)
				hammer = hammer || strings.Contains(d.Text, "收下破天锤")
				instructions = instructions || strings.Contains(d.Text, "六具阿修罗")
				repeat = repeat || strings.Contains(d.Text, "等你的好消息")
				boat = boat || strings.Contains(d.Text, "走吧") && strings.Contains(d.Text, "上船")
				landed = landed || strings.Contains(d.Text, "总算靠岸")
			}
			if hammer && !giftSaved || landed && !arrivalSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				if landed {
					arrivalSaved = true
				} else {
					giftSaved = true
				}
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" {
			c.Actor = v.Leader
			if v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
				if amount, found := campaignInventoryAmount(v, "破天锤"); found {
					if amount != 1 {
						t.Fatal("original gift missing or duplicated", inventories, amount)
					}
					save(false)
					v = view(t, m, g, 0)
					inventories++
					scans = 0
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 1})
				} else {
					if scans >= 64 {
						t.Fatal("original inventory has no hammer", originalVisibleText(v, false))
					}
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 32})
					scans++
				}
			}
		} else if v.Phase == "field" {
			if v.Scene == 15 {
				if !hammer || !instructions || !repeat || !boat || !landed || !giftSaved || !boatSaved || !arrivalSaved || inventories != 2 || v.Map != 3 || v.Cash != 550 || len(v.Party) != 1 || v.Party[0].Role != 0 {
					t.Fatal("original departure/landing incomplete", hammer, instructions, repeat, boat, landed, inventories, v.Map, v.Cash)
				}
				save(false)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original hammer/forgetfulness gift, six-statue instructions, repeated giver conversation with exactly one visible hammer, boat preparation and island landing; saved reading/inventory/scripts and fresh runtimes; not island maze or full campaign", "players": players, "pages": pages, "cash": v.Cash, "scene": v.Scene, "map": v.Map, "hammerAmount": 1, "inventoryChecks": inventories, "giftReadingSaved": giftSaved, "boatPreparationSaved": boatSaved, "arrivalReadingSaved": arrivalSaved, "partialReadingSaved": players > 1, "saves": saves}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-island-landing.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			if hammer && inventories == 0 || repeat && inventories == 1 {
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
			} else {
				x, y, npc := 1184, 784, 0
				switch v.Scene {
				case 3:
				case 4:
					x, y, npc = 1024, 1664, 60
					if repeat && inventories == 2 {
						x, y, npc = 976, 1704, 0
					}
				case 5:
					x, y = 1824, 1328
				case 6:
					x, y, npc = 1152, 1376, 124
					if boat {
						x, y, npc = 1184, 1424, 0
					}
				default:
					t.Fatal("island walkthrough left original route", v.Scene, v.X, v.Y)
				}
				g = campaignWalkTo(t, m, &c, g, v, x, y, npc)
				if step%50 == 0 {
					t.Log("island walk", step, v.Scene, v.X, v.Y, x, y)
				}
			}
		} else if boat && !boatSaved && v.Scene == 6 && v.Phase == "script" {
			save(false)
			boatSaved = true
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("original island departure did not finish", v.Scene, v.X, v.Y, hammer, repeat, boat, landed, inventories)
	return m, c, g
}

// Read the inventory's displayed label and cyan amount sprites. The original
// 98 layout omits the amount for a single item; no inventory memory is exposed.
func campaignInventoryAmount(v pal.View, name string) (int, bool) {
	if v.Menu.Kind != 5 {
		return 0, false
	}
	seen := map[int]bool{}
	nodes := [][16]int{}
	var visit func(int)
	visit = func(id int) {
		if id <= 0 || id > len(v.Raster.Nodes) || seen[id] {
			return
		}
		seen[id] = true
		n := v.Raster.Nodes[id-1]
		visit(n[1])
		visit(n[2])
		nodes = append(nodes, n)
	}
	if v.Raster != nil {
		visit(v.Raster.Root)
	} else {
		for _, op := range v.Overlay {
			switch op.Kind {
			case 0:
				nodes = append(nodes, [16]int{0: 2, 5: op.ID, 6: op.X, 7: op.Y, 8: op.Color})
			case 1:
				nodes = append(nodes, [16]int{0: 1, 5: op.Source, 6: op.ID, 7: op.Frame, 8: op.X, 9: op.Y})
			}
		}
	}
	letters := []rune(name)
	for _, first := range nodes {
		if first[0] != 2 || first[5] != int(letters[0]) || first[8] == 0 || first[6]%100 != 15 || first[7] < 12 || first[7] > 120 {
			continue
		}
		matched := 0
		for _, n := range nodes {
			if n[0] == 2 && n[8] != 0 && n[7] == first[7] && n[6] >= first[6] && (n[6]-first[6])%16 == 0 {
				at := (n[6] - first[6]) / 16
				if at < len(letters) && n[5] == int(letters[at]) {
					matched |= 1 << at
				}
			}
		}
		if matched != (1<<len(letters))-1 {
			continue
		}
		amount, numbered := 0, false
		for _, n := range nodes {
			if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] >= 56 && n[7] <= 65 && n[9] == first[7]+5 {
				switch n[8] - first[6] {
				case 87:
					amount += n[7] - 56
					numbered = true
				case 81:
					amount += (n[7] - 56) * 10
					numbered = true
				}
			}
		}
		if !numbered {
			amount = 1
		}
		return amount, true
	}
	return 0, false
}
