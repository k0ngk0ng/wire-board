package contra

import "errors"

// World owns persistent scene and campaign state. Pending Campaign work is
// completed within one transaction; it is never serialized as a savepoint.
// Audio/display submission and the deterministic entropy adapter run at the
// outer frame boundary, before Advance consumes controller samples.
type World struct {
	Active   ActiveWorld
	Campaign Campaign
	Scene    CampaignSceneState
}

// Advance connects the recovered jungle battle/exit/game-over phases. Scene
// initialization, intro and menus are enabled separately as their runners land.
// tick is monotonic and is independent of the original wrapping Frame byte.
func (w *World) Advance(tick uint64, inputs [2]PlayerInput, resources *LevelResources) ([]WorldSoundCommand, error) {
	return w.advanceAtomic(tick, inputs, resources, nil)
}

func (w *World) AdvanceWithAudio(tick uint64, inputs [2]PlayerInput, resources *LevelResources, audio *AudioState) ([]WorldSoundCommand, error) {
	if audio == nil {
		return nil, errors.New("missing world audio control")
	}
	return w.advanceAtomic(tick, inputs, resources, audio)
}

func (w *World) advanceAtomic(tick uint64, inputs [2]PlayerInput, resources *LevelResources, audio *AudioState) ([]WorldSoundCommand, error) {
	if w == nil || resources == nil {
		return nil, errors.New("incomplete world tick")
	}
	if err := w.Campaign.Validate(&w.Active.Combat.World, true); err != nil {
		return nil, err
	}
	if w.Campaign.HasTick {
		if tick == w.Campaign.LastTick {
			return nil, nil
		}
		if tick < w.Campaign.LastTick {
			return nil, errors.New("world tick moved backwards")
		}
	}
	if w.Campaign.Outcome != CampaignOngoing {
		return nil, nil
	}
	if w.Active.Presentation.Terminated || w.Active.Combat.World.Level >= 8 {
		return nil, errors.New("world requires submitted presentation and recovered stage")
	}
	switch w.Active.Combat.World.LevelRoutine {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11:
	default:
		return nil, errors.New("campaign scene runner is not connected")
	}
	next := *w
	next.Active.Presentation.Queue = w.Active.Presentation.Queue.Clone()
	next.Active.Enemies.PendingSounds = append([]uint8(nil), w.Active.Enemies.PendingSounds...)
	var nextAudio *AudioState
	if audio != nil {
		copy := audio.Clone()
		nextAudio = &copy
	}
	commands, err := next.advance(tick, inputs, resources, nextAudio)
	if err != nil {
		return nil, err
	}
	if err = next.Campaign.Validate(&next.Active.Combat.World, true); err != nil {
		return nil, err
	}
	*w = next
	if audio != nil {
		*audio = *nextAudio
	}
	return commands, nil
}

