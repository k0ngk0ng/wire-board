package engine

// Failure inventory before implementation: accept edits outside halftime,
// consume RNG/time while editing, lose accepted equipment on close/restore,
// create or duplicate unowned items, use a stale inventory index, bind equipment
// to a seat, change earned progress/opponent tactics, permit source-ineligible
// tactic selection, mutate prior snapshots, lose carry/scores at confirmation,
// replay old edits after commit, or misreport the source full-inventory discard.
import (
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

func TestCampaignPreparationOriginalEquipment(t *testing.T) {
	var rows []struct {
		Kind             string
		Character, Index uint8
		Before, After    campaign.Progress
	}
	readPropAttackFixture(t, "../campaign/testdata/equipment-calls.json.gz", &rows)
	if len(rows) != 884 {
		t.Fatal("missing original equipment calls")
	}
	discards := 0
	for i, row := range rows {
		p := CampaignPreparation{Progress: row.Before, Period: row.Before.Period + 1, Revision: 7, Teammates: [2]uint8{0, 1}, HumanTactics: [2]uint8{4, 4}}
		if row.Kind == "cancel" {
			if row.Before != row.After {
				t.Fatal("source cancellation changed accepted items")
			}
			continue
		}
		a := CampaignPreparationAction{Type: row.Kind, Period: p.Period, Revision: p.Revision, Character: row.Character, InventoryIndex: row.Index, Slot: campaign.Slot(row.Index)}
		got, discarded, e := editCampaignPreparation(p, a, false)
		if row.Kind == "equip" && row.Before.Inventory[row.Index] == campaign.Empty {
			if e == nil || got != p || discarded != campaign.Empty {
				t.Fatal("empty inventory accepted", i, e)
			}
			continue
		}
		if e != nil || got.Progress != row.After || got.Teammates != p.Teammates || got.HumanTactics != p.HumanTactics {
			t.Fatal("original equipment differs", i, e, got.Progress, row.After)
		}
		wantRevision := p.Revision
		if row.Before != row.After {
			wantRevision++
		}
		if got.Revision != wantRevision {
			t.Fatal("revision differs", i, got.Revision, wantRevision)
		}
		if discarded != campaign.Empty {
			discards++
			if row.Kind != "unequip" || discarded != row.Before.Equipment[row.Character][row.Index] {
				t.Fatal("wrong source discard", i, discarded)
			}
		}
	}
	if discards != 12 {
		t.Fatal("missing full-inventory discard cases", discards)
	}
	t.Logf("original equipment records=%d explicit full-inventory discards=%d", len(rows), discards)
}

func campaignPreparationHalftime(t *testing.T) RuntimeState {
	t.Helper()
	p, choice, _ := campaignHomeFixture(t)
	s, e := NewCampaignRuntime(p, choice)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 24000 && !(s.AwaitingNextPeriod && s.WaitVideoFrames == 0); i++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if !s.AwaitingNextPeriod || s.Carry == nil || s.Transition.Scene.Transition.Match.Period != 3 {
		t.Fatal("natural halftime not reached")
	}
	return s
}

func TestCampaignPreparationEditRestoreAndCompleteMatch(t *testing.T) {
	for repeat := 0; repeat < 2; repeat++ {
		s := campaignPreparationHalftime(t)
		initial := s
		digest := sha256.New()
		initialJSON, _ := json.Marshal(initial)
		digest.Write(initialJSON)
		view, e := ViewCampaignPreparation(s)
		if e != nil {
			t.Fatal(e)
		}
		if view.Revision != 0 || view.Period != 3 || !view.CanChooseTactic || view.Teammates != ([2]uint8{0, 1}) {
			t.Fatal("invalid preparation view", view)
		}
		actions := []CampaignPreparationAction{
			{Type: "unequip", Character: 0, Slot: campaign.Shirt},
			{Type: "equip", Character: 1, InventoryIndex: 0},
			{Type: "teammates", Teammates: [2]uint8{1, 0}},
			{Type: "tactic", Tactic: 2},
		}
		for i, a := range actions {
			view, e = ViewCampaignPreparation(s)
			if e != nil {
				t.Fatal(e)
			}
			a.Period, a.Revision = view.Period, view.Revision
			before, _ := json.Marshal(s)
			step, e := ApplyCampaignPreparation(s, a)
			if e != nil || step.DiscardedItem != campaign.Empty {
				t.Fatal(i, e, step.DiscardedItem)
			}
			unchanged, _ := json.Marshal(s)
			if string(before) != string(unchanged) {
				t.Fatal("editing mutated input snapshot")
			}
			if step.State.VideoFrames != s.VideoFrames || step.State.LogicSteps != s.LogicSteps || !reflect.DeepEqual(step.State.Transition, s.Transition) || !reflect.DeepEqual(step.State.Carry, s.Carry) || !reflect.DeepEqual(step.State.Config, s.Config) {
				t.Fatal("preparation edited live gameplay, committed config or time")
			}
			s = step.State
			encoded, _ := json.Marshal(step)
			digest.Write(encoded)
			if _, e = ApplyCampaignPreparation(s, a); e == nil {
				t.Fatal("stale revision accepted", i)
			}
			raw, _ := json.Marshal(s)
			var restored RuntimeState
			if e = json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			if e = ValidateRuntimeState(restored); e != nil {
				t.Fatal(e)
			}
			got, e := ViewCampaignPreparation(restored)
			if e != nil || got.Revision != uint64(i+1) {
				t.Fatal("accepted preparation lost on restore", e, got)
			}
			s = restored
		}
		view, e = ViewCampaignPreparation(s)
		if e != nil {
			t.Fatal(e)
		}
		if view.Progress.Equipment[0][0] != campaign.Empty || view.Progress.Equipment[1][0] != 1 || view.Progress.Inventory[0] != campaign.Empty || view.HumanTactics != ([2]uint8{2, 4}) {
			t.Fatal("owned transfer or team tactic differs", view)
		}
		for _, kind := range []string{"unowned", "progress", "opponent-tactic", "duplicate", "revision", "period"} {
			raw, _ := json.Marshal(s)
			var bad RuntimeState
			json.Unmarshal(raw, &bad)
			switch kind {
			case "unowned":
				bad.CampaignPreparation.Progress.Inventory[0] = 7
			case "progress":
				bad.CampaignPreparation.Progress.RewardSet = 1
			case "opponent-tactic":
				bad.CampaignPreparation.HumanTactics[1] = 5
			case "duplicate":
				bad.CampaignPreparation.Teammates = [2]uint8{0, 0}
			case "revision":
				bad.CampaignPreparation.Revision = 1 << 53
			case "period":
				bad.CampaignPreparation.Period = 5
			}
			if ValidateRuntimeState(bad) == nil {
				t.Fatal("invalid preparation restored", kind)
			}
		}
		// Accepted selections survive closing a UI without submitting an action,
		// including later platform takeover; only new tactic editing is disabled.
		auto, e := SetRuntimeAutomation(s, [4]bool{true, true, true, true})
		if e != nil {
			t.Fatal(e)
		}
		autoView, e := ViewCampaignPreparation(auto)
		if e != nil || autoView.CanChooseTactic || autoView.HumanTactics[0] != 2 {
			t.Fatal("takeover discarded selection or enabled computer-only tactics", e)
		}
		if _, e = ApplyCampaignPreparation(auto, CampaignPreparationAction{Type: "tactic", Tactic: 3, Period: autoView.Period, Revision: autoView.Revision}); e == nil {
			t.Fatal("all-computer team changed source menu tactic")
		}
		if s.CampaignPreparation.HumanTactics[0] != 2 {
			t.Fatal("takeover aliased original snapshot")
		}
		scores := initial.Transition.Scene.Transition.Scores
		for i := 0; i < 500 && s.Playing == nil; i++ {
			held := [4]Buttons{}
			if s.AwaitingNextPeriod && s.WaitVideoFrames == 0 {
				held[0] = ButtonStart
			}
			r, e := StepVideo(s, held)
			if e != nil {
				t.Fatal(e)
			}
			s = r.State
			if r.LogicAdvanced {
				encoded, _ := json.Marshal(r)
				digest.Write(encoded)
			}
		}
		if s.Playing == nil || s.CampaignPreparation != nil || s.Config.Match.Characters[0] != 1 || s.Config.Match.Characters[1] != 0 || s.Config.Match.ControllerAssignments != initial.Config.Match.ControllerAssignments || s.Playing.World.Preparation.Scores != scores {
			t.Fatal("half confirmation lost choices, seats or scores")
		}
		if s.Playing.Context.Actions.ShotEquipment[0] != 1 || s.Playing.Context.Actions.ShotEquipment[1] != 255 || s.Playing.Context.HumanTactics != ([2]uint8{2, 4}) || s.Config.Campaign.Equipment != view.Progress.Equipment || s.Config.Campaign.Inventory != view.Progress.Inventory {
			t.Fatal("new loadout not applied by character")
		}
		if _, e = ApplyCampaignPreparation(s, CampaignPreparationAction{Type: "unequip", Character: 1, Slot: campaign.Shirt, Period: 3, Revision: 4}); e == nil {
			t.Fatal("old preparation edited live play")
		}
		for i := 0; i < 48000 && (s.Outcome == nil || !s.Outcome.Confirmed); i++ {
			r, e := StepVideo(s, [4]Buttons{})
			if e != nil {
				t.Fatal(e)
			}
			s = r.State
			if r.LogicAdvanced {
				encoded, _ := json.Marshal(r)
				digest.Write(encoded)
			}
		}
		if s.Outcome == nil || !s.Outcome.Confirmed {
			raw, _ := json.Marshal(s)
			t.Logf("before=%s", raw)
			t.Fatal("edited campaign did not finish", s.VideoFrames)
		}
		settled, e := SettleCampaignRuntime(s)
		if e != nil || settled.Progress.Equipment != view.Progress.Equipment {
			t.Fatal("settlement lost accepted halftime equipment", e)
		}
		t.Logf("repeat=%d accepted_revision=%d edited_roles=%v final_video=%d scores=%v next_stage=%s preparation_and_second_half_sha256=%x", repeat, view.Revision, view.Teammates, s.VideoFrames, s.Outcome.Scores, settled.NextStage, digest.Sum(nil))
	}
}

func TestCampaignPreparationRejectsUnavailableAndStaleEdits(t *testing.T) {
	p, choice, _ := campaignHomeFixture(t)
	opening, e := NewCampaignRuntime(p, choice)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ViewCampaignPreparation(opening); e == nil {
		t.Fatal("preparation available before halftime")
	}
	s := campaignPreparationHalftime(t)
	view, e := ViewCampaignPreparation(s)
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range []CampaignPreparationAction{
		{Type: "equip", Character: 0, InventoryIndex: 0, Period: 3},
		{Type: "unequip", Character: 4, Slot: campaign.Shirt, Period: 3},
		{Type: "unequip", Character: 0, Slot: 3, Period: 3},
		{Type: "tactic", Tactic: 0, Period: 3},
		{Type: "tactic", Tactic: 6, Period: 3},
		{Type: "teammates", Teammates: [2]uint8{1, 1}, Period: 3},
		{Type: "teammates", Teammates: [2]uint8{0, 4}, Period: 3},
		{Type: "teammates", Teammates: [2]uint8{1, 0}, Period: 1},
		{Type: "teammates", Teammates: [2]uint8{1, 0}, Period: 3, Revision: 1},
		{Type: "unknown", Period: 3},
	} {
		r, e := ApplyCampaignPreparation(s, a)
		if e == nil || !reflect.DeepEqual(r.State, s) || r.DiscardedItem != campaign.Empty {
			t.Fatal("invalid edit not atomic", a, e)
		}
	}
	r, e := ApplyCampaignPreparation(s, CampaignPreparationAction{Type: "teammates", Teammates: view.Teammates, Period: view.Period, Revision: view.Revision})
	if e != nil || !reflect.DeepEqual(r.State, s) {
		t.Fatal("no-op edit changed snapshot", e)
	}
	// Both-human cooperative teams have no human+computer partner instruction.
	coop, e := SetRuntimeAutomation(s, [4]bool{false, false, true, true})
	if e != nil {
		t.Fatal(e)
	}
	v, e := ViewCampaignPreparation(coop)
	if e != nil || v.CanChooseTactic {
		t.Fatal("two-human team exposed partner tactic", e)
	}
}
