// Package pairs owns the migrated wire-pairs rules. The host never decides paths.
package pairs

import "sort"

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type Cell struct {
	Kind string `json:"kind"`
	Icon int    `json:"icon,omitempty"`
	Ice  bool   `json:"ice,omitempty"`
}
type Board struct {
	Rows  int     `json:"rows"`
	Cols  int     `json:"cols"`
	Cells []*Cell `json:"cells"`
}
type Pair [2]int
type Level struct {
	Number      int    `json:"number"`
	Mechanic    string `json:"mechanic"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Board       Board  `json:"board"`
	Solution    []Pair `json:"-"`
}

func (b Board) Clone() Board {
	c := b
	c.Cells = make([]*Cell, len(b.Cells))
	for i, v := range b.Cells {
		if v != nil {
			q := *v
			c.Cells[i] = &q
		}
	}
	return c
}
func TileCount(b Board) int {
	n := 0
	for _, c := range b.Cells {
		if c != nil && c.Kind == "tile" {
			n++
		}
	}
	return n
}
func point(b Board, i int) Point { return Point{i % b.Cols, i / b.Cols} }
func sign(n int) int {
	if n < 0 {
		return -1
	}
	if n > 0 {
		return 1
	}
	return 0
}

// FindPath searches straight, one-corner, then every two-corner route, including
// the one-cell outside rim. Geometry mode is used only during generation.
func FindPath(b Board, a, z int, geometry bool) []Point {
	if b.Rows < 1 || b.Cols < 1 || a == z || a < 0 || z < 0 || a >= len(b.Cells) || z >= len(b.Cells) {
		return nil
	}
	ca, cz := b.Cells[a], b.Cells[z]
	if ca == nil || cz == nil || ca.Kind != "tile" || cz.Kind != "tile" || (!geometry && (ca.Ice || cz.Ice || ca.Icon != cz.Icon)) {
		return nil
	}
	start, end := point(b, a), point(b, z)
	free := func(p Point) bool {
		if p.X < -1 || p.Y < -1 || p.X > b.Cols || p.Y > b.Rows {
			return false
		}
		if p == start || p == end || p.X == -1 || p.Y == -1 || p.X == b.Cols || p.Y == b.Rows {
			return true
		}
		return b.Cells[p.Y*b.Cols+p.X] == nil
	}
	line := func(p, q Point) bool {
		if p.X != q.X && p.Y != q.Y {
			return false
		}
		dx, dy := sign(q.X-p.X), sign(q.Y-p.Y)
		for {
			if !free(p) {
				return false
			}
			if p == q {
				return true
			}
			p.X += dx
			p.Y += dy
		}
	}
	check := func(route ...Point) []Point {
		compact := []Point{route[0]}
		for _, p := range route[1:] {
			if p != compact[len(compact)-1] {
				compact = append(compact, p)
			}
		}
		for i := 1; i < len(compact); i++ {
			if !line(compact[i-1], compact[i]) {
				return nil
			}
		}
		return compact
	}
	if line(start, end) {
		return []Point{start, end}
	}
	for _, corner := range []Point{{start.X, end.Y}, {end.X, start.Y}} {
		if r := check(start, corner, end); r != nil {
			return r
		}
	}
	for x := -1; x <= b.Cols; x++ {
		if r := check(start, Point{x, start.Y}, Point{x, end.Y}, end); r != nil {
			return r
		}
	}
	for y := -1; y <= b.Rows; y++ {
		if r := check(start, Point{start.X, y}, Point{end.X, y}, end); r != nil {
			return r
		}
	}
	return nil
}
func neighbors(b Board, i int) []int {
	p := point(b, i)
	out := []int{}
	for _, q := range []Point{{p.X - 1, p.Y}, {p.X + 1, p.Y}, {p.X, p.Y - 1}, {p.X, p.Y + 1}} {
		if q.X >= 0 && q.Y >= 0 && q.X < b.Cols && q.Y < b.Rows {
			out = append(out, q.Y*b.Cols+q.X)
		}
	}
	return out
}
func RemovePair(b Board, a, z int) (Board, bool) {
	if FindPath(b, a, z, false) == nil {
		return b, false
	}
	c := b.Clone()
	c.Cells[a] = nil
	c.Cells[z] = nil
	for _, i := range append(neighbors(b, a), neighbors(b, z)...) {
		if cell := c.Cells[i]; cell != nil {
			if cell.Kind == "crate" {
				c.Cells[i] = nil
			} else if cell.Kind == "tile" {
				cell.Ice = false
			}
		}
	}
	return c, true
}
func FindMove(b Board) *Pair {
	groups := make([][]int, 52)
	for i, c := range b.Cells {
		if c != nil && c.Kind == "tile" && !c.Ice && c.Icon > 0 && c.Icon < 52 {
			groups[c.Icon] = append(groups[c.Icon], i)
		}
	}
	for _, g := range groups {
		for a := 0; a < len(g); a++ {
			for z := a + 1; z < len(g); z++ {
				if FindPath(b, g[a], g[z], false) != nil {
					p := Pair{g[a], g[z]}
					return &p
				}
			}
		}
	}
	return nil
}
func VerifySolution(b Board, solution []Pair) bool {
	for _, p := range solution {
		var ok bool
		b, ok = RemovePair(b, p[0], p[1])
		if !ok {
			return false
		}
	}
	return TileCount(b) == 0
}

// The original uint32 PRNG is preserved for reproducible source parity.
type rng struct{ s uint32 }

func (r *rng) next() float64 {
	r.s += 0x6D2B79F5
	t := (r.s ^ (r.s >> 15)) * (1 | r.s)
	t = (t + (t^(t>>7))*(61|t)) ^ t
	return float64(t^(t>>14)) / 4294967296
}
func (r *rng) shuffle(a []int) []int {
	out := append([]int(nil), a...)
	for i := len(out) - 1; i > 0; i-- {
		j := int(r.next() * float64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	return out
}
func mechanicFor(n int) string {
	switch {
	case n <= 3:
		return "classic"
	case n == 4:
		return "stone"
	case n == 5:
		return "reward"
	case n <= 7:
		return "crate"
	case n <= 9:
		return "ice"
	}
	return []string{"mixed", "classic", "stone", "crate", "ice", "reward", "classic", "mixed", "speed", "mixed"}[n%10]
}

var meta = map[string][2]string{
	"classic": {"初见绿野", "相同图案以最多两次转弯的线路连接。"},
	"stone":   {"林间小径", "石块阻挡路线，可以从外侧绕行或使用破障锤。"},
	"crate":   {"木箱秘境", "在木箱旁完成配对，击碎木箱打开通道。"},
	"ice":     {"薄荷冰川", "在冰封图案旁完成配对，解冻后才能消除。"},
	"reward":  {"缤纷时刻", "更小的棋盘，连击不断，比比谁的眼力更快。"},
	"mixed":   {"森林奇遇", "石块与木箱混合，寻找通路完成消除。"},
	"speed":   {"追光旅途", "跟上闪电的节奏，尽快清空棋盘。"},
}

func GenerateLevel(n int, seed uint32) Level {
	r := rng{seed ^ (uint32(n) * 2654435761)}
	mechanic := mechanicFor(n)
	rows, cols := 10, 14
	if mechanic == "reward" {
		rows = 6
	} else if n < 12 {
		rows = 8
	}
	if n < 20 {
		cols = 10
	} else if n < 60 {
		cols = 12
	}
	pool := make([]int, 51)
	for i := range pool {
		pool[i] = i + 1
	}
	if n <= 3 {
		pool = []int{1, 2, 3, 4, 5, 6, 7, 9, 10, 12, 14, 17, 18, 24, 26, 27}
	}
	icons := r.shuffle(pool)
	icons = icons[:min(len(icons), min(28, 12+n/3))]
	l := Level{Number: n, Mechanic: mechanic, Name: meta[mechanic][0], Description: meta[mechanic][1]}
	for attempt := 0; attempt < 6; attempt++ {
		b := Board{Rows: rows, Cols: cols, Cells: make([]*Cell, rows*cols)}
		available := []int{}
		for i := range b.Cells {
			b.Cells[i] = &Cell{Kind: "tile", Icon: 1}
			if i%cols > 0 && i%cols < cols-1 && i >= cols && i < (rows-1)*cols {
				available = append(available, i)
			}
		}
		available = r.shuffle(available)
		holes := 4 + 2*min(4, n/8)
		if n <= 2 {
			holes = 0
		} else if mechanic == "reward" {
			holes = 8
		}
		for k, i := range available[:holes] {
			b.Cells[i] = nil
			switch mechanic {
			case "stone":
				b.Cells[i] = &Cell{Kind: "stone"}
			case "crate":
				b.Cells[i] = &Cell{Kind: "crate"}
			case "mixed":
				kind := "stone"
				if k%2 == 1 {
					kind = "crate"
				}
				b.Cells[i] = &Cell{Kind: kind}
			}
		}
		work := b.Clone()
		order := []Pair{}
		for TileCount(work) > 0 {
			candidates := []int{}
			for i, c := range work.Cells {
				if c != nil && c.Kind == "tile" {
					candidates = append(candidates, i)
				}
			}
			candidates = r.shuffle(candidates)
			var pair *Pair
			for tries := 0; tries < 90 && pair == nil; tries++ {
				a, z := candidates[int(r.next()*float64(len(candidates)))], candidates[int(r.next()*float64(len(candidates)))]
				if FindPath(work, a, z, true) != nil {
					pair = &Pair{a, z}
				}
			}
			if pair == nil {
			outer:
				for a := 0; a < len(candidates); a++ {
					for z := a + 1; z < len(candidates); z++ {
						if FindPath(work, candidates[a], candidates[z], true) != nil {
							pair = &Pair{candidates[a], candidates[z]}
							break outer
						}
					}
				}
			}
			if pair == nil {
				break
			}
			order = append(order, *pair)
			work, _ = RemovePair(work, pair[0], pair[1])
		}
		if TileCount(work) > 0 {
			continue
		}
		for k, p := range order {
			for _, i := range p {
				b.Cells[i] = &Cell{Kind: "tile", Icon: icons[k%len(icons)]}
			}
		}
		if mechanic == "ice" {
			ranks := map[int]int{}
			positions := []int{}
			for k, p := range order {
				for _, i := range p {
					ranks[i] = k
					positions = append(positions, i)
				}
			}
			count := 0
			for _, i := range r.shuffle(positions) {
				if count >= min(10, 4+n/10) {
					break
				}
				for _, neighbor := range neighbors(b, i) {
					if rank, ok := ranks[neighbor]; ok && rank < ranks[i] {
						b.Cells[i].Ice = true
						count++
						break
					}
				}
			}
		}
		if VerifySolution(b, order) {
			l.Board = b
			l.Solution = order
			return l
		}
	}
	l.Board = Board{Rows: rows, Cols: cols, Cells: make([]*Cell, rows*cols)}
	for i := 0; i < len(l.Board.Cells); i += 2 {
		icon := icons[(i/2)%len(icons)]
		l.Board.Cells[i] = &Cell{Kind: "tile", Icon: icon}
		l.Board.Cells[i+1] = &Cell{Kind: "tile", Icon: icon}
		l.Solution = append(l.Solution, Pair{i, i + 1})
	}
	return l
}
func Reshuffle(b Board, seed uint32) (Board, bool) {
	r := rng{seed}
	positions, icons := []int{}, []int{}
	for i, c := range b.Cells {
		if c != nil && c.Kind == "tile" {
			positions = append(positions, i)
			icons = append(icons, c.Icon)
		}
	}
	if len(positions) == 0 {
		return b.Clone(), false
	}
	for attempt := 0; attempt < 40; attempt++ {
		values := r.shuffle(icons)
		c := b.Clone()
		for k, i := range positions {
			c.Cells[i].Icon = values[k]
		}
		if FindMove(c) != nil {
			return c, false
		}
	}
	sort.Ints(icons)
	c := Board{Rows: b.Rows, Cols: b.Cols, Cells: make([]*Cell, len(b.Cells))}
	for i, icon := range icons {
		c.Cells[i] = &Cell{Kind: "tile", Icon: icon}
	}
	return c, true
}
