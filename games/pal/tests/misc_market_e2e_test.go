package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

func miscMerchantFirst(tables *resources.Tables, v pal.View, x, y, direction int) int {
	delta := [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}}[direction]
	points := [][2]int{{x, y}}
	a, b := x, y
	for step := 0; step < 4; step++ {
		points = append(points, [2]int{a + delta[0], b + delta[1]}, [2]int{a, b + delta[1]*2}, [2]int{a + delta[0]*2, b})
		a, b = a+delta[0], b+delta[1]
	}
	for index, point := range points {
		for number := int(tables.Scenes[v.Scene-1].FirstEvent) + 1; number <= int(tables.Scenes[v.Scene].FirstEvent); number++ {
			e := tables.Events[number-1]
			if e.State <= 0 || e.TriggerMode >= 4 || int(e.TriggerMode)*6-4 <= index {
				continue
			}
			ex, ey := int(e.X), int(e.Y)
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == number {
					ex, ey = actor[3], actor[4]
				}
			}
			if point[0]/32 == ex/32 && point[1]/16 == ey/16 && (point[0]%32 != 0) == (ex%32 != 0) {
				return number
			}
		}
	}
	return 0
}

func miscMerchantSearch(r *capitalEconomyDriver, event int, label string) {
	r.t.Helper()
	v := r.field()
	x, y := r.position(event)
	best, direction, stance := int(^uint(0)>>1), -1, [2]int{}
	for index, flags := range v.Field.Cells[:289] {
		if flags&3 != 0 {
			continue
		}
		u, z := index%17-8, index/17-8
		px, py := v.X+(u-z)*16, v.Y+(u+z)*8
		for d := 0; d < 4; d++ {
			if miscMerchantFirst(r.tables, v, px, py, d) != event {
				continue
			}
			score := fieldTestDistance(v.X, v.Y, px, py)
			if score < best {
				best, direction, stance = score, d, [2]int{px, py}
			}
		}
	}
	if direction < 0 {
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 128)
		return
	}
	if stance != [2]int{v.X, v.Y} {
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, stance[0], stance[1], 0)
		return
	}
	r.keys([]uint32{4, 8, 16, 32}[direction], direction)
	r.keys(0, 4)
	r.progress()
	v = r.field()
	if miscMerchantFirst(r.tables, v, v.X, v.Y, v.Field.Direction) != event {
		return
	}
	if !r.proof.Saved[label+"-search-pose"] {
		r.save(label + "-search-pose")
	}
	r.keys(2, 4)
}

func miscMerchantTrade(r *capitalEconomyDriver, scene, event int, label, choice, receipt string, delta int) map[string]any {
	r.t.Helper()
	r.begin(label, "talk", choice, receipt)
	before := r.field()
	if before.Scene != scene {
		r.t.Fatal("original market source scene changed", scene, before.Scene)
	}
	r.save(label + "-before")
	readings := r.proof.Readings
	visits := map[[2]int]int{}
	for step := 0; step < 1400; step++ {
		v := r.field()
		if r.proof.Readings > readings || r.choiceSent {
			if !r.choiceSent || !r.readingSaved || int64(v.Cash)-int64(before.Cash) != int64(delta) || !r.proof.Saved[label+"-search-pose"] {
				r.t.Fatal("original merchant choice or wallet changed", event, label, before.Cash, v.Cash, delta, r.choiceSent, r.readingSaved)
			}
			r.mode, r.choice, r.receipt = "", "", ""
			r.save(label + "-after")
			return map[string]any{"label": label, "event": event, "choice": choice, "cashBefore": before.Cash, "cashAfter": v.Cash, "delta": delta, "receipt": receipt}
		}
		at := [2]int{v.X, v.Y}
		visits[at]++
		if v.Scene != scene || visits[at] > 50 {
			r.t.Fatal("original merchant search blocked", event, label, at)
		}
		miscMerchantSearch(r, event, label)
		r.progress()
	}
	r.t.Fatal("original merchant transaction timed out", event, label)
	return nil
}

