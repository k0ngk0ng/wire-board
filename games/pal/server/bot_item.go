package pal

import "strings"

// Only displayed dialogue, visible inventory glyphs and an observed actor ID
// enter this plan. Item IDs, native scripts and inventory memory are not read.
type fieldBotWine struct {
	Scene     int            `json:"scene,omitempty"`
	NPC       int            `json:"npc,omitempty"`
	Wanted    bool           `json:"wanted,omitempty"`
	Checked   bool           `json:"checked,omitempty"`
	Available bool           `json:"available,omitempty"`
	Menu      bool           `json:"menu,omitempty"`
	Moves     int            `json:"moves,omitempty"`
	Rewind    bool           `json:"rewind,omitempty"`
	Pending   uint32         `json:"pending,omitempty"`
	Settled   int            `json:"settled,omitempty"`
	Cursor    string         `json:"cursor,omitempty"`
	Anchor    *fieldBotPlace `json:"anchor,omitempty"`
}

func validFieldBotWine(w *fieldBotWine) bool {
	return w == nil || w.Scene >= 0 && w.Scene <= 65535 && w.NPC >= 0 && w.NPC <= 65535 && w.Moves >= 0 && w.Moves <= 512 && (w.Pending == 0 || w.Pending == 8 || w.Pending == 32) && w.Settled >= 0 && w.Settled <= 2 && len(w.Cursor) <= 256 && (!w.Wanted || w.Scene != 0) && (!w.Menu || w.Wanted) && (!w.Available || w.Checked) && (w.Anchor == nil || validBotPlace(*w.Anchor) && w.Anchor.Scene == w.Scene)
}

// Releasing physical input can cancel an accepted but unconsumed cursor step.
// Keep the observed task and actual menu selection, then rescan from that
// selection instead of treating the cancelled step as an inventory boundary.
func releaseBotItemInput(memory *fieldBotMemory) {
	if memory == nil || memory.Wine == nil || !memory.Wine.Menu {
		return
	}
	w := memory.Wine
	w.Moves, w.Pending, w.Settled, w.Cursor, w.Rewind = 0, 0, 0, "", true
}

func rememberBotWine(memory *fieldBotMemory, v View) {
	if memory == nil {
		return
	}
	if d := v.Dialogue; d != nil {
		if strings.Contains(d.Text, "醉道士") && (strings.Contains(d.Text, "酒") || strings.Contains(d.Text, "喝")) && (strings.Contains(d.Text, "口") || strings.Contains(d.Text, "给我")) {
			if memory.Wine == nil {
				memory.Wine = &fieldBotWine{}
			}
			w := memory.Wine
			w.Scene, w.Wanted = v.Scene, true
			w.Anchor = &fieldBotPlace{v.Scene, v.X, v.Y}
			if near := memory.Nearby; near != nil && near.Scene == v.Scene {
				w.NPC = near.NPC
			}
		}
		if strings.Contains(d.Text, "桂花酒") && strings.Contains(d.Text, "收在怀里") {
			if memory.Wine == nil {
				memory.Wine = &fieldBotWine{}
			}
			memory.Wine.Checked, memory.Wine.Available = true, true
		}
		if strings.Contains(d.Text, "怎么喝光了") && memory.Wine != nil {
			w := memory.Wine
			w.Wanted, w.Available, w.Menu, w.Checked, w.Moves = false, false, false, true, 0
			w.Pending, w.Settled, w.Cursor = 0, 0, ""
		}
	}
	if v.Phase == "menu" && v.Menu.Kind == 5 && strings.Contains(fieldBotVisibleText(v, true), "桂花酒") {
		if memory.Wine == nil {
			memory.Wine = &fieldBotWine{}
		}
		memory.Wine.Checked, memory.Wine.Available = true, true
	}
}

func rememberBotWineTarget(memory *fieldBotMemory, v View) {
	if memory == nil || memory.Wine == nil || !memory.Wine.Wanted || memory.Wine.NPC != 0 || memory.Wine.Scene != v.Scene || v.Field == nil {
		return
	}
	closest := 129
	for _, npc := range v.Field.Actors {
		if npc[0] != 1 {
			continue
		}
		if distance := fieldDistance(v.X, v.Y, npc[3], npc[4]); distance < closest {
			memory.Wine.NPC, closest = npc[1], distance
		}
	}
}

