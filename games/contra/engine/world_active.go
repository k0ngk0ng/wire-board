package contra

import "errors"

// ActiveWorld joins the normal battle phases. Campaign entrances/exits and the
// platform tick/entropy scheduler wrap this primitive; it does not claim to
// implement those transitions or initialize a playable campaign by itself.
type ActiveWorld struct {
	ThrowWork      BaseThrowWork
	Combat         CombatFrame
	Terrain        TerrainScrollState
	Header         PaletteState
	Presentation   WorldPresentation
	Enemies        EnemySystem
	Scores         ArcadeScores
	Animation      CHRAnimation
	Snow           SnowControlState
	Random         RandomState
	Work           TerrainWork
	GenerationBias uint8
	HUD, Flicker   bool
}

type WorldSoundCommand struct {
	Kind string `json:"kind"`
	ID   uint8  `json:"id,omitempty"`
}

// Advance runs C001 through the next sprite composition for an active jungle
// frame. The display boundary and entropy input occur before this call. Errors
// preserve the entire scene. Other scene/campaign paths are enabled only after
// their whole-world integration evidence is in place.
func (s *ActiveWorld) Advance(inputs [2]PlayerInput, resources *LevelResources) ([]WorldSoundCommand, error) {
	if s == nil || resources == nil || s.Presentation.Terminated || s.Combat.World.Level != 0 || s.Combat.World.LevelRoutine != 4 || s.Combat.World.GameRoutine != 5 || s.Combat.World.BossDefeated != 0 || inputs[0].Pressed&ButtonStart != 0 {
		return nil, errors.New("active-world phase requires a normal unpaused jungle frame")
	}
	next := *s
	next.Presentation.Queue = s.Presentation.Queue.Clone()
	next.Enemies.PendingSounds = append([]uint8(nil), s.Enemies.PendingSounds...)
	commands, err := next.advance(inputs, resources)
	if err != nil {
		return nil, err
	}
	*s = next
	return commands, nil
}

func (s *ActiveWorld) advance(inputs [2]PlayerInput, resources *LevelResources) ([]WorldSoundCommand, error) {
	s.Combat.World.Frame++
	s.Flicker = true
	return s.advanceBattle(inputs, resources)
}

func (s *ActiveWorld) advanceBattle(inputs [2]PlayerInput, resources *LevelResources) ([]WorldSoundCommand, error) {
	w := &s.Combat.World
	if _, err := s.Snow.Advance(w.Level, w.Screen, w.ScreenScroll, &s.Animation); err != nil {
		return nil, err
	}
	combat, err := s.Combat.advanceWithWork(inputs, &s.Work, &s.ThrowWork)
	if err != nil {
		return nil, err
	}
	var commands []WorldSoundCommand
	addSounds := func(values []uint8) {
		for _, id := range values {
			commands = append(commands, WorldSoundCommand{Kind: "sound", ID: id})
		}
	}
	addSounds(combat.Sounds)
	if combat.GenerationBiasChanged {
		s.GenerationBias = combat.GenerationBiasValue
	}
	if s.Combat.Players[0].GameOver&s.Combat.Players[1].GameOver != 0 {
		return nil, errors.New("active-world game-over transition requires campaign runner")
	}
	part, err := s.advanceEnvironment(inputs, resources, nil, false)
	if err != nil {
		return nil, err
	}
	commands = append(commands, part...)
	if err = s.compose(); err != nil {
		return nil, err
	}
	return commands, nil
}

