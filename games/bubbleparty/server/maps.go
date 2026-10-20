package bubbleparty

type mapDef struct {
	id      string
	density int
	extras  [][2]int
}

var mapDefs = []mapDef{
	{"community10", 0, nil},
	{"waterpark", 40, [][2]int{{2, 6}, {3, 6}, {12, 6}, {11, 6}}},
	{"forest", 56, [][2]int{{5, 2}, {5, 3}, {9, 10}, {9, 9}}},
	{"desert", 44, [][2]int{{3, 7}, {4, 7}, {11, 5}, {10, 5}}},
}

var spawn0 = [][2]int{{1, 1}, {1, 3}, {1, 9}, {1, 11}}
var spawn1 = [][2]int{{13, 11}, {13, 9}, {13, 3}, {13, 1}}

func mapIDs() []string {
	ids := make([]string, len(mapDefs))
	for i, m := range mapDefs {
		ids[i] = m.id
	}
	return ids
}

func mapByID(id string) (mapDef, bool) {
	for _, m := range mapDefs {
		if m.id == id {
			return m, true
		}
	}
	return mapDef{}, false
}

func spawnsFor(teamSize, team int) [][2]int {
	src := spawn0
	if team == 1 {
		src = spawn1
	}
	out := make([][2]int, 0, teamSize)
	for i := 0; i < teamSize && i < len(src); i++ {
		out = append(out, src[i])
	}
	return out
}

func spawnSafe(x, y int) bool {
	for _, s := range spawn0 {
		if absInt(s[0]-x) <= 1 && absInt(s[1]-y) <= 1 {
			return true
		}
	}
	for _, s := range spawn1 {
		if absInt(s[0]-x) <= 1 && absInt(s[1]-y) <= 1 {
			return true
		}
	}
	return false
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func minInt2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mapCells(id string, seed uint64) []byte {
	if id == "community10" {
		return districtCells()
	}
	def, ok := mapByID(id)
	if !ok {
		def, _ = mapByID("community10")
	}
	cells := make([]byte, Cols*Rows)
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			idx := y*Cols + x
			if x == 0 || y == 0 || x == Cols-1 || y == Rows-1 {
				cells[idx] = CellWall
				continue
			}
			if x%2 == 0 && y%2 == 0 && !spawnSafe(x, y) {
				cells[idx] = CellWall
			}
		}
	}
	for _, p := range def.extras {
		if !spawnSafe(p[0], p[1]) {
			cells[p[1]*Cols+p[0]] = CellWall
		}
	}
	for y := 1; y < Rows-1; y++ {
		for x := 1; x < Cols-1; x++ {
			if cells[y*Cols+x] != CellEmpty || spawnSafe(x, y) {
				continue
			}
			// Canonical coordinates keep both 180° and axis symmetry.
			cx, cy := minInt2(x, Cols-1-x), minInt2(y, Rows-1-y)
			if hash64(seed, "crate:"+id+":"+itoa(cx)+":"+itoa(cy))%100 < uint64(def.density) {
				cells[y*Cols+x] = CellCrate
			}
		}
	}
	for _, p := range append(append([][2]int{}, spawn0...), spawn1...) {
		cells[p[1]*Cols+p[0]] = CellEmpty
	}
	return cells
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// reachable reports whether every spawn point can be reached from (x, y) when
// crates are treated as destructible (walls remain blocking).
func reachable(cells []byte, x, y int) bool {
	seen := make([]bool, Cols*Rows)
	queue := [][2]int{{x, y}}
	seen[y*Cols+x] = true
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range directions {
			nx, ny := p[0]+d[0], p[1]+d[1]
			if nx < 0 || nx >= Cols || ny < 0 || ny >= Rows {
				continue
			}
			idx := ny*Cols + nx
			if seen[idx] || cells[idx] == CellWall {
				continue
			}
			seen[idx] = true
			queue = append(queue, [2]int{nx, ny})
		}
	}
	for _, p := range append(append([][2]int{}, spawn0...), spawn1...) {
		if !seen[p[1]*Cols+p[0]] {
			return false
		}
	}
	return true
}

