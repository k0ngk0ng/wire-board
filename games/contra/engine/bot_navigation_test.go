package contra

// Before the edge fix: a visible body near a platform edge must jump while
// still supported; waiting until its center crosses the edge loses the legal
// jump window. A safe lower ledge may be walked onto, a ceiling must reject a
// jump path, and an ordinary long floor must not trigger repeated jumping.
// Respawn descent must also remain over a reachable visible platform instead
// of drifting into a gap; it cannot create a midair A press or override an
// already selected jump landing.
import "testing"

func botNavigationFloor(tick uint64, x int) BotObservation {
	o := BotObservation{Tick: tick, Actor: 0, Seats: 1, Phase: "battle", StageKnown: true, SceneKnown: true, GridKnown: true}
	o.Players[0] = BotPlayer{X: x, Y: 67, Alive: true, Visible: true, Sprite: 3}
	for y := range o.TerrainKnown {
		for x := range o.TerrainKnown[y] {
			o.TerrainKnown[y][x] = true
		}
	}
	for x := 0; x < 5; x++ {
		o.Terrain[5][x] = TerrainPlatform
	}
	return o
}

func TestBotVisibleCliffStartsJumpBeforeLosingSupport(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 71+int(tick))
		for x := 7; x < 17; x++ {
			o.Terrain[7][x] = TerrainPlatform
		}
		pad := b.Decide(o)
		if tick == 2 && pad&ButtonA == 0 {
			t.Fatal("walked off a visible deep gap before starting the available jump", pad)
		}
	}
}

func TestBotVisibleContinuousFloorDoesNotJump(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 71+int(tick))
		for x := range o.Terrain[5] {
			o.Terrain[5][x] = TerrainPlatform
		}
		if pad := b.Decide(o); pad&ButtonA != 0 {
			t.Fatal("invented a cliff on a continuous visible floor", pad)
		}
	}
}

func TestBotVisibleSafeLowerLedgeDoesNotNeedJump(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 71+int(tick))
		for x := 5; x < 17; x++ {
			o.Terrain[7][x] = TerrainPlatform
		}
		if pad := b.Decide(o); pad&ButtonA != 0 {
			t.Fatal("jumped instead of taking a visible safe drop", pad)
		}
	}
}

func TestBotVisibleCliffWithCeilingDoesNotInventJump(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 71+int(tick))
		for x := 0; x < 17; x++ {
			o.Terrain[3][x] = TerrainSolid
			if x >= 7 {
				o.Terrain[7][x] = TerrainPlatform
			}
		}
		if pad := b.Decide(o); pad&ButtonA != 0 {
			t.Fatal("jumped into a visible ceiling", pad)
		}
	}
}

func TestBotVisibleRespawnDescentKeepsReachableLanding(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 66+int(tick))
		clear(o.Terrain[:])
		for x := 0; x < 5; x++ {
			o.Terrain[11][x] = TerrainPlatform
		}
		for x := 7; x < 17; x++ {
			o.Terrain[9][x] = TerrainPlatform
		}
		o.Players[0].Y = 105 + int(tick)*5
		o.Players[0].Sprite = 8
		pad := b.Decide(o)
		if pad&ButtonA != 0 {
			t.Fatal("created an unavailable midair jump", pad)
		}
		if tick == 2 && pad&ButtonRight != 0 {
			t.Fatal("drifted off the only reachable visible landing", pad)
		}
	}
}

// The descent predictor must not place the character ahead of the observed
// body without spending movement time. A tiny deep ledge beyond a wide gap
// is not a safe drop merely because an artificial horizontal offset reaches it.
func TestBotVisibleDeepLedgeDoesNotCancelNecessaryJump(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 102+int(tick))
		clear(o.Terrain[:])
		o.Players[0].Y = 100
		for x := 3; x <= 6; x++ {
			o.Terrain[7][x] = TerrainPlatform
		}
		for x := 9; x <= 12; x++ {
			o.Terrain[9][x] = TerrainPlatform
		}
		for x := 9; x <= 10; x++ {
			o.Terrain[13][x] = TerrainPlatform
		}
		if pad := b.Decide(o); tick == 2 && pad&ButtonA == 0 {
			t.Fatal("a physically unreachable deep ledge cancelled the safe jump", pad)
		}
	}
}

