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

func loadFinalMazeRewardRoute(t *testing.T, scene int, source []byte, expected []int, tables *resources.Tables) (trialRewardRouteCase, string) {
	t.Helper()
	data, err := os.ReadFile("../../../.local/pal/verification/final-maze-rewards-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Passed, DiagnosticOnly, OriginalDataEdits bool
		SSSSHA256                                 string `json:"sssSHA256"`
		Cases                                     []struct {
			trialRewardRouteCase
			SourceSHA256    string `json:"sourceSHA256"`
			UnplannedEvents []int  `json:"unplannedEvents"`
		}
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.Passed || !plan.DiagnosticOnly || plan.OriginalDataEdits || plan.SSSSHA256 != "ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756" {
		t.Fatal("final maze plan identity changed")
	}
	sourceHash := sha256.Sum256(source)
	for _, candidate := range plan.Cases {
		if candidate.Scene != scene {
			continue
		}
		if candidate.SourceSHA256 != hex.EncodeToString(sourceHash[:]) {
			t.Fatal("final maze source changed", scene)
		}
		wanted, actual := append([]int{}, expected...), append([]int{}, candidate.Events...)
		sort.Ints(wanted)
		sort.Ints(actual)
		if !bytes.Equal(raw(wanted), raw(actual)) || len(candidate.Steps) != len(expected) || candidate.EarthReturns != 0 {
			t.Fatal("final maze candidate set changed", scene)
		}
		portals := 0
		for index, step := range candidate.Steps {
			entry := tables.Events[step.Event-1]
			if step.Event != candidate.Events[index] || step.Scene != scene || step.Position != [2]int{int(entry.X), int(entry.Y)} {
				t.Fatal("final maze original reward changed", step)
			}
			for _, leg := range step.PortalsBefore {
				if leg.Kind != "portal" || leg.Scene != 280 && (leg.Scene < 286 || leg.Scene > 294) || leg.Event <= int(tables.Scenes[leg.Scene-1].FirstEvent) || leg.Event > int(tables.Scenes[leg.Scene].FirstEvent) || len(leg.ChainedEvents) != 0 {
					t.Fatal("final maze portal scope changed", leg)
				}
				original := tables.Events[leg.Event-1]
				jump, placement := tables.Scripts[original.TriggerScript], tables.Scripts[original.TriggerScript+1]
				position := [2]int{int(placement.Operands[0])*32 + int(placement.Operands[2])*16, int(placement.Operands[1])*16 + int(placement.Operands[2])*8}
				if original.State <= 0 || original.Sprite != 0 || leg.Position != [2]int{int(original.X), int(original.Y)} || leg.Script != int(original.TriggerScript) || leg.Mode != int(original.TriggerMode) || leg.Search != (original.TriggerMode < 4) || jump.Opcode != 0x59 || int(jump.Operands[0]) != leg.DestinationScene || leg.RawDestinationScene != leg.DestinationScene || placement.Opcode != 0x46 || position != leg.Destination || leg.RawDestination != position {
					t.Fatal("final maze original portal or landing changed", leg)
				}
				portals++
			}
		}
		if portals != candidate.PortalSteps {
			t.Fatal("final maze portal count changed", scene)
		}
		hash := sha256.Sum256(data)
		return candidate.trialRewardRouteCase, hex.EncodeToString(hash[:])
	}
	t.Fatal("final maze plan scene absent", scene)
	return trialRewardRouteCase{}, ""
}
