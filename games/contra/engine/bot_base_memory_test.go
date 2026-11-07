package contra

// Before base target persistence: a soldier crossing a visible wall core must
// not make the controller walk into its still-active electrical barrier.
// The remembered position must expire, never invent HP/destruction, and must
// not leak across a visibly changed stage or scene.
import "testing"

func botBaseObservation(tick uint64) BotObservation {
	o := BotObservation{Tick: tick, Actor: 0, Seats: 1, Phase: "battle", Stage: 1, StageKnown: true, Scene: 1, SceneKnown: true}
	o.Players[0] = BotPlayer{X: 126, Y: 167, Sprite: 80, Visible: true, Alive: true}
	return o
}

func TestBotVisibleBaseCorePersistsThroughBriefOcclusion(t *testing.T) {
	var b Bot
	o := botBaseObservation(1)
	o.Enemies = []BotEnemy{{X: 128, Y: 112, Kind: 20}}
	b.Decide(o)
	for tick := uint64(2); tick <= 60; tick++ {
		o = botBaseObservation(tick)
		o.Enemies = []BotEnemy{{X: 128, Y: 110, Kind: 21, Sprite: 147}}
		if pad := b.Decide(o); pad&ButtonUp != 0 {
			t.Fatal("occlusion became permission to enter the electrical barrier", tick, pad)
		}
	}
	advanced := false
	for tick := uint64(61); tick <= 181; tick++ {
		advanced = advanced || b.Decide(botBaseObservation(tick))&ButtonUp != 0
	}
	if !advanced {
		t.Fatal("an indefinitely missing core was treated as a permanent hidden object")
	}
}

func TestBotVisibleBaseCoreMemoryDoesNotCrossScene(t *testing.T) {
	var b Bot
	o := botBaseObservation(1)
	o.Enemies = []BotEnemy{{X: 128, Y: 112, Kind: 20}}
	b.Decide(o)
	o = botBaseObservation(2)
	o.Stage = 3
	if pad := b.Decide(o); pad&ButtonUp == 0 {
		t.Fatal("old core location carried into a visibly different stage", pad)
	}
}

// Back-wall targets at visibly different heights need different legal firing
// poses. Ducking must retain B, while moving into a low target's lane must not
// be blocked by entering the stationary crouch too early.
func TestBotVisibleLowBaseCoreUsesCrouchedFire(t *testing.T) {
	o := botBaseObservation(4)
	o.Players[0].X, o.Players[0].Y = 131, 168
	o.Enemies = []BotEnemy{{X: 128, Y: 112, Kind: 20}}
	var b Bot
	if pad := b.Decide(o); pad&(ButtonDown|ButtonB) != ButtonDown|ButtonB || pad&(ButtonLeft|ButtonRight) != 0 {
		t.Fatal("visible low core did not receive aligned crouched fire", pad)
	}
}

func TestBotVisibleStandingBaseCoreKeepsStandingFire(t *testing.T) {
	o := botBaseObservation(4)
	o.Players[0].Y = 168
	o.Enemies = []BotEnemy{{X: 128, Y: 104, Kind: 20}}
	var b Bot
	if pad := b.Decide(o); pad&ButtonB == 0 || pad&ButtonDown != 0 {
		t.Fatal("normal-height core got the wrong firing pose", pad)
	}
}

func TestBotVisibleLowBaseCoreAlignsBeforeCrouching(t *testing.T) {
	o := botBaseObservation(4)
	o.Players[0].X, o.Players[0].Y = 100, 168
	o.Enemies = []BotEnemy{{X: 128, Y: 112, Kind: 20}}
	var b Bot
	if pad := b.Decide(o); pad&ButtonRight == 0 || pad&ButtonDown != 0 {
		t.Fatal("crouch prevented reaching the visible target's firing lane", pad)
	}
}

func TestBotVisibleBaseCoreCommitmentSurvivesOtherTargets(t *testing.T) {
	var b Bot
	o := botBaseObservation(1)
	o.Players[0].X = 200
	o.Enemies = []BotEnemy{{X: 104, Y: 112, Kind: 20}, {X: 136, Y: 112, Kind: 20}, {X: 152, Y: 112, Kind: 20}}
	b.Decide(o)
	for tick := uint64(2); tick <= 60; tick++ {
		o = botBaseObservation(tick)
		o.Players[0].X = 201
		o.Enemies = []BotEnemy{{X: 104, Y: 112, Kind: 20}, {X: 136, Y: 112, Kind: 20}}
		if pad := b.Decide(o); pad&ButtonLeft != 0 {
			t.Fatal("briefly occluded selected core was replaced by a different firing lane", tick, pad)
		}
	}
	reselected := false
	for tick := uint64(61); tick <= 181; tick++ {
		o.Tick = tick
		pad := b.Decide(o)
		reselected = reselected || pad&ButtonLeft != 0
	}
	if !reselected {
		t.Fatal("permanently absent target prevented selecting remaining visible cores")
	}
}
