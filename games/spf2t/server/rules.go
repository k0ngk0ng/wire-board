package spf2t

import (
	"fmt"
	"sort"
)

func (p *Player) canPlaceAt(x, y, rot int) bool {
	if p.Active == nil {
		return false
	}
	grid, err := gemGrid(p.Gems)
	if err != nil {
		return false
	}
	return pairFits(grid, *p.Active, x, y, rot)
}

func (p *Player) tryMove(dx int) bool {
	if p.Active == nil || p.Resolve != nil {
		return false
	}
	if !p.canPlaceAt(p.Active.X+dx, p.Active.Y, p.Active.Rot) {
		return false
	}
	p.Active.X += dx
	return true
}

func (p *Player) tryRotate(dir int) bool {
	if p.Active == nil || p.Resolve != nil {
		return false
	}
	target := ((p.Active.Rot+dir)%4 + 4) % 4
	kicks := [][2]int{{0, 0}, {-1, 0}, {1, 0}, {0, 1}, {0, -1}}
	for _, kick := range kicks {
		x, y := p.Active.X+kick[0], p.Active.Y+kick[1]
		if p.canPlaceAt(x, y, target) {
			p.Active.X, p.Active.Y, p.Active.Rot = x, y, target
			return true
		}
	}
	return false
}

func (p *Player) resetLock(now int64) {
	if p.LockAt > 0 && p.LockResets < MaxLockResets {
		p.LockAt = now + LockDelayMS
		p.LockResets++
	}
}

func (p *Player) softDrop(now int64) bool {
	if p.Active == nil || p.Resolve != nil {
		return false
	}
	if p.canPlaceAt(p.Active.X, p.Active.Y+1, p.Active.Rot) {
		p.Active.Y++
		p.FallAt = now + FallMS
		p.resetLock(now)
		return true
	}
	if p.LockAt == 0 {
		p.LockAt = now + LockDelayMS
	}
	return false
}

func (p *Player) hardDrop() int {
	if p.Active == nil || p.Resolve != nil {
		return 0
	}
	y := dropYFor(p.Gems, *p.Active)
	dist := y - p.Active.Y
	p.Active.Y = y
	return dist
}

func (p *Player) removeGemIDs(ids map[int]bool) []Gem {
	if len(ids) == 0 {
		return nil
	}
	removed := make([]Gem, 0, len(ids))
	kept := p.Gems[:0]
	for _, g := range p.Gems {
		if ids[g.ID] {
			removed = append(removed, g)
		} else {
			kept = append(kept, g)
		}
	}
	p.Gems = kept
	return removed
}

func (p *Player) addGem(g Gem) int {
	if g.ID == 0 {
		p.NextID++
		g.ID = p.NextID
	}
	if g.W == 0 {
		g.W = 1
	}
	if g.H == 0 {
		g.H = 1
	}
	p.Gems = append(p.Gems, g)
	return g.ID
}

// settleGems drops every entity until supported. Power gems are rigid: every
// bottom-row cell must have empty space below before the whole block moves.
func settleGems(p *Player) {
	grid, err := gemGrid(p.Gems)
	if err != nil {
		return
	}
	order := make([]int, len(p.Gems))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := p.Gems[order[i]], p.Gems[order[j]]
		ab, bb := a.Y+a.H-1, b.Y+b.H-1
		if ab != bb {
			return ab > bb
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.ID < b.ID
	})
	for _, idx := range order {
		g := &p.Gems[idx]
		for {
			can := true
			for _, c := range g.bottomCells() {
				ny := c.Y + 1
				if ny >= Rows || grid[ny][c.X] != 0 {
					can = false
					break
				}
			}
			if !can {
				break
			}
			for _, c := range g.cells() {
				grid[c.Y][c.X] = 0
			}
			g.Y++
			for _, c := range g.cells() {
				grid[c.Y][c.X] = g.ID
			}
		}
	}
}

type powerRect struct {
	X, Y, W, H int
	Color      string
}

func powerKey(r powerRect) string {
	return fmt.Sprintf("%d,%d,%d,%d,%s", r.X, r.Y, r.W, r.H, r.Color)
}

func (r powerRect) area() int { return r.W * r.H }

func rectContains(outer, inner powerRect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.W <= outer.X+outer.W && inner.Y+inner.H <= outer.Y+outer.H
}

