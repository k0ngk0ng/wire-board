package engine

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
)

type OpeningWorldOptions struct {
	RecoverUnsupportedBall bool
	// Nil retains the original empty inventory used by fresh single matches.
	Inventory *[16]campaign.Item
	// The video/scene driver owns the original outer-loop counter. Fresh
	// platforms may start it at zero; a continued scene passes its current byte.
	LogicFrame   uint8
	HumanTactics [2]uint8
}

// WorldFromOriginalOpening bridges a completed semantic opening to the real
// world engine. All remaining zero fields originate in the original cold RAM
// clear, not a recorded expected snapshot. It is not a saved-game restore API.
func WorldFromOriginalOpening(s OpeningState, options OpeningWorldOptions) (WorldLoopState, WorldLoopContext, error) {
	var world WorldLoopState
	var ctx WorldLoopContext
	t := s.Transition
	if t.Scene != 1 || t.Match.Phase != MatchPlaying || !t.Match.Entered || t.Court >= 8 {
		return world, ctx, fmt.Errorf("world handoff requires a completed supported court opening")
	}
	for _, v := range options.HumanTactics {
		if v >= 6 {
			return world, ctx, fmt.Errorf("invalid original human tactic")
		}
	}
	profiles, err := s.CharacterProfiles()
	if err != nil {
		return world, ctx, err
	}
	ctx = WorldLoopContext{Actions: WorldActionContext{CourtID: t.Court, Profiles: profiles, Controls: t.PlayerControls, Characters: t.Characters, Teams: t.Teams, Equipment: uint8(s.Equipment[0][0]), EnhancementEnabled: t.Mode&128 == 0}, ObjectKinds: t.Kinds, HumanTactics: options.HumanTactics, Difficulty: t.Match.Period, LayoutID: t.Scene, TrackedObject: t.TrackedObject}
	ctx.Actions.RecoverUnsupportedBall = options.RecoverUnsupportedBall
	// The pursuit carry quirk reads original $055C and $00E6. C9B6/F8BD
	// clears the former and FA8C clears the latter before a cold match.
	// Explicit zero differs from nil (unknown); no observed fixture is loaded.
	ctx.PursuitAlias74 = &PursuitAlias74{}
	// C9B6/F8BD and FA8C also clear the reserved 055B/E5/E7 sources.
	// FA8C clears F4. Bank3 A0D3 fills the initial D3..E2 item list
	// with FF; DA (slot7) is read only if the pursuit alias reaches X.
	zeroDepth, emptyListItem := uint8(0), uint8(255)
	if options.Inventory != nil {
		emptyListItem = uint8(options.Inventory[7])
	}
	ctx.PursuitReserved = &PursuitReservedBytes{Depth88: &zeroDepth, XHigh88: &emptyListItem}
	copy(ctx.HoopLinks[:], s.Links[7:])
	for i, g := range s.Grades {
		ctx.AttackGrades[i] = g.Attack
		ctx.Actions.RecoveryGrades[i] = g.Recovery
		ctx.Actions.ShotEquipment[i] = uint8(s.Equipment[t.Characters[i]][0])
		ctx.Actions.Enhancements[i] = uint8(s.Equipment[t.Characters[i]][1])
		ctx.JumpEnhancements[i] = uint8(s.Equipment[t.Characters[i]][2])
		// Bank4 91BD uses the last shooter's object index (0/1 only), then
		// its character's shoe slot. This consumer has no Mode/team gate.
		ctx.Actions.Profiles[i].ScoringBonus = i < 2 && s.Equipment[t.Characters[i]][2] == 27
	}
	// The original generic arc lookup's seven pre-hoop entries alias the last
	// seven character equipment bytes. Decode the selections, not a fixture.
	for i := range ctx.Actions.OtherObjectArcFlags {
		flat := i + 5
		ctx.Actions.OtherObjectArcFlags[i] = uint8(s.Equipment[flat/3][flat%3])
	}
	p := &world.Preparation
	a := &p.Actions
	a.Ball = openingBall(s)
	a.Holder = int8(t.Owners[2])
	a.SpecialHolder = t.SpecialHolder
	a.ExcludedCatcher = s.ExcludedCatcher
	a.Random = t.Random
	a.MountFlags = s.MountFlags
	a.DetachedCount = s.DetachedCount
	a.LastShooter = int8(t.LastShooter)
	for i := 0; i < 4; i++ {
		o := t.Objects[i]
		a.Actors[i] = WorldActionActor{Player: Player{Body: o.Body, Action: Action(o.Action), Entered: o.Entered, Request: o.Request, Pose: o.Pose, Animation: o.Animation, Airborne: o.Airborne, FacingLeft: o.FacingLeft, Heading: Heading(0), NewYorkPassage: t.Passages[i]}, ShotCycle: t.ShotCycle[i], Endurance: t.Endurance[i]}
		a.PlayerLinks[i] = s.Links[i]
		p.TeamAI.Players[i] = AIControlState{Mode: AIMode(t.AIModes[i] & 127), Entered: t.AIModes[i]&128 != 0, Plan: AIMode(t.AIPlans[i])}
		world.Input.Memory[i].LastAttackTap = Buttons(s.InputDelays[i])
		world.Input.Memory[i].LastTap = Buttons(s.InputDelays[i+4])
		world.Input.Memory[i].PendingButtons = Buttons(s.InputDelays[i+8])
	}
	for i := 0; i < 2; i++ {
		o := t.Objects[i+4]
		a.Auxiliary[i] = AuxiliaryState{Body: o.Body, Action: o.Action, Entered: o.Entered, Pose: o.Pose, Animation: o.Animation, Airborne: o.Airborne, FacingLeft: o.FacingLeft, Owner: int8(t.Owners[i]), Attachment: s.Links[i+4], Request: o.Request, NewYorkPassage: t.Passages[i+4], ObjectLink: t.AuxiliaryLinks[i]}
		if t.Court == 4 {
			a.Auxiliary[i].Kind = t.Kinds[i+4]
		}
		p.TeamAI.Teams[i] = TeamStrategyState{Mode: t.TeamAI[i] & 127, Entered: t.TeamAI[i]&128 != 0}
	}
	a.BallLink = s.Links[6]
	for i := 0; i < 6; i++ {
		o := t.Objects[i+7]
		a.Hoops[i] = WorldActionHoop{Position: o.Body.Position, Velocity: o.Body.Velocity, Gravity: o.Body.Gravity, Action: HoopAction(o.Action), Entered: o.Entered, Owner: t.Owners[i+3], Durability: t.HoopDurability[i], Pose: o.Pose, FacingLeft: o.FacingLeft, Animation: o.Animation, Airborne: o.Airborne, Request: HoopRequest{Action: HoopAction(o.Request.Action), Pending: o.Request.Pending}}
	}
	p.Scores = t.Scores
	p.CourtScoreFlags = [2]uint8{t.Passages[6], t.Passages[7]}
	p.CourtOutcomeFlags = t.Passages[8]
	p.ScoreSoundDelay = s.ScoreSoundDelay
	p.PreviousBallAction = BallAction(uint8(t.ResultControllers[0]) & 127)
	p.ObservedBallAction = BallAction(uint8(t.ResultControllers[1]) & 127)
	p.DisplayFlags = t.DisplayFlags
	world.Camera = CameraState{DisplayedX: t.CameraScroll[0], X: t.CameraScroll[2], Y: t.CameraScroll[3], TileColumn: 48, Shake: t.EndCounter}
	if t.Court == 5 {
		world.LaunchColumn = &CourtColumnCommit{Column: 47, Phase: 0}
	}
	// BE2B finishes its court-column loop at $30. The next main-loop boundary
	// clears only draw slots 0..11; the thirteenth cold slot remains untouched.
	world.Projection = resetOriginalWorldDrawList(s.Projection)
	world.LogicFrame = options.LogicFrame
	world.DisplayedCameraY = t.CameraScroll[1]
	return world, ctx, nil
}
