package contra

import "errors"

// FrameEntropy retains only adaptation metadata. Original random work remains
// owned by World.Active.Random, including every enemy mutation. No duplicate
// accumulator is serialized in the platform session.
type FrameEntropy struct {
	Version   uint8  `json:"version"`
	Seed      uint64 `json:"seed,string"`
	Entropy   uint64 `json:"entropy,string"`
	Frames    uint64 `json:"frames,string"`
	IdleCarry bool   `json:"idleCarry"`
}

func (f FrameEntropy) adapter(work RandomState) AdaptedRandom {
	return AdaptedRandom{Version: f.Version, Seed: f.Seed, Entropy: f.Entropy, Frames: f.Frames, IdleCarry: f.IdleCarry, Work: work}
}
func (f *FrameEntropy) advance(frame uint64, counter uint8, work *RandomState) error {
	a := f.adapter(*work)
	if err := a.AdvanceFrame(frame, counter); err != nil {
		return err
	}
	f.Entropy, f.Frames, f.IdleCarry = a.Entropy, a.Frames, a.IdleCarry
	*work = a.Work
	return nil
}

// Session is the independent platform runtime: it starts without a ROM/RAM
// snapshot and binds authoritative frame IDs, audio and the permitted entropy
// adaptation. Input mapping and network authorization remain outside the engine.
type Session struct {
	Version           uint8
	World             World
	Random            FrameEntropy
	Audio             AudioState
	Controls          Controls
	Finalized         bool
	FinalFlushTicks   uint8
	PresentationTicks uint64 `json:"presentationTicks,string"`
	Revision          uint64 `json:"revision,string"`
	ResumeRequested   bool
	// PresentedLives belongs to the same display boundary as AppliedOAM. Rule
	// work may produce the next reserves before that composition is submitted.
	PresentedLives [2]uint8
	Review         *BattleReview `json:"review,omitempty"`
}
type SessionFrame struct {
	Duplicate    bool
	Finalized    bool
	Presentation PresentedFrame
	Audio        AudioUpdate
}

func NewSession(seed uint64, players int, epoch string) (*Session, error) {
	if players < 1 || players > 2 || epoch == "" {
		return nil, errors.New("invalid Contra session configuration")
	}
	sprites, err := NewSpriteCompositionState()
	if err != nil {
		return nil, err
	}
	s := &Session{Version: 1, Audio: NewAudioState(epoch), Random: FrameEntropy{Version: FrameRandomVersion, Seed: seed, Entropy: seed}}
	s.World.Campaign = NewCampaign()
	a := &s.World.Active
	a.Combat.World = OutdoorWorld{GameRoutine: 5, PlayerMode: uint8(players - 1), Scroll: CollisionScroll{NameTable: 0xb0}}
	a.Combat.Players[0].Lives = 2
	if players == 2 {
		a.Combat.Players[1].Lives = 2
	} else {
		a.Combat.Players[1].GameOver = 1
	}
	a.Scores = NewArcadeScores()
	a.Presentation = WorldPresentation{Sprites: sprites, AppliedOAM: sprites.OAM, Display: DisplayState{Mask: 0x1e}, Queue: GraphicsQueue{Mode: GraphicsText}}
	s.commitPublicLives()
	return s, nil
}

func (s *Session) commitPublicLives() {
	for i, player := range s.World.Active.Combat.Players {
		s.PresentedLives[i] = player.Lives
	}
}

func (s *Session) Step(frame uint64, inputs [2]PlayerInput, resources *LevelResources) (SessionFrame, error) {
	if resources == nil {
		return SessionFrame{}, errors.New("missing session level resources")
	}
	if err := s.Validate(); err != nil {
		return SessionFrame{}, err
	}
	if frame == s.Random.Frames && frame != 0 {
		return SessionFrame{Duplicate: true, Finalized: s.Finalized, Presentation: s.World.Active.Presentation.Snapshot(), Audio: AudioUpdate{View: s.Audio.View()}}, nil
	}
	if s.World.Campaign.Outcome != CampaignOngoing {
		return SessionFrame{}, errors.New("finished campaign cannot run another gameplay frame")
	}
	if s.Random.Frames == ^uint64(0) || frame != s.Random.Frames+1 {
		return SessionFrame{}, errors.New("session requires next authoritative frame")
	}
	next := *s
	if next.Revision == ^uint64(0) {
		return SessionFrame{}, errors.New("session revision exhausted")
	}
	retainReview := s.battleReviewCandidate()
	reviewWorld := s.World.Active.Combat.World
	reviewExit := s.World.Campaign.EndSequence
	next.Audio = s.Audio.Clone()
	next.World.Active.Presentation.Queue = s.World.Active.Presentation.Queue.Clone()
	if _, err := next.World.Active.Presentation.CommitWithScroll(next.World.Active.Combat.World.Scroll); err != nil {
		return SessionFrame{}, err
	}
	next.commitPublicLives()
	next.Audio.Context.EndPlayersAlive = next.World.Campaign.AliveAtExit
	audio := next.Audio.Advance(next.World.Campaign.Paused)
	if len(audio.UnsupportedIDs) != 0 {
		return SessionFrame{}, errors.New("unresolved session audio control")
	}
	if err := next.Random.advance(frame, next.World.Active.Combat.World.Frame, &next.World.Active.Random); err != nil {
		return SessionFrame{}, err
	}
	if next.ResumeRequested {
		inputs[0].Held |= ButtonStart
		inputs[0].Pressed |= ButtonStart
		next.ResumeRequested = false
	}
	if _, err := next.World.AdvanceWithAudio(frame, inputs, resources, &next.Audio); err != nil {
		return SessionFrame{}, err
	}
	if retainReview {
		next.retainBattleReview(frame, next.Revision+1, reviewWorld.Level, reviewWorld.LevelRoutine, reviewExit)
	}
	next.Revision++
	*s = next
	return SessionFrame{Presentation: s.World.Active.Presentation.Snapshot(), Audio: audio}, nil
}

// StepPads is the platform input boundary. It derives newly pressed edges from
// authenticated held pads in the same transaction as physics, audio and RNG.
// Step remains available for already-sampled deterministic rule replays.
func (s *Session) StepPads(frame uint64, pads [2]Buttons, resources *LevelResources) (SessionFrame, error) {
	if s == nil {
		return SessionFrame{}, errors.New("missing Contra session")
	}
	if frame != 0 && frame == s.Random.Frames {
		return s.Step(frame, [2]PlayerInput{}, resources)
	}
	next := *s
	next.Controls.Sample(pads, next.World.Active.Combat.World.PlayerMode != 0)
	var inputs [2]PlayerInput
	for i := range inputs {
		inputs[i] = PlayerInput{Held: next.Controls.Held[i], Pressed: next.Controls.Pressed[i]}
	}
	result, err := next.Step(frame, inputs, resources)
	if err != nil {
		return SessionFrame{}, err
	}
	*s = next
	return result, nil
}

// projectEnemySound is pure: earlier world requests are in audio.Pending, while
// the prefix describes only this enemy phase. The final phase result is queued
// once by World, after all rule work succeeds.
func projectEnemySound(audio *AudioState, code uint8, prefix EnemyAudioPrefix) (bool, error) {
	if audio == nil {
		return false, errors.New("missing audio control projection")
	}
	copy := audio.Clone()
	if prefix.Reset {
		copy.Queue(AudioCommand{Kind: "reset"})
	}
	for _, id := range prefix.Sounds {
		copy.Queue(AudioCommand{Kind: "sound", ID: id})
	}
	return copy.QueueSound(code)
}
