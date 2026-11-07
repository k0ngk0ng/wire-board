package contra

// Before identity repair: wrong weak history rejects current complete bodies;
// one visible body is assigned to both seats; equal shared-head assignments
// depend on input order; ambiguous evidence renews a false track indefinitely.
import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestBotVisiblePrimaryRepairsSwappedHistory(t *testing.T) {
	f, e := os.Open("testdata/bot-coop-identity-observations.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var samples struct{ Observations []BotObservation }
	if e = json.NewDecoder(z).Decode(&samples); e != nil {
		t.Fatal(e)
	}
	var observed BotObservation
	found := false
	for _, o := range samples.Observations {
		if o.Players[0].Visible && o.Players[1].Visible {
			observed = o
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing two visible complete bodies")
	}
	for actor := 0; actor < 2; actor++ {
		o := observed
		o.Actor = actor
		b := Bot{LastTick: o.Tick - 1, Phase: "battle", Stage: 2, StageKnown: true, SceneKnown: true, LastViewX: o.ViewX, LastViewY: o.ViewY, GridKnown: o.GridKnown, JumpTargetActive: true, JumpTargetX: 84, JumpCooldown: 11}
		b.PlayerMemory = [2]BotPlayer{o.Players[1], o.Players[0]}
		b.PlayerSeenAt = [2]uint64{o.Tick - 1, o.Tick - 1}
		b.LastX, b.LastY = b.PlayerMemory[actor].X, b.PlayerMemory[actor].Y
		got := b.publicMemory(o)
		for i := 0; i < 2; i++ {
			if !got.Players[i].Visible || got.Players[i].X != o.Players[i].X || got.Players[i].Y != o.Players[i].Y {
				t.Fatal("old wrong identity overrode a complete body", actor, i, got.Players)
			}
		}
		if b.JumpTargetActive {
			t.Fatal("wrong identity's old navigation target survived correction")
		}
	}
}
func TestBotWeakIdentityCannotOccupyAnotherCompleteBody(t *testing.T) {
	o := BotObservation{Tick: 11, Actor: 1, Seats: 2, Phase: "battle"}
	o.Players[0] = BotPlayer{X: 100, Y: 100, Visible: true, Alive: true, Sprite: 13}
	o.PlayerCandidates = []BotPlayerCandidate{{Mask: 2, Player: BotPlayer{X: 100, Y: 100, Visible: true, Alive: true, Sprite: 17}}}
	b := Bot{LastTick: 10, Phase: "battle"}
	b.PlayerSeenAt[1] = 10
	b.PlayerMemory[1] = BotPlayer{X: 100, Y: 100, Visible: true, Alive: true, Sprite: 17}
	got := b.publicMemory(o)
	if got.Players[1].Visible || b.PlayerSeenAt[1] == 11 || got.Players[1].Alive {
		t.Fatal("another seat claimed the complete visible body", got.Players)
	}
}
func TestBotEqualIdentityAssignmentsRemainUnresolved(t *testing.T) {
	o := BotObservation{Tick: 11, Actor: 0, Seats: 2, Phase: "battle", PlayerCandidates: []BotPlayerCandidate{{Mask: 3, Exact: true, Player: BotPlayer{X: 60, Y: 100, Visible: true, Alive: true, Sprite: 25}}, {Mask: 3, Exact: true, Player: BotPlayer{X: 100, Y: 100, Visible: true, Alive: true, Sprite: 25}}}}
	b := Bot{LastTick: 10, Phase: "battle", PlayerSeenAt: [2]uint64{10, 10}, PlayerMemory: [2]BotPlayer{{X: 80, Y: 100, Alive: true}, {X: 80, Y: 100, Alive: true}}}
	other := b
	first := b.publicMemory(o)
	o.PlayerCandidates[0], o.PlayerCandidates[1] = o.PlayerCandidates[1], o.PlayerCandidates[0]
	second := other.publicMemory(o)
	if first.Players[0].Visible || first.Players[1].Visible || b.PlayerSeenAt != [2]uint64{10, 10} {
		t.Fatal("equivalent assignments forced seat identity", first.Players)
	}
	if !reflect.DeepEqual(first.Players, second.Players) || !reflect.DeepEqual(b, other) {
		t.Fatal("candidate order changed identity")
	}
}

// All assignments must be resolved before old-position fallback. Otherwise a
// lower-index seat can borrow the location just assigned to the higher seat.
func TestBotFallbackCannotOccupyNewCandidateTrack(t *testing.T) {
	for missing := 0; missing < 2; missing++ {
		o := BotObservation{Tick: 11, Actor: missing, Seats: 2, Phase: "battle"}
		other := 1 - missing
		o.PlayerCandidates = []BotPlayerCandidate{{Mask: 1 << uint(other), Exact: true, Player: BotPlayer{X: 100, Y: 100, Sprite: 25}}}
		b := Bot{PlayerSeenAt: [2]uint64{10, 10}, PlayerMemory: [2]BotPlayer{{X: 100, Y: 100, Alive: true}, {X: 100, Y: 100, Alive: true}}}
		got := b.publicMemory(o)
		if got.Players[missing].Alive || got.Players[missing].Visible || !got.Players[other].Visible {
			t.Fatal("fallback duplicated the newly assigned body", missing, got.Players)
		}
	}
}
