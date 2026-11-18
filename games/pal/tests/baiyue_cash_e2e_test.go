package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func finaleOriginalCashAction(t *testing.T) func(pal.View) ([]byte, bool) {
	t.Helper()
	settled, opening := 0, 0
	return func(v pal.View) ([]byte, bool) {
		b := v.Battle
		if b == nil {
			return nil, false
		}
		boss := false
		for _, enemy := range b.Enemies {
			boss = boss || enemy.Object == 546
		}
		if !boss || b.Current < 0 || b.Current >= len(v.Party) || v.Party[b.Current].Role != 4 {
			return nil, false
		}
		if b.Phase != 0 || b.State == 0 || v.Seq != v.AppliedSeq {
			opening = 0
			return nil, true
		}
		if settled > 0 {
			settled--
			return nil, true
		}
		a := pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4}
		switch b.State {
		case 1:
			if b.Menu == 0 {
				if v.Cash < 5000 {
					t.Fatal("actual cash-skill test exhausted its earned public budget", v.Cash)
				}
				if opening >= 4 {
					a.Keys = 1024
					opening = 0
				} else if b.Selected != 1 {
					a.Direction, a.Keys = 1, 8
				} else {
					a.Keys = 2
					opening++
				}
			} else if b.Menu == 1 {
				opening = 0
				if originalVisibleText(v, true) == "乾坤一掷" {
					a.Keys = 2
				} else {
					a.Direction, a.Keys = 3, 32
				}
			} else {
				a.Keys = 1
			}
		case 2, 4:
			if b.Action != 3 {
				a.Keys = 1
			} else if b.State == 4 {
				a.Keys = 2
			} else {
				target := -1
				for i, enemy := range b.Enemies {
					if enemy.Object == 546 {
						target = i
					}
				}
				if b.Target == target {
					a.Keys = 2
				} else {
					a.Direction, a.Keys = 3, 32
				}
			}
		default:
			a.Keys = 1
		}
		settled = 2
		return raw(a), true
	}
}

func TestOriginalBaiyueCashSkillSavedCostAndVictory(t *testing.T) {
	if os.Getenv("PAL_BAIYUE_CASH") != "1" {
		t.Skip("explicit actual earned Baiyue battle and normal cash-skill player inputs")
	}
	name := "baiyue-cash-source-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != "17ef6b84d63d327110075452f1ed70bbd543fb0bb1d6689165283e35e55c5136" {
		t.Fatal("cash-skill source is not the actual earned final battle")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "baiyue-cash", proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}, allowedRoles: map[int]bool{0: true, 1: true, 4: true}, battlePartySize: 3, milestones: map[string]string{"victory": "你已经无路可逃了"}}
	r.manualBattle = finaleOriginalCashAction(t)
	initial := r.visible()
	lastCash, casts := initial.Cash, 0
	selectedSaved, costSaved := false, false
	r.notice = func(v pal.View) {
		if v.Cash < lastCash {
			if lastCash-v.Cash != 5000 {
				t.Fatal("original cash skill consumed an unexpected amount", lastCash, v.Cash)
			}
			r.proof.BattleSpent += 5000
			casts++
		}
		lastCash = v.Cash
		if b := v.Battle; b != nil && b.Current >= 0 && b.Current < len(v.Party) && v.Party[b.Current].Role == 4 && b.State == 1 && b.Menu == 1 && originalVisibleText(v, true) == "乾坤一掷" && !selectedSaved {
			if casts != 0 || v.Cash != initial.Cash {
				t.Fatal("cash spell selection consumed money before confirmation")
			}
			selectedSaved = true
			r.save("cash-spell-selected")
		}
		if casts > 0 && !costSaved {
			costSaved = true
			r.save("first-cash-cost-animation")
		}
	}
	for step := 0; step < 10000 && !r.proof.Seen["victory"]; step++ {
		v := r.visible()
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive {
			t.Fatal("actual final battle player plan was defeated", v.Party)
		}
		if step%500 == 0 {
			t.Log("actual Baiyue cash plan", step, casts, v.Cash, v.Party)
		}
		r.progress()
	}
	v := r.visible()
	if !r.proof.Seen["victory"] || r.proof.Wins != 1 || casts == 0 || !selectedSaved || !costSaved || r.proof.RewardCash < 12800 || int64(v.Cash) != int64(initial.Cash)-int64(casts*5000)+int64(r.proof.RewardCash) {
		t.Fatal("original cash-skill cost, saved action or final victory missing", casts, selectedSaved, costSaved, v.Cash, r.proof)
	}
	for kind, data := range map[string][]byte{"state": r.g.Data, "result": raw(map[string]any{"passed": true, "scope": "actual earned Baiyue encounter, player-scripted ordinary Anu all-field cash spell and existing public decisions for the other roles, unconfirmed original spell selection and spent-cost animation fresh-runtime saves, actual original reward and victory dialogue; not whole-stage or production final-battle bot acceptance", "sourceSHA256": hex.EncodeToString(hash[:]), "casts": casts, "spent": casts * 5000, "cashBefore": initial.Cash, "cashAfter": v.Cash, "rewardCash": r.proof.RewardCash, "proof": r.proof, "party": v.Party})} {
		if err := os.WriteFile(campaignEvidencePath("baiyue-cash-"+kind+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
