package basketball

// Failure inventory before integration: confirmation settles before reward
// video; active video cannot decode/accept Start; timer freezes after result;
// restore re-awards inventory or loses presentation; finished reward adds a
// second Continue menu instead of entering preparation with final source RNG.
import (
	"compress/gzip"
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

func TestCampaignPresentationHostLifecycle(t *testing.T) {
	ctx, m := campaignHostHalftime(t)
	s, err := decode(m)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 48000 && !engine.RuntimePresentationActive(s.Runtime); n++ {
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
	if !engine.RuntimePresentationActive(s.Runtime) {
		t.Fatal("earned reward presentation not reached")
	}
	s.Options.Characters = s.Runtime.Config.Match.Characters
	s.Options.Campaign = &CampaignOptions{Password: "020010", Opponent: 1, Tactic: 4}
	s.Campaign = &CampaignState{Stage: "playing", Round: 1}
	if err = s.settleCampaign(); err != nil {
		t.Fatal(err)
	}
	if s.campaignMenu() {
		t.Fatal("reward settled before source video finished")
	}
	m, err = s.match()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = decode(m); err != nil {
		t.Fatal("active presentation decode", err)
	}
	before := s.Runtime.VideoFrames
	key := m.CheckpointKey
	saved := false
	for n := 0; n < 150 && !s.campaignMenu(); n++ {
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		m, err = onTimer(ctx, m)
		if err != nil {
			t.Fatal(n, err)
		}
		s, err = decode(m)
		if err != nil {
			t.Fatal(n, err)
		}
		if n == 4 {
			prior := s.Runtime
			m, err = restore(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			restored, e := decode(m)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(prior.Presentation, restored.Runtime.Presentation) {
				t.Fatal("restore changed presentation")
			}
			saved = true
		}
	}
	if !saved || s.Runtime.VideoFrames <= before || !engine.RuntimeFinished(s.Runtime) || s.Campaign.Stage != "prepare" || s.Campaign.Preparation == nil {
		t.Fatal("reward did not lead directly to preparation", s.phase())
	}
	if key == m.CheckpointKey || m.Result != nil {
		t.Fatal("campaign boundary not durable or incorrectly terminal")
	}
	expected, err := engine.SettleCampaignRuntime(s.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	if s.Campaign.Preparation.Progress != expected.Progress || s.Campaign.Preparation.Random != expected.Random {
		t.Fatal("handoff lost inventory or source RNG")
	}
	stable, _ := json.Marshal(s.Campaign)
	if err = s.settleCampaign(); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(s.Campaign)
	if string(stable) != string(after) {
		t.Fatal("settled reward twice")
	}
}

func endingConfirmationHost(t *testing.T) (gameapi.Context, gameapi.Match) {
	t.Helper()
	raw, e := os.Open("testdata/ending-needs-confirm.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	gz, e := gzip.NewReader(raw)
	if e != nil {
		t.Fatal(e)
	}
	defer gz.Close()
	var r engine.RuntimeState
	if e = json.NewDecoder(gz).Decode(&r); e != nil {
		t.Fatal(e)
	}
	password, e := campaign.EncodePassword(*r.Config.Campaign)
	if e != nil {
		t.Fatal(e)
	}
	s := State{Runtime: r, Epoch: "0123456789abcdef01234567", ServerAt: 1000000, Options: Options{Mode: "solo", Court: campaignCourtName(r.Config.Match.Court), Teams: r.Config.Match.Teams, Characters: r.Config.Match.Characters, Campaign: &CampaignOptions{Password: password, Opponent: r.Config.Match.Teams[1], Tactic: r.Config.HumanTactics[0]}}, Campaign: &CampaignState{Stage: "playing", Round: 14}}
	for i, c := range r.Config.Match.ControllerAssignments {
		s.Inputs[i] = InputState{Automated: c < 0, LastPacketAt: s.ServerAt}
	}
	ctx := gameapi.Context{Players: 1, Actor: 0, Now: time.UnixMilli(s.ServerAt), Automated: []bool{false}}
	m, e := s.match()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = decode(m); e != nil {
		t.Fatal(e)
	}
	return ctx, m
}

func TestCampaignEndingConfirmationDeadline(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		ctx, m := endingConfirmationHost(t)
		ctx.Automated[0] = automatic
		previousKey := m.CheckpointKey
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		var err error
		m, err = onTimer(ctx, m)
		if err != nil {
			t.Fatal(err)
		}
		s, err := decode(m)
		if err != nil {
			t.Fatal(err)
		}
		v, err := project(m, 0)
		if err != nil {
			t.Fatal(err)
		}
		deadline := v.(View).PresentationAutomaticAt
		if automatic {
			if m.CheckpointKey == previousKey {
				t.Fatal("confirmation deadline is not a durable boundary")
			}
			if deadline != ctx.Now.UnixMilli()+3000 {
				t.Fatal("ending must publish fresh 3 second confirmation deadline", deadline)
			}
			m, err = restore(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			restored, _ := project(m, 0)
			if restored.(View).PresentationAutomaticAt != deadline {
				t.Fatal("restore reset countdown")
			}
			ctx.Now = time.UnixMilli(deadline - 1)
			m, err = onTimer(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
			s, err = decode(m)
			if err != nil {
				t.Fatal(err)
			}
			if engine.RuntimeFinished(s.Runtime) {
				t.Fatal("automatic ending confirmed before visible deadline")
			}
		} else {
			if deadline != 0 {
				t.Fatal("human ending auto confirms")
			}
			raw, _ := json.Marshal(map[string]any{"type": "buttons", "epoch": s.Epoch, "sequence": 1, "buttons": engine.ButtonStart})
			m, err = apply(ctx, m, raw)
			if err != nil {
				t.Fatal("human Start rejected", err)
			}
		}
		for n := 0; n < 30 && m.Result == nil; n++ {
			ctx.Now = ctx.Now.Add(100 * time.Millisecond)
			m, err = onTimer(ctx, m)
			if err != nil {
				t.Fatal(err)
			}
		}
		if m.Result == nil {
			t.Fatal("ending did not complete", automatic)
		}
	}
}
