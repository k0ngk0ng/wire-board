package racing

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"math"
)

// Essential numeric tire/terrain rules only; original client blobs stay on CDN.
//
//go:embed data/terrain-index.json data/terrain/*.json data/terrain-materials.json data/terrain-wheels.json
var classicTerrainFiles embed.FS

const classicTerrainCell = 200.

type classicTerrainMesh struct {
	Segment  int         `json:"segment"`
	Surface  int         `json:"surface"`
	Vertices [][]float64 `json:"vertices"`
	Faces    [][]int     `json:"faces"`
}
type classicTerrainCatalog struct {
	Version      int                  `json:"version"`
	Unit         string               `json:"unit"`
	ID           string               `json:"id"`
	Geometry     string               `json:"geometry"`
	SourceSHA256 string               `json:"sourceSha256"`
	Surfaces     []classicTerrainMesh `json:"surfaces"`
}
type classicTerrainTriangle struct {
	a, b, c          [3]float64
	den, nx, ny, nz  float64
	surface, segment int
}
type classicTerrainInfo struct {
	geometry  string
	triangles []classicTerrainTriangle
	grid      map[[2]int][]int
}
type ClassicTerrainContact struct {
	Found   bool    `json:"found"`
	Surface int     `json:"surface"`
	Height  float64 `json:"height"`
	NX      float64 `json:"nx"`
	NY      float64 `json:"ny"`
	NZ      float64 `json:"nz"`
	Segment int     `json:"segment"`
}
type classicTerrainWheel struct {
	Front    bool      `json:"front"`
	Position []float64 `json:"position"`
	Radius   float64   `json:"radius"`
}
type classicTerrainVehicle struct {
	GroundOffset float64               `json:"groundOffset"`
	Wheels       []classicTerrainWheel `json:"wheels"`
}
type classicTerrainRegistry struct {
	byGeometry map[string]*classicTerrainInfo
	factors    []float64
	vehicles   map[string]classicTerrainVehicle
}

var classicTerrain = loadClassicTerrain()

func loadClassicTerrain() classicTerrainRegistry {
	result := classicTerrainRegistry{byGeometry: map[string]*classicTerrainInfo{}}
	materials, err := classicTerrainFiles.ReadFile("data/terrain-materials.json")
	if err != nil {
		panic(err)
	}
	wheels, err := classicTerrainFiles.ReadFile("data/terrain-wheels.json")
	if err != nil {
		panic(err)
	}
	var material struct {
		SurfaceFactors []float64 `json:"surfaceFactors"`
	}
	var wheel struct {
		Version  int                              `json:"version"`
		Unit     string                           `json:"unit"`
		Vehicles map[string]classicTerrainVehicle `json:"vehicles"`
	}
	if json.Unmarshal(materials, &material) != nil || len(material.SurfaceFactors) != 32 || json.Unmarshal(wheels, &wheel) != nil || wheel.Version != 1 || wheel.Unit != "decimetres" {
		panic("invalid terrain wheel/material rules")
	}
	result.factors, result.vehicles = material.SurfaceFactors, wheel.Vehicles
	for _, factor := range result.factors {
		if !finite(factor) || factor < 0 || factor > 2 {
			panic("invalid terrain factor")
		}
	}
	for _, id := range []string{"p911", "fm50", "ldia", "mcf1", "ccor"} {
		v := result.vehicles[id]
		if len(v.Wheels) != 4 || !finite(v.GroundOffset) {
			panic("invalid original wheel footprint")
		}
		for _, w := range v.Wheels {
			if len(w.Position) != 3 || !finite(w.Radius) || w.Radius <= 0 {
				panic("invalid original wheel radius")
			}
			for _, n := range w.Position {
				if !finite(n) {
					panic("invalid original wheel position")
				}
			}
		}
	}
	materialHash, wheelHash := sha256.Sum256(materials), sha256.Sum256(wheels)
	var entries []struct {
		ID              string `json:"id"`
		Geometry        string `json:"geometry"`
		SHA256          string `json:"sha256"`
		SourceSHA256    string `json:"sourceSha256"`
		MaterialsSHA256 string `json:"materialsSha256"`
		WheelsSHA256    string `json:"wheelsSha256"`
	}
	index, err := classicTerrainFiles.ReadFile("data/terrain-index.json")
	if err != nil || json.Unmarshal(index, &entries) != nil {
		panic("invalid terrain index")
	}
	for _, entry := range entries {
		data, err := classicTerrainFiles.ReadFile("data/terrain/" + entry.Geometry + ".json")
		if err != nil {
			panic(err)
		}
		hash := sha256.Sum256(data)
		var catalog classicTerrainCatalog
		track, ok := classicTrackByGeometry(entry.Geometry)
		if hex.EncodeToString(hash[:]) != entry.SHA256 || hex.EncodeToString(materialHash[:]) != entry.MaterialsSHA256 || hex.EncodeToString(wheelHash[:]) != entry.WheelsSHA256 || json.Unmarshal(data, &catalog) != nil || !ok || catalog.Version != 1 || catalog.Unit != "decimetres" || catalog.Geometry != entry.Geometry || catalog.ID != entry.ID || track.ID != entry.ID || catalog.SourceSHA256 != entry.SourceSHA256 {
			panic("terrain catalog does not match frozen sources")
		}
		result.byGeometry[entry.Geometry] = buildClassicTerrain(catalog)
	}
	return result
}

