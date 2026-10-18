package basketball

// Failure inventory (before implementation): unauthenticated/automated seats
// change shared equipment; a second seat rewrites the first controller's menu;
// an old epoch/period/revision edits a shifted inventory index; edits consume
// time/RNG or input sequence numbers; restore loses accepted edits; Start changes
// the roster but leaves stale server options; preparation leaks into single-match
// mode; malformed extra fields smuggle ordinary inputs alongside menu edits.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Generate a halftime from the same source-backed password/roster boundary as
// engine acceptance. This is a controlled host-boundary test, not a menu run.
func campaignHostHalftime(t *testing.T, cooperative ...bool) (gameapi.Context, gameapi.Match) {
	t.Helper()
	var f struct {
		Config       engine.OriginalMatchConfiguration
		HumanTactics [2]uint8
	}
	raw, err := os.ReadFile("../engine/testdata/campaign-home-entry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	p, err := campaign.DecodePassword("020010")
	if err != nil {
		t.Fatal(err)
	}
	p, err = campaign.Equip(p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cooperative) > 0 && cooperative[0] {
		p.Mode = 1
	}
	r, err := engine.NewCampaignRuntime(p, engine.CampaignMatchSelection{Opponent: f.Config.Teams[1], Characters: f.Config.Characters, HumanTactics: f.HumanTactics, Seed: f.Config.Seed})
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 24000 && !(r.AwaitingNextPeriod && r.WaitVideoFrames == 0); n++ {
		step, e := engine.StepVideo(r, [4]engine.Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		r = step.State
	}
	if !r.AwaitingNextPeriod || r.WaitVideoFrames != 0 {
		t.Fatal("halftime not reached")
	}
	ctx := gameapi.Context{Players: 1, Actor: 0, Automated: []bool{false}, Now: time.UnixMilli(1000000)}
	s := State{Options: Options{Mode: "solo", Court: "court-0", Teams: r.Config.Match.Teams, Characters: r.Config.Match.Characters}, Epoch: "0123456789abcdef01234567", ServerAt: ctx.Now.UnixMilli(), Runtime: r, Events: []SoundEvent{}}
	if p.Mode == 1 {
		ctx.Players = 2
		ctx.Automated = []bool{false, false}
		s.Options.Mode = "cooperative"
	}
	for actor := range s.Inputs {
		s.Inputs[actor] = InputState{Automated: r.Config.Match.ControllerAssignments[actor] < 0, IdleAt: s.ServerAt + idleMS, LastPacketAt: s.ServerAt}
	}
	m, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	return ctx, m
}

func campaignEditJSON(t *testing.T, m gameapi.Match, a engine.CampaignPreparationAction) json.RawMessage {
	t.Helper()
	var s State
	if e := json.Unmarshal(m.Data, &s); e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(map[string]any{"type": "campaign-preparation", "epoch": s.Epoch, "preparation": a})
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func TestHostCampaignPreparationCommitRestore(t *testing.T) {
	ctx, m := campaignHostHalftime(t)
	views := []any{}
	record := func() {
		v, err := project(m, 0)
		if err != nil {
			t.Fatal(err)
		}
		views = append(views, v)
	}
	record()
	initial, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	actions := []engine.CampaignPreparationAction{
		{Type: "unequip", Character: 0, Slot: campaign.Shirt},
		{Type: "equip", Character: 1, InventoryIndex: 0},
		{Type: "teammates", Teammates: [2]uint8{1, 0}},
		{Type: "tactic", Tactic: 2},
	}
	for index, a := range actions {
		a.Period = 3
		a.Revision = uint64(index)
		raw := campaignEditJSON(t, m, a)
		if !acceptStale(ctx, m, raw) {
			t.Fatal("valid epoch/revision rejected on older room version", index)
		}
		before := string(m.Data)
		next, err := apply(ctx, m, raw)
		if err != nil {
			t.Fatal(index, err)
		}
		if string(m.Data) != before {
			t.Fatal("mutated prior match")
		}
		if err = next.Validate(1); err != nil {
			t.Fatal(err)
		}
		s, err := decode(next)
		if err != nil {
			t.Fatal(err)
		}
		if s.Runtime.VideoFrames != initial.Runtime.VideoFrames || s.Runtime.LogicSteps != initial.Runtime.LogicSteps || !reflect.DeepEqual(s.Runtime.Transition, initial.Runtime.Transition) || !reflect.DeepEqual(s.Runtime.Carry, initial.Runtime.Carry) || !reflect.DeepEqual(s.Runtime.Config, initial.Runtime.Config) || s.Inputs[0].Sequence != 0 {
			t.Fatal("menu edit advanced gameplay or consumed controller sequence")
		}
		if acceptStale(ctx, next, raw) {
			t.Fatal("stale inventory revision accepted")
		}
		if _, err = apply(ctx, next, raw); err == nil {
			t.Fatal("replayed edit accepted")
		}
		m, err = restore(ctx, next)
		if err != nil {
			t.Fatal(err)
		}
		v, err := project(m, -1)
		if err != nil {
			t.Fatal(err)
		}
		if v.(View).Preparation == nil || v.(View).Preparation.Revision != uint64(index+1) || v.(View).CanPrepare {
			t.Fatal("spectator lost preparation or gained edit permission")
		}
		own, err := project(m, 0)
		if err != nil || !own.(View).CanPrepare {
			t.Fatal("captain cannot edit", err)
		}
		record()
	}
	// Submit an actual controller Start and let the host timer commit the menu.
	raw, _ := json.Marshal(map[string]any{"type": "buttons", "epoch": initial.Epoch, "sequence": 1, "buttons": engine.ButtonStart})
	m, e = apply(ctx, m, raw)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 100; n++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		ctx.Actor = -1
		m, e = onTimer(ctx, m)
		if e != nil {
			t.Fatal(e)
		}
		record()
		s, err := decode(m)
		if err != nil {
			t.Fatal("roster commit invalidated saved options", err)
		}
		if s.Runtime.Playing != nil {
			if s.Options.Characters != s.Runtime.Config.Match.Characters || s.Options.Characters[0] != 1 || s.Runtime.Config.Campaign.Equipment[1][0] != 1 || s.Runtime.Config.HumanTactics[0] != 2 || s.Runtime.CampaignPreparation != nil {
				t.Fatal("confirmed preparation not committed")
			}
			if _, err = restore(ctx, m); err != nil {
				t.Fatal(err)
			}
			t.Logf("host preparation edits=4 frame=%d second-half=%d scores=%v", initial.Runtime.VideoFrames, s.Runtime.VideoFrames, s.scores())
			dir := filepath.Join("..", "..", "..", ".local", "campaign-host-preparation")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"scope": "controlled source-backed halftime; actual module edits, restore and timer; not campaign room entry", "views": views})
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(dir, "host-views.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("second half did not start")
}

func TestHostCampaignPreparationRejectsInvalidRequests(t *testing.T) {
	ctx, m := campaignHostHalftime(t)
	a := engine.CampaignPreparationAction{Type: "unequip", Period: 3, Revision: 0, Character: 0, Slot: campaign.Shirt}
	base := campaignEditJSON(t, m, a)
	for _, kind := range []string{"spectator", "automated", "epoch", "period", "revision", "missing", "sequence", "buttons", "seat", "basedOn", "events", "unknown", "extra-edit-field"} {
		t.Run(kind, func(t *testing.T) {
			c := ctx
			c.Automated = append([]bool{}, ctx.Automated...)
			var p map[string]any
			json.Unmarshal(base, &p)
			switch kind {
			case "spectator":
				c.Actor = -1
			case "automated":
				c.Automated[0] = true
			case "epoch":
				p["epoch"] = "old"
			case "period":
				p["preparation"].(map[string]any)["period"] = 1
			case "revision":
				p["preparation"].(map[string]any)["revision"] = 1
			case "missing":
				delete(p, "preparation")
			case "sequence":
				p["sequence"] = 1
			case "buttons":
				p["buttons"] = 1
			case "seat":
				p["seat"] = 0
			case "basedOn":
				p["basedOn"] = 0
			case "events":
				p["events"] = []any{}
			case "unknown":
				p["type"] = "equip"
			case "extra-edit-field":
				p["preparation"].(map[string]any)["item"] = 31
			}
			raw, _ := json.Marshal(p)
			if acceptStale(c, m, raw) {
				t.Fatal("invalid action accepted stale")
			}
			got, e := apply(c, m, raw)
			if e == nil || !reflect.DeepEqual(got, m) {
				t.Fatal("invalid action changed match", e)
			}
		})
	}
	var s State
	json.Unmarshal(m.Data, &s)
	s.Options.Mode = "versus"
	bad, _ := s.match()
	if _, e := decode(bad); e == nil {
		t.Fatal("campaign accepted competitive seat mapping")
	}
	coopCtx, coopMatch := campaignHostHalftime(t, true)
	coopCtx.Actor = 1
	if _, e := apply(coopCtx, coopMatch, campaignEditJSON(t, coopMatch, a)); e == nil {
		t.Fatal("second controller changed shared menu")
	}
	coopCtx.Actor = 0
	if _, e := apply(coopCtx, coopMatch, campaignEditJSON(t, coopMatch, a)); e != nil {
		t.Fatal("cooperative captain cannot edit", e)
	}
}
