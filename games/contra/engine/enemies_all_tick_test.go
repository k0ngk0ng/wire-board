package contra

// The native whole-phase fixtures precede dispatch changes. They exercise each
// reachable initialized kind, live full-pool iteration, shared source counters,
// authentic graphics admission, player contact and generated lower slots. They
// are component integration evidence, not a completed eight-stage campaign.
import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

func TestOriginalAllStageEnemyPhaseDispatch(t *testing.T) {
	counts := [8]int{33, 54, 48, 63, 51, 39, 54, 30}
	for level := 0; level < 8; level++ {
		t.Run(fmt.Sprintf("stage%d", level+1), func(t *testing.T) {
			d := loadEnemyTicks(t, fmt.Sprintf("stage-%d-visual", level+1))
			if len(d.Records) != counts[level] {
				t.Fatal("original phase coverage", len(d.Records))
			}
			raw, err := os.ReadFile(fmt.Sprintf("leveldata/stage-%d.json", level+1))
			if err != nil {
				t.Fatal(err)
			}
			resources, err := DecodeLevelResources(raw)
			if err != nil {
				t.Fatal(err)
			}
			fields := playerOracleData{PlayerFields: d.PlayerFields}
			for _, r := range d.Records {
				label := d.Cases[r.ID-1].Name
				w, aw := enemyTickWorld(d, r.Before.World), enemyTickWorld(d, r.After.World)
				players := [2]Player{playerFromOracle(fields, r.Before.Players[0]), playerFromOracle(fields, r.Before.Players[1])}
				inputs := [2]PlayerInput{}
				for i, p := range r.Before.Players {
					v := oraclePlayerValues(fields, p)
					inputs[i] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
				}
				weapons := originalWeaponPool(t, r.Before.Weapons)
				scores := arcadeScoresFromOracle(t, r.Before.Score)
				decode := func(s string, n int) []uint8 {
					b, e := hex.DecodeString(s)
					if e != nil || len(b) != n {
						t.Fatalf("%s buffer%d: %v", label, n, e)
					}
					return b
				}
				mapped := [128]uint8(decode(r.Before.Supertiles, 128))
				terrain := CollisionWindow(decode(r.Before.Collision, 128))
				encoded := decode(r.Before.Graphics, 128)
				q := GraphicsQueue{Used: w["queueUsed"]}
				requested := 0
				variant := RegularMap
				if w["sceneType"]&0x80 != 0 {
					variant = BaseBossMap
				}
				tileset, err := resources.Graphics(level, variant)
				if err != nil {
					t.Fatal(err)
				}
				// The source scenario switches the enemy scene without loading the
				// Boss background; its original room attribute page remains active.
				if variant == BaseBossMap {
					regular, e := resources.Graphics(level, RegularMap)
					if e != nil {
						t.Fatal(e)
					}
					tileset.BaseAttributes = regular.BaseAttributes
					tileset.SourceAttributePage = regular.SourceAttributePage
				}
				var header PaletteState
				headerBytes := decode(r.Before.Header, 15)
				copy(header.CollisionLimits[:], headerBytes[:3])
				copy(header.CycleIndices[:], headerBytes[3:7])
				copy(header.PaletteIndices[:], headerBytes[7:])
				palette := PaletteOutput{FadeTimer: w["paletteTimer"], PauseCycle: w["palettePause"], FlashTimer: w["flash"], RequestedColors: w["paletteRequest"], Cycle: w["paletteCycle"]}
				copy(palette.Desired[:], decode(r.Before.Palette, 32))
				cleared, tank, bossScroll, autoScroll, pace := w["screenCleared"], w["tankScroll"], w["bossScrollComplete"], w["autoScrollB"], w["presentationPace"]
				address := uint16(w["locatorLow"]) | uint16(w["locatorHigh"])<<8
				c := EnemyStepContext{Level: uint8(level), SceneType: w["sceneType"], Vertical: w["vertical"] != 0, Frame: w["frame"], FrameScroll: w["frameScroll"], Random: w["random"], GameLoop: w["gameLoop"], WeaponStrength: w["weaponStrength"], AttackEnabled: w["attackEnabled"] != 0,
					Screen: w["screen"], ScreenScroll: w["screenScroll"], SpawnVariantSeed: w["generationBias"], ScreenCleared: &cleared, TankScroll: &tank, Terrain: &terrain, SolidBulletBackground: w["solidFlags"]&0x80 != 0, SolidPickupBackground: w["solidFlags"]&1 != 0,
					Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["control"]}, Scene: EnemySceneContext{BossScrollComplete: &bossScroll, Palette: &palette, Header: &header}, AlienBoss: AlienBossContext{AutoScroll: &autoScroll, PresentationPace: &pace, CollisionAddress: &address}}
				c.Scene.RefreshBackgroundPalette = func(count uint8) (PaletteResult, error) { return palette.Load(count, c.SceneType, header, resources) }
				random := RandomState{Value: w["random"]}
				c.SharedRandom = &random
				resets := 0
				c.OnBossControl = func(control BossControl) error {
					switch control.Kind {
					case BossAudioReset:
						resets++
					case BossBeginDefeat:
						w["bossDefeated"], w["delayLow"] = 1, 255
					case BossExitReady:
						w["delayLow"], w["delayHigh"] = control.Delay, 0
					}
					return nil
				}
				change := func(req EnemyTerrainRequest, tile bool) (EnemyTerrainResponse, error) {
					if requested >= len(r.Requests) {
						t.Fatalf("%s extra graphic %+v tile%v", label, req, tile)
					}
					want := r.Requests[requested]
					requested++
					if req.X != want.X || req.Y != want.Y || req.Code != want.Code || q.Used != want.Used || tile != want.Tile {
						t.Fatalf("%s graphic %+v tile%v used%d want%+v", label, req, tile, q.Used, want)
					}
					gc := GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tileset}
					var v GraphicsRequestResult
					var e error
					if tile {
						animations, x := OriginalTileAnimations(uint8(level))
						if x != nil {
							return EnemyTerrainResponse{}, x
						}
						v, e = q.RequestTileAnimation(req.X, req.Y, req.Code, gc, animations)
					} else {
						v, e = q.RequestSupertile(req.X, req.Y, req.Code, gc)
					}
					if v.Accepted {
						address = v.Address
					}
					return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
				}
				c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) { return change(req, false) }
				c.ChangeTiles = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) { return change(req, true) }
				collisions := EnemyCollisionContext{Players: &players, Inputs: inputs, Weapons: &weapons, Scores: &scores, Level: uint8(level), SceneType: c.SceneType, Demo: w["demo"] != 0, Flash: w["flash"]}
				s := EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies), ScreenReadOffset: w["screenReadOffset"], Base: BaseEnemyState{w["cores"], w["plating"], w["attackRounds"], w["redSoldier"], w["grenadeLauncher"]}, BossPartsDestroyed: w["bossParts"], AlienAimIndex: w["alienAimIndex"], JointScroll: w["jointScroll"]}
				result, err := s.Tick(&c, &collisions)
				if err != nil {
					t.Fatalf("%s: %v", label, err)
				}
				want := enemyPoolFromOracle(r.After.Enemies)
				if s.Slots != want {
					for i, e := range s.Slots {
						if e != want[i] {
							t.Fatalf("%s slot%d got%+v want%+v", label, i, e, want[i])
						}
					}
				}
				for i, p := range players {
					want := oraclePlayerValues(fields, r.After.Players[i])
					for name, v := range playerAsOracle(p, inputs[i]) {
						if v != want[name] {
							t.Fatalf("%s player%d %s %d/%d", label, i, name, v, want[name])
						}
					}
				}
				if weapons != originalWeaponPool(t, r.After.Weapons) || scores != arcadeScoresFromOracle(t, r.After.Score) {
					t.Fatalf("%s weapons/score differ", label)
				}
				if result.AudioReset != r.AudioReset || resets != r.AudioResets || !bytes.Equal(result.Sounds, r.Sounds) || len(s.PendingSounds) != 0 {
					t.Fatalf("%s audio result%+v want%v", label, result, r.Sounds)
				}
				if c.SpawnVariantSeed != aw["generationBias"] || s.GenerationBiasChanged {
					t.Fatalf("%s bias%02x/%02x writes%+v", label, c.SpawnVariantSeed, aw["generationBias"], r.GenerationBiasWrites)
				}
				at := int(w["queueUsed"])
				for _, p := range q.Packets {
					b, e := p.Encode()
					if e != nil {
						t.Fatal(e)
					}
					copy(encoded[at:], b)
					at += len(b)
				}
				if requested != len(r.Requests) || q.Used != aw["queueUsed"] || !bytes.Equal(encoded, decode(r.After.Graphics, 128)) || !bytes.Equal(mapped[:], decode(r.After.Supertiles, 128)) || !bytes.Equal(terrain[:], decode(r.After.Collision, 128)) {
					t.Fatalf("%s graphics/terrain differ", label)
				}
				w["screenCleared"], w["tankScroll"], w["bossScrollComplete"], w["autoScrollB"], w["presentationPace"] = cleared, tank, bossScroll, autoScroll, pace
				w["cores"], w["plating"], w["attackRounds"], w["redSoldier"], w["grenadeLauncher"] = s.Base.CoresRemaining, s.Base.PlatingDestroyed, s.Base.AttackRounds, s.Base.RedSoldierCreated, s.Base.GrenadeLauncher
				w["bossParts"], w["alienAimIndex"], w["jointScroll"] = s.BossPartsDestroyed, s.AlienAimIndex, s.JointScroll
				w["random"] = random.Value
				w["paletteTimer"], w["palettePause"], w["flash"] = palette.FadeTimer, palette.PauseCycle, result.Flash
				w["paletteRequest"], w["paletteCycle"] = palette.RequestedColors, palette.Cycle
				headerAfter := append(append(append([]uint8{}, header.CollisionLimits[:]...), header.CycleIndices[:]...), header.PaletteIndices[:]...)
				if !bytes.Equal(headerAfter, decode(r.After.Header, 15)) || !bytes.Equal(palette.Desired[:], decode(r.After.Palette, 32)) {
					t.Fatalf("%s palette/header differ", label)
				}
				for name, v := range w {
					if name != "queueUsed" && name != "generationBias" && name != "locatorLow" && name != "locatorHigh" && v != aw[name] {
						t.Fatalf("%s shared %s %d/%d", label, name, v, aw[name])
					}
				}
			}
		})
	}
}
