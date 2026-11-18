package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestOriginalMarketHarborFixtureFromNewGame(t *testing.T) {
	if os.Getenv("PAL_MARKET_HARBOR_FIXTURE") != "1" {
		t.Skip("explicit ordinary new-game harbor fixture")
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != "misc-rewards-v1" || campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("harbor fixture requires isolated authority profile")
	}
	m, c, g := originalVillage(t, 1)
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: &riverMedicineDriver{t: t, m: m, c: c, g: g, p: riverMedicineProof{Saved: map[string]bool{}}}, tables: tables, prefix: "market-harbor-entry", milestones: map[string]string{}, allowedRoles: map[int]bool{0: true}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	for step := 0; step < 1400; step++ {
		v := r.field()
		if v.Scene == 6 {
			break
		}
		if v.Scene != 5 {
			t.Fatal("original harbor path left village", v.Scene)
		}
		r.g = r.w.walk(t, r.m, &r.c, r.g, v, 1824, 1328, 0)
		r.progress()
	}
	v := r.field()
	if v.Scene != 6 || len(v.Party) != 1 || v.Party[0].Role != 0 || v.Cash != 550 {
		t.Fatal("ordinary harbor fixture changed original start", v.Scene, v.Cash, v.Party)
	}
	r.save("arrived")
	parents := []map[string]string{}
	for _, name := range []string{"campaign-inn-guests.json", "campaign-kitchen-tray.json", "campaign-guest-wine.json", "campaign-drunkard-wine.json", "campaign-market-village.json"} {
		path := campaignEvidencePath(name, 1)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var parent struct{ Passed bool }
		if json.Unmarshal(data, &parent) != nil || !parent.Passed {
			t.Fatal("ordinary harbor precursor not accepted", name)
		}
		hash := sha256.Sum256(data)
		parents = append(parents, map[string]string{"name": name, "sha256": hex.EncodeToString(hash[:])})
	}
	hash := sha256.Sum256(r.g.Data)
	result := raw(map[string]any{"passed": true, "resumedTestRoute": false, "generatedFromNewGame": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "nativeEdits": false, "scope": "ordinary original new game, inn guests and wine, aunt market errand and actual harbor walking, fresh-runtime save and stale-input rejection; isolated fixture, not a new full-campaign acceptance", "parents": parents, "stateSHA256": hex.EncodeToString(hash[:]), "scene": v.Scene, "position": [2]int{v.X, v.Y}, "cash": v.Cash, "proof": r.proof})
	for name, data := range map[string][]byte{"market-harbor-entry-state.json": r.g.Data, "market-harbor-entry.json": result} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
