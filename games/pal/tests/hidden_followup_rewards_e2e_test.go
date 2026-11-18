package pal_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func hiddenFollowupConversation(t *testing.T, r *capitalEconomyDriver, scene, event int, label, token string, shop bool, cash uint32) {
	t.Helper()
	hit := false
	r.notice = func(v pal.View) {
		if hit || v.Scene != scene {
			return
		}
		if shop {
			if v.Phase != "menu" || v.Menu.Kind != 8 || v.Seq != v.AppliedSeq {
				return
			}
		} else if v.Dialogue == nil || !strings.Contains(v.Dialogue.Text, token) {
			return
		}
		if v.Cash != cash {
			t.Fatal("reward handover cash differs", event, label, v.Cash, cash)
		}
		hit = true
		r.save(label + "-handover")
		if shop {
			r.keys(1, 4)
		}
	}
	visits := map[[2]int]int{}
	for i := 0; i < 1500; i++ {
		v := r.field()
		if v.Scene != scene {
			t.Fatal("optional reward left original scene", event, v.Scene)
		}
		if hit {
			r.notice = nil
			return
		}
		at := [2]int{v.X, v.Y}
		visits[at]++
		if visits[at] > 50 {
			t.Fatal("original reward search made no progress", event, label, at)
		}
		miscMerchantSearch(r, event, label)
		r.progress()
	}
	t.Fatal("original optional handover missing", event, label)
}

