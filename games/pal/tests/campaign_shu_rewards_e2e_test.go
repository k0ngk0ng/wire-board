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

func TestOriginalShuAlchemySevenHiddenRewardsAndSavedNoDuplication(t *testing.T) {
	name := "shu-tower-step-5-state.json"
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables,
		prefix: "shu-rewards", milestones: map[string]string{}, reserveScenes: []int{161},
		proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	r.reserve(-1)
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("shu-rewards-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	initial := r.field()
	if initial.Scene != 161 || len(initial.Party) != 2 || initial.Party[0].Role != 0 || initial.Party[1].Role != 2 {
		t.Fatal("Shu alchemy source is not the earned Li/Yue arrival", initial.Scene, initial.Party)
	}
	spots := []struct {
		Event int
		Item  string
	}{
		{2680, "天香续命露"}, {2681, "紫菁玉蓉膏"}, {2682, "雪莲子"}, {2683, "灵葫仙丹"},
		{2684, "九阴散"}, {2685, "金刚符"}, {2686, "圣灵符"},
	}
	names := []string{}
	for _, q := range spots {
		names = append(names, q.Item)
	}
	expected := r.inventoryCounts(names...)
	before := map[string]int{}
	for name, amount := range expected {
		before[name] = amount
	}
	claimed := []map[string]any{}
	for _, q := range spots {
		x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
		label, seen := fmt.Sprint("reward-", q.Event), false
		r.notice = func(v pal.View) {
			text := strings.NewReplacer(" ", "", "　", "", "\n", "").Replace(hiddenNoticeText(v))
			if !seen && strings.Contains(text, "得到"+q.Item) {
				seen = true
				r.save(label + "-notice")
			}
		}
		t.Log("original Shu hidden reward", q.Event, q.Item, x, y)
		for i := 0; ; i++ {
			v := r.field()
			if v.Scene != 161 || i > 600 {
				t.Fatal("original Shu hidden reward approach failed", q, v.Scene, v.X, v.Y)
			}
			if fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				for _, a := range v.Field.Actors {
					if a[0] == 1 && a[1] == q.Event {
						t.Fatal("hidden Shu reward gained an actor marker", q.Event, a)
					}
				}
				break
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 96)
			r.progress()
		}
		r.save(label + "-unpicked")
		visits := map[[2]int]int{}
		for i := 0; i < 200 && !seen; i++ {
			v := r.field()
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 32 || v.Scene != 161 {
				t.Fatal("original hidden Shu search stance blocked", q, at, v.Scene)
			}
			r.search(q.Event, x, y)
			r.progress()
		}
		if !seen {
			t.Fatal("original hidden Shu reward notice missing", q)
		}
		expected[q.Item]++
		after := r.inventoryCounts(names...)
		if !bytes.Equal(raw(expected), raw(after)) {
			t.Fatal("original hidden Shu reward inventory delta changed", q, expected, after)
		}
		r.save(label + "-collected")
		for retry := 0; retry < 3; retry++ {
			r.search(q.Event, x, y)
			r.progress()
			r.field()
		}
		if !bytes.Equal(raw(after), raw(r.inventoryCounts(names...))) {
			t.Fatal("original hidden Shu reward duplicated after saved repeated searches", q)
		}
		r.save(label + "-repeated")
		claimed = append(claimed, map[string]any{"scene": 161, "event": q.Event, "position": [2]int{x, y}, "item": q.Item, "amount": 1, "repeatSearches": 3, "invisible": true})
	}
	v := r.field()
	if v.Scene != 161 || len(claimed) != 7 || r.proof.Battles != 0 || v.Cash != initial.Cash || !bytes.Equal(raw(v.Party), raw(initial.Party)) || r.inventoryCounts("玉佛珠")["玉佛珠"] != 0 {
		t.Fatal("optional hidden Shu collection changed narrative, party or cash", v.Scene, v.Party, v.Cash, r.proof)
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "actual optional exploration of seven original invisible alchemy-room rewards; original quantities and notices, pickup-stage fresh-runtime saves, three repeated saved searches per reward and no visible marker; no native edits or story-count increment", "claimed": claimed, "inventoryBefore": before, "inventoryAfter": expected, "party": v.Party, "cash": v.Cash, "proof": r.proof})
	for name, data := range map[string][]byte{"shu-hidden-rewards.json": report, "shu-hidden-rewards-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
