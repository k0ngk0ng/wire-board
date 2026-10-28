package contra

// Failure scenarios before implementation: the screen trigger fires early or
// misses crossed positions; repeat data advances incorrectly; slot priority or
// mandatory bullet replacement differs; initialized slots retain old fields;
// shared enemy properties differ from level-specific properties; coordinate
// wrapping or inactive slot data is silently normalized.

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func enemyFromOracle(b []uint8) Enemy {
	return Enemy{Sprite: b[0], Y: b[1], X: b[2], SpriteAttributes: b[3], Routine: b[4], FractionY: b[5], FractionX: b[6],
		VelocityY: int16(uint16(b[7])<<8 | uint16(b[8])), VelocityX: int16(uint16(b[9])<<8 | uint16(b[10])),
		Kind: b[11], AnimationDelay: b[12], VariableA: b[13], AttackDelay: b[14], Frame: b[15], HP: b[16], ScoreCollision: b[17], StateWidth: b[18], Attributes: b[19], Data: [4]uint8{b[20], b[21], b[22], b[23]}}
}
func enemyPoolFromOracle(rows [][]uint8) [16]Enemy {
	var result [16]Enemy
	for i, row := range rows {
		result[i] = enemyFromOracle(row)
	}
	return result
}
func TestOriginalEnemyScreenSpawning(t *testing.T) {
	f, err := os.Open("testdata/original-enemy-spawn.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Records  []struct {
			ID, Frame, Actor int
			Kind             string
			Before, After    struct {
				Enemies [][]uint8
				World   []uint8
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original {
		t.Fatal("not original execution")
	}
	counts := map[string]int{}
	for _, r := range oracle.Records {
		world := r.Before.World
		system := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies), ScreenReadOffset: world[20]}
		var err error
		switch r.Kind {
		case "initialize":
			err = system.Initialize(r.Actor, system.Slots[r.Actor].Kind, world[3])
		case "screenSpawn":
			err = system.SpawnScreen(EnemySpawnContext{Level: world[3], SceneType: world[7], Vertical: world[8] != 0, Screen: world[9], Scroll: world[10]})
		default:
			t.Fatal("unexpected fixture case", r.Kind)
		}
		if err != nil {
			t.Fatalf("case %d %s: %v", r.ID, r.Kind, err)
		}
		counts[r.Kind]++
		want := enemyPoolFromOracle(r.After.Enemies)
		if system.Slots != want || system.ScreenReadOffset != r.After.World[20] {
			for i, e := range system.Slots {
				if e != want[i] {
					t.Fatalf("case %d %s frame %d slot %d: got %+v want %+v before %+v", r.ID, r.Kind, r.Frame, i, e, want[i], enemyFromOracle(r.Before.Enemies[i]))
				}
			}
			t.Fatalf("case %d read offset got %d want %d", r.ID, system.ScreenReadOffset, r.After.World[20])
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"screenSpawn": 1200, "initialize": 28}) {
		t.Fatal("fixture coverage changed", counts)
	}
	t.Logf("original spawn coverage: %v", counts)
}
