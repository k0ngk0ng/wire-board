package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type sacredTreeChecks struct {
	InventoryBefore, InventoryAfterPhoenix, InventoryAfterCave map[string]int
	PhoenixObserved                                            bool
	PhoenixAcceptedSaved                                       bool
	EmptyRepeatSearches                                        int
	LastEmptyPrompt                                            uint64
	PartyOrders                                                map[string][]int
	LiftsUsed                                                  map[int]bool
	BananasPicked                                              bool
	BananaRepeatSearches                                       int
	MonkeysFed                                                 map[int]bool
	MonkeyChildrenSeen                                         map[int]bool
}

type sacredTreeLift struct {
	Scene, Event, Radius int
	Entry, Destination   [2]int
}

func sacredTreeTouchArea(areas map[[2]int][2]int, entry, destination [2]int, radius int) {
	for x := entry[0]/16*16 - radius; x <= entry[0]+radius; x += 16 {
		for y := entry[1]/8*8 - radius/2; y <= entry[1]+radius/2; y += 8 {
			if x < 0 || y < 0 || (x/16-y/8)%2 != 0 || fieldTestDistance(x, y, entry[0], entry[1]) >= radius {
				continue
			}
			at := [2]int{x, y}
			if _, exists := areas[at]; !exists {
				areas[at] = destination
			}
		}
	}
}

func sacredTreeLifts(t *testing.T, tables *resources.Tables) []sacredTreeLift {
	t.Helper()
	lifts := []sacredTreeLift{}
	for _, scene := range []int{186, 193} {
		for i := tables.Scenes[scene-1].FirstEvent; i < tables.Scenes[scene].FirstEvent; i++ {
			e := tables.Events[i]
			if e.State <= 0 || e.Sprite != 501 || e.TriggerMode < 4 {
				continue
			}
			for pc := int(e.TriggerScript); pc < int(e.TriggerScript)+32; pc++ {
				s := tables.Scripts[pc]
				if s.Opcode <= 3 {
					break
				}
				if s.Opcode == 0x44 {
					lifts = append(lifts, sacredTreeLift{Scene: scene, Event: int(i) + 1, Radius: (int(e.TriggerMode)-4)*32 + 16,
						Entry:       [2]int{int(e.X), int(e.Y)},
						Destination: [2]int{int(s.Operands[0])*32 + int(s.Operands[2])*16, int(s.Operands[1])*16 + int(s.Operands[2])*8}})
				}
			}
		}
	}
	if len(lifts) < 2 {
		t.Fatal("original plant lift references missing", lifts)
	}
	return lifts
}

var sacredTreeReadings = map[string]string{
	"bananas": "摘得两支香蕉", "feed-first": "来～来～给你香蕉",
	"feed-second": "你也要香蕉，对吧",
	"wind-orb":    "得到风灵珠", "egg-stolen": "你才是小偷呢",
	"rescued": "好心救你", "egg-kept": "这颗蛋暂时由我保管",
	"anu-agreement": "一言为定，跟我走", "tree-cave": "今天我瞧见那魔头外出",
	"empty-chest": "可恶．．是空的", "water-search": "我在找水灵珠",
	"orb-lore": "骨骸炼化成五颗灵珠", "drought": "已经打了九年的战争",
	"treasure-intent": "先把这里的宝物全拿走再说",
}

var sacredTreeSteps = []struct {
	Scene, Event, Dest int
	Token              string
	Party              []int
}{
	{175, 2869, 177, "", []int{0}},
	{177, 2932, 177, "bananas", []int{0}},
	{177, 2916, 187, "", []int{0}},
	{187, 3133, 193, "", []int{0}},
	{193, 3298, 186, "", []int{0}},
	{186, 3116, 189, "anu-agreement", []int{4, 0}},
	{189, 3156, 185, "tree-cave", []int{4, 0}},
	{185, 3014, 185, "treasure-intent", []int{0, 4}},
	{185, 3011, 188, "", []int{0, 4}},
	{188, 3151, 187, "", []int{0, 4}},
	{187, 3132, 180, "", []int{0, 4}},
}

func sacredTreeField(r *caiyiForestDriver) pal.View {
	r.t.Helper()
	v := r.field()
	if r.notice != nil {
		r.notice(v)
	}
	return r.visible()
}