func (w *World) advance(tick uint64, inputs [2]PlayerInput, resources *LevelResources, audio *AudioState) ([]WorldSoundCommand, error) {
	s, c := &w.Active, &w.Campaign
	world := &s.Combat.World
	origin := world.LevelRoutine
	world.Frame++
	if world.LevelRoutine == 4 {
		s.Flicker = true
		if c.Paused {
			s.projectPausedSprites()
			if err := s.animate(resources); err != nil {
				return nil, err
			}
		}
	}
	plan, err := c.BeginTick(tick, world, CampaignInput{Pressed: inputs[0].Pressed, Players: w.players()})
	if err != nil {
		return nil, err
	}
	var commands []WorldSoundCommand
	emit := func(values ...WorldSoundCommand) {
		commands = append(commands, values...)
		if audio != nil {
			for _, v := range values {
				audio.Queue(AudioCommand{Kind: v.Kind, ID: v.ID})
			}
		}
	}
	consume := func(plan CampaignPlan) error {
		for _, event := range plan.Commands {
			switch event.Kind {
			case "stopAudio":
				emit(WorldSoundCommand{Kind: "reset"})
			case "resetRuntime", "restartLevel":
				if err := ApplyCampaignRuntimeReset(s, &w.Scene, event.Reset); err != nil {
					return err
				}
			case "advanceLevel", "offerContinue", "cleared", "failed":
				// Campaign owns the decision; this is its once-only audit event.
			default:
				return errors.New("world campaign command requires scene runner")
			}
		}
		for _, id := range plan.Sounds {
			emit(WorldSoundCommand{Kind: "sound", ID: id})
		}
		return nil
	}
	if err = consume(plan); err != nil {
		return nil, err
	}
	if origin == 5 && (world.LevelRoutine == 6 || world.LevelRoutine == 7) {
		out, e := PrepareCampaignGameOver(s, world.LevelRoutine == 6)
		if e != nil {
			return nil, e
		}
		if err = w.applyScene(out); err != nil {
			return nil, err
		}
	}
	if origin == 6 {
		out, e := StepCampaignContinueDisplay(s, c, inputs[0].Pressed)
		if e != nil {
			return nil, e
		}
		if err = w.applyScene(out); err != nil {
			return nil, err
		}
	}
	if plan.Work == CampaignInitializeLevel || plan.Work == CampaignLevelIntro || plan.Work == CampaignPrepareBattle || plan.Work == CampaignLevelEntrance || plan.Work == CampaignRunInterlude {
		work := plan.Work
		var out CampaignSceneResult
		completed := false
		switch work {
		case CampaignInitializeLevel:
			out, err = StepCampaignInitialize(c, s, resources)
			completed = true
		case CampaignLevelIntro:
			out, err = w.Scene.StepIntro(c, s, inputs, false)
		case CampaignPrepareBattle:
			out, err = StepCampaignPrepare(c, s, resources, false)
			completed = true
		case CampaignLevelEntrance:
			completed, err = StepCampaignEntrance(s, resources)
		case CampaignRunInterlude:
			out, err = w.Scene.StepInterlude(c, s, inputs, CampaignSceneEnvironment{Busy: s.Presentation.Display.Busy, DisplayMask: s.Presentation.Display.Mask})
		}
		if err != nil {
			return nil, err
		}
		if err = w.applyScene(out); err != nil {
			return nil, err
		}
		if work == CampaignLevelEntrance && completed {
			// CCC5 publishes the lives HUD only after DB1C finishes streaming.
			s.HUD = true
		}
		emit(out.Sounds...)
		plan, err = c.AfterPresentation(world, completed)
		if err != nil {
			return nil, err
		}
		if work == CampaignInitializeLevel || work == CampaignPrepareBattle {
			// The real phase already emitted these requests in instruction order.
			// The campaign plan records the same boundary, not another playback.
			var boundary []WorldSoundCommand
			for _, event := range plan.Commands {
				if event.Kind != "stopAudio" {
					return nil, errors.New("unexpected initialization campaign command")
				}
				boundary = append(boundary, WorldSoundCommand{Kind: "reset"})
			}
			for _, id := range plan.Sounds {
				boundary = append(boundary, WorldSoundCommand{Kind: "sound", ID: id})
			}
			if len(boundary) != len(out.Sounds) {
				return nil, errors.New("initialization audio boundary mismatch")
			}
			for i := range boundary {
				if boundary[i] != out.Sounds[i] {
					return nil, errors.New("initialization audio order mismatch")
				}
			}
		} else if err = consume(plan); err != nil {
			return nil, err
		}
	}
	if plan.Work == CampaignRunEndSequence {
		if err = w.Scene.StepEndSequence(c, s, &inputs); err != nil {
			return nil, err
		}
		plan, err = c.AfterPresentation(world, false)
		if err != nil {
			return nil, err
		}
		if err = consume(plan); err != nil {
			return nil, err
		}
	}
	if plan.Work == CampaignCombat {
		if c.OriginRoutine != 9 {
			if _, err = s.Snow.Advance(world.Level, world.Screen, world.ScreenScroll, &s.Animation); err != nil {
				return nil, err
			}
		}
		combat, e := s.Combat.advanceWithWork(inputs, &s.Work, &s.ThrowWork)
		if e != nil {
			return nil, e
		}
		if combat.GenerationBiasChanged {
			s.GenerationBias = combat.GenerationBiasValue
		}
		for _, id := range combat.Sounds {
			emit(WorldSoundCommand{Kind: "sound", ID: id})
		}
		plan, err = c.AfterCombat(world, w.players())
		if err != nil {
			return nil, err
		}
		if err = consume(plan); err != nil {
			return nil, err
		}
	}
	if plan.Work == CampaignEnemies || plan.Work == CampaignExitEnemies {
		part, e := s.advanceEnvironment(inputs, resources, c, plan.Work == CampaignExitEnemies, audio)
		if e != nil {
			return nil, e
		}
		emit(part...)
		plan, err = c.AfterEnemies(world, w.players())
		if err != nil {
			return nil, err
		}
		if err = consume(plan); err != nil {
			return nil, err
		}
	}
	if c.Pending != CampaignNoWork {
		return nil, errors.New("world left unfinished campaign work")
	}
	if origin == 11 && world.LevelRoutine == 5 {
		if err = s.Presentation.DiscardGraphics(GraphicsText); err != nil {
			return nil, err
		}
	}
	if err = s.compose(); err != nil {
		return nil, err
	}
	return commands, nil
}

func (w *World) players() [2]CampaignPlayer {
	var out [2]CampaignPlayer
	for i, p := range w.Active.Combat.Players {
		out[i] = CampaignPlayer{State: p.State, GameOver: p.GameOver, Lives: p.Lives, Weapon: p.Weapon}
	}
	return out
}
