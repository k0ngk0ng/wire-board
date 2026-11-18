package pal_test

// E2E first: Blackwater exits route to the wrong map; the equipped original
// bead fails at the aura; stairs are treated as floor; switches or gate frames
// reset on resume; general rewards repeat; the original Li-only fall or blood
// pool reunion is lost; a fresh runtime accepts pre-save input. Walkthrough
// identities below are test data only, never runtime bot knowledge.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
)

type ghostTombProof struct {
	Route, Switches, Stairs, Battles, Wins, Commands, Readings, Saves int
	General, GeneralWon, LiOnly, Calling, Pool, Victims, Reunited     bool
	InBattle, GeneralBattle                                           bool
	RewardXP, RewardCash                                              int
	BattleCash                                                        uint32
	LastPrompt                                                        uint64
	Saved                                                             map[string]bool
	Open                                                              map[int]bool
	GateFrames                                                        map[int]int
	Rewards                                                           [][2]int
}

type ghostTombDriver struct {
	*riverMedicineDriver
	proof ghostTombProof
}

type tombWaypoint struct {
	Kind                     string
	Scene, Event, X, Y, Dest int
	Land                     [2]int
	Opens                    [2]int
}

func TestOriginalBlackwaterGeneralTombFallAndSave(t *testing.T) {
	name := "campaign-jade-temple-equipped-state.json"
	p := ghostTombProof{Saved: map[string]bool{}, Open: map[int]bool{}, GateFrames: map[int]int{}}
	if checkpoint := os.Getenv("PAL_TOMB_CHECKPOINT"); checkpoint != "" {
		if checkpoint != "failure" && checkpoint != "general-battle" {
			t.Fatal("unsupported tomb diagnostic checkpoint", checkpoint)
		}
		name = "ghost-tomb-" + checkpoint + "-state.json"
		b, err := os.ReadFile(campaignEvidencePath("ghost-tomb-"+checkpoint+"-proof.json", 1))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &p); err != nil {
			t.Fatal(err)
		}
	}
	if p.GateFrames == nil {
		p.GateFrames = map[int]int{}
	}
	r := &ghostTombDriver{riverMedicineDriver: loadRiverMedicine(t, name), proof: p}
	defer func() {
		if !t.Failed() {
			return
		}
		for name, data := range map[string][]byte{"ghost-tomb-failure-state.json": r.g.Data, "ghost-tomb-failure-proof.json": raw(r.proof), "ghost-tomb-failure-view.json": raw(r.visible())} {
			if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
				t.Log(err)
			}
		}
	}()
	r.w = campaignMapWalker{allowTouch: true, avoidTouches: map[int][][3]int{}}
	prefix := []tombWaypoint{
		{Kind: "stair", Scene: 58, Event: 945, X: 1520, Y: 1848, Dest: 55, Land: [2]int{416, 544}},
		{Kind: "stair", Scene: 55, Event: 925, X: 1488, Y: 248, Dest: 62, Land: [2]int{1312, 1376}},
		{Kind: "stair", Scene: 62, Event: 1176, X: 160, Y: 448, Dest: 64, Land: [2]int{1840, 1896}},
		{Kind: "stair", Scene: 64, Event: 1206, X: 1600, Y: 272, Dest: 65, Land: [2]int{416, 1840}},
		{Kind: "stair", Scene: 65, Event: 1220, X: 560, Y: 264, Dest: 61, Land: [2]int{1360, 1528}},
	}
	route := append(prefix, tombRoute...)
	for r.proof.Route < len(route) {
		q := route[r.proof.Route]
		v := r.field()
		if v.Scene != q.Scene {
			t.Fatal("wrong original tomb route", r.proof.Route, q, v.Scene, v.X, v.Y)
		}
		r.reserve(q.Event)
		t.Log("tomb waypoint", r.proof.Route, q, "from", v.X, v.Y)
		switch q.Kind {
		case "stair":
			r.walk(q.Scene, q.X, q.Y, 0)
			v = r.field()
			if v.Scene != q.Dest || fieldTestDistance(v.X, v.Y, q.Land[0], q.Land[1]) > 128 {
				t.Fatal("tomb stair/aura failed", q, v.Scene, v.X, v.Y)
			}
			r.proof.Stairs++
		case "switch":
			r.walk(q.Scene, q.X, q.Y, 0)
			opened := false
			for i := 0; i < 64; i++ {
				v = r.field()
				pressed, visibleGate, gateReady := false, false, false
				for _, a := range v.Field.Actors {
					if a[0] != 1 {
						continue
					}
					if a[1] == q.Event && a[5] == 1 {
						pressed = true
					}
					if a[1] == q.Opens[0] {
						visibleGate = true
						gateReady = a[5] >= 4
					}
				}
				// Remote doors are absent from the bounded public view. Observe
				// the pressed plate here and the door along the ordinary route.
				opened = pressed && (!visibleGate || gateReady)
				if opened {
					break
				}
				r.progress()
			}
			if !opened {
				t.Fatal("original tomb switch/gate did not settle", q, v.Field.Actors)
			}
			for _, event := range q.Opens {
				r.proof.Open[event] = true
			}
			r.proof.Switches++
		case "general":
			for i := 0; i < 1500; i++ {
				v = r.field()
				if v.Scene == 60 && r.proof.GeneralWon {
					break
				}
				if v.Scene != 66 {
					t.Fatal("general route left original room", v.Scene)
				}
				if fieldTestDistance(v.X, v.Y, q.X, q.Y) > 160 {
					r.g = r.w.walk(t, r.m, &r.c, r.g, v, q.X, q.Y, 96)
				} else {
					r.g = campaignWalkTo(t, r.m, &r.c, r.g, v, q.X, q.Y, q.Event)
				}
				r.progress()
			}
			v = r.field()
			if v.Scene != 60 || !r.proof.GeneralWon {
				t.Fatal("ghost general/fall never completed", r.proof, v.Scene)
			}
		}
		r.proof.Route++
		r.save(fmt.Sprint("waypoint-", r.proof.Route))
	}
	v := r.field()
	if v.Scene != 60 || v.Map != 76 || len(v.Party) != 3 || v.Party[0].Role != 0 || v.Party[1].Role != 1 || v.Party[2].Role != 2 || !r.proof.General || !r.proof.GeneralWon || !r.proof.LiOnly || !r.proof.Calling || !r.proof.Pool || !r.proof.Victims || !r.proof.Reunited || r.proof.Battles != r.proof.Wins || r.proof.Switches != 21 {
		t.Fatal("original tomb incomplete", r.proof, v.Scene, v.Party)
	}
	r.save("complete")
	source, err := os.ReadFile(campaignEvidencePath("campaign-jade-temple-equipped-state.json", 1))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(source)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "developmentProfile": developmentHealthProfile(), "sourceCheckpointSHA256": hex.EncodeToString(hash[:]), "scope": "ordinary original Blackwater movement, equipped bead aura entry, animated tomb gates and actual stairs, ghost general with displayed reward, Li-only fall and blood-pool reunion; fresh-runtime saves preserve exact public state and reject old input; no native state/script edits", "scene": v.Scene, "map": v.Map, "party": v.Party, "proof": r.proof}, "", "  ")
	for name, data := range map[string][]byte{"campaign-ghost-tomb-state.json": r.g.Data, "campaign-ghost-tomb.json": report} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (r *ghostTombDriver) save(name string) {
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
		r.t.Fatal("tomb save changed original continuation", name)
	}
	if _, err = r.m.Apply(r.c, r.g, raw(pal.Action{Type: "input", Epoch: before.Epoch, Seq: before.Seq + 1, ControlAt: before.ControlAt, Direction: 4, Keys: 2})); err == nil {
		r.t.Fatal("tomb accepted pre-save input", name)
	}
	r.proof.Saves++
	r.proof.Saved[name] = true
	for suffix, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof)} {
		if err := os.WriteFile(campaignEvidencePath("ghost-tomb-"+name+"-"+suffix+".json", 1), data, 0600); err != nil {
			r.t.Fatal(err)
		}
	}
}