func sacredTreeClimbs(t *testing.T, tables *resources.Tables) map[int]map[[2]int][2]int {
	t.Helper()
	moves := map[int]map[[2]int][2]int{}
	rows := []map[string]any{}
	counts := map[int]int{}
	for _, scene := range []int{186, 193} {
		moves[scene] = map[[2]int][2]int{}
		for i := tables.Scenes[scene-1].FirstEvent; i < tables.Scenes[scene].FirstEvent; i++ {
			e := tables.Events[i]
			if e.State <= 0 || e.TriggerMode < 4 || e.TriggerScript == 0 {
				continue
			}
			var last *resources.Script
			supported := true
			for pc := int(e.TriggerScript); pc < len(tables.Scripts) && pc < int(e.TriggerScript)+16; pc++ {
				s := tables.Scripts[pc]
				if s.Opcode <= 3 {
					break
				}
				switch s.Opcode {
				case 0x7a, 0x7b:
					value := s
					last = &value
				case 0x46, 0x7f, 0x9, 0x6e:
				default:
					supported = false
				}
			}
			if !supported || last == nil || last.Operands[2] > 1 {
				continue
			}
			entry := [2]int{int(e.X), int(e.Y)}
			destination := [2]int{int(last.Operands[0])*32 + int(last.Operands[2])*16, int(last.Operands[1])*16 + int(last.Operands[2])*8}
			radius := (int(e.TriggerMode)-4)*32 + 16
			sacredTreeTouchArea(moves[scene], entry, destination, radius)
			counts[scene]++
			rows = append(rows, map[string]any{"scene": scene, "event": int(i) + 1, "originalScript": e.TriggerScript, "entry": entry, "radius": radius, "destination": destination})
		}
	}
	if counts[193] != 52 || counts[186] != 66 {
		t.Fatal("original sacred tree climb reference is incomplete", counts)
	}
	if err := os.WriteFile(campaignEvidencePath("sacred-tree-climb-reference.json", 1), raw(map[string]any{"scope": "read-only original same-scene climb planning; tests still send ordinary movement and let native scripts move party/camera; never bot policy or native edits", "moves": rows}), 0600); err != nil {
		t.Fatal(err)
	}
	return moves
}

