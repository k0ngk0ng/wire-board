package contra

// Before correcting the final projection: native 031A/0334 are both player
// coordinates and the logical display coordinates. Platform contact occurs
// after D296 and may change Y; the later compositor must read that same value.
// Expected coordinates below are actual original collision return values.
import "testing"

func TestOriginalPlatformContactPublishesFinalActorCoordinates(t *testing.T) {
	d := loadEnemyCollisionOracle(t, "controlled")
	fields := playerOracleData{PlayerFields: d.PlayerFields}
	checked := 0
	for _, r := range d.Records {
		if r.Kind != "players" {
			continue
		}
		before := [2]Player{playerFromOracle(fields, r.Before.Players[0]), playerFromOracle(fields, r.Before.Players[1])}
		want := [2]Player{playerFromOracle(fields, r.After.Players[0]), playerFromOracle(fields, r.After.Players[1])}
		if before[0].y() == want[0].y() && before[1].y() == want[1].y() {
			continue
		}
		w := ActiveWorld{Enemies: EnemySystem{Slots: enemyPoolFromOracle(r.Before.Enemies)}, Scores: arcadeScoresFromOracle(t, r.Before.Score)}
		w.Combat.Players = before
		w.Combat.World.Level = r.Before.World[1]
		w.Combat.Weapons = originalWeaponPool(t, r.Before.Weapons)
		var inputs [2]PlayerInput
		for i, p := range before {
			v := oraclePlayerValues(fields, r.Before.Players[i])
			inputs[i] = PlayerInput{Held: Buttons(v["held"]), Pressed: Buttons(v["pressed"])}
			w.Combat.Display[i] = LogicalSprite{Code: p.DisplaySprite, X: p.X, Y: p.y(), Attributes: p.DisplayAttribute}
		}
		if _, e := w.Enemies.CollidePlayers(r.Slot, &EnemyCollisionContext{Players: &w.Combat.Players, Inputs: inputs, Weapons: &w.Combat.Weapons, Scores: &w.Scores, Level: r.Before.World[1], SceneType: r.Before.World[2], Flash: r.Before.World[3]}); e != nil {
			t.Fatal(e)
		}
		if e := w.compose(); e != nil {
			t.Fatal(e)
		}
		for actor, p := range want {
			expected := LogicalSprite{Code: p.DisplaySprite, X: p.X, Y: p.y(), Attributes: p.DisplayAttribute}
			if w.Combat.Display[actor] != expected {
				t.Fatalf("original record%d actor%d got%+v want%+v", r.ID, actor, w.Combat.Display[actor], expected)
			}
		}
		checked++
	}
	if checked != 20 {
		t.Fatal("platform-contact source coverage", checked)
	}
}
