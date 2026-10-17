package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalEightCourtTerrainAndBoundaryInstructions(t *testing.T) {
	f, e := os.Open("testdata/court-profiles.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var data struct {
		Probes []struct {
			Court uint8
			State CourtTerrainProbe
			Want  CourtTerrainProbeResult
		}
		Bounds []struct {
			Court, Object uint8
			State         CourtBoundaryState
			Want          CourtBoundaryState
		}
	}
	if e = json.NewDecoder(z).Decode(&data); e != nil {
		t.Fatal(e)
	}
	seen := [8]bool{}
	edges := 0
	for i, row := range data.Probes {
		got, e := ProbeOriginalCourtTerrain(row.State, row.Court)
		if e != nil || got != row.Want {
			t.Fatalf("89FB probe %d court%d state%+v got%+v want%+v error%v", i, row.Court, row.State, got, row.Want, e)
		}
		seen[row.Court] = true
		if got.State.Flags&64 != 0 || got.State.Position.Depth != row.State.Position.Depth {
			edges++
		}
	}
	for i, row := range data.Bounds {
		got, e := ClampOriginalCourtTravel(row.State, row.Court, row.Object)
		if e != nil || got != row.Want {
			t.Fatalf("8021 bounds %d court%d object%d state%+v got%+v want%+v error%v", i, row.Court, row.Object, row.State, got, row.Want, e)
		}
	}
	if len(data.Probes) != 29960 || len(data.Bounds) != 19968 || edges == 0 || seen != ([8]bool{true, true, true, true, true, true, true, true}) {
		t.Fatal("oracle coverage", len(data.Probes), len(data.Bounds), edges, seen)
	}
	t.Logf("original isolated instruction comparison: %d terrain probes, %d object boundary cases, %d platform edge effects", len(data.Probes), len(data.Bounds), edges)
}

func TestOriginalCourtProfileLimitsAndNewYorkParity(t *testing.T) {
	rows := [8]uint8{10, 12, 10, 11, 10, 11, 9, 11}
	limits := [8]uint8{73, 88, 73, 88, 80, 81, 72, 81}
	for c := uint8(0); c < 8; c++ {
		p, e := OriginalCourtProfile(c)
		if e != nil || p.Court != c || p.TerrainRows != rows[c] || p.MaximumDepth != limits[c] {
			t.Fatal(c, p, e)
		}
		for y := uint8(0); y < rows[c]; y++ {
			for x := uint8(0); x < 64; x++ {
				cell, e := OriginalCourtSurface(c, Fixed(x)*8*Pixel+255, Fixed(y)*8*Pixel+255)
				if e != nil {
					t.Fatal(c, x, y, e)
				}
				if cell.Height() != Fixed(uint8(cell)&248)*Pixel || cell.Travel() != uint8(cell)&7 {
					t.Fatal("raw-cell decoding", cell)
				}
				if c == 1 && uint8(cell)&248 != originalNewYorkShadowSurface[y][x] {
					t.Fatal("existing New York floor changed", x, y, cell)
				}
			}
		}
		if _, e := OriginalCourtSurface(c, 0, Fixed(rows[c])*8*Pixel); e == nil {
			t.Fatal("out-of-table depth accepted", c)
		}
	}
	for _, p := range []Position{{X: -1}, {X: 512 * Pixel}, {Depth: -1}} {
		if _, e := OriginalCourtSurface(1, p.X, p.Depth); e == nil {
			t.Fatal("out-of-table position accepted", p)
		}
	}
	if _, e := OriginalCourtProfile(8); e == nil {
		t.Fatal("unknown court accepted")
	}
	s := CourtTerrainProbe{Position: Position{X: 512 * Pixel}, Flags: 255}
	if got, e := ProbeOriginalCourtTerrain(s, 1); e == nil || got.State != s {
		t.Fatal("invalid terrain probe must fail atomically", got, e)
	}
	b := CourtBoundaryState{Body: Body{Position: Position{X: 200 * Pixel}}, Action: 2, Entered: true}
	if got, e := ClampOriginalCourtTravel(b, 1, 13); e == nil || got != b {
		t.Fatal("invalid object boundary must fail atomically", got, e)
	}
}