func TestOriginalEarlyMerchantsPaidRepeatAndSavedChoices(t *testing.T) {
	mode := os.Getenv("PAL_MISC_MERCHANT")
	if mode == "" {
		t.Skip("explicit original early merchant transactions")
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != "misc-rewards-v1" {
		t.Fatal("early merchants require isolated scope")
	}
	specs := map[string]struct {
		Source, Item, Receipt            string
		Scene, Event, Price, DeclineCost int
	}{
		"fruit":      {"market-harbor-entry-state.json", "水果", "来喔", 6, 129, 25, 0},
		"sugar":      {"campaign-yueru-meeting-state.json", "糖葫芦", "买糖葫芦吗", 24, 438, 20, 20},
		"rice-left":  {"white-river-ginkgo-search-state.json", "糯米", "买糯米吗", 51, 846, 100, 0},
		"rice-right": {"white-river-ginkgo-search-state.json", "糯米", "买糯米吗", 51, 847, 100, 0},
	}
	q, ok := specs[mode]
	if !ok {
		t.Fatal("unsupported original merchant", mode)
	}
	prefix := fmt.Sprint("misc-market-v1-", q.Event)
	r := newCapitalEconomy(t, q.Source, prefix)
	r.reserveScenes = []int{5, 6, 23, 24, 50, 51, 53}
	r.reserve(-1)
	observe := r.notice
	r.notice = func(v pal.View) {
		if r.receipt != "" && !r.readingSaved && strings.Contains(originalVisibleText(v, false), r.receipt) {
			r.readingSaved = true
			r.save(r.label + "-reading")
			v = r.visible()
		}
		observe(v)
	}
	if q.Scene == 24 {
		r.door(415, 24)
		r.save("north-suzhou-arrived")
	}
	if q.Scene == 51 {
		r.door(885, 50)
		r.save("white-river-arrived")
		r.door(830, 51)
		r.save("rice-shop-arrived")
	}
	initial := r.field()
	before := r.inventoryCounts(q.Item)
	steps := []map[string]any{}
	steps = append(steps, miscMerchantTrade(r, q.Scene, q.Event, "declined-first", "否", q.Receipt, -q.DeclineCost))
	for purchase := 1; purchase <= 2; purchase++ {
		steps = append(steps, miscMerchantTrade(r, q.Scene, q.Event, fmt.Sprint("purchase-", purchase), "是", q.Receipt, -q.Price))
		after := r.inventoryCounts(q.Item)
		if after[q.Item]-before[q.Item] != purchase {
			t.Fatal("original paid item count changed", q.Event, purchase, before, after)
		}
	}
	steps = append(steps, miscMerchantTrade(r, q.Scene, q.Event, "declined-last", "否", q.Receipt, -q.DeclineCost))
	final := r.field()
	after := r.inventoryCounts(q.Item)
	if after[q.Item]-before[q.Item] != 2 || int64(final.Cash)-int64(initial.Cash) != int64(-2*q.Price-2*q.DeclineCost) || r.proof.Battles != 0 || !bytes.Equal(raw(initial.Party), raw(final.Party)) {
		t.Fatal("original merchant repeat, decline or saved transaction changed", q.Event, initial.Cash, final.Cash, before, after)
	}
	r.save("complete")
	source, err := os.ReadFile(campaignEvidencePath(q.Source, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	result := raw(map[string]any{"passed": true, "nativeEdits": false, "kind": "original-paid-repeatable-merchant", "event": q.Event, "scene": q.Scene, "item": q.Item, "price": q.Price, "declineCost": q.DeclineCost, "chargeBeforeChoice": q.DeclineCost > 0, "sourceCheckpoint": q.Source, "sourceSHA256": hex.EncodeToString(hash[:]), "purchases": 2, "declines": 2, "steps": steps, "inventoryBefore": before, "inventoryAfter": after, "cashInitial": initial.Cash, "cashFinal": final.Cash, "proof": r.proof})
	for suffix, data := range map[string][]byte{"result": result, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