func (r *ghostTombDriver) progress() {
	r.t.Helper()
	v := r.visible()
	p := &r.proof
	if v.Field != nil {
		for _, a := range v.Field.Actors {
			if _, gate := tombGates[a[1]]; a[0] == 1 && gate && a[5] > p.GateFrames[a[1]] {
				p.GateFrames[a[1]] = a[5]
			}
		}
	}
	for _, member := range v.Party {
		if member.Controller != 0 || member.Role < 0 || member.Role > 2 {
			r.t.Fatal("tomb changed solo ownership", v.Party)
		}
	}
	if p.InBattle && v.Battle == nil {
		if p.RewardXP < 0 || p.RewardCash < 0 || v.Cash != p.BattleCash+uint32(p.RewardCash) {
			r.t.Fatal("tomb reward missing/repeated or defeat", p, v.Cash)
		}
		p.InBattle = false
		p.Wins++
		p.Rewards = append(p.Rewards, [2]int{p.RewardXP, p.RewardCash})
		if p.GeneralBattle {
			p.GeneralWon = true
			r.save("general-victory")
			v = r.visible()
		}
	}
	if len(v.Party) == 1 {
		if !p.GeneralWon || v.Party[0].Role != 0 {
			r.t.Fatal("tomb lost original temporary Li-only cast", v.Party)
		}
		p.LiOnly = true
		if !p.Saved["li-only-fall"] {
			r.save("li-only-fall")
			v = r.visible()
		}
	} else if len(v.Party) != 3 {
		r.t.Fatal("invented tomb party", v.Party)
	}
	if p.LiOnly && len(v.Party) == 3 {
		p.Reunited = true
	}
	if d := v.Dialogue; d != nil {
		if d.ID != p.LastPrompt {
			p.LastPrompt = d.ID
			p.Readings++
			p.General = p.General || strings.Contains(d.Text, "本～将～军")
			p.Calling = p.Calling || strings.Contains(d.Text, "灵儿！月如！")
			p.Pool = p.Pool || strings.Contains(d.Text, "赤鬼王所在的血池")
			p.Victims = p.Victims || strings.Contains(d.Text, "残害了多少生命")
			r.t.Log("tomb reading", p.Readings, v.Scene, strings.ReplaceAll(d.Text, "\n", " / "))
		}
		for _, mark := range []struct {
			name string
			done bool
		}{{"general-reading", p.General}, {"calling-reading", p.Calling}, {"pool-reading", p.Pool}, {"victims-reading", p.Victims}} {
			if mark.done && !p.Saved[mark.name] {
				r.save(mark.name)
			}
		}
		v = r.visible()
		r.g = apply(r.t, r.m, r.c, r.g, pal.Action{Type: "read", Epoch: v.Epoch, Prompt: v.Dialogue.ID})
	} else if v.Battle != nil {
		if !p.InBattle {
			p.InBattle = true
			p.Battles++
			p.BattleCash = v.Cash
			p.RewardXP, p.RewardCash = -1, -1
			p.GeneralBattle = false
			for _, e := range v.Battle.Enemies {
				if e.Object == 472 {
					p.GeneralBattle = true
				}
			}
			if p.Battles > 60 {
				r.t.Fatal("tomb repeats too many battles", p)
			}
			if p.GeneralBattle {
				if !p.General || p.GeneralWon {
					r.t.Fatal("general battle out of original order")
				}
				r.save("general-battle")
				v = r.visible()
			}
			r.t.Log("tomb battle", p.Battles, p.GeneralBattle, v.Scene, v.Battle.Enemies)
		}
		if xp, cash, ok := campaignBattleReward(v); ok {
			p.RewardXP, p.RewardCash = xp, cash
		}
		alive := false
		for _, member := range v.Party {
			alive = alive || member.HP > 0
		}
		if !alive {
			r.t.Fatal("original tomb party defeated")
		}
		a, err := r.m.BotAction(r.c, r.g)
		if err != nil {
			r.t.Fatal(err)
		}
		if len(a) > 0 {
			r.g, err = r.m.Apply(campaignBotContext(r.c), r.g, a)
			if err != nil {
				r.t.Fatal(err)
			}
			p.Commands++
		}
	}
	r.c.Now = time.UnixMilli(r.g.SystemDeadline + 200)
	var err error
	r.g, err = r.m.OnTimer(r.c, r.g)
	if err != nil {
		r.t.Fatal(err)
	}
}

