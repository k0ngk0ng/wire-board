package contra

import "errors"

// StepCampaignInitialize is original CC5F's presentation preparation. It
// selects semantic resources, never an executable enemy-table pointer. The
// caller applies returned display/audio effects in this phase, then completes
// the routine boundary exactly once through Campaign.AfterPresentation.
func StepCampaignInitialize(c *Campaign, w *ActiveWorld, resources *LevelResources) (CampaignSceneResult, error) {
	if err := campaignInitializeContext(c, w, resources, 0); err != nil {
		return CampaignSceneResult{}, err
	}
	pages, ok := campaignScenePresets[10]
	if !ok {
		return CampaignSceneResult{}, errors.New("missing introduction pattern preset")
	}
	packets, err := sceneTextPackets(6, false, false)
	if err != nil {
		return CampaignSceneResult{}, err
	}
	next := *w
	out := campaignClearBackground(&next)
	out.Controls = append([]CampaignSceneControl{{Kind: "display-mask", Value: 0x1e}}, out.Controls...)
	out.Sounds = []WorldSoundCommand{{Kind: "reset"}}
	next.Presentation.Patterns.Desired = pages
	// C910 discarded the prior queue. The introduction uses CAC5's text parser.
	next.Presentation.Queue.Mode = GraphicsText
	for _, packet := range packets {
		if err = next.Presentation.Queue.AppendText(packet); err != nil {
			return CampaignSceneResult{}, err
		}
	}
	*w = next
	return out, nil
}

// StepCampaignPrepare is original CC7D. Only transient object pages are
// cleared; player zero-page state, lives, weapons, collision and the unused
// map half survive. Header loading and snow initialization have their existing
// semantic owners. Demo affects music only and is explicit at this boundary.
func StepCampaignPrepare(c *Campaign, w *ActiveWorld, resources *LevelResources, demo bool) (CampaignSceneResult, error) {
	if err := campaignInitializeContext(c, w, resources, 2); err != nil {
		return CampaignSceneResult{}, err
	}
	info, err := resources.Stage(int(w.Combat.World.Level))
	if err != nil {
		return CampaignSceneResult{}, err
	}
	pages, err := resources.CHRPreset(info.CHRPreset)
	if err != nil {
		return CampaignSceneResult{}, err
	}
	next, nc := *w, *c
	// C328 clears 0300..05FF, not player motion fractions/velocity in zero page.
	next.Combat.Display = [10]LogicalSprite{}
	next.Combat.Weapons = WeaponSystem{}
	next.Enemies.Slots = [16]Enemy{}
	for i := range next.Combat.Players {
		p := &next.Combat.Players[i]
		p.X, p.DisplaySprite, p.DisplayAttribute = 0, 0, 0
		p.setY(0)
	}
	out := campaignClearBackground(&next)
	next.Presentation.Patterns.Desired = pages
	world := &next.Combat.World
	world.BossDefeated = 0
	nc.AliveAtExit = 0
	world.SceneType, world.ScrollType, world.StopScroll = info.SceneType, info.ScrollType, info.StopScrollScreen
	next.Header = info.Initial
	next.Combat.SolidCollision = info.CollisionCheck
	next.Terrain.AlternateGraphicsScreen = info.Alternate.TriggerScreen
	next.Terrain.Variant = RegularMap
	next.Snow = SnowControlState{}
	next.Animation.Timer, next.Animation.Pace = 0, 0
	if err = next.Presentation.Sprites.ResetSnow(); err != nil {
		return CampaignSceneResult{}, err
	}
	// DAD3 resets only cursors used by the selected streaming direction.
	world.Screen, world.ScreenScroll, next.Terrain.MapOffset = 0, 0, 0
	if world.ScrollType != 0 {
		next.Terrain.Axis = 29
		next.Terrain.TileAddress = 0x23a0
	} else {
		next.Terrain.TransitionRemaining = 48
		if world.SceneType != 0 {
			next.Presentation.Palette.FadeTimer = 16
			effect, e := next.Presentation.Palette.Load(16, world.SceneType, next.Header, resources)
			if e != nil {
				return CampaignSceneResult{}, e
			}
			next.applyPaletteBias(effect)
			next.Terrain.TransitionRemaining = 32
		}
		next.Terrain.Axis = 0
		next.Terrain.TileAddress = 0x2000
		next.Terrain.AttributeAddress = 0x23c0
	}
	if err = resources.LoadMap(int(world.Level), RegularMap, 0, 0, &next.Terrain.Supertiles); err != nil {
		return CampaignSceneResult{}, err
	}
	effect, err := next.Presentation.Palette.Load(32, world.SceneType, next.Header, resources)
	if err != nil {
		return CampaignSceneResult{}, err
	}
	next.applyPaletteBias(effect)
	world.Scroll.Y = [8]uint8{0xe0, 0xe8, 0, 0xe8, 0xe0, 0xe0, 0xe0, 0xe0}[world.Level]
	if !demo {
		out.Sounds = append(out.Sounds, WorldSoundCommand{Kind: "sound", ID: [8]uint8{0x46, 0x5a, 0x4a, 0x5a, 0x4e, 0x52, 0x46, 0x56}[world.Level]})
	}
	next.Presentation.Queue.Mode = GraphicsRectangular
	out.Controls = append(out.Controls, CampaignSceneControl{Kind: "queue-mode", Value: 0xff})
	*w, *c = next, nc
	return out, nil
}

func campaignInitializeContext(c *Campaign, w *ActiveWorld, resources *LevelResources, routine uint8) error {
	if c == nil || w == nil || resources == nil || w.Presentation.Terminated || w.Combat.World.GameRoutine != 5 || w.Combat.World.LevelRoutine != routine || w.Combat.World.Level >= 8 || w.Combat.World.PlayerMode > 1 {
		return errors.New("invalid campaign initialization context")
	}
	_, err := resources.Stage(int(w.Combat.World.Level))
	return err
}

func campaignClearBackground(w *ActiveWorld) CampaignSceneResult {
	w.Presentation.Queue = GraphicsQueue{Mode: w.Presentation.Queue.Mode}
	w.Combat.World.Scroll.X, w.Combat.World.Scroll.Y = 0, 0
	return CampaignSceneResult{ImmediateBackground: sceneBackground(0), Controls: []CampaignSceneControl{{Kind: "display-disable", Value: 0}, {Kind: "display-busy", Value: 5}, {Kind: "ppu-control", Value: 0xb0}}}
}
