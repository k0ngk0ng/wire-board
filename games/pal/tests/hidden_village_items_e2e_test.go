package pal_test

// E2E first: original outdoor herbs become unsearchable or fail to disappear;
// map-baked rewards gain invented markers; several spots sharing one script
// incorrectly share claimed state; inventory quantities or timed notices change;
// acquisition-time saves replay or lose rewards; old searches cross a resume.
// Coordinates and event IDs are walkthrough evidence, never production bot data.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func TestOriginalYuhangVillageHiddenItemsAndHerbsSave(t *testing.T) {
	if developmentHealthProfile() != "" {
		t.Fatal("hidden village acceptance requires the original health profile")
	}
	m, c, g := originalVillage(t, 1)
	visible := func() pal.View { return view(t, m, g, 0) }
	initial := visible()
	if initial.Scene != 5 || initial.Cash != 550 || len(initial.Party) != 1 || initial.Party[0].HP != 150 {
		t.Fatal("not an original-HP village checkpoint", initial.Scene, initial.Cash, initial.Party)
	}
	defer func() {
		if t.Failed() {
			for name, data := range map[string][]byte{"hidden-village-failure-state.json": g.Data, "hidden-village-failure-view.json": raw(visible())} {
				if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	saves, notices := 0, map[int]bool{}
	readings, lastPrompt := []string{}, uint64(0)
	var target *hiddenInnSpot
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
			return v
		}
		if !bytes.Equal(raw(normalize(before)), raw(normalize(visible()))) {
			t.Fatal("outdoor hidden item save changed native continuation", name)
		}
		if _, err = m.Apply(c, g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
			t.Fatal("outdoor hidden item save accepted an old search", name)
		}
		saves++
		if err = os.WriteFile(campaignEvidencePath("hidden-village-"+name+"-state.json", 1), g.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	keys := func(direction int, buttons uint32) {
		t.Helper()
		v := visible()
		g = apply(t, m, c, g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: direction, Keys: buttons})
	}
	progress := func() {
		t.Helper()
		v := visible()
		if target != nil && (strings.Contains(hiddenNoticeText(v), target.Notice) || v.Dialogue != nil && strings.Contains(v.Dialogue.Text, target.Notice)) && !notices[target.Event] {
			notices[target.Event] = true
			save(fmt.Sprint("event-", target.Event, "-notice"))
			v = visible()
		}
		if v.Dialogue != nil {
			// Once the item is gone, ordinary search can reach a nearby NPC.
			// Preserve that original conversation rather than masking the NPC.
			if lastPrompt != v.Dialogue.ID {
				lastPrompt = v.Dialogue.ID
				readings = append(readings, v.Dialogue.Text)
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
		for i := 0; i < 128; i++ {
			v := visible()
			if v.Phase == "field" && v.Seq == v.AppliedSeq {
				return v
			}
			progress()
		}
		t.Fatal("outdoor search failed to return to the original field", visible().Phase)
		return pal.View{}
	}
	w := campaignMapWalker{avoidTouches: map[int][][3]int{}}
	walk := func(scene, x, y, reach int) {
		t.Helper()
		visits := map[[2]int]int{}
		for i := 0; i < 512; i++ {
			v := field()
			if v.Scene != scene || fieldTestDistance(v.X, v.Y, x, y) <= reach {
				return
			}
			p := [2]int{v.X, v.Y}
			visits[p]++
			if visits[p] > 12 {
				t.Fatal("outdoor hidden-item path blocked", scene, x, y, p)
			}
			g = w.walk(t, m, &c, g, v, x, y, reach)
			progress()
		}
		t.Fatal("outdoor hidden-item walk exhausted", scene, x, y)
	}
	items := map[string]int{}
	inventory := func() {
		t.Helper()
		field()
		keys(4, 2048)
		for i := 0; i < 128 && (visible().Menu.Kind != 5 || visible().Seq != visible().AppliedSeq); i++ {
			progress()
		}
		if visible().Menu.Kind != 5 {
			t.Fatal("outdoor rewards unavailable in original inventory")
		}
		keys(4, 65536)
		progress()
		progress()
		found := map[string]bool{}
		for scan := 0; scan < 80; scan++ {
			v := visible()
			for name, want := range items {
				if amount, ok := campaignInventoryAmount(v, name); ok {
					if amount != want {
						t.Fatal("outdoor hidden item missing or duplicated", name, amount, want)
					}
					found[name] = true
				}
			}
			if len(found) == len(items) {
				keys(4, 1)
				field()
				return
			}
			keys(4, 32)
			progress()
			progress()
		}
		t.Fatal("outdoor hidden-item labels missing from original inventory", items, found)
	}
	spots := []struct {
		hiddenInnSpot
		Sprite int
	}{
		{hiddenInnSpot{5, 110, 1168, 1624, "水果", "得到水果", 0}, 0},
		{hiddenInnSpot{5, 111, 1200, 1608, "水果", "得到水果", 0}, 0},
		{hiddenInnSpot{5, 112, 1392, 1288, "鸡蛋", "发现鸡蛋", 0}, 0},
		{hiddenInnSpot{5, 108, 1184, 336, "龙涎草", "采得一颗龙涎草", 0}, 68},
		{hiddenInnSpot{5, 109, 1248, 352, "止血草", "采得一颗止血草", 0}, 69},
		{hiddenInnSpot{6, 144, 752, 1736, "止血草", "发现止血草", 0}, 0},
		{hiddenInnSpot{6, 143, 496, 1448, "水果", "发现水果", 0}, 0},
		{hiddenInnSpot{6, 145, 240, 1464, "水果", "发现水果", 0}, 0},
		{hiddenInnSpot{6, 141, 288, 1040, "藤甲", "捡到一件藤制的甲胄", 0}, 0},
		{hiddenInnSpot{6, 146, 400, 1096, "止血草", "发现止血草", 0}, 0},
		{hiddenInnSpot{6, 147, 784, 1288, "蜂巢", "发现蜂巢", 0}, 0},
		{hiddenInnSpot{6, 148, 832, 1264, "蜂王蜜", "发现蜂王蜜", 0}, 0},
	}
	claimed := []map[string]any{}
	blocked := []map[string]any{}
	for _, q := range spots {
		target = nil
		if q.Scene == 6 && field().Scene == 5 {
			walk(5, 1824, 1328, 0)
		}
		if field().Scene != q.Scene {
			t.Fatal("hidden-item route left the original village/harbor", q, field().Scene)
		}
		target = &q.hiddenInnSpot
		t.Log("outdoor original search", q.Scene, q.Event, q.Item)
		walk(q.Scene, q.X, q.Y, 96)
		v := field()
		actorPresent := false
		for _, actor := range v.Field.Actors {
			if actor[0] == 1 && actor[1] == q.Event {
				actorPresent = true
				if actor[2] != q.Sprite {
					t.Fatal("original outdoor herb sprite changed", q, actor[2])
				}
			}
		}
		if actorPresent != (q.Sprite != 0) {
			t.Fatal("original hidden/visible search appearance changed", q, actorPresent)
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
			if q.Event == 145 {
				// The source contains this reward, but its own cell and all
				// four mode-1 search stances are blocked in the real map.
				// Retain it as unresolved original content; never move it or
				// treat a static reward record as a collected gameplay item.
				du, dv := q.X-v.X, q.Y-v.Y
				u, z := (du/16+dv/8)/2, (dv/8-du/16)/2
				if u < -8 || u > 8 || z < -8 || z > 8 || v.Field.Cells[(z+8)*17+u+8]&1 == 0 {
					t.Fatal("unreviewed fruit is not blocked at its original location", q)
				}
				blocked = append(blocked, map[string]any{"scene": q.Scene, "event": q.Event, "position": [2]int{q.X, q.Y}, "item": q.Item, "mode1AdjacentStancesBlocked": 4, "ownCellBlocked": true, "publicField": v.Field, "collected": false, "status": "unresolved original inaccessible candidate; not gameplay acceptance"})
				continue
			}
			t.Fatal("no original outdoor search stance", q, v.X, v.Y)
		}
		walk(q.Scene, stance[0], stance[1], 0)
		keys(direction, []uint32{4, 8, 16, 32}[direction])
		keys(4, 0)
		progress()
		field()
		save(fmt.Sprint("event-", q.Event, "-unpicked"))
		keys(4, 2)
		progress()
		v = field()
		if !notices[q.Event] {
			t.Fatal("original outdoor search notice missing", q, hiddenNoticeText(v))
		}
		items[q.Item]++
		inventory()
		save(fmt.Sprint("event-", q.Event, "-picked"))
		for retry := 0; retry < 3; retry++ {
			keys(4, 2)
			progress()
			field()
		}
		inventory()
		v = field()
		if v.Cash != initial.Cash || !bytes.Equal(raw(v.Party), raw(initial.Party)) {
			t.Fatal("outdoor exploration changed original wallet/party", v.Cash, v.Party)
		}
		if q.Sprite != 0 {
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					t.Fatal("picked original herb did not disappear", q.Event)
				}
			}
		}
		claimed = append(claimed, map[string]any{"scene": q.Scene, "event": q.Event, "position": [2]int{q.X, q.Y}, "item": q.Item, "sprite": q.Sprite, "notification": true, "repeatAfterSaveRejected": true, "originalHerbDisappearance": q.Sprite != 0})
	}
	if len(claimed) != 11 || len(blocked) != 1 || len(notices) != 11 || saves != 33 || !bytes.Equal(raw(items), raw(map[string]int{"水果": 3, "鸡蛋": 1, "龙涎草": 1, "止血草": 3, "藤甲": 1, "蜂巢": 1, "蜂王蜜": 1})) {
		t.Fatal("outdoor hidden-item acceptance did not cover its complete batch", len(claimed), len(blocked), len(notices), saves, items)
	}
	save("complete")
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "original-HP new-game route to Yuhang village/harbor; ordinary movement, hidden search and visible herb harvesting; exact displayed quantities, original notices, fresh-runtime unpicked/acquisition/picked saves, no duplicate rewards or invented hidden markers; nearby NPC conversations remain intact; partial regional loot coverage; inaccessible candidate is explicitly excluded from collected sites", "developmentProfile": developmentHealthProfile(), "sites": claimed, "unresolvedBlockedCandidates": blocked, "items": items, "cash": visible().Cash, "saves": saves, "readings": readings}, "", "  ")
	if err := os.WriteFile(campaignEvidencePath("hidden-village-rewards.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}
