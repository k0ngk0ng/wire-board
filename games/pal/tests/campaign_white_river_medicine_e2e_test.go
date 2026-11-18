package pal_test

// Test first: borrowed tools duplicate/disappear; an item works away from its
// original target; fishing or trapping loses a native animation/continuation;
// ingredients or medicine repeat on resume; party departure/rejoin is skipped;
// the original six-herb handover cannot continue or accepts stale controls.
// All routes/event identities here are walkthrough data, never bot strategy.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type riverMedicineProof struct {
	CaughtAt                                                [2]int
	Borrowed, FishCaught, RodReturned                       bool
	TrapPicked, TrapPlaced, DeerCaught, Antler, YueReturned bool
	Brewed, Cured, Temple, TempleStory                      bool
	Saves, Readings, Steps                                  int
	LastPrompt                                              uint64
	Saved                                                   map[string]bool
}

type riverMedicineDriver struct {
	t *testing.T
	m gameapi.Module
	c gameapi.Context
	g gameapi.Match
	p riverMedicineProof
	w campaignMapWalker
}

func TestOriginalWhiteRiverReunionGatheringTreatmentAndSave(t *testing.T) {
	r := loadRiverMedicine(t, "campaign-hidden-dragon-state.json")
	defer r.failure()
	source, err := os.ReadFile(campaignEvidencePath("campaign-hidden-dragon-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	r.m, r.c, r.g = runWhiteRiverPrescription(t, r.m, r.c, r.g, whiteRiverProof{Saved: map[string]bool{}})
	runRiverFishing(r)
	runRiverMedicine(r)
	readReport := func(name string) map[string]any {
		t.Helper()
		b, err := os.ReadFile(campaignEvidencePath(name, 1))
		if err != nil {
			t.Fatal(err)
		}
		var report map[string]any
		if err = json.Unmarshal(b, &report); err != nil {
			t.Fatal(err)
		}
		return report
	}
	full := map[string]any{"passed": true, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "developmentProfile": developmentHealthProfile(), "scope": "continuous ordinary original gameplay from genuine hidden-dragon end through reunion, prescription, party split, all herb tasks, tool return, medicine and original three-role party; not a new-game shortcut or native-state edit", "prefix": readReport("campaign-white-river-prescription.json"), "medicine": readReport("campaign-white-river.json")}
	b, _ := json.MarshalIndent(full, "", "  ")
	if err := os.WriteFile(campaignEvidencePath("campaign-white-river-full.json", 1), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func loadRiverMedicine(t *testing.T, name string) *riverMedicineDriver {
	t.Helper()
	b, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	m, _ := nativeModule(t)
	g := gameapi.Match{Schema: 1, Data: b}
	v := view(t, m, g, 0)
	c := gameapi.Context{Players: 1, Now: time.UnixMilli(v.ControlAt), Automated: []bool{false}}
	g, err = m.ResumeSaved(c, g)
	if err != nil {
		t.Fatal(err)
	}
	return &riverMedicineDriver{t: t, m: m, c: c, g: g, p: riverMedicineProof{Saved: map[string]bool{}}}
}

func (r *riverMedicineDriver) visible() pal.View { return view(r.t, r.m, r.g, 0) }

func (r *riverMedicineDriver) failure() {
	if !r.t.Failed() {
		return
	}
	for name, data := range map[string][]byte{"white-river-medicine-failure-state.json": r.g.Data, "white-river-medicine-failure-proof.json": raw(r.p), "white-river-medicine-failure-view.json": raw(r.visible())} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			r.t.Log(err)
		}
	}
}

func (r *riverMedicineDriver) save(name string) {
	r.t.Helper()
	before := r.visible()
	var err error
	r.g, err = r.m.SaveControl(r.c, r.g, true)
	if err != nil {
		r.t.Fatal(err)
	}
	r.c.Now = r.c.Now.Add(24 * time.Hour)
	r.m, _ = nativeModule(r.t)
	r.g, err = r.m.ResumeSaved(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
	normalize := func(v pal.View) pal.View {
		v.Epoch, v.Seq, v.AppliedSeq, v.DiscardedSeq, v.ControlAt = 0, 0, 0, 0, 0
		return v
	}
	if !bytes.Equal(raw(normalize(before)), raw(normalize(r.visible()))) {
		r.t.Fatal("medicine save changed native continuation", name)
	}
	if _, err := r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("medicine save accepted old controls", name)
	}
	r.p.Saves++
	r.p.Saved[name] = true
	for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.p)} {
		if err := os.WriteFile(campaignEvidencePath("white-river-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *riverMedicineDriver) keys(keys uint32, direction int) {
	r.t.Helper()
	v := r.visible()
	r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: direction, Keys: keys})
}

func (r *riverMedicineDriver) progress() {
	r.t.Helper()
	v := r.visible()
	r.p.Steps++
	if d := v.Dialogue; d != nil {
		if d.ID != r.p.LastPrompt {
			r.p.LastPrompt = d.ID
			r.p.Readings++
			r.p.Borrowed = r.p.Borrowed || strings.Contains(d.Text, "用完记得要还我")
			r.p.RodReturned = r.p.RodReturned || strings.Contains(d.Text, "钓竿还您")
			r.p.DeerCaught = r.p.DeerCaught || strings.Contains(d.Text, "鹿兄啊")
			r.p.YueReturned = r.p.YueReturned || strings.Contains(d.Text, "三样药材我已弄到手")
			r.p.Brewed = r.p.Brewed || strings.Contains(d.Text, "要趁热喝")
			r.p.Cured = r.p.Cured || strings.Contains(d.Text, "赵姑娘已经可以下床走动了")
			r.p.Temple = r.p.Temple || r.p.Cured && strings.Contains(d.Text, "玉佛寺")
			r.p.TempleStory = r.p.TempleStory || r.p.Cured && strings.Contains(d.Text, "都出家当了和尚")
			r.t.Log("river medicine reading", r.p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		for _, mark := range []struct {
			name string
			done bool
		}{{"rod-borrow-reading", r.p.Borrowed}, {"rod-return-reading", r.p.RodReturned}, {"deer-reading", r.p.DeerCaught}, {"yue-herbs-reading", r.p.YueReturned}, {"brew-reading", r.p.Brewed}, {"cure-reading", r.p.Cured}, {"temple-reading", r.p.Temple}} {
			if mark.done && !r.p.Saved[mark.name] {
				r.save(mark.name)
			}
		}
		v = r.visible()
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
	}
	r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
	var err error
	r.g, err = r.m.OnTimer(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
}

func (r *riverMedicineDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 2048; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			return v
		}
		r.progress()
	}
	r.t.Fatal("original medicine flow did not return to the field", r.visible().Phase)
	return pal.View{}
}

func (r *riverMedicineDriver) walk(scene, x, y, reach int) {
	r.t.Helper()
	visits := map[[2]int]int{}
	for i := 0; i < 1200; i++ {
		v := r.field()
		if v.Scene != scene {
			return
		}
		if fieldTestDistance(v.X, v.Y, x, y) <= reach {
			return
		}
		place := [2]int{v.X, v.Y}
		visits[place]++
		if visits[place] > 32 {
			r.t.Fatal("medicine waypoint repeats a blocked position", scene, x, y, v.X, v.Y)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, reach)
		r.progress()
		if i%100 == 0 {
			r.t.Log("river medicine walk", i, scene, v.X, v.Y, x, y)
		}
	}
	r.t.Fatal("medicine waypoint was not reached", scene, x, y, r.visible().X, r.visible().Y)
}

func (r *riverMedicineDriver) portal(from, to, x, y int) {
	r.t.Helper()
	v := r.field()
	if v.Scene != from {
		r.t.Fatal("wrong medicine portal source", from, v.Scene)
	}
	r.walk(from, x, y, 0)
	v = r.field()
	if v.Scene != to {
		r.t.Fatal("original medicine portal failed", from, to, v.Scene, v.X, v.Y)
	}
}

func (r *riverMedicineDriver) talk(scene, x, y, npc int, done func() bool) {
	r.t.Helper()
	for i := 0; i < 500 && !done(); i++ {
		v := r.field()
		if done() {
			break
		}
		if v.Scene != scene {
			r.t.Fatal("medicine NPC scene changed", scene, v.Scene)
		}
		if fieldTestDistance(v.X, v.Y, x, y) > 160 {
			r.walk(scene, x, y, 96)
		} else {
			r.g = campaignWalkTo(r.t, r.m, &r.c, r.g, v, x, y, npc)
			r.progress()
		}
	}
	if !done() {
		r.t.Fatal("original medicine NPC did not reach expected dialogue", scene, npc)
	}
	r.field()
}

func (r *riverMedicineDriver) menu() pal.View {
	r.t.Helper()
	for i := 0; i < 128; i++ {
		v := r.visible()
		if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
			return v
		}
		r.progress()
	}
	r.t.Fatal("original ingredient inventory did not open", r.visible().Phase, r.visible().Menu)
	return pal.View{}
}

func (r *riverMedicineDriver) inventory(item string, selectUsable bool) {
	r.t.Helper()
	r.field()
	r.keys(2048, 4)
	r.menu()
	r.keys(65536, 4)
	r.progress()
	r.progress()
	for i := 0; i < 96; i++ {
		v := r.menu()
		n, ok := campaignInventoryAmount(v, item)
		if ok && (!selectUsable || strings.Contains(originalVisibleText(v, true), item)) {
			if n != 1 {
				r.t.Fatal("original medicine item missing/duplicated", item, n)
			}
			if !selectUsable {
				r.keys(1, 4)
				r.field()
			}
			return
		}
		r.keys(32, 4)
		r.progress()
		r.progress()
	}
	r.t.Fatal("original medicine item was not found", item, originalVisibleText(r.visible(), false))
}

func (r *riverMedicineDriver) absent(items ...string) {
	r.t.Helper()
	r.field()
	r.keys(2048, 4)
	r.menu()
	r.keys(65536, 4)
	r.progress()
	r.progress()
	previous := ""
	for i := 0; i < 32; i++ {
		v := r.menu()
		for _, item := range items {
			if n, ok := campaignInventoryAmount(v, item); ok && n > 0 {
				r.t.Fatal("original ingredient/tool was not consumed/returned", item, n)
			}
		}
		page := originalVisibleText(v, false)
		if page == previous {
			r.keys(1, 4)
			r.field()
			return
		}
		previous = page
		r.keys(128, 4)
		r.progress()
		r.progress()
	}
	r.t.Fatal("ingredient absence scan did not reach the original last page")
}

func (r *riverMedicineDriver) westPose(scene, x, y int) {
	r.t.Helper()
	r.walk(scene, x+16, y+8, 0)
	v := r.field()
	r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, Direction: 1, Keys: 8})
	r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 2, Direction: 4})
	r.progress()
	v = r.field()
	if v.X != x || v.Y != y || v.Field.Direction != 1 {
		r.t.Fatal("ordinary west-facing medicine pose failed", scene, x, y, v.X, v.Y, v.Field.Direction)
	}
}

