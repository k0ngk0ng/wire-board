package pal_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type towerRewardCandidate struct {
	Scene, Map, Event, X, Y, Sprite, Script int
	Grants                                  []struct {
		PC, Item, Amount, Cash int
		Name                   string
	}
	Messages []string
}

func towerPreservedUnreachable(t *testing.T, v pal.View, candidate towerRewardCandidate) (map[string]any, bool) {
	t.Helper()
	if candidate.Event != 2576 {
		return nil, false
	}
	var catalog struct {
		Version int
		Records map[string]struct {
			Scene, Map, Event, X, Y, Item, Script                                                   int
			MapSHA256, ReferenceURL, ReferenceText, ReferenceSHA256, ReferenceQuote, Classification string
		}
	}
	raw, err := os.ReadFile("original-reward-exceptions.json")
	if err != nil || json.Unmarshal(raw, &catalog) != nil || catalog.Version != 1 || len(catalog.Records) != 1 {
		t.Fatal("original unreachable reward record missing or invalid", err)
	}
	record, ok := catalog.Records[strconv.Itoa(candidate.Event)]
	if !ok || record.Classification != "original-unreachable" || record.Scene != v.Scene || record.Map != v.Map || record.Event != candidate.Event || record.X != candidate.X || record.Y != candidate.Y || record.Script != candidate.Script || len(candidate.Grants) != 1 || record.Item != candidate.Grants[0].Item || record.ReferenceURL != "https://gamegene.cn/wiki/363" || record.ReferenceText != ".local/pal/hidden-items-research/tower-text-guide.md" || record.ReferenceQuote != "化尸水（拿不到）" {
		t.Fatal("unreviewed original reward cannot be classified as unreachable", candidate.Event)
	}
	text, err := os.ReadFile("../../../" + record.ReferenceText)
	textSHA := sha256.Sum256(text)
	if err != nil || hex.EncodeToString(textSHA[:]) != record.ReferenceSHA256 || !bytes.Contains(text, []byte(record.ReferenceQuote)) {
		t.Fatal("original unreachable reward requires its retained textual evidence", err)
	}
	compressed, err := os.ReadFile(fmt.Sprintf("../../../.local/pal/visual/pal/v1/maps/%d.bin.gz", v.Map))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	ground, err := io.ReadAll(io.LimitReader(reader, 65537))
	reader.Close()
	mapSHA := sha256.Sum256(ground)
	if err != nil || len(ground) != 65536 || hex.EncodeToString(mapSHA[:]) != record.MapSHA256 || v.Field == nil {
		t.Fatal("original unreachable reward map changed", err)
	}
	type point struct{ x, y int }
	goals := map[point]bool{{candidate.X, candidate.Y}: true}
	directions := []point{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}}
	for _, d := range directions {
		goals[point{candidate.X - d.x, candidate.Y - d.y}] = true
	}
	start := point{v.X, v.Y}
	queue := []point{start}
	seen := map[point]bool{start: true}
	for at := 0; at < len(queue); at++ {
		p := queue[at]
		if goals[p] {
			t.Fatal("preserved reward gained an ordinary static route; classification requires a new review", candidate.Event, p)
		}
		for _, d := range directions {
			next := point{p.x + d.x, p.y + d.y}
			if seen[next] || next.x < v.Field.Offset[0]/32*32 || next.x/32 >= 63-v.Field.Offset[0]/32 || next.y < v.Field.Offset[1]/16*16 || next.y >= 2048 {
				continue
			}
			index := ((next.y/16*64+next.x/32)*2 + next.x%32/16) * 4
			if binary.LittleEndian.Uint32(ground[index:])&8192 != 0 {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return map[string]any{"event": record.Event, "scene": record.Scene, "map": record.Map, "position": [2]int{record.X, record.Y}, "item": candidate.Grants[0].Name, "status": record.Classification, "normalPickupVerified": false, "nativeRecordPreserved": true, "optimisticWalkablePositions": len(seen), "mapSHA256": record.MapSHA256, "referenceURL": record.ReferenceURL, "referenceSHA256": record.ReferenceSHA256}, true
}

func TestOriginalTowerOptionalChestsAndSavedNoDuplication(t *testing.T) {
	requested := os.Getenv("PAL_TOWER_REWARDS_SCENE")
	if requested == "" {
		t.Skip("explicit original optional tower chest exploration")
	}
	scene, err := strconv.Atoi(requested)
	entries := map[int][2]int{166: {2, 14}, 167: {3, 12}, 148: {5, 6}, 168: {6, 8}, 149: {10, 5}, 155: {15, 5}, 157: {19, 10}, 169: {20, 3}}
	entry, ok := entries[scene]
	if err != nil || !ok {
		t.Fatal("unsupported original tower collection scene", requested)
	}
	prefix := fmt.Sprint("tower-rewards-", scene)
	if err := os.Remove(campaignEvidencePath(prefix+"-result.json", 1)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	name := fmt.Sprint("tower-upper-step-", entry[0], "-state.json")
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../../.local/pal/verification/tower-upper-reward-candidates.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Candidates []towerRewardCandidate
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	spots := []towerRewardCandidate{}
	for _, q := range catalog.Candidates {
		if q.Scene != scene {
			continue
		}
		e := tables.Events[q.Event-1]
		if int(e.X) != q.X || int(e.Y) != q.Y || e.Sprite != 10 || int(e.TriggerScript) != q.Script || len(q.Grants) != 1 || len(q.Messages) != 1 {
			t.Fatal("unreviewed original chest candidate", q)
		}
		g := q.Grants[0]
		s := tables.Scripts[g.PC]
		if g.Item <= 0 || g.Cash != 0 || s.Opcode != 0x1f || int(s.Operands[0]) != g.Item || max(1, int(s.Operands[1])) != g.Amount {
			t.Fatal("original tower chest grant changed or needs separate review", q)
		}
		spots = append(spots, q)
	}
	if len(spots) != entry[1] {
		t.Fatal("original tower chest scene inventory incomplete", scene, len(spots), entry[1])
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), tables: tables, physicalAttacks: true,
		prefix: prefix, milestones: map[string]string{}, reserveScenes: []int{scene},
		proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	r.w = campaignMapWalker{rememberCollision: true, allowTouch: true, avoidTouches: map[int][][3]int{}}
	forceInputs := 0
	r.manualBattle = func(v pal.View) ([]byte, bool) {
		if v.Battle == nil {
			return nil, false
		}
		danger := false
		for _, enemy := range v.Battle.Enemies {
			danger = danger || enemy.Object == 441 || enemy.Object == 421
		}
		if !danger {
			return nil, false
		}
		action, ok := caiyiForestPhysicalAttack(v)
		if !ok {
			return nil, true
		}
		if v.Battle.State == 1 && v.Battle.Menu == 0 {
			action.Direction, action.Keys = 4, 32768
			forceInputs++
		}
		return raw(action), true
	}
	r.reserve(-1)
	for _, q := range spots {
		r.w.avoidTouches[scene] = append(r.w.avoidTouches[scene], [3]int{q.X, q.Y, 16})
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	initial := r.field()
	if initial.Scene != scene || len(initial.Party) != 2 || initial.Party[0].Role != 0 || initial.Party[1].Role != 2 {
		t.Fatal("tower optional collection source is not the original earned floor", scene, initial.Scene, initial.Party)
	}
	names := []string{}
	for _, q := range spots {
		names = append(names, q.Grants[0].Name)
	}
	expected := r.inventoryCounts(names...)
	seen, frames := map[int]bool{}, map[int]int{}
	claimed := []map[string]any{}
	preserved := []map[string]any{}
	for _, q := range spots {
		if record, ok := towerPreservedUnreachable(t, r.field(), q); ok {
			preserved = append(preserved, record)
			continue
		}
		label := fmt.Sprint("reward-", q.Event)
		e := tables.Events[q.Event-1]
		closedFrame := int(e.Direction*e.Frames + e.Frame)
		call := tables.Scripts[q.Script]
		opened := tables.Scripts[call.Operands[0]]
		if call.Opcode != 4 || opened.Opcode != 0x14 {
			t.Fatal("unreviewed original chest opening script", q.Event, call, opened)
		}
		openFrame := int(opened.Operands[0])
		r.notice = func(v pal.View) {
			if r.proof.InBattle && v.Battle == nil {
				r.w.known = nil
			}
			if v.Field != nil {
				for _, a := range v.Field.Actors {
					if a[0] == 1 && a[1] == q.Event {
						frames[q.Event] = a[5]
					}
				}
			}
			if !seen[q.Event] && strings.Contains(hiddenNoticeText(v), q.Messages[0]) {
				seen[q.Event] = true
				r.save(label + "-notice")
			}
		}
		t.Log("original optional tower chest", scene, q.Event, q.Grants[0])
		visits := map[[2]int]int{}
		for i := 0; ; i++ {
			v := r.field()
			at := [2]int{v.X, v.Y}
			visits[at]++
			if v.Scene != scene || i > 2200 || visits[at] > 60 {
				t.Fatal("ordinary tower chest approach blocked", q.Event, v.Scene, at)
			}
			if fieldTestDistance(v.X, v.Y, q.X, q.Y) <= 96 {
				break
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, q.X, q.Y, 96)
			r.progress()
		}
		present := false
		for _, a := range r.field().Field.Actors {
			present = present || a[0] == 1 && a[1] == q.Event && a[2] == 10 && a[5] == closedFrame
		}
		if !present {
			t.Fatal("original unopened tower chest sprite missing", q.Event)
		}
		r.save(label + "-unpicked")
		v := r.field()
		type searchStance struct{ x, y, facing, distance int }
		stances := []searchStance{}
		for d, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
			x, y := q.X-delta[0], q.Y-delta[1]
			u, z := ((x-v.X)/16+(y-v.Y)/8)/2, ((y-v.Y)/8-(x-v.X)/16)/2
			if u < -8 || u > 8 || z < -8 || z > 8 || v.Field.Cells[(z+8)*17+u+8]&3 != 0 {
				continue
			}
			stances = append(stances, searchStance{x, y, d, fieldTestDistance(v.X, v.Y, x, y)})
		}
		if len(stances) == 0 {
			t.Fatal("original chest has no observed open search stance", q.Event)
		}
		sort.SliceStable(stances, func(i, j int) bool { return stances[i].distance < stances[j].distance })
		chosen := 0
		t.Log("original tower search stances", q.Event, stances)
		search := func() {
			v := r.field()
			stance := stances[chosen]
			if v.X != stance.x || v.Y != stance.y {
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, stance.x, stance.y, 0)
				return
			}
			r.keys([]uint32{4, 8, 16, 32}[stance.facing], stance.facing)
			r.keys(0, 4)
			r.progress()
			r.field()
			r.keys(2, 4)
		}
		searchVisits := map[[2]int]int{}
		for i := 0; i < 600 && !seen[q.Event]; i++ {
			v := r.field()
			at := [2]int{v.X, v.Y}
			searchVisits[at]++
			if searchVisits[at] > 6 {
				chosen++
				if chosen >= len(stances) {
					t.Fatal("all observed tower chest search stances blocked", q.Event, at, stances)
				}
				searchVisits = map[[2]int]int{}
				t.Log("original tower alternate search stance", q.Event, stances[chosen])
			}
			search()
			r.progress()
			r.field()
		}
		for _, a := range r.field().Field.Actors {
			if a[0] == 1 && a[1] == q.Event {
				frames[q.Event] = a[5]
			}
		}
		if !seen[q.Event] || frames[q.Event] != openFrame {
			t.Fatal("original tower chest notice/opening missing", q.Event, seen[q.Event], frames[q.Event])
		}
		expected[q.Grants[0].Name] += q.Grants[0].Amount
		after := r.inventoryCounts(names...)
		if !bytes.Equal(raw(expected), raw(after)) {
			t.Fatal("original tower chest inventory delta changed", q.Event, expected, after)
		}
		r.save(label + "-collected")
		for repeat := 0; repeat < 3; repeat++ {
			search()
			r.progress()
			r.field()
		}
		if !bytes.Equal(raw(after), raw(r.inventoryCounts(names...))) {
			t.Fatal("original tower chest duplicated after saved searches", q.Event)
		}
		r.save(label + "-repeated")
		claimed = append(claimed, map[string]any{"scene": scene, "event": q.Event, "position": [2]int{q.X, q.Y}, "item": q.Grants[0].Name, "amount": q.Grants[0].Amount, "openFrame": frames[q.Event], "notice": q.Messages[0], "repeatSearches": 3})
	}
	v := r.field()
	if v.Scene != scene || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 || len(claimed)+len(preserved) != entry[1] {
		t.Fatal("optional tower collection altered original route or battles", v.Scene, r.proof, len(claimed))
	}
	hash := sha256.Sum256(source)
	if !bytes.Equal(raw(expected), raw(r.inventoryCounts(names...))) {
		t.Fatal("original final inventory differs from actual claimed rewards")
	}
	report := raw(map[string]any{"passed": true, "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "scope": "all listed original floor reward records accounted for: actual ordinary pickups with exact quantities/notices/opening, fresh-runtime saves and three repeated searches, and separately source/map/text-verified original unreachable records; physical attacks and original F shortcut for native enemies 441/421, normal MP costs and no collected-item consumption; no native edits or false pickup claims", "originalRecordCount": entry[1], "preservedUnreachable": preserved, "forceInputs": forceInputs, "sourceCheckpoint": name, "sourceSHA256": hex.EncodeToString(hash[:]), "claimed": claimed, "inventory": expected, "scene": v.Scene, "cash": v.Cash, "party": v.Party, "proof": r.proof})
	for kind, data := range map[string][]byte{"result": report, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+kind+".json", 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