func TestOriginalScriptSwapSideRewardsSaveAndNoDuplication(t *testing.T) {
	mode := os.Getenv("PAL_SCRIPT_SWAP_REWARD")
	if mode == "" {
		t.Skip("explicit original script swap reward from ordinary new game")
	}
	scope := "script-swap-" + mode + "-v1"
	if mode == "doctor" {
		scope = "script-swap-doctor-v3"
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != scope || campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("script swap reward requires isolated original authority evidence")
	}
	var m gameapi.Module
	var c gameapi.Context
	var g gameapi.Match
	var event, scene, id int
	var item, notice string
	switch mode {
	case "boots":
		m, c, g = originalVillage(t, 1)
		event, scene, id, item, notice = 85, 5, 237, "布靴", "缝制的布靴"
	case "doctor":
		parent := "script-swap-v1-192-door-45-arrived-state.json"
		data, err := os.ReadFile(campaignEvidencePath(parent, 1))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "93055a38f3b2f0771bc4b31643479fc08bf31b49d52cdc422d7b749bfd2599f3" {
			t.Fatal("original doctor route parent differs", err)
		}
		r := loadRiverMedicine(t, parent)
		m, c, g = r.m, r.c, r.g
		event, scene, id, item, notice = 192, 11, 107, "灵山仙芝", "上好的灵芝"
	default:
		t.Fatal("unsupported original script swap reward", mode)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("script-swap-v1-%d", event)
	source := prefix + "-pre-reward-source.json"
	if err := os.WriteFile(campaignEvidencePath(source, 1), g.Data, 0600); err != nil {
		t.Fatal(err)
	}
	base := &caiyiForestDriver{riverMedicineDriver: &riverMedicineDriver{t: t, m: m, c: c, g: g}, tables: tables, prefix: prefix, reserveScenes: []int{3, 4, 5, 11}, milestones: map[string]string{}, allowedRoles: map[int]bool{0: true}, allowedMenus: map[int]bool{8: true}, proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	base.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	base.reserve(-1)
	r := &capitalEconomyDriver{caiyiForestDriver: base, source: source}
	initial := r.field()
	if event == 192 {
		if initial.Scene != 5 {
			t.Fatal("doctor source did not return from the inn", initial.Scene)
		}
		r.reserve(101)
		for step := 0; step < 800; step++ {
			v := r.field()
			if v.Scene != 5 {
				t.Fatal("original pharmacy approach left village", v.Scene)
			}
			if v.X == 1248 && v.Y == 1568 {
				break
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, 1248, 1568, 0)
			r.progress()
		}
		for step := 0; step < 24 && !r.proof.Saved["entry-neighbor-search-pose"]; step++ {
			miscMerchantSearch(r, 105, "entry-neighbor")
			r.progress()
		}
		if !r.proof.Saved["entry-neighbor-search-pose"] {
			t.Fatal("could not talk to original neighbor at pharmacy entrance")
		}
		r.field()
		r.save("neighbor-moved")
		door := tables.Events[100]
		for step := 0; step < 800 && r.field().Scene == 5; step++ {
			r.g = r.w.walk(t, r.m, &r.c, r.g, r.field(), int(door.X), int(door.Y), 32)
			r.progress()
		}
		if v := r.field(); v.Scene != 11 {
			t.Fatal("original pharmacy door area not reached", v.Scene, v.X, v.Y)
		}
		r.reserve(-1)
		r.save("door-101-arrived")
	}
	before := r.inventoryCounts(item)[item]
	r.save("unstarted")
	if event == 85 {
		hiddenFollowupConversation(t, r, scene, event, "first-sister", "你等会再来找我拿", false, initial.Cash)
		seen := false
		r.notice = func(v pal.View) {
			if !seen && v.Dialogue != nil && strings.Contains(v.Dialogue.Text, "你和李大娘要是有空") {
				seen = true
				r.save("second-sister-handover")
			}
		}
		approach := tables.Events[83]
		for step := 0; step < 1400 && !seen; step++ {
			v := r.field()
			if seen {
				break
			}
			if v.Scene != scene {
				t.Fatal("original invitation approach left village", v.Scene)
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(approach.X), int(approach.Y), 0)
			r.progress()
		}
		if !seen {
			t.Fatal("original sister invitation did not run")
		}
		r.field()
		r.notice = nil
		r.save("invitation-finished")
	}
	hiddenFollowupConversation(t, r, scene, event, "reward", notice, false, initial.Cash)
	after := r.inventoryCounts(item)[item]
	if after != before+1 {
		t.Fatal("original script swap reward did not grant once", event, before, after)
	}
	r.save("received")
	for retry := 1; retry <= 3; retry++ {
		label := fmt.Sprintf("repeat-%d", retry)
		r.notice = func(v pal.View) {
			if v.Phase == "menu" && v.Menu.Kind == 8 && v.Seq == v.AppliedSeq {
				r.keys(1, 4)
			}
		}
		for step := 0; step < 1400 && !r.proof.Saved[label+"-search-pose"]; step++ {
			if r.field().Scene != scene {
				t.Fatal("repeat talk left original location", event)
			}
			miscMerchantSearch(r, event, label)
			r.progress()
		}
		if !r.proof.Saved[label+"-search-pose"] || r.inventoryCounts(item)[item] != after || r.field().Cash != initial.Cash {
			t.Fatal("script swap reward repeats or cannot search", event, label)
		}
	}
	r.notice = nil
	r.save("repeated")
	r.finish(prefix+"-result.json", map[string]any{"passed": true, "event": event, "scene": scene, "item": item, "itemID": id, "nativeEdits": false, "amountBefore": before, "amountAfter": after, "amountAfterRepeat": r.inventoryCounts(item)[item], "price": 0, "cashBefore": initial.Cash, "cashAfter": r.field().Cash, "repeatSearches": 3, "freshNewGameParent": true})
}

func TestOriginalFollowupDialogueRewardsSaveAndNoDuplication(t *testing.T) {
	if os.Getenv("PAL_FOLLOWUP_REWARDS") != "1" {
		t.Skip("explicit original followup rewards from accepted campaign checkpoints")
	}
	if os.Getenv("PAL_EVIDENCE_SCOPE") != "followup-rewards-v1" {
		t.Fatal("followup reward test requires isolated evidence")
	}
	for _, q := range []struct {
		Name, Source, Item, Notice, Repeat string
		Scene, Event, ItemID, Charge       int
	}{
		{"boatman", "hidden-island-entry-state.json", "行军丹", "你带着或许用的上", "我看～咱们还是回去算了", 15, 202, 100, 0},
		{"silver-hairpin", "campaign-yueru-meeting-state.json", "银钗", "文钱就行啦", "", 24, 434, 199, 400},
	} {
		t.Run(q.Name, func(t *testing.T) {
			prefix := fmt.Sprintf("followup-v1-%d", q.Event)
			r := newCapitalEconomy(t, q.Source, prefix)
			r.reserveScenes = []int{15, 23, 24}
			r.reserve(-1)
			if q.Scene == 24 {
				r.door(415, 24)
				r.save("suzhou-arrived")
			}
			initial := r.field()
			before := r.inventoryCounts(q.Item)[q.Item]
			if initial.Cash < uint32(q.Charge) {
				t.Fatal("source does not have original purchase price", initial.Cash, q.Charge)
			}
			r.save("unstarted")
			if q.Event == 202 {
				hiddenFollowupConversation(t, r, q.Scene, q.Event, "greeting", "找到菩萨娘娘了吗", false, initial.Cash)
				if r.inventoryCounts(q.Item)[q.Item] != before {
					t.Fatal("first greeting must not hand over medicine")
				}
				r.save("between-visits")
			}
			expectedCash := initial.Cash - uint32(q.Charge)
			hiddenFollowupConversation(t, r, q.Scene, q.Event, "reward", q.Notice, false, expectedCash)
			after := r.inventoryCounts(q.Item)[q.Item]
			if after != before+1 || r.field().Cash != expectedCash {
				t.Fatal("original reward inventory delta or price differs", before, after, r.field().Cash)
			}
			r.save("received")
			for retry := 1; retry <= 3; retry++ {
				hiddenFollowupConversation(t, r, q.Scene, q.Event, fmt.Sprintf("repeat-%d", retry), q.Repeat, q.Event == 434, expectedCash)
				if r.inventoryCounts(q.Item)[q.Item] != after {
					t.Fatal("original reward duplicated after save", q.Event, retry)
				}
			}
			final := r.field()
			if r.proof.Battles != 0 || final.Cash != expectedCash || !bytes.Equal(raw(initial.Party), raw(final.Party)) {
				t.Fatal("optional dialogue changed unrelated original state", q.Event, r.proof.Battles, final.Cash)
			}
			r.save("repeated")
			r.finish(prefix+"-result.json", map[string]any{"passed": true, "event": q.Event, "scene": q.Scene, "item": q.Item, "itemID": q.ItemID, "nativeEdits": false, "amountBefore": before, "amountAfter": after, "amountAfterRepeat": r.inventoryCounts(q.Item)[q.Item], "price": q.Charge, "cashBefore": initial.Cash, "cashAfter": final.Cash, "repeatSearches": 3})
		})
	}
}