// Walking off a ledge has a publicly distinct falling pose and an initial
// horizontal commitment. It cannot use the immediately steerable respawn/jump
// predictor to return onto a platform that is already above its feet.
func TestBotVisibleLedgeFallKeepsCommittedDirection(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 109+int(tick)*2)
		clear(o.Terrain[:])
		o.Players[0].Y = 100
		if tick == 2 {
			o.Players[0].Sprite = 5
		}
		for x := 3; x <= 6; x++ {
			o.Terrain[7][x] = TerrainPlatform
		}
		for x := 0; x <= 4; x++ {
			o.Terrain[9][x] = TerrainPlatform
		}
		for x := 9; x <= 12; x++ {
			o.Terrain[9][x] = TerrainPlatform
		}
		for x := 9; x <= 10; x++ {
			o.Terrain[13][x] = TerrainPlatform
		}
		if pad := b.Decide(o); tick == 2 && pad&(ButtonLeft|ButtonA) != 0 {
			t.Fatal("used steerable-jump planning during committed ledge fall", pad)
		}
	}
}

// The submitted picture can still show the last standing pose immediately
// after a jump input. Keep the chosen landing through that presentation delay
// instead of replacing it with a return to the old platform at the apex.
func TestBotVisibleJumpGoalSurvivesSubmittedStandingFrame(t *testing.T) {
	var b Bot
	makeObservation := func(tick uint64, x, y int, sprite uint8) BotObservation {
		o := botNavigationFloor(tick, x)
		o.Players[0].Y, o.Players[0].Sprite = y, sprite
		for x := 7; x < 17; x++ {
			o.Terrain[7][x] = TerrainPlatform
		}
		return o
	}
	b.Decide(makeObservation(1, 72, 67, 3))
	if pad := b.Decide(makeObservation(2, 73, 67, 3)); pad&ButtonA == 0 {
		t.Fatal("setup did not start the visible gap jump")
	}
	goal := b.JumpTargetX
	for _, o := range []BotObservation{makeObservation(3, 73, 67, 3), makeObservation(4, 74, 63, 8)} {
		pad := b.Decide(o)
		if !b.JumpTargetActive || b.JumpTargetX != goal || pad&ButtonA != 0 {
			t.Fatal("submitted standing frame lost the pending landing or retriggered jump", o.Tick, goal, b.JumpTargetX, b.JumpTargetActive, pad)
		}
	}
}

// A high ledge can be reached first by the foot center at its near edge. The
// intended target must lie inside the ledge, but rejecting that first legal
// contact because the full body width has not crossed yet traps the bot below.
func TestBotVisibleHighLedgeAcceptsCenterFirstContact(t *testing.T) {
	var b Bot
	for tick := uint64(1); tick <= 2; tick++ {
		o := botNavigationFloor(tick, 124+int(tick))
		clear(o.Terrain[:])
		o.ViewX = 10
		o.Players[0].Y = 148
		for x := 7; x <= 8; x++ {
			o.Terrain[10][x] = TerrainPlatform
		}
		for x := 11; x <= 14; x++ {
			o.Terrain[7][x] = TerrainPlatform
		}
		pad := b.Decide(o)
		if tick == 2 && (pad&ButtonA == 0 || !b.JumpTargetActive || b.JumpTargetX < 170) {
			t.Fatal("rejected reachable ledge and repeated a jump on the original platform", pad, b.JumpTargetX, b.JumpTargetActive)
		}
	}
}

func TestBotVisibleVerticalJumpGoalSurvivesSubmittedStandingFrame(t *testing.T) {
	var b Bot
	makeObservation := func(tick uint64, y int, sprite uint8) BotObservation {
		o := botNavigationFloor(tick, 96)
		clear(o.Terrain[:])
		o.Stage, o.Vertical = 2, true
		o.Players[0].Y, o.Players[0].Sprite = y, sprite
		for x := 0; x < 17; x++ {
			o.Terrain[11][x] = TerrainPlatform
		}
		for x := 8; x <= 10; x++ {
			o.Terrain[8][x] = TerrainPlatform
		}
		return o
	}
	b.Decide(makeObservation(1, 164, 3))
	if pad := b.Decide(makeObservation(2, 164, 3)); pad&ButtonA == 0 {
		t.Fatal("setup did not start the visible climb", pad)
	}
	goal := b.JumpTargetX
	for _, o := range []BotObservation{makeObservation(3, 164, 3), makeObservation(4, 160, 8)} {
		pad := b.Decide(o)
		if !b.JumpTargetActive || b.JumpTargetX != goal || pad&ButtonA != 0 {
			t.Fatal("vertical display delay lost the landing", o.Tick, goal, b.JumpTargetX, b.JumpTargetActive, pad)
		}
	}
}