func buildClassicTerrain(catalog classicTerrainCatalog) *classicTerrainInfo {
	out := &classicTerrainInfo{geometry: catalog.Geometry, grid: map[[2]int][]int{}}
	for _, patch := range catalog.Surfaces {
		if patch.Surface < 0 {
			panic("invalid raw terrain effect")
		}
		vertices := make([][3]float64, len(patch.Vertices))
		for i, v := range patch.Vertices {
			if len(v) != 3 {
				panic("invalid terrain coordinate")
			}
			for k, n := range v {
				if !finite(n) {
					panic("non-finite terrain coordinate")
				}
				vertices[i][k] = n
			}
		}
		for _, face := range patch.Faces {
			for _, i := range face {
				if i < 0 || i >= len(vertices) {
					panic("invalid terrain face")
				}
			}
			for i := 1; i+1 < len(face); i++ {
				a, b, c := vertices[face[0]], vertices[face[i]], vertices[face[i+1]]
				den := (b[2]-c[2])*(a[0]-c[0]) + (c[0]-b[0])*(a[2]-c[2])
				if math.Abs(den) < 1e-8 {
					continue
				}
				ux, uy, uz := b[0]-a[0], b[1]-a[1], b[2]-a[2]
				vx, vy, vz := c[0]-a[0], c[1]-a[1], c[2]-a[2]
				nx, ny, nz := uy*vz-uz*vy, uz*vx-ux*vz, ux*vy-uy*vx
				length := math.Sqrt(nx*nx + ny*ny + nz*nz)
				if length < 1e-8 {
					continue
				}
				if ny < 0 {
					nx, ny, nz = -nx, -ny, -nz
				}
				index := len(out.triangles)
				out.triangles = append(out.triangles, classicTerrainTriangle{a: a, b: b, c: c, den: den, nx: nx / length, ny: ny / length, nz: nz / length, surface: patch.Surface, segment: patch.Segment})
				x0, x1 := int(math.Floor(math.Min(a[0], math.Min(b[0], c[0]))/classicTerrainCell)), int(math.Floor(math.Max(a[0], math.Max(b[0], c[0]))/classicTerrainCell))
				z0, z1 := int(math.Floor(math.Min(a[2], math.Min(b[2], c[2]))/classicTerrainCell)), int(math.Floor(math.Max(a[2], math.Max(b[2], c[2]))/classicTerrainCell))
				for x := x0; x <= x1; x++ {
					for z := z0; z <= z1; z++ {
						key := [2]int{x, z}
						out.grid[key] = append(out.grid[key], index)
					}
				}
			}
		}
	}
	return out
}

