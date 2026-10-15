package junqi

import "sort"

type Move struct {
	Unit   string `json:"unit"`
	From   int    `json:"from"`
	To     int    `json:"to"`
	Path   []int  `json:"path"`
	Turned bool   `json:"turned"`
}

func allied(n, a, b int) bool { return a == b || n == 4 && a%2 == b%2 }
func mobile(kind string) bool { return kind != "flag" && kind != "mine" && kind != "unknown" }

type link struct {
	to    int
	lines []string
}

func legalMoves(g Graph, pieces []PieceView, seat int) []Move {
	adj := make([][]link, len(g.Nodes))
	for _, e := range g.Edges {
		adj[e.A] = append(adj[e.A], link{e.B, e.Lines})
		adj[e.B] = append(adj[e.B], link{e.A, e.Lines})
	}
	occupied := map[int]PieceView{}
	for _, p := range pieces {
		occupied[p.Pos] = p
	}
	out := []Move{}
	for _, p := range pieces {
		if p.Seat != seat || !mobile(p.Kind) || p.Pos < 0 || p.Pos >= len(g.Nodes) || g.Nodes[p.Pos].HQ {
			continue
		}
		paths := map[int]Move{}
		allowed := func(to int) bool {
			other, found := occupied[to]
			return !found || !allied(g.Players, seat, other.Seat) && !g.Nodes[to].Camp
		}
		for _, edge := range adj[p.Pos] {
			if allowed(edge.to) {
				paths[edge.to] = Move{p.ID, p.Pos, edge.to, []int{p.Pos, edge.to}, false}
			}
		}
		walk := func(line string, turn bool) {
			type entry struct {
				node int
				path []int
			}
			queue := []entry{{p.Pos, []int{p.Pos}}}
			seen := map[int]bool{p.Pos: true}
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				for _, edge := range adj[cur.node] {
					if len(edge.lines) == 0 || line != "" && !has(edge.lines, line) || seen[edge.to] {
						continue
					}
					seen[edge.to] = true
					if !allowed(edge.to) {
						continue
					}
					path := append(append([]int(nil), cur.path...), edge.to)
					if _, exists := paths[edge.to]; !exists {
						paths[edge.to] = Move{p.ID, p.Pos, edge.to, path, turn}
					}
					if _, full := occupied[edge.to]; !full {
						queue = append(queue, entry{edge.to, path})
					}
				}
			}
		}
		lines := []string{}
		for _, edge := range adj[p.Pos] {
			for _, line := range edge.lines {
				if !has(lines, line) {
					lines = append(lines, line)
				}
			}
		}
		sort.Strings(lines)
		for _, line := range lines {
			walk(line, false)
		}
		if p.Kind == "engineer" {
			walk("", true)
		}
		for _, m := range paths {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Unit == out[j].Unit {
			return out[i].To < out[j].To
		}
		return out[i].Unit < out[j].Unit
	})
	return out
}

var ranks = map[string]int{"commander": 9, "general": 8, "division": 7, "brigade": 6, "regiment": 5, "battalion": 4, "company": 3, "platoon": 2, "engineer": 1}
var inventory = map[string]int{"commander": 1, "general": 1, "division": 2, "brigade": 2, "regiment": 2, "battalion": 2, "company": 3, "platoon": 3, "engineer": 3, "bomb": 2, "mine": 3, "flag": 1}

func combat(attacker, defender string) (aliveA, aliveD bool) {
	if attacker == "bomb" || defender == "bomb" {
		return false, false
	}
	if defender == "flag" {
		return true, false
	}
	if defender == "mine" {
		return attacker == "engineer", attacker != "engineer"
	}
	return ranks[attacker] > ranks[defender], ranks[defender] > ranks[attacker]
}