func rectOverlaps(a, b powerRect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// mergePowerGems repeatedly turns maximal solid same-color rectangles >=2x2
// into single rigid entities. Existing power gems may grow; a candidate that
// cuts a power gem in half is rejected.
func mergePowerGems(p *Player) []powerRect {
	var existing []powerRect
	for _, g := range p.Gems {
		if g.Kind == GemPower {
			existing = append(existing, powerRect{X: g.X, Y: g.Y, W: g.W, H: g.H, Color: g.Color})
		}
	}
	created := []powerRect{}
	for {
		var color [Rows][Cols]string
		for _, g := range p.Gems {
			if g.Kind != GemNormal && g.Kind != GemPower {
				continue
			}
			for _, c := range g.cells() {
				color[c.Y][c.X] = g.Color
			}
		}
		exact := map[string]bool{}
		for _, r := range existing {
			exact[powerKey(r)] = true
		}
		var best *powerRect
		for y := 0; y < Rows; y++ {
			for x := 0; x < Cols; x++ {
				base := color[y][x]
				if base == "" {
					continue
				}
				for w := 1; x+w <= Cols; w++ {
					if color[y][x+w-1] != base {
						break
					}
					for h := 1; y+h <= Rows; h++ {
						ok := true
						for yy := y; yy < y+h && ok; yy++ {
							for xx := x; xx < x+w; xx++ {
								if color[yy][xx] != base {
									ok = false
									break
								}
							}
						}
						if !ok {
							break
						}
						if w < 2 || h < 2 {
							continue
						}
						r := powerRect{X: x, Y: y, W: w, H: h, Color: base}
						if exact[powerKey(r)] {
							continue
						}
						partial := false
						for _, old := range existing {
							if old.Color == base && rectOverlaps(old, r) && !rectContains(r, old) {
								partial = true
								break
							}
						}
						if partial {
							continue
						}
						if best == nil || betterRect(r, *best) {
							copy := r
							best = &copy
						}
					}
				}
			}
		}
		if best == nil {
			break
		}
		remove := map[int]bool{}
		for _, g := range p.Gems {
			if g.Kind != GemNormal && g.Kind != GemPower {
				continue
			}
			for _, c := range g.cells() {
				if c.X >= best.X && c.X < best.X+best.W && c.Y >= best.Y && c.Y < best.Y+best.H {
					remove[g.ID] = true
					break
				}
			}
		}
		p.removeGemIDs(remove)
		p.NextID++
		p.Gems = append(p.Gems, Gem{ID: p.NextID, Kind: GemPower, Color: best.Color, X: best.X, Y: best.Y, W: best.W, H: best.H})
		existing = append(existing, *best)
		created = append(created, *best)
	}
	return created
}

func betterRect(a, b powerRect) bool {
	if a.area() != b.area() {
		return a.area() > b.area()
	}
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	if a.X != b.X {
		return a.X < b.X
	}
	if a.W != b.W {
		return a.W > b.W
	}
	return a.H > b.H
}

type clearGroup struct {
	color      string
	cells      []Point
	gemIDs     []int
	flatCells  int
	crashCells int
	powers     []powerRect
}

func powerBase(area, w, h int) int {
	switch area {
	case 4:
		return 9
	case 6:
		return 13
	case 8:
		return 17
	case 9:
		return 21
	case 10:
		return 25
	case 12:
		return 30
	case 15:
		return 39
	case 16:
		return 50
	}
	base := 3*area + 2
	if w == h {
		base += (area + 9) / 10
	}
	return base
}

// chainBonus follows the arcade's escalating chain payoff for chains 1..9 and
// adds a constant increment thereafter.
func chainBonus(chain int) int {
	table := []int{0, 2, 4, 10, 16, 22, 28, 34}
	if chain < 1 {
		return 0
	}
	if chain <= len(table) {
		return table[chain-1]
	}
	return table[len(table)-1] + 6*(chain-len(table))
}

func (p *Player) gridAndGems() ([Rows][Cols]int, map[int]Gem) {
	grid, _ := gemGrid(p.Gems)
	byID := make(map[int]Gem, len(p.Gems))
	for _, g := range p.Gems {
		byID[g.ID] = g
	}
	return grid, byID
}

func (p *Player) detectCrashGroups() []clearGroup {
	grid, byID := p.gridAndGems()
	visited := map[[2]int]bool{}
	var groups []clearGroup
	for _, crash := range p.Gems {
		if crash.Kind != GemCrash {
			continue
		}
		triggered := false
		for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			x, y := crash.X+d[0], crash.Y+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows {
				continue
			}
			id := grid[y][x]
			if id == 0 {
				continue
			}
			g, ok := byID[id]
			if !ok || g.Color != crash.Color {
				continue
			}
			if g.Kind == GemNormal || g.Kind == GemCrash || g.Kind == GemPower {
				triggered = true
				break
			}
		}
		if !triggered || visited[[2]int{crash.X, crash.Y}] {
			continue
		}
		group := clearGroup{color: crash.Color}
		queue := []Point{{X: crash.X, Y: crash.Y}}
		seenGems := map[int]bool{}
		seenCells := map[[2]int]bool{}
		for len(queue) > 0 {
			pt := queue[0]
			queue = queue[1:]
			key := [2]int{pt.X, pt.Y}
			if visited[key] || seenCells[key] {
				continue
			}
			id := grid[pt.Y][pt.X]
			if id == 0 {
				continue
			}
			g, ok := byID[id]
			if !ok || g.Color != crash.Color {
				continue
			}
			if g.Kind != GemNormal && g.Kind != GemCrash && g.Kind != GemPower {
				continue
			}
			visited[key] = true
			seenCells[key] = true
			group.cells = append(group.cells, pt)
			if !seenGems[g.ID] {
				seenGems[g.ID] = true
				group.gemIDs = append(group.gemIDs, g.ID)
				switch g.Kind {
				case GemNormal:
					group.flatCells++
				case GemCrash:
					group.flatCells++
					group.crashCells++
				case GemPower:
					group.powers = append(group.powers, powerRect{X: g.X, Y: g.Y, W: g.W, H: g.H, Color: g.Color})
					for _, c := range g.cells() {
						if !visited[[2]int{c.X, c.Y}] && !seenCells[[2]int{c.X, c.Y}] {
							queue = append(queue, c)
						}
					}
				}
			}
			for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
				nx, ny := pt.X+d[0], pt.Y+d[1]
				if nx >= 0 && nx < Cols && ny >= 0 && ny < Rows && !visited[[2]int{nx, ny}] {
					queue = append(queue, Point{X: nx, Y: ny})
				}
			}
		}
		if len(group.cells) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func (p *Player) clearGroups(groups []clearGroup) (cells []Point, flat, crash, counter int, powers []powerRect) {
	grid, byID := p.gridAndGems()
	remove := map[int]bool{}
	cellSeen := map[[2]int]bool{}
	for _, group := range groups {
		flat += group.flatCells
		crash += group.crashCells
		powers = append(powers, group.powers...)
		for _, id := range group.gemIDs {
			remove[id] = true
		}
		for _, c := range group.cells {
			if !cellSeen[[2]int{c.X, c.Y}] {
				cellSeen[[2]int{c.X, c.Y}] = true
				cells = append(cells, c)
			}
		}
	}
	// Numbered counter gems adjacent to any removed cell are destroyed too.
	for _, c := range cells {
		for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			x, y := c.X+d[0], c.Y+d[1]
			if x < 0 || x >= Cols || y < 0 || y >= Rows {
				continue
			}
			id := grid[y][x]
			g, ok := byID[id]
			if !ok || g.Kind != GemCounter || remove[id] {
				continue
			}
			remove[id] = true
			if !cellSeen[[2]int{x, y}] {
				cellSeen[[2]int{x, y}] = true
				cells = append(cells, Point{X: x, Y: y})
				counter++
			}
		}
	}
	p.removeGemIDs(remove)
	return cells, flat, crash, counter, powers
}

