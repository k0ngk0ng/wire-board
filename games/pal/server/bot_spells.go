package pal

import "strings"

type battleSpellRow struct {
	name       string
	x, y       int
	selected   bool
	affordable bool
}

func battleVisibleSpells(v View) []battleSpellRow {
	type point struct{ x, y int }
	type glyph struct{ code, color int }
	letters := map[point]glyph{}
	add := func(code, x, y, color int) {
		if color != 0 {
			letters[point{x, y}] = glyph{code, color}
		}
	}
	if v.Raster == nil {
		for _, op := range v.Overlay {
			if op.Kind == 0 {
				add(op.ID, op.X, op.Y, op.Color)
			}
		}
	} else {
		seen := make([]bool, len(v.Raster.Nodes))
		type pending struct {
			id   int
			emit bool
		}
		stack := []pending{{id: v.Raster.Root}}
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if p.id <= 0 || p.id > len(seen) {
				continue
			}
			n := v.Raster.Nodes[p.id-1]
			if p.emit {
				if n[0] == 2 {
					add(n[5], n[6], n[7], n[8])
				}
				continue
			}
			if seen[p.id-1] {
				continue
			}
			seen[p.id-1] = true
			stack = append(stack, pending{id: p.id, emit: true}, pending{id: n[2]}, pending{id: n[1]})
		}
	}
	rows := []battleSpellRow{}
	for y := 54; y <= 172; y += 18 {
		for col := 0; col < 3; col++ {
			x := 35 + col*87
			first, ok := letters[point{x, y}]
			if !ok {
				continue
			}
			var name strings.Builder
			for at := x; at < x+82; {
				g, ok := letters[point{at, y}]
				if !ok {
					break
				}
				name.WriteRune(rune(g.code))
				if g.code > 255 {
					at += 16
				} else {
					at += 8
				}
			}
			if name.Len() > 0 {
				rows = append(rows, battleSpellRow{name: name.String(), x: x, y: y,
					selected:   first.color >= 249 && first.color <= 254 || first.color == 28,
					affordable: first.color != 24 && first.color != 28})
			}
		}
	}
	return rows
}

func battleSpellRank(name string, healing bool, wounded int) int {
	if healing {
		if name == "五气朝元" {
			if wounded > 1 {
				return 50
			}
			return 15
		}
		return map[string]int{"气疗术": 10, "观音咒": 20, "凝神归元": 30, "元灵归心术": 40}[name]
	}
	for rank, names := range [][]string{
		{"御剑术", "气剑指", "风咒", "旋风咒", "雷咒", "五雷咒", "冰咒", "玄冰咒", "炎咒", "三昧真火", "土咒", "飞岩术"},
		{"万剑诀", "一阳指"},
		{"天剑", "七诀剑气", "剑气斩", "天雷破", "风卷残云", "风雪冰天", "炎杀咒", "地裂天崩"},
		{"剑神", "武神", "雷神", "雪妖", "火神", "山神", "风神", "炼狱真火", "万蛊蚀天", "万蚁蚀象", "斩龙诀", "狂雷"},
	} {
		for _, candidate := range names {
			if name == candidate {
				return (rank + 1) * 10
			}
		}
	}
	return 0
}

func observedSpellKeys(v View, healing bool) (uint32, bool) {
	rows := battleVisibleSpells(v)
	selected, target, rank, wounded := -1, -1, 0, 0
	for _, p := range v.Party {
		if recoveryNeeded(p) {
			wounded++
		}
	}
	for i, row := range rows {
		if row.selected {
			selected = i
		}
		if !row.affordable {
			continue
		}
		score := battleSpellRank(row.name, healing, wounded)
		if score > rank || score > 0 && score == rank && row.selected {
			target, rank = i, score
		}
	}
	if selected < 0 || target < 0 {
		return 0, false
	}
	if selected == target {
		return 2, true
	}
	if target < selected {
		return 8, true
	}
	return 32, true
}
