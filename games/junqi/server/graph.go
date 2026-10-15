package junqi

import (
	"fmt"
	"sort"
)

type Node struct {
	ID   int  `json:"id"`
	Seat int  `json:"seat"`
	Row  int  `json:"row"`
	Col  int  `json:"col"`
	X    int  `json:"x"`
	Y    int  `json:"y"`
	Camp bool `json:"camp"`
	HQ   bool `json:"hq"`
}
type Edge struct {
	A     int      `json:"a"`
	B     int      `json:"b"`
	Lines []string `json:"lines"`
	Curve bool     `json:"curve"`
}
type Graph struct {
	Players int    `json:"players"`
	Nodes   []Node `json:"nodes"`
	Edges   []Edge `json:"edges"`
}

func Home(seat, row, col int) int { return seat*30 + row*5 + col }
func isCamp(row, col int) bool {
	return row == 1 && (col == 1 || col == 3) || row == 2 && col == 2 || row == 3 && (col == 1 || col == 3)
}
func has(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func makeGraph(n int) Graph {
	g := Graph{Players: n, Nodes: []Node{}, Edges: []Edge{}}
	base := 3
	if n == 2 {
		base = 1
	}
	for seat := 0; seat < n; seat++ {
		for r := 0; r < 6; r++ {
			for c := 0; c < 5; c++ {
				x, y := c-2, base+r
				turns := seat
				if n == 2 {
					turns *= 2
				}
				for k := 0; k < turns; k++ {
					x, y = -y, x
				}
				g.Nodes = append(g.Nodes, Node{Home(seat, r, c), seat, r, c, x, y, isCamp(r, c), r == 5 && (c == 1 || c == 3)})
			}
		}
	}
	if n == 4 {
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				g.Nodes = append(g.Nodes, Node{ID: 120 + r*3 + c, Seat: -1, Row: r, Col: c, X: 2*c - 2, Y: 2*r - 2})
			}
		}
	}
	edges := map[[2]int]*Edge{}
	add := func(a, b int, rail bool, extra string, curve bool) {
		if a > b {
			a, b = b, a
		}
		key := [2]int{a, b}
		edge := edges[key]
		if edge == nil {
			edge = &Edge{A: a, B: b, Lines: []string{}}
			edges[key] = edge
		}
		if rail {
			line := ""
			if g.Nodes[a].X == g.Nodes[b].X {
				line = fmt.Sprintf("v:%d", g.Nodes[a].X)
			} else if g.Nodes[a].Y == g.Nodes[b].Y {
				line = fmt.Sprintf("h:%d", g.Nodes[a].Y)
			}
			if line != "" && !has(edge.Lines, line) {
				edge.Lines = append(edge.Lines, line)
			}
		}
		if extra != "" && !has(edge.Lines, extra) {
			edge.Lines = append(edge.Lines, extra)
		}
		edge.Curve = edge.Curve || curve
	}
	for seat := 0; seat < n; seat++ {
		for r := 0; r < 6; r++ {
			for c := 0; c < 5; c++ {
				a := Home(seat, r, c)
				if c < 4 {
					add(a, Home(seat, r, c+1), r == 0 || r == 4, "", false)
				}
				if r < 5 {
					add(a, Home(seat, r+1, c), r < 4 && (c == 0 || c == 4), "", false)
				}
				if isCamp(r, c) {
					for _, d := range [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
						rr, cc := r+d[0], c+d[1]
						if rr >= 0 && rr < 6 && cc >= 0 && cc < 5 {
							add(a, Home(seat, rr, cc), false, "", false)
						}
					}
				}
			}
		}
	}
	if n == 2 {
		for _, c := range []int{0, 2, 4} {
			add(Home(0, 0, c), Home(1, 0, 4-c), true, "", false)
		}
	} else {
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				a := 120 + r*3 + c
				if c < 2 {
					add(a, a+1, true, "", false)
				}
				if r < 2 {
					add(a, a+3, true, "", false)
				}
			}
		}
		for seat := 0; seat < 4; seat++ {
			for _, c := range []int{0, 2, 4} {
				a := Home(seat, 0, c)
				p := g.Nodes[a]
				x, y := p.X, p.Y
				if x == 3 {
					x = 2
				}
				if x == -3 {
					x = -2
				}
				if y == 3 {
					y = 2
				}
				if y == -3 {
					y = -2
				}
				b := 120 + (y+2)/2*3 + (x+2)/2
				add(a, b, true, "", false)
			}
			next := (seat + 1) % 4
			line := fmt.Sprintf("arc:%d", seat)
			add(Home(seat, 0, 0), Home(next, 0, 4), false, line, true)
			for r := 0; r < 4; r++ {
				add(Home(seat, r, 0), Home(seat, r+1, 0), true, line, false)
				add(Home(next, r, 4), Home(next, r+1, 4), true, line, false)
			}
		}
	}
	for _, e := range edges {
		sort.Strings(e.Lines)
		g.Edges = append(g.Edges, *e)
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].A == g.Edges[j].A {
			return g.Edges[i].B < g.Edges[j].B
		}
		return g.Edges[i].A < g.Edges[j].A
	})
	return g
}