// Public decoration codes are independent of collision cells. The district
// layout follows the official screenshot. Pine versus fruit-herb classification
// is documented in geometry-original-evidence.md; precise runtime collision
// radii remain an implementation choice, not a recovered original constant.
const (
	TileGround = iota
	TileRoad
	TileHedge
	TileTree
	TileHouseRed
	TileHouseBlue
	TileHouseYellow
	TileBrickRed
	TileBrickOrange
	TileCrate
	TileWall
)

var districtRows = []string{
	".ORORT..CTHRH.H",
	".HCHCTC..TRO...",
	".OROOT.CCTHCHCH",
	"CHCHCTC..TOROOO",
	"OORORT..CTHCHCH",
	"OHOHOTCC.TORORO",
	"TTTTT...C.TTTTT",
	"OORORTC..TOHOHR",
	"HCHCHT.CCTOOROO",
	"OORORTC..TCHCHC",
	"H.HCHT..CTOROR.",
	"..OORTCC.TCHCH.",
	"H.HRHT..CTOOO..",
}

// The official image has six conifers on the horizontal middle row. The tall
// overlapping fruit shrubs in columns 5 and 9 are passable herbs, not conifers.
func districtTree(x, y int) bool {
	return y == 6 && (x == 0 || x == 2 || x == 4 || x == 10 || x == 12 || x == 14)
}

func districtCells() []byte {
	cells := make([]byte, Cols*Rows)
	for y, row := range districtRows {
		for x, c := range row {
			switch c {
			case 'H':
				cells[y*Cols+x] = CellWall
			case 'T':
				if districtTree(x, y) {
					cells[y*Cols+x] = CellWall
				}
			case 'R', 'O', 'C':
				cells[y*Cols+x] = CellCrate
			}
		}
	}
	return cells
}

var districtSpawns = [2][][2]int{
	{{0, 0}, {1, 11}, {6, 6}, {7, 1}},
	{{14, 12}, {13, 1}, {8, 7}, {7, 10}},
}

func mapSpawnsFor(id string, teamSize, team int) [][2]int {
	if id != "community10" {
		return spawnsFor(teamSize, team)
	}
	return append([][2]int(nil), districtSpawns[team][:teamSize]...)
}

// Only the first four corner spawns require openings. These seven soft blocks
// are the explicit, documented safety adjustments to the visual reference.
func clearDistrictSpawnExit(cells []byte, x, y int) {
	var clear [][2]int
	switch [2]int{x, y} {
	case [2]int{0, 0}:
		clear = [][2]int{{0, 3}}
	case [2]int{14, 12}:
		clear = [][2]int{{14, 9}}
	case [2]int{1, 11}:
		clear = [][2]int{{1, 9}, {1, 8}}
	case [2]int{13, 1}:
		clear = [][2]int{{13, 2}, {13, 3}, {13, 4}}
	}
	for _, p := range clear {
		if cells[p[1]*Cols+p[0]] == CellCrate {
			cells[p[1]*Cols+p[0]] = CellEmpty
		}
	}
}

func mapTiles(id string, cells []byte) []int {
	tiles := make([]int, len(cells))
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			idx := y*Cols + x
			base := TileGround
			var mark byte
			if id == "community10" {
				mark = districtRows[y][x]
				if x >= 6 && x <= 8 {
					base = TileRoad
				}
			}
			tile := base
			switch cells[idx] {
			case CellWall:
				tile = TileWall
				if mark == 'T' && districtTree(x, y) {
					tile = TileTree
				}
				if mark == 'H' {
					tile = TileHouseRed
					if x < 5 && y >= 7 {
						tile = TileHouseBlue
					}
					if x >= 10 && y < 6 {
						tile = TileHouseYellow
					}
				}
			case CellCrate:
				tile = TileCrate
				if mark == 'R' {
					tile = TileBrickRed
				}
				if mark == 'O' {
					tile = TileBrickOrange
				}
				// Old saves retain their collision cells. Do not draw a solid tree
			// over a legacy walkable cell or add a wall beneath an active player.
			default:
				if mark == 'T' {
					tile = TileHedge
				}
			}
			tiles[idx] = tile
		}
	}
	return tiles
}
