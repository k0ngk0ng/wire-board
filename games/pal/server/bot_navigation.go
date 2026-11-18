package pal

import "container/heap"

// The graph records consecutive publicly controllable positions, including
// the endpoints of script movement and scene changes. Its reverse edges are navigation hypotheses;
// ordinary input and the original engine still decide what can be traversed.
func validFieldBotNavigation(m *fieldBotMemory) bool {
	if len(m.Nav) > 1024 || len(m.Edges) > 4096 || m.Still < 0 || m.Still > 8 || m.NavLast != nil && !validBotPlace(*m.NavLast) {
		return false
	}
	seen := make(map[fieldBotPlace]bool, len(m.Nav))
	for _, p := range m.Nav {
		if !validBotPlace(p) || seen[p] {
			return false
		}
		seen[p] = true
	}
	if m.NavLast != nil && !seen[*m.NavLast] {
		return false
	}
	edges := make(map[[2]int]bool, len(m.Edges))
	for _, e := range m.Edges {
		if e[0] < 0 || e[1] >= len(m.Nav) || e[0] >= e[1] || edges[e] {
			return false
		}
		edges[e] = true
	}
	return true
}

func rememberBotNavigation(m *fieldBotMemory, v View) {
	// Script interpolation points are visible but cannot be reached by ordinary
	// walking. Join the last controllable position to the next field position.
	if v.Field == nil || v.Scene < 1 || v.Scene > 65535 {
		return
	}
	p := fieldBotPlace{v.Scene, v.X, v.Y}
	if !validBotPlace(p) {
		return
	}
	if m.NavLast != nil && *m.NavLast == p {
		m.Still = min(8, m.Still+1)
		return
	}
	m.Still = 0
	find := func(p fieldBotPlace) int {
		for i, n := range m.Nav {
			if n == p {
				return i
			}
		}
		return -1
	}
	index := find(p)
	last := -1
	if m.NavLast != nil {
		last = find(*m.NavLast)
	}
	if len(m.Nav) >= 1024 || len(m.Edges) >= 4096 {
		m.Nav, m.Edges = nil, nil
		index, last = -1, -1
		if m.NavLast != nil {
			m.Nav = append(m.Nav, *m.NavLast)
			last = 0
		}
	}
	if index < 0 {
		index = len(m.Nav)
		m.Nav = append(m.Nav, p)
	}
	if last >= 0 && last != index {
		e := [2]int{min(last, index), max(last, index)}
		exists := false
		for _, old := range m.Edges {
			if old == e {
				exists = true
				break
			}
		}
		if !exists {
			m.Edges = append(m.Edges, e)
		}
	}
	m.NavLast = &p
}