func TestOriginalSacredTreePhoenixAnuCaveAndSave(t *testing.T) {
	if os.Getenv("PAL_SACRED_TREE") != "1" {
		t.Skip("explicit actual sacred tree full-story acceptance")
	}
	root, err := os.ReadFile(campaignEvidencePath("campaign-tower-bottom-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-tower-bottom-state.json", root, "ca57221386ee71293f13de95651842e452651968b7bef8e4f2fbe5af0bee5650"); err != nil {
		t.Fatal(err)
	}
	checkpoint := os.Getenv("PAL_SACRED_TREE_CHECKPOINT")
	checks := sacredTreeChecks{PartyOrders: map[string][]int{}, LiftsUsed: map[int]bool{}, MonkeysFed: map[int]bool{}, MonkeyChildrenSeen: map[int]bool{}}
	var r *caiyiForestDriver
	name := "campaign-tower-bottom-state.json"
	if checkpoint == "" {
		r = runSageDream(t, "sacred-tree")
	} else {
		if checkpoint != "failure" {
			n, err := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if err != nil || n < 0 || n > len(sacredTreeSteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported sacred tree debug checkpoint", checkpoint)
			}
		}
		name = "sacred-tree-" + checkpoint + "-state.json"
		p := caiyiForestProof{}
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, err := os.ReadFile(campaignEvidencePath("sacred-tree-"+checkpoint+"-"+kind+".json", 1))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, target); err != nil {
				t.Fatal(err)
			}
		}
		r = &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "sacred-tree", proof: p}
	}
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r.tables, r.prefix, r.milestones = tables, "sacred-tree", sacredTreeReadings
	r.allowedRoles = map[int]bool{0: true, 4: true}
	r.reserveScenes = []int{175, 177, 187, 193, 186, 189, 185, 188, 180}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	r.w.sameSceneMoves = sacredTreeClimbs(t, tables)
	climbs := r.w.sameSceneMoves
	lifts := sacredTreeLifts(t, tables)
	if checks.LiftsUsed == nil {
		checks.LiftsUsed = map[int]bool{}
	}
	if checks.MonkeysFed == nil {
		checks.MonkeysFed = map[int]bool{}
	}
	if checks.MonkeyChildrenSeen == nil {
		checks.MonkeyChildrenSeen = map[int]bool{}
	}
	refreshMoves := func() {
		r.w.sameSceneMoves = map[int]map[[2]int][2]int{}
		for scene, areas := range climbs {
			r.w.sameSceneMoves[scene] = map[[2]int][2]int{}
			for entry, destination := range areas {
				r.w.sameSceneMoves[scene][entry] = destination
			}
		}
		for _, lift := range lifts {
			if !checks.LiftsUsed[lift.Event] {
				sacredTreeTouchArea(r.w.sameSceneMoves[lift.Scene], lift.Entry, lift.Destination, lift.Radius)
			}
		}
	}
	refreshMoves()
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("sacred-tree-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	observations := 0
	bananaNotices := []map[string]any{}
	lastField := [2]int{r.visible().X, r.visible().Y}
	r.notice = func(v pal.View) {
		observations++
		if v.Scene == 177 && r.proof.Step == 1 && v.Phase != "menu" {
			text := hiddenNoticeText(v)
			if text != "" && len(bananaNotices) < 256 {
				bananaNotices = append(bananaNotices, map[string]any{"phase": v.Phase, "allGlyphs": text, "coloredGlyphs": originalVisibleText(v, false), "dialogue": v.Dialogue})
			}
		}
		onLift := false
		for _, draw := range v.Draws {
			onLift = onLift || draw.Kind == 0 && draw.A == 541 && draw.C == 0
			if v.Scene == 193 && draw.Kind == 0 && draw.D == 1 && draw.C >= 3303 && draw.C <= 3305 {
				checks.MonkeyChildrenSeen[draw.C] = true
				if !r.proof.Saved["monkey-children-birth"] {
					save("monkey-children-birth")
				}
			}
		}
		if onLift {
			for _, lift := range lifts {
				if lift.Scene == v.Scene && !checks.LiftsUsed[lift.Event] && fieldTestDistance(lastField[0], lastField[1], lift.Entry[0], lift.Entry[1]) < 160 {
					checks.LiftsUsed[lift.Event] = true
					refreshMoves()
					save(fmt.Sprint("plant-lift-", lift.Event))
					t.Log("original plant ride saved", lift.Event, lastField, lift.Destination)
				}
			}
		}
		if v.Phase == "field" {
			lastField = [2]int{v.X, v.Y}
		}
		if d := v.Dialogue; d != nil && v.Scene == 185 && strings.Contains(d.Text, "一个空箱子") && d.ID != checks.LastEmptyPrompt {
			checks.LastEmptyPrompt = d.ID
			checks.EmptyRepeatSearches++
		}
		if v.Scene == 193 && v.Phase == "script" && !r.proof.Saved["original-climb-in-progress"] {
			save("original-climb-in-progress")
		}
		roles := []int{}
		for _, p := range v.Party {
			roles = append(roles, p.Role)
		}
		valid := bytes.Equal(raw(roles), raw([]int{0})) || bytes.Equal(raw(roles), raw([]int{4, 0})) || bytes.Equal(raw(roles), raw([]int{0, 4}))
		if !valid || r.proof.Step < 5 && len(roles) != 1 {
			t.Fatal("original sacred tree temporary party changed", r.proof.Step, v.Scene, roles)
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			label := fmt.Sprint("field-", v.Scene, "-step-", r.proof.Step)
			if !r.proof.Saved[label] {
				checks.PartyOrders[label] = roles
				save(label)
			}
		}
		if b := v.Battle; b != nil {
			r.battlePartySize = 1
			if r.proof.Step > 5 {
				r.battlePartySize = 2
			}
			if len(roles) != r.battlePartySize {
				t.Fatal("sacred tree battle party changed", r.proof.Step, roles)
			}
			alive := false
			for _, p := range v.Party {
				alive = alive || p.HP > 0
			}
			if !alive {
				t.Fatal("original sacred tree battle defeated the whole party", observations, v.Scene, v.Party)
			}
			for _, e := range b.Enemies {
				if e.Object == 464 {
					if v.Scene != 186 || !bytes.Equal(raw(roles), raw([]int{0})) {
						t.Fatal("original golden phoenix battle changed", v.Scene, roles)
					}
					checks.PhoenixObserved = true
					for _, p := range b.Players {
						if p.State == 2 && !checks.PhoenixAcceptedSaved {
							checks.PhoenixAcceptedSaved = true
							save("phoenix-accepted")
						}
					}
				}
			}
			if observations%250 == 0 {
				t.Log("sacred tree public battle", observations, r.proof.Step, v.Party, b)
			}
		}
	}
	feedMonkey := func(event, expected int, token string) bool {
		v := sacredTreeField(r)
		x, y := xFrom(tables, event), yFrom(tables, event)
		delta := [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}}[v.Field.Direction]
		px, py := v.X+delta[0], v.Y+delta[1]
		facing := px/32 == x/32 && py/16 == y/16 && (px%32 != 0) == (x%32 != 0)
		if !facing {
			if fieldTestDistance(v.X, v.Y, x, y) > 96 {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 64)
			} else {
				r.search(event, x, y)
			}
			r.progress()
			return false
		}
		r.keys(2048, 4)
		for i := 0; i < 128 && (r.visible().Menu.Kind != 5 || r.visible().Seq != r.visible().AppliedSeq); i++ {
			r.progress()
		}
		if r.visible().Menu.Kind != 5 {
			t.Fatal("original banana item menu did not open", event)
		}
		r.keys(65536, 4)
		r.progress()
		r.progress()
		selected := false
		for i := 0; i < 128; i++ {
			v := r.visible()
			if strings.Contains(originalVisibleText(v, true), "香蕉") {
				amount, ok := campaignInventoryAmount(v, "香蕉")
				if !ok || amount != expected {
					t.Fatal("original banana quantity before monkey changed", event, expected, amount)
				}
				selected = true
				break
			}
			r.keys(32, 4)
			r.progress()
			r.progress()
		}
		if !selected {
			t.Fatal("earned bananas not found in the original menu", event)
		}
		save(fmt.Sprint("banana-selected-", event))
		r.keys(2, 4)
		for i := 0; i < 1200; i++ {
			v := r.visible()
			if v.Phase == "field" && v.Seq == v.AppliedSeq {
				break
			}
			if v.Phase == "menu" && v.Menu.Kind == 5 && v.Seq == v.AppliedSeq {
				r.keys(1, 4)
			}
			r.progress()
		}
		v = sacredTreeField(r)
		if !r.proof.Seen[token] || r.inventoryCounts("香蕉")["香蕉"] != expected-1 {
			t.Fatal("original banana did not feed its intended monkey", event, token, r.proof.Seen)
		}
		present := false
		for _, actor := range v.Field.Actors {
			present = present || actor[0] == 1 && actor[1] == event
		}
		if event == 3301 && present || event == 3302 && !present {
			t.Fatal("original fed monkey appearance changed", event, present)
		}
		u, z := ((x-v.X)/16+(y-v.Y)/8)/2, ((y-v.Y)/8-(x-v.X)/16)/2
		if u < -8 || u > 8 || z < -8 || z > 8 || v.Field.Cells[(z+8)*17+u+8]&1 != 0 {
			t.Fatal("original fed monkey still blocks its cell", event)
		}
		checks.MonkeysFed[event] = true
		save(fmt.Sprint("monkey-", event, "-fed"))
		return true
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible()), "banana-notices": raw(bananaNotices)} {
				if err := os.WriteFile(campaignEvidencePath("sacred-tree-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	if checkpoint == "" {
		sacredTreeField(r)
		checks.InventoryBefore = r.inventoryCounts("风灵珠", "水灵珠", "金凤凰蛋壳", "天书")
		save("step-0")
	}
	for r.proof.Step < len(sacredTreeSteps) {
		q := sacredTreeSteps[r.proof.Step]
		t.Log("original sacred tree waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 3000; i++ {
			v := sacredTreeField(r)
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("original sacred tree route entered an unexpected field", q, v.Scene, v.X, v.Y)
			}
			if v.Scene == 193 && !checks.MonkeysFed[3301] {
				feedMonkey(3301, 2, "feed-first")
				continue
			}
			if v.Scene == 193 && !checks.MonkeysFed[3302] {
				feedMonkey(3302, 1, "feed-second")
				continue
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 50 {
				t.Fatal("original sacred tree approach blocked", q, at)
			}
			x, y := xFrom(tables, q.Event), yFrom(tables, q.Event)
			if q.Event == 2932 && fieldTestDistance(v.X, v.Y, x, y) <= 96 {
				r.search(q.Event, x, y)
			} else {
				reach := 0
				if q.Event == 2932 {
					reach = 96
				}
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, reach)
			}
			if os.Getenv("PAL_SACRED_TREE_PLAN_ONLY") == "1" {
				t.Skip("captured original public route planning only; not gameplay acceptance")
			}
			r.progress()
		}
		v := sacredTreeField(r)
		roles := []int{}
		for _, p := range v.Party {
			roles = append(roles, p.Role)
		}
		if v.Scene != q.Dest || q.Token != "" && !r.proof.Seen[q.Token] || !bytes.Equal(raw(roles), raw(q.Party)) {
			t.Fatal("original sacred tree waypoint or party incomplete", q, v.Scene, roles)
		}
		if q.Event == 2932 {
			if r.inventoryCounts("香蕉")["香蕉"] != 2 {
				t.Fatal("original banana tree did not grant exactly two bananas")
			}
			checks.BananasPicked = true
			save("bananas-collected")
			for repeat := 0; repeat < 3; repeat++ {
				r.keys(2, 4)
				r.progress()
				sacredTreeField(r)
				checks.BananaRepeatSearches++
			}
			if r.inventoryCounts("香蕉")["香蕉"] != 2 {
				t.Fatal("original banana tree duplicated after repeated searches")
			}
		}
		if q.Event == 3116 {
			checks.InventoryAfterPhoenix = r.inventoryCounts("风灵珠", "水灵珠", "金凤凰蛋壳", "天书")
			if !checks.PhoenixObserved || !checks.PhoenixAcceptedSaved || checks.InventoryAfterPhoenix["风灵珠"] != 1 || checks.InventoryAfterPhoenix["金凤凰蛋壳"] != 0 {
				t.Fatal("original phoenix reward/Anu custody changed", checks)
			}
		}
		if q.Event == 3014 {
			checks.InventoryAfterCave = r.inventoryCounts("风灵珠", "水灵珠", "金凤凰蛋壳", "天书")
			if !bytes.Equal(raw(checks.InventoryAfterPhoenix), raw(checks.InventoryAfterCave)) {
				t.Fatal("original empty tree-cave chest fabricated sacred items", checks)
			}
			for i := 0; i < 180 && checks.EmptyRepeatSearches < 3; i++ {
				r.search(3013, xFrom(tables, 3013), yFrom(tables, 3013))
				r.progress()
				sacredTreeField(r)
			}
			if checks.EmptyRepeatSearches != 3 {
				t.Fatal("original empty chest did not produce three genuine repeated searches", checks.EmptyRepeatSearches)
			}
			if !bytes.Equal(raw(checks.InventoryAfterCave), raw(r.inventoryCounts("风灵珠", "水灵珠", "金凤凰蛋壳", "天书"))) {
				t.Fatal("original empty chest changed inventory after repeated searches")
			}
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	v := sacredTreeField(r)
	for name := range sacredTreeReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original sacred tree milestone missing", name, r.proof.Seen)
		}
	}
	if v.Scene != 180 || v.Map != 149 || r.proof.Battles == 0 || r.proof.Battles != r.proof.Wins || checks.EmptyRepeatSearches != 3 || !r.proof.Saved["original-climb-in-progress"] || len(checks.LiftsUsed) == 0 || !checks.BananasPicked || len(checks.MonkeysFed) != 2 || checks.BananaRepeatSearches != 3 || len(checks.MonkeyChildrenSeen) != 3 || !r.proof.Saved["monkey-children-birth"] {
		t.Fatal("original sacred tree ending incomplete", v.Scene, v.Map, r.proof, checks)
	}
	hash := sha256.Sum256(source)
	report := raw(map[string]any{"passed": true, "scope": "original sage dream/treatment and ordered memories, sacred tree route, solo golden phoenix and wind orb, Anu theft/rescue/custody and joining, tree-cave empty chest and original orb/drought lore, original party orders and fresh-runtime saves; no native edits or premature sacred egg/water orb grant", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "rootStage17SHA256": hex.EncodeToString(rootHash[:]), "scene": v.Scene, "map": v.Map, "party": v.Party, "cash": v.Cash, "checks": checks, "proof": r.proof})
	for name, data := range map[string][]byte{"campaign-sacred-tree.json": report, "campaign-sacred-tree-state.json": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
