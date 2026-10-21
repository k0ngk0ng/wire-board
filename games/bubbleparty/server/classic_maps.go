package bubbleparty

import (
	"encoding/json"
	"sort"

	bubbledata "github.com/k0ngk0ng/wire-board/games/bubbleparty/data"
)

type classicMap struct {
	ID       string   `json:"id"`
	Width    int      `json:"width"`
	Height   int      `json:"height"`
	Spawns   [][2]int `json:"spawns"`
	Ground   [][]int  `json:"ground"`
	Objects  [][]int  `json:"objects"`
	Metadata struct {
		Pairs       [][4]int `json:"pairedCoordinatesRaw"`
		ItemWeights []struct {
			Name   string `json:"name"`
			Weight int    `json:"weightRaw"`
		} `json:"itemWeights"`
	} `json:"metadata"`
}
type classicTile struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Movable   bool   `json:"movable"`
	Footprint []int  `json:"footprintRaw"`
	Info      struct {
		Fields []int `json:"fieldsRaw"`
	} `json:"tileInfo"`
}
type classicCatalog struct {
	Maps  []classicMap  `json:"maps"`
	Tiles []classicTile `json:"tiles"`
}

var classicData = func() classicCatalog {
	var catalog classicCatalog
	if err := json.Unmarshal(bubbledata.Classic2002, &catalog); err != nil {
		panic("invalid embedded classic map catalog: " + err.Error())
	}
	if len(catalog.Maps) != 112 {
		panic("incomplete embedded classic map catalog")
	}
	return catalog
}()
var classicMapIndex = func() map[string]*classicMap {
	out := map[string]*classicMap{}
	for i := range classicData.Maps {
		m := &classicData.Maps[i]
		out[m.ID] = m
	}
	return out
}()
var classicTileIndex = func() map[int]*classicTile {
	out := map[int]*classicTile{}
	for i := range classicData.Tiles {
		t := &classicData.Tiles[i]
		out[t.ID] = t
	}
	return out
}()

func classicMapIDs() []string {
	out := make([]string, 0, len(classicData.Maps))
	for _, m := range classicData.Maps {
		out = append(out, m.ID)
	}
	return out
}
func classicOffset(m classicMap) (int, int) { return (Cols - m.Width) / 2, (Rows - m.Height) / 2 }
func nativeInBounds(x, y int) bool          { return x >= 0 && y >= 0 && x < Cols && y < Rows }
func classicObjects(id string) []int {
	m := classicMapIndex[id]
	if m == nil {
		return nil
	}
	out := make([]int, Cols*Rows)
	for i := range out {
		out[i] = -1
	}
	ox, oy := classicOffset(*m)
	for y, row := range m.Objects {
		for x, t := range row {
			out[(y+oy)*Cols+x+ox] = t
		}
	}
	return out
}
func classicCells(id string) ([]byte, bool) {
	m := classicMapIndex[id]
	if m == nil {
		return nil, false
	}
	cells := make([]byte, Cols*Rows)
	ox, oy := classicOffset(*m)
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			if x < ox || y < oy || x >= ox+m.Width || y >= oy+m.Height {
				cells[y*Cols+x] = CellWall
			}
		}
	}
	// Original loader expands from anchor X rightward and from Y-height+1 up to
	// anchor Y (jump table 0x43442c, case target 0x43429d).
	mark := func(x, y, w, h int, kind byte) {
		for dy := 1 - h; dy <= 0; dy++ {
			for dx := 0; dx < w; dx++ {
				tx, ty := x+dx, y+dy
				if nativeInBounds(tx, ty) {
					cells[ty*Cols+tx] = kind
				}
			}
		}
	}
	for _, layer := range [][][]int{m.Ground, m.Objects} {
		for y, row := range layer {
			for x, id := range row {
				t := classicTileIndex[id]
				if t == nil {
					continue
				}
				f := t.Info.Fields
				if t.Category == "static" || (t.Category == "ground" && len(f) > 6 && f[6] != 0) {
					w, h := 1, 1
					if len(t.Footprint) == 2 {
						w, h = t.Footprint[0], t.Footprint[1]
					}
					if w < 1 || h < 1 || w > Cols || h > Rows {
						panic("invalid classic footprint")
					}
					mark(x+ox, y+oy, w, h, CellWall)
				}
			}
		}
	}
	for y, row := range m.Objects {
		for x, id := range row {
			t := classicTileIndex[id]
			if t != nil && t.Category == "box" && cells[(y+oy)*Cols+x+ox] != CellWall {
				kind := byte(CellCrate)
				if t.Movable {
					kind = CellPushCrate
				}
				cells[(y+oy)*Cols+x+ox] = kind
			}
		}
	}
	return cells, true
}

func classicSpawns(id string, teamSize, team int) ([][2]int, bool) {
	m := classicMapIndex[id]
	if m == nil || teamSize < 1 || teamSize > 4 || team < 0 || team > 1 {
		return nil, false
	}
	cells, _ := classicCells(id)
	ox, oy := classicOffset(*m)
	source := make([][2]int, 0, len(m.Spawns))
	used := map[[2]int]bool{}
	for _, p := range m.Spawns {
		p[0] += ox
		p[1] += oy
		if nativeInBounds(p[0], p[1]) && cells[p[1]*Cols+p[0]] == CellEmpty && !used[p] {
			source = append(source, p)
			used[p] = true
		}
	}
	selected := make([][2]int, 0, teamSize*2)
	// Choose spread-out source positions first. Additional platform seats use
	// only real empty cells with an exit; no source obstacle is removed.
	choose := func(candidates [][2]int) {
		for len(candidates) > 0 && len(selected) < teamSize*2 {
			best, bestDistance := 0, -1
			for i, p := range candidates {
				distance := Cols*Cols + Rows*Rows
				for _, q := range selected {
					dx, dy := p[0]-q[0], p[1]-q[1]
					d := dx*dx + dy*dy
					if d < distance {
						distance = d
					}
				}
				if distance > bestDistance {
					best, bestDistance = i, distance
				}
			}
			selected = append(selected, candidates[best])
			candidates = append(candidates[:best], candidates[best+1:]...)
		}
	}
	choose(source)
	if len(selected) < teamSize*2 {
		candidates := [][2]int{}
		s := State{MapID: id, NativeObjects: classicObjects(id)}
		for y := 0; y < Rows; y++ {
			for x := 0; x < Cols; x++ {
				p := [2]int{x, y}
				if used[p] || cells[y*Cols+x] != CellEmpty {
					continue
				}
				exit := false
				for _, d := range directions {
					nx, ny := x+d[0], y+d[1]
					if nativeInBounds(nx, ny) && cells[ny*Cols+nx] == CellEmpty && s.nativePassage(x, y, d[0], d[1], false) {
						exit = true
						break
					}
				}
				if exit {
					candidates = append(candidates, p)
				}
			}
		}
		choose(candidates)
	}
	if len(selected) < teamSize*2 {
		return nil, false
	}
	// Sort geographically then assign opposite sides to teams. This does not
	// change source coordinates or the immutable map; it adapts platform teams.
	sort.Slice(selected, func(i, j int) bool {
		if selected[i][0] != selected[j][0] {
			return selected[i][0] < selected[j][0]
		}
		return selected[i][1] < selected[j][1]
	})
	return append([][2]int(nil), selected[team*teamSize:(team+1)*teamSize]...), true
}
