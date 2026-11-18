package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestOriginalTrialOverlappingPortalFirstTriggerAndSavedLanding(t *testing.T) {
	if os.Getenv("PAL_TRIAL_PRIORITY") != "1" {
		t.Skip("explicit current-core overlapping trial portal witness")
	}
	if campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("original trial priority requires the registered authoritative profile")
	}
	planBytes, err := os.ReadFile("../../../.local/pal/verification/trial-deep-priority-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Passed           bool             `json:"passed"`
		DiagnosticOnly   bool             `json:"diagnosticOnly"`
		SourceCheckpoint string           `json:"sourceCheckpoint"`
		SourceSHA256     string           `json:"sourceSHA256"`
		Legs             []trialRewardLeg `json:"legs"`
	}
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.Passed || !plan.DiagnosticOnly || plan.SourceCheckpoint != "regional-hidden-v18-226-state.json" || len(plan.Legs) == 0 || plan.Legs[len(plan.Legs)-1].Event != 3949 {
		t.Fatal("original trial priority plan changed")
	}
	source, err := os.ReadFile(campaignEvidencePath(plan.SourceCheckpoint, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if plan.SourceSHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("original trial priority source changed")
	}
	if err := acceptedCampaignRoot(plan.SourceCheckpoint, source, ""); err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := tables.Events[3948], tables.Events[3953]
	if a.X != b.X || a.Y != b.Y || a.State != 1 || b.State != 1 || a.TriggerMode != 4 || b.TriggerMode != 4 || a.VanishTime != 0 || b.VanishTime != 0 || a.AutoScript != 0 || b.AutoScript != 0 || a.TriggerScript != 30942 || b.TriggerScript != 31042 {
		t.Fatal("original overlapping portal pair changed")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, plan.SourceCheckpoint), tables: tables, prefix: "trial-priority-v1", allowedRoles: map[int]bool{0: true, 4: true}, reserveScenes: []int{215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227}, physicalAttacks: true, milestones: map[string]string{}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	reserveTrialRewardObjects(r, -1)
	r.save("source")
	trace := []map[string]any{}
	for index, leg := range plan.Legs {
		if leg.Event == 3949 {
			r.save("before-first-overlap")
		}
		trace = append(trace, followTrialRewardLeg(r, leg, index+1))
	}
	final := r.field()
	if final.Scene != 227 || final.X != 1488 || final.Y != 1272 || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 {
		t.Fatal("current original core did not choose the first overlapping portal", final.Scene, final.X, final.Y)
	}
	r.save("first-overlap-arrived")
	core := "2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa"
	result := raw(map[string]any{"passed": true, "nativeBuildID": core, "nativeEdits": false, "scope": "ordinary inputs from completed genuine upper reward branch, current original first overlapping portal actual landing and fresh-runtime saves; no direct script execution or original state edits", "sourceCheckpoint": plan.SourceCheckpoint, "sourceSHA256": plan.SourceSHA256, "firstEvent": 3949, "shadowedEvent": 3954, "actualDestinationScene": final.Scene, "actualDestination": [2]int{final.X, final.Y}, "route": trace, "proof": r.proof})
	for suffix, data := range map[string][]byte{"result": result, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath("trial-priority-v1-"+suffix+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile("../../../.local/pal/verification/trial-deep-priority-result.json", result, 0600); err != nil {
		t.Fatal(err)
	}
}
