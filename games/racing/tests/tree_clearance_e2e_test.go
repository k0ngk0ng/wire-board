package racing_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/racing/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func propEdgesByGeometry(t *testing.T) map[string]map[int]bool {
	t.Helper()
	var index []struct {
		Geometry string `json:"geometry"`
		Catalog  string `json:"catalog"`
		Current  bool   `json:"current"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "server", "data", "prop-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	result := map[string]map[int]bool{}
	for _, entry := range index {
		if !entry.Current {
			continue
		}
		data, err := os.ReadFile(filepath.Join("..", "server", "data", "props", entry.Catalog+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var catalog struct {
			Props []struct {
				RuleEdges []int `json:"ruleEdges"`
			} `json:"props"`
		}
		if err := json.Unmarshal(data, &catalog); err != nil {
			t.Fatal(err)
		}
		edges := map[int]bool{}
		for _, prop := range catalog.Props {
			for _, edge := range prop.RuleEdges {
				edges[edge] = true
			}
		}
		result[entry.Geometry] = edges
	}
	return result
}

func pointSegmentDist(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	l2 := dx*dx + dy*dy
	f := 0.
	if l2 > 1e-12 {
		f = math.Max(0, math.Min(1, ((px-ax)*dx+(py-ay)*dy)/l2))
	}
	return math.Hypot(px-(ax+dx*f), py-(ay+dy*f))
}

func TestGermanyAndParkStaticTreesDoNotIntrudePavedRoad(t *testing.T) {
	tracksByID := map[string]game.Track{}
	for _, track := range game.Tracks() {
		if track.Version == 3 {
			tracksByID[track.ID] = track
		}
	}
	propEdges := propEdgesByGeometry(t)
	for _, id := range []string{"germany", "park", "france", "coastal", "uk"} {
		track, ok := tracksByID[id]
		if !ok {
			t.Fatalf("missing classic track %s", id)
		}
		destructible := propEdges[track.Geometry]
		intrusions := 0
		for edgeIdx, b := range track.Barriers {
			if b.Kind != "object" || destructible[edgeIdx] {
				continue
			}
			midX, midY := (b.A.X+b.B.X)/2, (b.A.Y+b.B.Y)/2
			height := (b.Bottom + b.Top) / 2
			road := game.ClassicRoadAt(track, midX, midY, height, -1)
			if math.Abs(road.Height-b.Bottom) > 35 && math.Abs(road.Height-height) > 35 {
				continue
			}
			edge := road.Right
			if road.Side < 0 {
				edge = road.Left
			}
			distCenter := math.Abs(road.Side)
			if distCenter < edge-1.5 {
				intrusions++
			}
		}
		if id == "germany" || id == "park" {
			if intrusions > 0 {
				t.Fatalf("%s still has %d object barrier edges intruding onto paved road", id, intrusions)
			}
		}
	}
}

func TestGermanyAndParkStaticBarrierCountDropsBillboards(t *testing.T) {
	tracksByID := map[string]game.Track{}
	for _, track := range game.Tracks() {
		if track.Version == 3 {
			tracksByID[track.ID] = track
		}
	}
	for id, maxObjectEdges := range map[string]int{"germany": 800, "park": 2200} {
		track := tracksByID[id]
		objectEdges := 0
		for _, b := range track.Barriers {
			if b.Kind == "object" {
				objectEdges++
			}
		}
		if objectEdges == 0 || objectEdges > maxObjectEdges {
			t.Fatalf("%s object barrier edges out of expected billboard-free range: %d (max %d)", id, objectEdges, maxObjectEdges)
		}
	}
}

func TestLegacySavedFixturesStillRestoreAfterGeometryUpgrade(t *testing.T) {
	mod := game.Module()
	ctx := gameapi.Context{Players: 2, Now: time.Unix(1794500000, 0)}
	entries, err := os.ReadDir(filepath.Join("fixtures", "stale-v3-laps"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("fixtures", "stale-v3-laps", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m := gameapi.Match{Schema: 1, Data: data}
		if _, err := mod.Restore(ctx, m); err != nil {
			t.Fatalf("legacy fixture %s failed to restore: %v", entry.Name(), err)
		}
	}
}

func TestPropBreakEventRetainsImpactPosition(t *testing.T) {
	mod, ctx, m := begin(t, 2, 1, "france")
	target := game.Point{X: 3204.8711373, Y: 1313.243409}
	for step := 0; step < 900 && !classicPropBroken(classicPropsView(t, mod, m), 1); step++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		m = classicPropDrive(t, mod, &ctx, m, 0, target, 13)
		ctx.Actor = -1
		var err error
		m, err = mod.OnTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := classicView(t, mod, m, 0)
	for _, rawEvent := range v["events"].([]any) {
		e := rawEvent.(map[string]any)
		if e["type"] == "prop" && e["prop"] == float64(1) {
			x, _ := e["x"].(float64)
			y, _ := e["y"].(float64)
			if pointSegmentDist(x, y, target.X, target.Y, target.X, target.Y) > 120 {
				t.Fatalf("prop break event lost impact coordinates: x=%v y=%v target=%+v", x, y, target)
			}
			return
		}
	}
	t.Fatal("prop break event not emitted")
}
