package contra

// Integration gate for newly recovered player modes: shared indoor progression
// can change the peer before its turn; ScreenCleared must reach player/weapon
// rules; vertical follower correction must retain Y fractions and high-byte
// carry; completed indoor transitions reset only after both actors run. These
// original scene starts include the explicitly recorded controlled setup.

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalCombatFrameModes(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/world-modes-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d frameOracle
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if len(d.PlayerFields) != 46 || len(d.WorldFields) != 24 || len(d.Scenarios) != 6 {
		t.Fatal("incomplete original modes frame oracle")
	}
	counts := map[[2]uint8]int{}
	for _, s := range d.Scenarios {
		for _, r := range s.Records {
			frame, input := combatFromOracle(t, d, r.Before)
			want, _ := combatFromOracle(t, d, r.After)
			counts[[2]uint8{frame.World.SceneType, frame.World.ScrollType}]++
			if _, err := frame.AdvancePlayersAndProjectiles(input); err != nil {
				t.Fatalf("%s frame %d: %v", s.Name, r.Frame, err)
			}
			if !reflect.DeepEqual(frame, want) {
				if frame.Players != want.Players {
					t.Fatalf("%s frame %d players\ngot %+v\nwant %+v", s.Name, r.Frame, frame.Players, want.Players)
				}
				if frame.World != want.World {
					t.Fatalf("%s frame %d world\ngot %+v\nwant %+v", s.Name, r.Frame, frame.World, want.World)
				}
				if frame.Weapons != want.Weapons {
					t.Fatalf("%s frame %d projectile pool differs", s.Name, r.Frame)
				}
				t.Fatalf("%s frame %d render/collision differs", s.Name, r.Frame)
			}
		}
	}
	wantCounts := map[[2]uint8]int{{1, 0}: 1815, {0, 1}: 1139, {128, 0}: 765}
	if !reflect.DeepEqual(counts, wantCounts) {
		t.Fatal("missing mode frame coverage", counts)
	}
	t.Logf("3719 original complete mode frame phases match: %v", counts)
}
