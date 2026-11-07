package contra

// Failures considered before indoor pickup support: using outdoor floor
// queries in perspective rooms, wrong initial horizontal speed, crossing the
// barrier before an item is collectible, dropping a resting item after a map
// redraw, and changing the established outdoor behavior.
import "testing"

func TestOriginalBasePickups(t *testing.T) {
	verifyBaseOriginal(t, []int{0}, func(s *EnemySystem, slot int, c EnemyStepContext) error { return s.StepPickup(slot, c) })
}
