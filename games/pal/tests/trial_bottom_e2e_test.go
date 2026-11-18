package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type trialBottomBoat struct {
	Event    int
	Position [2]int
	Moves    map[[2]int][2]int
}

func TestOriginalTrialBottomTransitDiagnostic(t *testing.T) {
	if os.Getenv("PAL_TRIAL_BOTTOM_DIAGNOSTIC") != "1" {
		t.Skip("explicit diagnostic only")
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, "trial-bottom-v1-two-boat-transit-1-state.json"), tables: nil, prefix: "trial-bottom-diagnostic", allowedRoles: map[int]bool{0: true, 4: true}, milestones: map[string]string{}, proof: caiyiForestProof{Saved: map[string]bool{}, Seen: map[string]bool{}, Scenes: map[int]int{}}}
	r.notice = func(v pal.View) {
		if v.Phase == "script" {
			r.save(fmt.Sprint("tick-", r.proof.Saves))
		}
	}
	for step := 0; step < 256; step++ {
		v := r.visible()
		t.Log("transit diagnostic", step, v.Phase, v.X, v.Y, v.Seq, v.AppliedSeq, v.DiscardedSeq)
		if v.Phase == "field" {
			r.save(fmt.Sprint("field-", step))
			if v.Seq == v.AppliedSeq {
				return
			}
		}
		r.progress()
	}
	t.Fatal("diagnostic transit did not settle")
}

func trialBottomBoats(t *testing.T, tables *resources.Tables) []trialBottomBoat {
	t.Helper()
	boats := []trialBottomBoat{}
	for event := 3607; event <= 3635; event++ {
		if event > 3607 && event < 3614 {
			continue
		}
		e := tables.Events[event-1]
		start := int(e.TriggerScript)
		position := [2]int{int(e.X), int(e.Y)}
		if event == 3607 {
			start, position = 31292, [2]int{272, 1448}
		}
		boat := trialBottomBoat{Event: event, Position: position, Moves: map[[2]int][2]int{}}
		lastRide, entry := position, position
		for pc := start; pc < start+32; pc++ {
			s := tables.Scripts[pc]
			if s.Opcode == 0 || s.Opcode == 2 {
				break
			}
			if s.Opcode == 0x44 || s.Opcode == 0x7a {
				p := [2]int{int(s.Operands[0])*32 + int(s.Operands[2])*16, int(s.Operands[1])*16 + int(s.Operands[2])*8}
				if s.Opcode == 0x44 {
					lastRide = p
				} else {
					boat.Moves[entry] = p
					entry = lastRide
				}
			}
		}
		if len(boat.Moves) < 2 {
			t.Fatal("original bottom boat route incomplete", event, boat)
		}
		boats = append(boats, boat)
	}
	return boats
}

func trialBottomSpells(r *caiyiForestDriver, role int, label string) map[string]bool {
	r.t.Helper()
	r.field()
	r.keys(32768, 4)
	for step := 0; step < 256 && (r.visible().Menu.Kind != 3 || r.visible().Seq != r.visible().AppliedSeq); step++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 3 {
		r.t.Fatal("original bottom spell role selector absent")
	}
	wanted := -1
	for index, member := range r.visible().Party {
		if member.Role == role {
			wanted = index
		}
	}
	if wanted < 0 {
		r.t.Fatal("original bottom spell role absent", role)
	}
	for step := 0; step < 32; step++ {
		v := r.visible()
		if v.Menu.Selected == wanted {
			r.keys(2, 4)
			break
		}
		r.keys(8, 1)
		r.progress()
		r.progress()
	}
	for step := 0; step < 128 && (r.visible().Menu.Kind != 10 || r.visible().Seq != r.visible().AppliedSeq); step++ {
		r.progress()
	}
	if r.visible().Menu.Kind != 10 || r.visible().Menu.Role != role {
		r.t.Fatal("original bottom spell list absent", role, r.visible().Menu)
	}
	r.keys(65536, 4)
	r.progress()
	r.progress()
	found, previous := map[string]bool{}, ""
	for step := 0; step < 64; step++ {
		text := originalVisibleText(r.visible(), false)
		for _, name := range []string{"风神", "雷神", "雪妖", "火神", "山神"} {
			if strings.Contains(text, name) && !found[name] {
				found[name] = true
				r.save(fmt.Sprint(label, "-role-", role, "-", name))
			}
		}
		if text == previous {
			break
		}
		previous = text
		r.keys(128, 4)
		r.progress()
		r.progress()
	}
	r.save(fmt.Sprint(label, "-role-", role, "-menu"))
	r.keys(1, 4)
	r.field()
	return found
}