func TestOriginalWhiteRiverFishingAndReturnRodSave(t *testing.T) {
	r := loadRiverMedicine(t, "campaign-white-river-prescription-state.json")
	defer r.failure()
	runRiverFishing(r)
}

func runRiverFishing(r *riverMedicineDriver) {
	t := r.t
	t.Helper()
	v := r.field()
	if v.Scene != 53 || len(v.Party) != 1 || v.Party[0].Role != 0 {
		t.Fatal("not the genuine original prescription continuation", v.Scene, v.Party)
	}
	r.portal(53, 50, 384, 1280)
	r.portal(50, 52, 896, 1168)
	r.talk(52, 800, 336, 877, func() bool { return r.p.Borrowed })
	r.inventory("钓竿", false)
	r.save("rod-borrowed")
	r.portal(52, 50, 800, 480)
	r.westPose(50, 272, 1400)
	r.inventory("钓竿", true)
	r.save("fishing-selected")
	r.keys(2, 4)
	r.progress()
	r.field()
	r.inventory("鲤鱼", false)
	r.inventory("钓竿", false)
	r.p.FishCaught = true
	r.save("fish-caught")
	r.portal(50, 52, 896, 1168)
	r.talk(52, 800, 336, 877, func() bool { return r.p.RodReturned })
	r.absent("钓竿")
	r.inventory("鲤鱼", false)
	r.save("fishing-complete")
	for name, data := range map[string][]byte{"campaign-white-river-fishing-state.json": r.g.Data, "campaign-white-river-fishing.json": raw(map[string]any{"passed": true, "scope": "actual original borrowed fishing rod, native target/animation, one live carp, original tool return and fresh-runtime selected-menu/reading saves; not complete story stage 6", "proof": r.p, "party": r.visible().Party, "scene": r.visible().Scene})} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOriginalWhiteRiverTrapMedicineAndThreePartySave(t *testing.T) {
	r := loadRiverMedicine(t, "campaign-white-river-fishing-state.json")
	defer r.failure()
	b, err := os.ReadFile(campaignEvidencePath("campaign-white-river-fishing.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Proof riverMedicineProof `json:"proof"`
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	r.p = report.Proof
	runRiverMedicine(r)
}

func TestOriginalWhiteRiverMedicineRejectsWrongFacing(t *testing.T) {
	r := loadRiverMedicine(t, "white-river-medicine-selected-state.json")
	defer r.failure()
	before := r.visible()
	r.keys(1, 4)
	r.progress()
	r.field()
	r.keys(32, 3)
	r.keys(0, 4)
	r.progress()
	v := r.field()
	if v.Field.Direction != 3 {
		t.Fatal("ordinary wrong-facing setup failed", v.Field.Direction)
	}
	r.inventory("六神丹", true)
	r.keys(2, 4)
	r.progress()
	r.field()
	r.inventory("六神丹", false)
	v = r.field()
	if r.p.Cured || len(v.Party) != 2 || !bytes.Equal(raw(before.Party), raw(v.Party)) || v.Scene != 54 {
		t.Fatal("wrong-facing medicine advanced treatment or changed resources", v.Party, r.p)
	}
	report := raw(map[string]any{"passed": true, "scope": "ordinary original six-god medicine used facing away from Ling'er; exactly one dose remains, original party/resources and story remain unchanged", "medicineStillOne": true, "noTreatmentOrPartyAdvance": true})
	if err := os.WriteFile(campaignEvidencePath("white-river-medicine-wrong-facing.json", 1), report, 0600); err != nil {
		t.Fatal(err)
	}
}

func runRiverMedicine(r *riverMedicineDriver) {
	t := r.t
	t.Helper()
	v := r.field()
	if v.Scene != 52 || len(v.Party) != 1 || !r.p.FishCaught || !r.p.RodReturned {
		t.Fatal("not the actual carp/tool-return continuation", v.Scene, v.Party, r.p)
	}
	r.portal(52, 50, 800, 480)
	r.portal(50, 49, 528, 1656)
	r.westPose(49, 800, 1024)
	r.keys(2, 4)
	r.progress()
	r.field()
	r.inventory("捕兽夹", false)
	r.p.TrapPicked = true
	r.save("trap-picked")
	// The first two landings block narrow passages. Trigger the second
	// escape normally, then lay the trap on the reachable return path.
	// This walkthrough never moves the deer or its trap through native memory.
	r.w.allowTouch = true
	r.walk(49, 576, 864, 96)
	for i := 0; i < 128; i++ {
		r.progress()
	}
	r.w.avoidTouches = map[int][][3]int{49: {{944, 504, 160}}}
	r.westPose(49, 800, 544)
	r.inventory("捕兽夹", true)
	r.save("trap-selected")
	r.keys(2, 4)
	r.progress()
	v = r.field()
	planted := false
	for _, a := range v.Field.Actors {
		planted = planted || a[0] == 1 && a[1] == 798 && a[3] == 784 && a[4] == 536
	}
	if !planted {
		t.Fatal("original trap was not placed on the ordinary facing tile", v.X, v.Y, v.Field.Actors)
	}
	r.p.TrapPlaced = true
	r.absent("捕兽夹")
	r.save("trap-placed")
	r.w.avoidTouches = nil
	for _, landing := range [][2]int{{944, 504}, {1216, 544}, {1088, 400}, {1216, 320}} {
		r.walk(49, landing[0], landing[1], 96)
		for i := 0; i < 128; i++ {
			r.progress()
		}
	}
	r.walk(49, 800, 544, 96)
	trapped := false
	for i := 0; i < 256; i++ {
		v = r.field()
		for _, a := range v.Field.Actors {
			if a[0] == 1 && a[1] == 797 && a[5] == 16 {
				trapped = true
				r.p.CaughtAt = [2]int{a[3], a[4]}
			}
		}
		if trapped {
			break
		}
		r.progress()
	}
	if !trapped {
		t.Fatal("original deer did not enter its ordinary trap", r.visible().Field)
	}
	r.save("deer-trapped")
	// The original actor may take its final movement step as the trap fires.
	// Approach the actually displayed captured deer, then search its adjacent
	// tile (mode 1), instead of assuming it stays on the trap's original cell.
	r.walk(49, r.p.CaughtAt[0]+16, r.p.CaughtAt[1]+8, 0)
	r.keys(8, 1)
	r.progress()
	r.field()
	r.keys(2, 4)
	r.progress()
	r.field()
	if !r.p.DeerCaught {
		t.Fatal("captured original deer did not grant its dialogue", r.visible().Field)
	}
	r.inventory("鹿茸", false)
	r.inventory("鲤鱼", false)
	r.inventory("银杏果", false)
	r.p.Antler = true
	r.save("antler-collected")
	r.portal(49, 50, 1824, 1504)
	r.portal(50, 53, 1632, 704)
	r.talk(53, 896, 880, 888, func() bool { return r.p.YueReturned })
	r.save("six-herbs-ready")
	r.talk(53, 848, 888, 898, func() bool { return r.p.Brewed })
	v = r.field()
	if len(v.Party) != 2 || v.Party[0].Role != 0 || v.Party[1].Role != 2 {
		t.Fatal("original brewing did not restore Yueru", v.Party)
	}
	r.inventory("六神丹", false)
	r.absent("银杏果", "鲤鱼", "鹿茸")
	r.save("medicine-brewed")
	r.portal(53, 54, 864, 816)
	// The bed blocks the closest cell. This reachable original facing pose
	// is within the actual item script's search zone; no scene/item mutation.
	r.westPose(54, 1488, 600)
	r.inventory("六神丹", true)
	r.save("medicine-selected")
	r.keys(2, 4)
	r.progress()
	v = r.field()
	if !r.p.Cured || !r.p.Temple || !r.p.TempleStory || v.Scene != 54 || len(v.Party) != 3 || v.Party[0].Role != 0 || v.Party[1].Role != 1 || v.Party[2].Role != 2 {
		t.Fatal("original treatment/three-party/temple story incomplete", v.Scene, v.Party, r.p)
	}
	for _, p := range v.Party {
		if p.Controller != 0 {
			t.Fatal("treated party is not manually owned by the solo player", v.Party)
		}
	}
	r.absent("六神丹", "银杏果", "鲤鱼", "鹿茸", "钓竿")
	r.save("medicine-complete")
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original borrowed rod/fishing/return, original deer trap and antler release, Yueru's three-herb dialogue and rejoin, actual ingredient consumption and six-god medicine, ordinary treatment of Ling'er, manual original three-role party and Jade Buddha Temple story; fresh-runtime selected item/reading/field saves", "scene": v.Scene, "map": v.Map, "party": v.Party, "proof": r.p, "oldInputRejected": true}, "", "  ")
	for name, data := range map[string][]byte{"campaign-white-river-state.json": r.g.Data, "campaign-white-river.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
