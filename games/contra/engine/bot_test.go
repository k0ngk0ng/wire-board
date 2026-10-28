package contra

// Failure inventory before strategy implementation: hidden HP/timers/RNG or
// future map reads, invisible enemy leakage, world mutation during observation,
// opposing directions/accidental pause, coupled cooperative controllers,
// restored bot divergence, duplicate frames changing intent, and cancelling
// voluntary assistance resetting the session or retaining an old held pad.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBotPublicObservation(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewSession(42, 2, "bot-public")
	if e != nil {
		t.Fatal(e)
	}
	s.World.Active.Combat.World.LevelRoutine = 4
	s.World.Active.Combat.Players[0] = Player{X: 60, Lives: 2, DisplaySprite: 1, Motion: PlayerMotion{Position: 100 << 8}}
	s.World.Active.Enemies.Slots[0] = Enemy{Sprite: 30, X: 130, Y: 90, Kind: 1, HP: 20, AttackDelay: 45, VelocityX: 511}
	s.World.Active.Enemies.Slots[1] = Enemy{Sprite: 0, X: 20, Y: 20, Kind: 7, HP: 12}
	before, _ := json.Marshal(s)
	hud := BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
	frame, e := NewBotVisualFrame(s.World.Active.Presentation.Snapshot(), hud, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	observed, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("observation changed session")
	}
	hidden := s.World
	hidden.Active.Random.Value ^= 173
	hidden.Active.GenerationBias ^= 77
	hidden.Active.Enemies.Generator = SoldierGenerator{Routine: 3, Timer: 81, X: 240, Y: 199, Screen: 12, Generated: 77}
	for i := range hidden.Active.Enemies.Slots {
		p := &hidden.Active.Enemies.Slots[i]
		p.HP ^= 255
		p.AttackDelay ^= 41
		p.AnimationDelay ^= 19
		p.VariableA ^= 29
		p.Routine ^= 3
		p.Frame ^= 3
		p.Data = [4]uint8{199, 8, 44, 12}
		p.VelocityX = 777
		p.VelocityY = -555
	}
	hidden.Active.Enemies.Slots[1].X = 220
	hidden.Active.Enemies.Slots[1].Y = 170
	hidden.Active.Enemies.Slots[1].Kind = 19
	hidden.Active.Combat.Players[0].X = 199
	clear(hidden.Active.Combat.Terrain[:])
	hiddenFrame, e := NewBotVisualFrame(hidden.Active.Presentation.Snapshot(), hud, catalog, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	masked, e := ObserveBotFrame(hiddenFrame, vision)
	if e != nil || !reflect.DeepEqual(observed, masked) {
		t.Fatal("hidden enemy/random state leaked", e)
	}
	// No display boundary has run. AppliedMask is disabled and the submitted
	// OAM is hidden, so current rule objects must not become visual observations.
	if len(observed.Enemies) != 0 {
		t.Fatal("unsubmitted rule entity leaked through an entirely hidden frame", observed.Enemies)
	}
	if observed.Players[0].Visible || observed.Players[0].X != 0 || observed.Players[0].Y != 0 {
		t.Fatal("unsubmitted player position leaked through an entirely hidden frame")
	}
	for _, actor := range []int{-1, 2} {
		if _, e := frame.ForActor(actor); e == nil {
			t.Fatal("invalid seat accepted")
		}
	}
	if _, e := ObserveBotFrame(BotVisualFrame{}, vision); e == nil {
		t.Fatal("missing visual frame accepted")
	}
}
func TestBotIntentRestoreAndCancellation(t *testing.T) {
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	resources := levelResources(t, "stage-1.json")
	s, e := NewSession(42, 2, "bot-cancel")
	if e != nil {
		t.Fatal(e)
	}
	bots := [2]Bot{}
	acted := false
	for frame := uint64(1); frame <= 1100; frame++ {
		before, _ := json.Marshal(s)
		var pads [2]Buttons
		hud := BotPublicHUD{Seats: 2, Paused: BotPublicBool{Known: true, Value: s.World.Campaign.Paused}, Players: [2]BotPublicPlayer{{Occupied: true, Lives: BotPublicNumber{Known: true, Value: s.PresentedLives[0]}}, {Occupied: true, Lives: BotPublicNumber{Known: true, Value: s.PresentedLives[1]}}}}
		visual, e := NewBotVisualFrame(s.World.Active.Presentation.Snapshot(), hud, catalog, 0, frame)
		if e != nil {
			t.Fatal(e)
		}
		shared, e := ObserveBotFrame(visual, vision)
		if e != nil {
			t.Fatal(e)
		}
		for actor := range 2 {
			obs := shared
			obs.Actor = actor
			obs.Enemies = append([]BotEnemy(nil), shared.Enemies...)
			raw, _ := json.Marshal(bots[actor])
			var restored Bot
			if e = json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			pads[actor] = bots[actor].Decide(obs)
			acted = acted || pads[actor] != 0
			if pads[actor] != restored.Decide(obs) || !reflect.DeepEqual(bots[actor], restored) {
				t.Fatal("bot restore diverged")
			}
			duplicate := bots[actor]
			if duplicate.Decide(obs) != pads[actor] || !reflect.DeepEqual(duplicate, bots[actor]) {
				t.Fatal("duplicate bot observation advanced intent")
			}
			if pads[actor]&(ButtonLeft|ButtonRight) == ButtonLeft|ButtonRight || pads[actor]&(ButtonUp|ButtonDown) == ButtonUp|ButtonDown {
				t.Fatal("conflicting directions")
			}
			if s.World.Active.Combat.World.LevelRoutine == 4 && pads[actor]&ButtonStart != 0 {
				t.Fatal("bot paused the battle")
			}
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("strategy mutated authoritative game")
		}
		if _, e = s.StepPads(frame, pads, resources); e != nil {
			t.Fatal(frame, e)
		}
	}
	if !acted {
		t.Fatal("visual controller never produced an input before cancellation")
	}
	// The platform ceases calling Bot when voluntary assistance is cancelled.
	// The same Session processes a real user's zero pad; no bot-owned reset exists.
	raw, _ := json.Marshal(s)
	var manual Session
	if e = json.Unmarshal(raw, &manual); e != nil {
		t.Fatal(e)
	}
	result, e := s.StepPads(1101, [2]Buttons{}, resources)
	if e != nil {
		t.Fatal(e)
	}
	want, e := manual.StepPads(1101, [2]Buttons{}, resources)
	if e != nil || !reflect.DeepEqual(result, want) || !reflect.DeepEqual(*s, manual) {
		t.Fatal("cancel changed session", e)
	}
	if s.Controls.Held != ([2]Buttons{}) {
		t.Fatal("cancel retained bot input")
	}
}
