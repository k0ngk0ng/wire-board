package contra

import "errors"

// ApplyCampaignRuntimeReset applies a decision already made by Campaign. The
// original CD1F/C309 clears transient zero-page owners and object/map storage;
// it does not create a new world, recompute a result, or reset display/audio.
// In particular 0190..0196, snow particles, desired colors/pages and scores are
// outside that clear. The command carries authoritative life/weapon resources.
func ApplyCampaignRuntimeReset(w *ActiveWorld, scene *CampaignSceneState, reset *CampaignReset) error {
	if w == nil || scene == nil || reset == nil || reset.PlayerMode > 1 || reset.PlayerMode != w.Combat.World.PlayerMode {
		return errors.New("invalid campaign reset context")
	}
	switch reset.Kind {
	case CampaignResetAdvance, CampaignResetGameOver:
		if !reset.ClearStageRuntime || reset.ClearArcadeScores || reset.Players[0].State != 0 || reset.Players[1].State != 0 {
			return errors.New("invalid stage reset command")
		}
	case CampaignResetContinue:
		if reset.ClearStageRuntime || !reset.ClearArcadeScores {
			return errors.New("invalid continue reset command")
		}
	default:
		return errors.New("unknown campaign reset command")
	}
	next, ns := *w, *scene
	if reset.ClearStageRuntime {
		next.HUD = false
		world := &next.Combat.World
		world.SceneType, world.ScrollType, world.StopScroll = 0, 0, 0
		world.Screen, world.ScreenScroll, world.FrameScroll = 0, 0, 0
		world.AutoScroll = [2]uint8{}
		world.BossScrollComplete, world.ActivePlayers, world.AttackEnabled, world.IndoorScroll = 0, 0, 0, 0
		next.Combat.SolidCollision, next.Combat.ScreenCleared = 0, 0
		next.Combat.Terrain = CollisionWindow{}
		next.Combat.Weapons = WeaponSystem{}
		next.Combat.Display = [10]LogicalSprite{}
		next.Combat.Players = [2]Player{}
		next.Terrain = TerrainScrollState{}
		next.Header = PaletteState{}
		next.Animation = CHRAnimation{}
		next.Snow = SnowControlState{}
		generator := next.Enemies.Generator
		next.Enemies = EnemySystem{Generator: SoldierGenerator{Screen: generator.Screen, Generated: generator.Generated}}
		palette := &next.Presentation.Palette
		palette.FadeTimer, palette.Cycle, palette.PauseCycle, palette.FlashTimer = 0, 0, 0, 0
		ns.IntroScroll, ns.Reveal = 0, 0
		ns.Interlude = CampaignInterludeState{}
	}
	if reset.ClearArcadeScores {
		next.Scores.Hundreds = [2]uint16{}
		next.Scores.NextLife = reset.NextLife
	}
	for i, resources := range reset.Players {
		p := &next.Combat.Players[i]
		p.State, p.GameOver, p.Lives, p.Weapon = resources.State, resources.GameOver, resources.Lives, resources.Weapon
	}
	*w, *scene = next, ns
	return nil
}
