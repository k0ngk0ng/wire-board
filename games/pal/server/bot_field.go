package pal

import (
	"strings"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

type fieldBotVisit struct {
	Scene int    `json:"scene"`
	NPC   int    `json:"npc"`
	Frame uint32 `json:"frame"`
}

// History contains only prior public field observations, never native memory,
// event scripts or undiscovered coordinates. It is bounded and saved with play.
type fieldBotMemory struct {
	Recent      []fieldBotVisit `json:"recent,omitempty"`
	Nearby      *fieldBotVisit  `json:"nearby,omitempty"`
	Last        *fieldBotPlace  `json:"last,omitempty"`
	Links       []fieldBotPlace `json:"links,omitempty"`
	Walks       []fieldBotPlace `json:"walks,omitempty"`
	SearchItems bool            `json:"searchItems,omitempty"`
	Wine        *fieldBotWine   `json:"wine,omitempty"`
	Nav         []fieldBotPlace `json:"nav,omitempty"`
	Edges       [][2]int        `json:"edges,omitempty"`
	NavLast     *fieldBotPlace  `json:"navLast,omitempty"`
	Still       int             `json:"still,omitempty"`
}

type fieldBotPlace struct {
	Scene int `json:"scene"`
	X     int `json:"x"`
	Y     int `json:"y"`
}

func validBotPlace(place fieldBotPlace) bool {
	return place.Scene > 0 && place.Scene <= 65535 && place.X >= -65536 && place.X <= 131070 && place.Y >= -65536 && place.Y <= 131070
}

func validFieldBotMemory(memory *fieldBotMemory) bool {
	if memory == nil {
		return true
	}
	if !validFieldBotWine(memory.Wine) || !validFieldBotNavigation(memory) {
		return false
	}
	if len(memory.Recent) > 16 {
		return false
	}
	if len(memory.Links) > 32 || len(memory.Walks) > 256 || memory.Last != nil && !validBotPlace(*memory.Last) {
		return false
	}
	for _, place := range memory.Links {
		if !validBotPlace(place) {
			return false
		}
	}
	for _, place := range memory.Walks {
		if !validBotPlace(place) {
			return false
		}
	}
	if visit := memory.Nearby; visit != nil && (visit.Scene < 1 || visit.Scene > 65535 || visit.NPC < 1 || visit.NPC > 65535) {
		return false
	}
	for i, visit := range memory.Recent {
		if visit.Scene < 1 || visit.Scene > 65535 || visit.NPC < 1 || visit.NPC > 65535 {
			return false
		}
		for _, previous := range memory.Recent[:i] {
			if visit.Scene == previous.Scene && visit.NPC == previous.NPC {
				return false
			}
		}
	}
	return true
}

func fieldDistance(x, y, targetX, targetY int) int {
	dx, dy := x-targetX, y-targetY
	return max(dx, -dx) + 2*max(dy, -dy)
}

func rememberFieldConversation(memory *fieldBotMemory, previous, current View) {
	if memory == nil {
		return
	}
	rememberBotWine(memory, current)
	rememberBotNavigation(memory, current)
	if d := current.Dialogue; d != nil && strings.Contains(d.Text, "桌上") && strings.Contains(d.Text, "酒菜") && strings.Contains(d.Text, "端") {
		// This instruction is public dialogue. A newly requested item search
		// must not be suppressed by an earlier look at the same table.
		memory.SearchItems = true
	}
	if current.Field != nil {
		place := fieldBotPlace{current.Scene, current.Field.Anchor[0], current.Field.Anchor[1]}
		if memory.Last == nil || *memory.Last != place {
			memory.Walks = append(memory.Walks, place)
			if len(memory.Walks) > 256 {
				memory.Walks = memory.Walks[len(memory.Walks)-256:]
			}
		}
		if last := memory.Last; last != nil {
			distance := fieldDistance(last.X, last.Y, place.X, place.Y)
			if last.Scene != place.Scene || distance > 256 || previous.Phase == "script" && distance > 32 {
				rememberBotLink(memory, *last)
				rememberBotLink(memory, place)
			}
		}
		memory.Last = &place
	}
	if memory.Nearby != nil && memory.Nearby.Scene != current.Scene {
		memory.Nearby = nil
	}
	observed := previous
	if current.Field != nil {
		observed = current
	}
	if observed.Field != nil {
		memory.Nearby = nil
	}
	npc, distance := 0, 129
	if observed.Field != nil {
		for _, actor := range observed.Field.Actors {
			if actor[0] != 1 {
				continue
			}
			if d := fieldDistance(observed.X, observed.Y, actor[3], actor[4]); d < distance {
				npc, distance = actor[1], d
			}
		}
		if npc != 0 {
			memory.Nearby = &fieldBotVisit{observed.Scene, npc, observed.Field.Frame}
		}
	}
	// Original scripts may yield between the visible approach and dialogue.
	// Retain that public observation rather than inspecting the event script.
	if current.Dialogue == nil || memory.Nearby == nil || memory.Nearby.Scene != current.Scene {
		rememberBotWineTarget(memory, current)
		return
	}
	rememberBotVisit(memory, *memory.Nearby)
}

func rememberBotVisit(memory *fieldBotMemory, visit fieldBotVisit) {
	for i, previous := range memory.Recent {
		if previous.Scene == visit.Scene && previous.NPC == visit.NPC {
			memory.Recent = append(memory.Recent[:i], memory.Recent[i+1:]...)
			break
		}
	}
	memory.Recent = append(memory.Recent, visit)
	if len(memory.Recent) > 16 {
		memory.Recent = memory.Recent[len(memory.Recent)-16:]
	}
}

// An unsuccessful search is observable too. Remember it so static scenery
// does not capture the bot forever while searchable items remain eligible.
func rememberBotSearch(memory *fieldBotMemory, v View) {
	if memory == nil || v.Field == nil {
		return
	}
	npc, distance, item := 0, 289, false
	for _, actor := range v.Field.Actors {
		if actor[0] == 1 && (!recentlySpoken(memory, v.Scene, actor[1], v.Field.Frame) || memory.SearchItems && actor[6] == 0) {
			d := fieldDistance(v.X, v.Y, actor[3], actor[4])
			if d > 192 {
				continue
			}
			if actor[6] == 0 && !memory.SearchItems {
				d += 96
			}
			if d < distance {
				npc, distance, item = actor[1], d, actor[6] == 0
			}
		}
	}
	if npc != 0 {
		visit := fieldBotVisit{v.Scene, npc, v.Field.Frame}
		memory.Nearby = &visit
		rememberBotVisit(memory, visit)
		if item {
			memory.SearchItems = false
		}
	}
}

func rememberBotLink(memory *fieldBotMemory, place fieldBotPlace) {
	for i, previous := range memory.Links {
		if previous.Scene == place.Scene && fieldDistance(previous.X, previous.Y, place.X, place.Y) <= 64 {
			memory.Links = append(memory.Links[:i], memory.Links[i+1:]...)
			break
		}
	}
	memory.Links = append(memory.Links, place)
	if len(memory.Links) > 32 {
		memory.Links = memory.Links[len(memory.Links)-32:]
	}
}

func recentlySpoken(memory *fieldBotMemory, scene, npc int, frame uint32) bool {
	if memory != nil {
		for _, visit := range memory.Recent {
			if visit.Scene == scene && visit.NPC == npc && frame-visit.Frame < 128 {
				return true
			}
		}
	}
	return false
}

func fieldBotAction(v View, memory *fieldBotMemory, actor int) (Action, bool) {
	if actor != v.Leader || v.Seq != v.AppliedSeq {
		return Action{}, false
	}
	action := Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4}
	if v.Phase == "menu" {
		if keys, ok := fieldBotWineMenu(v, memory); ok {
			action.Keys = keys
			return action, true
		}
		action.Keys = 1 // Close an unfinished human menu without consuming items.
		return action, true
	}
	if v.Phase == "system" {
		return Action{Type: "system_close", Epoch: v.Epoch}, true
	}
	f := v.Field
	if v.Phase != "field" || f == nil || f.Radius != 8 || len(f.Cells) != 334 || f.Direction < 0 || f.Direction > 3 {
		return Action{}, false
	}
	if direction, keys, ok := fieldBotWineField(v, memory); ok {
		action.Direction, action.Keys = direction, keys
		return action, true
	}
	if memory != nil && memory.Wine != nil && memory.Wine.Wanted && memory.Wine.Available {
		if direction, ok := fieldBotKnownRoute(v, memory, memory.Wine.Anchor); ok {
			action.Direction = direction
			if direction < 4 {
				action.Keys = []uint32{4, 8, 16, 32}[direction]
			}
			return action, true
		}
	}
	npcDirection, npcKeys, distance := -1, uint32(0), 385
	searchItems := memory != nil && memory.SearchItems
	blockedTouch := [289]bool{}
	for i := range blockedTouch {
		blockedTouch[i] = f.Cells[i]&2 != 0
	}
	for _, npc := range f.Actors {
		if npc[0] != 1 || recentlySpoken(memory, v.Scene, npc[1], f.Frame) && !(searchItems && npc[6] == 0) {
			continue
		}
		d := fieldDistance(v.X, v.Y, npc[3], npc[4])
		score := d
		if npc[6] == 0 && !searchItems {
			score += 96
		}
		if score >= distance {
			continue
		}
		// Reach an actual search pose, not just a point close to a sprite.
		// A visible actor may stand across a wall or on a different floor.
		blocked := blockedTouch
		for index := range blocked {
			u, w := index%17-8, index/17-8
			x, y := f.Anchor[0]+(u-w)*16, f.Anchor[1]+(u+w)*8
			if fieldDistance(x, y, npc[3], npc[4]) <= 96 {
				blocked[index] = false
			}
		}
		step, keys := fieldBotApproach(f, npc[3], npc[4], blocked)
		if step >= 0 {
			npcDirection, npcKeys, distance = step, keys, score
		}
	}
	direction := -1
	knownBoundaries, boundaryRank := fieldBotKnownBoundaries(v.Scene, f, memory)
	returnDirection, returnScore := -1, int(^uint(0)>>1)
	questDirection, questScore := -1, int(^uint(0)>>1)
	questVisits := 0
	questRank := 0
	returning := memory != nil && memory.Wine != nil && memory.Wine.Wanted && memory.Wine.Available && memory.Wine.NPC != 0
	// Touch boundaries are public collision flags. Prefer an unvisited visible
	// boundary, and remember only transitions actually observed after walking.
	portalScore := int(^uint(0) >> 1)
	for w := -8; w <= 8; w++ {
		for u := -8; u <= 8; u++ {
			index := (w+8)*17 + u + 8
			if f.Cells[index] != 2 {
				continue
			}
			x, y := f.Anchor[0]+(u-w)*16, f.Anchor[1]+(u+w)*8
			score := fieldDistance(v.X, v.Y, x, y)
			if returning {
				visits := 0
				for _, walk := range memory.Walks {
					if walk.Scene == v.Scene && fieldDistance(walk.X, walk.Y, x, y) < 64 {
						visits++
					}
				}
				cost := visits*2048 + score + boundaryRank[index]*16
				if cost < questScore {
					if step := fieldBotRoute(f, x, y, [289]bool{}, true); step >= 0 {
						questDirection, questScore = step, cost
						questVisits = visits
						questRank = boundaryRank[index]
					}
				}
			}
			if knownBoundaries[index] {
				score += boundaryRank[index] * 1024
				if score < returnScore {
					if step := fieldBotRoute(f, x, y, [289]bool{}, true); step >= 0 {
						returnScore, returnDirection = score, step
					}
				}
				continue
			}
			if score < portalScore {
				if step := fieldBotRoute(f, x, y, knownBoundaries, true); step >= 0 {
					portalScore, direction = score, step
				}
			}
		}
	}
	// A discovered exit may be just outside the current collision window. Walk
	// toward its observed arrival point rather than scanning an empty room again.
	if memory != nil {
		for i, place := range memory.Links {
			distance := fieldDistance(v.X, v.Y, place.X, place.Y)
			if place.Scene != v.Scene || distance > 512 {
				continue
			}
			score := distance + (i+1)*1024 + 512
			if score < returnScore {
				if step := fieldBotRoute(f, place.X, place.Y, [289]bool{}, false); step >= 0 {
					returnScore, returnDirection = score, step
				}
			}
		}
	}
	// After a visible item reward, finish the recorded request before cycling
	// through unrelated conversations. Revisit observed exits when the recipient
	// is not currently reachable; original collision/transition rules still run.
	if returning {
		if questDirection >= 0 && (memory.Wine.Scene != v.Scene || questVisits == 0 && questRank == 0) {
			direction = questDirection
		} else {
			// On the recipient's map, step away from a used doorway and
			// discover the reachable floor before choosing that doorway again.
			var explored bool
			direction, explored = fieldBotExplore(v.Scene, f, memory)
			if explored && returnDirection >= 0 {
				direction = returnDirection
			}
		}
		action.Keys = 0
	} else if npcDirection >= 0 && !returning {
		direction, action.Keys = npcDirection, npcKeys
	}
	if direction < 0 {
		var explored bool
		direction, explored = fieldBotExplore(v.Scene, f, memory)
		if explored && returnDirection >= 0 {
			direction = returnDirection
		}
	}
	if direction >= 0 {
		u, w := fieldBotDeltas[direction][0], fieldBotDeltas[direction][1]
		if action.Keys&2 == 0 && f.Cells[(w+8)*17+u+8]&2 != 0 {
			x, y := f.Anchor[0]+(u-w)*16, f.Anchor[1]+(u+w)*8
			for _, npc := range f.Actors {
				// NPC autoscripts run only in the active scene. Let a recent
				// conversational partner finish approaching this doorway before
				// leaving; the ordinary recent-visit timeout bounds the wait.
				if npc[0] == 1 && npc[6] > 0 && recentlySpoken(memory, v.Scene, npc[1], f.Frame) && fieldDistance(x, y, npc[3], npc[4]) <= 96 {
					return action, true
				}
			}
		}
		action.Direction = direction
		action.Keys |= []uint32{4, 8, 16, 32}[direction]
	} else {
		action.Keys = 2
	}
	return action, true
}

