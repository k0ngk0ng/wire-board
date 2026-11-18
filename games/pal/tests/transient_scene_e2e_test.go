package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalSageVisitRemainsScriptedAcrossSavedDirections(t *testing.T) {
	if os.Getenv("PAL_TRANSIENT_SCENE") != "1" {
		t.Skip("explicit original sage-visit scene continuation")
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != "transient-scene-v1" || campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("transient scene requires isolated authority profile")
	}
	name := "campaign-tower-bottom-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptedCampaignRoot(name, source, ""); err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "transient-scene-v1", allowedRoles: map[int]bool{0: true}, milestones: map[string]string{}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	if v := r.visible(); v.Scene != 174 || v.Phase != "script" {
		t.Fatal("transient scene parent is not the original post-tower entry", v.Scene, v.Phase)
	}
	observed := []map[string]any{}
	probes := []map[string]any{}
	entered, left, probed := false, false, false
	r.notice = func(v pal.View) {
		if v.Scene == 178 {
			if left || v.Phase != "script" && v.Phase != "dialogue" || len(v.Party) != 1 || v.Party[0].Role != 0 || v.Battle != nil {
				t.Fatal("sage visit exposed an unexpected exploration state", v.Scene, v.Phase, v.Party)
			}
			observed = append(observed, map[string]any{"phase": v.Phase, "position": [2]int{v.X, v.Y}, "prompt": v.Dialogue})
			if !entered {
				entered = true
				r.save("entered")
			}
		} else if entered && !left {
			if v.Scene != 175 {
				t.Fatal("sage visit did not return to the original room", v.Scene)
			}
			left = true
			r.save("returned-room")
		}
	}
	r.save("source")
	for step := 0; step < 16000; step++ {
		v := r.visible()
		if v.Scene == 178 && v.Dialogue != nil && !probed {
			probed = true
			r.save("first-dialogue")
			for direction := 0; direction < 4; direction++ {
				label := fmt.Sprint("direction-", direction)
				r.save(label + "-before")
				before := r.visible()
				original := append([]byte(nil), r.g.Data...)
				_, rejected := r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, Direction: direction}))
				if rejected == nil || !strings.Contains(rejected.Error(), "当前不能控制队伍") || !bytes.Equal(original, r.g.Data) {
					t.Fatal("scripted dialogue accepted field movement or changed rejected state", direction, rejected)
				}
				for frame := 0; frame < 8; frame++ {
					r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
					r.g, err = r.m.OnTimer(r.c, r.g)
					if err != nil {
						t.Fatal(err)
					}
					after := r.visible()
					if after.Scene != 178 || after.Phase != "dialogue" || after.Dialogue == nil || after.Dialogue.ID != before.Dialogue.ID || after.X != before.X || after.Y != before.Y || after.Cash != before.Cash {
						t.Fatal("direction input escaped the original waiting dialogue", direction, frame, after.Scene, after.Phase, after.X, after.Y)
					}
				}
				r.save(label + "-after")
				probes = append(probes, map[string]any{"direction": direction, "inputRejected": true, "timerCalls": 8, "position": [2]int{before.X, before.Y}, "prompt": before.Dialogue.ID, "fieldExposed": false})
			}
		}
		if v.Phase == "field" {
			if !entered || !left || !probed || v.Scene != 175 {
				t.Fatal("sage visit reached field before the required scripted return", v.Scene, entered, left, probed)
			}
			break
		}
		r.progress()
	}
	if v := r.visible(); v.Phase != "field" || v.Scene != 175 || len(observed) == 0 || len(probes) != 4 || r.proof.Battles != 0 {
		t.Fatal("sage visit continuation incomplete", v.Scene, v.Phase, len(observed), len(probes))
	}
	r.save("quest-field")
	amount := r.inventoryCounts("香蕉")["香蕉"]
	if amount != 0 {
		t.Fatal("scripted visit granted an unsearched banana", amount)
	}
	r.save("inventory-verified")
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "diagnosticOnly": true, "readyForProgress": false, "event": 2947, "scene": 178, "map": 201, "classification": "scripted-visit-confirmed-independent-attribution-pending", "sourceCheckpoint": name, "sourceSHA256": hex.EncodeToString(hash[:]), "nativeEdits": false, "normalPickupVerified": false, "fieldExposed": false, "observed": observed, "directionProbes": probes, "bananaFinalAmount": amount, "finalScene": r.visible().Scene, "proof": r.proof})
	for suffix, data := range map[string][]byte{"result": report, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath("transient-scene-v1-"+suffix+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
