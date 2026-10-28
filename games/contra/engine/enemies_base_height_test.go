package contra

// Failure inventory before preserving the copied attribute: omit the previous
// actual hitbox height, use the creator's own later box, use a fixed value,
// normalize the oracle, or ignore the last player-vs-projectile box ordering.
import (
	"encoding/hex"
	"testing"
)

func TestOriginalBaseRollerInheritsCollisionHeight(t *testing.T) {
	d := loadEnemyTicks(t, "roller-height")
	if len(d.Records) != 5 {
		t.Fatal("real Base2 phase coverage")
	}
	fields := playerOracleData{PlayerFields: d.PlayerFields}
	resources := levelResources(t, "stage-4.json")
	tiles, err := resources.Graphics(3, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	animations, err := OriginalTileAnimations(3)
	if err != nil {
		t.Fatal(err)
	}
	created := 0
	for _, r := range d.Records {
		w := enemyTickWorld(d, r.Before.World)
		after := enemyTickWorld(d, r.After.World)
		players := [2]Player{playerFromOracle(fields, r.Before.Players[0]), playerFromOracle(fields, r.Before.Players[1])}
		inputs := [2]PlayerInput{}
		for i, row := range r.Before.Players {
			v := oraclePlayerValues(fields, row)
			inputs[i] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
		}
		weapons := originalWeaponPool(t, r.Before.Weapons)
		scores := arcadeScoresFromOracle(t, r.Before.Score)
		b, _ := hex.DecodeString(r.Before.Collision)
		terrain := CollisionWindow(b)
		b, _ = hex.DecodeString(r.Before.Supertiles)
		mapped := [128]uint8(b)
		q := GraphicsQueue{Used: w["queueUsed"]}
		cleared := w["screenCleared"]
		random := RandomState{Value: w["random"]}
		c := EnemyStepContext{Level: 3, SceneType: 1, Frame: w["frame"], FrameScroll: w["frameScroll"], Screen: w["screen"], ScreenScroll: w["screenScroll"], WeaponStrength: w["weaponStrength"], GameLoop: w["gameLoop"], AttackEnabled: w["attackEnabled"] != 0, SharedRandom: &random, SpawnVariantSeed: w["generationBias"], ScreenCleared: &cleared, Terrain: &terrain, Scroll: CollisionScroll{X: w["scrollX"], Y: w["scrollY"], NameTable: w["control"]}}
		change := func(req EnemyTerrainRequest, tile bool) (EnemyTerrainResponse, error) {
			gc := GraphicsContext{Scroll: c.Scroll, Supertiles: &mapped, Tileset: tiles}
			var v GraphicsRequestResult
			var e error
			if tile {
				v, e = q.RequestTileAnimation(req.X, req.Y, req.Code, gc, animations)
			} else {
				v, e = q.RequestSupertile(req.X, req.Y, req.Code, gc)
			}
			return EnemyTerrainResponse{Accepted: v.Accepted, AddressValid: v.Accepted, Address: v.Address, AttributeLookupWritten: v.AttributeLookupWritten, AttributeLookup: v.AttributeLookup}, e
		}
		c.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) { return change(req, false) }
		c.ChangeTiles = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) { return change(req, true) }
		before := enemyPoolFromOracle(r.Before.Enemies)
		s := EnemySystem{Slots: before, Base: BaseEnemyState{w["cores"], w["plating"], w["attackRounds"], w["redSoldier"], w["grenadeLauncher"]}}
		collisions := EnemyCollisionContext{Level: 3, SceneType: 1, Players: &players, Inputs: inputs, Weapons: &weapons, Scores: &scores, Flash: w["flash"]}
		if _, err = s.Tick(&c, &collisions); err != nil {
			t.Fatalf("frame%d: %v", r.Frame, err)
		}
		want := enemyPoolFromOracle(r.After.Enemies)
		for i, e := range want {
			if before[i].Routine == 0 && e.Kind == 17 && e.Routine != 0 {
				created++
				if w["inheritedThrowAttribute"] == e.Attributes {
					t.Fatal("fixture must distinguish frame-entry metadata from the intervening hitbox")
				}
			}
		}
		if s.Slots != want {
			for i, e := range s.Slots {
				if e != want[i] {
					t.Fatalf("frame%d slot%d got%+v want%+v", r.Frame, i, e, want[i])
				}
			}
		}
		if c.SpawnVariantSeed != after["generationBias"] {
			t.Fatal("unrelated generation effect changed")
		}
	}
	if created != 1 {
		t.Fatal("real created-roller count", created)
	}
}
