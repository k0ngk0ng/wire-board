package contra

// Failure inventory was fixed before implementation in the capture script:
// pose/variable boxes and byte edges; original SBC carry; second actor first;
// water Down, spawn protection, barrier damage; platform landing/inertia; HP
// sentinels; laser chain order; item/R preservation; score cap and extra lives.
// Every expected state below comes from actual original routine execution.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type enemyCollisionSnapshot struct {
	Players, Enemies [][]uint8
	Weapons, Score   string
	World            []uint8
}
type enemyCollisionOracle struct {
	Original     bool `json:"source_is_original_execution"`
	Scope        string
	PlayerFields []struct{ Name, Address string } `json:"player_fields"`
	Cases        []struct{ Name string }
	Records      []struct {
		ID, Frame, Slot, Case int
		Kind                  string
		Before, After         enemyCollisionSnapshot
		Sounds                []uint8
	}
}

func loadEnemyCollisionOracle(t *testing.T, scope string) enemyCollisionOracle {
	t.Helper()
	f, err := os.Open("testdata/original-enemy-collision-" + scope + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d enemyCollisionOracle
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if !d.Original || d.Scope != scope || len(d.PlayerFields) != 44 {
		t.Fatal("wrong collision oracle identity")
	}
	return d
}
func arcadeScoresFromOracle(t *testing.T, encoded string) ArcadeScores {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != 10 {
		t.Fatal("invalid arcade score bytes", err)
	}
	word := func(i int) uint16 { return uint16(b[i]) | uint16(b[i+1])<<8 }
	return ArcadeScores{HighScore: word(0), Hundreds: [2]uint16{word(2), word(4)}, NextLife: [2]uint16{word(6), word(8)}}
}
func TestOriginalEnemyCollisions(t *testing.T) {
	counts := map[string]int{}
	for _, scope := range []string{"natural", "natural-hurt", "controlled", "controlled-special"} {
		d := loadEnemyCollisionOracle(t, scope)
		fields := playerOracleData{PlayerFields: d.PlayerFields}
		for _, r := range d.Records {
			label := fmt.Sprintf("%s %s id%d frame%d case%d", scope, r.Kind, r.ID, r.Frame, r.Case)
			if r.Case > 0 {
				label += " " + d.Cases[r.Case-1].Name
			}
			players := [2]Player{playerFromOracle(fields, r.Before.Players[0]), playerFromOracle(fields, r.Before.Players[1])}
			inputs := [2]PlayerInput{}
			for i, row := range r.Before.Players {
				v := oraclePlayerValues(fields, row)
				inputs[i] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
			}
			weapons := originalWeaponPool(t, r.Before.Weapons)
			scores := arcadeScoresFromOracle(t, r.Before.Score)
			system := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies)}
			c := EnemyCollisionContext{Players: &players, Inputs: inputs, Weapons: &weapons, Scores: &scores, Demo: r.Before.World[0] != 0, Level: r.Before.World[1], SceneType: r.Before.World[2], Flash: r.Before.World[3]}
			var result EnemyCollisionResult
			var err error
			switch r.Kind {
			case "players":
				result, err = system.CollidePlayers(r.Slot, &c)
			case "bullets":
				result, err = system.CollideProjectiles(r.Slot, &c)
			default:
				t.Fatal("unknown collision kind", r.Kind)
			}
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			for i, player := range players {
				want := oraclePlayerValues(fields, r.After.Players[i])
				for key, value := range playerAsOracle(player, inputs[i]) {
					if want[key] != value {
						t.Fatalf("%s actor%d %s got%d want%d before%v", label, i, key, value, want[key], oraclePlayerValues(fields, r.Before.Players[i]))
					}
				}
			}
			wantEnemies := enemyPoolFromOracle(r.After.Enemies)
			if system.Slots != wantEnemies {
				for i, e := range system.Slots {
					if e != wantEnemies[i] {
						t.Fatalf("%s enemy%d got%+v want%+v before%+v", label, i, e, wantEnemies[i], enemyFromOracle(r.Before.Enemies[i]))
					}
				}
			}
			if want := originalWeaponPool(t, r.After.Weapons); weapons != want {
				for i, p := range weapons.Projectiles {
					if p != want.Projectiles[i] {
						t.Fatalf("%s projectile%d got%+v want%+v", label, i, p, want.Projectiles[i])
					}
				}
			}
			if want := arcadeScoresFromOracle(t, r.After.Score); scores != want {
				t.Fatalf("%s scores got%+v want%+v", label, scores, want)
			}
			if result.Flash != r.After.World[3] {
				t.Fatalf("%s flash got%d want%d", label, result.Flash, r.After.World[3])
			}
			if !reflect.DeepEqual(append([]uint8{}, result.Sounds...), append([]uint8{}, r.Sounds...)) {
				t.Fatalf("%s sounds got%v want%v", label, result.Sounds, r.Sounds)
			}
			counts[scope+"/"+r.Kind]++
		}
	}
	want := map[string]int{"natural/players": 814, "natural/bullets": 513, "natural-hurt/players": 2812, "natural-hurt/bullets": 2616, "controlled/players": 966, "controlled/bullets": 401, "controlled-special/bullets": 76}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("coverage changed: %v", counts)
	}
	t.Logf("original collision calls: %v", counts)
}

func TestArcadeScoreInitializationMatchesOriginal(t *testing.T) {
	d := loadEnemyCollisionOracle(t, "natural")
	if got, want := NewArcadeScores(), arcadeScoresFromOracle(t, d.Records[0].Before.Score); got != want {
		t.Fatalf("initial game score got%+v want%+v", got, want)
	}
}
