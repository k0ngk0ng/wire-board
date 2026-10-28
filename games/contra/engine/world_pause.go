package contra

import "errors"

// AdvanceWithPause adds the original CE94 presentation prefix to an ordinary
// battle frame. The pause bit has exactly one owner, Campaign. This primitive
// does not run campaign transitions or the platform's monotonic tick scheduler.
// The caller commits pending presentation/audio before calling it even paused.
func (s *ActiveWorld) AdvanceWithPause(inputs [2]PlayerInput, resources *LevelResources, campaign *Campaign) ([]WorldSoundCommand, error) {
	if s == nil || resources == nil || campaign == nil || campaign.Pending != CampaignNoWork || campaign.Outcome != CampaignOngoing || s.Presentation.Terminated || s.Combat.World.Level != 0 || s.Combat.World.LevelRoutine != 4 || s.Combat.World.GameRoutine != 5 || s.Combat.World.BossDefeated != 0 {
		return nil, errors.New("pause-aware frame requires a normal jungle battle")
	}
	next, progress := *s, *campaign
	next.Presentation.Queue = s.Presentation.Queue.Clone()
	next.Enemies.PendingSounds = append([]uint8(nil), s.Enemies.PendingSounds...)
	next.Combat.World.Frame++
	next.Flicker = true
	if progress.Paused {
		next.projectPausedSprites()
		if err := next.animate(resources); err != nil {
			return nil, err
		}
	}
	plan := progress.updatePause(CampaignInput{Pressed: inputs[0].Pressed})
	var commands []WorldSoundCommand
	for _, id := range plan.Sounds {
		commands = append(commands, WorldSoundCommand{Kind: "sound", ID: id})
	}
	if progress.Paused {
		if err := next.compose(); err != nil {
			return nil, err
		}
	} else {
		part, err := next.advanceBattle(inputs, resources)
		if err != nil {
			return nil, err
		}
		commands = append(commands, part...)
	}
	*s, *campaign = next, progress
	return commands, nil
}

func (s *ActiveWorld) projectPausedSprites() {
	frame := s.Combat.World.Frame
	// D2B0 only projects alternating bullet slots; it never moves projectiles.
	for slot := range 8 {
		p := s.Combat.Weapons.Projectiles[slot*2+int(frame&1)]
		s.Combat.Display[slot+2] = LogicalSprite{Code: p.Sprite, X: p.X, Y: p.Y, Attributes: p.Attributes}
	}
	// FA52 -> bank4/5 A4EF republishes both actors without advancing physics.
	for actor := 1; actor >= 0; actor-- {
		p := &s.Combat.Players[actor]
		p.publishSprite(actor, frame)
		s.Combat.Display[actor] = LogicalSprite{Code: p.DisplaySprite, X: p.X, Y: p.y(), Attributes: p.DisplayAttribute}
	}
}
