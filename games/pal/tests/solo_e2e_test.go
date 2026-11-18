package pal_test

// Test first for the user's single-player scope: metadata/configuration still
// admit multiple seats; an original multi-account save resumes; one human loses
// control of another party member; manual input is replaced by autoplay; saving
// loses the selected role, resources or pending menu; old controls survive.
// Combat uses the explicit original PAL_StartBattle fixture, not story progress.
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

func TestSoloOriginalPartyManualAndSavedContinuation(t *testing.T) {
	m := originalBattleModule(t)
	if m.MinPlayers != 1 || m.MaxPlayers != 1 || !m.SupportsSave {
		t.Fatal("single-player metadata is not authoritative", m.MinPlayers, m.MaxPlayers, m.SupportsSave)
	}
	if _, err := m.Configure(1, raw(map[string]any{})); err != nil {
		t.Fatal(err)
	}
	for _, players := range []int{0, 2, 3} {
		if _, err := m.Configure(players, raw(map[string]any{})); err == nil {
			t.Fatal("solo original game admitted another seat count", players)
		}
	}
	// Reject the unsupported seat header before attempting to interpret an
	// obsolete native payload. No archived local research save is required.
	legacy := raw(map[string]any{"players": 3})
	if _, err := m.ResumeSaved(gameapi.Context{Players: 3, Now: time.Unix(1800000000, 0)}, gameapi.Match{Schema: 1, Data: legacy}); err == nil || !strings.Contains(err.Error(), "单人") {
		t.Fatal("legacy multiplayer save was not rejected at the single-player boundary", err)
	}
	c := gameapi.Context{Players: 1, Actor: 0, Now: time.Unix(1800000000, 0), Automated: []bool{false}}
	g, err := m.Start(c, raw(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	commands := [3]int{}
	saved := false
	for step := 0; step < 2048; step++ {
		v := view(t, m, g, 0)
		for _, p := range v.Party {
			if p.Controller != 0 {
				t.Fatal("single human cannot control every original party member", v.Party)
			}
		}
		if v.Cash == 3 {
			if !saved || commands[0] == 0 || commands[1] == 0 || commands[2] == 0 {
				t.Fatal("manual full-party proof incomplete", saved, commands)
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "single-seat admission and legacy multi-save rejection; manual original three-member battle, fresh-runtime save and unchanged original three-cash result; isolated original battle fixture, not campaign completion", "commandsByRole": commands, "manualWholeParty": true, "saved": saved, "cash": v.Cash}, "", "  ")
			if err := os.WriteFile(campaignEvidencePath("solo-manual-party.json", 3), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if b := v.Battle; b != nil && b.Phase == 0 && b.State != 0 && v.Seq == v.AppliedSeq {
			if b.Current < 0 || b.Current >= len(v.Party) {
				t.Fatal("invalid original party turn")
			}
			// A human uses the ordinary original attack icon and target menu.
			// No BotAction or automated input is used in this scenario.
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, ControlAt: v.ControlAt, Seq: v.Seq + 1, Direction: 4, Keys: 2})
			commands[v.Party[b.Current].Role]++
			if !saved && b.Current == 1 {
				before := view(t, m, g, 0)
				g, err = m.SaveControl(c, g, true)
				if err != nil {
					t.Fatal(err)
				}
				c.Now = c.Now.Add(24 * time.Hour)
				m = originalBattleModule(t)
				g, err = m.ResumeSaved(c, g)
				if err != nil {
					t.Fatal(err)
				}
				after := view(t, m, g, 0)
				if !bytes.Equal(raw(before.Party), raw(after.Party)) || !bytes.Equal(raw(before.Battle), raw(after.Battle)) || before.Cash != after.Cash || after.Epoch == before.Epoch {
					t.Fatal("solo save changed the original party/menu or kept old controls")
				}
				if _, err := m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, ControlAt: before.ControlAt, Seq: before.Seq + 1, Direction: 4, Keys: 2})); err == nil {
					t.Fatal("solo restore accepted pre-save controls")
				}
				saved = true
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("single human could not finish original full-party battle")
}