// advanceEnvironment is CCE8, or CE40's reduced exit path. It does not advance
// players or finish a frame; Campaign owns the transitions around this phase.
func (s *ActiveWorld) advanceEnvironment(inputs [2]PlayerInput, resources *LevelResources, campaign *Campaign, exitOnly bool, audioContext ...*AudioState) ([]WorldSoundCommand, error) {
	w := &s.Combat.World
	var commands []WorldSoundCommand
	addSounds := func(values []uint8) {
		for _, id := range values {
			commands = append(commands, WorldSoundCommand{Kind: "sound", ID: id})
		}
	}
	result, err := s.Terrain.AdvanceQueued(TerrainScrollContext{World: w, Terrain: &s.Combat.Terrain, Header: &s.Header, ScreenCleared: &s.Combat.ScreenCleared, Queue: &s.Presentation.Queue, Resources: resources, Work: &s.Work})
	if err != nil {
		return nil, err
	}
	if result.ResetEnemyReadOffset {
		s.Enemies.ScreenReadOffset = 0
	}
	for i, jump := range result.ForceJump {
		if jump != 0 {
			s.Combat.Players[i].ForceJump = jump
		}
	}
	for _, effect := range result.Effects {
		switch effect.Kind {
		case ScrollCHRPreset:
			pages, e := resources.CHRPreset(int(effect.Value))
			if e != nil {
				return nil, e
			}
			s.Presentation.Patterns.Desired = pages
		case ScrollPaletteTimer:
			s.Presentation.Palette.FadeTimer = effect.Value
		case ScrollPaletteLoad:
			loaded, e := s.Presentation.Palette.Load(effect.Value, w.SceneType, s.Header, resources)
			if e != nil {
				return nil, e
			}
			s.applyPaletteBias(loaded)
		case ScrollSound:
			addSounds([]uint8{effect.Value})
		case ScrollAudioReset:
			commands = append(commands, WorldSoundCommand{Kind: "reset"})
		default:
			return nil, errors.New("unknown world scroll effect")
		}
	}
	tileset, err := resources.Graphics(int(w.Level), s.Terrain.Variant)
	if err != nil {
		return nil, err
	}
	context := EnemyStepContext{ThrowWork: &s.ThrowWork, Level: w.Level, SceneType: w.SceneType, Vertical: w.ScrollType != 0, Frame: w.Frame, FrameScroll: w.FrameScroll,
		Screen: w.Screen, ScreenScroll: w.ScreenScroll, SharedRandom: &s.Random, SpawnVariantSeed: s.GenerationBias,
		WeaponStrength: w.WeaponStrength, AttackEnabled: w.AttackEnabled != 0, SolidBulletBackground: s.Combat.SolidCollision&0x80 != 0,
		SolidPickupBackground: s.Combat.SolidCollision&1 != 0, Terrain: &s.Combat.Terrain, Scroll: w.Scroll, ScreenCleared: &s.Combat.ScreenCleared, TankScroll: &s.Terrain.TankScroll}
	context.AlienBoss = AlienBossContext{AutoScroll: &w.AutoScroll[1], PresentationPace: &s.Animation.Pace, CollisionAddress: &s.Work.Value}
	context.Scene = EnemySceneContext{BossScrollComplete: &w.BossScrollComplete, Palette: &s.Presentation.Palette, Header: &s.Header,
		RefreshBackgroundPalette: func(count uint8) (PaletteResult, error) {
			return s.Presentation.Palette.Load(count, w.SceneType, s.Header, resources)
		}}
	if len(audioContext) > 0 && audioContext[0] != nil {
		// Player/campaign requests are already pending. Terrain effects precede
		// the enemy phase and are added only to this pure projection snapshot.
		entry := audioContext[0].Clone()
		for _, event := range commands {
			entry.Queue(AudioCommand{Kind: event.Kind, ID: event.ID})
		}
		context.SoundRequestCarry = func(code uint8, prefix EnemyAudioPrefix) (bool, error) {
			return projectEnemySound(&entry, code, prefix)
		}
	}
	context.ChangeTerrain = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
		r, e := s.Presentation.Queue.RequestSupertile(req.X, req.Y, req.Code, GraphicsContext{Scroll: w.Scroll, Supertiles: &s.Terrain.Supertiles, Tileset: tileset})
		if e == nil && r.Accepted {
			s.Work.RecordGraphicLocation(r.Address)
		}
		return EnemyTerrainResponse{Accepted: r.Accepted, AddressValid: r.Accepted, Address: r.Address, AttributeLookupWritten: r.AttributeLookupWritten, AttributeLookup: r.AttributeLookup}, e
	}
	context.ChangeTiles = func(req EnemyTerrainRequest) (EnemyTerrainResponse, error) {
		animations, e := OriginalTileAnimations(w.Level)
		if e != nil {
			return EnemyTerrainResponse{}, e
		}
		r, e := s.Presentation.Queue.RequestTileAnimation(req.X, req.Y, req.Code, GraphicsContext{Scroll: w.Scroll, Supertiles: &s.Terrain.Supertiles, Tileset: tileset}, animations)
		if e == nil && r.Accepted {
			s.Work.RecordGraphicLocation(r.Address)
		}
		return EnemyTerrainResponse{Accepted: r.Accepted, AddressValid: r.Accepted, Address: r.Address, AttributeLookupWritten: r.AttributeLookupWritten, AttributeLookup: r.AttributeLookup}, e
	}
	context.ClearQueuedGraphicsPrefix = s.Presentation.Queue.ClearPrefix
	// Boss transitions belong to Campaign, not an inferred clear flag here.
	context.OnBossControl = func(BossControl) error { return errors.New("active-world Boss transition requires campaign runner") }
	if campaign != nil {
		context.GameLoop = campaign.Loops
		context.OnBossControl = func(event BossControl) error {
			switch event.Kind {
			case BossBeginDefeat:
				w.BossDefeated = 1
				campaign.DelayLow = 255
			case BossExitReady:
				campaign.DelayLow = event.Delay
				campaign.DelayHigh = 0
			case BossAudioReset:
			default:
				return errors.New("unknown campaign Boss effect")
			}
			return nil
		}
	}
	collisions := EnemyCollisionContext{Players: &s.Combat.Players, Inputs: inputs, Weapons: &s.Combat.Weapons, Scores: &s.Scores, Level: w.Level, SceneType: w.SceneType, Flash: s.Presentation.Palette.FlashTimer}
	hits, err := s.Enemies.Tick(&context, &collisions)
	if err != nil {
		return nil, err
	}
	if hits.AudioReset {
		commands = append(commands, WorldSoundCommand{Kind: "reset"})
	}
	addSounds(hits.Sounds)
	s.GenerationBias = context.SpawnVariantSeed
	s.Presentation.Palette.FlashTimer = hits.Flash
	if exitOnly {
		if w.Level != 7 {
			if err = s.cyclePalette(resources); err != nil {
				return nil, err
			}
		}
		return commands, nil
	}
	if err = s.Enemies.SpawnScreen(EnemySpawnContext{Level: w.Level, SceneType: w.SceneType, Screen: w.Screen, Scroll: w.ScreenScroll, Vertical: w.ScrollType != 0, ScreenCleared: &s.Combat.ScreenCleared}); err != nil {
		return nil, err
	}
	// A97B selects the current stream even when AA11's room has already
	// been consumed. This is the final source carried into a quiet Base frame.
	if w.Level == 1 || w.Level == 3 {
		if int(w.Screen) >= len(screenStreamIdentity[w.Level]) {
			return nil, errors.New("missing Base screen stream identity")
		}
		s.ThrowWork.record(screenStreamIdentity[w.Level][w.Screen])
	}
	spawn, err := s.Enemies.SpawnSoldiers(context)
	if err != nil {
		return nil, err
	}
	s.GenerationBias = spawn.NextVariantSeed
	if err = s.animate(resources); err != nil {
		return nil, err
	}
	return commands, nil
}

