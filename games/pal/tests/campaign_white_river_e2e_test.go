package pal_test

// Test first: the actual cave exit/village/doctor transitions fail; reunion or
// the prescription is skipped; Yueru leaves or rejoins at the wrong time;
// the adjacent ginkgo search grants no item or duplicates it; fresh-runtime
// saves lose a reading/script/party/menu boundary or accept stale controls.
// Original event identities and coordinates are test walkthrough data only.
import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type whiteRiverProof struct {
	Reunion, Prescription, Split, Secret, Solo, Ginkgo bool
	Readings, Saves, Steps, Scans                      int
	LastPrompt                                         uint64
	Saved                                              map[string]bool
}

func TestOriginalWhiteRiverReunionPrescriptionAndGinkgoSave(t *testing.T) {
	name := "campaign-hidden-dragon-state.json"
	proof := whiteRiverProof{Saved: map[string]bool{}}
	if os.Getenv("PAL_WHITE_RIVER_CHECKPOINT") == "failure" {
		name = "white-river-failure-state.json"
		b, err := os.ReadFile(campaignEvidencePath("white-river-failure-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &proof); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal("run the genuine hidden-dragon route first", err)
	}
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	runWhiteRiverPrescription(t, m, c, g, proof)
}

func runWhiteRiverPrescription(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, proof whiteRiverProof) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	visits := map[[4]int]int{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"white-river-failure-state.json": g.Data, "white-river-failure-proof.json": raw(proof), "white-river-failure-view.json": raw(view(t, m, g, 0))} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	save := func(name string) {
		t.Helper()
		before := view(t, m, g, 0)
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
		normalize := func(v pal.View) pal.View {
			v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
			return v
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(view(t, m, g, 0)))) {
			t.Fatal("white river save changed original continuation", name)
		}
		if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("white river accepted pre-save controls", name)
		}
		proof.Saves++
		proof.Saved[name] = true
		if err := os.WriteFile(campaignEvidencePath("white-river-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for step := 0; step < 6000; step++ {
		proof.Steps++
		v := view(t, m, g, 0)
		for _, p := range v.Party {
			if p.Controller != 0 || p.Role != 0 && p.Role != 2 {
				t.Fatal("prescription altered original solo party", v.Party)
			}
		}
		if len(v.Party) == 1 {
			if !proof.Split || v.Party[0].Role != 0 {
				t.Fatal("Yueru left before her original medicine task", proof, v.Party)
			}
			proof.Solo = true
		}
		if d := v.Dialogue; d != nil {
			if d.ID != proof.LastPrompt {
				proof.LastPrompt = d.ID
				proof.Readings++
				proof.Reunion = proof.Reunion || strings.Contains(d.Text, "灵儿．．！！")
				proof.Prescription = proof.Prescription || strings.Contains(d.Text, "照单子上的配方煎药")
				proof.Split = proof.Split || strings.Contains(d.Text, "正午之前我会赶回来")
				proof.Secret = proof.Secret || strings.Contains(d.Text, "不想让他们知道这件事")
				t.Log("white river reading", proof.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
			}
			for _, mark := range []struct {
				name string
				done bool
			}{{"reunion", proof.Reunion}, {"prescription", proof.Prescription}, {"split-reading", proof.Split}, {"secret", proof.Secret}} {
				if mark.done && !proof.Saved[mark.name] {
					save(mark.name)
				}
			}
			v = view(t, m, g, 0)
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		} else if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			proof.Scans++
			if proof.Scans > 128 {
				t.Fatal("original ginkgo inventory did not settle", originalVisibleText(v, false))
			}
			n, found := campaignInventoryAmount(v, "银杏果")
			if found {
				if n != 1 || !proof.Solo || !proof.Secret {
					t.Fatal("original ginkgo missing/duplicated or party transition skipped", n, proof)
				}
				proof.Ginkgo = true
				if !proof.Saved["ginkgo-inventory"] {
					proof.Saved["ginkgo-inventory"] = true
					save("ginkgo-inventory")
					v = view(t, m, g, 0)
				}
			}
			keys := uint32(32)
			if proof.Ginkgo {
				keys = 1
			}
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 4, Keys: keys})
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if proof.Ginkgo {
				save("prescription-complete")
				if err := os.WriteFile(campaignEvidencePath("campaign-white-river-prescription-state.json", 1), g.Data, 0600); err != nil {
					t.Fatal(err)
				}
				report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual cave exit, White River village, Ling'er reunion and prescription, Yueru's original departure and private physician dialogue, ginkgo acquisition through ordinary adjacent search and actual inventory; not complete stage 6", "scene": v.Scene, "party": v.Party, "proof": proof}, "", "  ")
				if err := os.WriteFile(campaignEvidencePath("campaign-white-river-prescription.json", 1), report, 0600); err != nil {
					t.Fatal(err)
				}
				return m, c, g
			}
			stage := 0
			for _, done := range []bool{proof.Reunion, proof.Prescription, proof.Split, proof.Secret, proof.Solo} {
				if done {
					stage++
				}
			}
			place := [4]int{v.Scene, v.X, v.Y, stage}
			visits[place]++
			if visits[place] > 32 {
				t.Fatal("white river walkthrough repeats a position", place, v.Field, proof)
			}
			x, y := 0, 0
			switch v.Scene {
			case 49:
				x, y = 1824, 1504
			case 50:
				x, y = 1632, 704
			case 53:
				if !proof.Prescription {
					x, y = 864, 816
				} else if !proof.Solo {
					g = campaignWalkTo(t, m, &c, g, v, 848, 888, 898)
				} else if v.X != 496 || v.Y != 856 {
					x, y = 496, 856
				} else {
					keys, direction := uint32(8|2), 1
					if proof.Saved["ginkgo-search"] {
						keys, direction = 2048, 4
					} else {
						save("ginkgo-search")
						v = view(t, m, g, 0)
					}
					g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
				}
			case 54:
				if !proof.Prescription {
					if fieldTestDistance(v.X, v.Y, 1392, 584) > 160 {
						g = walker.walk(t, m, &c, g, v, 1392, 584, 96)
					} else {
						g = campaignWalkTo(t, m, &c, g, v, 1392, 584, 906)
					}
				} else {
					x, y = 1312, 896
				}
			default:
				t.Fatal("white river left the original prescription route", v.Scene, v.X, v.Y)
			}
			if x != 0 {
				g = walker.walk(t, m, &c, g, v, x, y, 0)
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if step%100 == 0 {
			t.Log("white river position", step, v.Scene, v.X, v.Y, v.Phase, v.Party)
		}
	}
	t.Fatal("original prescription/ginkgo flow never completed", proof)
	return m, c, g
}
