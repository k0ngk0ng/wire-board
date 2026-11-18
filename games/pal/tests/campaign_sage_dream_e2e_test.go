package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalSageDreamTreatmentMemoriesAndSavedQuest(t *testing.T) {
	if os.Getenv("PAL_SAGE_DREAM") != "1" {
		t.Skip("explicit actual sage dream and treatment continuation")
	}
	runSageDream(t, "sage-dream")
}

func runSageDream(t *testing.T, prefix string) *caiyiForestDriver {
	t.Helper()
	name := "campaign-tower-bottom-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.ReadFile(campaignEvidencePath("campaign-tower-bottom.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	var origin struct {
		Passed, ResumedTestRoute bool
		Scene, Map               int
	}
	if err := json.Unmarshal(parent, &origin); err != nil || !origin.Passed || origin.Scene != 174 || origin.Map != 20 {
		t.Fatal("sage continuation has no verified actual tower-ending parent", err, origin)
	}
	milestones := map[string]string{
		"nightmare": "那．．我自己过去了", "awake": "您可终于醒过来了",
		"pregnancy": "胎儿．．我～我的", "sacred-medicine": "二兽乃我苗族圣兽",
		"yue-fate": "天灵碎裂，回天乏术", "saint-visit": "圣姑，他们的情况如何",
		"saint-leaves": "别让他们知道我来过这里",
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: prefix, milestones: milestones,
		allowedRoles: map[int]bool{0: true},
		proof:        caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	initial := r.visible()
	if initial.Scene != 174 || initial.Map != 20 || len(initial.Party) != 1 || initial.Party[0].Role != 0 {
		t.Fatal("sage dream did not start at the original next-story entry", initial.Scene, initial.Party)
	}
	memories := []int{}
	r.notice = func(v pal.View) {
		if v.Scene >= 194 && v.Scene <= 200 && !r.proof.Saved[fmt.Sprint("memory-", v.Scene)] {
			memories = append(memories, v.Scene)
			if len(v.Party) != 1 || v.Party[0].Role != 0 {
				t.Fatal("original Yue memory changed temporary party", v.Scene, v.Party)
			}
			r.save(fmt.Sprint("memory-", v.Scene))
		}
		if v.Battle != nil || v.Cash != initial.Cash {
			t.Fatal("automatic dream/treatment introduced a battle or changed cash", v.Scene, v.Cash)
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	r.save("before")
	v := r.field()
	if v.Scene != 175 || v.Map != 202 || len(v.Party) != 1 || v.Party[0].Role != 0 || r.proof.Battles != 0 {
		t.Fatal("original sage treatment did not return the sole Li party", v.Scene, v.Map, v.Party, r.proof)
	}
	for name := range milestones {
		if !r.proof.Seen[name] {
			t.Fatal("original dream/treatment milestone missing", name, r.proof.Seen)
		}
	}
	for _, scene := range []int{174, 175, 178, 194, 195, 196, 197, 198, 199, 200} {
		if r.proof.Scenes[scene] == 0 {
			t.Fatal("original dream/treatment or memory scene missing", scene)
		}
	}
	if !bytes.Equal(raw(memories), raw([]int{194, 195, 196, 197, 198, 199, 200})) {
		t.Fatal("original seven-memory order changed", memories)
	}
	r.save("quest-field")
	items := r.inventoryCounts("天书", "金凤凰蛋壳", "风灵珠")
	for name, amount := range items {
		if amount != 0 {
			t.Fatal("original book consumption or not-yet-earned sacred items changed", name, amount)
		}
	}
	r.save("inventory-verified")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "actual post-tower nightmare, sage treatment/pregnancy and sacred-medicine quest, Yue fate, seven original memories and sword-saint visit; ordinary reading/menu inputs and fresh-runtime saves; no sacred-tree/phoenix/stage18 completion claim", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceSHA256": hex.EncodeToString(hash[:]), "parentStage17ResumedTestRoute": origin.ResumedTestRoute, "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "inventory": items, "memoryOrder": memories, "proof": r.proof})
	reportName, stateName := "sage-dream-result.json", "sage-dream-state.json"
	if prefix != "sage-dream" {
		reportName, stateName = prefix+"-sage-entry.json", prefix+"-sage-entry-state.json"
	}
	for name, data := range map[string][]byte{reportName: report, stateName: r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