func (p *Player) diamondClear(target string, dx, dy int) (cells []Point, flat, counter int, powers []powerRect) {
	if !validColor(target) {
		return nil, 0, 0, nil
	}
	remove := map[int]bool{}
	seen := map[[2]int]bool{}
	for _, g := range p.Gems {
		if g.Kind == GemDiamond && g.X == dx && g.Y == dy {
			remove[g.ID] = true
			continue
		}
		if g.Color != target {
			continue
		}
		switch g.Kind {
		case GemNormal:
			remove[g.ID] = true
			flat++
		case GemCrash:
			remove[g.ID] = true
			flat++
		case GemCounter:
			remove[g.ID] = true
			counter++
		case GemPower:
			remove[g.ID] = true
			powers = append(powers, powerRect{X: g.X, Y: g.Y, W: g.W, H: g.H, Color: g.Color})
		}
		for _, c := range g.cells() {
			if !seen[[2]int{c.X, c.Y}] {
				seen[[2]int{c.X, c.Y}] = true
				cells = append(cells, c)
			}
		}
	}
	p.removeGemIDs(remove)
	return cells, flat, counter, powers
}

func diamondTarget(p *Player, x, y int) string {
	grid, byID := p.gridAndGems()
	ny := y + 1
	if ny < 0 || ny >= Rows || x < 0 || x >= Cols {
		return ""
	}
	id := grid[ny][x]
	if id == 0 {
		return ""
	}
	g, ok := byID[id]
	if !ok || g.Kind == GemDiamond {
		return ""
	}
	return g.Color
}

func stepAttack(chain, base, groupCount int, diamond bool) int {
	if base <= 0 {
		return 0
	}
	if groupCount < 1 {
		groupCount = 1
	}
	simNum := 4 + groupCount
	if simNum > 10 {
		simNum = 10
	}
	base = base * simNum / 5
	if diamond {
		base /= 2
	}
	if base <= 0 {
		base = 1
	}
	return base + chainBonus(chain)
}
