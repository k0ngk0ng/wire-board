package contra

// Natural failure acceptance: standard two-human start, legal held pads and
// timer callbacks only. No stored-state edits, bonus lives, damage injection or
// injected outcome. The harness observes the continue-menu boundary to send
// Select/Start, but never acts as a bot or claims visual-strategy acceptance.
import (
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"testing"
)

func TestContraModuleStandardSharedFailure(t *testing.T) {
	m, c, g := initial(t, 2)
	start := saved(t, g)
	type recordedInput struct {
		AtMS  int64          `json:"atMS"`
		Actor int            `json:"actor"`
		Held  engine.Buttons `json:"held"`
	}
	var inputs []recordedInput
	sendHeld := func(held engine.Buttons) {
		inputs = append(inputs, recordedInput{c.Now.UnixMilli(), c.Actor, held})
		g = input(t, m, c, g, held)
	}
	if start.Session.World.Active.Combat.Players[0].Lives != 2 || start.Session.World.Active.Combat.Players[1].Lives != 2 {
		t.Fatal("nonstandard starting reserves")
	}
	menu := false
	for batch := 0; batch < 4000; batch++ {
		if batch%5 == 0 {
			for actor := 0; actor < 2; actor++ {
				c.Actor = actor
				sendHeld(engine.ButtonRight)
			}
		}
		g = ticks(t, m, &c, g, 50)
		s := saved(t, g)
		if g.Result != nil {
			t.Fatal("result before the original continue decision")
		}
		if s.Session.World.Active.Combat.World.LevelRoutine == 6 {
			for _, player := range s.Session.World.Active.Combat.Players {
				if player.Lives != 0 || player.GameOver == 0 {
					t.Fatal("continue offered before both human characters exhausted reserves")
				}
			}
			t.Logf("standard shared exhaustion reached continue menu at frame %d", s.Session.Random.Frames)
			menu = true
			break
		}
	}
	if !menu {
		t.Fatal("legal rightward run did not reach shared exhaustion")
	}
	for actor := 0; actor < 2; actor++ {
		c.Actor = actor
		sendHeld(0)
	}
	g = ticks(t, m, &c, g, 50)
	c.Actor = 0
	sendHeld(engine.ButtonSelect)
	g = ticks(t, m, &c, g, 50)
	sendHeld(0)
	g = ticks(t, m, &c, g, 50)
	sendHeld(engine.ButtonStart)
	g = ticks(t, m, &c, g, 50)
	for i := 0; i < 10 && g.Result == nil; i++ {
		g = ticks(t, m, &c, g, 50)
	}
	s := saved(t, g)
	if g.Result == nil || !g.Result.Cooperative || g.Result.Draw || len(g.Result.Winners) != 0 || !s.Session.Finalized || g.HasComputerPlayers || g.SystemDeadline != 0 {
		t.Fatal("natural shared failure was not finalized cooperatively")
	}
	if s.Session.World.Campaign.Outcome != engine.CampaignFailed {
		t.Fatal("wrong terminal outcome")
	}
	t.Logf("natural shared failure finalized: frame=%d revision=%d epoch=%s build=%s", s.Session.Random.Frames, s.Session.Revision, s.Epoch, s.RulesBuildID)
	replay, err := json.Marshal(map[string]any{"seed": start.Session.Random.Seed, "epoch": s.Epoch, "rulesBuildID": s.RulesBuildID, "timerStepMS": 50, "initialAtMS": start.ServerAt, "inputs": inputs, "finalFrame": s.Session.Random.Frames, "finalRevision": s.Session.Revision})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("REPLAY " + string(replay))
}
