package pal_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type capitalEconomyDriver struct {
	*caiyiForestDriver
	source, mode, label, choice, receipt string
	choiceSent, readingSaved, itemSent   bool
}

func TestOriginalCapitalThreeFundingGrantsWithRealPurchasesAndSave(t *testing.T) {
	if os.Getenv("PAL_CAPITAL_ECONOMY") != "1" {
		t.Skip("explicit original capital funding and shop acceptance")
	}
	r := newCapitalEconomy(t, "capital-manor-step-27-state.json", "capital-funding-v1")
	initial := r.field()
	initialItems := r.inventoryCounts("金童剑")
	if initial.Cash != 14499 || r.tables.Objects[183][1] != 12000 {
		t.Fatal("original funding/shop source or price changed", initial.Cash)
	}
	steps := []map[string]any{}
	for grant := 1; grant <= 3; grant++ {
		if grant > 1 {
			steps = append(steps, r.converse(109, 2003, fmt.Sprint("grant-", grant, "-declined"), "否", "", 0))
		}
		choice := ""
		if grant > 1 {
			choice = "是"
		}
		steps = append(steps, r.converse(109, 2003, fmt.Sprint("grant-", grant), choice, "得三万文钱", 30000))
		steps = append(steps, r.converse(109, 2003, fmt.Sprint("grant-", grant, "-high-balance"), "", "", 0))
		r.toCity()
		r.door(1822, 136)
		steps = append(steps, r.spendBelowThirtyThousand(grant))
		r.door(2323, 101)
		r.toGarden()
	}
	for repeat := 1; repeat <= 3; repeat++ {
		steps = append(steps, r.converse(109, 2003, fmt.Sprint("funding-exhausted-", repeat), "", "", 0))
	}
	final := r.field()
	items := r.inventoryCounts("金童剑")
	if final.Cash != 20499 || items["金童剑"]-initialItems["金童剑"] != 7 || r.proof.Battles != 0 {
		t.Fatal("original capped funding or purchased inventory changed", final.Cash, initialItems, items)
	}
	r.finish("capital-funding-v1-result.json", map[string]any{"passed": true, "kind": "original-capped-funding", "event": 2003, "grants": 3, "grantAmount": 30000, "shopItem": 183, "shopPrice": 12000, "shopQuantity": 7, "cashInitial": initial.Cash, "cashFinal": final.Cash, "inventoryInitial": initialItems, "inventoryFinal": items, "steps": steps})
}

func TestOriginalSugarAndWineRepeatPurchasesCancelAndSave(t *testing.T) {
	if os.Getenv("PAL_CAPITAL_ECONOMY") != "1" {
		t.Skip("explicit original repeatable merchant acceptance")
	}
	for _, spec := range []struct {
		name, source, item  string
		scene, event, price int
	}{
		{"sugar", "toad-mountain-step-18-state.json", "糖葫芦", 112, 2086, 30},
		{"wine", "capital-manor-step-33-state.json", "酒", 128, 2225, 100},
	} {
		t.Run(spec.name, func(t *testing.T) {
			r := newCapitalEconomy(t, spec.source, "capital-merchant-v1-"+spec.name)
			if spec.scene == 128 {
				r.door(1811, 128)
			}
			initial := r.field()
			before := r.inventoryCounts(spec.item)
			steps := []map[string]any{r.converse(spec.scene, spec.event, "purchase-declined", "否", "", 0)}
			for purchase := 1; purchase <= 2; purchase++ {
				steps = append(steps, r.converse(spec.scene, spec.event, fmt.Sprint("purchase-", purchase), "是", "", -spec.price))
				after := r.inventoryCounts(spec.item)
				if after[spec.item]-before[spec.item] != purchase {
					t.Fatal("original paid item quantity wrong", spec.item, before, after)
				}
			}
			steps = append(steps, r.converse(spec.scene, spec.event, "purchase-declined-again", "否", "", 0))
			final := r.field()
			after := r.inventoryCounts(spec.item)
			if int64(final.Cash)-int64(initial.Cash) != int64(-2*spec.price) || after[spec.item]-before[spec.item] != 2 || r.proof.Battles != 0 {
				t.Fatal("original paid repeat transaction changed", initial.Cash, final.Cash, before, after)
			}
			r.finish("capital-merchant-v1-"+spec.name+"-result.json", map[string]any{"passed": true, "kind": "original-paid-repeatable-purchase", "event": spec.event, "price": spec.price, "item": spec.item, "purchases": 2, "cashInitial": initial.Cash, "cashFinal": final.Cash, "inventoryInitial": before, "inventoryFinal": after, "steps": steps})
		})
	}
}

