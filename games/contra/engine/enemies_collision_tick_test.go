package contra

// Integration failures listed before Tick: stale player targets after hits;
// copied/ascending enemy slots; old OnEnemy/sounds retained; wrong post-AI
// collision gates; deferred Falcon and spawned-slot effects; sound reordering;
// fake terrain acceptance and discarded mixed graphics-buffer contents.

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type enemyTickSnapshot struct {
	Header, Palette                                 string
	Players, Enemies                                [][]uint8
	Weapons, Score, Graphics, Supertiles, Collision string
	World                                           []uint8
}
type enemyTickOracle struct {
	Original     bool `json:"source_is_original_execution"`
	Scope        string
	PlayerFields []struct{ Name, Address string } `json:"player_fields"`
	WorldFields  []struct{ Name, Address string } `json:"world_fields"`
	Cases        []struct{ Name string }
	Records      []struct {
		ID, Frame     int
		AudioReset    bool
		AudioResets   int
		Before, After enemyTickSnapshot
		Sounds        []uint8
		Slots         []struct{ Slot, Kind, Routine int }
		Requests      []struct {
			X, Y, Code, Used uint8
			Tile             bool
		}
		GenerationBiasWrites []struct {
			PC                string
			Bank, Slot, Value uint8
		}
	}
}

