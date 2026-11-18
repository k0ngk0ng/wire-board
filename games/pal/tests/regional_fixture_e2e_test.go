package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalHiddenIslandEntryFromNewGameAndFreshSavedRuntime(t *testing.T) {
	if os.Getenv("PAL_GENERATE_HIDDEN_ISLAND_ENTRY") != "1" {
		t.Skip("explicit fresh original island reward fixture")
	}
	if campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("fresh island fixture requires the current authoritative development runtime")
	}
	t.Setenv("PAL_EVIDENCE_SCOPE", "hidden-island-entry")
	m, c, g := originalIslandLanding(t, 1)
	before := view(t, m, g, 0)
	if before.Phase != "field" || before.Scene != 15 || before.Map != 3 || len(before.Party) != 1 || before.Party[0].Role != 0 {
		t.Fatal("fresh original island landing did not reach the actual shore", before.Scene, before.Phase, before.Party)
	}
	var err error
	g, err = m.SaveControl(c, g, true)
	if err != nil {
		t.Fatal(err)
	}
	c.Now = c.Now.Add(24 * time.Hour)
	m, _ = nativeModule(t)
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(v pal.View) pal.View {
		v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
		return v
	}
	if !bytes.Equal(raw(normalize(before)), raw(normalize(view(t, m, g, 0)))) {
		t.Fatal("fresh island fixture lost original continuation")
	}
	if _, err = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		t.Fatal("fresh island fixture accepted a stale search")
	}
	digest := func(data []byte) string {
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(campaignEvidencePath("profile.json", 1)), "campaign-*-1.json"))
	if err != nil || len(paths) < 4 {
		t.Fatal("fresh island fixture has no complete original precursor reports", err, len(paths))
	}
	parents := []map[string]any{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		var report struct {
			Passed bool `json:"passed"`
		}
		if err != nil || json.Unmarshal(data, &report) != nil || !report.Passed {
			t.Fatal("fresh original precursor report failed", path, err)
		}
		parents = append(parents, map[string]any{"path": path, "sha256": digest(data)})
	}
	result := raw(map[string]any{"passed": true, "scope": "fresh original new-game opening, inn guests, wine, village, original medicine request and island landing; full ordinary precursor reports isolated from accepted campaign evidence, fresh-runtime saved shore and stale search rejection; no native state edits", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "authorityFixture": true, "generatedFromNewGame": true, "scene": before.Scene, "map": before.Map, "party": before.Party, "cash": before.Cash, "stateSHA256": digest(g.Data), "parents": parents})
	t.Setenv("PAL_EVIDENCE_SCOPE", "")
	for name, data := range map[string][]byte{"hidden-island-entry-state.json": g.Data, "hidden-island-entry.json": result} {
		path := campaignEvidencePath(name, 1)
		old, err := os.ReadFile(path)
		if err == nil {
			if !bytes.Equal(old, data) {
				t.Fatal("accepted island fixture cannot be overwritten", path)
			}
			continue
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
