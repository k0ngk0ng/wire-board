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

type finaleChecks struct {
	PartyOrders                                    map[string][]int
	Bosses                                         map[int]bool
	Accepted                                       map[int]bool
	PalaceGuards, PalaceGuardsAccepted             bool
	PoisonOrbPrepared                              bool
	CashStarted, CashSelectionSaved, CashCostSaved bool
	CashBefore, CashLast, CashSpent                uint32
	CashCasts                                      int
	EndingClips                                    map[int]bool
	CreditsSeen, CreditsSaved                      bool
}

var finaleReadings = map[string]string{
	"king-greeting": "爹～我是灵儿", "king-forgiveness": "能够得到你的原谅",
	"baiyue-intervenes": "怎可让一位妖女当我们的领袖", "king-false": "真正的巫王早就被我杀了",
	"ling-injured": "还是没有心跳啊", "li-revenge": "可恶！！我替灵儿报仇",
	"baiyue-ambition": "我要风就风、要雨就雨", "baiyue-sacrifice": "迎接这伟大日子的祭品吧",
	"ling-returns": "请你们务必帮我阻止他", "ling-defies": "在没有粉碎你的疯狂的野心",
	"baiyue-defeated": "你已经无路可逃了", "baiyue-death": "这老家伙居然自己了断",
	"peace": "终于恢复和平的日子", "ending-escape": "糟了！快走",
}

var finaleSteps = []struct {
	Scene, Event, Dest int
	Token              string
}{
	{292, 4967, 293, ""}, {293, 4997, 294, ""}, {294, 5051, 280, ""},
	{280, 4751, 289, ""}, {289, 4871, 288, ""}, {288, 4851, 286, ""},
	{286, 4808, 278, ""}, {278, 4736, 282, ""}, {282, 4796, 279, ""},
	{279, 4743, 279, "li-revenge"}, {279, 4738, 282, ""},
	{282, 4801, 282, "ending-escape"},
}

