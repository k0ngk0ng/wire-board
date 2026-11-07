package contra

// Before visual memory: ordinal/ordering changes cannot become identity;
// ambiguous heads require public prior tracks; unknown HUD fields cannot become
// facts; input observations remain immutable; a public class disclosed earlier
// may survive its visibly ambiguous closed pose, but may never be invented.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func trackingObservation(tick uint64) BotObservation {
	o := BotObservation{Tick: tick, Actor: 0, Seats: 2, Phase: "battle", StageKnown: true, SceneKnown: true, GridKnown: true}
	o.Players = [2]BotPlayer{{X: 60, Y: 100, Visible: true, Alive: true, LivesKnown: true, Lives: 2, Sprite: 13}, {X: 100, Y: 100, Visible: true, Alive: true, LivesKnown: true, Lives: 2, Sprite: 13}}
	for y := range o.Terrain {
		for x := range o.Terrain[y] {
			o.Terrain[y][x] = TerrainPlatform
			o.TerrainKnown[y][x] = true
		}
	}
	return o
}

func TestBotVisibleTrackPrefersExactCandidateAtSameLocation(t *testing.T) {
	var b Bot
	b.Decide(trackingObservation(1))
	o := trackingObservation(2)
	o.Players[0] = BotPlayer{LivesKnown: true, Lives: 2}
	// JSON exercises the public candidate contract without depending on the
	// implementation field existing before this regression is introduced.
	if err := json.Unmarshal([]byte(`[{"Mask":1,"Exact":false,"Player":{"X":60,"Y":100,"Sprite":14,"Visible":true}},{"Mask":1,"Exact":true,"Player":{"X":60,"Y":100,"Sprite":25,"Visible":true}}]`), &o.PlayerCandidates); err != nil {
		t.Fatal(err)
	}
	b.Decide(o)
	if b.PlayerMemory[0].Sprite != 25 {
		t.Fatal("partial standing pose displaced exact head artwork on the same existing track", b.PlayerMemory[0])
	}
}
func TestBotTracksVisibleGeometryNotOrdinals(t *testing.T) {
	a, b := Bot{}, Bot{}
	for tick := uint64(1); tick <= 3; tick++ {
		one := trackingObservation(tick)
		one.Enemies = []BotEnemy{{ID: 0, Kind: 1, Sprite: 30, X: 140 - int(tick)*2, Y: 80}, {ID: 1, Kind: 1, Sprite: 30, X: 210 - int(tick)*2, Y: 90}}
		two := one
		two.Enemies = []BotEnemy{one.Enemies[1], one.Enemies[0]}
		two.Enemies[0].ID = 97
		two.Enemies[1].ID = 55
		original := append([]BotEnemy(nil), two.Enemies...)
		if a.Decide(one) != b.Decide(two) || !reflect.DeepEqual(a, b) {
			t.Fatal("input ordinal/order became hidden identity", tick)
		}
		if !reflect.DeepEqual(two.Enemies, original) {
			t.Fatal("strategy mutated a shared observation")
		}
	}
}
func TestBotAmbiguousHeadsUseOnlyVisibleHistory(t *testing.T) {
	b := Bot{}
	b.Decide(trackingObservation(1))
	o := trackingObservation(2)
	for i := range o.Players {
		o.Players[i] = BotPlayer{LivesKnown: true, Lives: 2}
	}
	o.PlayerCandidates = []BotPlayerCandidate{{Mask: 3, Player: BotPlayer{X: 101, Y: 100, Sprite: 25, Visible: true}}, {Mask: 3, Player: BotPlayer{X: 61, Y: 100, Sprite: 25, Visible: true}}}
	original := append([]BotPlayerCandidate(nil), o.PlayerCandidates...)
	b.Decide(o)
	if b.PlayerMemory[0].X != 61 || b.PlayerMemory[1].X != 101 || b.PlayerSeenAt != ([2]uint64{2, 2}) {
		t.Fatal("public tracks did not disambiguate heads", b.PlayerMemory)
	}
	if !reflect.DeepEqual(o.PlayerCandidates, original) {
		t.Fatal("strategy changed shared candidates")
	}
	fresh := Bot{}
	fresh.Decide(o)
	if fresh.PlayerSeenAt != ([2]uint64{}) {
		t.Fatal("unseen actor identities were invented")
	}
}
func TestBotRemembersOnlyDisclosedEmplacementClass(t *testing.T) {
	b := Bot{}
	o := trackingObservation(1)
	o.Scene = 1
	o.Enemies = []BotEnemy{{ID: 13, X: 128, Y: 104, Kind: 20}}
	b.Decide(o)
	o.Tick = 2
	o.Enemies[0].ID = 0
	o.Enemies[0].Kind = 254
	b.Decide(o)
	if b.TrackKind[0] != 20 {
		t.Fatal("visible core identity was forgotten")
	}
	fresh := Bot{}
	fresh.Decide(o)
	if fresh.TrackKind[0] != 0 {
		t.Fatal("shared closed pixels revealed undisclosed core identity")
	}
}

func TestBotCompleteBodyOverridesNearbyPartialHistory(t *testing.T) {
	b := Bot{}
	b.Decide(trackingObservation(1))
	o := trackingObservation(2)
	o.Players[0].X = 160
	o.PlayerCandidates = []BotPlayerCandidate{{Mask: 1, Player: BotPlayer{X: 61, Y: 100, Visible: true, Sprite: 13}}}
	b.Decide(o)
	if b.PlayerMemory[0].X != 160 {
		t.Fatal("partial history displaced a complete identified body", b.PlayerMemory[0])
	}
	o = trackingObservation(3)
	o.Phase = "intro"
	o.Players[0].X = 250
	b.Decide(o)
	if b.PlayerSeenAt != ([2]uint64{}) {
		t.Fatal("intro illustration became a gameplay actor")
	}
}
