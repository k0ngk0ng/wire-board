package contra

// Before visual family context: logical compatibility numbers are reused by
// different public artwork. An outdoor tank/carrier must not imply an indoor
// Base simply because its number is 19/20; pixel aliases cannot choose a family.
import (
	"encoding/json"
	"testing"
)

func TestBotBackgroundKindDoesNotInventBaseScene(t *testing.T) {
	for _, known := range []bool{false, true} {
		o := BotObservation{Tick: 1, Actor: 0, Seats: 1, Phase: "battle", Stage: 4, StageKnown: known, SceneKnown: known}
		o.Players[0] = BotPlayer{X: 120, Y: 160, Visible: true, Alive: true, Sprite: 13}
		o.Enemies = []BotEnemy{{X: 180, Y: 100, Kind: 20}}
		var b Bot
		b.Decide(o)
		if b.Scene == 1 {
			t.Fatal("reused background kind invented a Base scene", known)
		}
	}
}

func TestBotRecognizedBaseFamilySuppliesVisibleScene(t *testing.T) {
	o := BotObservation{Tick: 1, Actor: 0, Seats: 1, Phase: "battle"}
	o.Players[0] = BotPlayer{X: 126, Y: 168, Visible: true, Alive: true, Sprite: 80}
	if err := json.Unmarshal([]byte(`[{"X":128,"Y":104,"Kind":20,"Family":"base-emplacement"}]`), &o.Enemies); err != nil {
		t.Fatal(err)
	}
	var b Bot
	b.Decide(o)
	if b.Scene != 1 || !b.SceneKnown {
		t.Fatal("a known visible Base face lost its scene meaning")
	}
}

func TestBotVisualFamilyAliasesRemainAmbiguous(t *testing.T) {
	a := botVisionTestTemplate("enemy", -1, 20, 0, true)
	b := a
	if err := json.Unmarshal([]byte(`{"Class":"enemy","Player":-1,"Kind":20,"Family":"base-emplacement"}`), &a.Label); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"Class":"enemy","Player":-1,"Kind":20,"Family":"snow-carrier"}`), &b.Label); err != nil {
		t.Fatal(err)
	}
	v, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	f := botVisionTestFrame()
	botVisionPlace(&f, a, 80, 90)
	o, err := ObserveBotFrame(f, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Enemies) != 0 {
		t.Fatal("identical artwork selected one of contradictory visible families", o.Enemies)
	}
}

func TestBotMachineFlameDoesNotBecomeJungleHatch(t *testing.T) {
	o := BotObservation{Tick: 1, Actor: 0, Seats: 1, Phase: "battle", Stage: 5, StageKnown: true, SceneKnown: true}
	o.Players[0] = BotPlayer{X: 128, Y: 196, Visible: true, Alive: true, Sprite: 13}
	o.Enemies = []BotEnemy{{X: 160, Y: 184, Kind: 17, Family: "machine-flame"}}
	var b Bot
	pad := b.Decide(o)
	if b.Stage != 5 || pad&ButtonUp != 0 {
		t.Fatal("reused flame kind invoked Jungle hatch behavior", b.Stage, pad)
	}
}
