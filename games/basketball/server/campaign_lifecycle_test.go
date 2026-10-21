package basketball

// Failure inventory before implementation: draw a new reward table on password
// import/Continue/restore; turn an intermediate win into a final room result;
// award twice; lose equipped inventory; reuse prior scores or controller packets
// in a new match; allow editing a completed/active campaign; admit a defeated
// opponent; grant unowned loadout; confuse home court with opposing team; retain
// stale roster options; advance frozen result physics; leak second-seat or
// spectator authority; skip source roster draw; persist mutable result aliases.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestCampaignRoomEntry(t *testing.T) {
	for _, tc := range []struct {
		players int
		raw     string
		mode    uint8
		period  uint8
		court   uint8
	}{
		{1, `{"mode":"solo","campaign":{"opponent":1}}`, 0, 0, 1},
		{2, `{"mode":"cooperative","campaign":{"opponent":4}}`, 1, 0, 4},
		{1, `{"mode":"solo","campaign":{"password":"020010","opponent":4}}`, 0, 2, 0},
	} {
		raw, e := configure(tc.players, json.RawMessage(tc.raw))
		if e != nil {
			t.Fatal(e)
		}
		var o Options
		json.Unmarshal(raw, &o)
		r, e := newCampaignMatch(o, engine.RandomState(0x12345678))
		if e != nil {
			t.Fatal(e)
		}
		if r.Config.Match.Mode != tc.mode || r.Config.Match.Court != tc.court || r.Config.Campaign.Period != tc.period || r.Config.Match.Teams[1] != o.Campaign.Opponent {
			t.Fatal("entry differs", r.Config)
		}
		seed := engine.RandomState(0x12345678)
		if o.Campaign.Password == "" {
			initial, e := engine.NewOriginalCampaign(tc.mode, seed)
			if e != nil {
				t.Fatal(e)
			}
			seed = initial.Random
			if r.Config.Campaign.RewardSet != initial.Progress.RewardSet {
				t.Fatal("wrong initial reward draw")
			}
		}
		roster, e := engine.SelectOriginalCampaignRoster(tc.mode, [2]uint8{o.Characters[0], o.Characters[1]}, seed)
		if e != nil {
			t.Fatal(e)
		}
		if r.Config.Match.Characters != roster.Characters || r.Config.Match.Seed != roster.Random {
			t.Fatal("entry changed original roster/RNG")
		}
		ctx := gameapi.Context{Players: tc.players, Actor: 0, Automated: make([]bool, tc.players), Now: time.UnixMilli(1000000)}
		m, e := start(ctx, raw)
		if e != nil {
			t.Fatal(e)
		}
		s, e := decode(m)
		if e != nil {
			t.Fatal(e)
		}
		if s.Campaign == nil || s.Campaign.Stage != "playing" || m.Result != nil || s.Options.Characters != s.Runtime.Config.Match.Characters {
			t.Fatal("campaign not bound to room")
		}
		before := s.Runtime.Config
		m, e = restore(ctx, m)
		if e != nil {
			t.Fatal(e)
		}
		s, e = decode(m)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(s.Runtime.Config, before) {
			t.Fatal("restore re-imported password or drew RNG")
		}
	}
	for _, raw := range []string{
		`{"mode":"versus","campaign":{"opponent":1}}`,
		`{"mode":"solo","campaign":{"password":"030020","opponent":1}}`,
		`{"mode":"solo","campaign":{"password":"000001","opponent":1}}`,
		`{"mode":"solo","campaign":{"opponent":8}}`,
		`{"mode":"solo","campaign":{"opponent":1,"tactic":6}}`,
		`{"mode":"solo","characters":[0,0,0,1],"campaign":{"opponent":1}}`,
		`{"mode":"solo","campaign":{"opponent":1,"rewardSet":3}}`,
	} {
		if _, e := configure(1, json.RawMessage(raw)); e == nil {
			t.Fatal("invalid campaign options accepted", raw)
		}
	}
}