func (r *ghostTombDriver) field() pal.View {
	r.t.Helper()
	for i := 0; i < 4096; i++ {
		v := r.visible()
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			// Observe battle settlement and final party restoration even when
			// the native script returns directly to the field in one batch.
			if r.proof.InBattle {
				r.progress()
				continue
			}
			if r.proof.LiOnly && len(v.Party) == 3 {
				r.proof.Reunited = true
			}
			return v
		}
		r.progress()
	}
	r.t.Fatal("original tomb flow never returned to field", r.visible().Phase)
	return pal.View{}
}

func (r *ghostTombDriver) walk(scene, x, y, reach int) {
	r.t.Helper()
	visits := map[[2]int]int{}
	for i := 0; i < 1600; i++ {
		v := r.field()
		if v.Scene != scene {
			return
		}
		if fieldTestDistance(v.X, v.Y, x, y) <= reach {
			return
		}
		place := [2]int{v.X, v.Y}
		visits[place]++
		if visits[place] > 24 {
			r.t.Fatal("tomb walkthrough repeats blocked position", scene, x, y, v.X, v.Y)
		}
		r.g = r.w.walk(r.t, r.m, &r.c, r.g, v, x, y, reach)
		r.progress()
		if i%100 == 0 {
			r.t.Log("tomb walking", i, scene, v.X, v.Y, "toward", x, y)
		}
	}
	r.t.Fatal("tomb waypoint not reached", scene, x, y)
}