func newCapitalEconomy(t *testing.T, source, prefix string) *capitalEconomyDriver {
	t.Helper()
	data, err := os.ReadFile(campaignEvidencePath(source, 1))
	if err != nil {
		t.Fatal(err)
	}
	if campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("capital economy requires current registered authoritative profile")
	}
	if err := acceptedCampaignRoot(source, data, ""); err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	base := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, source), tables: tables, prefix: prefix, reserveScenes: []int{101, 109, 112, 115, 116, 117, 119, 128, 136}, milestones: map[string]string{}, physicalAttacks: true, proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	base.allowedMenus = map[int]bool{8: true, 12: true}
	base.allowedRoles = map[int]bool{}
	for _, p := range base.visible().Party {
		base.allowedRoles[p.Role] = true
	}
	base.battlePartySize = len(base.visible().Party)
	base.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	base.reserve(-1)
	r := &capitalEconomyDriver{caiyiForestDriver: base, source: source}
	base.notice = r.observe
	t.Cleanup(func() {
		if t.Failed() {
			for suffix, content := range map[string][]byte{"state": r.g.Data, "view": raw(r.visible()), "proof": raw(r.proof)} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-failure-"+suffix+".json", 1), content, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	})
	return r
}

func (r *capitalEconomyDriver) begin(label, mode, choice, receipt string) {
	r.label, r.mode, r.choice, r.receipt = label, mode, choice, receipt
	r.choiceSent, r.readingSaved, r.itemSent = false, false, false
}

func (r *capitalEconomyDriver) observe(v pal.View) {
	if r.receipt != "" && !r.readingSaved {
		text := hiddenNoticeText(v)
		if v.Dialogue != nil {
			text += v.Dialogue.Text
		}
		if strings.Contains(text, r.receipt) {
			r.readingSaved = true
			r.save(r.label + "-reading")
			v = r.visible()
		}
	}
	if v.Phase != "menu" || v.Seq != v.AppliedSeq {
		return
	}
	wanted := -1
	for index, item := range v.Menu.Items {
		if item.Label == r.choice && r.choice != "" {
			wanted = index
		}
	}
	if wanted >= 0 && !r.choiceSent {
		value := v.Menu.Items[wanted].Value
		if v.Menu.Selected != value {
			r.keys(32, 4)
		} else {
			r.save(r.label + "-choice")
			r.choiceSent = true
			r.keys(2, 4)
		}
		return
	}
	if r.mode == "buy" && v.Menu.Kind == 8 && !r.itemSent {
		for _, item := range v.Menu.Items {
			if item.Label == "金童剑" {
				if !item.Enabled || v.Menu.Selected != item.Value {
					r.t.Fatal("original selected shop item is not available", v.Menu)
				}
				r.save(r.label + "-item-selected")
				r.itemSent = true
				r.keys(2, 4)
				return
			}
		}
	}
}

func (r *capitalEconomyDriver) position(event int) (int, int) {
	e := r.tables.Events[event-1]
	x, y := int(e.X), int(e.Y)
	if event == 2003 {
		x, y = 672, 1264
	}
	if f := r.visible().Field; f != nil {
		for _, actor := range f.Actors {
			if actor[0] == 1 && actor[1] == event {
				x, y = actor[3], actor[4]
			}
		}
	}
	return x, y
}

func (r *capitalEconomyDriver) field() pal.View {
	r.t.Helper()
	for step := 0; step < 1200; step++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq && !r.proof.InBattle {
			return v
		}
		r.progress()
	}
	r.t.Fatal("original economy operation did not return to field", r.label, r.visible().Phase, r.visible().Menu)
	return pal.View{}
}

func (r *capitalEconomyDriver) converse(scene, event int, label, choice, receipt string, delta int) map[string]any {
	r.t.Helper()
	r.begin(label, "talk", choice, receipt)
	before := r.field()
	if before.Scene != scene {
		r.t.Fatal("original economy conversation source scene changed", label, before.Scene)
	}
	r.save(label + "-before")
	readings := r.proof.Readings
	visits := map[[2]int]int{}
	for step := 0; step < 1200; step++ {
		v := r.field()
		if r.proof.Readings > readings || r.choiceSent {
			if int64(v.Cash)-int64(before.Cash) != int64(delta) || choice != "" && !r.choiceSent || receipt != "" && !r.readingSaved {
				r.t.Fatal("original transaction amount, choice or receipt changed", label, before.Cash, v.Cash, delta, r.choiceSent, r.readingSaved)
			}
			r.mode, r.choice, r.receipt = "", "", ""
			r.save(label + "-after")
			return map[string]any{"label": label, "event": event, "choice": choice, "receipt": receipt, "cashBefore": before.Cash, "cashAfter": v.Cash, "delta": delta}
		}
		if v.Scene != scene {
			r.t.Fatal("original transaction changed scene", label, v.Scene)
		}
		pose := [2]int{v.X, v.Y}
		visits[pose]++
		if visits[pose] > 40 {
			r.t.Fatal("original economy search repeated a blocked pose", label, pose, event)
		}
		x, y := r.position(event)
		if fieldTestDistance(v.X, v.Y, x, y) > 160 {
			r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 128)
		} else {
			r.search(event, x, y)
		}
		r.progress()
	}
	r.t.Fatal("original transaction did not finish", label)
	return nil
}

