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

func TestOriginalFalseKingPublicPolicyAndSavedVictory(t *testing.T) {
	if os.Getenv("PAL_FALSE_KING_BOT") != "1" {
		t.Skip("explicit actual final-palace false king production-policy replay")
	}
	name := "false-king-bot-source-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "a525daa227386b642128eb397c481de9a70786da51e3f7b4d91abe74cad5262c" {
		t.Fatal("false king source is not the actual earned palace battle")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "false-king-bot", proof: caiyiForestProof{Saved: map[string]bool{}}}
	initial := r.visible()
	if initial.Scene != 279 || initial.Battle == nil || len(initial.Party) != 2 || initial.Party[0].Role != 0 || initial.Party[1].Role != 4 || len(initial.Battle.Enemies) != 1 || initial.Battle.Enemies[0].Object != 462 {
		t.Fatal("false king replay is not the original Li/Anu palace battle", initial.Scene, initial.Party, initial.Battle)
	}
	trace := []any{}
	commands, reward := 0, -1
	saved := false
	defer func() {
		for kind, data := range map[string][]byte{"trace": raw(trace), "view": raw(r.visible()), "state": r.g.Data} {
			if err := os.WriteFile(campaignEvidencePath("false-king-bot-"+kind+".json", 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	for step := 0; step < 20000; step++ {
		v := r.visible()
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive {
			t.Fatal("actual false king production policy defeated", step, commands, v.Party)
		}
		if v.Battle == nil {
			if reward != 0 || v.Cash != initial.Cash || !saved {
				t.Fatal("false king normal zero-cash victory or saved action missing", reward, saved, v.Cash)
			}
			report := raw(map[string]any{"passed": true, "scope": "production policy from actual earned final-palace false king source, ordinary public inputs, original zero-cash victory and accepted-action fresh-runtime continuation; not complete finale acceptance", "sourceSHA256": hex.EncodeToString(hash[:]), "commands": commands, "timerObservations": step, "rewardCash": reward, "party": v.Party})
			if err := os.WriteFile(campaignEvidencePath("false-king-bot-result.json", 1), report, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if _, cash, ok := campaignBattleReward(v); ok {
			reward = cash
		}
		if !saved {
			for _, member := range v.Battle.Players {
				if member.State == 2 {
					r.save("accepted-action")
					saved = true
					break
				}
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
			if action.Keys != 0 || step%100 == 0 {
				trace = append(trace, map[string]any{"step": step, "action": action, "battle": v.Battle, "party": v.Party, "selected": originalVisibleText(v, true), "visible": originalVisibleText(v, false)})
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
			commands++
		}
		if step%500 == 0 {
			t.Log("actual false king public policy", step, commands, v.Party)
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("actual false king policy did not finish", commands)
}

func TestOriginalFalseKingVisiblePoisonAndSourceState(t *testing.T) {
	if os.Getenv("PAL_FALSE_KING_POISON") != "1" {
		t.Skip("explicit actual battle public poisoned-portrait observation")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, "false-king-bot-source-state.json"), prefix: "false-king-poison", proof: caiyiForestProof{Saved: map[string]bool{}}}
	for step := 0; step < 2000; step++ {
		v := r.visible()
		if v.Battle == nil {
			t.Fatal("original battle ended before a visible poison observation")
		}
		portraits := map[int][2]int{}
		if v.Raster != nil {
			for _, n := range v.Raster.Nodes {
				if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] >= 48 && n[7] <= 53 {
					portraits[n[7]-48] = [2]int{n[10], n[11]}
				}
			}
		}
		poisoned := []int{}
		for _, member := range v.Party {
			p := portraits[member.Role]
			if member.HP > 0 && p[0] == 3 && p[1] > 0 {
				poisoned = append(poisoned, member.Role)
			}
		}
		if len(poisoned) > 0 && v.Battle.Phase == 0 && v.Battle.State == 1 {
			r.save("first-visible-poison")
			for kind, data := range map[string][]byte{"view": raw(r.visible()), "result": raw(map[string]any{"passed": true, "scope": "actual publicly visible colored original battle portraits at an input boundary; no native poison-state reads or enemy-stat reads", "step": step, "roles": poisoned, "portraits": portraits})} {
				if err := os.WriteFile(campaignEvidencePath("false-king-poison-"+kind+".json", 1), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			return
		}
		a, err := r.m.BotAction(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) > 0 {
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("actual public poison was not observed")
}

func TestOriginalFalseKingPublicPurificationAndSavedAction(t *testing.T) {
	if os.Getenv("PAL_FALSE_KING_PURIFY") != "1" {
		t.Skip("explicit actual poisoned portrait, ordinary cleansing and fresh-runtime acceptance")
	}
	name := "false-king-poison-first-visible-poison-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "9fec7b85d92d836492d70ed591abb88781cc4c3ebff8fffbd0d382c2ca4f80b1" {
		t.Fatal("purification source is not the actual visible-poison saved boundary")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "false-king-purification", proof: caiyiForestProof{Saved: map[string]bool{}}}
	poison := func(v pal.View) map[int]bool {
		portraits := map[int][2]int{}
		if v.Raster != nil {
			for _, n := range v.Raster.Nodes {
				if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] >= 48 && n[7] <= 53 {
					portraits[n[7]-48] = [2]int{n[10], n[11]}
				}
			}
		}
		result := map[int]bool{}
		for _, member := range v.Party {
			p := portraits[member.Role]
			result[member.Role] = member.HP > 0 && p[0] == 3 && p[1] > 0
		}
		return result
	}
	initial := poison(r.visible())
	if !initial[0] || !initial[4] {
		t.Fatal("actual source no longer displays both poisoned portraits", initial)
	}
	selected := map[int]bool{}
	accepted := false
	normal := func(v pal.View, role int) bool {
		known, mode := false, -1
		if v.Raster != nil {
			for _, n := range v.Raster.Nodes {
				if n[0] == 1 && n[5] == 2 && n[6] == 0 && n[7] == 48+role {
					known, mode = true, n[10]
				}
			}
		}
		return known && (mode == 0 || mode == 1)
	}
	for step := 0; step < 6000; step++ {
		v := r.visible()
		if v.Battle == nil {
			t.Fatal("actual poison battle ended before ordinary cleansing")
		}
		if accepted && v.Battle.Phase == 0 && v.Battle.State == 1 {
			for _, member := range v.Party {
				if member.HP > 0 && initial[member.Role] && normal(v, member.Role) && !poison(v)[member.Role] {
					r.save("poison-cleared")
					result := raw(map[string]any{"passed": true, "scope": "actual visible-poison original source, ordinary public cleansing selection, accepted-action fresh-runtime save and actual living portrait returning to normal; not complete boss victory", "sourceSHA256": hex.EncodeToString(hash[:]), "step": step, "selected": selected, "acceptedSaved": accepted, "clearedRole": member.Role, "party": v.Party})
					if err := os.WriteFile(campaignEvidencePath("false-king-purification-result.json", 1), result, 0600); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
		}
		if !accepted {
			for _, player := range v.Battle.Players {
				if player.State == 2 && player.Action == 3 && selected[player.Role] {
					r.save("purification-accepted")
					accepted = true
					break
				}
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
			if action.Keys == 2 && v.Battle.State == 1 && v.Battle.Menu == 1 && originalVisibleText(v, true) == "灵血咒" {
				selected[v.Party[v.Battle.Current].Role] = true
			}
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				t.Fatal(err)
			}
		}
		r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
		r.g, err = r.m.OnTimer(r.c, r.g)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("actual cleansing selection, accepted save or normal portrait was not observed", selected, accepted)
}

func TestOriginalFalseKingPreparationInventoryObservation(t *testing.T) {
	if os.Getenv("PAL_FALSE_KING_PREPARATION") != "1" {
		t.Skip("explicit original pre-palace inventory observation")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, "finale-step-9-state.json"), prefix: "false-king-preparation", proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}, allowedRoles: map[int]bool{0: true, 1: true, 4: true}, milestones: map[string]string{}}
	v := r.field()
	if v.Scene != 279 || len(v.Party) != 3 {
		t.Fatal("preparation source is not the actual first palace arrival")
	}
	items := trialWormInventory(r, "", "五毒珠", "毒龙胆", "血海棠", "九阴散", "蟠果", "灵蛊", "灵葫仙丹", "七星剑", "木剑", "铁剑", "天蛇杖", "巫月神刀", "鬼头杖", "酒葫芦", "金刚符", "隐蛊", "舍利子")
	result := raw(map[string]any{"passed": true, "scope": "ordinary visible original inventory at actual pre-palace arrival; no grants or native inventory reads", "items": items, "party": r.visible().Party})
	if err := os.WriteFile(campaignEvidencePath("false-king-preparation-inventory.json", 1), result, 0600); err != nil {
		t.Fatal(err)
	}
}
