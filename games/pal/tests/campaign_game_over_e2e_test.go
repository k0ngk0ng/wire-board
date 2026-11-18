package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestOriginalCampaignNaturalDefeatAndSavedRecovery(t *testing.T) {
	if os.Getenv("PAL_GAME_OVER_CAMPAIGN") != "1" {
		t.Skip("explicit original new-game route, natural defeat and native recovery")
	}
	if campaignVariant() != "game-over" || developmentHealthProfile() != "" {
		t.Fatal("natural defeat requires the independent original-profile game-over campaign core")
	}
	var m gameapi.Module
	var c gameapi.Context
	var g gameapi.Match
	if os.Getenv("PAL_GAME_OVER_CHECKPOINT") == "earned" {
		data, err := os.ReadFile(campaignEvidencePath("game-over-earned-origin.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		var origin struct {
			Passed bool
			SHA256 string
		}
		if err = json.Unmarshal(data, &origin); err != nil || !origin.Passed {
			t.Fatal("missing earned original route receipt", err)
		}
		data, err = os.ReadFile(campaignEvidencePath("game-over-earned-battle-state.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != origin.SHA256 {
			t.Fatal("earned original battle checkpoint changed")
		}
		r := loadRiverMedicine(t, "game-over-earned-battle-state.json")
		m, c, g = r.m, r.c, r.g
	} else {
		m, c, g = originalReturnMedicine(t, 1)
		m, c, g = runOriginalFirstPartyJoin(t, m, c, g, 1, firstPartyProof{StopAtBattle: true})
		if view(t, m, g, 0).Battle == nil {
			t.Fatal("ordinary original route did not reach its first rescue battle")
		}
		if err := os.WriteFile(campaignEvidencePath("game-over-earned-battle-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256(g.Data)
		if err := os.WriteFile(campaignEvidencePath("game-over-earned-origin.json", 1), raw(map[string]any{"passed": true, "scope": "genuine original-profile new game through ordinary medicine, night lesson and first rescue battle entry; no native-state edits, only a source receipt for failure testing", "sha256": hex.EncodeToString(h[:])}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	initial := view(t, m, g, 0)
	if len(initial.Party) != 2 || initial.Party[0].Role != 0 || initial.Party[1].Role != 1 {
		t.Fatal("actual rescue source lost the original Li/Ling party", initial.Party)
	}
	for _, p := range initial.Party {
		if p.Controller != 0 || p.HP <= 0 || p.MaxHP >= 9999 {
			t.Fatal("rescue source changed original health or single-player ownership", initial.Party)
		}
	}
	var err error
	healthy, err := m.SaveControl(c, g, true)
	if err != nil || healthy.PreserveSavedCheckpoint {
		t.Fatal("healthy battle entry did not create a valid manual point", err)
	}
	c.Now = c.Now.Add(24 * time.Hour)
	m, _ = nativeModule(t)
	g, err = m.ResumeSaved(c, healthy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(campaignEvidencePath("game-over-healthy-match.json", 1), raw(healthy), 0600); err != nil {
		t.Fatal(err)
	}
	saved := map[string]bool{}
	trace := []map[string]any{}
	last := ""
	defenses := 0
	reading := false
	save := func(label string) {
		t.Helper()
		before := view(t, m, g, 0)
		paused, e := m.SaveControl(c, g, true)
		if e != nil {
			t.Fatal(e)
		}
		if before.GameOver != nil && !paused.PreserveSavedCheckpoint {
			t.Fatal("failure save would replace the healthy baseline", label)
		}
		if e = os.WriteFile(campaignEvidencePath("game-over-"+label+"-match.json", 1), raw(paused), 0600); e != nil {
			t.Fatal(e)
		}
		c.Now = c.Now.Add(24 * time.Hour)
		m, _ = nativeModule(t)
		g, e = m.ResumeSaved(c, paused)
		if e != nil {
			t.Fatal(e)
		}
		normalize := func(v pal.View) pal.View {
			v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
			return v
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(view(t, m, g, 0)))) {
			t.Fatal("actual defeat save lost original display or rules", label)
		}
		if _, e = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 1024})); e == nil {
			t.Fatal("actual defeat continuation admitted old input", label)
		}
		saved[label] = true
	}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"game-over-failure-state.json": g.Data, "game-over-failure-trace.json": raw(trace)} {
				if e := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); e != nil {
					t.Log(e)
				}
			}
		}
	}()
	for i := 0; i < 6000; i++ {
		v := view(t, m, g, 0)
		key := fmt.Sprint(v.Phase, v.Party, v.GameOver, v.PaletteEffect)
		if key != last {
			trace = append(trace, map[string]any{"step": i, "phase": v.Phase, "party": v.Party, "gameOver": v.GameOver, "palette": v.PaletteEffect, "cash": v.Cash})
			last = key
		}
		if v.Cash != initial.Cash || g.Result != nil {
			t.Fatal("natural defeat changed cash or issued a terminal result", v.Cash, g.Result)
		}
		totalHP := 0
		for _, p := range v.Party {
			totalHP += p.HP
		}
		if totalHP > 0 && totalHP <= 12 && v.Battle != nil && v.Battle.Phase == 0 && v.Battle.State == 1 && v.Seq == v.AppliedSeq && !saved["earned-low-health"] {
			save("earned-low-health")
			if err = os.WriteFile(campaignEvidencePath("game-over-low-health-state.json", 1), g.Data, 0600); err != nil {
				t.Fatal(err)
			}
			h := sha256.Sum256(g.Data)
			if err = os.WriteFile(campaignEvidencePath("game-over-low-health-origin.json", 1), raw(map[string]any{"passed": true, "scope": "same earned original battle after only ordinary defense, alive original input boundary with low health; no stat/RNG/resource edits; browser entry only", "sha256": hex.EncodeToString(h[:])}), 0600); err != nil {
				t.Fatal(err)
			}
			v = view(t, m, g, 0)
		}
		if v.Phase == "gameover" {
			if !reading || defenses == 0 || !g.Restartable || !g.PreserveSavedCheckpoint || g.SystemDeadline != 0 {
				t.Fatal("original failure did not stop for authorized recovery", v.GameOver)
			}
			if !saved["reload-wait"] {
				save("reload-wait")
			}
			before := bytes.Clone(g.Data)
			c.Now = c.Now.Add(30 * 24 * time.Hour)
			g, err = m.OnTimer(c, g)
			if err != nil || !bytes.Equal(before, g.Data) {
				t.Fatal("retained native failure advanced during an idle timer", err)
			}
			break
		}
		if v.GameOver != nil && v.PaletteEffect.RedStep >= 16 && !saved["red-middle"] {
			save("red-middle")
			v = view(t, m, g, 0)
		}
		allDead := true
		for _, p := range v.Party {
			allDead = allDead && p.HP == 0
		}
		if allDead && !saved["all-defeated"] {
			save("all-defeated")
			v = view(t, m, g, 0)
		}
		if d := v.Dialogue; d != nil {
			if strings.Contains(d.Text, "胜败乃兵家常事也") && strings.Contains(d.Text, "大侠请重新来过吧") {
				reading = true
				if !saved["original-failure-reading"] {
					save("original-failure-reading")
					v = view(t, m, g, 0)
					d = v.Dialogue
				}
			}
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: d.ID})
		} else if b := v.Battle; b != nil && b.Phase == 0 && b.State == 1 && v.Seq == v.AppliedSeq {
			g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4, Keys: 1024})
			defenses++
		}
		if g.SystemDeadline <= 0 {
			t.Fatal("original failure lost a required timer before recovery state", v.Phase)
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
		if i == 5999 {
			t.Fatal("ordinary defense never reached original defeat", view(t, m, g, 0).Party)
		}
	}
	failure := g
	if view(t, m, g, 0).Phase != "gameover" {
		t.Fatal("original defeat boundary missing")
	}
	g, err = m.RestoreCheckpoint(c, healthy, failure)
	if err != nil {
		t.Fatal(err)
	}
	restored := view(t, m, g, 0)
	if restored.GameOver != nil || restored.Phase != "battle" || !bytes.Equal(raw(restored.Party), raw(initial.Party)) || !bytes.Equal(raw(restored.Battle), raw(initial.Battle)) || restored.Epoch <= view(t, m, failure, 0).Epoch {
		t.Fatal("actual original defeat did not restore the healthy battle boundary", restored.Party)
	}
	for label := range map[string]bool{"all-defeated": true, "red-middle": true, "original-failure-reading": true, "reload-wait": true} {
		if !saved[label] {
			t.Fatal("actual failure continuation save omitted", label)
		}
	}
	report := raw(map[string]any{"passed": true, "scope": "genuine original-profile new-game route to first rescue battle, ordinary defense and natural defeat, actual32-step original failure script/message/reload boundary, failure saves and fresh runtimes, idle no-op and healthy manual recovery; no stats/resources/RNG/outcome edits; browser and platform room save semantics are separate", "resumedEarnedSource": os.Getenv("PAL_GAME_OVER_CHECKPOINT") == "earned", "initialParty": initial.Party, "defenseCommands": defenses, "saved": saved, "trace": trace})
	for name, data := range map[string][]byte{"game-over-actual.json": report, "game-over-recovered-state.json": g.Data} {
		if err = os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