func (r *ghostTombDriver) reserve(target int) {
	for scene := range r.w.avoidTouches {
		delete(r.w.avoidTouches, scene)
	}
	for _, p := range tombPortals {
		if p[1] != target {
			r.w.avoidTouches[p[0]] = append(r.w.avoidTouches[p[0]], [3]int{p[2], p[3], 48})
		}
	}
	for event, p := range tombGates {
		if !r.proof.Open[event] {
			r.w.avoidTouches[p[0]] = append(r.w.avoidTouches[p[0]], [3]int{p[1], p[2], 16})
		}
	}
}

// Original static route, used only to generate ordinary walkthrough inputs.
var tombRoute = []tombWaypoint{
	{Kind: "switch", Scene: 61, Event: 1142, X: 1440, Y: 1520, Opens: [2]int{1130, 1131}},
	{Kind: "stair", Scene: 61, Event: 1119, X: 1376, Y: 1008, Dest: 66, Land: [2]int{1232, 1160}},
	{Kind: "switch", Scene: 66, Event: 1289, X: 1280, Y: 1184, Opens: [2]int{1257, 1258}},
	{Kind: "switch", Scene: 66, Event: 1301, X: 1440, Y: 1328, Opens: [2]int{1281, 1282}},
	{Kind: "switch", Scene: 66, Event: 1300, X: 1504, Y: 1216, Opens: [2]int{1279, 1280}},
	{Kind: "switch", Scene: 66, Event: 1285, X: 1408, Y: 1824, Opens: [2]int{1249, 1250}},
	{Kind: "switch", Scene: 66, Event: 1286, X: 1296, Y: 1704, Opens: [2]int{1251, 1252}},
	{Kind: "switch", Scene: 66, Event: 1287, X: 1296, Y: 1544, Opens: [2]int{1253, 1254}},
	{Kind: "switch", Scene: 66, Event: 1288, X: 976, Y: 1656, Opens: [2]int{1255, 1256}},
	{Kind: "stair", Scene: 66, Event: 1246, X: 816, Y: 1624, Dest: 61, Land: [2]int{960, 1472}},
	{Kind: "switch", Scene: 61, Event: 1143, X: 1040, Y: 1496, Opens: [2]int{1132, 1133}},
	{Kind: "switch", Scene: 61, Event: 1144, X: 736, Y: 1440, Opens: [2]int{1134, 1135}},
	{Kind: "switch", Scene: 61, Event: 1145, X: 1104, Y: 1880, Opens: [2]int{1136, 1137}},
	{Kind: "stair", Scene: 61, Event: 1126, X: 640, Y: 1712, Dest: 66, Land: [2]int{496, 1864}},
	{Kind: "switch", Scene: 66, Event: 1302, X: 640, Y: 1744, Opens: [2]int{1283, 1284}},
	{Kind: "switch", Scene: 66, Event: 1293, X: 288, Y: 1232, Opens: [2]int{1265, 1266}},
	{Kind: "switch", Scene: 66, Event: 1290, X: 864, Y: 1120, Opens: [2]int{1259, 1260}},
	{Kind: "switch", Scene: 66, Event: 1296, X: 1056, Y: 928, Opens: [2]int{1271, 1272}},
	{Kind: "switch", Scene: 66, Event: 1297, X: 944, Y: 696, Opens: [2]int{1273, 1274}},
	{Kind: "switch", Scene: 66, Event: 1291, X: 784, Y: 1000, Opens: [2]int{1261, 1262}},
	{Kind: "switch", Scene: 66, Event: 1292, X: 400, Y: 1096, Opens: [2]int{1263, 1264}},
	{Kind: "stair", Scene: 66, Event: 1244, X: 320, Y: 1104, Dest: 61, Land: [2]int{464, 952}},
	{Kind: "stair", Scene: 61, Event: 1120, X: 1536, Y: 752, Dest: 66, Land: [2]int{1392, 904}},
	{Kind: "switch", Scene: 66, Event: 1298, X: 1008, Y: 152, Opens: [2]int{1275, 1276}},
	{Kind: "switch", Scene: 66, Event: 1294, X: 752, Y: 424, Opens: [2]int{1267, 1268}},
	{Kind: "switch", Scene: 66, Event: 1295, X: 256, Y: 384, Opens: [2]int{1269, 1270}},
	{Kind: "general", Scene: 66, Event: 1303, X: 544, Y: 256},
}
var tombPortals = [][4]int{
	{61, 1118, 1392, 1512},
	{61, 1119, 1376, 1008},
	{61, 1120, 1536, 752},
	{61, 1121, 1776, 648},
	{61, 1122, 1616, 376},
	{61, 1123, 1312, 272},
	{61, 1124, 544, 688},
	{61, 1125, 432, 968},
	{61, 1126, 640, 1712},
	{61, 1127, 928, 1488},
	{61, 1128, 784, 1240},
	{61, 1129, 1152, 896},
	{66, 1238, 1264, 1144},
	{66, 1239, 1424, 888},
	{66, 1240, 1664, 784},
	{66, 1241, 1504, 512},
	{66, 1242, 1200, 408},
	{66, 1243, 432, 824},
	{66, 1244, 320, 1104},
	{66, 1245, 528, 1848},
	{66, 1246, 816, 1624},
	{66, 1247, 672, 1376},
	{66, 1248, 1040, 1032},
	{62, 1175, 1280, 1392},
	{62, 1176, 160, 448},
	{62, 1177, 1280, 800},
	{62, 1178, 912, 664},
	{62, 1179, 528, 520},
	{63, 1191, 928, 784},
	{63, 1192, 960, 1616},
	{63, 1193, 1216, 1328},
	{64, 1205, 1872, 1912},
	{64, 1206, 1600, 272},
	{65, 1219, 352, 1872},
	{65, 1220, 560, 264},
}
var tombGates = map[int][3]int{
	1130: {61, 1520, 1576},
	1131: {61, 1536, 1568},
	1132: {61, 896, 1440},
	1133: {61, 912, 1432},
	1134: {61, 752, 1400},
	1135: {61, 768, 1392},
	1136: {61, 416, 1536},
	1137: {61, 432, 1528},
	1138: {61, 1648, 536},
	1139: {61, 1632, 528},
	1140: {61, 432, 472},
	1141: {61, 416, 464},
	1249: {66, 1376, 1808},
	1250: {66, 1392, 1800},
	1251: {66, 1408, 1664},
	1252: {66, 1424, 1656},
	1253: {66, 1216, 1632},
	1254: {66, 1200, 1624},
	1255: {66, 1088, 1616},
	1256: {66, 1104, 1608},
	1257: {66, 1136, 1256},
	1258: {66, 1120, 1248},
	1259: {66, 896, 1120},
	1260: {66, 880, 1112},
	1261: {66, 768, 1056},
	1262: {66, 752, 1048},
	1263: {66, 480, 1104},
	1264: {66, 464, 1096},
	1265: {66, 448, 1232},
	1266: {66, 464, 1224},
	1267: {66, 592, 440},
	1268: {66, 576, 432},
	1269: {66, 336, 392},
	1270: {66, 320, 384},
	1271: {66, 1088, 928},
	1272: {66, 1072, 920},
	1273: {66, 928, 784},
	1274: {66, 912, 776},
	1275: {66, 960, 240},
	1276: {66, 944, 232},
	1277: {66, 1680, 1176},
	1278: {66, 1664, 1168},
	1279: {66, 1680, 1368},
	1280: {66, 1696, 1360},
	1281: {66, 1520, 1336},
	1282: {66, 1504, 1328},
	1283: {66, 704, 1856},
	1284: {66, 688, 1848},
}
