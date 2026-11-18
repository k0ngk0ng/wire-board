package pal_test

// Test first using the actual unedited inn ambush checkpoint: offensive-only
// defeat, missing affordable healing, wrong ally, lost selected healing on
// save, duplicate healing after resume, or scripted departure losing readers.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalAmbushRecoveryBotSavedHealing(t *testing.T) {
	b, err := os.ReadFile(campaignEvidencePath("ambush-battle-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(b)
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	cash := v.Cash
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{true}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	saved, healed := false, false
	lastHP := map[int]int{}
	selectedSpells := map[int]string{}
	pendingHealing := [][4]int{}
	preservedHealingAuto := 0
	trace := []any{}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{
				"battle-bot-recovery-failure-state.json": g.Data,
				"battle-bot-recovery-failure-trace.json": raw(trace),
			} {
				if e := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); e != nil {
					t.Log(e)
				}
			}
		}
	}()
	for step := 0; step < 2400; step++ {
		v = view(t, m, g, 0)
		if v.GameOver != nil {
			t.Fatal("recovery bot reached a real original game-over boundary", v.GameOver, v.Party)
		}
		if v.Cash < cash {
			t.Fatal("recovery bot lost original ambush", v.Cash, v.Party)
		}
		if v.Battle != nil {
			if len(pendingHealing) > 0 && (v.Battle.Phase != 0 || v.Battle.Players[len(v.Party)-1].State == 2) {
				for _, expected := range pendingHealing {
					p := v.Battle.Players[expected[0]]
					if p.Action != expected[1] || p.Object != expected[2] || p.Target != expected[3] {
						t.Fatal("last-role original auto replaced accepted healing", expected, p)
					}
				}
				preservedHealingAuto++
				pendingHealing = nil
			}
			if v.Battle.Phase != 0 {
				clear(selectedSpells)
			}
			for _, p := range v.Party {
				if old, ok := lastHP[p.Role]; ok && p.HP > old {
					healed = true
				}
				lastHP[p.Role] = p.HP
			}
			if !saved && v.Battle.Menu == 1 && strings.Contains(originalVisibleText(v, true), "观音咒") {
				before := v
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
				if !reflect.DeepEqual(before.Battle, after.Battle) || !reflect.DeepEqual(before.Party, after.Party) || !reflect.DeepEqual(before.Raster, after.Raster) {
					t.Fatal("saved healing choice changed native menu")
				}
				saved = true
			}
		}
		if v.Phase == "field" && v.Scene == 2 && len(v.Party) == 1 {
			if !saved || !healed || v.Cash <= cash || preservedHealingAuto == 0 {
				t.Fatal("recovery proof incomplete", saved, healed, v.Cash, preservedHealingAuto)
			}
			report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "production recovery bot from actual original inn ambush checkpoint through victory/departure; public HP healing and selected original spell saved in fresh runtime; not full campaign/autoplay", "candidate": campaignVariant(), "sourceCheckpointSHA256": hex.EncodeToString(sourceHash[:]), "players": 1, "healed": healed, "selectedHealingSaved": saved, "acceptedHealingPreservedAcrossLastRoleAuto": preservedHealingAuto, "cashBefore": cash, "cashAfter": v.Cash, "ordinaryInputTrace": trace}, "", "  ")
			if err := os.WriteFile(campaignEvidencePath("battle-bot-recovery.json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		for actor := 0; actor < c.Players; actor++ {
			c.Actor = actor
			a, err := m.BotAction(c, g)
			if err != nil {
				t.Fatal(err)
			}
			if len(a) > 0 {
				var action pal.Action
				if err := json.Unmarshal(a, &action); err != nil {
					t.Fatal(err)
				}
				if action.Keys != 0 && v.Battle != nil {
					trace = append(trace, map[string]any{"step": step, "battle": v.Battle, "party": v.Party, "keys": action.Keys, "selected": originalVisibleText(v, true)})
					if action.Keys == 2 && v.Battle.State == 1 && v.Battle.Menu == 1 {
						selectedSpells[v.Battle.Current] = originalVisibleText(v, true)
					}
					if action.Keys == 32768 && v.Battle.Current == len(v.Party)-1 {
						for i, p := range v.Battle.Players[:v.Battle.Current] {
							spell := selectedSpells[i]
							if p.State == 2 && p.Action == 3 && (strings.Contains(spell, "气疗术") || strings.Contains(spell, "观音咒")) {
								pendingHealing = append(pendingHealing, [4]int{i, p.Action, p.Object, p.Target})
							}
						}
					}
				}
				g, err = m.Apply(c, g, a)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("recovery bot never finished original ambush")
}
