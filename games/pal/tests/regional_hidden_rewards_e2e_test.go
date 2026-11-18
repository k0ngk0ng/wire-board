package pal_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestOriginalRegionalHiddenRewardsAndSavedNoDuplication(t *testing.T) {
	requested := os.Getenv("PAL_HIDDEN_REGION")
	if requested == "" {
		t.Skip("explicit original regional optional rewards")
	}
	specs := map[int]struct {
		Source string
		Events []int
	}{
		2:   {"campaign-ambush-state.json", []int{14, 15, 22, 23, 24}},
		4:   {"campaign-ambush-state.json", []int{66, 67, 68}},
		7:   {"campaign-first-party-state.json", []int{167, 168}},
		9:   {"campaign-first-party-state.json", []int{182, 183, 184, 185}},
		10:  {"campaign-first-party-state.json", []int{190}},
		17:  {"campaign-homecoming-state.json", []int{223, 224, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234}},
		18:  {"hidden-island-entry-state.json", []int{236, 264}},
		20:  {"campaign-homecoming-state.json", []int{334, 335, 336, 337, 338, 339, 340, 341}},
		21:  {"campaign-homecoming-state.json", []int{364, 393, 394}},
		25:  {"campaign-yueru-meeting-state.json", []int{475, 476, 477, 478}},
		28:  {"campaign-yueru-meeting-state.json", []int{516}},
		37:  {"tournament-ling-illness-state.json", []int{608, 609, 610, 611}},
		39:  {"tournament-manor-state.json", []int{628, 629, 630, 631, 632}},
		41:  {"hidden-dragon-stair-4-state.json", []int{668, 669}},
		42:  {"hidden-dragon-stair-6-state.json", []int{684, 685, 686}},
		45:  {"hidden-dragon-cave-party-state.json", []int{713}},
		46:  {"hidden-dragon-stair-2-state.json", []int{742, 743, 745}},
		47:  {"hidden-dragon-stair-1-state.json", []int{757}},
		48:  {"hidden-dragon-open-door-state.json", []int{775, 776, 777, 778, 779, 780, 781, 782, 783, 784, 785, 786, 787, 788, 789}},
		49:  {"campaign-hidden-dragon-state.json", []int{800}},
		51:  {"campaign-hidden-dragon-state.json", nil},
		53:  {"white-river-ginkgo-search-state.json", []int{887}},
		54:  {"campaign-white-river-state.json", []int{911, 912, 913, 914, 915}},
		59:  {"jade-temple-aura-state.json", []int{974, 975, 976, 977, 978, 979, 980, 981, 982, 983}},
		60:  {"blood-pool-complete-state.json", []int{1048, 1073, 1074, 1075, 1076, 1077, 1078, 1079}},
		61:  {"ghost-tomb-waypoint-31-state.json", []int{1168, 1170, 1169, 1171, 1173, 1174}},
		62:  {"ghost-tomb-waypoint-2-state.json", []int{1190}},
		63:  {"ghost-tomb-waypoint-2-state.json", []int{1197, 1198, 1195, 1199, 1200, 1201, 1194, 1202, 1203, 1204}},
		65:  {"blood-pool-earth-orb-escape-complete-state.json", []int{1229, 1230, 1231, 1232, 1233, 1234, 1235, 1236, 1237}},
		66:  {"ghost-tomb-waypoint-31-state.json", []int{1333, 1334, 1335, 1336, 1323, 1324, 1325, 1326, 1327, 1329, 1330, 1331, 1332, 1328}},
		70:  {"ghost-mountain-route-17-state.json", []int{1376, 1377, 1378, 1379}},
		71:  {"ghost-mountain-route-11-state.json", []int{1387, 1388}},
		72:  {"ghost-mountain-route-12-state.json", []int{1396, 1397}},
		73:  {"ghost-mountain-route-12-state.json", []int{1405, 1406}},
		74:  {"ghost-mountain-route-15-state.json", []int{1413}},
		75:  {"ghost-mountain-route-13-state.json", []int{1416}},
		76:  {"ghost-mountain-route-12-state.json", []int{1421, 1422, 1423, 1424, 1425}},
		77:  {"ghost-mountain-route-18-state.json", []int{1479, 1485, 1480, 1481, 1478, 1482, 1483, 1484, 1486}},
		81:  {"yangzhou-step-2-state.json", []int{1516, 1517}},
		87:  {"yangzhou-step-22-state.json", []int{1631}},
		93:  {"yangzhou-step-47-state.json", []int{1697, 1698, 1699, 1700, 1701, 1702, 1703, 1705, 1706, 1707}},
		96:  {"yangzhou-step-5-state.json", []int{1766, 1767}},
		97:  {"yangzhou-step-24-state.json", []int{1779}},
		102: {"toad-mountain-step-2-state.json", []int{1871, 1870, 1873, 1874, 1872, 1875, 1876, 1845, 1869, 1877, 1878, 1879}},
		103: {"toad-mountain-step-1-state.json", []int{1883, 1888, 1889, 1890}},
		104: {"toad-mountain-step-8-state.json", []int{1911, 1919, 1920, 1921, 1922, 1923, 1924, 1925}},
		105: {"toad-mountain-step-10-state.json", []int{1956, 1957, 1958, 1959, 1960, 1961, 1962, 1963, 1964}},
		101: {"capital-manor-step-15-state.json", []int{1818}},
		109: {"capital-manor-step-19-state.json", []int{2035}},
		138: {"capital-manor-step-33-state.json", []int{2336}},
		175: {"sacred-tree-field-175-step-0-state.json", []int{2880, 2881, 2882, 2883, 2884, 2885, 2886, 2887}},
		177: {"sacred-tree-field-177-step-0-state.json", []int{2918, 2919, 2920, 2921, 2922, 2923, 2924, 2932, 2925, 2926, 2927, 2928, 2929, 2930}},
		180: {"campaign-sacred-tree-state.json", []int{2958, 2959}},
		185: {"sacred-tree-field-185-step-8-state.json", []int{3036, 3037, 3038, 3039, 3040, 3041, 3042, 3043, 3031, 3030, 3028, 3029, 3032, 3033, 3034, 3035}},
		189: {"sacred-tree-field-189-step-5-state.json", []int{3158, 3159, 3160}},
		187: {"sacred-tree-field-187-step-2-state.json", []int{3150, 3136, 3137, 3147, 3148, 3149, 3146}},
		193: {"sacred-tree-monkey-3302-fed-state.json", []int{3315}},
		176: {"peach-quest-v1-field-176-state.json", []int{2915}},
		181: {"peach-quest-v1-field-181-state.json", []int{2974, 2973, 2975, 2976, 2977}},
		190: {"peach-quest-v1-field-190-state.json", []int{3188, 3189, 3191, 3192, 3193, 3194, 3195}},
		191: {"peach-quest-v1-field-191-state.json", []int{3211, 3215, 3217, 3214, 3216, 3213}},
		192: {"peach-quest-v1-field-191-state.json", []int{3223, 3224, 3225, 3226, 3227, 3228, 3229, 3230, 3231, 3232, 3233, 3234, 3235, 3236, 3237, 3238, 3239, 3240, 3241, 3242, 3243, 3244}},
		208: {"campaign-dali-kirin-state.json", []int{3513, 3514, 3515, 3516, 3517, 3518, 3519, 3520, 3521, 3522, 3523, 3524, 3525, 3526, 3527, 3528, 3529, 3530, 3531, 3532, 3533, 3534, 3535, 3539, 3536, 3538, 3540, 3541, 3542, 3543, 3544, 3545, 3546}},
		210: {"campaign-dali-kirin-state.json", []int{3576, 3577, 3578, 3579}},
		263: {"temple-past-field-263-step-18-state.json", []int{4580, 4581, 4582, 4583, 4584, 4585, 4586}},
		264: {"temple-past-field-264-step-18-state.json", []int{4601, 4602, 4603, 4604, 4605, 4606}},
		265: {"temple-past-field-262-step-16-state.json", []int{4643, 4644, 4645, 4646}},
		253: {"temple-past-field-253-step-32-state.json", []int{4445, 4444, 4446, 4447, 4448}},
		251: {"temple-past-field-251-step-32-state.json", []int{4415, 4416, 4417, 4418, 4419, 4420, 4421, 4422}},
		220: {"sacred-medicine-trial-entry-state.json", []int{3749, 3750, 3751, 3752, 3753, 3754, 3755, 3756, 3757, 3758, 3759, 3760, 3761, 3762, 3763, 3764, 3765, 3766, 3767, 3768, 3769, 3770, 3771, 3772, 3773, 3774, 3775, 3776, 3777, 3778, 3779, 3780, 3781, 3782, 3783, 3784}},
		221: {"sacred-medicine-trial-entry-state.json", []int{3788, 3789, 3790, 3791, 3792, 3793, 3794, 3795, 3796, 3797, 3798, 3799, 3800, 3801, 3802, 3803, 3804, 3805, 3806, 3807, 3808, 3809}},
		223: {"sacred-medicine-trial-entry-state.json", []int{3846, 3847, 3848, 3849, 3850, 3851, 3852, 3853, 3854, 3855, 3856, 3857, 3858, 3859, 3860, 3861, 3862, 3863, 3864, 3865, 3866, 3867, 3868, 3869, 3870, 3871}},
		224: {"sacred-medicine-trial-entry-state.json", []int{3891, 3892, 3893, 3894, 3895, 3896, 3897, 3898, 3899, 3900, 3901, 3902, 3903, 3904, 3905, 3906, 3907, 3908, 3909, 3910, 3911, 3912, 3913, 3914, 3915, 3916, 3917, 3918, 3919, 3920, 3921}},
		226: {"sacred-medicine-trial-entry-state.json", []int{3958, 3959, 3960, 3961, 3962, 3963, 3964, 3965}},
		227: {"sacred-medicine-trial-entry-state.json", []int{3995, 3996, 3997, 3998, 3999, 4001, 4002, 4003, 4004, 4005, 4006, 4007, 4008}},
		167: {"tower-upper-step-3-state.json", []int{2760, 2761, 2762, 2765, 2766, 2767, 2768, 2769, 2770, 2771, 2772}},
		280: {"finale-field-280-step-3-state.json", []int{4789, 4791, 4790, 4788, 4787}},
		287: {"finale-field-286-step-6-state.json", []int{4850, 4849, 4848}},
		288: {"finale-field-288-step-5-state.json", []int{4870, 4869}},
		289: {"finale-field-289-step-4-state.json", []int{4915, 4916, 4908, 4907, 4906, 4912, 4909, 4910, 4911, 4914, 4913}},
		290: {"finale-field-288-step-5-state.json", []int{4952, 4950, 4951, 4953, 4955, 4954}},
		291: {"finale-field-280-step-3-state.json", []int{4966, 4965, 4964}},
		292: {"finale-field-292-step-0-state.json", []int{4992, 4993, 4994, 4995}},
		293: {"finale-field-293-step-1-state.json", []int{5047, 5041, 5042, 5043, 5040, 5046, 5048, 5044, 5045}},
		294: {"finale-field-294-step-2-state.json", []int{5068, 5069, 5070, 5076, 5075}},
	}
	scene, err := strconv.Atoi(requested)
	spec, ok := specs[scene]
	if err != nil || !ok || campaignVariant() != "audio" || developmentHealthProfile() != "hp-9999" || os.Getenv("PAL_TEST_AUTHORITY") != "1" {
		t.Fatal("regional rewards require a registered current-core authoritative fixture", requested)
	}
	trialRegion := scene == 220 || scene == 221 || scene >= 223 && scene <= 227
	finalRegion := scene == 280 || scene >= 287 && scene <= 294
	towerRemainder := scene == 167
	extendedRegion := finalRegion || towerRemainder
	trialNavigation := trialRegion || extendedRegion
	forestPhase := os.Getenv("PAL_FOREST_PHASE")
	if forestPhase != "" && (scene != 193 || forestPhase != "pre-lift") {
		t.Fatal("unsupported original forest reward phase", forestPhase, scene)
	}
	if forestPhase == "pre-lift" {
		spec.Source = "sacred-tree-monkey-3301-fed-state.json"
		spec.Events = []int{3313, 3314, 3316, 3317}
	}
	scholarOffer := os.Getenv("PAL_HIDDEN_SCHOLAR_OFFER")
	if scholarOffer != "" && (scene != 103 || scholarOffer != "renegotiated") {
		t.Fatal("unsupported original scholar offer branch", scholarOffer, scene)
	}
	if scholarOffer == "renegotiated" {
		spec.Events = []int{1883}
	}
	auditedEvents := []int{}
	if scene == 51 {
		auditedEvents = []int{861, 862, 863, 864, 865, 866, 867, 868, 869, 870}
	}
	if len(spec.Events) == 0 && len(auditedEvents) == 0 {
		t.Fatal("regional reward fixture has no explicit pickup or audit records")
	}
	lotusChestAudit := scene == 18 && os.Getenv("PAL_HIDDEN_LOTUS_CHEST") == "1"
	if lotusChestAudit && len(spec.Events) == 1 {
		spec.Events = append(spec.Events, 264)
	}
	source, err := os.ReadFile(campaignEvidencePath(spec.Source, 1))
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptedCampaignRoot(spec.Source, source, ""); err != nil {
		t.Fatal(err)
	}
	tables, err := resources.Load98Tables(os.DirFS("../../../.local/pal/original-98"))
	if err != nil {
		t.Fatal(err)
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
	for _, candidate := range catalog.Candidates {
		byID[candidate.Event] = candidate
	}
	trialPlan, trialPlanHash := trialRewardRouteCase{}, ""
	if trialNavigation {
		if towerRemainder {
			trialPlan, trialPlanHash = loadTowerRemainderRewardRoute(t, source, spec.Events, tables)
		} else if finalRegion {
			trialPlan, trialPlanHash = loadFinalMazeRewardRoute(t, scene, source, spec.Events, tables)
		} else {
			trialPlan, trialPlanHash = loadTrialRewardRoute(t, scene, source, spec.Events, tables)
		}
		spec.Events = trialPlan.Events
	}
	spots := []towerRewardCandidate{}
	names := []string{}
	specialSprites := map[int]int{1048: 299, 1486: 230, 1631: 380, 1818: 134, 1845: 387, 1883: 83, 2035: 308, 2932: 519}
	for _, event := range spec.Events {
		q, ok := byID[event]
		e := tables.Events[event-1]
		if !ok || q.Scene != scene || q.X != int(e.X) || q.Y != int(e.Y) || q.Sprite != int(e.Sprite) || q.Script != int(e.TriggerScript) || e.TriggerMode < 1 || e.TriggerMode > 3 || len(q.Grants) == 0 || len(q.Messages) == 0 {
			t.Fatal("regional original reward record changed", event)
		}
		if q.Sprite != 0 && q.Sprite != 10 && q.Sprite != 68 && q.Sprite != 69 && q.Sprite != 70 && q.Sprite != 389 && specialSprites[q.Event] != q.Sprite {
			t.Fatal("regional visible reward requires a separate appearance review", event, q.Sprite)
		}
		if event == 1845 {
			if q.Script != 15780 || tables.Scripts[15793].Opcode != 0x1f || tables.Scripts[15793].Operands != [3]uint16{271, 0, 0} || tables.Scripts[15803].Opcode != 1 || tables.Scripts[15804].Opcode != 0x3b {
				t.Fatal("original merchant corpse continuation changed")
			}
			message, err := simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[5423]))
			if err != nil || message != "搜得布包" {
				t.Fatal("original merchant corpse loot notice changed", message, err)
			}
			q.Messages = []string{message}
		}
		if event == 1883 {
			if q.Script != 21184 || tables.Scripts[21207].Opcode != 0xa || tables.Scripts[21207].Operands[0] != 21220 || tables.Scripts[21216].Opcode != 0x1e || tables.Scripts[21216].Operands[0] != 1000 || tables.Scripts[21261].Opcode != 0x1e || tables.Scripts[21261].Operands[0] != 1500 {
				t.Fatal("original scholar payment branches changed")
			}
			messageID := 7833
			if scholarOffer == "renegotiated" {
				q.Grants = append(q.Grants[:0:0], q.Grants...)
				q.Grants[0].PC, q.Grants[0].Cash = 21261, 1500
				messageID = 7856
			}
			message, err := simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[messageID]))
			if err != nil || message == "" {
				t.Fatal("original scholar payment notice missing", err)
			}
			q.Messages = []string{message}
		}
		if event == 1911 {
			if q.Script != 21552 || tables.Scripts[21552].Opcode != 0x16 || tables.Scripts[21552].Operands != [3]uint16{1910, 0, 2} || tables.Scripts[21556].Opcode != 0x1f || tables.Scripts[21556].Operands != [3]uint16{262, 0, 0} || tables.Scripts[21557].Opcode != 1 {
				t.Fatal("original toad corpse opening or orb script changed")
			}
			message, err := simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[7970]))
			if err != nil || !strings.Contains(message, "五毒珠") {
				t.Fatal("original toad orb notice missing", message, err)
			}
			q.Messages = []string{strings.ReplaceAll(message, "\"", "")}
		}
		if event == 1818 {
			message, err := simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[7181]))
			if err != nil || message != "得到一封情书" || q.Script != 19833 || tables.Scripts[19836].Opcode != 0xa || tables.Scripts[19843].Operands != [3]uint16{287, 0, 0} || tables.Scripts[19851].Opcode != 1 {
				t.Fatal("original guard letter request changed", message, err)
			}
			q.Messages = []string{message}
		}
		if scene == 187 {
			for index, message := range q.Messages {
				q.Messages[index] = strings.ReplaceAll(message, "\"", "")
			}
		}
		for _, grant := range q.Grants {
			instruction := tables.Scripts[grant.PC]
			item := grant.Item > 0 && grant.Amount > 0 && grant.Cash == 0 && instruction.Opcode == 0x1f && int(instruction.Operands[0]) == grant.Item && max(1, int(instruction.Operands[1])) == grant.Amount
			cash := grant.Item == 0 && grant.Amount == 0 && grant.Cash > 0 && instruction.Opcode == 0x1e && int(int16(instruction.Operands[0])) == grant.Cash
			if !item && !cash {
				t.Fatal("regional grant needs a separate review", event, grant)
			}
			if item {
				names = append(names, grant.Name)
			}
		}
		spots = append(spots, q)
	}
	if scene == 21 {
		names = append(names, "苗刀")
	}
	if trialNavigation {
		names = append(names, "傀儡虫", "土灵珠")
	}
	if scene == 93 {
		e := tables.Events[1703]
		text, err := simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[12347]))
		if err != nil || text != "获得紫青玉蓉膏" || e.TriggerScript != 37479 || e.Sprite != 389 || e.TriggerMode != 1 || tables.Scripts[37482].Opcode != 0x1f || tables.Scripts[37482].Operands != [3]uint16{} || tables.Scripts[37483].Opcode != 2 || tables.Scripts[37483].Operands[0] != 36509 {
			t.Fatal("original zero-item chest record changed", e, text, err)
		}
		spots = append(spots, towerRewardCandidate{Scene: 93, Map: 85, Event: 1704, X: int(e.X), Y: int(e.Y), Sprite: int(e.Sprite), Script: int(e.TriggerScript), Messages: []string{text}})
		names = append(names, "紫菁玉蓉膏")
	}
	prefix := fmt.Sprint("regional-hidden-v18-", scene)
	if finalRegion {
		prefix = fmt.Sprint("regional-hidden-v19-", scene)
	}
	if towerRemainder {
		prefix = fmt.Sprint("regional-hidden-v20-", scene)
	}
	if scene == 193 {
		prefix += "-post-lift"
		if forestPhase == "pre-lift" {
			prefix = fmt.Sprint("regional-hidden-v18-", scene, "-pre-lift")
		}
	}
	if scholarOffer == "renegotiated" {
		prefix += "-renegotiated"
	}
	if lotusChestAudit {
		prefix += "-chest-audit"
	}
	if err := os.Remove(campaignEvidencePath(prefix+"-result.json", 1)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	r := &caiyiForestDriver{riverMedicineDriver: loadRiverMedicine(t, spec.Source), tables: tables, prefix: prefix, milestones: map[string]string{}, reserveScenes: []int{scene}, physicalAttacks: true, proof: caiyiForestProof{Seen: map[string]bool{}, Saved: map[string]bool{}, Scenes: map[int]int{}}}
	initial := r.visible()
	r.allowedRoles = map[int]bool{}
	for _, member := range initial.Party {
		r.allowedRoles[member.Role] = true
	}
	r.battlePartySize = len(initial.Party)
	forceInputs := 0
	if extendedRegion {
		r.manualBattle = func(v pal.View) ([]byte, bool) {
			a, ok := caiyiForestPhysicalAttack(v)
			if !ok {
				return nil, true
			}
			if v.Battle.State == 1 && v.Battle.Menu == 0 {
				a.Direction, a.Keys = 4, 32768
				forceInputs++
			}
			return raw(a), true
		}
	}

	if scene == 101 {
		r.allowedMenus = map[int]bool{12: true}
	}
	r.w = campaignMapWalker{allowTouch: true, rememberCollision: true, avoidTouches: map[int][][3]int{}}
	if scene == 251 || scene == 253 {
		magic, err := tables.Data.Chunk(4)
		if err != nil {
			t.Fatal(err)
		}
		waterCosts := map[int]int{}
		for _, object := range []int{348, 320} {
			at := int(tables.Objects[object][0])*32 + 24
			if at+2 > len(magic) {
				t.Fatal("original water battle spell data missing", object)
			}
			waterCosts[object] = int(binary.LittleEndian.Uint16(magic[at:]))
		}
		if waterCosts[348] != 44 || waterCosts[320] != 68 {
			t.Fatal("original water battle spell costs changed", waterCosts)
		}
		settled, spellScans := 0, 0
		r.manualBattle = func(v pal.View) ([]byte, bool) {
			b := v.Battle
			if b == nil {
				return nil, false
			}
			water := false
			for _, enemy := range b.Enemies {
				water = water || enemy.Object == 547
			}
			if !water {
				return nil, false
			}
			if b.Phase != 0 || b.State == 0 || v.Seq != v.AppliedSeq {
				return nil, true
			}
			if settled > 0 {
				settled--
				return nil, true
			}
			action, ok := caiyiForestPhysicalAttack(v)
			if !ok {
				return nil, true
			}
			spell, cost := "天剑", waterCosts[348]
			if b.Current >= 0 && b.Current < len(v.Party) && v.Party[b.Current].Role == 3 {
				spell, cost = "雷神", waterCosts[320]
			}
			if b.Current >= 0 && b.Current < len(v.Party) && v.Party[b.Current].MP >= cost {
				if b.State == 1 && b.Menu == 0 {
					if b.Selected != 1 {
						action.Direction, action.Keys = 1, 8
					} else {
						action.Direction, action.Keys = 4, 2
					}
				} else if b.State == 1 && b.Menu == 1 {
					if originalVisibleText(v, true) == spell {
						action.Direction, action.Keys = 4, 2
						spellScans = 0
					} else {
						spellScans++
						if spellScans > 64 {
							t.Fatal("original water reward route spell unavailable", originalVisibleText(v, true), originalVisibleText(v, false))
						}
						action.Direction, action.Keys = 3, 32
					}
				} else if (b.State == 2 || b.State == 4) && b.Action == 3 {
					action.Keys = 2
				}
			}
			settled = 2
			return raw(action), true
		}
	}
	forestBase := map[[2]int][2]int{}
	forestUsed := map[int]bool{3299: true, 3300: true}
	if forestPhase == "pre-lift" {
		forestUsed = map[int]bool{}
	}
	forestLifts := []sacredTreeLift{}
	refreshForest := func() {}
	if scene == 193 {
		climbs := 0
		for index := tables.Scenes[192].FirstEvent; index < tables.Scenes[193].FirstEvent; index++ {
			e := tables.Events[index]
			if e.State <= 0 || e.TriggerMode < 4 || e.TriggerScript == 0 {
				continue
			}
			last, supported := resources.Script{}, true
			for pc := int(e.TriggerScript); pc < int(e.TriggerScript)+16; pc++ {
				s := tables.Scripts[pc]
				if s.Opcode <= 3 {
					break
				}
				switch s.Opcode {
				case 0x7a, 0x7b:
					last = s
				case 0x46, 0x7f, 0x9, 0x6e:
				default:
					supported = false
				}
			}
			if !supported || last.Opcode == 0 || last.Operands[2] > 1 {
				continue
			}
			climbs++
			destination := [2]int{int(last.Operands[0])*32 + int(last.Operands[2])*16, int(last.Operands[1])*16 + int(last.Operands[2])*8}
			sacredTreeTouchArea(forestBase, [2]int{int(e.X), int(e.Y)}, destination, (int(e.TriggerMode)-4)*32+16)
		}
		if climbs != 52 {
			t.Fatal("original east forest climb inventory changed", climbs)
		}
		for _, lift := range sacredTreeLifts(t, tables) {
			if lift.Scene == 193 {
				forestLifts = append(forestLifts, lift)
			}
		}
		refreshForest = func() {
			moves := map[[2]int][2]int{}
			for at, destination := range forestBase {
				moves[at] = destination
			}
			for _, lift := range forestLifts {
				if !forestUsed[lift.Event] {
					sacredTreeTouchArea(moves, lift.Entry, lift.Destination, lift.Radius)
				}
			}
			r.w.sameSceneMoves = map[int]map[[2]int][2]int{193: moves}
		}
		refreshForest()
	}
	if scene == 180 {
		moves := map[[2]int][2]int{}
		for _, event := range []int{2950, 2951, 2952, 2953} {
			e := tables.Events[event-1]
			last := resources.Script{}
			for pc := int(e.TriggerScript); pc < int(e.TriggerScript)+8; pc++ {
				s := tables.Scripts[pc]
				if s.Opcode <= 3 {
					break
				}
				if s.Opcode == 0x7a {
					last = s
				}
			}
			if e.TriggerMode != 5 || last.Opcode != 0x7a || last.Operands[2] > 1 {
				t.Fatal("original mountain stair reference changed", event)
			}
			destination := [2]int{int(last.Operands[0])*32 + int(last.Operands[2])*16, int(last.Operands[1])*16 + int(last.Operands[2])*8}
			sacredTreeTouchArea(moves, [2]int{int(e.X), int(e.Y)}, destination, 48)
		}
		r.w.sameSceneMoves = map[int]map[[2]int][2]int{180: moves}
	}
	if scene == 61 || scene == 66 {
		r.reserveScenes = []int{61, 66}
	}
	if scene == 63 {
		r.reserveScenes = []int{62, 63}
	}
	if scene >= 70 && scene <= 77 {
		r.reserveScenes = []int{69, 70, 71, 72, 73, 74, 75, 76, 77}
	}
	if scene == 96 {
		r.reserveScenes = []int{83, 96}
	}
	if scene == 97 {
		r.reserveScenes = []int{85, 97}
	}
	if scene >= 102 && scene <= 105 {
		r.reserveScenes = []int{102, 103, 104, 105}
	}
	if scene == 138 {
		r.reserveScenes = []int{101, 138}
	}
	if scene == 181 {
		r.reserveScenes = []int{181, 190}
	}
	if scene == 192 {
		r.reserveScenes = []int{176, 191, 192}
	}
	if scene == 208 || scene == 210 {
		r.reserveScenes = []int{207, 208, 209, 210}
	}
	if scene == 265 {
		r.reserveScenes = []int{262, 265}
	}
	if trialNavigation {
		r.reserveScenes = []int{215, 216, 217, 218, 219, 220, 221, 222, 223, 224, 225, 226, 227}
		if finalRegion {
			r.reserveScenes = []int{280, 286, 287, 288, 289, 290, 291, 292, 293, 294}
		}
		if towerRemainder {
			r.reserveScenes = []int{167}
		}
	}
	if scene == 18 {
		r.w.rememberCollision = false
	}
	r.reserve(-1)
	if trialNavigation {
		reserveTrialRewardObjects(r, -1)
	}
	if scene == 251 {
		e := tables.Events[4411]
		r.w.avoidTouches[251] = append(r.w.avoidTouches[251], [3]int{int(e.X), int(e.Y), (int(e.TriggerMode)-4)*32 + 16})
	}
	if scene == 93 {
		e := tables.Events[1691]
		r.w.avoidTouches[93] = append(r.w.avoidTouches[93], [3]int{int(e.X), int(e.Y), (int(e.TriggerMode)-4)*32 + 16})
	}
	cavePortals := map[int][][3]int{}
	if scene >= 41 && scene <= 48 {
		cavePortals = map[int][][3]int{
			41: {{1856, 1888, 48}, {528, 952, 48}},
			42: {{768, 672, 48}, {496, 824, 48}, {1376, 864, 48}, {1200, 968, 48}},
			45: {{1904, 1896, 80}, {720, 1432, 48}, {912, 1304, 48}, {1344, 592, 80}},
			46: {{192, 1488, 48}, {304, 1912, 48}, {1184, 288, 48}},
			47: {{832, 512, 48}, {1440, 768, 48}, {1440, 1408, 48}, {992, 1056, 48}, {496, 712, 48}, {832, 1648, 48}, {528, 1656, 48}, {1728, 1728, 48}, {1552, 1544, 48}, {704, 1056, 48}, {176, 1112, 48}},
			48: {{448, 976, 16}, {1344, 1360, 80}},
		}
		for floor, portals := range cavePortals {
			r.w.avoidTouches[floor] = portals
		}
	}
	transitScenes := map[int]bool{2: true, 4: true, 5: true, 6: true, 7: true, 9: true, 10: true, 15: true, 16: true, 17: true, 18: true, 20: true, 21: true, 23: true, 24: true, 25: true, 26: true, 28: true, 36: true, 37: true, 39: true, 41: true, 42: true, 45: true, 46: true, 47: true, 48: true}
	for _, field := range []int{49, 50, 51, 52, 53, 54, 55, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 69, 70, 71, 72, 73, 74, 75, 76, 77, 81, 83, 85, 87, 93, 94, 96, 97, 101, 102, 103, 104, 105, 109, 138, 176, 181, 190, 191, 192, 207, 208, 209, 210, 251, 253, 262, 263, 264, 265, 266} {
		transitScenes[field] = true
	}
	travel := func(x, y int, label string, arrived func(pal.View) bool) {
		t.Helper()
		visits := map[[3]int]int{}
		for step := 0; step < 1400; step++ {
			current := r.field()
			if arrived(current) {
				return
			}
			at := [3]int{current.Scene, current.X, current.Y}
			visits[at]++
			if !transitScenes[current.Scene] || visits[at] > 20 {
				t.Fatal("original regional room transition blocked", label, at)
			}
			if current.Scene != 2 && current.Scene != 4 {
				r.g = r.w.walk(t, r.m, &r.c, r.g, current, x, y, 0)
				r.progress()
				continue
			}
			direction := nextInnWalk(current, x, y)
			if direction < 0 {
				t.Fatal("original inn transition lost visible movement", label, at)
			}
			r.keys([]uint32{4, 8, 16, 32}[direction], direction)
			r.keys(0, 4)
			r.progress()
		}
		t.Fatal("original regional transition timed out", label)
	}
	tombDoor := func(event, target int) {
		t.Helper()
		e := tables.Events[event-1]
		r.reserve(event)
		travel(int(e.X), int(e.Y), fmt.Sprint("original tomb stair ", event), func(v pal.View) bool { return v.Scene == target })
		r.reserve(-1)
	}
	peachDoor := func(event, target int) {
		t.Helper()
		e := tables.Events[event-1]
		r.reserve(event)
		travel(int(e.X), int(e.Y), fmt.Sprint("original Peach doorway ", event), func(v pal.View) bool { return v.Scene == target })
		r.reserve(-1)
		r.save(fmt.Sprint("peach-door-", event, "-arrived"))
	}
	daliDoor := func(event, target int) {
		t.Helper()
		e := tables.Events[event-1]
		r.reserve(event)
		travel(int(e.X), int(e.Y), fmt.Sprint("original Dali doorway ", event), func(v pal.View) bool { return v.Scene == target })
		r.reserve(-1)
		r.save(fmt.Sprint("dali-door-", event, "-arrived"))
	}
	pastDoor := func(event, target int) {
		t.Helper()
		e := tables.Events[event-1]
		r.reserve(event)
		travel(int(e.X), int(e.Y), fmt.Sprint("original past doorway ", event), func(v pal.View) bool { return v.Scene == target })
		r.reserve(-1)
		r.save(fmt.Sprint("past-door-", event, "-arrived"))
	}
	defer func() {
		if t.Failed() {
			for suffix, value := range map[string][]byte{"state": r.g.Data, "view": raw(r.visible()), "proof": raw(r.proof)} {
				if err := os.WriteFile(campaignEvidencePath(prefix+"-failure-"+suffix+".json", 1), value, 0600); err != nil {
					t.Log(err)
				}
			}
		}
	}()
	if scene == 4 || scene == 7 || scene == 9 || scene == 10 {
		if scene == 4 {
			travel(416, 400, "aunt room to upper corridor", func(v pal.View) bool { return v.Scene == 4 })
		} else {
			travel(1152, 384, "original bedroom to upper corridor", func(v pal.View) bool { return v.Scene == 4 })
			travel(1296, 1368, "upper corridor railing", func(v pal.View) bool { return v.Scene == 4 && v.X == 1296 && v.Y == 1368 })
		}
		travel(1168, 1368, "original descending stair", func(v pal.View) bool { return v.Scene == 4 && v.X >= 1248 && v.Y >= 1456 })
	}
	if scene == 7 || scene == 9 || scene == 10 {
		travel(976, 1704, "original inn door to village", func(v pal.View) bool { return v.Scene == 5 })
		if scene == 7 {
			travel(128, 1344, "village to original mountain path", func(v pal.View) bool { return v.Scene == 7 })
		} else if scene == 9 {
			travel(1520, 1640, "village to northern home", func(v pal.View) bool { return v.Scene == 9 })
		} else {
			travel(1824, 1328, "village to original harbor", func(v pal.View) bool { return v.Scene == 6 })
			if scene == 10 {
				travel(640, 1680, "harbor to southern home", func(v pal.View) bool { return v.Scene == 10 })
			}
		}
	}
	if scene == 17 {
		travel(224, 1120, "water palace surroundings to original forest", func(v pal.View) bool { return v.Scene == 17 })
	}
	if scene == 21 {
		travel(1088, 592, "water palace surroundings to original interior", func(v pal.View) bool { return v.Scene == 21 })
	}
	if scene == 18 {
		travel(1472, 448, "original shore to island forest", func(v pal.View) bool { return v.Scene == 16 })
		travel(1824, 944, "original forest to lotus maze", func(v pal.View) bool { return v.Scene == 18 })
	}
	if scene == 25 {
		travel(1120, 1424, "Suzhou street to southwestern home", func(v pal.View) bool { return v.Scene == 25 })
	}
	if scene == 28 {
		travel(880, 1080, "Suzhou southern to northern street", func(v pal.View) bool { return v.Scene == 24 })
		travel(1088, 800, "Suzhou street to original inn", func(v pal.View) bool { return v.Scene == 26 })
		r.reserveScenes = []int{26, 28}
		r.reserve(498)
		travel(880, 632, "original inn ascending stair", func(v pal.View) bool { return v.Scene == 26 && v.Y <= 560 })
		travel(704, 448, "upper inn corridor to northern room", func(v pal.View) bool { return v.Scene == 28 })
	}
	if scene == 37 && r.field().Scene == 36 {
		travel(464, 872, "original manor garden to Ling'er room", func(v pal.View) bool { return v.Scene == 37 })
	}
	if scene == 51 {
		travel(1824, 1504, "original southern fields to White River village", func(v pal.View) bool { return v.Scene == 50 })
		travel(928, 832, "White River village to original rice shop", func(v pal.View) bool { return v.Scene == 51 })
	}
	if scene == 59 {
		travel(992, 560, "original Jade Buddha temple entrance", func(v pal.View) bool { return v.Scene == 59 })
	}
	if scene == 63 {
		r.reserve(1177)
		travel(1280, 800, "Blackwater street to original northern house", func(v pal.View) bool { return v.Scene == 63 })
		r.reserve(-1)
	}
	if scene == 61 {
		tombDoor(1239, 61)
		tombDoor(1125, 66)
		tombDoor(1245, 61)
		tombDoor(1127, 66)
		tombDoor(1238, 61)
	}
	if scene == 72 || scene == 73 || scene == 76 {
		event := map[int]int{72: 1359, 73: 1361, 76: 1367}[scene]
		entrance := tables.Events[event-1]
		r.reserve(event)
		travel(int(entrance.X), int(entrance.Y), fmt.Sprint("original optional ghost-mountain cave entrance ", event), func(v pal.View) bool { return v.Scene == scene })
		r.reserve(-1)
	}
	if scene == 96 || scene == 97 {
		event := map[int]int{96: 1552, 97: 1576}[scene]
		entrance := tables.Events[event-1]
		r.reserve(event)
		travel(int(entrance.X), int(entrance.Y), fmt.Sprint("original Yangzhou house entrance ", event), func(v pal.View) bool { return v.Scene == scene })
		r.reserve(-1)
	}
	if scene == 138 {
		r.reserve(1821)
		travel(736, 432, "original capital merchant house entrance", func(v pal.View) bool { return v.Scene == 138 })
		r.reserve(-1)
	}
	if scene == 192 {
		peachDoor(3197, 176)
		peachDoor(2890, 192)
	}
	if scene == 208 {
		daliDoor(3466, 208)
	}
	if scene == 210 {
		daliDoor(3460, 209)
		daliDoor(3551, 210)
	}
	if scene == 265 {
		pastDoor(4564, 265)
	}
	initial = r.field()
	if extendedRegion && [3]int{initial.Scene, initial.X, initial.Y} != trialPlan.SourcePosition {
		t.Fatal("final maze source placement changed", initial.Scene, initial.X, initial.Y, trialPlan.SourcePosition)
	}
	if initial.Scene != scene && (!trialNavigation || initial.Scene != 216 && !finalRegion) || len(initial.Party) == 0 {
		t.Fatal("actual regional parent did not reach its original field", initial.Scene)
	}
	if len(auditedEvents) != 0 {
		for _, event := range auditedEvents {
			candidate := byID[event]
			original := tables.Events[event-1]
			if candidate.Scene != scene || candidate.Sprite != 0 || original.Sprite != 0 || candidate.Script != int(original.TriggerScript) || candidate.X != int(original.X) || candidate.Y != int(original.Y) {
				t.Fatal("original rice shop audit record changed", event)
			}
			for _, actor := range initial.Field.Actors {
				if actor[0] == 1 && actor[1] == event {
					t.Fatal("unrendered original rice shop record gained a visible marker", event)
				}
			}
		}
		r.save("rice-shop-records-preserved")
	}
	inventory := func() map[string]int {
		if !trialNavigation {
			return r.inventoryCounts(names...)
		}
		for attempt := 0; attempt < 8; attempt++ {
			before := r.proof.Battles
			counts := trialWormInventory(r, "", names...)
			if r.proof.Battles == before {
				return counts
			}
		}
		t.Fatal("original trial inventory did not settle after normal encounters")
		return nil
	}
	expected := inventory()
	inventoryInitial := map[string]int{}
	for name, amount := range expected {
		inventoryInitial[name] = amount
	}
	initial = r.field()
	preCollectionBattles := r.proof.Battles
	expectedCash := initial.Cash
	combatAllowed := scene == 7 || scene == 18 || scene >= 41 && scene <= 48 || scene >= 60 && scene <= 66 || scene >= 70 && scene <= 77 || scene == 81 || scene == 87 || scene == 93 || scene >= 102 && scene <= 105 || scene == 180 || scene == 185 || scene == 189 || scene == 193 || scene == 181 || scene == 190 || scene == 191 || scene == 251 || scene == 253 || trialNavigation
	battleCashChanges := []map[string]any{}
	battleItemGrants := []map[string]any{}
	currentFruitDrops, currentFruitBattle := 0, 0
	fruitDropMessage := ""
	currentWormEnemies, currentWormBattle := map[int]int{}, 0
	wormSlots := map[int]int{}
	wormAppearances := []map[string]any{}
	wormDropMessage := ""
	if trialNavigation {
		if tables.Scripts[42828].Opcode != 0x1f || tables.Scripts[42828].Operands != [3]uint16{152, 0, 0} || tables.Scripts[42830].Opcode != 0xffff || tables.Scripts[42830].Operands[0] != 13349 || tables.Scripts[42831].Opcode != 0 {
			t.Fatal("original trial worm end-script reward changed")
		}
		wormDropMessage, err = simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[13349]))
		if err != nil || wormDropMessage != "获得一只傀儡虫" {
			t.Fatal("original trial worm drop notice changed", wormDropMessage, err)
		}
	}
	if scene == 181 {
		if tables.Objects[402][3] != 41008 || tables.Scripts[41008].Opcode != 0x1f || tables.Scripts[41008].Operands != [3]uint16{104, 0, 0} || tables.Scripts[41010].Opcode != 0xffff || tables.Scripts[41010].Operands[0] != 13115 {
			t.Fatal("original mouse fruit enemy reward changed")
		}
		fruitDropMessage, err = simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[13115]))
		if err != nil || fruitDropMessage == "" {
			t.Fatal("original mouse fruit battle notice missing", err)
		}
	}
	lastForestPose := [2]int{initial.X, initial.Y}
	observeBattle := func(v pal.View) {
		if trialNavigation {
			if v.Battle != nil && !r.proof.InBattle {
				currentWormEnemies, currentWormBattle = map[int]int{}, r.proof.Battles+1
				wormSlots = map[int]int{}
				for _, enemy := range v.Battle.Enemies {
					if !extendedRegion && enemy.Object > 0 && enemy.Object < len(tables.Objects) && tables.Objects[enemy.Object][3] == 42828 {
						currentWormEnemies[enemy.Object]++
					}
				}
			}
			if extendedRegion && v.Battle != nil {
				for slot, enemy := range v.Battle.Enemies {
					if enemy.Object > 0 && enemy.Object < len(tables.Objects) && tables.Objects[enemy.Object][3] == 42828 && wormSlots[slot] != enemy.Object {
						currentWormEnemies[enemy.Object]++
						label := fmt.Sprint("battle-", currentWormBattle, "-worm-appearance-", len(wormAppearances)+1)
						wormAppearances = append(wormAppearances, map[string]any{"battle": currentWormBattle, "enemy": enemy.Object, "slot": slot, "label": label, "afterInitialObservation": r.proof.InBattle})
						r.save(label)
					}
					wormSlots[slot] = enemy.Object
				}
			}
			if len(currentWormEnemies) > 0 && strings.Contains(hiddenNoticeText(v), wormDropMessage) && !r.proof.Saved[fmt.Sprint("battle-", currentWormBattle, "-worm-drop")] {
				r.save(fmt.Sprint("battle-", currentWormBattle, "-worm-drop"))
			}
			if r.proof.InBattle && v.Battle == nil {
				enemies := []int{}
				for enemy := range currentWormEnemies {
					enemies = append(enemies, enemy)
				}
				sort.Ints(enemies)
				for _, enemy := range enemies {
					amount := currentWormEnemies[enemy]
					expected["傀儡虫"] += amount
					battleItemGrants = append(battleItemGrants, map[string]any{"battle": currentWormBattle, "enemy": enemy, "endScript": 42828, "item": 152, "name": "傀儡虫", "amount": amount})
				}
			}
		}
		if scene == 181 {
			if v.Battle != nil && !r.proof.InBattle {
				currentFruitDrops, currentFruitBattle = 0, r.proof.Battles+1
				for _, enemy := range v.Battle.Enemies {
					if enemy.Object == 402 {
						currentFruitDrops++
					}
				}
			}
			if currentFruitDrops > 0 && strings.Contains(hiddenNoticeText(v), fruitDropMessage) && !r.proof.Saved[fmt.Sprint("battle-", currentFruitBattle, "-fruit-drop")] {
				r.save(fmt.Sprint("battle-", currentFruitBattle, "-fruit-drop"))
			}
			if r.proof.InBattle && v.Battle == nil && currentFruitDrops > 0 {
				expected["鼠儿果"] += currentFruitDrops
				battleItemGrants = append(battleItemGrants, map[string]any{"battle": currentFruitBattle, "enemy": 402, "endScript": 41008, "item": 104, "name": "鼠儿果", "amount": currentFruitDrops})
			}
		}
		if scene == 193 {
			onLift := false
			for _, draw := range v.Draws {
				onLift = onLift || draw.Kind == 0 && draw.A == 541 && draw.C == 0
			}
			if onLift {
				for _, lift := range forestLifts {
					if !forestUsed[lift.Event] && fieldTestDistance(lastForestPose[0], lastForestPose[1], lift.Entry[0], lift.Entry[1]) < 160 {
						forestUsed[lift.Event] = true
						refreshForest()
						r.save(fmt.Sprint("forest-lift-", lift.Event, "-in-progress"))
					}
				}
			}
			if v.Phase == "field" {
				lastForestPose = [2]int{v.X, v.Y}
			}
		}
		if scene == 180 && v.Phase == "script" && !r.proof.Saved["mountain-climb-in-progress"] {
			r.save("mountain-climb-in-progress")
		}
		if r.proof.InBattle && v.Battle == nil {
			if !combatAllowed || r.proof.BattleSpent != 0 || r.proof.RewardCash < 0 || int64(v.Cash) != int64(r.proof.BattleCash)+int64(r.proof.RewardCash) {
				t.Fatal("regional original battle wallet or physical-only cost mismatch", v.Cash, r.proof)
			}
			expectedCash += uint32(r.proof.RewardCash)
			battleCashChanges = append(battleCashChanges, map[string]any{"cashBefore": r.proof.BattleCash, "cashAfter": v.Cash, "rewardCash": r.proof.RewardCash})
			r.w.known = nil
		}
	}
	r.notice = observeBattle
	if scene == 265 {
		for _, event := range []int{4628, 4630, 4632} {
			e := tables.Events[event-1]
			r.save(fmt.Sprint("past-monster-", event, "-before"))
			opened := false
			for step := 0; step < 600; step++ {
				v := r.field()
				if v.Scene != 265 {
					t.Fatal("original breeding chamber switch left its room", event, v.Scene)
				}
				for _, actor := range v.Field.Actors {
					opened = opened || actor[0] == 1 && actor[1] == event && actor[5] != 0
				}
				if opened {
					break
				}
				if fieldTestDistance(v.X, v.Y, int(e.X), int(e.Y)) <= 96 {
					r.search(event, int(e.X), int(e.Y))
				} else {
					r.g = r.w.walk(t, r.m, &r.c, r.g, v, int(e.X), int(e.Y), 96)
				}
				r.progress()
			}
			if !opened {
				t.Fatal("original breeding chamber switch did not open", event)
			}
			r.save(fmt.Sprint("past-monster-", event, "-opened"))
			r.w.known = nil
		}
	}
	caveDoor := func(floor, x, y int, label string, target int) {
		t.Helper()
		r.w.avoidTouches[floor] = nil
		for _, portal := range cavePortals[floor] {
			if portal[0] != x || portal[1] != y {
				r.w.avoidTouches[floor] = append(r.w.avoidTouches[floor], portal)
			}
		}
		travel(x, y, label, func(v pal.View) bool { return v.Scene == target })
		r.w.avoidTouches[floor] = cavePortals[floor]
	}
	claimed := []map[string]any{}
	trialRouteTrace := []map[string]any{}
	emptyChestAudits := []map[string]any{}
	seen := map[int]bool{}
	tombPlate := func(event int) {
		t.Helper()
		plate := tables.Events[event-1]
		travel(int(plate.X), int(plate.Y), fmt.Sprint("original optional tomb pressure plate ", event), func(v pal.View) bool {
			for _, actor := range v.Field.Actors {
				if actor[0] == 1 && actor[1] == event && actor[5] == 1 {
					return true
				}
			}
			return false
		})
		r.save(fmt.Sprint("tomb-optional-switch-", event))
		r.w.known = nil
	}
	for _, q := range spots {
		if trialNavigation {
			step := trialPlan.Steps[len(claimed)]
			if step.Event != q.Event {
				t.Fatal("trial reward execution order changed", q.Event, step.Event)
			}
			for _, leg := range step.PortalsBefore {
				trialRouteTrace = append(trialRouteTrace, followTrialRewardLeg(r, leg, len(trialRouteTrace)+1))
				if leg.Kind == "earth-return" && (!bytes.Equal(raw(expected), raw(inventory())) || r.field().Cash != expectedCash) {
					t.Fatal("original trial earth return changed already earned inventory or wallet")
				}
			}
		}
		if scene == 60 && q.Event == 1048 {
			for _, event := range []int{1042, 1047} {
				plate := tables.Events[event-1]
				travel(int(plate.X), int(plate.Y), fmt.Sprint("original optional blood-pool plate ", event), func(v pal.View) bool {
					for _, actor := range v.Field.Actors {
						if actor[0] == 1 && actor[1] == event && actor[5] == 1 {
							return true
						}
					}
					return false
				})
				r.save(fmt.Sprint("blood-pool-optional-switch-", event))
				r.w.known = nil
			}
		}
		if scene == 61 {
			switch q.Event {
			case 1169:
				tombDoor(1119, 66)
				tombDoor(1246, 61)
			case 1173:
				tombDoor(1126, 66)
				tombDoor(1244, 61)
			case 1174:
				tombDoor(1120, 66)
				tombDoor(1242, 61)
				tombPlate(1146)
			}
		}
		if scene == 66 {
			switch q.Event {
			case 1323:
				tombDoor(1239, 61)
				tombDoor(1125, 66)
			case 1329:
				tombDoor(1245, 61)
				tombDoor(1127, 66)
			case 1328:
				tombDoor(1246, 61)
				tombDoor(1126, 66)
				tombDoor(1244, 61)
				tombDoor(1120, 66)
				tombDoor(1242, 61)
				tombPlate(1147)
				tombDoor(1129, 66)
			}
		}
		if scene == 63 {
			switch q.Event {
			case 1195:
				r.reserve(1191)
				travel(928, 784, "northern Blackwater house to original street", func(v pal.View) bool { return v.Scene == 62 })
				r.reserve(1178)
				travel(912, 664, "Blackwater street to original southern house", func(v pal.View) bool { return v.Scene == 63 })
			case 1194:
				r.reserve(1192)
				travel(960, 1616, "southern Blackwater house to original street", func(v pal.View) bool { return v.Scene == 62 })
				r.reserve(1179)
				travel(528, 520, "Blackwater street to original western house", func(v pal.View) bool { return v.Scene == 63 })
			}
			r.reserve(-1)
		}
		if scene == 45 && q.Event == 713 {
			caveDoor(45, 720, 1432, "cave first floor to original short corridor", 47)
			caveDoor(47, 832, 512, "short corridor to original chest alcove", 45)
		}
		if scene == 39 {
			r.reserveScenes = []int{36, 39}
			switch q.Event {
			case 630:
				r.reserve(622)
				travel(1392, 1176, "eastern manor room to original garden", func(v pal.View) bool { return v.Scene == 36 })
				r.reserve(587)
				travel(1104, 104, "garden to northern manor room", func(v pal.View) bool { return v.Scene == 39 && v.X < 1000 && v.Y < 1000 })
			case 631:
				r.reserve(621)
				travel(384, 800, "northern manor room to original garden", func(v pal.View) bool { return v.Scene == 36 })
				r.reserve(589)
				travel(1792, 448, "garden to southern manor room", func(v pal.View) bool { return v.Scene == 39 && v.X < 1000 && v.Y > 1000 })
			}
			r.reserve(-1)
		}
		if scene == 25 {
			switch q.Event {
			case 476:
				r.reserve(464)
				travel(272, 1096, "southwestern home to Suzhou street", func(v pal.View) bool { return v.Scene == 23 })
				travel(1344, 1536, "Suzhou street to southeastern home", func(v pal.View) bool { return v.Scene == 25 && v.X > 1000 })
			case 477:
				r.reserve(465)
				travel(1152, 1136, "southeastern home to Suzhou street", func(v pal.View) bool { return v.Scene == 23 })
				travel(880, 1080, "Suzhou southern to northern street", func(v pal.View) bool { return v.Scene == 24 })
				travel(1440, 272, "Suzhou street to northwestern home", func(v pal.View) bool { return v.Scene == 25 && v.X < 1000 && v.Y < 600 })
			case 478:
				r.reserve(462)
				travel(320, 480, "northwestern home to Suzhou street", func(v pal.View) bool { return v.Scene == 24 })
				travel(1632, 368, "Suzhou street to northeastern home", func(v pal.View) bool { return v.Scene == 25 && v.X > 1000 && v.Y < 600 })
			}
			r.reserve(-1)
		}
		if scene == 18 && q.Event == 264 {
			travel(1296, 584, "original K lotus and its blocking gate", func(v pal.View) bool {
				if v.Scene != 18 || v.Field == nil {
					return false
				}
				for _, actor := range v.Field.Actors {
					if actor[0] == 1 && actor[1] == 260 && actor[3] == 1280 && actor[4] == 560 {
						return true
					}
				}
				return false
			})
			r.save("lotus-K-gate-opened")
		}
		original := tables.Events[q.Event-1]
		originalEmpty := q.Event == 1704
		inventoryBeforeEmpty := map[string]int{}
		if originalEmpty {
			for name, amount := range expected {
				inventoryBeforeEmpty[name] = amount
			}
		}
		questBundle := q.Event == 1631
		scholar := q.Event == 1883
		merchantCorpse := q.Event == 1845
		if questBundle && (q.Script != 14041 || q.Sprite != 380 || tables.Scripts[14041].Opcode != 0x49 || tables.Scripts[14041].Operands != [3]uint16{0xffff, 0, 0} || tables.Scripts[14042].Opcode != 5 || tables.Scripts[14045].Opcode != 0x1f || tables.Scripts[14045].Operands != [3]uint16{271, 0, 0} || tables.Scripts[14046].Opcode != 0) {
			t.Fatal("original disappearing quest bundle script changed", q.Event)
		}
		groundWeapon := q.Event == 1486
		groundVial := q.Event == 2035
		renewable := q.Event == 2922
		renewableBefore := expected["鼠儿果"]
		renewableSearches := []map[string]any{}
		if renewable && (q.Script != 28191 || q.Sprite != 0 || tables.Scripts[28193].Opcode != 0x1f || tables.Scripts[28193].Operands != [3]uint16{104, 0, 0} || tables.Scripts[28194].Opcode != 0) {
			t.Fatal("original renewable fruit entry or quantity changed")
		}
		banana := q.Event == 2932
		if banana && (q.Script != 25980 || q.Sprite != 519 || tables.Scripts[25980].Opcode != 0x14 || tables.Scripts[25980].Operands != [3]uint16{1, 0, 0} || tables.Scripts[25984].Opcode != 0x1f || tables.Scripts[25984].Operands != [3]uint16{291, 2, 0} || tables.Scripts[25985].Opcode != 1) {
			t.Fatal("original banana tree pickup and frame changed")
		}
		if banana {
			for index, message := range q.Messages {
				q.Messages[index] = strings.ReplaceAll(message, "\"", "")
			}
		}
		if q.Sprite == 70 && (tables.Scripts[q.Script].Opcode != 0x49 || tables.Scripts[q.Script].Operands != [3]uint16{65535, 0, 0}) {
			t.Fatal("original fungus or vine disappearance changed", q.Event)
		}
		if groundVial && (q.Script != 21569 || tables.Scripts[21569].Opcode != 0x49 || tables.Scripts[21569].Operands != [3]uint16{0xffff, 0, 0} || tables.Scripts[21573].Opcode != 0x1f || tables.Scripts[21573].Operands != [3]uint16{109, 0, 0}) {
			t.Fatal("original garden vial script changed")
		}
		if groundWeapon && (q.Script != 36921 || q.Sprite != 230 || tables.Scripts[36921].Opcode != 0x49 || tables.Scripts[36921].Operands != [3]uint16{0xffff, 0, 0} || tables.Scripts[36922].Opcode != 5 || tables.Scripts[36925].Opcode != 0x1f || tables.Scripts[36925].Operands != [3]uint16{174, 0, 0} || tables.Scripts[36926].Opcode != 0) {
			t.Fatal("original disappearing ground weapon script changed", q.Event)
		}
		persistentBody := q.Event == 1048
		if persistentBody && (q.Script != 12349 || q.Sprite != 299 || tables.Scripts[12351].Opcode != 0x1f || tables.Scripts[12351].Operands != [3]uint16{152, 2, 0} || tables.Scripts[12352].Opcode != 1 || tables.Scripts[12355].Opcode != 0) {
			t.Fatal("original two-worm persistent body script changed", q.Event)
		}
		closedFrame, openFrame := -1, -1
		disappears := q.Sprite == 68 || q.Sprite == 69 || q.Sprite == 70 || groundWeapon || questBundle || scholar || groundVial
		if q.Sprite == 10 || q.Sprite == 389 {
			closedFrame = int(original.Direction*original.Frames + original.Frame)
			call := tables.Scripts[q.Script]
			if call.Opcode == 4 && tables.Scripts[call.Operands[0]].Opcode == 0x14 {
				openFrame = int(tables.Scripts[call.Operands[0]].Operands[0])
			} else {
				for pc := q.Script; pc < q.Script+16 && pc < len(tables.Scripts); pc++ {
					script := tables.Scripts[pc]
					if script.Opcode <= 3 {
						break
					}
					disappears = disappears || script.Opcode == 0x49 && script.Operands[0] == 0xffff && script.Operands[1] == 0
				}
				if !disappears {
					t.Fatal("regional original object appearance requires a separate review", q.Event)
				}
			}
		}
		if scene == 9 && q.Event == 183 {
			r.reserve(175)
			travel(624, 520, "northern home to village", func(v pal.View) bool { return v.Scene == 5 })
			travel(1280, 1232, "village to southern home", func(v pal.View) bool { return v.Scene == 9 && v.Y > 1000 })
			r.reserve(-1)
		}
		if scene == 2 && q.Event == 14 {
			travel(416, 400, "aunt room to upper corridor", func(v pal.View) bool { return v.Scene == 4 })
			travel(1296, 1368, "upper corridor railing", func(v pal.View) bool { return v.Scene == 4 && v.X == 1296 && v.Y == 1368 })
			travel(1456, 1400, "guest room door", func(v pal.View) bool { return v.Scene == 2 && v.X > 1000 && v.Y > 800 })
		}
		if scene == 2 && q.Event == 22 {
			travel(1200, 1128, "guest room to corridor", func(v pal.View) bool { return v.Scene == 4 })
			travel(1168, 1368, "original descending stair", func(v pal.View) bool { return v.Scene == 4 && v.X >= 1248 && v.Y >= 1456 })
			travel(992, 1472, "lower corridor kitchen door", func(v pal.View) bool { return v.Scene == 2 && v.X < 1000 && v.Y > 800 })
			travel(528, 1208, "original kitchen threshold", func(v pal.View) bool { return v.Scene == 2 && v.X == 528 && v.Y == 1208 })
			travel(560, 1176, "inside original kitchen", func(v pal.View) bool { return v.Scene == 2 && v.X >= 544 && v.Y <= 1184 })
		}
		if scene == 181 && q.Event == 2977 {
			peachDoor(2971, 190)
			peachDoor(3162, 181)
		}
		if scene == 192 {
			for _, pair := range map[int][][2]int{3227: {{3220, 176}, {2892, 192}}, 3231: {{3222, 176}, {2889, 192}}, 3236: {{3219, 176}, {2891, 192}}, 3243: {{3221, 176}, {2888, 192}}}[q.Event] {
				peachDoor(pair[0], pair[1])
			}
		}
		if scene == 208 {
			for _, pair := range map[int][][2]int{3521: {{3499, 207}, {3463, 208}}, 3528: {{3496, 207}, {3464, 208}}, 3532: {{3497, 207}, {3465, 208}}, 3539: {{3498, 207}, {3470, 208}}, 3536: {{3503, 207}, {3469, 208}}, 3543: {{3502, 207}, {3468, 208}}}[q.Event] {
				daliDoor(pair[0], pair[1])
			}
		}
		label := fmt.Sprint("reward-", q.Event)
		scholarSearches := 0
		scholarDecisions := []string{}
		scholarDeclined := false
		escortFollower := 0
		bladeMessage, bladeSeen, bladeBefore := "", false, expected["苗刀"]
		vesselMessage, vesselSeen, vesselBefore := "", false, expected["炼蛊皿"]
		if q.Event == 2886 {
			if q.Script != 28162 || tables.Scripts[28165].Opcode != 1 || tables.Scripts[28166].Opcode != 0x3e || tables.Scripts[28168].Opcode != 0x1f || tables.Scripts[28168].Operands != [3]uint16{268, 0, 0} || tables.Scripts[28169].Opcode != 1 || tables.Scripts[28170].Opcode != 0 {
				t.Fatal("original silkworm followup vessel changed")
			}
			vesselMessage, err = simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[tables.Scripts[28167].Operands[0]]))
			if err != nil || vesselMessage != "得到一只炼蛊皿" {
				t.Fatal("original followup vessel message changed", vesselMessage, err)
			}
		}
		if q.Event == 364 {
			grant := tables.Scripts[5149]
			name, err := simplifiedchinese.GBK.NewDecoder().String(string(tables.Words[179]))
			if err != nil || name != "苗刀" || grant.Opcode != 0x1f || grant.Operands[0] != 179 || grant.Operands[1] != 0 || tables.Scripts[5143].Opcode != 1 || tables.Scripts[5146].Opcode != 1 || tables.Scripts[5150].Opcode != 1 || tables.Scripts[5153].Opcode != 0 || tables.Scripts[5148].Opcode != 0xffff || tables.Scripts[5148].Operands[0] != 1893 {
				t.Fatal("original palace chest hidden blade branch changed", grant, name, err)
			}
			bladeMessage, err = simplifiedchinese.GBK.NewDecoder().String(string(tables.Messages[1893]))
			if err != nil || bladeMessage == "" {
				t.Fatal("original hidden blade notice could not be decoded", err)
			}
		}
		r.notice = func(v pal.View) {
			observeBattle(v)
			if (scholar || q.Event == 1818) && v.Phase == "menu" && v.Seq == v.AppliedSeq {
				want := "是"
				if scholarOffer == "renegotiated" && !scholarDeclined {
					want = "否"
				}
				selected := -1
				for index, item := range v.Menu.Items {
					if item.Label == want {
						selected = index
					}
				}
				if selected >= 0 {
					if v.Menu.Selected != selected {
						direction, keys := 3, uint32(32)
						if selected < v.Menu.Selected {
							direction, keys = 1, 8
						}
						r.keys(keys, direction)
					} else {
						r.save(fmt.Sprint(label, "-decision-", len(scholarDecisions)+1))
						scholarDecisions = append(scholarDecisions, want)
						scholarDeclined = scholarDeclined || want == "否"
						r.keys(2, 4)
					}
				}
			}
			if !seen[q.Event] {
				noticeText := hiddenNoticeText(v)
				if (q.Event == 1911 || scene == 177 || scene == 187 || scene == 193) && v.Dialogue != nil {
					noticeText += strings.ReplaceAll(v.Dialogue.Text, "\"", "")
				}
				for _, message := range q.Messages {
					if strings.Contains(noticeText, message) {
						seen[q.Event] = true
						r.save(label + "-notice")
						break
					}
				}
			}
			if bladeMessage != "" && !bladeSeen && strings.Contains(hiddenNoticeText(v), bladeMessage) {
				bladeSeen = true
				r.save(label + "-blade-notice")
			}
			if vesselMessage != "" && !vesselSeen && strings.Contains(hiddenNoticeText(v), vesselMessage) {
				vesselSeen = true
				r.save(label + "-vessel-notice")
			}
		}
		visits := map[[2]int]int{}
		for step := 0; ; step++ {
			v := r.field()
			at := [2]int{v.X, v.Y}
			visits[at]++
			if v.Scene != scene || step > 2500 || visits[at] > 40 {
				t.Fatal("original regional approach blocked", q.Event, v.Scene, at)
			}
			if fieldTestDistance(v.X, v.Y, q.X, q.Y) <= 128 {
				break
			}
			r.g = r.w.walk(t, r.m, &r.c, r.g, v, q.X, q.Y, 128)
			r.progress()
		}
		v := r.field()
		actorPresent := false
		for _, actor := range v.Field.Actors {
			if actor[0] == 1 && actor[1] == q.Event {
				actorPresent = true
				if actor[2] != q.Sprite {
					t.Fatal("regional original herb appearance changed", q.Event, actor[2])
				}
				if closedFrame >= 0 && actor[5] != closedFrame {
					t.Fatal("regional original unopened chest frame changed", q.Event, actor[5], closedFrame)
				}
			}
		}
		if actorPresent != (q.Sprite != 0) {
			t.Fatal("regional hidden marker or original herb appearance changed", q.Event, actorPresent)
		}
		type stance struct{ x, y, facing, distance int }
		stances := []stance{}
		for index, flags := range v.Field.Cells[:289] {
			if flags&1 != 0 || flags&2 != 0 && !combatAllowed {
				continue
			}
			u, z := index%17-8, index/17-8
			x, y := v.X+(u-z)*16, v.Y+(u+z)*8
			reserved := false
			for _, portal := range r.w.avoidTouches[v.Scene] {
				reserved = reserved || fieldTestDistance(x, y, portal[0], portal[1]) < portal[2]
			}
			if reserved {
				continue
			}
			for direction, delta := range [][2]int{{-16, 8}, {-16, -8}, {16, -8}, {16, 8}} {
				a, b := x, y
				points := [][2]int{{a, b}}
				for step := 0; step < 4; step++ {
					points = append(points, [2]int{a + delta[0], b + delta[1]}, [2]int{a, b + delta[1]*2}, [2]int{a + delta[0]*2, b})
					a, b = a+delta[0], b+delta[1]
				}
				for i, point := range points {
					if i >= int(original.TriggerMode)*6-4 || point[0]/32 != q.X/32 || point[1]/16 != q.Y/16 || (point[0]%32 != 0) != (q.X%32 != 0) {
						continue
					}
					facing := direction
					if i == 0 {
						facing = 4
					}
					stances = append(stances, stance{x, y, facing, fieldTestDistance(v.X, v.Y, x, y)})
					break
				}
			}
		}
		if len(stances) == 0 {
			t.Fatal("regional reward has no observed search stance", q.Event)
		}
		sort.SliceStable(stances, func(i, j int) bool { return stances[i].distance < stances[j].distance })
		selected := 0
		picked := false
		repeatPose := [3]int{}
		search := func() bool {
			current := r.field()
			if picked {
				if current.X != repeatPose[0] || current.Y != repeatPose[1] || current.Field.Direction != repeatPose[2] {
					t.Fatal("saved repeat search lost its exact original pose", q.Event, repeatPose, current.X, current.Y, current.Field.Direction)
				}
				r.keys(2, 4)
				return true
			}
			p := stances[selected]
			if current.X != p.x || current.Y != p.y {
				r.g = r.w.walk(t, r.m, &r.c, r.g, current, p.x, p.y, 0)
				return false
			}
			if p.facing < 4 && current.Field.Direction != p.facing {
				r.keys([]uint32{4, 8, 16, 32}[p.facing], p.facing)
				r.keys(0, 4)
				r.progress()
				r.field()
			}
			pose := r.field()
			repeatPose = [3]int{pose.X, pose.Y, pose.Field.Direction}
			if scholar {
				scholarSearches++
			}
			r.keys(2, 4)
			return true
		}
		pickupCashBefore := r.field().Cash
		pickupBattleStart := len(battleCashChanges)
		r.save(label + "-unpicked")
		standingVisits := map[[2]int]int{}
		for step := 0; step < 800 && !seen[q.Event]; step++ {
			current := r.field()
			if scholar && scholarDeclined && scholarSearches > 0 && scholarSearches <= 2 {
				name := fmt.Sprint(label, "-declined-field-", scholarSearches)
				if !r.proof.Saved[name] {
					r.save(name)
					current = r.field()
				}
			}
			at := [2]int{current.X, current.Y}
			standingVisits[at]++
			if current.Scene != scene {
				t.Fatal("regional search changed scene", q.Event, current.Scene)
			}
			if standingVisits[at] > 6 {
				selected++
				if selected >= len(stances) {
					t.Fatal("all original regional search stances blocked", q.Event, at)
				}
				standingVisits = map[[2]int]int{}
			}
			search()
			r.progress()
			r.field()
		}
		if !seen[q.Event] {
			t.Fatal("original regional loot notice missing", q.Event)
		}
		for _, grant := range q.Grants {
			if grant.Item > 0 {
				expected[grant.Name] += grant.Amount
			} else {
				expectedCash += uint32(grant.Cash)
			}
		}
		after := inventory()
		if !bytes.Equal(raw(expected), raw(after)) || r.field().Cash != expectedCash {
			t.Fatal("regional original inventory or cash quantity changed", q.Event, expected, after, expectedCash, r.field().Cash)
		}
		if disappears {
			for _, actor := range r.field().Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					t.Fatal("regional collected original herb remained visible", q.Event)
				}
			}
		}
		if openFrame >= 0 {
			opened := false
			for _, actor := range r.field().Field.Actors {
				opened = opened || actor[0] == 1 && actor[1] == q.Event && actor[2] == q.Sprite && actor[5] == openFrame
			}
			if !opened {
				t.Fatal("regional original chest opening frame missing", q.Event, openFrame)
			}
		}
		if scholar {
			wantFollower := 82
			if scholarOffer == "renegotiated" {
				wantFollower = 83
			}
			following := r.field().Field
			for _, actor := range following.Actors {
				if actor[0] == 2 {
					escortFollower = actor[2]
				}
			}
			if following.Followers != 1 || escortFollower != wantFollower {
				t.Fatal("original scholar follower did not join", scholarOffer, escortFollower, following.Followers)
			}
		}
		r.save(label + "-collected")
		picked = true
		repeatSearches := 0
		repeatLimit := 3
		if bladeMessage != "" {
			repeatLimit = 6
		}
		if vesselMessage != "" {
			repeatLimit = 4
		}
		for step := 0; repeatSearches < repeatLimit; step++ {
			if step > 800 {
				t.Fatal("regional repeated search did not reach its original stance", q.Event)
			}
			performed := search()
			if performed {
				repeatSearches++
			}
			r.progress()
			r.field()
			if performed && renewable {
				expected["鼠儿果"]++
				counts := inventory()
				if !bytes.Equal(raw(expected), raw(counts)) {
					t.Fatal("original fresh fruit search did not grant exactly one", repeatSearches, expected, counts)
				}
				renewableSearches = append(renewableSearches, map[string]any{"search": repeatSearches, "before": expected["鼠儿果"] - 1, "after": counts["鼠儿果"]})
				r.save(fmt.Sprint(label, "-renewable-search-", repeatSearches))
			}
			if performed && vesselMessage != "" {
				if repeatSearches == 1 {
					expected["炼蛊皿"]++
				}
				counts := inventory()
				if !bytes.Equal(raw(expected), raw(counts)) || !vesselSeen {
					t.Fatal("original silkworm vessel continuation changed", repeatSearches, expected, counts)
				}
				if repeatSearches == 1 {
					r.save(label + "-vessel-collected")
				}
			}
			if performed && bladeMessage != "" {
				if repeatSearches == 2 {
					expected["苗刀"]++
				}
				counts := inventory()
				if !bytes.Equal(raw(expected), raw(counts)) || repeatSearches >= 2 && !bladeSeen {
					t.Fatal("original palace chest continuation changed its staged rewards", repeatSearches, expected, counts, bladeSeen)
				}
				if repeatSearches <= 3 {
					r.save(fmt.Sprint(label, "-followup-", repeatSearches))
				}
			}
		}
		if bladeMessage != "" {
			after = inventory()
			if !bytes.Equal(raw(after), raw(expected)) {
				t.Fatal("hidden blade continuation inventory changed", expected, after)
			}
		}
		if renewable || vesselMessage != "" {
			after = inventory()
		}
		repeatInventory := inventory()
		referenceInventory := after
		if trialNavigation {
			referenceInventory = expected
			after = repeatInventory
		}
		if !bytes.Equal(raw(referenceInventory), raw(repeatInventory)) || r.field().Cash != expectedCash {
			t.Fatal("regional reward repeated after saved search", q.Event, "inventory", referenceInventory, repeatInventory, "cash", r.field().Cash, expectedCash)
		}
		r.save(label + "-repeated")
		if openFrame >= 0 {
			opened := false
			for _, actor := range r.field().Field.Actors {
				opened = opened || actor[0] == 1 && actor[1] == q.Event && actor[2] == q.Sprite && actor[5] == openFrame
			}
			if !opened {
				t.Fatal("regional original chest opening did not survive saved repeated searches", q.Event)
			}
		}
		if disappears {
			for _, actor := range r.field().Field.Actors {
				if actor[0] == 1 && actor[1] == q.Event {
					t.Fatal("original disappearing object reappeared after saved search", q.Event)
				}
			}
		}
		if persistentBody || merchantCorpse {
			present := false
			for _, actor := range r.field().Field.Actors {
				present = present || actor[0] == 1 && actor[1] == q.Event && actor[2] == q.Sprite
			}
			if !present {
				t.Fatal("original searched body disappeared", q.Event)
			}
		}
		pickupCashAfter := r.field().Cash
		pickupBattles := append([]map[string]any{}, battleCashChanges[pickupBattleStart:]...)
		pickupRewardCash := 0
		for _, grant := range q.Grants {
			pickupRewardCash += grant.Cash
		}
		for _, battle := range pickupBattles {
			pickupRewardCash += battle["rewardCash"].(int)
		}
		if int64(pickupCashAfter)-int64(pickupCashBefore) != int64(pickupRewardCash) {
			t.Fatal("original pickup wallet and intervening battles do not reconcile", q.Event, pickupCashBefore, pickupCashAfter, pickupRewardCash)
		}
		entry := map[string]any{"scene": scene, "event": q.Event, "position": [2]int{q.X, q.Y}, "grants": q.Grants, "cashBefore": pickupCashBefore, "cashAfter": pickupCashAfter, "battleCashDuringPickup": pickupBattles, "notification": true, "repeatSearches": repeatSearches, "repeatPose": repeatPose, "repeatedFromSavedPose": true, "invisible": q.Sprite == 0, "originalHerbDisappearance": q.Sprite == 68 || q.Sprite == 69 || q.Sprite == 70, "disappearsAfterPickup": disappears, "closedFrame": closedFrame, "openFrame": openFrame, "searchMode": original.TriggerMode}
		if renewable {
			entry["originalRenewableHarvest"] = true
			entry["inventoryBefore"] = renewableBefore
			entry["inventoryAfter"] = after["鼠儿果"]
			entry["freshSearches"] = renewableSearches
		}
		if banana {
			present := false
			for _, actor := range r.field().Field.Actors {
				present = present || actor[0] == 1 && actor[1] == q.Event && actor[2] == 519 && actor[5] == 1
			}
			if !present {
				t.Fatal("original picked banana tree frame did not survive saved searches")
			}
			entry["originalBananaTreeFrame"] = 1
		}
		if persistentBody {
			entry["persistentBodyAfterSavedSearches"] = true
		}
		if groundWeapon {
			entry["originalGroundWeaponDisappearance"] = true
		}
		if questBundle {
			entry["originalQuestBundleDisappearance"] = true
		}
		if merchantCorpse {
			entry["originalMerchantCorpsePreserved"] = true
		}
		if scholar {
			entry["originalEscortContract"] = true
			entry["escortFollowerSprite"] = escortFollower
			entry["decisions"] = scholarDecisions
			entry["searchesBeforeReward"] = scholarSearches
			entry["scholarOffer"] = scholarOffer
		}
		if q.Event == 1911 {
			opened := false
			for _, actor := range r.field().Field.Actors {
				opened = opened || actor[0] == 1 && actor[1] == 1910 && actor[2] == 485 && actor[5] == 2
			}
			if !opened {
				t.Fatal("original opened toad corpse frame not preserved")
			}
			entry["originalPoisonCorpseFrame"] = 2
		}
		if q.Event == 1818 {
			entry["originalLetterRequest"] = true
			entry["decisions"] = scholarDecisions
		}
		if groundVial {
			entry["originalGroundVialDisappearance"] = true
		}
		if bladeMessage != "" {
			entry["followup"] = map[string]any{"PC": 5149, "Item": 179, "Amount": 1, "Name": "苗刀", "notice": bladeMessage, "noticeSeen": bladeSeen, "afterRepeat": 2, "before": bladeBefore, "after": expected["苗刀"], "noDuplicateSearches": repeatSearches - 2}
		}
		if vesselMessage != "" {
			entry["followup"] = map[string]any{"PC": 28168, "Item": 268, "Amount": 1, "Name": "炼蛊皿", "notice": vesselMessage, "noticeSeen": vesselSeen, "afterRepeat": 1, "before": vesselBefore, "after": expected["炼蛊皿"], "noDuplicateSearches": repeatSearches - 1}
		}
		if originalEmpty {
			entry["grants"] = []any{}
			entry["originalZeroItemChest"] = true
			entry["zeroItemGrantPC"] = 37482
			entry["countedCandidate"] = false
			entry["originalNotice"] = q.Messages[0]
			entry["inventoryBefore"] = inventoryBeforeEmpty
			entry["inventoryAfter"] = after
			emptyChestAudits = append(emptyChestAudits, entry)
		} else {
			claimed = append(claimed, entry)
		}
		t.Log("regional original reward verified", scene, q.Event)
	}
	final := r.field()
	partyMatches := bytes.Equal(raw(final.Party), raw(initial.Party))
	if combatAllowed {
		partyMatches = len(final.Party) == len(initial.Party)
		if partyMatches {
			for i, member := range final.Party {
				before := initial.Party[i]
				partyMatches = partyMatches && member.Role == before.Role && member.Controller == before.Controller && member.Name == before.Name
			}
		}
	}
	if final.Scene != scene || final.Cash != expectedCash || !partyMatches || len(claimed) != len(spec.Events) || r.proof.Battles != r.proof.Wins || r.proof.ScriptedFinishes != 0 || len(battleCashChanges) != r.proof.Battles-preCollectionBattles || !combatAllowed && r.proof.Battles != 0 {
		t.Fatal("regional optional branch changed cash, party, original scene or expected reward count", final.Scene)
	}
	checksum := sha256.Sum256(source)
	wantedEmptyAudits := 0
	if scene == 93 {
		wantedEmptyAudits = 1
	}
	if len(emptyChestAudits) != wantedEmptyAudits {
		t.Fatal("original zero-item chest audit missing", len(emptyChestAudits))
	}
	version := 18
	if finalRegion {
		version = 19
	}
	if towerRemainder {
		version = 20
	}
	result := map[string]any{"passed": true, "verificationVersion": version, "scope": "explicit original pickups, quest items and escort dialogue branches, or separately identified rice-shop/zero-item chest audits; actual pickups check original search footprints, object appearance, inventory/cash, timed notice saves and repeat searches; no native edits", "candidate": campaignVariant(), "developmentProfile": developmentHealthProfile(), "authorityFixture": true, "auditOnly": len(auditedEvents) != 0, "auditedEvents": auditedEvents, "emptyChestAudits": emptyChestAudits, "scholarOffer": scholarOffer, "sourceCheckpoint": spec.Source, "sourceSHA256": hex.EncodeToString(checksum[:]), "scene": scene, "claimed": claimed, "inventory": expected, "cashInitial": initial.Cash, "cashFinal": final.Cash, "combatAllowed": combatAllowed, "preCollectionBattles": preCollectionBattles, "battleCashChanges": battleCashChanges, "partyInitial": initial.Party, "party": final.Party, "proof": r.proof}
	if scene == 181 || scene == 208 || scene == 210 || scene == 251 || scene == 253 || scene >= 263 && scene <= 265 || trialNavigation {
		result["inventoryInitial"] = inventoryInitial
		result["battleItemGrants"] = battleItemGrants
	}
	if scene == 208 {
		result["pendingOriginalEvents"] = []int{3537}
	}
	if scene == 227 {
		result["separatelyReviewedOriginalEvents"] = []int{4000}
	}
	if extendedRegion {
		result["wormAppearances"] = wormAppearances
		result["forceInputs"] = forceInputs
	}
	if trialNavigation {
		result["routePlanSHA256"] = trialPlanHash
		result["trialRoute"] = trialRouteTrace
		result["trialPortalSteps"] = trialPlan.PortalSteps
		result["trialEarthReturns"] = trialPlan.EarthReturns
	}
	if scene == 193 {
		result["forestPhase"] = forestPhase
	}
	for suffix, value := range map[string][]byte{"result": raw(result), "state": r.g.Data} {
		if err := os.WriteFile(campaignEvidencePath(prefix+"-"+suffix+".json", 1), value, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