var fieldBotDeltas = [4][2]int{{0, 1}, {-1, 0}, {0, -1}, {1, 0}}

// Original Search compares staggered map tiles after UpdateParty has attempted
// the requested step. Even a close actor can be outside the two nearest search
// tiles; align first rather than repeatedly confirming in the wrong direction.
func fieldBotSearchDirection(f *engine.FieldState, u, w, targetX, targetY, rangeEnd int) int {
	sameTile := func(x, y int) bool {
		return x/32 == targetX/32 && y/16 == targetY/16 && (x%32 != 0) == (targetX%32 != 0)
	}
	for direction, delta := range fieldBotDeltas {
		nu, nw := u+delta[0], w+delta[1]
		if nu < -8 || nu > 8 || nw < -8 || nw > 8 {
			continue
		}
		dx, dy := (delta[0]-delta[1])*16, (delta[0]+delta[1])*8
		x, y := f.Anchor[0]+(u-w)*16, f.Anchor[1]+(u+w)*8
		if f.Cells[(nw+8)*17+nu+8]&1 == 0 {
			x, y = x+dx, y+dy
		}
		if sameTile(x, y) {
			return direction
		}
		for i := 0; i < 4; i++ {
			if i*3+1 <= rangeEnd && sameTile(x+dx, y+dy) || i*3+2 <= rangeEnd && sameTile(x, y+dy*2) || i*3+3 <= rangeEnd && sameTile(x+dx*2, y) {
				return direction
			}
			x, y = x+dx, y+dy
		}
	}
	return -1
}