func loadEnemyTicks(t *testing.T, scope string) enemyTickOracle {
	t.Helper()
	f, err := os.Open("testdata/original-enemy-collision-tick-" + scope + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d enemyTickOracle
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if !d.Original || d.Scope != scope || len(d.PlayerFields) != 44 {
		t.Fatal("wrong whole-phase oracle")
	}
	return d
}
func enemyTickWorld(d enemyTickOracle, values []uint8) map[string]uint8 {
	out := map[string]uint8{}
	for i, f := range d.WorldFields {
		out[f.Name] = values[i]
	}
	return out
}
func TestOriginalWholeEnemyPhase(t *testing.T) {
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	mixed := 0
	requests := 0
	for _, scope := range []string{"machine-bias", "ordinary-bias", "traverse-bias", "controlled-bias", "boss-bias", "natural-boss", "falcon-sniper"} {
		baseScope := strings.TrimSuffix(scope, "-bias")
		d := loadEnemyTicks(t, scope)
		fields := playerOracleData{PlayerFields: d.PlayerFields}
		for _, r := range d.Records {
			label := fmt.Sprintf("%s frame%d record%d", scope, r.Frame, r.ID)
			if baseScope == "controlled" || baseScope == "boss" {
				label += " " + d.Cases[r.ID-1].Name
			}
			players := [2]Player{playerFromOracle(fields, r.Before.Players[0]), playerFromOracle(fields, r.Before.Players[1])}
			inputs := [2]PlayerInput{}
			for i, p := range r.Before.Players {
				v := oraclePlayerValues(fields, p)
				inputs[i] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
			}
			weapons := originalWeaponPool(t, r.Before.Weapons)
			scores := arcadeScoresFromOracle(t, r.Before.Score)
			world := enemyTickWorld(d, r.Before.World)
			afterWorld := enemyTickWorld(d, r.After.World)
			decode := func(encoded string, n int) []uint8 {
				b, err := hex.DecodeString(encoded)
				if err != nil || len(b) != n {
					t.Fatalf("%s invalid original buffer %d: %v", label, n, err)
				}
				return b
			}
			mapped := ([128]uint8)(decode(r.Before.Supertiles, 128))
			terrain := CollisionWindow(decode(r.Before.Collision, 128))
			queue := decode(r.Before.Graphics, 128)
			q := GraphicsQueue{Used: world["queueUsed"]}
			requestIndex := 0
			c := EnemyStepContext{Level: world["level"], SceneType: world["sceneType"], Vertical: world["vertical"] != 0, Frame: world["frame"], FrameScroll: world["frameScroll"], Random: world["random"], GameLoop: world["gameLoop"], WeaponStrength: world["weaponStrength"], AttackEnabled: world["attackEnabled"] != 0, SolidBulletBackground: world["solidFlags"]&0x80 != 0, SolidPickupBackground: world["solidFlags"]&1 != 0, Terrain: &terrain, Scroll: CollisionScroll{X: world["scrollX"], Y: world["scrollY"], NameTable: world["control"]}}
			sharedRandom := RandomState{Value: world["random"]}
			c.SharedRandom = &sharedRandom
			c.SpawnVariantSeed = world["generationBias"]
			// A stale convenience projection must not override the authoritative
			// shared work byte. Expected outcomes remain the real original trace.
			c.Random ^= 0xff
			defeatedCallbacks := 0
			c.OnBossDoorSignal = func(signal BossDoorSignal) error {
				switch signal {
				case BossDoorDefeated:
					world["bossDefeated"] = 1
					world["delayLow"] = 255
					defeatedCallbacks++
				case BossDoorTunnelFinished:
					world["delayLow"], world["delayHigh"] = 48, 0
				default:
					t.Fatalf("%s invalid Boss callback %d", label, signal)
				}
				return nil
			}
			c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
				if requestIndex >= len(r.Requests) {
					t.Fatalf("%s unexpected graphics request %+v", label, req)
				}
				want := r.Requests[requestIndex]
				requestIndex++
				requests++
				if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used {
					t.Fatalf("%s graphics request got%+v used%d want%+v", label, req, q.Used, want)
				}
				result, err := q.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset})
				return EnemyTerrainResponse{Accepted: result.Accepted, AddressValid: result.Accepted, Address: result.Address, AttributeLookupWritten: result.AttributeLookupWritten, AttributeLookup: result.AttributeLookup}, err
			}
			collisions := EnemyCollisionContext{Players: &players, Inputs: inputs, Weapons: &weapons, Scores: &scores, Level: world["level"], SceneType: world["sceneType"], Demo: world["demo"] != 0, Flash: world["flash"]}
			system := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies), ScreenReadOffset: world["screenReadOffset"]}
			result, err := system.Tick(&c, &collisions)
			if err != nil {
				t.Fatalf("%s: %v", label, err)
			}
			wantEnemies := enemyPoolFromOracle(r.After.Enemies)
			if system.Slots != wantEnemies {
				for i, e := range system.Slots {
					if e != wantEnemies[i] {
						t.Fatalf("%s slot%d got%+v want%+v before%+v", label, i, e, wantEnemies[i], enemyFromOracle(r.Before.Enemies[i]))
					}
				}
			}
			for i, p := range players {
				if c.Players[i] != (EnemyTarget{State: p.State, X: p.X, Y: p.y(), GameOver: p.GameOver}) {
					t.Fatalf("%s final target projection is stale for actor%d", label, i)
				}
				want := oraclePlayerValues(fields, r.After.Players[i])
				for name, value := range playerAsOracle(p, inputs[i]) {
					if value != want[name] {
						t.Fatalf("%s actor%d field%s got%d want%d", label, i, name, value, want[name])
					}
				}
			}
			if want := originalWeaponPool(t, r.After.Weapons); weapons != want {
				t.Fatalf("%s player projectiles differ", label)
			}
			if want := arcadeScoresFromOracle(t, r.After.Score); scores != want {
				t.Fatalf("%s score got%+v want%+v", label, scores, want)
			}
			if result.Flash != afterWorld["flash"] || collisions.Flash != afterWorld["flash"] || system.ScreenReadOffset != afterWorld["screenReadOffset"] {
				t.Fatalf("%s shared phase fields differ", label)
			}
			if sharedRandom.Value != afterWorld["random"] {
				t.Fatalf("%s shared random changed unexpectedly", label)
			}
			if c.SpawnVariantSeed != afterWorld["generationBias"] {
				t.Fatalf("%s generation bias got%02x want%02x before%02x writes%+v", label, c.SpawnVariantSeed, afterWorld["generationBias"], world["generationBias"], r.GenerationBiasWrites)
			}
			if system.GenerationBiasChanged {
				t.Fatalf("%s left an unconsumed generation bias notification", label)
			}
			if !slices.Equal(result.Sounds, r.Sounds) || len(system.PendingSounds) != 0 {
				t.Fatalf("%s sounds got%v pending%v want%v", label, result.Sounds, system.PendingSounds, r.Sounds)
			}
			if result.AudioReset != r.AudioReset || defeatedCallbacks != r.AudioResets {
				t.Fatalf("%s Boss reset/callback differs reset%t callbacks%d expected%t/%d", label, result.AudioReset, defeatedCallbacks, r.AudioReset, r.AudioResets)
			}
			if len(r.Sounds) > 1 && !slices.Equal(r.Sounds, []uint8{r.Sounds[0], r.Sounds[0]}) {
				mixed++
			}
			if requestIndex != len(r.Requests) {
				t.Fatalf("%s graphics calls got%d want%d", label, requestIndex, len(r.Requests))
			}
			at := int(world["queueUsed"])
			for _, packet := range q.Packets {
				encoded, err := packet.Encode()
				if err != nil {
					t.Fatal(err)
				}
				copy(queue[at:], encoded)
				at += len(encoded)
			}
			if q.Used != afterWorld["queueUsed"] || !bytes.Equal(queue, decode(r.After.Graphics, 128)) {
				t.Fatalf("%s graphics queue differs got used%d want%d", label, q.Used, afterWorld["queueUsed"])
			}
			if !bytes.Equal(mapped[:], decode(r.After.Supertiles, 128)) || !bytes.Equal(terrain[:], decode(r.After.Collision, 128)) {
				t.Fatalf("%s mapped terrain differs", label)
			}
			for name, value := range world {
				if name != "queueUsed" && name != "flash" && name != "generationBias" && value != afterWorld[name] {
					t.Fatalf("%s unmodeled shared mutation %s: %d -> %d", label, name, value, afterWorld[name])
				}
			}
			counts[baseScope]++
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"machine": 700, "ordinary": 1200, "traverse": 3480, "controlled": 11, "boss": 5, "natural-boss": 42, "falcon-sniper": 3}) || mixed < 2 {
		t.Fatal("whole-phase coverage changed", counts, "mixed sound frames", mixed)
	}
	t.Logf("matched complete original enemy phases %v, %d graphics requests", counts, requests)
}