func (s *ActiveWorld) animate(resources *LevelResources) error {
	w := &s.Combat.World
	if err := s.Animation.Step(CHRAnimationContext{Level: w.Level, SceneType: w.SceneType, Alternate: s.Terrain.AlternateGraphicsScreen, Random: s.Random.Value, Screen: w.Screen, Counter: w.Frame}, &s.Combat.ScreenCleared, &s.Presentation.Patterns.Desired); err != nil {
		return err
	}
	return s.cyclePalette(resources)
}

func (s *ActiveWorld) cyclePalette(resources *LevelResources) error {
	w := &s.Combat.World
	palette, err := s.Presentation.Palette.Step(PaletteCycleContext{GameRoutine: w.GameRoutine, Counter: w.Frame, Level: w.Level, SceneType: w.SceneType, Alternate: s.Terrain.AlternateGraphicsScreen, Header: &s.Header, Resources: resources, Queue: &s.Presentation.Queue})
	if err != nil {
		return err
	}
	s.applyPaletteBias(palette)
	return nil
}

func (s *ActiveWorld) compose() error {
	w := &s.Combat.World
	// The cartridge's actor coordinates are also its logical sprite coordinates.
	// Enemy platform contact can update them after the player/projectile phase.
	for i, p := range s.Combat.Players {
		s.Combat.Display[i] = LogicalSprite{Code: p.DisplaySprite, X: p.X, Y: p.y(), Attributes: p.DisplayAttribute}
	}
	composition := SpriteCompositionInput{Ending: w.GameRoutine == 6 && w.Level == 8, Flicker: s.Flicker, HUD: s.HUD, TwoPlayers: w.PlayerMode != 0, Level: w.Level, FrameScroll: w.FrameScroll, SnowControl: s.Snow.Visibility}
	copy(composition.Objects[:10], s.Combat.Display[:])
	for i, e := range s.Enemies.Slots {
		composition.Objects[i+10] = LogicalSprite{Code: e.Sprite, X: e.X, Y: e.Y, Attributes: e.SpriteAttributes}
	}
	for i, p := range s.Combat.Players {
		composition.Lives[i] = p.Lives
		composition.GameOver[i] = p.GameOver
	}
	composed, err := s.Presentation.Sprites.Compose(composition)
	if err != nil {
		return err
	}
	if composed.GenerationBiasChanged {
		s.GenerationBias = composed.GenerationBiasValue
	}
	return s.Presentation.Seal()
}

func (s *ActiveWorld) applyPaletteBias(result PaletteResult) {
	if result.GenerationBiasChanged {
		s.GenerationBias = result.GenerationBiasValue
	}
}
