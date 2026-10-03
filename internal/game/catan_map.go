package game

import (
	"fmt"
	"math"
	"sort"
)

// Resource order is shared by the bank, prices, cards and trade offers.
var CatanResources = []string{"木材", "砖块", "羊毛", "粮食", "矿石"}
var catanPrices = map[string][]int{
	"catan_road": {1, 1, 0, 0, 0}, "catan_settlement": {1, 1, 1, 1, 0},
	"catan_city": {0, 0, 0, 2, 3}, "catan_buy_dev": {0, 0, 1, 1, 1},
}

type CatanTile struct {
	ID       int     `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Resource int     `json:"resource"`
	Number   int     `json:"number"`
	Vertices []int   `json:"vertices"`
}
type CatanVertex struct {
	ID    int     `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Owner int     `json:"owner"`
	Level int     `json:"level"`
}
type CatanEdge struct {
	ID    int `json:"id"`
	A     int `json:"a"`
	B     int `json:"b"`
	Owner int `json:"owner"`
}
type CatanPort struct {
	Edge     int `json:"edge"`
	Resource int `json:"resource"` // -1 is a generic 3:1 port.
}

func (g *Catan) makeMap() {
	vertices := map[string]int{}
	edges := map[string]int{}
	uses := map[int]int{}
	for r := -2; r <= 2; r++ {
		for q := max(-2, -r-2); q <= min(2, -r+2); q++ {
			x := math.Sqrt(3)*(float64(q)+float64(r)/2)*62 + 340
			y := float64(r)*93 + 290
			t := CatanTile{ID: len(g.Tiles), X: x, Y: y, Vertices: []int{}}
			for k := 0; k < 6; k++ {
				a := (30 + float64(k)*60) * math.Pi / 180
				vx := x + 62*math.Cos(a)
				vy := y + 62*math.Sin(a)
				key := fmt.Sprintf("%.3f:%.3f", vx, vy)
				id, ok := vertices[key]
				if !ok {
					id = len(g.Vertices)
					vertices[key] = id
					g.Vertices = append(g.Vertices, CatanVertex{id, vx, vy, -1, 0})
				}
				t.Vertices = append(t.Vertices, id)
			}
			for k := 0; k < 6; k++ {
				a, b := t.Vertices[k], t.Vertices[(k+1)%6]
				if a > b {
					a, b = b, a
				}
				key := fmt.Sprintf("%d:%d", a, b)
				id, ok := edges[key]
				if !ok {
					id = len(g.Edges)
					edges[key] = id
					g.Edges = append(g.Edges, CatanEdge{id, a, b, -1})
				}
				uses[id]++
			}
			g.Tiles = append(g.Tiles, t)
		}
	}
	terrain := []int{0, 0, 0, 0, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 5}
	shuffle(terrain)
	numbers := []int{2, 3, 3, 4, 4, 5, 5, 6, 6, 8, 8, 9, 9, 10, 10, 11, 11, 12}
	for {
		shuffle(numbers)
		at := 0
		for i := range g.Tiles {
			g.Tiles[i].Resource = terrain[i]
			if terrain[i] == 5 {
				g.Robber = i
				g.Tiles[i].Number = 0
			} else {
				g.Tiles[i].Number = numbers[at]
				at++
			}
		}
		valid := true
		for i, t := range g.Tiles {
			if t.Number != 6 && t.Number != 8 {
				continue
			}
			for j := i + 1; j < len(g.Tiles); j++ {
				u := g.Tiles[j]
				if (u.Number == 6 || u.Number == 8) && math.Hypot(t.X-u.X, t.Y-u.Y) < 110 {
					valid = false
				}
			}
		}
		if valid {
			break
		}
	}
	boundary := []int{}
	for id, n := range uses {
		if n == 1 {
			boundary = append(boundary, id)
		}
	}
	angle := func(id int) float64 {
		e := g.Edges[id]
		a, b := g.Vertices[e.A], g.Vertices[e.B]
		return math.Atan2((a.Y+b.Y)/2-290, (a.X+b.X)/2-340)
	}
	sort.Slice(boundary, func(i, j int) bool { return angle(boundary[i]) < angle(boundary[j]) })
	ports := []int{-1, -1, -1, -1, 0, 1, 2, 3, 4}
	shuffle(ports)
	for i, at := range []int{0, 3, 7, 10, 13, 17, 20, 23, 27} {
		g.Ports = append(g.Ports, CatanPort{boundary[at], ports[i]})
	}
}
func (g *Catan) touching(v int) []int {
	out := []int{}
	for _, e := range g.Edges {
		if e.A == v || e.B == v {
			out = append(out, e.ID)
		}
	}
	return out
}
func (g *Catan) canSettlement(p, v int, setup bool) bool {
	if v < 0 || v >= len(g.Vertices) || g.Vertices[v].Level != 0 {
		return false
	}
	connected := false
	for _, id := range g.touching(v) {
		e := g.Edges[id]
		other := e.A
		if other == v {
			other = e.B
		}
		if g.Vertices[other].Level > 0 {
			return false
		}
		if e.Owner == p {
			connected = true
		}
	}
	return setup || connected
}
func (g *Catan) canRoad(p, id int) bool {
	if id < 0 || id >= len(g.Edges) || g.Edges[id].Owner >= 0 {
		return false
	}
	e := g.Edges[id]
	for _, v := range []int{e.A, e.B} {
		vertex := g.Vertices[v]
		if vertex.Level > 0 {
			if vertex.Owner == p {
				return true
			}
			continue
		}
		for _, next := range g.touching(v) {
			if g.Edges[next].Owner == p {
				return true
			}
		}
	}
	return false
}
func (g *Catan) pieces(p int) (roads, settlements, cities int) {
	for _, e := range g.Edges {
		if e.Owner == p {
			roads++
		}
	}
	for _, v := range g.Vertices {
		if v.Owner == p {
			if v.Level == 1 {
				settlements++
			} else if v.Level == 2 {
				cities++
			}
		}
	}
	return
}
func (g *Catan) rates(p int) []int {
	rates := []int{4, 4, 4, 4, 4}
	for _, port := range g.Ports {
		e := g.Edges[port.Edge]
		if g.Vertices[e.A].Owner != p && g.Vertices[e.B].Owner != p {
			continue
		}
		if port.Resource < 0 {
			for i := range rates {
				rates[i] = min(rates[i], 3)
			}
		} else {
			rates[port.Resource] = 2
		}
	}
	return rates
}
func (g *Catan) roadLength(p int) int {
	used := make([]bool, len(g.Edges))
	best := 0
	var walk func(int, int)
	walk = func(v, n int) {
		best = max(best, n)
		if n > 0 && g.Vertices[v].Level > 0 && g.Vertices[v].Owner != p {
			return
		}
		for _, id := range g.touching(v) {
			e := g.Edges[id]
			if e.Owner != p || used[id] {
				continue
			}
			next := e.A
			if next == v {
				next = e.B
			}
			used[id] = true
			walk(next, n+1)
			used[id] = false
		}
	}
	for _, v := range g.Vertices {
		walk(v.ID, 0)
	}
	return best
}
