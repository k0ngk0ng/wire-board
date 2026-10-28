package contra

import "testing"

// An aligned low-core shooter is crouching. Dodging a nearby roller requires
// A with Down released: neutral Down+A enters the original drop-check path.
func TestBotBaseRollerDodgeReleasesCrouchForJump(t *testing.T) {
	o := botBaseObservation(4)
	o.Players[0].X, o.Players[0].Y = 131, 168
	o.Enemies = []BotEnemy{{X: 128, Y: 112, Kind: 20}, {X: 131, Y: 160, Kind: 17}}
	var b Bot
	pad := b.play(o)
	if pad&ButtonA == 0 || pad&ButtonDown != 0 {
		t.Fatal("roller dodge retains neutral Down+A instead of jumping", pad)
	}
}
