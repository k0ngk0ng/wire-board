package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalTrialGaijiaoPublicBattleAndSavedContinuation(t *testing.T) {
	if os.Getenv("PAL_TRIAL_GAIJIAO_BOT") != "1" {
		t.Skip("explicit genuine trial Gaijiao battle replay")
	}
	name := "trial-gaijiao-bot-source-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "fec04132cffe23ffa1daf233fc92b535400469a3995812175be3fdec57429ff9" {
		t.Fatal("genuine trial Gaijiao battle source changed")
	}
	physical := os.Getenv("PAL_TRIAL_GAIJIAO_PHYSICAL") == "1"
	prefix := "trial-gaijiao-bot"
	if physical {
		prefix = "trial-gaijiao-manual-attack"
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: prefix, proof: caiyiForestProof{Saved: map[string]bool{}}}
	initial := r.visible()
	if initial.Scene != 216 || initial.Battle == nil || len(initial.Party) != 1 || initial.Party[0].Role != 0 {
		t.Fatal("trial Gaijiao source is not the earned solo Li encounter")
	}
	trace := []any{}
	commands, reward := 0, -1
	victory, acceptedSaved, defeated, rescue := false, false, false, false
	defer func() {
		for kind, data := range map[string][]byte{"trace": raw(trace), "view": raw(r.visible()), "state": r.g.Data} {
			if err := os.WriteFile(campaignEvidencePath(prefix+"-"+kind+".json", 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 15000; step++ {
		v := r.visible()
		if v.Battle == nil && v.Phase == "field" {
			paidCorrectly := victory && reward >= 0 && v.Cash == initial.Cash+uint32(reward) || !victory && defeated && v.Cash == initial.Cash
			if !paidCorrectly || !acceptedSaved || !rescue || len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 4 {
				t.Fatal("trial Gaijiao did not preserve original outcome, rescue and party", victory, reward, acceptedSaved, defeated, rescue, v.Party)
			}
			report := raw(map[string]any{"passed": true, "scope": "earned solo Li versus original Gaijiao and scorpion; ordinary public inputs, original permitted defeat or normal victory, Anu intervention and actual reunion, accepted-action and defeated-boundary fresh-runtime saves; no native or stat changes", "manualPhysicalReference": physical, "sourceSHA256": hex.EncodeToString(hash[:]), "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "timerObservations": step, "commands": commands, "cashBefore": initial.Cash, "cashAfter": v.Cash, "rewardCash": reward, "normalVictory": victory, "originalPermittedDefeat": defeated, "party": v.Party})
			if err := os.WriteFile(campaignEvidencePath(prefix+"-result.json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if v.Battle == nil {
			if v.Dialogue != nil {
				rescue = rescue || strings.Contains(v.Dialogue.Text, "通通给我住手")
				trace = append(trace, map[string]any{"step": step, "dialogue": v.Dialogue, "party": v.Party})
				r.g = apply(t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
			}
			r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
			r.g, err = r.m.OnTimer(r.c, r.g)
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if v.Party[0].HP == 0 && !defeated {
			defeated = true
			r.save("original-defeat")
			v = r.visible()
		}
		if !acceptedSaved {
			for _, member := range v.Battle.Players {
				if member.State == 2 {
					r.save("accepted-action")
					acceptedSaved = true
					v = r.visible()
				}
			}
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			victory, reward = true, cash
		}
		var actionBytes []byte
		if v.Dialogue != nil {
			actionBytes = raw(pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		} else if physical {
			if action, ok := caiyiForestPhysicalAttack(v); ok {
				actionBytes = raw(action)
			}
		} else {
			actionBytes, err = r.m.BotAction(r.c, r.g)
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(actionBytes) > 0 {
			var action pal.Action
			if err := json.Unmarshal(actionBytes, &action); err != nil {
				t.Fatal(err)
			}
			if action.Keys != 0 || step%50 == 0 {
				trace = append(trace, map[string]any{"step": step, "action": action, "battle": v.Battle, "party": v.Party, "selected": originalVisibleText(v, true), "visible": originalVisibleText(v, false)})
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, actionBytes)
			if err != nil {
				t.Fatal(err)
			}
			commands++
		}
		if step%500 == 0 {
			t.Log("genuine trial Gaijiao replay", step, commands, physical, v.Party, v.Battle)
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("genuine trial Gaijiao strategy did not finish", commands)
}
