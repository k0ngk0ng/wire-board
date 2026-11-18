package engine_test

// Test first, explicit motor fixture: original PAL_RNGPlay timing/endpoints,
// prior screen/base capture, terminal empty record, C locals and fractional
// deadlines lost by saving, seed/money altered, projected base aliases native
// state, or corrupt movie ranges accepted. Actual story stays separately gated.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func rngRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	b, err := os.ReadFile(filepath.Join(root, ".local/pal/rng-fixtures/1/rules.wasm"))
	if err != nil {
		t.Fatal("build --rng-fixture 1 first", err)
	}
	r, err := engine.NewRuntime(context.Background(), b, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(context.Background()) })
	return r
}

func TestOriginalRNGSegmentedPlaybackSavedContinuation(t *testing.T) {
	ctx := context.Background()
	r := rngRuntime(t)
	s, err := r.StartCampaignForPlayers(ctx, 12345, 3)
	if err != nil {
		t.Fatal(err)
	}
	initialSeed, initialCash := s.View.Seed, s.View.Cash
	var original struct {
		Pages []struct {
			View struct {
				Clock  uint32
				Raster struct{ Nodes [][16]int }
			}
		}
	}
	b, err := os.ReadFile("../../../.local/pal/verification/original-rng-reference.json")
	if err != nil {
		t.Fatal("generate original RNG playback reference first", err)
	}
	if err = json.Unmarshal(b, &original); err != nil {
		t.Fatal(err)
	}
	clocks := map[int]uint32{}
	for _, p := range original.Pages {
		frame := -1
		for _, n := range p.View.Raster.Nodes {
			if n[0] == 10 {
				frame = n[7]
			}
		}
		if frame >= 0 {
			clocks[frame] = p.View.Clock
		}
	}
	if len(clocks) != 410 {
		t.Fatal("incomplete original RNG timing reference")
	}
	seen := map[int]bool{}
	saved := map[int]bool{}
	var mirror *engine.Snapshot
	var fresh *engine.Runtime
	for step := 0; step < 2500; step++ {
		if s.View.Raster == nil || s.View.Raster.Base == nil {
			t.Fatal("RNG lost prior screen base")
		}
		base := s.View.Raster.Base
		if base.SceneReady || base.ClearColor != 255 {
			t.Fatal("RNG initial screen changed", base)
		}
		v, err := r.View(s)
		if err != nil {
			t.Fatal(err)
		}
		v.Raster.Base.ClearColor = 17
		again, err := r.View(s)
		if err != nil || again.Raster.Base.ClearColor != 255 {
			t.Fatal("RNG public base aliases saved screen")
		}
		frame := -1
		for _, n := range s.View.Raster.Nodes {
			if n[0] == 10 {
				if n[5] != 1 {
					t.Fatal("wrong original movie")
				}
				frame = n[7]
			}
		}
		if frame >= 0 {
			if clocks[frame] != s.View.Clock {
				t.Fatal("original C RNG deadline changed", frame, clocks[frame], s.View.Clock)
			}
			if frame >= 410 {
				t.Fatal("RNG terminal record became a rendered frame", frame)
			}
			seen[frame] = true
			if (frame == 55 || frame == 112 || frame == 267 || frame == 409) && !saved[frame] {
				data, err := r.Encode(s)
				if err != nil {
					t.Fatal(err)
				}
				fresh = rngRuntime(t)
				restored, err := fresh.Decode(data)
				if err != nil {
					t.Fatal(err)
				}
				restored, err = fresh.ReleaseInputs(ctx, restored)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(s.View.Raster, restored.View.Raster) {
					t.Fatal("RNG save lost pixels or graph")
				}
				mirror = &restored
				saved[frame] = true
			}
		}
		if s.Event.Kind == engine.SceneComplete {
			break
		}
		if s.Event.Value < 1 || s.Event.Value > 1000 {
			t.Fatal("invalid original RNG timing", s.Event)
		}
		s, err = r.Step(ctx, s, 0)
		if err != nil {
			t.Fatal(err)
		}
		if mirror != nil {
			next, err := fresh.Step(ctx, *mirror, 0)
			if err != nil {
				t.Fatal(err)
			}
			mirror = &next
			if s.Event != next.Event || !reflect.DeepEqual(s.View, next.View) {
				t.Fatal("RNG saved native continuation diverged")
			}
		}
	}
	if s.Event.Kind != engine.SceneComplete || len(seen) != 410 || len(saved) != 4 || s.View.Clock != 25627 || s.View.Seed != initialSeed || s.View.Cash != initialCash {
		t.Fatal("original segmented RNG incomplete", s.Event, len(seen), len(saved), s.View.Clock)
	}
	bad := s
	graph := *s.View.Raster
	graph.Nodes = append([][16]int(nil), graph.Nodes...)
	bad.View.Raster = &graph
	for i, n := range graph.Nodes {
		if n[0] == 10 {
			graph.Nodes[i][6] = -1
			break
		}
	}
	if _, err := r.Encode(bad); err == nil {
		t.Fatal("invalid saved RNG range accepted")
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "frames": len(seen), "freshRuntimeSaves": len(saved), "clock": s.View.Clock, "scope": "isolated original PAL_RNGPlay across six sword lesson ranges, original timing/empty stop, prior screen projection and fresh native continuation; not actual campaign or browser acceptance"}, "", "  ")
	if err := os.WriteFile("../../../.local/pal/verification/rng-native-save.json", report, 0600); err != nil {
		t.Fatal(err)
	}
}
