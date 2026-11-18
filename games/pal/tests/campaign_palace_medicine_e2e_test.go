package pal_test

// Test first: the garden guide/indoor transition cannot run; the original
// medicine is absent, duplicated by repeated dialogue, or lost on resume;
// shared reading changes; an NPC becomes an extra party member; a nonleader
// uses the shared inventory; saved menu drawing or event progress changes.
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

type palaceMedicineProof struct {
	Garden, Medicine, Named, Repeated bool
	GardenSaved, GiftSaved, NameSaved bool
	Inventories, Scans, Saves         int
	Readings                          int
	LastPrompt                        uint64
}

func TestOriginalCampaignPalaceMedicineAndNamedFairySave(t *testing.T) {
	for _, players := range []int{1} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			originalPalaceMedicine(t, players)
		})
	}
}

func originalPalaceMedicine(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalFairyPalaceEntrance(t, players)
	return runOriginalPalaceMedicine(t, m, c, g, players, palaceMedicineProof{})
}

func runOriginalPalaceMedicine(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int, proof palaceMedicineProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"palace-medicine-failure-state.json": g.Data, "palace-medicine-failure-proof.json": raw(proof)} {
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
					t.Fatal("palace partial reading changed", actor, read)
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
			t.Fatal("palace medicine save changed original progress or drawing")
		}
		c.Actor = after.Leader
		old := pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2}
		if _, err := m.Apply(c, g, raw(old)); err == nil {
			t.Fatal("palace medicine accepted pre-save input")
		}
		proof.Saves++
	}
	for step := 0; step < 1500; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Garden = proof.Garden || strings.Contains(d.Text, "进去拿药")
				proof.Medicine = proof.Medicine || strings.Contains(d.Text, "紫金丹") && strings.Contains(d.Text, "服下")
				proof.Named = proof.Named || strings.Contains(d.Text, "我叫赵灵儿")
				proof.Repeated = proof.Repeated || strings.Contains(d.Text, "不许赖皮")
				t.Log("palace medicine reading", proof.Readings, v.Scene, d.ID)
			}
			if proof.Garden && !proof.GardenSaved || proof.Medicine && !proof.GiftSaved || proof.Named && !proof.NameSaved {
				save(true)
				v = view(t, m, g, 0)
				d = v.Dialogue
				if proof.Named {
					proof.NameSaved = true
				} else if proof.Medicine {
					proof.GiftSaved = true
				} else {
					proof.GardenSaved = true
				}
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			c.Actor = v.Leader
			if amount, found := campaignInventoryAmount(v, "紫金丹"); found {
				if amount != 1 {
					t.Fatal("original palace gift missing or duplicated", amount)
				}
				if players > 1 {
					wrong := c
					wrong.Actor = (v.Leader + 1) % players
					before := bytes.Clone(g.Data)
					if _, err := m.Apply(wrong, g, raw(pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4, Keys: 2})); err == nil || !bytes.Equal(before, g.Data) {
						t.Fatal("nonleader changed shared palace inventory")
					}
				}
				save(false)
				v = view(t, m, g, 0)
				proof.Inventories++
				proof.Scans = 0
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 1})
			} else {
				proof.Scans++
				if proof.Scans > 64 {
					t.Fatal("original palace inventory has no medicine")
				}
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 32})
			}
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if proof.Inventories == 2 {
				if !proof.GardenSaved || !proof.GiftSaved || !proof.NameSaved || !proof.Repeated || v.Scene != 21 || v.Map != 9 || v.Cash != 550 || len(v.Party) != 1 || v.Party[0].Role != 0 {
					t.Fatal("original palace gift story incomplete", proof, v.Scene, v.Map, v.Cash)
				}
				save(false)
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original palace garden guide, medicine gift and named fairy dialogue, repeated gift conversation and exactly one visible Zijindan, saved shared reading/inventory and fresh runtimes; not party joining or full campaign", "players": players, "scene": v.Scene, "map": v.Map, "cash": v.Cash, "partyRoles": []int{0}, "medicineAmount": 1, "inventoryChecks": proof.Inventories, "gardenReadingSaved": proof.GardenSaved, "giftReadingSaved": proof.GiftSaved, "nameReadingSaved": proof.NameSaved, "repeatedConversation": proof.Repeated, "partialReadingSaved": players > 1, "staleControlsRejected": true, "readings": proof.Readings, "saves": proof.Saves}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-palace-medicine.json", players), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			if proof.Named && (proof.Inventories == 0 || proof.Repeated && proof.Inventories == 1) {
				c.Actor = v.Leader
				g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: 2048})
			} else if v.Scene == 20 {
				x, y, reach := 864, 800, 96
				if proof.Garden {
					x, y, reach = 1088, 592, 64
				}
				g = walker.walk(t, m, &c, g, v, x, y, reach)
			} else if v.Scene == 21 {
				if proof.Inventories == 1 {
					g = campaignWalkTo(t, m, &c, g, v, 1152, 944, 343)
				} else {
					g = walker.walk(t, m, &c, g, v, 1152, 944, 32)
				}
			} else {
				t.Fatal("palace medicine left original route", v.Scene, v.X, v.Y)
			}
		} else if v.Battle != nil {
			t.Fatal("unexpected battle during original medicine gift")
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := view(t, m, g, 0)
	t.Fatal("original palace medicine did not finish", v.Scene, v.X, v.Y, proof)
	return m, c, g
}
