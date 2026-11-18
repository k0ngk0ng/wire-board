package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

type trialRewardLeg struct {
	Kind                string `json:"kind"`
	Scene               int    `json:"scene"`
	Event               int    `json:"event"`
	Position            [2]int `json:"position"`
	Mode                int    `json:"mode"`
	Script              int    `json:"script"`
	Search              bool   `json:"search"`
	RawDestinationScene int    `json:"rawDestinationScene"`
	RawDestination      [2]int `json:"rawDestination"`
	DestinationScene    int    `json:"destinationScene"`
	Destination         [2]int `json:"destination"`
	ChainedEvents       []int  `json:"chainedEvents"`
	TeleportScript      int    `json:"teleportScript"`
}

type trialRewardStep struct {
	Event         int              `json:"event"`
	Scene         int              `json:"scene"`
	Position      [2]int           `json:"position"`
	PortalsBefore []trialRewardLeg `json:"portalsBefore"`
}

type trialRewardRouteCase struct {
	Scene            int               `json:"scene"`
	SourceCheckpoint string            `json:"sourceCheckpoint"`
	SourcePosition   [3]int            `json:"sourcePosition"`
	Events           []int             `json:"events"`
	Steps            []trialRewardStep `json:"steps"`
	PortalSteps      int               `json:"portalSteps"`
	EarthReturns     int               `json:"earthReturns"`
}

func loadTrialRewardRoute(t *testing.T, scene int, source []byte, expected []int, tables *resources.Tables) (trialRewardRouteCase, string) {
	t.Helper()
	planPath := "../../../.local/pal/verification/trial-rewards-plan.json"
	if scene >= 223 && scene <= 227 {
		planPath = "../../../.local/pal/verification/trial-deep-rewards-plan.json"
	}
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Passed           bool                   `json:"passed"`
		DiagnosticOnly   bool                   `json:"diagnosticOnly"`
		SSSSHA256        string                 `json:"sssSHA256"`
		SourceCheckpoint string                 `json:"sourceCheckpoint"`
		SourceSHA256     string                 `json:"sourceSHA256"`
		Cases            []trialRewardRouteCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(source)
	if !plan.Passed || !plan.DiagnosticOnly || plan.SSSSHA256 != "ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756" || plan.SourceCheckpoint != "sacred-medicine-trial-entry-state.json" || plan.SourceSHA256 != hex.EncodeToString(sourceHash[:]) {
		t.Fatal("trial reward planning source changed")
	}
	var selected trialRewardRouteCase
	for _, candidate := range plan.Cases {
		if candidate.Scene == scene {
			selected = candidate
		}
	}
	wanted, actual := append([]int{}, expected...), append([]int{}, selected.Events...)
	sort.Ints(wanted)
	sort.Ints(actual)
	if selected.SourceCheckpoint != plan.SourceCheckpoint || len(selected.Steps) != len(expected) || !bytes.Equal(raw(wanted), raw(actual)) {
		t.Fatal("trial reward plan omitted or added original event points", scene)
	}
	portals, earthReturns := 0, 0
	for index, step := range selected.Steps {
		original := tables.Events[step.Event-1]
		if step.Scene != scene || step.Event != selected.Events[index] || step.Position != [2]int{int(original.X), int(original.Y)} {
			t.Fatal("trial reward plan point changed", step)
		}
		for _, leg := range step.PortalsBefore {
			if leg.Kind == "earth-return" {
				earthReturns++
				if leg.Scene < 217 || leg.Scene > 227 || int(tables.Scenes[leg.Scene-1].OnTeleport) != leg.TeleportScript || leg.TeleportScript != 30770 || leg.DestinationScene != 216 || leg.Destination != [2]int{832, 736} {
					t.Fatal("trial reward earth return changed", leg)
				}
				continue
			}
			portals++
			if leg.Kind != "portal" || leg.Scene < 215 || leg.Scene > 227 || leg.Event < 1 || leg.Event > len(tables.Events) {
				t.Fatal("trial reward route contains an unsupported leg", leg)
			}
			entry := tables.Events[leg.Event-1]
			if leg.Event <= int(tables.Scenes[leg.Scene-1].FirstEvent) || leg.Event > int(tables.Scenes[leg.Scene].FirstEvent) || entry.State <= 0 || entry.Sprite != 0 || leg.Script != int(entry.TriggerScript) || leg.Mode != int(entry.TriggerMode) || leg.Search != (entry.TriggerMode < 4) || leg.Position != [2]int{int(entry.X), int(entry.Y)} {
				t.Fatal("trial reward original portal changed", leg)
			}
			jump, placement := tables.Scripts[leg.Script], tables.Scripts[leg.Script+1]
			position := [2]int{int(placement.Operands[0])*32 + int(placement.Operands[2])*16, int(placement.Operands[1])*16 + int(placement.Operands[2])*8}
			if jump.Opcode != 0x59 || int(jump.Operands[0]) != leg.RawDestinationScene || placement.Opcode != 0x46 || position != leg.RawDestination {
				t.Fatal("trial reward raw landing changed", leg)
			}
			nextScene := leg.RawDestinationScene
			for _, following := range leg.ChainedEvents {
				entry := tables.Events[following-1]
				if following <= int(tables.Scenes[nextScene-1].FirstEvent) || following > int(tables.Scenes[nextScene].FirstEvent) || entry.Sprite != 0 || entry.TriggerMode < 4 || fieldTestDistance(position[0], position[1], int(entry.X), int(entry.Y)) >= (int(entry.TriggerMode)-4)*32+16 {
					t.Fatal("trial reward automatic chain changed", leg, following)
				}
				jump, placement = tables.Scripts[entry.TriggerScript], tables.Scripts[entry.TriggerScript+1]
				if jump.Opcode != 0x59 || placement.Opcode != 0x46 {
					t.Fatal("trial reward chained placement changed", following)
				}
				nextScene = int(jump.Operands[0])
				position = [2]int{int(placement.Operands[0])*32 + int(placement.Operands[2])*16, int(placement.Operands[1])*16 + int(placement.Operands[2])*8}
			}
			if nextScene != leg.DestinationScene || position != leg.Destination {
				t.Fatal("trial reward final landing changed", leg)
			}
		}
	}
	if portals != selected.PortalSteps || earthReturns != selected.EarthReturns {
		t.Fatal("trial reward route step count changed")
	}
	hash := sha256.Sum256(data)
	return selected, hex.EncodeToString(hash[:])
}