func TestEnemyTickClearsEphemeralStateAndRejectsUnknownLevel(t *testing.T) {
	var players [2]Player
	players[0].OnEnemy = 1
	players[1].OnEnemy = 2
	weapons := WeaponSystem{}
	scores := NewArcadeScores()
	terrain := CollisionWindow{}
	c := EnemyStepContext{Terrain: &terrain, SpawnVariantSeed: 0xa7}
	collisions := EnemyCollisionContext{Players: &players, Weapons: &weapons, Scores: &scores}
	s := EnemySystem{PendingSounds: []uint8{0x2f}, GenerationBiasChanged: true, GenerationBiasValue: 0x11}
	got, err := s.Tick(&c, &collisions)
	if err != nil || len(got.Sounds) != 0 || len(s.PendingSounds) != 0 || players[0].OnEnemy != 0 || players[1].OnEnemy != 0 {
		t.Fatal("stale enemy phase state", got, err, s.PendingSounds, players)
	}
	if c.SpawnVariantSeed != 0xa7 || s.GenerationBiasChanged {
		t.Fatal("empty phase overwrote incoming generation bias")
	}
	c.Level = 1
	collisions.Level = 1
	if _, err = s.Tick(&c, &collisions); err == nil {
		t.Fatal("unrecovered level silently accepted")
	}
}