func fieldBotKnownRoute(v View, m *fieldBotMemory, target *fieldBotPlace) (int, bool) {
	if target == nil || len(m.Nav) == 0 || v.Field == nil {
		return 0, false
	}
	start, goal, best := -1, -1, 65
	for i, p := range m.Nav {
		if p.Scene == v.Scene && p.X == v.X && p.Y == v.Y {
			start = i
		}
		if p.Scene == target.Scene {
			if d := fieldDistance(p.X, p.Y, target.X, target.Y); d < best {
				goal, best = i, d
			}
		}
	}
	if start < 0 || goal < 0 || start == goal {
		return 0, false
	}
	adj := make([][]int, len(m.Nav))
	for _, e := range m.Edges {
		adj[e[0]] = append(adj[e[0]], e[1])
		adj[e[1]] = append(adj[e[1]], e[0])
	}
	parent := make([]int, len(m.Nav))
	costs := make([]int, len(m.Nav))
	for i := range parent {
		parent[i] = -1
		costs[i] = int(^uint(0) >> 1)
	}
	parent[start] = start
	costs[start] = 0
	queue := &fieldBotNavQueue{{start, 0}}
	for queue.Len() > 0 {
		step := heap.Pop(queue).(fieldBotNavStep)
		if step.cost != costs[step.node] {
			continue
		}
		if step.node == goal {
			break
		}
		for _, n := range adj[step.node] {
			from, to := m.Nav[step.node], m.Nav[n]
			cost := max(1, (fieldDistance(from.X, from.Y, to.X, to.Y)+31)/32)
			if fieldBotNavTransition(from, to) {
				cost = 256
			}
			if next := step.cost + cost; next < costs[n] {
				parent[n], costs[n] = step.node, next
				heap.Push(queue, fieldBotNavStep{n, next})
			}
		}
	}
	if parent[goal] < 0 {
		return 0, false
	}
	path := []int{goal}
	for path[len(path)-1] != start {
		path = append(path, parent[path[len(path)-1]])
	}
	for a, b := 0, len(path)-1; a < b; a, b = a+1, b-1 {
		path[a], path[b] = path[b], path[a]
	}
	firstTransition := len(path)
	for i := 1; i < len(path); i++ {
		if fieldBotNavTransition(m.Nav[path[i-1]], m.Nav[path[i]]) {
			firstTransition = i
			break
		}
	}
	// Observed walks need not intersect at an identical lattice point. If a
	// later part of the route is reachable on the displayed floor, walk there
	// directly instead of following an obsolete detour through another room.
	// Exclude all touch cells so this shortcut cannot silently change scenes.
	blocked := [289]bool{}
	for i := range blocked {
		blocked[i] = v.Field.Cells[i]&2 != 0
	}
	for i := len(path) - 1; i > firstTransition; i-- {
		p := m.Nav[path[i]]
		if p.Scene == v.Scene {
			if step := fieldBotRoute(v.Field, p.X, p.Y, blocked, true); step >= 0 {
				return step, true
			}
		}
	}
	direction := -1
	var exit *fieldBotPlace
	for i, n := range path[1:] {
		p := m.Nav[n]
		source := m.Nav[path[i]]
		if fieldBotNavTransition(source, p) {
			if fieldDistance(v.X, v.Y, source.X, source.Y) <= 192 {
				exit = &source
			}
			break
		}
		if step := fieldBotRoute(v.Field, p.X, p.Y, [289]bool{}, true); step >= 0 {
			direction = step
		}
	}
	if direction >= 0 && exit == nil {
		return direction, true
	}
	if v.Field.Cells[8*17+8]&2 != 0 && m.Still < 4 {
		return 4, true
	}
	// A scene transition can yield only after scripted entry movement. Search
	// nearby public trigger cells instead of assuming the observed point is a
	// door, and prefer positions not repeatedly walked through.
	allowed := [289]bool{}
	if exit != nil {
		nearest, distance := -1, 193
		for index, flags := range v.Field.Cells[:289] {
			if flags&2 == 0 {
				continue
			}
			u, w := index%17-8, index/17-8
			x, y := v.X+(u-w)*16, v.Y+(u+w)*8
			if d := fieldDistance(exit.X, exit.Y, x, y); d < distance {
				nearest, distance = index, d
			}
		}
		if nearest >= 0 {
			// Commit to the nearest trigger footprint. Recent walking through
			// a staircase must not send us to an unrelated bedroom doorway.
			queue := []int{nearest}
			allowed[nearest] = true
			for at := 0; at < len(queue); at++ {
				index := queue[at]
				for _, delta := range fieldBotDeltas {
					u, w := index%17+delta[0], index/17+delta[1]
					if u < 0 || u >= 17 || w < 0 || w >= 17 {
						continue
					}
					n := w*17 + u
					if !allowed[n] && v.Field.Cells[n]&2 != 0 {
						allowed[n] = true
						queue = append(queue, n)
					}
				}
			}
		}
	}
	score := int(^uint(0) >> 1)
	for w := -8; w <= 8; w++ {
		for u := -8; u <= 8; u++ {
			if v.Field.Cells[(w+8)*17+u+8] != 2 {
				continue
			}
			x, y := v.X+(u-w)*16, v.Y+(u+w)*8
			if fieldDistance(v.X, v.Y, x, y) > 192 {
				continue
			}
			if exit != nil && !allowed[(w+8)*17+u+8] {
				continue
			}
			visits := 0
			for _, old := range m.Walks {
				if old.Scene == v.Scene && fieldDistance(old.X, old.Y, x, y) < 64 {
					visits++
				}
			}
			cost := visits*1024 + fieldDistance(v.X, v.Y, x, y)
			if cost < score {
				if step := fieldBotRoute(v.Field, x, y, [289]bool{}, true); step >= 0 {
					direction, score = step, cost
				}
			}
		}
	}
	return direction, direction >= 0
}

func fieldBotNavTransition(from, to fieldBotPlace) bool {
	dx, dy := to.X-from.X, to.Y-from.Y
	return from.Scene != to.Scene || max(dx, -dx) != 16 || max(dy, -dy) != 8
}

type fieldBotNavStep struct{ node, cost int }
type fieldBotNavQueue []fieldBotNavStep

func (q fieldBotNavQueue) Len() int { return len(q) }
func (q fieldBotNavQueue) Less(i, j int) bool {
	if q[i].cost == q[j].cost {
		return q[i].node < q[j].node
	}
	return q[i].cost < q[j].cost
}
func (q fieldBotNavQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *fieldBotNavQueue) Push(x any)   { *q = append(*q, x.(fieldBotNavStep)) }
func (q *fieldBotNavQueue) Pop() any {
	last := (*q)[len(*q)-1]
	*q = (*q)[:len(*q)-1]
	return last
}
