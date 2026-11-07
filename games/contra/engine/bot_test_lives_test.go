package contra

// User-authorized test assistance must replenish reserves only. It must not
// grant invulnerability, undo damage/death, reset weapons/physics, revive an
// eliminated actor, touch an unused seat, or count as standard campaign proof.
import (
	"reflect"
	"testing"
)

func TestBotAssistedReservesPreserveOtherState(t *testing.T) {
	s, err := NewSession(42, 2, "assisted-boundary")
	if err != nil {
		t.Fatal(err)
	}
	res := levelResources(t, "stage-1.json")
	if _, err = s.StepPads(1, [2]Buttons{}, res); err != nil {
		t.Fatal(err)
	}
	s.World.Active.Combat.Players[0].Lives = 1
	s.PresentedLives[0] = 1
	s.World.Active.Combat.Players[0].State = 2
	s.World.Active.Combat.Players[0].Weapon = 19
	before := *s
	events := replenishBotTestReserves(s, 2)
	if len(events) != 2 || s.World.Active.Combat.Players[0].Lives != 10 || s.World.Active.Combat.Players[1].Lives != 10 {
		t.Fatal("wrong reserve grant", events)
	}
	for i := 0; i < 2; i++ {
		s.World.Active.Combat.Players[i].Lives = before.World.Active.Combat.Players[i].Lives
	}
	if !reflect.DeepEqual(*s, before) {
		t.Fatal("assistance changed damage, controls, physics, weapon or display state")
	}
	s.World.Active.Combat.Players[0].GameOver = 1
	if got := replenishBotTestReserves(s, 1); len(got) != 0 {
		t.Fatal("eliminated actor revived", got)
	}
}

func TestBotAssistanceShowsNaturalLossBeforeRefill(t *testing.T) {
	s, err := NewSession(42, 1, "assisted-visible-loss")
	if err != nil {
		t.Fatal(err)
	}
	replenishBotTestReserves(s, 1)
	res := levelResources(t, "stage-1.json")
	for frame := uint64(1); frame <= 1500; frame++ {
		before := s.World.Active.Combat.Players[0].Lives
		if _, err = s.StepPads(frame, [2]Buttons{ButtonRight, 0}, res); err != nil {
			t.Fatal(err)
		}
		if s.World.Active.Combat.Players[0].Lives >= before {
			replenishBotTestReserves(s, 1)
			continue
		}
		lost := s.World.Active.Combat.Players[0].Lives
		if events := replenishBotTestReserves(s, 1); len(events) != 0 || s.World.Active.Combat.Players[0].Lives != lost {
			t.Fatal("unsubmitted loss was hidden by immediate refill")
		}
		if _, err = s.StepPads(frame+1, [2]Buttons{ButtonRight, 0}, res); err != nil {
			t.Fatal(err)
		}
		if s.PresentedLives[0] != lost {
			t.Fatal("reserve loss never reached public HUD")
		}
		if events := replenishBotTestReserves(s, 1); len(events) != 1 || s.PresentedLives[0] != lost {
			t.Fatal("refill rewrote the currently visible loss")
		}
		return
	}
	t.Fatal("natural death not reached")
}

func TestBotAssistedReservesStillAllowNaturalDeaths(t *testing.T) {
	s, err := NewSession(42, 1, "assisted-natural")
	if err != nil {
		t.Fatal(err)
	}
	events := replenishBotTestReserves(s, 1)
	if len(events) != 1 || s.PresentedLives != [2]uint8{10, 0} {
		t.Fatal("initial assistance differs")
	}
	res := levelResources(t, "stage-1.json")
	deaths := 0
	for frame := uint64(1); frame <= 1500; frame++ {
		before := s.World.Active.Combat.Players[0].Lives
		if _, err = s.StepPads(frame, [2]Buttons{ButtonRight, 0}, res); err != nil {
			t.Fatal(err)
		}
		if s.World.Active.Combat.Players[0].Lives < before {
			deaths++
		}
		replenishBotTestReserves(s, 1)
		if s.World.Active.Combat.Players[0].GameOver != 0 || s.World.Active.Combat.Players[1].Lives != 0 {
			t.Fatal("assisted seat or unused seat changed incorrectly")
		}
	}
	if deaths == 0 {
		t.Fatal("assistance prevented expected natural reserve consumption")
	}
	t.Logf("natural deaths preserved with reserve replenishment: %d", deaths)
}

type botTestLifeRefill struct {
	Frame  uint64 `json:"frame"`
	Actor  int    `json:"actor"`
	Before uint8  `json:"before"`
	After  uint8  `json:"after"`
}

// Test-only inventory grant. Ten reserves stay below the original byte/intro
// limits and above the four-icon HUD cap. Death, respawn and weapon loss still
// execute normally; nothing is patched in production rules or an eliminated seat.
func replenishBotTestReserves(s *Session, players int) []botTestLifeRefill {
	if s.World.Campaign.Outcome != CampaignOngoing {
		return nil
	}
	var events []botTestLifeRefill
	for actor := 0; actor < players; actor++ {
		p := &s.World.Active.Combat.Players[actor]
		if p.GameOver != 0 || p.Lives >= 10 {
			continue
		}
		// Let the real decrement reach the public picture before granting
		// another reserve, so visible respawn cues remain available to bots.
		if s.Random.Frames != 0 && s.PresentedLives[actor] != p.Lives {
			continue
		}
		events = append(events, botTestLifeRefill{s.Random.Frames, actor, p.Lives, 10})
		p.Lives = 10
		if s.Random.Frames == 0 {
			s.PresentedLives[actor] = 10
		}
	}
	return events
}
