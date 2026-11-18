package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func originalAvailableMagic(v pal.View, name string) bool {
	type cell struct{ x, y int }
	glyphs := map[cell][16]int{}
	for _, n := range caveEquipmentNodes(v) {
		if n[0] == 2 && n[8] != 0 {
			glyphs[cell{n[6], n[7]}] = n
		}
	}
	for p, n := range glyphs {
		if p.y < 54 || p.y > 172 || (p.x-35)%87 != 0 || p.x < 35 || p.x > 209 || n[8] == 24 || n[8] == 28 {
			continue
		}
		matches := true
		for i, c := range []rune(name) {
			g, ok := glyphs[cell{p.x + i*16, p.y}]
			matches = matches && ok && g[5] == int(c) && g[8] != 24 && g[8] != 28
		}
		if matches {
			return true
		}
	}
	return false
}

func TestOriginalTowerKingPublicSpellStrategyAndSavedVictory(t *testing.T) {
	if os.Getenv("PAL_TOWER_KING_BOT") != "1" {
		t.Skip("explicit original king production-bot regression")
	}
	name := "tower-king-bot-source-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "b87e25b84a23813bbfe3c778ea1964c7c6b275465a36d56ee27b205e441a743e" {
		t.Fatal("retained original king battle changed")
	}
	r := loadRiverMedicine(t, name)
	initial := r.visible()
	if initial.Battle == nil || initial.Scene != 146 || len(initial.Party) != 3 {
		t.Fatal("original king regression source is not a three-role actual battle")
	}
	trace := []any{}
	prefix := "tower-king-bot"
	if run := os.Getenv("PAL_TOWER_KING_RUN"); run != "" {
		if run != "ranked" && run != "recovery" && run != "final" && run != "economy" {
			t.Fatal("unsupported king regression artifact prefix", run)
		}
		prefix = "tower-king-" + run
	}
	commands, reinforcement, advanced, saved := 0, false, false, false
	defer func() {
		for suffix, data := range map[string][]byte{"trace": raw(trace), "view": raw(r.visible()), "state": r.g.Data} {
			if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 7000; step++ {
		v := r.visible()
		if step%200 == 0 {
			t.Log("king public progression", step, commands, v.Party, v.Battle)
		}
		if v.Battle == nil {
			if v.Cash <= initial.Cash || !reinforcement || !advanced || !saved {
				t.Fatal("original king production bot failed to win with observed spell/save evidence", v.Cash, v.Party, reinforcement, advanced, saved)
			}
			report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceSHA256": hex.EncodeToString(hash[:]), "timerObservations": step, "commands": commands, "reinforcementsObserved": reinforcement, "advancedSpellSelected": advanced, "acceptedActionSaved": saved, "cashBefore": initial.Cash, "cashAfter": v.Cash, "party": v.Party, "scope": "production bot from unchanged actual king battle; public spell menu only, original reinforcement and normal victory, selected action fresh-runtime save and stale input rejected; no native or rule-table edits"})
			if err := os.WriteFile(campaignEvidencePath(prefix+".json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		alive, enemies := false, 0
		for _, p := range v.Party {
			alive = alive || p.HP > 0
		}
		if !alive {
			t.Fatal("original king bot defeated", step, v.Party)
		}
		for _, e := range v.Battle.Enemies {
			if e.Object != 0 {
				enemies++
				reinforcement = reinforcement || e.Object != 519
			}
		}
		if !saved && advanced {
			accepted := false
			for _, p := range v.Battle.Players {
				accepted = accepted || p.State == 2 && p.Action == 3
			}
			if accepted {
				before := v
				r.g, err = r.m.SaveControl(r.c, r.g, true)
				if err != nil {
					t.Fatal(err)
				}
				r.c.Now = r.c.Now.Add(7 * 24 * time.Hour)
				r.m, _ = nativeModule(t)
				r.g, err = r.m.ResumeSaved(r.c, r.g)
				if err != nil {
					t.Fatal(err)
				}
				normalize := func(v pal.View) pal.View {
					v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
					return v
				}
				if !bytes.Equal(raw(normalize(before)), raw(normalize(r.visible()))) {
					t.Fatal("king accepted magic save changed public continuation")
				}
				if _, err := r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
					t.Fatal("king save accepted stale controls")
				}
				saved = true
				v = r.visible()
			}
		}
		a, err := r.m.BotAction(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) > 0 {
			var action pal.Action
			if err := json.Unmarshal(a, &action); err != nil {
				t.Fatal(err)
			}
			text := originalVisibleText(v, true)
			if v.Battle.State == 1 && v.Battle.Menu == 1 && action.Keys == 2 {
				if enemies > 1 && strings.Contains(text, "御剑术") && originalAvailableMagic(v, "万剑诀") {
					t.Fatal("bot confirms lowest sword spell while affordable observed group sword spell is visible", step, text)
				}
				advanced = advanced || strings.Contains(text, "万剑诀") || strings.Contains(text, "五雷咒") || strings.Contains(text, "旋风咒")
			}
			if action.Keys != 0 || step%40 == 0 {
				trace = append(trace, map[string]any{"step": step, "action": action, "battle": v.Battle, "party": v.Party, "selected": text})
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
			commands++
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("original king public spell strategy did not complete", commands)
}
