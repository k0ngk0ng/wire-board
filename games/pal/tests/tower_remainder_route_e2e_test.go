package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func loadTowerRemainderRewardRoute(t *testing.T, source []byte, expected []int, tables *resources.Tables) (trialRewardRouteCase, string) {
	t.Helper()
	data, err := os.ReadFile("../../../.local/pal/verification/tower-remainder-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Passed, DiagnosticOnly, OriginalDataEdits bool
		SSSSHA256                                 string `json:"sssSHA256"`
		Case                                      struct {
			trialRewardRouteCase
			SourceSHA256    string `json:"sourceSHA256"`
			UnplannedEvents []int  `json:"unplannedEvents"`
		}
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	c := plan.Case
	if !plan.Passed || !plan.DiagnosticOnly || plan.OriginalDataEdits || plan.SSSSHA256 != "ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756" || c.SourceSHA256 != hex.EncodeToString(hash[:]) || c.SourceCheckpoint != "tower-upper-step-3-state.json" || c.Scene != 167 || c.SourcePosition != [3]int{167, 1024, 720} || c.PortalSteps != 0 || c.EarthReturns != 0 || !bytes.Equal(raw(c.UnplannedEvents), raw([]int{2763})) {
		t.Fatal("tower remainder original route or source changed")
	}
	wanted, actual := append([]int{}, expected...), append([]int{}, c.Events...)
	sort.Ints(wanted)
	sort.Ints(actual)
	if len(c.Steps) != 11 || !bytes.Equal(raw(wanted), raw(actual)) {
		t.Fatal("tower remainder chest set changed")
	}
	for index, step := range c.Steps {
		e := tables.Events[step.Event-1]
		if step.Event != c.Events[index] || step.Scene != 167 || len(step.PortalsBefore) != 0 || step.Position != [2]int{int(e.X), int(e.Y)} || e.Sprite != 10 || e.State != 2 || e.TriggerMode != 1 {
			t.Fatal("tower remainder original chest record changed", step)
		}
	}
	planHash := sha256.Sum256(data)
	return c.trialRewardRouteCase, hex.EncodeToString(planHash[:])
}
