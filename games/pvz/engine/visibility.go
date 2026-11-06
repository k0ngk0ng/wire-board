package engine

import (
	"slices"
	"strings"
)

func VisibilityFor(s State, viewer int) Visibility {
	cells := make([][]bool, s.Map.Rows)
	all := !s.Map.Fog || s.FogClearedUntil > s.Tick || viewer >= 0 && viewer < len(s.Players) && s.Players[viewer].Side == "zombies"
	for row := range cells {
		cells[row] = make([]bool, Columns)
		for col := range cells[row] {
			cells[row][col] = all || col < 4
		}
	}
	if !all {
		for _, p := range s.Plants {
			if p.Kind != "plantern" || p.HP <= 0 {
				continue
			}
			for row := max(0, p.Row-1); row <= min(s.Map.Rows-1, p.Row+1); row++ {
				for col := max(0, p.Col-1); col <= min(Columns-1, p.Col+1); col++ {
					cells[row][col] = true
				}
			}
		}
	}
	return Visibility{Cells: cells}
}
func ProjectState(s State, viewer int) State {
	out := Clone(s)
	if !s.Map.Fog {
		return out
	}
	out.Projection = true
	mask := VisibilityFor(s, viewer)
	visible := func(row, x int) bool {
		col := max(0, min(Columns-1, x/1000))
		return row >= 0 && row < len(mask.Cells) && mask.Cells[row][col]
	}
	own := func(owner int) bool {
		return viewer >= 0 && viewer < len(s.Players) && owner >= 0 && owner < len(s.Players) && s.Players[viewer].Team == s.Players[owner].Team
	}
	allZombies := out.Zombies
	out.Zombies = []Zombie{}
	visibleTargets := map[int]bool{}
	for _, z := range allZombies {
		if own(z.Owner) || z.Allegiance == "plants" || visible(z.Row, z.X) {
			out.Zombies = append(out.Zombies, z)
			visibleTargets[z.ID] = true
		}
	}
	for _, p := range out.Plants {
		visibleTargets[p.ID] = true
	}
	for i := range out.Zombies {
		z := &out.Zombies[i]
		if z.TargetID > 0 && !visibleTargets[z.TargetID] {
			z.TargetID = 0
			for k := range z.Flags {
				if strings.Contains(strings.ToLower(k), "target") {
					delete(z.Flags, k)
				}
			}
		}
	}
	// A friendly unit's automatic targeting must not become a hidden enemy sidecar.
	for i := range out.Plants {
		p := &out.Plants[i]
		if !visibleTargets[p.TargetID] {
			p.TargetID = 0
			if p.Kind != "cobcannon" {
				for k := range p.Flags {
					if strings.Contains(strings.ToLower(k), "target") {
						delete(p.Flags, k)
					}
				}
			}
		}
	}
	out.Projectiles = []Projectile{}
	for _, q := range s.Projectiles {
		if !visible(q.Row, q.X) && !(own(q.Owner) && q.Kind == "cob") {
			continue
		}
		if q.TargetID > 0 && !visibleTargets[q.TargetID] {
			q.TargetID = 0
			q.TargetX = 0
			q.TargetY = 0
			q.LandAt = 0
		}
		hits := []int{}
		for _, id := range q.Hits {
			if visibleTargets[id] {
				hits = append(hits, id)
			}
		}
		q.Hits = hits
		out.Projectiles = append(out.Projectiles, q)
	}
	out.Events = []Event{}
	for _, e := range s.Events {
		global := slices.Contains([]string{"round", "battle", "round-end", "finished"}, e.Type)
		ownSource := own(e.Owner) && slices.Contains([]string{"plant", "shovel", "coffee", "wake", "transform", "shoot", "sun", "coin", "collect", "charge", "ability"}, e.Type)
		ownBlast := own(e.Owner) && e.Type == "explosion" && slices.Contains([]string{"cherrybomb", "potatomine", "jalapeno", "doomshroom", "cob"}, e.Kind)
		if global || visible(e.Row, e.X) || ownSource || ownBlast {
			if e.Target > 0 && !visibleTargets[e.Target] {
				plantTarget := false
				for _, p := range out.Plants {
					if p.ID == e.Target {
						plantTarget = true
						break
					}
				}
				if !plantTarget {
					e.Target = 0
				}
			}
			out.Events = append(out.Events, e)
		}
	}
	out.Terrain = []Terrain{}
	for _, v := range s.Terrain {
		if own(v.Owner) || visible(v.Row, v.FromX) || visible(v.Row, v.ToX) {
			out.Terrain = append(out.Terrain, v)
		}
	}
	return out
}
