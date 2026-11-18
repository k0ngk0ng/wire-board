package pal_test

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type campaignMapWalker struct {
	maps              map[int][]byte
	known             map[[3]int]byte
	lotus             map[[2]int][2]int
	sameSceneMoves    map[int]map[[2]int][2]int
	allowTouch        bool
	rememberCollision bool
	staticOnly        bool
	avoidTouches      map[int][][3]int
}

func (w *campaignMapWalker) walk(t *testing.T, m gameapi.Module, c *gameapi.Context, g gameapi.Match, v pal.View, targetX, targetY, reach int) gameapi.Match {
	t.Helper()

	v = view(t, m, g, v.Leader)
	if v.Field == nil && !w.staticOnly {
		t.Fatal("walkthrough requires public movement state or an explicit static-map route", v.Scene)
	}
	if w.maps == nil {
		w.maps = map[int][]byte{}
		w.known = map[[3]int]byte{}
	}
	if v.Scene == 18 && v.Field != nil {

		if w.lotus == nil {
			w.lotus = map[[2]int][2]int{}
			for _, r := range mazeLotusRoutes {
				w.lotus[[2]int{r[1], r[2]}] = [2]int{r[5], r[6]}
			}
		}
		for _, actor := range v.Field.Actors {
			for _, r := range mazeLotusRoutes {
				if actor[0] != 1 || actor[1] != r[0] {
					continue
				}
				delete(w.lotus, [2]int{r[1], r[2]})
				delete(w.lotus, [2]int{r[3], r[4]})
				if actor[3] == r[1] && actor[4] == r[2] {
					w.lotus[[2]int{r[1], r[2]}] = [2]int{r[5], r[6]}
				} else if actor[3] == r[3] && actor[4] == r[4] {
					w.lotus[[2]int{r[3], r[4]}] = [2]int{r[7], r[8]}
				}
			}
			if actor[0] == 1 && actor[1] == 260 && actor[3] == 1104 && actor[4] == 472 {
				w.lotus[[2]int{1104, 472}] = [2]int{1296, 568}
			}
		}
	}
	if w.maps[v.Map] == nil {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", ".local/pal/visual/pal/v1/maps", fmt.Sprint(v.Map)+".bin.gz"))
		if err != nil {
			t.Fatal(err)
		}
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		data, err = io.ReadAll(io.LimitReader(reader, 65537))
		reader.Close()
		if err != nil || len(data) != 65536 {
			t.Fatal("invalid original walkthrough map", v.Map, err)
		}
		w.maps[v.Map] = data
	}

	if !w.rememberCollision || w.known == nil {
		w.known = map[[3]int]byte{}
	}
	offset := [2]int{v.X - v.CameraX, v.Y - v.CameraY}
	if v.Field != nil {
		offset = v.Field.Offset
		for i, flags := range v.Field.Cells[:289] {
			u, z := i%17-8, i/17-8
			w.known[[3]int{v.Scene, v.X + (u-z)*16, v.Y + (u+z)*8}] = flags
		}
	}
	index := func(x, y int) int {

		if x < offset[0]/32*32 || x/32 >= 63-offset[0]/32 || y < offset[1]/16*16 || y >= 2048 || x%16 != 0 || y%8 != 0 || (x/16-y/8)%2 != 0 {
			return -1
		}
		return (y/16*64+x/32)*2 + x%32/16
	}
	type node struct{ x, y, first, steps int }
	queue := []node{{v.X, v.Y, -1, 0}}
	seen := [16384]bool{}
	if at := index(v.X, v.Y); at < 0 {
		t.Fatal("walkthrough left original lattice", v.Scene, v.X, v.Y)
	} else {
		seen[at] = true
	}
	direction, score := -1, int(^uint(0)>>1)
	reserved := map[[2]int]bool{}
	if v.Scene == 18 {
		for _, r := range mazeLotusRoutes {
			reserved[[2]int{r[1], r[2]}] = true
			reserved[[2]int{r[3], r[4]}] = true
		}
		reserved[[2]int{1104, 472}] = true
	}
	for at := 0; at < len(queue); at++ {
		p := queue[at]
		if destination, ok := w.sameSceneMoves[v.Scene][[2]int{p.x, p.y}]; ok {
			if p.first < 0 {
				direction = 4
				break
			}
			if i := index(destination[0], destination[1]); i >= 0 && !seen[i] {
				seen[i] = true
				queue = append(queue, node{destination[0], destination[1], p.first, p.steps + 1})
			}
			continue
		}
		distance := fieldTestDistance(p.x, p.y, targetX, targetY)
		if p.first >= 0 {
			if next := distance*100 + p.steps; next < score {
				direction, score = p.first, next
			}
			if distance <= reach {
				direction = p.first
				break
			}
		}
		if destination, ok := w.lotus[[2]int{p.x, p.y}]; ok && v.Scene == 18 {
			if p.first < 0 {
				direction = 4
				break
			}
			if i := index(destination[0], destination[1]); i >= 0 && !seen[i] {
				seen[i] = true
				queue = append(queue, node{destination[0], destination[1], p.first, p.steps + 1})
			}
			continue
		}
		for d, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
			x, y := p.x+delta[0], p.y+delta[1]
			reservedTouch := false
			for _, footprint := range w.avoidTouches[v.Scene] {
				if fieldTestDistance(x, y, footprint[0], footprint[1]) < footprint[2] {
					reservedTouch = true
					break
				}
			}
			if reservedTouch {
				continue
			}
			i := index(x, y)
			if i < 0 || seen[i] {
				continue
			}
			flags, known := w.known[[3]int{v.Scene, x, y}]
			_, lotus := w.lotus[[2]int{x, y}]

			if reserved[[2]int{x, y}] && !lotus {
				continue
			}
			blockedTile := binary.LittleEndian.Uint32(w.maps[v.Map][i*4:])&8192 != 0
			encounter := w.rememberCollision && w.allowTouch && flags&2 != 0 && !blockedTile
			if known && flags&1 != 0 && !encounter || !known && blockedTile || flags&2 != 0 && !w.allowTouch && !lotus && fieldTestDistance(x, y, targetX, targetY) > 96 {
				continue
			}
			seen[i] = true
			first := p.first
			if first < 0 {
				first = d
			}
			queue = append(queue, node{x, y, first, p.steps + 1})
		}
	}
	if direction < 0 {
		t.Fatal("original walkthrough map has no route", v.Scene, v.X, v.Y, targetX, targetY)
	}
	if os.Getenv("PAL_TOWER_WALK_TRACE") == "1" {
		t.Log("tower route planning", v.Scene, v.X, v.Y, targetX, targetY, reach, direction, score, len(queue))
	}
	if os.Getenv("PAL_SACRED_TREE_PLAN_ONLY") == "1" {
		known, moves := [][3]int{}, [][4]int{}
		for point, flags := range w.known {
			if point[0] == v.Scene {
				known = append(known, [3]int{point[1], point[2], int(flags)})
			}
		}
		for entry, destination := range w.sameSceneMoves[v.Scene] {
			moves = append(moves, [4]int{entry[0], entry[1], destination[0], destination[1]})
		}
		dump := raw(map[string]any{"diagnosticOnly": true, "scene": v.Scene, "map": v.Map, "position": [2]int{v.X, v.Y}, "offset": offset, "target": [2]int{targetX, targetY}, "known": known, "moves": moves, "avoid": w.avoidTouches[v.Scene], "direction": direction, "score": score, "nodes": len(queue)})
		if err := os.WriteFile(campaignEvidencePath("sacred-tree-walk-plan.json", 1), dump, 0600); err != nil {
			t.Fatal(err)
		}
	}
	c.Actor = v.Leader
	keys := uint32(0)
	if direction < 4 {
		keys = []uint32{4, 8, 16, 32}[direction]
	}
	g = apply(t, m, *c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
	return apply(t, m, *c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
}

var mazeLotusRoutes = [][9]int{
	{244, 944, 888, 864, 848, 848, 840, 960, 896},
	{245, 864, 832, 704, 544, 688, 536, 848, 840},
	{246, 1088, 960, 1552, 824, 1568, 816, 1072, 952},
	{247, 1536, 784, 1296, 664, 1280, 656, 1552, 792},
	{248, 1632, 800, 1792, 720, 1808, 712, 1616, 808},
	{249, 1120, 880, 1248, 816, 1264, 808, 1104, 888},
	{250, 672, 544, 512, 720, 528, 728, 688, 536},
	{251, 1296, 824, 1456, 1144, 1472, 1152, 1280, 816},
	{252, 1440, 1056, 1056, 1248, 1040, 1256, 1456, 1048},
	{253, 1424, 1000, 1232, 904, 1216, 896, 1440, 1008},
	{254, 1184, 880, 1104, 840, 1088, 832, 1200, 888},
	{255, 1248, 880, 928, 416, 912, 408, 1232, 888},
	{256, 1232, 776, 1344, 576, 1328, 568, 1216, 784},
	{257, 1344, 560, 1408, 528, 1424, 520, 1328, 568},
	{258, 1088, 736, 560, 552, 544, 544, 1104, 728},
	{259, 1136, 600, 848, 456, 832, 448, 1152, 608},
	{260, 1296, 584, 1280, 560, 1296, 568, 1088, 464},
}