func rememberBotItemInput(memory *fieldBotMemory, v View, keys uint32) {
	rememberBotWine(memory, v)
	if memory == nil || memory.Wine == nil {
		return
	}
	w := memory.Wine
	if v.Phase == "field" && keys&2048 != 0 && w.Wanted && w.Scene == v.Scene {
		w.Menu, w.Moves = true, 0
		w.Rewind, w.Pending, w.Settled, w.Cursor = true, 0, 0, ""
	}
	if v.Phase == "menu" && w.Menu {
		if (keys&32 != 0 || keys&8 != 0) && w.Moves < 512 {
			w.Moves++
			w.Pending, w.Settled, w.Cursor = keys&(8|32), 0, fieldBotCursorName(v)
			if keys&32 != 0 {
				w.Rewind = false
			}
		} else if keys == 0 && w.Pending != 0 && w.Settled < 2 {
			w.Settled++
		}
		if keys&1 != 0 {
			if v.Menu.Kind == 5 && (fieldBotVisibleText(v, false) == "" || w.Moves >= 512 || w.Pending == 32 && w.Settled >= 2 && w.Cursor != "" && w.Cursor == fieldBotCursorName(v)) {
				w.Checked, w.Available = true, false
			}
			w.Menu = false
			w.Pending, w.Settled, w.Cursor = 0, 0, ""
		}
	}
}

func fieldBotWineMenu(v View, memory *fieldBotMemory) (uint32, bool) {
	if memory == nil || memory.Wine == nil {
		return 0, false
	}
	w := memory.Wine
	if !w.Wanted || !w.Menu || v.Menu.Kind != 5 {
		return 0, false
	}
	if fieldBotVisibleText(v, false) == "" {
		return 1, true
	}
	if strings.Contains(fieldBotVisibleText(v, true), "桂花酒") {
		return 2, true
	}
	if w.Moves >= 512 {
		return 1, true
	}
	if w.Pending != 0 {
		// Wait through the original redraw before deciding that a repeated
		// selected label means the cursor reached an end of the inventory.
		if w.Settled < 2 {
			return 0, true
		}
		if name := fieldBotCursorName(v); name != "" && name == w.Cursor {
			if w.Pending == 32 {
				return 1, true
			}
			return 32, true
		}
	}
	if w.Rewind {
		return 8, true
	}
	return 32, true
}

func fieldBotWineField(v View, memory *fieldBotMemory) (int, uint32, bool) {
	if memory == nil || memory.Wine == nil {
		return 0, 0, false
	}
	w := memory.Wine
	if !w.Wanted || w.Scene != v.Scene || w.NPC == 0 || w.Checked && !w.Available {
		return 0, 0, false
	}
	for _, npc := range v.Field.Actors {
		if npc[0] != 1 || npc[1] != w.NPC {
			continue
		}
		for direction, delta := range fieldBotDeltas {
			dx, dy := (delta[0]-delta[1])*16, (delta[0]+delta[1])*8
			if v.X+dx == npc[3] && v.Y+dy == npc[4] {
				if v.Field.Direction == direction {
					return 4, 2048, true
				}
				return direction, []uint32{4, 8, 16, 32}[direction], true
			}
		}
		blocked := [289]bool{}
		for index := range blocked {
			u, offset := index%17-8, index/17-8
			x, y := v.X+(u-offset)*16, v.Y+(u+offset)*8
			blocked[index] = v.Field.Cells[index]&2 != 0 && fieldDistance(x, y, npc[3], npc[4]) > 96
		}
		if direction, _ := fieldBotApproachRange(v.Field, npc[3], npc[4], blocked, 1); direction >= 0 {
			return direction, []uint32{4, 8, 16, 32}[direction], true
		}
	}
	return 0, 0, false
}

// Traverse only the displayed raster root. Retained menu backup surfaces can
// contain earlier selections and must not inform the bot's current choice.
func fieldBotVisibleText(v View, selected bool) string {
	return fieldBotText(v, selected, false)
}

func fieldBotCursorName(v View) string {
	return fieldBotText(v, true, true)
}

func fieldBotText(v View, selected, inactive bool) string {
	var text strings.Builder
	add := func(code, color int) {
		if color != 0 && (!selected || color >= 249 && color <= 254 || inactive && color == 28) {
			text.WriteRune(rune(code))
		}
	}
	if v.Raster == nil {
		for _, op := range v.Overlay {
			if op.Kind == 0 {
				add(op.ID, op.Color)
			}
		}
		return text.String()
	}
	seen := make([]bool, len(v.Raster.Nodes))
	type pending struct {
		id   int
		emit bool
	}
	stack := []pending{{id: v.Raster.Root}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		id := p.id
		if id <= 0 || id > len(seen) {
			continue
		}
		n := v.Raster.Nodes[id-1]
		if p.emit {
			if n[0] == 2 {
				add(n[5], n[8])
			}
			continue
		}
		if seen[id-1] {
			continue
		}
		seen[id-1] = true
		// Earlier surfaces are visited before the glyph drawn over them.
		stack = append(stack, pending{id: id, emit: true}, pending{id: n[2]}, pending{id: n[1]})
	}
	return text.String()
}