func TestCampaignPasswordDefaultOpponent(t *testing.T) {
	raw, err := configure(1, json.RawMessage(`{"mode":"solo","campaign":{"password":"020120"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var o Options
	json.Unmarshal(raw, &o)
	if o.Campaign.Opponent != 4 || o.Court != "court-4" {
		t.Fatal("password entry did not select first available original map opponent")
	}
}

func campaignFinishedHost(t *testing.T, edited bool) (gameapi.Context, gameapi.Match) {
	t.Helper()
	ctx, m := campaignHostHalftime(t)
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	if edited {
		for index, a := range []engine.CampaignPreparationAction{{Type: "unequip", Character: 0}, {Type: "equip", Character: 1}, {Type: "teammates", Teammates: [2]uint8{1, 0}}, {Type: "tactic", Tactic: 2}} {
			a.Period = 3
			a.Revision = uint64(index)
			step, e := engine.ApplyCampaignPreparation(s.Runtime, a)
			if e != nil {
				t.Fatal(e)
			}
			s.Runtime = step.State
		}
	}
	for n := 0; n < 48000 && !engine.RuntimeFinished(s.Runtime); n++ {
		var held [4]engine.Buttons
		if s.Runtime.AwaitingNextPeriod && s.Runtime.WaitVideoFrames == 0 {
			held[0] = engine.ButtonStart
		}
		step, e := engine.StepVideo(s.Runtime, held)
		if e != nil {
			t.Fatal(e)
		}
		s.Runtime = step.State
	}
	if !engine.RuntimeFinished(s.Runtime) {
		t.Fatal("normal match did not finish")
	}
	if (s.Runtime.Outcome.WinningTeam == 0) == edited {
		t.Fatal("controlled trajectory changed", s.Runtime.Outcome)
	}
	s.Options.Characters = s.Runtime.Config.Match.Characters
	s.Options.Campaign = &CampaignOptions{Password: "020010", Opponent: 1, Tactic: 4}
	s.Campaign = &CampaignState{Stage: "playing", Round: 1}
	if e = s.settleCampaign(); e != nil {
		t.Fatal(e)
	}
	m, e = s.match()
	if e != nil {
		t.Fatal(e)
	}
	return ctx, m
}

func campaignRequest(t *testing.T, m gameapi.Match, kind string, extra map[string]any) json.RawMessage {
	t.Helper()
	s, e := decode(m)
	if e != nil {
		t.Fatal(e)
	}
	a := map[string]any{"type": kind, "revision": s.Campaign.Revision}
	for k, v := range extra {
		a[k] = v
	}
	raw, e := json.Marshal(map[string]any{"type": "campaign", "epoch": s.Epoch, "campaign": a})
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

func TestCampaignResultContinueAndNextMatch(t *testing.T) {
	for _, edited := range []bool{false, true} {
		t.Run(map[bool]string{false: "win", true: "defeat"}[edited], func(t *testing.T) {
			ctx, m := campaignFinishedHost(t, edited)
			initial, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			want, e := engine.SettleCampaignRuntime(initial.Runtime)
			if e != nil {
				t.Fatal(e)
			}
			if m.Result != nil || !reflect.DeepEqual(initial.Campaign.Settlement, &want) {
				t.Fatal("intermediate result ended room or changed reward")
			}
			if e = m.Validate(1); e != nil {
				t.Fatal(e)
			}
			before, _ := json.Marshal(initial.Runtime)
			ctx.Now = ctx.Now.Add(100 * time.Millisecond)
			ctx.Actor = -1
			m, e = onTimer(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			m, e = restore(ctx, m)
			if e != nil {
				t.Fatal(e)
			}
			s, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			after, _ := json.Marshal(s.Runtime)
			if string(before) != string(after) || !reflect.DeepEqual(s.Campaign.Settlement, &want) {
				t.Fatal("result/restore advanced frozen match or re-awarded")
			}
			ctx.Actor = 0
			request := campaignRequest(t, m, "continue", nil)
			if s.Campaign.Stage != "prepare" {
				m, e = apply(ctx, m, request)
				if e != nil {
					t.Fatal(e)
				}
			}
			if _, e = apply(ctx, m, request); e == nil {
				t.Fatal("old result confirmation repeated")
			}
			s, e = decode(m)
			if e != nil {
				t.Fatal(e)
			}
			if s.Campaign.Stage != "prepare" || s.Campaign.Preparation.Progress != want.Progress || s.Campaign.Preparation.Random != want.Random {
				t.Fatal("Continue reset inventory/progress/random")
			}
			if edited && s.Campaign.Preparation.Progress.Equipment[1][0] != 1 {
				t.Fatal("defeat erased accepted equipment")
			}
			if !edited {
				if _, e = apply(ctx, m, campaignRequest(t, m, "opponent", map[string]any{"opponent": 1})); e == nil {
					t.Fatal("defeated opponent offered")
				}
				m, e = apply(ctx, m, campaignRequest(t, m, "equip", map[string]any{"character": 1, "inventoryIndex": 0}))
				if e != nil {
					t.Fatal(e)
				}
			}
			old, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			m, e = apply(ctx, m, campaignRequest(t, m, "start", nil))
			if e != nil {
				t.Fatal(e)
			}
			s, e = decode(m)
			if e != nil {
				t.Fatal(e)
			}
			p := old.Campaign.Preparation
			roster, e := engine.SelectOriginalCampaignRoster(p.Progress.Mode, p.Teammates, p.Random)
			if e != nil {
				t.Fatal(e)
			}
			if s.Epoch == old.Epoch || s.Runtime.VideoFrames != 0 || s.Campaign.Round != 2 || s.Campaign.Stage != "playing" || s.Campaign.Settlement != nil || s.Runtime.Config.Match.Seed != roster.Random || *s.Runtime.Config.Campaign != p.Progress || s.scores() != ([2]uint16{}) || s.Inputs[0].Sequence != 0 {
				t.Fatal("new round retained old state or redrew progress")
			}
			if _, e = apply(ctx, m, request); e == nil {
				t.Fatal("old epoch accepted")
			}
			if e = m.Validate(1); e != nil {
				t.Fatal(e)
			}
			for n := 0; n < 40 && s.Runtime.Playing == nil; n++ {
				ctx.Now = ctx.Now.Add(100 * time.Millisecond)
				ctx.Actor = -1
				m, e = onTimer(ctx, m)
				if e != nil {
					t.Fatal(e)
				}
				s, e = decode(m)
				if e != nil {
					t.Fatal(e)
				}
			}
			if s.Runtime.Playing == nil || s.Runtime.Playing.Match.Period != want.Progress.Period {
				t.Fatal("next original match not entered")
			}
			t.Logf("edited=%t prior=%v stage=%s nextOpponent=%d court=%d period=%d", edited, initial.scores(), want.NextStage, s.Runtime.Config.Match.Teams[1], s.Runtime.Config.Match.Court, s.Runtime.Playing.Match.Period)
		})
	}
}

func TestCampaignQuitAndUnauthorizedActions(t *testing.T) {
	ctx, m := campaignFinishedHost(t, true)
	for _, actor := range []int{-1, 1} {
		c := ctx
		c.Actor = actor
		if _, e := apply(c, m, campaignRequest(t, m, "continue", nil)); e == nil {
			t.Fatal("unauthorized continue", actor)
		}
	}
	auto := ctx
	auto.Automated = []bool{true}
	if _, e := apply(auto, m, campaignRequest(t, m, "quit", nil)); e == nil {
		t.Fatal("automated human endpoint accepted")
	}
	next, e := apply(ctx, m, campaignRequest(t, m, "quit", nil))
	if e != nil {
		t.Fatal(e)
	}
	if next.Result == nil || !next.Result.Cooperative || len(next.Result.Winners) != 0 {
		t.Fatal("quit did not finish unsuccessful campaign")
	}
	if e = next.Validate(1); e != nil {
		t.Fatal(e)
	}
	if _, e = apply(ctx, next, campaignRequest(t, next, "continue", nil)); e == nil {
		t.Fatal("quit resumed")
	}
}