func TestOriginalTrialBottomReedBoatsEquipmentFiveGodsAndSave(t *testing.T) {
	branch := os.Getenv("PAL_TRIAL_BOTTOM")
	if branch == "" {
		t.Skip("explicit original bottom cave and two genuine party branches")
	}
	if (branch != "two" && branch != "three") || campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("bottom cave requires authoritative registered profile", branch)
	}
	planBytes, err := os.ReadFile("../../../.local/pal/verification/trial-bottom-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Passed, DiagnosticOnly bool
		Branches               []struct {
			Branch, SourceCheckpoint, SourceSHA256 string
			Roles                                  []int
			Legs                                   []trialRewardLeg
		}
	}
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.Passed || !plan.DiagnosticOnly {
		t.Fatal("bottom original route plan failed")
	}
	selected := -1
	for index, row := range plan.Branches {
		if row.Branch == branch {
			selected = index
		}
	}
	if selected < 0 {
		t.Fatal("bottom party route absent")
	}
	p := plan.Branches[selected]
	source, err := os.ReadFile(campaignEvidencePath(p.SourceCheckpoint, 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	if hex.EncodeToString(hash[:]) != p.SourceSHA256 {
		t.Fatal("bottom parent changed")
	}
	if err := acceptedCampaignRoot(p.SourceCheckpoint, source, ""); err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	prefix := "trial-bottom-v1-" + branch
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, p.SourceCheckpoint), tables: tables, prefix: prefix, physicalAttacks: true, milestones: map[string]string{}, allowedRoles: map[int]bool{}, allowedMenus: map[int]bool{3: true, 10: true}, battlePartySize: len(p.Roles), proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	for _, role := range p.Roles {
		r.allowedRoles[role] = true
	}
	for scene := 214; scene <= 229; scene++ {
		r.reserveScenes = append(r.reserveScenes, scene)
	}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	reserveTrialRewardObjects(r, -1)
	defer func() {
		if t.Failed() {
			for suffix, data := range map[string][]byte{"state": r.g.Data, "view": raw(r.visible()), "proof": raw(r.proof)} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-failure-"+suffix+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	names := []string{"芦苇漂", "玄冥宝刀", "冥蛇杖", "无尘剑", "金罡珠", "魅影神靴", "傀儡虫", "土灵珠"}
	battleCash, battleDrops := []map[string]any{}, []map[string]any{}
	currentWorms := map[int]int{}
	currentBattle := 0
	observe := func(v pal.View) {
		if v.Battle != nil && !r.proof.InBattle {
			currentWorms = map[int]int{}
			currentBattle = r.proof.Battles + 1
			for _, enemy := range v.Battle.Enemies {
				if enemy.Object > 0 && tables.Objects[enemy.Object][3] == 42828 {
					currentWorms[enemy.Object]++
				}
			}
		}
		if len(currentWorms) > 0 && strings.Contains(hiddenNoticeText(v), "获得一只傀儡虫") && !r.proof.Saved[fmt.Sprint("battle-", currentBattle, "-worm-drop")] {
			r.save(fmt.Sprint("battle-", currentBattle, "-worm-drop"))
		}
		if r.proof.InBattle && v.Battle == nil {
			battleCash = append(battleCash, map[string]any{"cashBefore": r.proof.BattleCash, "cashAfter": v.Cash, "rewardCash": r.proof.RewardCash})
			for enemy, amount := range currentWorms {
				battleDrops = append(battleDrops, map[string]any{"battle": currentBattle, "enemy": enemy, "endScript": 42828, "item": 152, "name": "傀儡虫", "amount": amount})
			}
			r.w.known = nil
		}
	}
	r.notice = observe
	initialCash := r.field().Cash
	initialInventory := trialWormInventory(r, "", names...)
	r.save("source")
	trace := []map[string]any{}
	for index, leg := range p.Legs {
		r.w.rememberCollision = leg.Scene != 229
		t.Log("bottom original entry leg", index+1, leg.Event)
		trace = append(trace, followTrialRewardLeg(r, leg, index+1))
	}
	if v := r.field(); v.Scene != 214 || v.Map != 225 {
		t.Fatal("bottom original entry failed", v.Scene, v.Map)
	}
	for event := 3641; event <= 3645; event++ {
		e := tables.Events[event-1]
		r.w.avoidTouches[214] = append(r.w.avoidTouches[214], [3]int{int(e.X), int(e.Y), 16})
	}
	boats := trialBottomBoats(t, tables)
	boatReady := false
	refresh := func(v pal.View) {
		if v.Scene != 214 || v.Field == nil {
			return
		}
		for index := range boats {
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == boats[index].Event {
					boats[index].Position = [2]int{actor[3], actor[4]}
				}
			}
		}
		moves := map[[2]int][2]int{}
		for _, boat := range boats {
			if boat.Event == 3607 && !boatReady {
				continue
			}
			if destination, ok := boat.Moves[boat.Position]; ok {
				moves[boat.Position] = destination
			}
		}
		r.w.sameSceneMoves = map[int]map[[2]int][2]int{214: moves}
	}
	active, message := "", ""
	noticed := false
	boatSaves := 0
	activeBoat, savedBoatEpisode := 0, false
	r.notice = func(v pal.View) {
		observe(v)
		refresh(v)
		if active != "" && !noticed && strings.Contains(hiddenNoticeText(v)+originalVisibleText(v, false), message) {
			noticed = true
			r.save(active + "-notice")
		}
		if boatReady && activeBoat > 0 && !savedBoatEpisode && v.Scene == 214 && v.Phase == "script" {
			label := fmt.Sprint("boat-transit-", boatSaves+1)
			if !r.proof.Saved[label] {
				boatSaves++
				savedBoatEpisode = true
				r.save(label)
			}
		}
	}
	walk := func(x, y int) {
		t.Helper()
		visits := map[[2]int]int{}
		for step := 0; step < 6000; step++ {
			v := r.field()
			if boatReady {
				t.Log("bottom boat field", step, v.X, v.Y, v.Seq, v.AppliedSeq, v.DiscardedSeq)
			}
			if v.Scene != 214 {
				t.Fatal("bottom walk left original scene", v.Scene)
			}
			if v.X == x && v.Y == y {
				return
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 30 {
				t.Fatal("bottom original route blocked", at, x, y)
			}
			refresh(v)
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, x, y, 0)
			r.progress()
			if step%100 == 0 {
				t.Log("bottom walk", step, v.X, v.Y, x, y)
			}
		}
		t.Fatal("bottom original route timed out", x, y)
	}
	inventory := func() map[string]int { return trialWormInventory(r, "", names...) }
	initialSpells := map[int]map[string]bool{}
	for _, role := range p.Roles {
		initialSpells[role] = trialBottomSpells(r, role, "before-gods")
	}
	data, err := os.ReadFile("../../../.local/pal/verification/hidden-search-candidates.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Candidates []towerRewardCandidate }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	byID := map[int]towerRewardCandidate{}
	for _, q := range catalog.Candidates {
		byID[q.Event] = q
	}
	claimed := []map[string]any{}
	collect := func(event int) {
		t.Helper()
		q := byID[event]
		if q.Event != event || q.Scene != 214 || len(q.Grants) != 1 || len(q.Messages) == 0 {
			t.Fatal("bottom original candidate missing", event)
		}
		active, message, noticed = fmt.Sprint("reward-", event), q.Messages[0], false
		before := inventory()
		r.save(active + "-unpicked")
		for step := 0; step < 3000 && !noticed; step++ {
			v := r.field()
			if fieldTestDistance(v.X, v.Y, q.X, q.Y) <= 96 {
				r.search(event, q.X, q.Y)
			} else {
				refresh(v)
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, q.X, q.Y, 96)
			}
			r.progress()
		}
		if !noticed {
			t.Fatal("bottom original reward notice absent", event, message)
		}
		r.field()
		r.save(active + "-collected")
		after := inventory()
		grant := q.Grants[0]
		if after[grant.Name]-before[grant.Name] != grant.Amount {
			t.Fatal("bottom original reward inventory mismatch", event, before, after)
		}
		pose := r.field()
		for repeat := 0; repeat < 3; repeat++ {
			r.search(event, q.X, q.Y)
			r.progress()
			r.field()
		}
		if !bytes.Equal(raw(after), raw(inventory())) {
			t.Fatal("bottom reward repeated after save", event)
		}
		r.save(active + "-repeated")
		claimed = append(claimed, map[string]any{"event": event, "scene": 214, "position": [2]int{q.X, q.Y}, "grants": q.Grants, "notification": true, "repeatSearches": 3, "repeatPose": [3]int{pose.X, pose.Y, pose.Field.Direction}, "before": before, "after": after})
		active = ""
		t.Log("bottom original reward verified", event)
	}
	collect(3607)
	collect(3645)
	v := r.field()
	blocked, wall := -1, [2]int{}
	best := int(^uint(0) >> 1)
	for i, flags := range v.Field.Cells[:289] {
		if flags&3 != 0 {
			continue
		}
		u, z := i%17-8, i/17-8
		x, y := v.X+(u-z)*16, v.Y+(u+z)*8
		for direction, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
			fx, fy := x+delta[0], y+delta[1]
			at := (fy/16*64+fx/32)*2 + fx%32/16
			if at >= 0 && at < 16384 && binary.LittleEndian.Uint32(r.w.maps[225][at*4:])&8192 != 0 {
				if score := fieldTestDistance(v.X, v.Y, x, y); score < best {
					blocked, wall, best = direction, [2]int{x, y}, score
				}
			}
		}
	}
	if blocked < 0 {
		t.Fatal("bottom invalid-use witness has no blocked front")
	}
	walk(wall[0], wall[1])
	r.keys([]uint32{4, 8, 16, 32}[blocked], blocked)
	r.keys(0, 4)
	r.progress()
	r.field()
	trialWormInventory(r, "芦苇漂", names...)
	r.save("reed-invalid-selected")
	r.keys(2, 4)
	r.progress()
	r.field()
	r.save("reed-invalid-finished")
	if inventory()["芦苇漂"] != 1 {
		t.Fatal("invalid original reed placement consumed inventory")
	}
	walk(288, 1440)
	r.keys(4, 0)
	r.keys(0, 4)
	r.progress()
	r.field()
	trialWormInventory(r, "芦苇漂", names...)
	r.save("reed-bank-selected")
	r.keys(2, 4)
	r.progress()
	r.field()
	boatReady = true
	r.save("reed-bank-placed")
	if inventory()["芦苇漂"] != 0 {
		t.Fatal("original bank reed placement did not consume item")
	}
	refresh(r.field())
	activeBoat, savedBoatEpisode = 3607, false
	walk(512, 1568)
	r.save("reed-first-crossed")
	activeBoat = 0
	boatTrips := []map[string]any{{"event": 3607, "entry": [2]int{272, 1448}, "destination": [2]int{512, 1568}}}
	baseAvoid := append([][3]int{}, r.w.avoidTouches[214]...)
	for index, event := range []int{3614, 3630, 3631, 3632, 3634, 3635, 3635} {
		refresh(r.field())
		var boat trialBottomBoat
		for _, candidate := range boats {
			if candidate.Event == event {
				boat = candidate
			}
		}
		destination, ok := boat.Moves[boat.Position]
		if !ok {
			t.Fatal("original staged bottom boat has no current destination", boat)
		}
		r.w.sameSceneMoves = map[int]map[[2]int][2]int{214: {boat.Position: destination}}
		r.w.avoidTouches[214] = append([][3]int{}, baseAvoid...)
		for _, other := range boats {
			if other.Event != event {
				r.w.avoidTouches[214] = append(r.w.avoidTouches[214], [3]int{other.Position[0], other.Position[1], 16})
			}
		}
		activeBoat, savedBoatEpisode = event, false
		t.Log("bottom boarding original boat", event, boat.Position, destination)
		arrived := false
		for step := 0; step < 1600; step++ {
			v := r.field()
			if savedBoatEpisode && v.X == destination[0] && v.Y == destination[1] {
				arrived = true
				break
			}
			r.w.sameSceneMoves = map[int]map[[2]int][2]int{}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, boat.Position[0], boat.Position[1], 0)
			r.progress()
		}
		if !arrived {
			t.Fatal("original staged bottom boat did not arrive", event, boat.Position, destination)
		}
		r.save(fmt.Sprint("boat-", index+1, "-", event, "-arrived"))
		boatTrips = append(boatTrips, map[string]any{"event": event, "entry": boat.Position, "destination": destination})
		activeBoat = 0
	}
	refresh(r.field())
	r.w.avoidTouches[214] = append([][3]int{}, baseAvoid...)
	for _, boat := range boats {
		r.w.avoidTouches[214] = append(r.w.avoidTouches[214], [3]int{boat.Position[0], boat.Position[1], 16})
	}

	for _, event := range []int{3641, 3642, 3643, 3644} {
		collect(event)
	}
	gods := []map[string]any{}
	for event := 3636; event <= 3640; event++ {
		e := tables.Events[event-1]
		active = fmt.Sprint("god-", event)
		noticed = false
		for pc := int(e.TriggerScript); pc < int(e.TriggerScript)+8; pc++ {
			s := tables.Scripts[pc]
			if s.Opcode == 0xffff {
				message, err = simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[s.Operands[0]]))
				if err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		r.save(active + "-unpicked")
		for step := 0; step < 2400 && !noticed; step++ {
			v := r.field()
			if fieldTestDistance(v.X, v.Y, int(e.X), int(e.Y)) <= 96 {
				r.search(event, int(e.X), int(e.Y))
			} else {
				refresh(v)
				r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(e.X), int(e.Y), 96)
			}
			r.progress()
		}
		if !noticed {
			t.Fatal("original god teaching notice absent", event, message)
		}
		r.field()
		r.save(active + "-learned")
		for repeat := 0; repeat < 3; repeat++ {
			r.search(event, int(e.X), int(e.Y))
			r.progress()
			r.field()
		}
		r.save(active + "-repeated")
		gods = append(gods, map[string]any{"event": event, "message": message, "repeatSearches": 3})
		active = ""
	}
	finalSpells := map[int]map[string]bool{}
	for _, role := range p.Roles {
		finalSpells[role] = trialBottomSpells(r, role, "after-gods")
	}
	for role, spells := range map[int][]string{0: {"雷神", "山神"}, 4: {"风神", "雪妖"}} {
		for _, spell := range spells {
			if !finalSpells[role][spell] {
				t.Fatal("original god spell missing", role, spell)
			}
		}
	}
	fireRole := 4
	if branch == "three" {
		fireRole = 1
	}
	if !finalSpells[fireRole]["火神"] {
		t.Fatal("original fire god branch recipient wrong", branch, finalSpells)
	}
	r.save("complete")
	finalInventory := inventory()
	for _, name := range names[1:6] {
		if finalInventory[name]-initialInventory[name] != 1 {
			t.Fatal("bottom equipment inventory did not persist", name, initialInventory, finalInventory)
		}
	}
	if boatSaves == 0 || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 {
		t.Fatal("bottom boat saves or ordinary battle finish incomplete", boatSaves, r.proof)
	}
	wormDrops := 0
	for _, row := range battleDrops {
		wormDrops += row["amount"].(int)
	}
	if finalInventory["傀儡虫"]-initialInventory["傀儡虫"] != wormDrops || finalInventory["土灵珠"] != initialInventory["土灵珠"] {
		t.Fatal("bottom route normal worm drop or earth orb mismatch", initialInventory, finalInventory, battleDrops)
	}
	result := raw(map[string]any{"cashInitial": initialCash, "cashFinal": r.field().Cash, "battleCashChanges": battleCash, "battleItemGrants": battleDrops, "passed": true, "nativeEdits": false, "nativeBuildID": "2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa", "branch": branch, "sourceCheckpoint": p.SourceCheckpoint, "sourceSHA256": p.SourceSHA256, "roles": p.Roles, "route": trace, "claimed": claimed, "gods": gods, "fireRole": fireRole, "spellsBefore": initialSpells, "spellsAfter": finalSpells, "inventoryInitial": initialInventory, "inventory": finalInventory, "boatSaves": boatSaves, "boatTrips": boatTrips, "proof": r.proof})
	for suffix, value := range map[string][]byte{"result": result, "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), value, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