func fieldBotApproach(f *engine.FieldState, targetX, targetY int, blocked [289]bool) (int, uint32) {
	// Prefer poses valid even for the shortest search range. If scenery blocks
	// those poses, try the original longer range used across counters/tables.
	for _, rangeEnd := range []int{1, 12} {
		if direction, keys := fieldBotApproachRange(f, targetX, targetY, blocked, rangeEnd); direction >= 0 {
			return direction, keys
		}
	}
	return -1, 0
}

func fieldBotApproachRange(f *engine.FieldState, targetX, targetY int, blocked [289]bool, rangeEnd int) (int, uint32) {
	type node struct{ u, w, first int }
	queue := []node{{0, 0, -1}}
	seen := [289]bool{}
	seen[8*17+8] = true
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		if facing := fieldBotSearchDirection(f, p.u, p.w, targetX, targetY, rangeEnd); facing >= 0 {
			if p.first < 0 {
				return facing, 2
			}
			return p.first, 0
		}
		for direction, delta := range fieldBotDeltas {
			u, w := p.u+delta[0], p.w+delta[1]
			if u < -8 || u > 8 || w < -8 || w > 8 {
				continue
			}
			index := (w+8)*17 + u + 8
			if seen[index] || blocked[index] || f.Cells[index]&1 != 0 {
				continue
			}
			first := p.first
			if first < 0 {
				first = direction
			}
			seen[index] = true
			queue = append(queue, node{u, w, first})
		}
	}
	return -1, 0
}

