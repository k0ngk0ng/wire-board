package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type observedSpecialShot struct {
	observedDunk
	ShotCycle, SpecialShot, Zone, Team, Character uint8
	SpecialShots                                  [4]uint8
	EnhancementEnabled                            bool
	ShotEquipment                                 [4]uint8
	Random                                        [4]uint8
}

func TestOriginalSpecialShotReleaseCalls(t *testing.T) {
	var rows []struct {
		Kind, Scenario string
		Frame          int
		Actor          uint8
		Before, After  observedSpecialShot
	}
	raw, err := os.ReadFile("testdata/special-shot.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, r := range rows {
		if r.Kind != "player" {
			continue
		}
		b, a := r.Before, r.After
		original := Carrier{Player: playerFromObservation(b.observedPlayer), ShotCycle: int8(b.ShotCycle)}
		shot := ShotReleaseContext{Team: b.Team, Character: b.Character, Zone: CourtZone(b.Zone), EnhancementEnabled: b.EnhancementEnabled, Equipment: b.ShotEquipment[b.Character], Random: RandomState(binary.BigEndian.Uint32(b.Random[:]))}
		got, err := StepCarrierAction(original, CarrierContext{HasBall: b.Holder == int32(r.Actor), Computer: b.Control&128 != 0, ShotRelease: shot}, KunioProfile(), NewYorkGroundCourt())
		want := Carrier{Player: playerFromObservation(a.observedPlayer), ShotCycle: int8(a.ShotCycle)}
		flag := b.SpecialShot
		if got.ChangeSpecialShot {
			flag = got.SpecialShot
		}
		random := shot.Random
		if got.ChangeRandom {
			random = got.Random
		}
		ball := uint8(b.Ball.Action)
		if got.Ball.Change {
			ball = uint8(got.Ball.Action)
		}
		if err != nil || got.Carrier != want || flag != a.SpecialShot || random != RandomState(binary.BigEndian.Uint32(a.Random[:])) || ball != uint8(a.Ball.Action) {
			t.Fatalf("%s frame %d actor %d cycle %d zone %x\nbefore %+v\ngot %+v\nwant %+v err=%v", r.Scenario, r.Frame, r.Actor, b.ShotCycle, b.Zone, b, got, a, err)
		}
		count++
	}
	if count != 148 {
		t.Fatal("missing shot player calls", count)
	}
	t.Logf("matched %d original shot release/player calls", count)
}
func TestOriginalShotReleaseCycleAndZone(t *testing.T) {
	for _, tc := range []struct {
		name   string
		action Action
		cycle  int8
		zone   CourtZone
		flag   uint8
		ball   BallAction
		cursor uint8
		change bool
	}{
		{"cold cycle4 outside", ActionShoot, 4, 0x60, 0, 4, 0, false},
		{"ground ordinary", ActionShoot, -1, 0x31, 0, 4, 0, false},
		{"ground high", ActionShoot, 2, 0x31, 128, 4, 0, false},
		{"ground role special", ActionShoot, 4, 0x31, 128, 21, 2, true},
		{"ground depth outside", ActionShoot, 3, 0x35, 0, 4, 0, false},
		{"air role special", ActionAirShoot, 2, 0x31, 0, 21, 3, true},
		{"air high", ActionAirShoot, 4, 0x31, 128, 4, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ChooseOriginalShotRelease(tc.action, tc.cycle, ShotReleaseContext{Team: 0, Character: 1, Zone: tc.zone, Equipment: 255, Random: 12345})
			if err != nil || got.SpecialShot != tc.flag || got.BallAction != tc.ball || got.ChangeCursor != tc.change || got.Cursor != tc.cursor || got.Random != 12345 {
				t.Fatal(got, err)
			}
		})
	}
}
