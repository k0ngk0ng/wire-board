package racing

// Before implementation: lateral distance cannot change tire grip or rolling
// resistance; only actual terrain triangles at original wheel positions may.
// Unknown/no-hit surfaces stay neutral, bridge layers follow height, four-wheel
// samples preserve original local +X-left/+Z-front axes, and Go/TS must agree.
import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClassicSurfaceHasNoDistancePenalty(t *testing.T) {
	for _, side := range []float64{-10000, -300, -100, 0, 100, 300, 10000} {
		road := ClassicRoadContact{NY: 1, Left: 60, Right: 60, Side: side}
		surface := ClassicSurfaceFor(road, 0)
		if surface.Grip != 1 || surface.Resistance != 0 {
			t.Fatalf("distance %.1fm changed physical surface: %+v", side/10, surface)
		}
	}
}

func TestClassicTerrainClientParity(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	out := filepath.Join(root, ".local/racing-laps/continuous/terrain-parity")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	rows := []any{}
	for _, id := range []string{"france", "coastal", "uk", "park"} {
		track, _ := classicTrackByID(id)
		world := classicTerrain.byGeometry[track.Geometry]
		seen := map[int]bool{}
		for i, triangle := range world.triangles {
			if i%19 != 0 && seen[triangle.surface] {
				continue
			}
			seen[triangle.surface] = true
			x, z, height := (triangle.a[0]+triangle.b[0]+triangle.c[0])/3, (triangle.a[2]+triangle.b[2]+triangle.c[2])/3, (triangle.a[1]+triangle.b[1]+triangle.c[1])/3
			vehicle := []string{"p911", "fm50", "ldia", "mcf1", "ccor"}[i%5]
			car := Car{X: x, Y: z, Height: height, Angle: float64(i%11)*.4 - 2, Vehicle: vehicle}
			base := ClassicRoadSurface{Grip: 1, NormalY: .98, Grade: .03}
			rows = append(rows, map[string]any{"track": id, "car": car, "contact": ClassicTerrainAt(track, x, z, height), "surface": ClassicTerrainSurface(track, &car, base)})
		}
	}
	data, _ := json.Marshal(rows)
	fixture := filepath.Join(out, "go-probes.json")
	if err := os.WriteFile(fixture, data, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--experimental-strip-types", filepath.Join(root, "games/racing/tests/classic-terrain-parity.mjs"), fixture, filepath.Join(out, "report.json"))
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real terrain Go/TS parity: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func terrainTestPlane(code int, height, left, right float64) classicTerrainMesh {
	return classicTerrainMesh{Surface: code, Vertices: [][]float64{{left, height, -100}, {right, height, -100}, {right, height, 100}, {left, height, 100}}, Faces: [][]int{{0, 1, 2, 3}}}
}

func TestClassicTerrainLayersAndActualWheelMaterials(t *testing.T) {
	world := buildClassicTerrain(classicTerrainCatalog{Surfaces: []classicTerrainMesh{terrainTestPlane(1, 0, -100, 100), terrainTestPlane(3, 100, -100, 100)}})
	for _, height := range []float64{0, 100} {
		hit := classicTerrainContactAt(world, 10, 20, height)
		if !hit.Found || math.Abs(hit.Height-height) > 1e-6 || height == 0 && hit.Surface != 1 || height == 100 && hit.Surface != 3 {
			t.Fatal("wrong actual terrain layer", hit)
		}
	}
	if hit := classicTerrainContactAt(world, 1000, 1000, 0); hit.Found || hit.Surface != -1 {
		t.Fatal("outside mesh invented a material", hit)
	}
	world = buildClassicTerrain(classicTerrainCatalog{Surfaces: []classicTerrainMesh{terrainTestPlane(1, 0, -100, 0), terrainTestPlane(3, 0, 0, 100)}})
	car := Car{Vehicle: "p911", Height: 0, Angle: 0}
	base := ClassicRoadSurface{Grip: 1, NormalY: 1}
	surface := classicTerrainTireSurface(world, &car, base)
	if surface.FrontGrip >= 1 || math.Abs(surface.RearGrip-1) > 1e-8 || surface.Resistance != 0 || surface.Grip != 1 {
		t.Fatal("tire material did not come from front/rear actual mesh", surface)
	}
	car.X = 1000
	unknown := classicTerrainTireSurface(world, &car, base)
	if unknown.FrontGrip != 1 || unknown.RearGrip != 1 || unknown.Resistance != 0 {
		t.Fatal("no-hit was treated as grass/drag", unknown)
	}
	if ClassicMaterialGrip(999) != 1 {
		t.Fatal("unknown effect was guessed as grass")
	}
}

func TestClassicPoseUsesActualTerrainHeightAndSlope(t *testing.T) {
	for _, id := range []string{"france", "coastal", "uk", "park"} {
		track, _ := classicTrackByID(id)
		world := classicTerrain.byGeometry[track.Geometry]
		checked := false
		for _, tri := range world.triangles {
			if tri.surface == 0 || tri.ny < .1 {
				continue
			}
			x, z, h := (tri.a[0]+tri.b[0]+tri.c[0])/3, (tri.a[2]+tri.b[2]+tri.c[2])/3, (tri.a[1]+tri.b[1]+tri.c[1])/3
			road := ClassicRoadAt(track, x, z, h, -1)
			hit := ClassicTerrainAt(track, x, z, h)
			if !hit.Found || math.Abs(hit.Height-road.Height) < 2 {
				continue
			}
			car := Car{Vehicle: "p911", X: x, Y: z, Height: h, RouteIndex: road.Index, Angle: .4}
			classicPose(&car, track)
			if math.Abs(car.Height-hit.Height) > 1e-5 {
				t.Fatalf("%s car retained VROAD height %.5f instead of actual ground %.5f", id, car.Height, hit.Height)
			}
			pitch := math.Atan(-(hit.NX*math.Cos(car.Angle) + hit.NZ*math.Sin(car.Angle)) / math.Max(.1, hit.NY))
			if math.Abs(car.Pitch-pitch) > 1e-6 {
				t.Fatal("car slope did not follow actual contact normal", id)
			}
			checked = true
			break
		}
		if !checked {
			t.Fatal("no real non-VROAD terrain sample exercised", id)
		}
	}
	mesh := terrainTestPlane(1, 0, -100, 100)
	for i := range mesh.Vertices {
		mesh.Vertices[i][1] = mesh.Vertices[i][0] * .5
	}
	world := buildClassicTerrain(classicTerrainCatalog{Surfaces: []classicTerrainMesh{mesh}})
	surface := classicTerrainTireSurface(world, &Car{Vehicle: "p911"}, ClassicRoadSurface{Grip: 1, NormalY: 1})
	if surface.Grade < .4 || surface.NormalY > .95 {
		t.Fatal("actual tire slope was ignored", surface)
	}
}

func TestClassicTerrainNonDriveableAndVerticalFacesAreNotFloor(t *testing.T) {
	world := buildClassicTerrain(classicTerrainCatalog{Surfaces: []classicTerrainMesh{terrainTestPlane(1, 0, -100, 100), terrainTestPlane(0, 10, -100, 100)}})
	hit := classicTerrainContactAt(world, 10, 20, 10)
	if !hit.Found || hit.Surface != 1 || hit.Height != 0 {
		t.Fatal("not-driveable roof selected as tire ground", hit)
	}
	world = buildClassicTerrain(classicTerrainCatalog{Surfaces: []classicTerrainMesh{terrainTestPlane(0, 10, -100, 100)}})
	if classicTerrainContactAt(world, 10, 20, 10).Found {
		t.Fatal("roof alone invented driveable floor")
	}
	wall := classicTerrainMesh{Surface: 1, Vertices: [][]float64{{0, 0, -100}, {.0001, 100, -100}, {.0001, 100, 100}, {0, 0, 100}}, Faces: [][]int{{0, 1, 2, 3}}}
	world = buildClassicTerrain(classicTerrainCatalog{Surfaces: []classicTerrainMesh{wall}})
	if classicTerrainContactAt(world, .00005, 0, 50).Found {
		t.Fatal("near-vertical face selected as floor")
	}
}
