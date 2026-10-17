package engine

import "fmt"

// WorldContactState retains search metadata; links and owners have a single
// authoritative home in Actions rather than a second contact-state copy.
type WorldContactState struct {
	Rotation           uint8
	Marks              [7]uint8
	Candidates         [7]int8
	StartingBallAction BallAction
	Parity             uint8
}

type WorldPreparationState struct {
	Actions                                WorldActionState
	TeamAI                                 TeamAIState
	Contacts                               WorldContactState
	PreviousHolder                         int8
	Zones                                  [4]CourtZone
	ScoreDifference                        uint8
	Scores                                 [2]uint16
	ScoreSoundDelay                        uint8
	PreviousBallAction, ObservedBallAction BallAction
	SelectedBoard, DisplayFlags            uint8
	CourtScoreFlags                        [2]uint8
	CourtOutcomeFlags                      uint8 `json:",omitempty"` // original06DF slot outcome.
}

type WorldPreparationContext struct {
	PhysicalControllers         *[4]ControllerFrame
	Actions                     WorldActionContext
	Kinds                       [7]uint8
	Suppressed                  [7]bool
	AttackGrades                [4]uint8
	HumanTactics, HumanCommands [2]uint8
	HumanStatus                 [2]int8
	BackboardStrength           uint8
}

type WorldPreparationStep struct {
	State    WorldPreparationState
	SoundIDs []uint8
}

func preparationObjects(s WorldActionState, c WorldPreparationContext) ([7]ContactObject, error) {
	var result [7]ContactObject
	positions := worldLoopPositions(s)
	geometry := ContactGeometryContext{PhysicalControllers: c.PhysicalControllers, Positions: &positions}
	for i := range result {
		var action, variant, cursor uint8
		o := ContactObject{Suppressed: c.Suppressed[i]}
		switch {
		case i < 4:
			a := s.Actors[i]
			p := a.Player
			action, variant, cursor = uint8(p.Action), a.AttackVariant, p.Animation.Cursor
			o.Position, o.Entered, o.Mirrored, o.Owner = p.Body.Position, p.Entered, p.Pose.Mirrored, int8(c.Actions.Controls[i])
		case i < 6:
			a := s.Auxiliary[i-4]
			action, cursor = a.Action, a.Animation.Cursor
			o.Position, o.Entered, o.Mirrored, o.Owner = a.Body.Position, a.Entered, a.Pose.Mirrored, a.Owner
		default:
			b := s.Ball
			action, cursor = uint8(b.Action), b.Animation.Cursor
			o.Position, o.Entered, o.Mirrored, o.Owner = b.Body.Position, b.Entered, b.Pose.Mirrored, s.Holder
		}
		// The original search rejects an unentered player as both source and
		// target before reading its geometry. Its cursor still belongs to the
		// previous action and may not index a frame of the new action at all.
		if i < 4 && !o.Entered {
			o.Action = action
			result[i] = o
			continue
		}
		profile, err := OriginalContactProfileWithContext(ContactProfileKey{Kind: c.Kinds[i], Action: action, Variant: variant, Frame: cursor}, geometry)
		if err != nil {
			return result, fmt.Errorf("contact object %d: %w", i, err)
		}
		o.Action, o.Profile = action, profile
		result[i] = o
	}
	return result, nil
}

func prepareOriginalRiderLinks(s *WorldPreparationState, c WorldPreparationContext) error {
	r := RiderLinkState{Links: s.Actions.actionLinks()}
	for i := range r.Objects {
		o := RiderLinkObject{Kind: c.Kinds[i]}
		if i < 4 {
			p := s.Actions.Actors[i].Player
			o.Position, o.HeightVelocity, o.Action, o.Entered, o.Airborne = p.Body.Position, p.Body.Velocity.Height, p.Action, p.Entered, p.Airborne
			o.Control, o.AIMode = c.Actions.Controls[i], uint8(s.TeamAI.Players[i].Mode)
		} else {
			a := s.Actions.Auxiliary[i-4]
			o.Position, o.HeightVelocity, o.Action, o.Entered, o.Airborne = a.Body.Position, a.Body.Velocity.Height, Action(a.Action), a.Entered, a.Airborne
		}
		r.Objects[i] = o
	}
	got, err := StepOriginalRiderLinks(r)
	if err != nil {
		return err
	}
	s.Actions.setActionLinks(got.Links)
	for i := range s.Actions.Actors {
		s.Actions.Actors[i].Player.Action, s.Actions.Actors[i].Player.Entered = got.Objects[i].Action, got.Objects[i].Entered
	}
	return nil
}

