package contra

// Before co-op water-route recovery: a trailing player must not keep returning
// to an unreachable high bank while a visibly swimming teammate waits below.
// Require public stage/pose/position evidence; retain reachable dry routes.
import "testing"

func coopWaterObservation(actor int) BotObservation {
	o := BotObservation{Tick: 12, Actor: actor, Seats: 2, Phase: "battle", Stage: 0, StageKnown: true, Scene: 0, SceneKnown: true, GridKnown: true}
	o.Players[actor] = BotPlayer{X: 28, Y: 68, Alive: true, Visible: true, Sprite: 8, LivesKnown: true, Lives: 10}
	o.Players[actor^1] = BotPlayer{X: 168, Y: 212, Alive: true, Visible: true, Sprite: 14, LivesKnown: true, Lives: 10}
	o.PlayerCandidates = []BotPlayerCandidate{{Mask: 3, Exact: true, Player: BotPlayer{X: 168, Y: 212, Sprite: 29, Visible: true, Alive: true}}}
	for y := 6; y < 12; y++ {
		for x := 0; x < 2; x++ {
			o.Terrain[y][x] = TerrainSolid
			o.TerrainKnown[y][x] = true
		}
	}
	for y := 6; y < 12; y++ {
		for x := 11; x < 17; x++ {
			o.Terrain[y][x] = TerrainSolid
			o.TerrainKnown[y][x] = true
		}
	}
	return o
}
func TestBotCoopCanFollowVisibleSwimmingPeer(t *testing.T) {
	for actor := 0; actor < 2; actor++ {
		o := coopWaterObservation(actor)
		b := Bot{StageKnown: true, SceneKnown: true, Phase: "battle", LastTick: 11, LastX: 28, LastY: 68, JumpCooldown: 7, JumpTargetActive: true, JumpTargetX: 28}
		if !b.followSwimmingPeer(o) {
			t.Fatal("visible lower co-op route not considered", actor)
		}
		pad := b.Decide(o)
		if pad&ButtonRight == 0 || pad&(ButtonLeft|ButtonA) != 0 || b.JumpTargetActive {
			t.Fatal("continued jumping back to the old bank", actor, pad)
		}
	}
}
func TestBotCoopWaterRouteRequiresVisibleEvidence(t *testing.T) {
	for _, mode := range []string{"solo", "vertical", "unknown-stage", "other-stage", "base", "no-head", "partial-head", "wrong-mask", "invisible-peer", "dying-peer", "reachable-bank"} {
		t.Run(mode, func(t *testing.T) {
			o := coopWaterObservation(0)
			b := Bot{}
			switch mode {
			case "solo":
				o.Seats = 1
			case "vertical":
				o.Vertical = true
			case "unknown-stage":
				o.StageKnown = false
			case "other-stage":
				o.Stage = 2
			case "base":
				o.Scene = 1
			case "no-head":
				o.PlayerCandidates = nil
			case "partial-head":
				o.PlayerCandidates[0].Exact = false
			case "wrong-mask":
				o.PlayerCandidates[0].Mask = 1
			case "invisible-peer":
				o.Players[1].Visible = false
			case "dying-peer":
				o.Players[1].Sprite = 11
			case "reachable-bank":
				for y := 7; y < 12; y++ {
					for x := 4; x < 7; x++ {
						o.Terrain[y][x] = TerrainSolid
						o.TerrainKnown[y][x] = true
					}
				}
			}
			if b.followSwimmingPeer(o) {
				t.Fatal("invented or unnecessary water route", mode)
			}
		})
	}
}
