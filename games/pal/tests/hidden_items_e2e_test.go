package pal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type hiddenInnSpot struct {
	Scene, Event, X, Y int
	Item, Notice       string
	Cash               uint32
}

func hiddenNoticeText(v pal.View) string {
	var text strings.Builder
	if v.Raster == nil {
		for _, op := range v.Overlay {
			if op.Kind == 0 {
				text.WriteRune(rune(op.ID))
			}
		}
		return text.String()
	}
	seen := map[int]bool{}
	var visit func(int)
	visit = func(id int) {
		if id <= 0 || id > len(v.Raster.Nodes) || seen[id] {
			return
		}
		seen[id] = true
		n := v.Raster.Nodes[id-1]
		visit(n[1])
		visit(n[2])
		if n[0] == 2 {
			text.WriteRune(rune(n[5]))
		}
	}
	visit(v.Raster.Root)
	return text.String()
}

func TestOriginalYuhangHiddenItemsSearchSaveAndNoDuplication(t *testing.T) {
	m, _ := nativeModule(t)
	c := gameapi.Context{Players: 1, Now: time.Unix(1800000000, 0), Automated: []bool{false}}
	g := originalInn(t, m, &c)
	saves, readings := 0, 0
	lastPrompt := uint64(0)
	seen := map[int]bool{}
	savedNotice := map[int]bool{}
	items := map[string]int{}
	var target *hiddenInnSpot
	visible := func() pal.View { return view(t, m, g, 0) }
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"hidden-inn-failure-state.json": g.Data, "hidden-inn-failure-view.json": raw(visible())} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	keys := func(direction int, buttons uint32) {
		v := visible()
		g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: direction, Keys: buttons})
	}
	save := func(name string) {
		t.Helper()
		before := visible()
		var err error
		g, err = m.SaveControl(c, g, true)
		if err != nil {
			t.Fatal(err)
		}
		c.Now = c.Now.Add(24 * time.Hour)
		m, _ = nativeModule(t)
		g, err = m.ResumeSaved(c, g)
		if err != nil {
			t.Fatal(err)
		}
		normalize := func(v pal.View) pal.View {
			v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
			v.Local = nil
			return v
		}
		if before.Local == nil || visible().Local == nil || before.Local.Core != visible().Local.Core || before.Local.Generation == visible().Local.Generation {
			t.Fatal("hidden item save did not restore the same core with a fresh local control generation", name)
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(visible()))) {
			t.Fatal("hidden item save changed native continuation", name)
		}
		if _, err = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("hidden item save accepted old search", name)
		}
		saves++
		if err = os.WriteFile(campaignEvidencePath("hidden-inn-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	progress := func() {
		t.Helper()
		v := visible()
		if target != nil && (strings.Contains(hiddenNoticeText(v), target.Notice) || v.Dialogue != nil && strings.Contains(v.Dialogue.Text, target.Notice)) {
			seen[target.Event] = true
			if !savedNotice[target.Event] {
				save(fmt.Sprint("event-", target.Event, "-notice"))
				savedNotice[target.Event] = true
				v = visible()
			}
		}
		if v.Dialogue != nil {
			if lastPrompt != v.Dialogue.ID {
				lastPrompt = v.Dialogue.ID
				readings++
				t.Log("hidden item reading", v.Dialogue.Text)
			}
			g = apply(t, m, c, g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
		}
		c.Now = time.UnixMilli(g.SystemDeadline + 200)
		var err error
		g, err = m.OnTimer(c, g)
		if err != nil {
			t.Fatal(err)
		}
	}
	field := func() pal.View {
		t.Helper()
		for i := 0; i < 512; i++ {
			v := visible()
			if v.Phase == "field" && v.Seq == v.AppliedSeq {
				return v
			}
			progress()
		}
		t.Fatal("hidden item did not return to original field", visible().Phase)
		return pal.View{}
	}
	walker := campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{4: {{1472, 1520, 48}, {1616, 1480, 48}, {976, 1704, 80}}}}
	walk := func(scene, x, y int) {
		t.Helper()
		visits := map[[2]int]int{}
		for i := 0; i < 256; i++ {
			v := field()
			if v.Scene != scene || v.X == x && v.Y == y {
				return
			}
			point := [2]int{v.X, v.Y}
			visits[point]++
			if visits[point] > 16 {
				t.Fatal("hidden search stance blocked", scene, x, y, v.X, v.Y)
			}
			g = walker.walk(t, m, &c, g, v, x, y, 0)
			progress()
		}
		t.Fatal("hidden search walk exhausted", scene, x, y)
	}
	inventory := func() {
		t.Helper()
		field()
		keys(4, 2048)
		for i := 0; i < 128; i++ {
			v := visible()
			if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
				break
			}
			progress()
		}
		v := visible()
		if v.Menu.Kind != 5 {
			t.Fatal("hidden rewards not available in original inventory")
		}
		keys(4, 65536)
		progress()
		progress()
		v = visible()
		for name, amount := range items {
			n, found := campaignInventoryAmount(v, name)
			if !found || n != amount {
				t.Fatal("hidden item inventory missing/duplicated", name, n, found, amount)
			}
		}
		keys(4, 1)
		field()
	}
	initial := field()
	if initial.Scene != 2 || len(initial.Party) != 1 || initial.Party[0].Role != 0 || initial.Cash != 0 {
		t.Fatal("hidden inn test is not a fresh original game", initial.Scene, initial.Cash, initial.Party)
	}
	spots := []hiddenInnSpot{
		{2, 5, 1248, 272, "止血草", "得到止血草", 0}, {2, 6, 1264, 392, "皮帽", "得到皮帽", 0},
		{2, 7, 1456, 312, "木鞋", "得到木鞋", 0}, {2, 32, 1312, 368, "净衣符", "发现净衣符", 0},
		{3, 34, 1328, 632, "驱魔香", "得到驱魔香", 0}, {3, 43, 1184, 704, "还神丹", "发现还神丹", 0},
		{3, 44, 1456, 696, "忘魂花", "发现忘魂花", 0}, {3, 42, 1456, 712, "", "５０文钱", 50},
	}
	claimed := []map[string]any{}
	for index := range spots {
		q := spots[index]
		target = &q
		if q.Scene == 3 && field().Scene == 2 {
			target = nil
			walk(2, 1152, 384)
			v := field()
			if v.Scene != 4 {
				t.Fatal("original hidden-room exit failed", v.Scene)
			}
			walk(4, 1296, 1368)
			for step := 0; ; step++ {
				v = field()
				if v.X >= 1200 && v.Y >= 1456 {
					break
				}
				if step >= 128 {
					t.Fatal("original inn stairs did not descend", v.X, v.Y, v.Field.Layer)
				}
				g = walker.walk(t, m, &c, g, v, 1168, 1368, 0)
				progress()
			}
			walker.avoidTouches[4] = append(walker.avoidTouches[4], [3]int{1248, 1456, 16})
			walk(4, 1328, 1448)
			if field().Scene != 3 {
				t.Fatal("original guest-room entrance failed", field().Scene)
			}
			target = &q
		}
		v := field()
		if v.Scene != q.Scene {
			t.Fatal("wrong hidden-item original scene", v.Scene, q)
		}
		for step := 0; fieldTestDistance(v.X, v.Y, q.X, q.Y) > 96; step++ {
			if step >= 128 {
				t.Fatal("hidden item approach exhausted", q)
			}
			g = walker.walk(t, m, &c, g, v, q.X, q.Y, 96)
			progress()
			v = field()
			if v.Scene != q.Scene {
				t.Fatal("hidden approach left original room", q, v.Scene)
			}
		}
		for _, actor := range v.Field.Actors {
			if actor[0] == 1 && actor[1] == q.Event {
				t.Fatal("hidden search spot became a public actor", q.Event)
			}
		}
		stance, direction, best := [2]int{}, -1, int(^uint(0)>>1)
		for d, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
			x, y := q.X-delta[0], q.Y-delta[1]
			du, dv := x-v.X, y-v.Y
			if (du/16+dv/8)%2 != 0 {
				continue
			}
			u, z := (du/16+dv/8)/2, (dv/8-du/16)/2
			if u < -8 || u > 8 || z < -8 || z > 8 || v.Field.Cells[(z+8)*17+u+8]&3 != 0 {
				continue
			}
			if score := fieldTestDistance(v.X, v.Y, x, y); score < best {
				stance, direction, best = [2]int{x, y}, d, score
			}
		}
		if direction < 0 {
			t.Fatal("no public hidden search stance", q, v.X, v.Y)
		}
		walk(q.Scene, stance[0], stance[1])
		keys(direction, []uint32{4, 8, 16, 32}[direction])
		keys(4, 0)
		progress()
		v = field()
		beforeCash := v.Cash
		save(fmt.Sprint("event-", q.Event, "-unpicked"))
		keys(4, 2)
		progress()
		field()
		if !seen[q.Event] {
			t.Fatal("original hidden search notification absent", q, hiddenNoticeText(visible()))
		}
		if q.Item != "" {
			items[q.Item]++
			inventory()
		}
		if visible().Cash != beforeCash+q.Cash {
			t.Fatal("hidden cash reward changed", q, visible().Cash, beforeCash)
		}
		save(fmt.Sprint("event-", q.Event, "-picked"))
		for retry := 0; retry < 3; retry++ {
			keys(4, 2)
			progress()
			field()
		}
		if q.Item != "" {
			inventory()
		}
		if visible().Cash != beforeCash+q.Cash {
			t.Fatal("hidden cash can be farmed after resume", q)
		}
		if !bytes.Equal(raw(initial.Party), raw(visible().Party)) {
			t.Fatal("searching hidden items changed original party", visible().Party)
		}
		claimed = append(claimed, map[string]any{"scene": q.Scene, "event": q.Event, "position": [2]int{q.X, q.Y}, "item": q.Item, "cash": q.Cash, "notification": true, "repeatAfterSaveRejected": true})
	}
	save("complete")
	if visible().Cash != 550 || readings != 10 {
		t.Fatal("original inn story or hidden cash changed", visible().Cash, readings)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "fresh original game, ordinary movement/facing/search for map-baked invisible Yuhang inn items, original inventory/cash, exact fresh-runtime continuation, no duplicate rewards after resume, no invented public loot actor/marker; original forced inn guest dialogue/reward preserved while walking between rooms; partial hidden-item coverage, not whole-game completion", "developmentProfile": developmentHealthProfile(), "sites": claimed, "items": items, "cash": visible().Cash, "hiddenCash": 50, "originalInnGuestRewardCash": 500, "saves": saves, "readings": readings}, "", "  ")
	if err := os.WriteFile(campaignEvidencePath("hidden-inn-rewards.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}
