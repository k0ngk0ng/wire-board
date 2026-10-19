// Package maps owns the shared, validated 13x13 battlecity map catalog.
package maps

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	Cols = 13
	Rows = 13
)

// Cell codes shared with the frontend catalog.
const (
	Empty byte = iota
	Brick
	Steel
	Water
	Trees
	Ice
)

//go:embed catalog.json
var catalogRaw []byte

type Map struct {
	ID    string           `json:"id"`
	Name  string           `json:"name"`
	Cells [Rows][Cols]byte `json:"-"`
}

type wireCatalog struct {
	Version int `json:"version"`
	Maps    []struct {
		ID    string   `json:"id"`
		Name  string   `json:"name"`
		Cells []string `json:"cells"`
	} `json:"maps"`
}

var loaded []Map

func init() {
	var w wireCatalog
	if err := json.Unmarshal(catalogRaw, &w); err != nil {
		panic("battlecity map catalog: " + err.Error())
	}
	if err := validateCatalog(w); err != nil {
		panic("battlecity map catalog: " + err.Error())
	}
	loaded = make([]Map, len(w.Maps))
	for i, m := range w.Maps {
		loaded[i] = Map{ID: m.ID, Name: m.Name}
		for y := range Rows {
			for x := range Cols {
				loaded[i].Cells[y][x] = m.Cells[y][x] - '0'
			}
		}
	}
}

func validateCatalog(w wireCatalog) error {
	if w.Version != 1 {
		return fmt.Errorf("unsupported version %d", w.Version)
	}
	if len(w.Maps) < 12 {
		return fmt.Errorf("need at least 12 maps, got %d", len(w.Maps))
	}
	seen := map[string]bool{}
	for _, m := range w.Maps {
		if m.ID == "" || seen[m.ID] {
			return fmt.Errorf("duplicate/empty map id %q", m.ID)
		}
		seen[m.ID] = true
		if m.Name == "" {
			return fmt.Errorf("%s: empty name", m.ID)
		}
		if len(m.Cells) != Rows {
			return fmt.Errorf("%s: %d rows", m.ID, len(m.Cells))
		}
		var cells [Rows][Cols]byte
		for y, row := range m.Cells {
			if len(row) != Cols {
				return fmt.Errorf("%s: row %d length %d", m.ID, y, len(row))
			}
			for x := range Cols {
				c := row[x]
				if c < '0' || c > '5' {
					return fmt.Errorf("%s: invalid cell %q at %d,%d", m.ID, c, x, y)
				}
				cells[y][x] = c - '0'
			}
		}
		if err := Validate(cells); err != nil {
			return fmt.Errorf("%s: %w", m.ID, err)
		}
	}
	return nil
}

// Validate checks symmetry, fixed bases, spawn clearance and reachability.
func Validate(c [Rows][Cols]byte) error {
	for y := range Rows {
		for x := range Cols {
			if c[y][x] != c[Rows-1-y][Cols-1-x] {
				return fmt.Errorf("not 180-degree symmetric at %d,%d", x, y)
			}
		}
	}
	for team := 0; team < 2; team++ {
		bx, by := BaseCell(team)
		if c[by][bx] != Empty {
			return fmt.Errorf("base cell %d,%d is not empty", bx, by)
		}
		// Only the base cell is reserved; the 2x2 visual base is described by
		// the contract as one 16x16 cell plus the ring.
		for _, p := range Ring(team) {
			if c[p.Y][p.X] != Brick {
				return fmt.Errorf("base ring %d,%d is not brick", p.X, p.Y)
			}
		}
	}
	for _, p := range SpawnPoints(3, 0) {
		if c[p.Y][p.X] != Empty {
			return fmt.Errorf("spawn %d,%d is not empty", p.X, p.Y)
		}
		fy := p.Y + 1
		if c[fy][p.X] == Steel || c[fy][p.X] == Water {
			return fmt.Errorf("spawn %d,%d is blocked in front", p.X, p.Y)
		}
	}
	for _, p := range SpawnPoints(3, 1) {
		if c[p.Y][p.X] != Empty {
			return fmt.Errorf("spawn %d,%d is not empty", p.X, p.Y)
		}
		fy := p.Y - 1
		if c[fy][p.X] == Steel || c[fy][p.X] == Water {
			return fmt.Errorf("spawn %d,%d is blocked in front", p.X, p.Y)
		}
	}
	for _, p := range SpawnPoints(3, 0) {
		bx, by := BaseCell(1)
		if !reachable(c, p.X, p.Y, bx, by) {
			return fmt.Errorf("spawn %d,%d cannot reach lower base", p.X, p.Y)
		}
	}
	for _, p := range SpawnPoints(3, 1) {
		bx, by := BaseCell(0)
		if !reachable(c, p.X, p.Y, bx, by) {
			return fmt.Errorf("spawn %d,%d cannot reach upper base", p.X, p.Y)
		}
	}
	return nil
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func BaseCell(team int) (int, int) {
	if team == 0 {
		return 6, 0
	}
	return 6, 12
}

func Ring(team int) []Point {
	if team == 0 {
		return []Point{{5, 0}, {7, 0}, {5, 1}, {6, 1}, {7, 1}}
	}
	return []Point{{5, 12}, {7, 12}, {5, 11}, {6, 11}, {7, 11}}
}

// SpawnPoints returns the spawn cells used by a team of the given size.
// teamSize must be 1..3.
func SpawnPoints(teamSize, team int) []Point {
	xs := []int{2, 6, 10}
	switch teamSize {
	case 1:
		xs = []int{6}
	case 2:
		xs = []int{2, 10}
	}
	y := 2
	if team == 1 {
		y = 10
	}
	out := make([]Point, len(xs))
	for i, x := range xs {
		out[i] = Point{X: x, Y: y}
	}
	return out
}

func reachable(c [Rows][Cols]byte, sx, sy, tx, ty int) bool {
	if sx == tx && sy == ty {
		return true
	}
	q := [][2]int{{sx, sy}}
	seen := [Rows][Cols]bool{}
	seen[sy][sx] = true
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			x, y := p[0]+d[0], p[1]+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows || seen[y][x] {
				continue
			}
			if c[y][x] == Steel || c[y][x] == Water {
				continue
			}
			if x == tx && y == ty {
				return true
			}
			seen[y][x] = true
			q = append(q, [2]int{x, y})
		}
	}
	return false
}

// List returns a copy of the validated catalog.
func List() []Map {
	out := make([]Map, len(loaded))
	copy(out, loaded)
	return out
}

func ByID(id string) (Map, bool) {
	for _, m := range loaded {
		if m.ID == id {
			return m, true
		}
	}
	return Map{}, false
}

func ValidID(id string) bool {
	_, ok := ByID(id)
	return ok
}

var ErrUnknown = errors.New("未知地图")