func TestOriginalFinaleRoutePalaceBaiyueAndEnding(t *testing.T) {
	if os.Getenv("PAL_FINALE") != "1" {
		t.Skip("explicit original final maze, palace, Baiyue and complete ending")
	}
	name := "campaign-rain-prayer-state.json"
	root, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	rootHash := sha256.Sum256(root)
	if err := acceptedCampaignRoot("campaign-rain-prayer-state.json", root, "607b5afddcaccd3cf6674b449809695d5b4b39fc8f8ad7eedc283b9c24798ab5"); err != nil {
		t.Fatal(err)
	}
	checkpoint := os.Getenv("PAL_FINALE_CHECKPOINT")
	p := caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}
	checks := finaleChecks{PartyOrders: map[string][]int{}, Bosses: map[int]bool{}, Accepted: map[int]bool{}, EndingClips: map[int]bool{}}
	if checkpoint != "" {
		if checkpoint != "failure" && checkpoint != "prepared" {
			n, e := strconv.Atoi(strings.TrimPrefix(checkpoint, "step-"))
			if e != nil || n < 0 || n > len(finaleSteps) || checkpoint != fmt.Sprint("step-", n) {
				t.Fatal("unsupported finale debug checkpoint", checkpoint)
			}
		}
		name = "finale-" + checkpoint + "-state.json"
		for kind, target := range map[string]any{"proof": &p, "checks": &checks} {
			data, e := os.ReadFile(campaignEvidencePath("finale-"+checkpoint+"-"+kind+".json", 1))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, target); e != nil {
				t.Fatal(e)
			}
		}
	}
	if checks.EndingClips == nil {
		checks.EndingClips = map[int]bool{}
	}
	source, err := os.ReadFile(campaignEvidencePath(name, 1))
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(source)
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, name), prefix: "finale", proof: p, tables: tables, milestones: finaleReadings,
		allowedRoles: map[int]bool{0: true, 1: true, 4: true}, allowedMenus: map[int]bool{6: true, 13: true}, reserveScenes: []int{278, 279, 280, 282, 286, 288, 289, 292, 293, 294}}
	r.w = campaignMapWalker{rememberCollision: false, allowTouch: true, avoidTouches: map[int][][3]int{}}
	r.manualBattle = finaleOriginalCashAction(t)
	save := func(label string) {
		r.save(label)
		if err := os.WriteFile(campaignEvidencePath("finale-"+label+"-checks.json", 1), raw(checks), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r.notice = func(v pal.View) {
		roles := []int{}
		for _, member := range v.Party {
			roles = append(roles, member.Role)
		}
		allowed := false
		for _, order := range [][]int{{1, 0, 4}, {0}, {0, 4}, {0, 1, 4}} {
			allowed = allowed || bytes.Equal(raw(order), raw(roles))
		}
		if !allowed {
			t.Fatal("original finale party order changed", v.Scene, roles)
		}
		label := fmt.Sprint("party-", roles)
		if !r.proof.Saved[label] {
			checks.PartyOrders[label] = roles
			save(label)
		}
		if v.Phase == "field" && v.Seq == v.AppliedSeq {
			label = fmt.Sprint("field-", v.Scene, "-step-", r.proof.Step)
			if !r.proof.Saved[label] {
				save(label)
			}
		}
		if b := v.Battle; b != nil {
			r.battlePartySize = len(roles)
			for _, enemy := range b.Enemies {
				if enemy.Object != 462 && enemy.Object != 528 && enemy.Object != 546 {
					continue
				}
				checks.Bosses[enemy.Object] = true
				if enemy.Object == 546 && !checks.CashStarted {
					checks.CashStarted = true
					checks.CashBefore, checks.CashLast = v.Cash, v.Cash
				}
				if enemy.Object == 528 && v.Scene == 279 {
					checks.PalaceGuards = true
					for _, player := range b.Players {
						if player.State == 2 && !checks.PalaceGuardsAccepted {
							checks.PalaceGuardsAccepted = true
							save("accepted-palace-guards")
						}
					}
				}
				for _, player := range b.Players {
					if player.State == 2 && !checks.Accepted[enemy.Object] {
						checks.Accepted[enemy.Object] = true
						save(fmt.Sprint("accepted-boss-", enemy.Object))
					}
				}
			}
		}
		if checks.CashStarted && r.proof.InBattle {
			if v.Cash < checks.CashLast {
				if checks.CashLast-v.Cash != 5000 {
					t.Fatal("original finale cash-skill spending changed", checks.CashLast, v.Cash)
				}
				checks.CashCasts++
				checks.CashSpent += 5000
				r.proof.BattleSpent += 5000
			}
			checks.CashLast = v.Cash
			if b := v.Battle; b != nil && b.Current >= 0 && b.Current < len(v.Party) && v.Party[b.Current].Role == 4 && b.State == 1 && b.Menu == 1 && v.Seq == v.AppliedSeq && originalVisibleText(v, true) == "乾坤一掷" && !checks.CashSelectionSaved {
				if checks.CashCasts != 0 || v.Cash != checks.CashBefore {
					t.Fatal("finale cash skill charged before original confirmation")
				}
				checks.CashSelectionSaved = true
				save("cash-skill-selected")
			}
			if checks.CashCasts > 0 && !checks.CashCostSaved {
				checks.CashCostSaved = true
				save("cash-skill-cost-animation")
			}
		}
	}
	defer func() {
		if t.Failed() {
			for kind, data := range map[string][]byte{"state": r.g.Data, "proof": raw(r.proof), "checks": raw(checks), "view": raw(r.visible())} {
				if err := os.WriteFile(campaignEvidencePath("finale-failure-"+kind+".json", 1), data, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	for r.proof.Step < len(finaleSteps) {
		q := finaleSteps[r.proof.Step]
		if r.proof.Step == 9 && !checks.PoisonOrbPrepared {
			finaleEquipEarnedPoisonOrb(r)
			checks.PoisonOrbPrepared = true
			save("poison-orb-prepared")
		}
		t.Log("original finale waypoint", r.proof.Step, q)
		r.reserve(q.Event)
		visits := map[[2]int]int{}
		for i := 0; i < 10000; i++ {
			if r.proof.Seen["ending-escape"] {
				break
			}
			v := r.visible()
			if q.Token == "ending-escape" {
				if v.Phase != "field" || v.Seq != v.AppliedSeq {
					r.progress()
					continue
				}
			} else {
				v = sacredTreeField(r)
			}
			if v.Scene == q.Dest && (q.Token == "" || r.proof.Seen[q.Token]) {
				break
			}
			if v.Scene != q.Scene {
				t.Fatal("finale route entered unexpected field", q, v.Scene, v.X, v.Y)
			}
			at := [2]int{v.X, v.Y}
			visits[at]++
			if visits[at] > 60 {
				t.Fatal("finale approach blocked", q, at)
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, xFrom(tables, q.Event), yFrom(tables, q.Event), 0)
			r.progress()
		}
		r.proof.Step++
		save(fmt.Sprint("step-", r.proof.Step))
	}
	for name := range finaleReadings {
		if !r.proof.Seen[name] {
			t.Fatal("original finale milestone missing", name)
		}
	}
	for _, boss := range []int{462, 528, 546} {
		if !checks.Bosses[boss] || !checks.Accepted[boss] {
			t.Fatal("original finale boss or saved accepted action missing", boss)
		}
	}
	if !checks.PalaceGuards || !checks.PalaceGuardsAccepted || !checks.PoisonOrbPrepared {
		t.Fatal("original palace guard battle and its accepted saved action are missing")
	}
	if !checks.CashSelectionSaved || !checks.CashCostSaved || checks.CashCasts == 0 || checks.CashSpent != uint32(checks.CashCasts*5000) {
		t.Fatal("original finale cash-skill input, spending and saves missing", checks)
	}
	for i := 0; i < 40000 && r.visible().Phase != "finished"; i++ {
		v := r.visible()
		if e := v.Ending; e != nil && !e.Finished {
			checks.EndingClips[e.Clip] = true
			position := "first"
			if e.Frame >= e.Frames-8 {
				position = "last"
			} else if e.Frame >= e.Frames/2 {
				position = "middle"
			}
			label := fmt.Sprint("ending-", e.Clip, "-", position)
			if !r.proof.Saved[label] {
				save(label)
			}
		} else if v.Phase == "menu" && v.Menu.Kind == 13 {
			if r.g.Result != nil {
				t.Fatal("platform completion preceded the actual native credit wait")
			}
			checks.CreditsSeen = true
			if !checks.CreditsSaved {
				checks.CreditsSaved = true
				save("additional-credits")
			}
			v = r.visible()
			r.g = apply(t, r.m, r.c, r.g, pal.Action{Type: "input", Epoch: v.Epoch, Seq: v.Seq + 1, ControlAt: v.ControlAt, Direction: 4, Keys: 2})
			if r.visible().Phase == "finished" {
				break
			}
		}
		r.progress()
	}
	if r.visible().Phase != "finished" {
		t.Fatal("original finale did not finish its complete native ending")
	}
	if !checks.CreditsSeen || !checks.CreditsSaved || r.g.Result == nil || !r.g.Result.Unrated {
		t.Fatal("actual ending credits, save or native terminal result missing", checks)
	}
	for _, clip := range []int{4, 5, 6} {
		if !checks.EndingClips[clip] {
			t.Fatal("actual final route omitted an original ending movie", clip)
		}
		for _, position := range []string{"first", "middle", "last"} {
			if !r.proof.Saved[fmt.Sprint("ending-", clip, "-", position)] {
				t.Fatal("actual final route omitted a saved original movie position", clip, position)
			}
		}
	}
	if r.proof.Battles != r.proof.Wins {
		t.Fatal("original finale battle result mismatch", r.proof)
	}
	for name, data := range map[string][]byte{"campaign-finale-state.json": r.g.Data, "campaign-finale.json": raw(map[string]any{"passed": true, "resumedTestRoute": checkpoint != "", "sourceCheckpoint": name, "sourceCheckpointSHA256": hex.EncodeToString(sourceHash[:]), "rootStage23SHA256": hex.EncodeToString(rootHash[:]), "checks": checks, "proof": r.proof})} {
		if err := os.WriteFile(campaignEvidencePath(name, 1), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