func prepareOriginalInteractions(s *WorldPreparationState, c WorldPreparationContext) ([]uint8, error) {
	a := &s.Actions
	state := InteractionState{Ball: a.Ball, Holder: a.Holder, LastTouch: a.LastTouch, SelectedHoop: a.SelectedHoop, Auxiliary: a.Auxiliary, SpecialHolder: a.SpecialHolder}
	for i, p := range a.Actors {
		state.Players[i] = CombatActor{Player: p.Player, AttackVariant: p.AttackVariant, AttackGrade: c.AttackGrades[i], Endurance: p.Endurance}
	}
	ctx := InteractionContext{CourtID: c.Actions.CourtID, Candidates: s.Contacts.Candidates, Controls: c.Actions.Controls, Attachments: a.PlayerLinks,
		AuxiliaryKinds:  [2]uint8{c.Kinds[4], c.Kinds[5]},
		ExcludedCatcher: a.ExcludedCatcher, PreviousHolder: s.PreviousHolder, AuxiliaryOwners: [2]int8{a.Auxiliary[0].Owner, a.Auxiliary[1].Owner}, HoopOwners: a.hoopOwners()}
	got, err := StepOriginalInteractions(state, ctx)
	if err != nil {
		return nil, err
	}
	a.Ball, a.Holder, a.LastTouch, a.SelectedHoop = got.State.Ball, got.State.Holder, got.State.LastTouch, got.State.SelectedHoop
	a.Auxiliary, a.SpecialHolder = got.State.Auxiliary, got.State.SpecialHolder
	if got.ChangeCaptureCounter {
		a.CaptureCounter = got.CaptureCounter
	}
	for i, p := range got.State.Players {
		a.Actors[i].Player, a.Actors[i].AttackVariant, a.Actors[i].Endurance = p.Player, p.AttackVariant, p.Endurance
	}
	var sounds []uint8
	for _, sound := range got.Sounds {
		sounds = append(sounds, sound.ID)
	}
	return sounds, nil
}

func prepareOriginalTeams(s *WorldPreparationState, c WorldPreparationContext) error {
	a := &s.Actions
	ctx := TeamAIContext{Teams: c.Actions.Teams, HumanTactics: c.HumanTactics, HumanCommands: c.HumanCommands, HumanStatus: c.HumanStatus,
		Holder: uint8(a.Holder), PreviousHolder: uint8(s.PreviousHolder), ScoreDifference: s.ScoreDifference, CourtID: c.Actions.CourtID, Attachments: a.PlayerLinks}
	for team := range ctx.HasHuman {
		ctx.HasHuman[team] = c.Actions.Controls[team*2]&c.Actions.Controls[team*2+1]&128 == 0
	}
	for i, p := range a.Actors {
		ctx.Objects[i] = p.Player.Body.Position
		ctx.Airborne[i] = p.Player.Airborne
	}
	for i, p := range a.Auxiliary {
		ctx.Objects[i+4] = p.Body.Position
	}
	ctx.Objects[6] = a.Ball.Body.Position
	for i, h := range a.Hoops {
		ctx.Hoops[i] = AIHoopView{Position: h.Position, Action: h.Action, Owner: int8(h.Owner)}
		ctx.HoopEntered[i] = h.Entered
	}
	got, err := StepOriginalTeamAI(s.TeamAI, ctx)
	if err != nil {
		return err
	}
	s.TeamAI = got.State
	// Original $8026 commits only after the team phase consumed the old holder.
	s.PreviousHolder = a.Holder
	return nil
}