// A door's touch area can span several cells, including obstructed cells. Mark
// its whole visible connected footprint when any part matches an observed
// transition; otherwise the far edge looks new and leads back through the door.
func fieldBotKnownBoundaries(scene int, f *engine.FieldState, memory *fieldBotMemory) ([289]bool, [289]int) {
	known, seen := [289]bool{}, [289]bool{}
	ranks := [289]int{}
	for start := range seen {
		if seen[start] || f.Cells[start]&2 == 0 {
			continue
		}
		queue := []int{start}
		seen[start] = true
		rank := 0
		for i := 0; i < len(queue); i++ {
			index := queue[i]
			u, w := index%17-8, index/17-8
			if memory != nil {
				for n, place := range memory.Links {
					if place.Scene == scene && fieldDistance(place.X, place.Y, f.Anchor[0]+(u-w)*16, f.Anchor[1]+(u+w)*8) <= 96 {
						rank = max(rank, n+1)
					}
				}
			}
			for _, delta := range fieldBotDeltas {
				nu, nw := u+delta[0], w+delta[1]
				if nu < -8 || nu > 8 || nw < -8 || nw > 8 {
					continue
				}
				next := (nw+8)*17 + nu + 8
				if !seen[next] && f.Cells[next]&2 != 0 {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		if rank > 0 {
			for _, index := range queue {
				known[index] = true
				ranks[index] = rank
			}
		}
	}
	return known, ranks
}

// Prefer the nearest least-visited reachable location. Do not walk through a
// touch boundary as a side effect of exploring an ordinary floor tile: those
// boundaries have their own explicit selection above.
func fieldBotExplore(scene int, f *engine.FieldState, memory *fieldBotMemory) (int, bool) {
	type node struct{ u, w, first, steps int }
	queue := []node{{0, 0, -1, 0}}
	seen := [289]bool{}
	seen[8*17+8] = true
	best, score := -1, int(^uint(0)>>1)
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		x, y := f.Anchor[0]+(p.u-p.w)*16, f.Anchor[1]+(p.u+p.w)*8
		if p.first >= 0 {
			visits := 0
			if memory != nil {
				for _, place := range memory.Walks {
					if place.Scene == scene && fieldDistance(place.X, place.Y, x, y) < 64 {
						visits++
					}
				}
			}
			if next := visits*1024 + p.steps; next < score {
				best, score = p.first, next
			}
		}
		for direction, delta := range fieldBotDeltas {
			u, w := p.u+delta[0], p.w+delta[1]
			if u < -8 || u > 8 || w < -8 || w > 8 {
				continue
			}
			index := (w+8)*17 + u + 8
			if seen[index] || f.Cells[index] != 0 {
				continue
			}
			first := p.first
			if first < 0 {
				first = direction
			}
			seen[index] = true
			queue = append(queue, node{u, w, first, p.steps + 1})
		}
	}
	return best, score >= 1024
}

// Search only the current bounded public collision grid. Trigger rules and
// actual movement remain with the original engine through ordinary input.
func fieldBotRoute(f *engine.FieldState, targetX, targetY int, blocked [289]bool, exact bool) int {
	type node struct{ u, w, first, steps int }
	queue := []node{{0, 0, -1, 0}}
	seen := [289]bool{}
	seen[8*17+8] = true
	best, score := -1, fieldDistance(f.Anchor[0], f.Anchor[1], targetX, targetY)*100
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		x, y := f.Anchor[0]+(p.u-p.w)*16, f.Anchor[1]+(p.u+p.w)*8
		if p.first >= 0 {
			if exact && x == targetX && y == targetY {
				return p.first
			}
			if next := fieldDistance(x, y, targetX, targetY)*100 + p.steps; next < score {
				best, score = p.first, next
			}
		}
		for direction, delta := range fieldBotDeltas {
			u, w := p.u+delta[0], p.w+delta[1]
			if u < -8 || u > 8 || w < -8 || w > 8 {
				continue
			}
			index := (w+8)*17 + u + 8
			if seen[index] || blocked[index] || f.Cells[index]&1 != 0 {
				continue
			}
			first := p.first
			if first < 0 {
				first = direction
			}
			seen[index] = true
			queue = append(queue, node{u, w, first, p.steps + 1})
		}
	}
	if exact {
		return -1
	}
	return best
}
