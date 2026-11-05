package engine

import (
	"reflect"
	"testing"
)

func TestFighter7ActorPeerStopViews(t *testing.T) {
	s := ChuActorState{Fighter: ChuActorFighter{ObjectID: 0x8400, HitStopTicks: 17}, World: ChuActorWorld{OtherEnergy: map[uint16]int16{0x8800: 100, 0x8c00: 200, 0x9000: -1}, OtherHitStopTicks: map[uint16]uint8{0x8800: 3, 0x8c00: 7, 0x9000: 31}}}
	view := ChuActorView{}
	view.Fighters[1] = ChuAIFighter{Size: 0, Team: 2, Health: -1}
	view.Fighters[2] = ChuAIFighter{Size: 256, Team: 1, Health: 223}
	view.Fighters[3] = ChuAIFighter{Size: 256, Team: 3, Health: 0}
	a, err := s.actorAction(view)
	if err != nil || len(a.OtherFighters) != 3 {
		t.Fatal("peer view missing", err)
	}
	for id, stop := range s.World.OtherHitStopTicks {
		if a.OtherFighters[id].HitStopTicks != stop {
			t.Fatal("peer stop byte lost", id)
		}
	}
	if _, ok := a.OtherFighters[0x8400]; ok {
		t.Fatal("self stop duplicated into peer map")
	}
	f := a.OtherFighters[0x8800]
	f.HitStopTicks = 255
	a.OtherFighters[0x8800] = f
	s.acceptActorAction(a)
	if s.World.OtherHitStopTicks[0x8800] != 255 || s.World.OtherHitStopTicks[0x8c00] != 7 || s.Fighter.HitStopTicks != 17 {
		t.Fatal("peer stop merge retained old value or changed self")
	}
	if !reflect.DeepEqual(s.World.OtherEnergy, map[uint16]int16{0x8800: 100, 0x8c00: 200, 0x9000: -1}) {
		t.Fatal("peer stop merge changed energy")
	}
}