func prepareOriginalBoards(s *WorldPreparationState, c WorldPreparationContext) ([]uint8, error) {
	a := &s.Actions
	state := BoardContactState{Auxiliary: a.Auxiliary, Ball: a.Ball, Holder: a.Holder, Random: a.Random, Scores: s.Scores,
		ScoreSoundDelay: s.ScoreSoundDelay, DetachedCount: a.DetachedCount, PreviousBallAction: s.PreviousBallAction, ObservedBallAction: s.ObservedBallAction,
		MountFlags: a.MountFlags, SelectedBoard: s.SelectedBoard, DisplayFlags: s.DisplayFlags, CourtScoreFlags: s.CourtScoreFlags,
		CourtToggle: a.Actors[1].Player.NewYorkPassage}
	for i, p := range a.Actors {
		state.Players[i] = p.Player
	}
	for i, h := range a.Hoops {
		state.Hoops[i] = h.component()
	}
	if a.LastShooter < 0 || a.LastShooter >= 4 {
		return nil, fmt.Errorf("unsupported rim shooter %d", a.LastShooter)
	}
	selected := -1
	if a.SelectedHoop >= 7 && a.SelectedHoop < 13 {
		selected = int(a.SelectedHoop) - 7
	}
	profile := c.Actions.Profiles[a.LastShooter]
	ctx := BoardContactContext{CourtID: c.Actions.CourtID, BackboardStrength: c.BackboardStrength, AuxiliaryKinds: [2]uint8{c.Kinds[4], c.Kinds[5]},
		Rim: RimContext{SelectedHoop: selected, ShotOriginX: a.ShotOriginX, ShooterAction: a.Actors[a.LastShooter].Player.Action, ShotSkill: profile.ShotSkill, ScoringBonus: profile.ScoringBonus}}
	got, err := StepBoardContacts(state, ctx)
	if err != nil {
		return nil, err
	}
	for i, p := range got.State.Players {
		a.Actors[i].Player = p
	}
	// $06D8 is the same byte as actor 1's NY passage state, never a separate
	// stored court toggle in this world representation.
	a.Actors[1].Player.NewYorkPassage = got.State.CourtToggle
	a.Auxiliary, a.Ball, a.Holder, a.Random, a.DetachedCount, a.MountFlags = got.State.Auxiliary, got.State.Ball, got.State.Holder, got.State.Random, got.State.DetachedCount, got.State.MountFlags
	for i, h := range got.State.Hoops {
		// This phase does not write hoop owner bytes. The boolean component
		// view is insufficient to reconstruct an opaque mount identity.
		owner := a.Hoops[i].Owner
		a.Hoops[i].applyComponent(h, uint8(i))
		a.Hoops[i].Owner = owner
	}
	s.Scores, s.ScoreSoundDelay = got.State.Scores, got.State.ScoreSoundDelay
	s.PreviousBallAction, s.ObservedBallAction = got.State.PreviousBallAction, got.State.ObservedBallAction
	s.SelectedBoard, s.DisplayFlags, s.CourtScoreFlags = got.State.SelectedBoard, got.State.DisplayFlags, got.State.CourtScoreFlags
	return got.SoundIDs, nil
}

// StepWorldPreparation composes fixed $D317..D326 using the recovered rule
// components. It owns neither camera/HUD ticks nor input/AI/action execution.
// Any unsupported subphase rolls back the entire state and ordered sound list.
func StepWorldPreparation(original WorldPreparationState, c WorldPreparationContext) (WorldPreparationStep, error) {
	initial := WorldPreparationStep{State: original}
	if c.Actions.CourtID >= 8 {
		return initial, fmt.Errorf("world preparation court %d is not reconstructed", c.Actions.CourtID)
	}
	result := initial
	s := &result.State
	contacts := BeginOriginalContacts(ContactPhaseState{Rotation: s.Contacts.Rotation, Marks: s.Contacts.Marks, Candidates: s.Contacts.Candidates,
		Links: s.Actions.actionLinks(), StartingBallAction: s.Contacts.StartingBallAction, Parity: s.Contacts.Parity}, s.Actions.Ball.Action)
	s.Actions.setActionLinks(contacts.Links)
	s.Contacts = WorldContactState{Rotation: contacts.Rotation, Marks: contacts.Marks, Candidates: contacts.Candidates, StartingBallAction: contacts.StartingBallAction, Parity: contacts.Parity}
	objects, err := preparationObjects(s.Actions, c)
	if err != nil {
		return initial, err
	}
	search := FindObjectContacts(objects, s.Contacts.Rotation)
	s.Contacts.Rotation, s.Contacts.Candidates = search.Rotation, search.Candidates
	if err = prepareOriginalRiderLinks(s, c); err != nil {
		return initial, fmt.Errorf("rider links: %w", err)
	}
	sounds, err := prepareOriginalInteractions(s, c)
	if err != nil {
		return initial, fmt.Errorf("interactions: %w", err)
	}
	result.SoundIDs = append(result.SoundIDs, sounds...)
	for i, p := range s.Actions.Actors {
		s.Zones[i], err = CourtZoneForActor(p.Player.Body.Position, uint8(i))
		if err != nil {
			return initial, fmt.Errorf("zone %d: %w", i, err)
		}
	}
	s.ScoreDifference = OriginalScoreDifference(s.Scores)
	if err = prepareOriginalTeams(s, c); err != nil {
		return initial, fmt.Errorf("team strategy: %w", err)
	}
	sounds, err = prepareOriginalBoards(s, c)
	if err != nil {
		return initial, fmt.Errorf("board contacts: %w", err)
	}
	result.SoundIDs = append(result.SoundIDs, sounds...)
	return result, nil
}
