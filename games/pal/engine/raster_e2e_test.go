package engine_test

// Test first. Isolated failures: native offscreen dependencies are absent from
// Go/public snapshots; observations overwrite suspended C frames; a saved hit
// animation loses immutable backups or asset references; projection aliases its
// snapshot; cyclic/oversized saved draw graphs are accepted; metadata-only rule
// archives produce a different original draw sequence from the pixel reference.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalCombatRasterSavedContinuation(t *testing.T) {
	ctx := context.Background()
	r := battleRuntime(t)
	s, e := r.StartCampaignForPlayers(ctx, 12345, 3)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; s.View.Battle.State != 1; i++ {
		if i > 512 {
			t.Fatal("original battle selection unavailable")
		}
		s, e = r.Step(ctx, s, 0)
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.View.Raster == nil {
		t.Fatal("Go omitted original offscreen surface graph")
	}
	var oracle struct {
		Pages []struct {
			View struct {
				Raster *engine.RasterGraph `json:"raster"`
			} `json:"view"`
		} `json:"pages"`
	}
	data, e := os.ReadFile(filepath.Join("..", "..", "..", ".local/pal/verification/original-battle-coop-reference.json"))
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(data, &oracle); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.View.Raster, oracle.Pages[0].View.Raster) {
		t.Fatal("headless Go raster differs from original pixel C")
	}
	v, e := r.View(s)
	if e != nil {
		t.Fatal(e)
	}
	v.Raster.Nodes[0][5] ^= 1
	again, e := r.View(s)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(again.Raster, s.View.Raster) {
		t.Fatal("public projection aliases original raster nodes")
	}
	for _, corrupt := range []func(*engine.RasterGraph){func(g *engine.RasterGraph) { g.Root = len(g.Nodes) + 1 }, func(g *engine.RasterGraph) { g.Nodes[0][1] = 1 }} {
		bad := s
		copy := *s.View.Raster
		copy.Nodes = slices.Clone(copy.Nodes)
		bad.View.Raster = &copy
		corrupt(&copy)
		if _, e = r.Encode(bad); e == nil {
			t.Fatal("invalid saved surface graph accepted")
		}
	}
	// Reach a physical effect emitted by original DATA#10, then save mid-hit.
	for i := 0; i < 1024; i++ {
		found := false
		if g := s.View.Raster; g != nil {
			for _, n := range g.Nodes {
				found = found || (n[0] == 1 && n[5] == 9)
			}
		}
		if found {
			break
		}
		keys := uint32(0)
		if s.View.Battle.State != 0 && s.View.Battle.Phase == 0 {
			keys = 2
		}
		s, e = r.StepWithInput(ctx, s, engine.Input{Direction: 4, Keys: keys})
		if e != nil {
			t.Fatal(e)
		}
		if i == 1023 {
			t.Fatal("original physical effect was never projected")
		}
	}
	encoded, e := r.Encode(s)
	if e != nil {
		t.Fatal(e)
	}
	fresh := battleRuntime(t)
	restored, e := fresh.Decode(encoded)
	if e != nil {
		t.Fatal(e)
	}
	restored, e = fresh.ReleaseInputs(ctx, restored)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s.View.Raster, restored.View.Raster) {
		t.Fatal("input release erased saved effect or backup")
	}
	for i := 0; i < 16; i++ {
		s, e = r.Step(ctx, s, 0)
		if e != nil {
			t.Fatal(e)
		}
		restored, e = fresh.Step(ctx, restored, 0)
		if e != nil {
			t.Fatal(e)
		}
		if s.Event != restored.Event || !reflect.DeepEqual(s.View, restored.View) {
			t.Fatal("fresh instance lost saved combat graphics")
		}
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "original offscreen combat graph, stripped Go vs original C, detached projection, validated dependencies, saved mid-hit in a fresh runtime", "saveBytes": len(encoded), "nodes": len(s.View.Raster.Nodes)}, "", "  ")
	if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/combat-raster-go.json"), report, 0600); e != nil {
		t.Fatal(e)
	}
}
