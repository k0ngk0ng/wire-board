package contra

import "errors"

// PrepareCampaignGameOver is CD71's non-demo presentation after Campaign has
// produced its loss/continue decision. It does not decrement continues or emit
// 6E again: CampaignPlan is the sole owner of that sound and progression.
func PrepareCampaignGameOver(w *ActiveWorld, offerContinue bool) (CampaignSceneResult, error) {
	if w == nil || w.Combat.World.GameRoutine != 5 || w.Combat.World.Level >= 8 || w.Combat.World.PlayerMode > 1 || w.Presentation.Queue.Mode != GraphicsText || w.Presentation.Terminated || (offerContinue && w.Combat.World.LevelRoutine != 6) || (!offerContinue && w.Combat.World.LevelRoutine != 7) {
		return CampaignSceneResult{}, errors.New("invalid game-over presentation context")
	}
	pages, ok := campaignScenePresets[10]
	if !ok {
		return CampaignSceneResult{}, errors.New("missing game-over pattern preset")
	}
	next := *w
	next.Snow.Visibility = 0
	for i := range next.Presentation.Sprites.Snow {
		next.Presentation.Sprites.Snow[i].Flags = 0
	}
	out := campaignClearBackground(&next)
	next.Presentation.Patterns.Desired = pages
	draw := func(id uint8) error {
		packets, e := sceneTextPackets(id, false, false)
		if e != nil {
			return e
		}
		for _, p := range packets {
			if e = next.Presentation.Queue.AppendText(p); e != nil {
				return e
			}
		}
		return nil
	}
	for _, id := range []uint8{6, 13} {
		if e := draw(id); e != nil {
			return CampaignSceneResult{}, e
		}
	}
	if e := campaignResetScores(&next); e != nil {
		return CampaignSceneResult{}, e
	}
	if offerContinue {
		if e := draw(14); e != nil {
			return CampaignSceneResult{}, e
		}
	}
	*w = next
	return out, nil
}

// StepCampaignContinueDisplay consumes the already updated Choice and routine.
// Start has precedence over Select. Resource restoration and audio are handled
// by the preceding CampaignPlan; this step only writes original cursor fields.
func StepCampaignContinueDisplay(w *ActiveWorld, c *Campaign, pressed Buttons) (CampaignSceneResult, error) {
	if w == nil || c == nil || c.Choice > 1 || w.Combat.World.PlayerMode > 1 {
		return CampaignSceneResult{}, errors.New("invalid continue cursor context")
	}
	world := &w.Combat.World
	if pressed&ButtonStart != 0 {
		if c.Choice != 0 {
			if c.Outcome != CampaignFailed || world.GameRoutine != 0 {
				return CampaignSceneResult{}, errors.New("end choice has not been applied")
			}
			return CampaignSceneResult{}, nil
		}
		if world.GameRoutine != 5 || world.LevelRoutine != 0 {
			return CampaignSceneResult{}, errors.New("continue resources have not been applied")
		}
		o := sceneObject(w, 0)
		o.X, o.Y, o.Code = 0, 0, 0
		setSceneObject(w, 0, o)
		return CampaignSceneResult{}, nil
	}
	if world.GameRoutine != 5 || world.LevelRoutine != 6 {
		return CampaignSceneResult{}, errors.New("continue menu is not active")
	}
	o := sceneObject(w, 0)
	o.X, o.Code = 0x52, 0xaa
	// Authenticated cartridge C0A4, also used by the original title cursor.
	o.Y = [2]uint8{0xa2, 0xb2}[c.Choice]
	setSceneObject(w, 0, o)
	return CampaignSceneResult{}, nil
}

// CA2B/CA62 format scores in hundred-point units into the existing templates.
// No arcade score is changed by these background text writes.
func campaignResetScores(w *ActiveWorld) error {
	for i, score := range []uint16{w.Scores.HighScore, w.Scores.Hundreds[0], w.Scores.Hundreds[1]} {
		if i == 2 && w.Combat.World.PlayerMode == 0 {
			break
		}
		packets, e := sceneTextPackets(uint8(9+i), false, false)
		if e != nil {
			return e
		}
		if len(packets) != 1 || len(packets[0].Values) < 7 {
			return errors.New("invalid score text template")
		}
		values := packets[0].Values
		at := len(values) - 3
		for remaining := score; ; {
			values[at] = uint8(remaining%10) + 1
			remaining /= 10
			if remaining == 0 {
				break
			}
			at--
		}
		if score == 0 {
			values[len(values)-3] = 0
			values[len(values)-1] = 1
		} else {
			values[len(values)-2], values[len(values)-1] = 1, 1
		}
		if e = w.Presentation.Queue.AppendText(packets[0]); e != nil {
			return e
		}
	}
	return nil
}