func reserveTrialRewardObjects(r *caiyiForestDriver, target int) {
	r.reserve(target)
	for _, scene := range []int{167, 215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227, 280, 286, 287, 288, 289, 290, 291, 292, 293, 294} {
		for index := r.tables.Scenes[scene-1].FirstEvent; index < r.tables.Scenes[scene].FirstEvent; index++ {
			entry := r.tables.Events[index]
			if entry.State == 2 && (entry.Sprite == 10 || entry.Sprite == 389) {
				r.w.avoidTouches[scene] = append(r.w.avoidTouches[scene], [3]int{int(entry.X), int(entry.Y), 16})
			}
		}
	}
}

func followTrialRewardLeg(r *caiyiForestDriver, leg trialRewardLeg, number int) map[string]any {
	r.t.Helper()
	before := r.field()
	if before.Scene != leg.Scene {
		r.t.Fatal("trial reward route started outside its original scene", leg, before.Scene)
	}
	if leg.Kind == "earth-return" {
		amount := trialWormInventory(r, "土灵珠", "土灵珠")["土灵珠"]
		if amount != 1 {
			r.t.Fatal("trial reward original earth orb missing", amount)
		}
		r.save(fmt.Sprint("trial-route-", number, "-earth-selected"))
		r.keys(2, 4)
		r.progress()
		after := r.field()
		if after.Scene != leg.DestinationScene || fieldTestDistance(after.X, after.Y, leg.Destination[0], leg.Destination[1]) >= 48 {
			r.t.Fatal("original trial earth orb did not return normally", leg, after.Scene, after.X, after.Y)
		}
		reserveTrialRewardObjects(r, -1)
		r.save(fmt.Sprint("trial-route-", number, "-earth-returned"))
		return map[string]any{"step": number, "kind": leg.Kind, "sceneBefore": before.Scene, "sceneAfter": after.Scene, "positionAfter": [2]int{after.X, after.Y}, "originalLeg": leg}
	}
	reserveTrialRewardObjects(r, leg.Event)
	if !leg.Search {
		regions := r.w.avoidTouches[leg.Scene][:0]
		for _, region := range r.w.avoidTouches[leg.Scene] {
			if region != [3]int{leg.Position[0], leg.Position[1], (leg.Mode-4)*32 + 16} {
				regions = append(regions, region)
			}
		}
		r.w.avoidTouches[leg.Scene] = regions
	}
	visits := map[[3]int]int{}
	for step := 0; step < 6000; step++ {
		v := r.field()
		if v.Scene == leg.DestinationScene && (leg.Scene != leg.DestinationScene || fieldTestDistance(v.X, v.Y, leg.Destination[0], leg.Destination[1]) < 48) {
			if fieldTestDistance(v.X, v.Y, leg.Destination[0], leg.Destination[1]) >= 48 {
				r.t.Fatal("original trial reward portal landing changed", leg, v.X, v.Y)
			}
			reserveTrialRewardObjects(r, -1)
			r.save(fmt.Sprint("trial-route-", number, "-door-", leg.Event, "-arrived"))
			return map[string]any{"step": number, "kind": leg.Kind, "sceneBefore": before.Scene, "sceneAfter": v.Scene, "positionAfter": [2]int{v.X, v.Y}, "originalLeg": leg}
		}
		if v.Scene != leg.Scene {
			allowed := v.Scene == leg.RawDestinationScene
			for _, following := range leg.ChainedEvents {
				entry := r.tables.Events[following-1]
				allowed = allowed || v.Scene == int(r.tables.Scripts[entry.TriggerScript].Operands[0])
			}
			if !allowed {
				r.t.Fatal("trial reward portal entered an unexpected original scene", leg, v.Scene)
			}
			r.keys(0, 4)
			r.progress()
			continue
		}
		at := [3]int{v.Scene, v.X, v.Y}
		visits[at]++
		if visits[at] > 60 {
			r.t.Fatal("trial reward original portal approach blocked", leg, at)
		}
		if leg.Search && fieldTestDistance(v.X, v.Y, leg.Position[0], leg.Position[1]) <= 96 {
			r.search(leg.Event, leg.Position[0], leg.Position[1])
		} else {
			r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, leg.Position[0], leg.Position[1], 0)
		}
		r.progress()
	}
	r.t.Fatal("trial reward original portal approach timed out", leg)
	return nil
}
