package contra

// Player labels must come from recovered player animation outputs, not numeric
// adjacency in the shared sprite table. Unused/effect/cinematic art may look
// like a body fragment but must not acquire gameplay player identity.
import (
	"sort"
	"testing"
)

func TestBuiltinPlayerRecipesUseRecoveredAnimationOutputs(t *testing.T) {
	allowed := map[uint8]bool{}
	emit := func(scene uint8, p Player, water bool) {
		world := OutdoorWorld{SceneType: scene}
		var terrain CollisionWindow
		if water {
			for i := range terrain {
				terrain[i] = 0xaa
			}
		}
		p.State = 1
		p.X = 100
		p.Motion.Position = 100 << 8
		context := PlayerStepContext{World: &world, Terrain: &terrain}
		var sounds []uint8
		if err := p.animatePlayer(&context, &sounds); err != nil {
			t.Fatal(err)
		}
		allowed[p.Sprite] = true
	}
	for _, scene := range []uint8{0, 1, 0x80} {
		maxSequence := 4
		if scene == 1 {
			maxSequence = 7
		}
		for sequence := 0; sequence <= maxSequence; sequence++ {
			for frame := 0; frame < 6; frame++ {
				if scene == 0 && sequence == 4 && frame >= 5 {
					continue
				}
				for aim := uint8(0); aim < 10; aim++ {
					for recoil := uint8(0); recoil < 2; recoil++ {
						for _, timer := range []uint8{0, 27} {
							emit(scene, Player{Sequence: uint8(sequence), Recoil: recoil, SpecialSpriteTimer: timer, Motion: PlayerMotion{AnimationFrame: uint8(frame), Aim: aim}}, false)
						}
					}
				}
			}
		}
	}
	for frame := uint8(0); frame < 4; frame++ {
		emit(0, Player{Motion: PlayerMotion{Jump: 1, AnimationFrame: frame}}, false)
	}
	emit(0, Player{Fall: 1}, false)
	for aim := uint8(0); aim < 10; aim++ {
		for recoil := uint8(0); recoil < 2; recoil++ {
			emit(0, Player{Water: 4, Recoil: recoil, Motion: PlayerMotion{Aim: aim}}, true)
			emit(0, Player{Water: 4, Recoil: recoil, Motion: PlayerMotion{Aim: aim}}, false)
		}
	}
	emit(0, Player{Water: 0x10, WaterTimer: 5}, true)
	if !allowed[115] || !allowed[145] {
		t.Fatal("animation corpus omitted water entry or base exit")
	}
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	recipes, err := botBuiltinSpriteRecipes(bundle)
	if err != nil {
		t.Fatal(err)
	}
	invalid := map[uint8]bool{}
	for _, recipe := range recipes {
		if recipe.Label.Class == "player" && !allowed[recipe.Code] {
			invalid[recipe.Code] = true
		}
	}
	if len(invalid) != 0 {
		var codes []int
		for code := range invalid {
			codes = append(codes, int(code))
		}
		sort.Ints(codes)
		t.Fatal("non-player animation codes labeled as players", codes)
	}
}