func ClassicTerrainAt(track Track, x, y, height float64) ClassicTerrainContact {
	return classicTerrainContactAt(classicTerrain.byGeometry[track.Geometry], x, y, height)
}
func classicTerrainContactAt(world *classicTerrainInfo, x, z, heightHint float64) ClassicTerrainContact {
	out := ClassicTerrainContact{Surface: -1, Height: heightHint, NY: 1, Segment: -1}
	if world == nil || !finite(x) || !finite(z) || !finite(heightHint) {
		return out
	}
	best := math.Inf(1)
	for _, index := range world.grid[[2]int{int(math.Floor(x / classicTerrainCell)), int(math.Floor(z / classicTerrainCell))}] {
		t := world.triangles[index]
		if t.surface == 0 || t.ny < .1 {
			continue
		}
		u := ((t.b[2]-t.c[2])*(x-t.c[0]) + (t.c[0]-t.b[0])*(z-t.c[2])) / t.den
		v := ((t.c[2]-t.a[2])*(x-t.c[0]) + (t.a[0]-t.c[0])*(z-t.c[2])) / t.den
		w := 1 - u - v
		if u < -1e-7 || v < -1e-7 || w < -1e-7 {
			continue
		}
		height := u*t.a[1] + v*t.b[1] + w*t.c[1]
		distance := math.Abs(height - heightHint)
		if distance >= best-1e-7 {
			continue
		}
		best = distance
		out = ClassicTerrainContact{Found: true, Surface: t.surface, Height: height, NX: t.nx, NY: t.ny, NZ: t.nz, Segment: t.segment}
	}
	return out
}
func ClassicMaterialGrip(surface int) float64 {
	if surface < 0 || surface >= len(classicTerrain.factors) {
		return 1
	}
	k := (classicTerrain.factors[surface] + 1) / 2
	return k * (1 + .25*k) / 1.25
}
func ClassicTerrainSurface(t Track, c *Car, base ClassicRoadSurface) ClassicRoadSurface {
	return classicTerrainTireSurface(classicTerrain.byGeometry[t.Geometry], c, base)
}
func classicTerrainTireSurface(world *classicTerrainInfo, c *Car, base ClassicRoadSurface) ClassicRoadSurface {
	body, ok := classicTerrain.vehicles[c.Vehicle]
	if !ok {
		body = classicTerrain.vehicles["p911"]
	}
	cs, sn := math.Cos(c.Angle), math.Sin(c.Angle)
	front, rear, nFront, nRear := 0., 0., 0., 0.
	nx, ny, nz, hits := 0., 0., 0., 0
	for _, wheel := range body.Wheels {
		lx, ly, lz := wheel.Position[0], wheel.Position[1], wheel.Position[2]
		x, z := c.X+lz*cs+lx*sn, c.Y+lz*sn-lx*cs
		height := c.Height + body.GroundOffset + ly - wheel.Radius
		hit := classicTerrainContactAt(world, x, z, height)
		grip := 1.
		if hit.Found {
			grip = ClassicMaterialGrip(hit.Surface)
			nx += hit.NX
			ny += hit.NY
			nz += hit.NZ
			hits++
		}
		if wheel.Front {
			front += grip
			nFront++
		} else {
			rear += grip
			nRear++
		}
	}
	base.Grip, base.Resistance = 1, 0
	base.FrontGrip, base.RearGrip = 1, 1
	if nFront > 0 {
		base.FrontGrip = front / nFront
	}
	if nRear > 0 {
		base.RearGrip = rear / nRear
	}
	if length := math.Sqrt(nx*nx + ny*ny + nz*nz); hits > 0 && length > 1e-8 {
		nx, ny, nz = nx/length, ny/length, nz/length
		base.NormalY = math.Abs(ny)
		base.Grade = math.Atan(-(nx*cs + nz*sn) / math.Max(.1, ny))
	}
	return base
}
