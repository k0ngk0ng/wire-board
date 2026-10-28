package contra

import (
	"bytes"
	"testing"
)

// A previous slot's explosion must affect a later Giant sound's source carry,
// while each actual sound is delivered once. The audio callback is a pure
// projection, so the final event stream must still include both requests.
func TestEnemyTickSoundCarryIncludesEarlierSlots(t *testing.T) {
	s := EnemySystem{}
	s.Slots[15] = Enemy{Kind: 6, Routine: 7, Sprite: 5, StateWidth: 0x83, X: 200, Y: 100}
	s.Slots[14] = Enemy{Kind: 19, Routine: 5, Sprite: 0xba, StateWidth: 0x81, X: 100, Y: 155, HP: 64, AnimationDelay: 2}
	var players [2]Player
	weapons := WeaponSystem{}
	scores := NewArcadeScores()
	terrain := CollisionWindow{}
	calls := 0
	c := EnemyStepContext{Level: 5, Frame: 127, Random: 0, Terrain: &terrain}
	c.SoundRequestCarry = func(code uint8, prefix EnemyAudioPrefix) (bool, error) {
		calls++
		if code != 0x2b || prefix.Reset || !bytes.Equal(prefix.Sounds, []uint8{0x2f}) {
			t.Fatalf("sound projection code%02x prefix%+v", code, prefix)
		}
		return true, nil
	}
	collisions := EnemyCollisionContext{Level: 5, Players: &players, Weapons: &weapons, Scores: &scores}
	result, err := s.Tick(&c, &collisions)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || s.Slots[14].AnimationDelay != 0x82 || !bytes.Equal(result.Sounds, []uint8{0x2f, 0x2b}) {
		t.Fatalf("projection calls%d Giant%+v sounds%v", calls, s.Slots[14], result.Sounds)
	}
}
