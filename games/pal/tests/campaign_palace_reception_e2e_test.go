package pal_test

// Original overnight reception must finish before returning the medicine.
// Failure cases: the NPC cannot trigger; a partial reading or script save
// changes the original party, position or rewards; old input survives resume.
import (
	"bytes"
	"os"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func originalReturnMedicine(t *testing.T, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	m, c, g := originalPalaceMedicine(t, players)
	m, c, g = runOriginalPalaceReception(t, m, c, g, players)
	return runOriginalReturnMedicine(t, m, c, g, players, returnMedicineProof{})
}

func runOriginalPalaceReception(t *testing.T, m gameapi.Module, c gameapi.Context, g gameapi.Match, players int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	walker := campaignMapWalker{}
	readings, lastPrompt, saved := 0, uint64(0), false
	defer func() {
		if t.Failed() {
			if err := os.WriteFile(campaignEvidencePath("palace-reception-failure-state.json", players), g.Data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 1500; step++ {
		v := view(t, m, g, 0)
		if d := v.Dialogue; d != nil {
			if d.ID != lastPrompt {
				lastPrompt = d.ID
				readings++
			}
			if !saved {
				if players > 1 {
					c.Actor = 0
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
					v = view(t, m, g, 0)
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
				if before.Scene != v.Scene || before.X != v.X || before.Y != v.Y || before.Cash != v.Cash || !bytes.Equal(raw(before.Party), raw(v.Party)) || !bytes.Equal(raw(before.Dialogue), raw(v.Dialogue)) || !bytes.Equal(raw(before.Overlay), raw(v.Overlay)) || !bytes.Equal(raw(before.Raster), raw(v.Raster)) {
					t.Fatal("palace reception continuation changed original progress")
				}
				c.Actor = v.Leader
				if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
					t.Fatal("palace reception accepted pre-save input")
				}
				d, saved = v.Dialogue, true
			}
			if len(d.Read) != players {
				t.Fatal("palace reception changed reading roster")
			}
			for actor, read := range d.Read {
				if !read {
					c.Actor = actor
					g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
				}
			}
		} else if v.Phase == "field" && v.Seq == v.AppliedSeq {
			if readings > 0 {
				if !saved || v.Scene != 21 || v.Map != 9 || v.X != 528 || v.Y != 1512 || v.Cash != 550 || len(v.Party) != 1 || v.Party[0].Role != 0 {
					t.Fatal("original overnight reception incomplete", readings, v.Scene, v.X, v.Y, v.Party)
				}
				t.Log("original palace reception finished", readings, "reading stops")
				return m, c, g
			}
			g = walker.walk(t, m, &c, g, v, 496, 1528, 32)
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("original overnight reception did not finish")
	return m, c, g
}
