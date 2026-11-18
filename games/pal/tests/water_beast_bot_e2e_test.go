package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalWaterBeastPublicRecoveryAndSavedVictory(t *testing.T) {
	if os.Getenv("PAL_WATER_BEAST_BOT") != "1" {
		t.Skip("explicit genuine water beast public bot replay")
	}
	name := "water-beast-bot-source-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "a4c53ba9933a340c6c7f83e98d2175f49db74bca1b905b5f06aa9bafa88a2b0c" {
		t.Fatal("genuine original water beast source changed")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "water-beast-bot", proof: caiyiForestProof{Saved: map[string]bool{}}}
	initial := r.visible()
	if initial.Scene != 253 || initial.Battle == nil || len(initial.Party) != 2 || initial.Party[0].Role != 3 || initial.Party[1].Role != 0 {
		t.Fatal("water beast replay is not the earned queen/Li battle")
	}
	trace := []any{}
	dead, revived, saved, victory := false, false, false, false
	targetSaved := false
	reviveSelection := map[int]bool{}
	reward, commands := -1, 0
	previousHP := []int{initial.Party[0].HP, initial.Party[1].HP}
	defer func() {
		for kind, data := range map[string][]byte{"trace": raw(trace), "view": raw(r.visible()), "state": r.g.Data} {
			if err := os.WriteFile(campaignEvidencePath("water-beast-bot-"+kind+".json", 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 15000; step++ {
		v := r.visible()
		if v.Battle == nil {
			if !victory || reward < 0 || v.Cash != initial.Cash+uint32(reward) || !dead || !revived || !saved || !targetSaved {
				t.Fatal("water beast did not complete original victory/revival/save evidence", victory, reward, dead, revived, saved, targetSaved, v.Party)
			}
			report := raw(map[string]any{"passed": true, "scope": "production policy from unchanged earned water beast source; only public observations and ordinary inputs; actual ally revival, normal victory, selected dead target and accepted revival action fresh-runtime saves, stale input rejection", "sourceSHA256": hex.EncodeToString(hash[:]), "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "timerObservations": step, "commands": commands, "allyDied": dead, "allyRevived": revived, "acceptedRevivalSaved": saved, "selectedDeadTargetSaved": targetSaved, "cashBefore": initial.Cash, "cashAfter": v.Cash, "rewardCash": reward, "party": v.Party})
			if err := os.WriteFile(campaignEvidencePath("water-beast-bot-result.json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		alive, anyDead, accepted := false, false, false
		for i, member := range v.Party {
			alive = alive || member.HP > 0
			anyDead = anyDead || member.HP == 0
			if previousHP[i] == 0 && member.HP > 0 {
				revived = true
			}
			previousHP[i] = member.HP
		}
		if !alive {
			t.Fatal("genuine water beast production policy defeated", step, commands, v.Party)
		}
		if anyDead && !dead {
			dead = true
			data := append([]byte(nil), r.g.Data...)
			digest := sha256.Sum256(data)
			for kind, data := range map[string][]byte{"state": data, "view": raw(v), "receipt": raw(map[string]any{"sourceSHA256": hex.EncodeToString(hash[:]), "stateSHA256": hex.EncodeToString(digest[:]), "step": step, "commands": commands, "nativeStateEdited": false})} {
				if err := os.WriteFile(campaignEvidencePath("water-beast-first-dead-ally-"+kind+".json", 1), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, member := range v.Battle.Players {
			accepted = accepted || member.State == 2 && member.Action == 3 && reviveSelection[member.Role] && member.Target >= 0 && member.Target < len(v.Party) && v.Party[member.Target].HP == 0
		}
		b := v.Battle
		if !targetSaved && b.State == 3 && b.Action == 3 && b.Current >= 0 && b.Current < len(v.Party) && reviveSelection[v.Party[b.Current].Role] && b.Target >= 0 && b.Target < len(v.Party) && v.Party[b.Target].HP == 0 {
			r.save("revival-dead-target-selected")
			targetSaved = true
			v = r.visible()
		}
		if anyDead && accepted && !saved {
			r.save("revival-accepted")
			saved = true
			v = r.visible()
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			victory, reward = true, cash
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
			if action.Keys == 2 && v.Battle.State == 1 && v.Battle.Menu == 1 {
				selected := originalVisibleText(v, true)
				reviveSelection[v.Party[v.Battle.Current].Role] = selected == "赎魂" || selected == "赎魂咒" || selected == "还魂咒"
			}
			if action.Keys != 0 || step%50 == 0 {
				trace = append(trace, map[string]any{"step": step, "action": action, "battle": v.Battle, "party": v.Party, "selected": originalVisibleText(v, true), "visible": originalVisibleText(v, false)})
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
			commands++
		}
		if step%500 == 0 {
			t.Log("genuine water beast replay", step, commands, v.Party, v.Battle)
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("genuine water beast policy did not finish", commands)
}
