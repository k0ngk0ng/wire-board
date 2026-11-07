package contra

// Failure scenarios defined before implementation: wrong aim/movement order;
// ground/air conflicting-direction priority; motion on the first jump frame;
// resetting Y fractions; platform drop/ledge lock errors; landing with the wrong
// X inertia; omitted water animation physics; camera escaping a left-behind
// teammate; side/head probe offsets; wrong offscreen death; changing a peer or
// global state; injecting recorded after-state as an implementation input.
// Expected values come from real original-ROM normal-player calls, twice
// recorded with identical inputs. This is a reconstruction differential gate;
// full room/game E2E remains a separate requirement.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type playerOracleSnapshot struct {
	Player, Peer, World []uint8
	Map                 int
	ScreenCleared       uint8
}
type playerOracleCall struct {
	Name          string
	Before, After playerOracleSnapshot
}
type playerOracleStep struct {
	Sounds                       []uint8
	Kind                         string
	BeforePool, AfterPool        string
	ID, Frame, Actor, AimContext int
	Before, After                playerOracleSnapshot
	Calls                        []playerOracleCall
}
type playerOracleData struct {
	PlayerFields []struct{ Name, Address string } `json:"player_fields"`
	WorldFields  []struct{ Name, Address string } `json:"world_fields"`
	Scenarios    []struct {
		Name string
		Maps []struct {
			ID    int
			Bytes string
		}
		Steps []playerOracleStep
	}
}

func loadPlayerOracle(t *testing.T, names ...string) playerOracleData {
	t.Helper()
	name := "original-player-step.json.gz"
	if len(names) > 0 {
		name = names[0]
	}
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d playerOracleData
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if (len(d.PlayerFields) != 44 && len(d.PlayerFields) != 46) || len(d.Scenarios) == 0 {
		t.Fatal("incomplete original oracle")
	}
	return d
}

func oraclePlayerValues(d playerOracleData, row []uint8) map[string]uint8 {
	v := map[string]uint8{}
	for i, f := range d.PlayerFields {
		v[f.Name] = row[i]
	}
	return v
}

func playerFromOracle(d playerOracleData, row []uint8) Player {
	v := oraclePlayerValues(d, row)
	position := int32(v["yFraction"]) | int32(v["y"])<<8 | int32(v["yHigh"])<<16
	if position&0x800000 != 0 {
		position -= 0x1000000
	}
	return Player{
		Motion: PlayerMotion{Position: position, Velocity: int16(uint16(v["vyLo"]) | uint16(v["vyHi"])<<8), Jump: v["jump"],
			AnimationTimer: v["animTimer"], AnimationFrame: v["animFrame"], Aim: v["aim"]},
		State: v["state"], X: v["x"], FractionX: v["xFraction"], VelocityX: int8(v["vx"]), FractionVelocityX: v["vxFraction"],
		IndoorVelocityY: int16(uint16(v["indoorVyLo"]) | uint16(v["indoorVyHi"])<<8), Scrolling: v["scrolling"], Fall: v["fall"],
		Weapon: v["weapon"], MachineGunTimer: v["machineGunTimer"], SpawnInvincible: v["spawnInvincible"], Invincible: v["invincible"],
		Water: v["water"], Death: v["death"], OnEnemy: v["onEnemy"], FallLockY: v["fallLockY"], Sequence: v["sequence"],
		PreviousAim: v["previousAim"], Shock: v["shock"], ForceJump: v["forceJump"], WaterTimer: v["waterTimer"], Recoil: v["recoil"],
		IndoorAdvance: v["indoorAdvance"], SpecialSpriteTimer: v["specialSpriteTimer"], BoostX: int8(v["boostX"]),
		Sprite: v["sprite"], SpriteFlip: v["spriteFlip"], SkipLedge: v["skipLedge"], Lives: v["lives"], GameOver: v["gameOver"],
		DisplaySprite: v["displaySprite"], DisplayAttribute: v["displayAttribute"],
		IndoorAnchorX: v["indoorAnchorX"], IndoorAnchorY: v["indoorAnchorY"],
	}
}