func (r *capitalEconomyDriver) door(event, destination int) {
	r.t.Helper()
	r.begin("travel", "", "", "")
	r.reserve(event)
	e := r.tables.Events[event-1]
	for step := 0; step < 1600; step++ {
		v := r.field()
		if v.Scene == destination {
			r.reserve(-1)
			return
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, int(e.X), int(e.Y), 0)
		r.progress()
	}
	r.t.Fatal("original economy door blocked", event, destination)
}

func (r *capitalEconomyDriver) toCity() {
	for _, q := range [][2]int{{1998, 117}, {2150, 116}, {2136, 119}, {2163, 115}, {2124, 101}} {
		r.door(q[0], q[1])
	}
}

func (r *capitalEconomyDriver) toGarden() {
	for _, q := range [][2]int{{1810, 115}, {2123, 119}, {2161, 116}, {2135, 117}, {2149, 109}} {
		r.door(q[0], q[1])
	}
}

func (r *capitalEconomyDriver) spendBelowThirtyThousand(grant int) map[string]any {
	r.t.Helper()
	r.begin("shop-open", "", "", "")
	before := r.field()
	beforeItems := r.inventoryCounts("金童剑")
	for step := 0; step < 1600; step++ {
		v := r.visible()
		if v.Phase == "menu" && v.Menu.Kind == 8 && v.Seq == v.AppliedSeq {
			break
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			x, y := r.position(2324)
			if fieldTestDistance(v.X, v.Y, x, y) > 160 {
				r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, 128)
			} else {
				r.search(2324, x, y)
			}
		}
		r.progress()
	}
	if r.visible().Menu.Kind != 8 {
		r.t.Fatal("original weapon shop did not open", r.visible().Menu)
	}
	r.save(fmt.Sprint("spend-", grant, "-shop-open"))
	purchases := []map[string]any{}
	for r.visible().Cash >= 30000 {
		label := fmt.Sprint("spend-", grant, "-item-", len(purchases)+1)
		r.begin(label, "buy", "是", "")
		cash := r.visible().Cash
		for step := 0; step < 2000 && r.visible().Cash == cash; step++ {
			r.progress()
		}
		if r.visible().Cash+12000 != cash || !r.itemSent || !r.choiceSent {
			r.t.Fatal("original shop price/confirmation failed", label, cash, r.visible().Cash, r.visible().Menu)
		}
		r.begin(label, "", "", "")
		r.save(label + "-after")
		purchases = append(purchases, map[string]any{"label": label, "item": 183, "price": 12000, "cashBefore": cash, "cashAfter": r.visible().Cash})
	}
	r.mode, r.choice, r.receipt = "", "", ""
	for step := 0; step < 1000 && r.visible().Phase != "field"; step++ {
		if v := r.visible(); v.Phase == "menu" && v.Seq == v.AppliedSeq {
			r.keys(1, 4)
		}
		r.progress()
	}
	final := r.field()
	afterItems := r.inventoryCounts("金童剑")
	if afterItems["金童剑"]-beforeItems["金童剑"] != len(purchases) || before.Cash-final.Cash != uint32(len(purchases)*12000) || final.Cash >= 30000 {
		r.t.Fatal("original purchases did not account for spent funding", before.Cash, final.Cash, beforeItems, afterItems, purchases)
	}
	r.save(fmt.Sprint("spend-", grant, "-inventory"))
	return map[string]any{"label": fmt.Sprint("spend-", grant), "cashBefore": before.Cash, "cashAfter": final.Cash, "inventoryBefore": beforeItems, "inventoryAfter": afterItems, "purchases": purchases}
}

func (r *capitalEconomyDriver) finish(name string, result map[string]any) {
	r.t.Helper()
	data, err := os.ReadFile(campaignEvidencePath(r.source, 1))
	if err != nil {
		r.t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	result["verificationVersion"], result["candidate"], result["developmentProfile"] = 1, campaignVariant(), developmentHealthProfile()
	result["sourceCheckpoint"], result["sourceSHA256"], result["proof"] = r.source, hex.EncodeToString(digest[:]), r.proof
	result["nativeEdits"], result["resourceUploads"] = false, 0
	for n, content := range map[string][]byte{name: raw(result), name[:len(name)-5] + "-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(n, 1), content, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}