func playerAsOracle(p Player, input PlayerInput) map[string]uint8 {
	return map[string]uint8{
		"state": p.State, "x": p.X, "xFraction": p.FractionX, "y": uint8(p.Motion.Position >> 8), "yFraction": uint8(p.Motion.Position), "yHigh": uint8(p.Motion.Position >> 16),
		"vx": uint8(p.VelocityX), "vxFraction": p.FractionVelocityX, "vyLo": uint8(p.Motion.Velocity), "vyHi": uint8(p.Motion.Velocity >> 8),
		"indoorVyLo": uint8(p.IndoorVelocityY), "indoorVyHi": uint8(p.IndoorVelocityY >> 8), "jump": p.Motion.Jump, "scrolling": p.Scrolling, "fall": p.Fall,
		"animTimer": p.Motion.AnimationTimer, "animFrame": p.Motion.AnimationFrame, "weapon": p.Weapon, "machineGunTimer": p.MachineGunTimer,
		"spawnInvincible": p.SpawnInvincible, "invincible": p.Invincible, "water": p.Water, "death": p.Death, "onEnemy": p.OnEnemy, "fallLockY": p.FallLockY,
		"sequence": p.Sequence, "previousAim": p.PreviousAim, "aim": p.Motion.Aim, "shock": p.Shock, "forceJump": p.ForceJump, "waterTimer": p.WaterTimer,
		"recoil": p.Recoil, "indoorAdvance": p.IndoorAdvance, "specialSpriteTimer": p.SpecialSpriteTimer, "boostX": uint8(p.BoostX),
		"sprite": p.Sprite, "spriteFlip": p.SpriteFlip, "skipLedge": p.SkipLedge, "held": uint8(input.Held), "pressed": uint8(input.Pressed), "lives": p.Lives, "gameOver": p.GameOver,
		"displaySprite": p.DisplaySprite, "displayAttribute": p.DisplayAttribute,
		"indoorAnchorX": p.IndoorAnchorX, "indoorAnchorY": p.IndoorAnchorY,
	}
}

func worldFromOracle(d playerOracleData, row []uint8) OutdoorWorld {
	v := map[string]uint8{}
	for i, f := range d.WorldFields {
		v[f.Name] = row[i]
	}
	return OutdoorWorld{GameRoutine: v["gameRoutine"], Frame: v["frameCounter"], PlayerMode: v["playerMode"], LevelRoutine: v["levelRoutine"],
		Level: v["level"], BossDefeated: v["bossDefeated"], SceneType: v["sceneType"], ScrollType: v["scrollType"], StopScroll: v["stopScroll"],
		Screen: v["screen"], ScreenScroll: v["screenScroll"], FrameScroll: v["frameScroll"], AutoScroll: [2]uint8{v["autoScrollA"], v["autoScrollB"]},
		BossScrollComplete: v["bossScrollComplete"], ActivePlayers: v["activePlayers"], IndoorScroll: v["indoorScroll"], Scroll: CollisionScroll{X: v["scrollX"], Y: v["scrollY"], NameTable: v["ppuControl"]}}
}

func TestOriginalOutdoorNormalStepsWithoutShooting(t *testing.T) {
	d := loadPlayerOracle(t)
	checked := 0
	for _, scenario := range d.Scenarios {
		windows := map[int]CollisionWindow{}
		for _, m := range scenario.Maps {
			raw, err := hex.DecodeString(m.Bytes)
			if err != nil || len(raw) != 128 {
				t.Fatal("invalid captured collision map")
			}
			var w CollisionWindow
			copy(w[:], raw)
			windows[m.ID] = w
		}
		for _, step := range scenario.Steps {
			before := oraclePlayerValues(d, step.Before.Player)
			// No held B means the real weapon handler follows its original idle
			// path; no imported projectile pool or recorded after-state is needed.
			if Buttons(before["held"])&ButtonB != 0 {
				continue
			}
			player := playerFromOracle(d, step.Before.Player)
			peer := playerFromOracle(d, step.Before.Peer)
			world := worldFromOracle(d, step.Before.World)
			window := windows[step.Before.Map]
			input := PlayerInput{Held: Buttons(before["held"]), Pressed: Buttons(before["pressed"])}
			weapons := WeaponSystem{}
			_, err := player.StepOutdoor(&PlayerStepContext{Actor: step.Actor, Peer: &peer, World: &world, Terrain: &window, Input: input, Fire: weapons.Fire})
			if err != nil {
				t.Fatalf("%s frame%d actor%d: %v", scenario.Name, step.Frame, step.Actor, err)
			}
			want := oraclePlayerValues(d, step.After.Player)
			for field, got := range playerAsOracle(player, input) {
				if got != want[field] {
					t.Fatalf("%s frame%d actor%d %s got %d want %d; before=%v", scenario.Name, step.Frame, step.Actor, field, got, want[field], before)
				}
			}
			if expected := playerFromOracle(d, step.After.Peer); !reflect.DeepEqual(peer, expected) {
				t.Fatalf("%s frame%d changed peer", scenario.Name, step.Frame)
			}
			if expected := worldFromOracle(d, step.After.World); !reflect.DeepEqual(world, expected) {
				t.Fatalf("%s frame%d world got %+v want %+v", scenario.Name, step.Frame, world, expected)
			}
			checked++
		}
	}
	if checked < 900 {
		t.Fatalf("insufficient no-shooting original coverage: %d", checked)
	}
	t.Logf("compared %d complete original normal-player calls, all 44 player fields plus peer/world", checked)
}

func playerOracleWindows(t *testing.T, maps []struct {
	ID    int
	Bytes string
}) map[int]CollisionWindow {
	t.Helper()
	out := map[int]CollisionWindow{}
	for _, m := range maps {
		raw, err := hex.DecodeString(m.Bytes)
		if err != nil || len(raw) != 128 {
			t.Fatal("invalid captured collision window")
		}
		out[m.ID] = CollisionWindow(raw)
	}
	return out
}
