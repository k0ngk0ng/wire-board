package contra

import "errors"

// CampaignSceneControl changes a unique display owner held by the enclosing
// World. It never stores a parallel palette, pattern, queue or level routine.
type CampaignSceneControl struct {
	Kind  string
	Value uint8
}
type CampaignSceneResult struct {
	ImmediateBackground []BackgroundWrite
	Sounds              []WorldSoundCommand
	Controls            []CampaignSceneControl
}

func (s *CampaignSceneState) StepIntro(c *Campaign, w *ActiveWorld, inputs [2]PlayerInput, demo bool) (CampaignSceneResult, error) {
	if s == nil || c == nil || w == nil || w.Combat.World.GameRoutine != 5 || w.Combat.World.LevelRoutine != 1 || w.Combat.World.Level >= 8 || c.EndSequence > 10 || w.Presentation.Queue.Mode != GraphicsText {
		return CampaignSceneResult{}, errors.New("invalid campaign introduction")
	}
	ns, nc, nw := *s, *c, *w
	nw.Presentation.Queue = w.Presentation.Queue.Clone()
	out, err := ns.stepIntro(&nc, &nw, inputs, demo)
	if err != nil {
		return CampaignSceneResult{}, err
	}
	*s, *c, *w = ns, nc, nw
	return out, nil
}
func (s *CampaignSceneState) stepIntro(c *Campaign, w *ActiveWorld, inputs [2]PlayerInput, demo bool) (CampaignSceneResult, error) {
	var out CampaignSceneResult
	controls := func(kind string, value uint8) { out.Controls = append(out.Controls, CampaignSceneControl{kind, value}) }
	sound := func(id uint8) { out.Sounds = append(out.Sounds, WorldSoundCommand{Kind: "sound", ID: id}) }
	finish := func() {
		controls("graphics-mode", 1)
		controls("display-disable", 0)
		controls("display-busy", 5)
		out.Sounds = append(out.Sounds, WorldSoundCommand{Kind: "reset"})
		w.Combat.World.LevelRoutine++
		c.EndSequence = 0
	}
	split := func() { controls("split-scroll", s.IntroScroll) }
	if demo || inputs[0].Pressed&ButtonStart != 0 {
		finish()
		return out, nil
	}
	level := w.Combat.World.Level
	late := level >= 5
	// The original cycles colors and beacon before dispatching the sequence.
	at := int(w.Combat.World.Frame & 0x1c)
	colors := &w.Presentation.Palette
	colors.Desired[25] = campaignSceneIntroCycles[at]
	colors.Desired[26] = campaignSceneIntroCycles[at+1]
	colors.Desired[21] = campaignSceneIntroCycles[at+2]
	colors.Desired[22] = campaignSceneIntroCycles[at+3]
	colors.RequestedColors = 32
	beacon := sceneObject(w, 11)
	if beacon.Code != 0 {
		beacon.Code = campaignSceneIntroBeacon[(c.DelayLow>>3)&3]
		setSceneObject(w, 11, beacon)
		if c.DelayLow&31 == 31 {
			sound(0x18)
		}
	}
	draw := func(id uint8, blank, shift bool) error {
		packets, err := sceneTextPackets(id, blank, shift)
		if err != nil {
			return err
		}
		for _, p := range packets {
			if err = w.Presentation.Queue.AppendText(p); err != nil {
				return err
			}
		}
		return nil
	}
	switch c.EndSequence {
	case 0:
		copy(colors.Desired[:], campaignSceneIntroPalette[:])
		colors.Desired[0] = 0x20
		c.EndSequence++
	case 1:
		controls("graphics-mode", 0)
		o := sceneObject(w, 0)
		o.Code = 0x4f
		o.X = 0x50
		o.Y = 0x3d
		o.Attributes = 0x27
		setSceneObject(w, 0, o)
		out.ImmediateBackground = sceneBackground(14)
		w.Presentation.Queue.Reset()
		w.Combat.World.Scroll.X = 0
		w.Combat.World.Scroll.Y = 0
		controls("display-disable", 0)
		controls("ppu-control", 0xb0)
		controls("display-busy", 5)
		if err := sceneIntroLabels(w); err != nil {
			return out, err
		}
		o = sceneObject(w, 7)
		o.X = 0x6e
		o.Y = 0xb0
		setSceneObject(w, 7, o)
		for j := 0; j < 3; j++ {
			o = sceneObject(w, 8+j)
			o.Code = uint8(0xf4 + j)
			o.X = [3]uint8{0x80, 0xb0, 0xf0}[j]
			o.Y = [3]uint8{0xaf, 0x6f, 0x6f}[j]
			setSceneObject(w, 8+j, o)
		}
		c.DelayLow = 0x30
		c.EndSequence++
	case 2:
		c.DelayLow--
		if c.DelayLow == 0 {
			sound(0x14)
			c.EndSequence++
		}
	case 3:
		for j := 3; j >= 0; j-- {
			o := sceneObject(w, 7+j)
			old := o.X
			o.X--
			if old == 0 {
				if o.Code != 0 {
					o.Code = 0
				} else {
					o.Code = campaignSceneIntroScrollSprites[j]
				}
			}
			setSceneObject(w, 7+j, o)
		}
		split()
		old := s.IntroScroll
		s.IntroScroll++
		if old >= 0x90 {
			c.EndSequence = 4
			if late {
				c.EndSequence = 6
			}
			c.DelayLow = 0x30
		}
	case 4:
		split()
		c.DelayLow--
		if c.DelayLow == 0 {
			c.EndSequence++
		}
	case 5:
		for j := 3; j >= 0; j-- {
			o := sceneObject(w, 7+j)
			old := o.X
			o.X++
			if old == 255 {
				if o.Code != 0 {
					o.Code = 0
				} else {
					o.Code = campaignSceneIntroScrollSprites[j]
				}
			}
			setSceneObject(w, 7+j, o)
		}
		split()
		s.IntroScroll--
		if s.IntroScroll == 0 {
			c.DelayLow = 0x20
			c.EndSequence++
		}
	case 6:
		split()
		c.DelayLow--
		if c.DelayLow == 0 {
			s.Reveal = 255
			c.EndSequence++
		}
	case 7:
		split()
		s.Reveal++
		if s.Reveal < 10 {
			if err := draw(s.Reveal+25, true, late); err != nil {
				return out, err
			}
			s.Reveal++
			// CAC5 returns carry set after its FE terminator; the second ADC
			// therefore selects one further template than the first call.
			second := s.Reveal + 26
			if late {
				second--
			} // 8511 address adjustment clears carry.
			if err := draw(second, true, late); err != nil {
				return out, err
			}
		} else {
			if err := draw(35, false, late); err != nil {
				return out, err
			}
			s.Reveal = 255
			sound(0x16)
			c.EndSequence++
		}
	case 8:
		split()
		s.Reveal++
		if s.Reveal < 10 {
			if err := draw(s.Reveal+25, false, late); err != nil {
				return out, err
			}
		} else {
			s.Reveal = 5
			c.DelayLow = 8
			c.EndSequence++
		}
	case 9:
		split()
		c.DelayLow--
		if c.DelayLow != 0 {
			break
		}
		if late {
			if err := draw(s.Reveal+31, false, false); err != nil {
				return out, err
			}
			c.DelayLow = 24
			old := s.Reveal
			s.Reveal++
			if old < level {
				break
			}
		}
		o := sceneObject(w, 2)
		o.Code = 0xd6
		o.X = campaignSceneStagePositions[int(level)*4]
		o.Y = campaignSceneStagePositions[int(level)*4+1]
		setSceneObject(w, 2, o)
		o = sceneObject(w, 11)
		o.Code = 1
		o.X = campaignSceneStagePositions[int(level)*4+2]
		o.Y = campaignSceneStagePositions[int(level)*4+3]
		setSceneObject(w, 11, o)
		offset := 0
		if late {
			offset = 3
		}
		for j := 0; j < 3; j++ {
			o = sceneObject(w, 12+j)
			o.Code = uint8(0xda + j)
			o.X = campaignSceneLabelPositions[offset+j]
			o.Y = [3]uint8{0x72, 0x5f, 0x5c}[j]
			setSceneObject(w, 12+j, o)
		}
		c.DelayLow = 0xe0
		c.EndSequence++
	case 10:
		split()
		c.DelayLow--
		if c.DelayLow == 0 {
			finish()
		}
	}
	return out, nil
}
func sceneObject(w *ActiveWorld, i int) LogicalSprite {
	if i < 10 {
		return w.Combat.Display[i]
	}
	p := w.Enemies.Slots[i-10]
	return LogicalSprite{Code: p.Sprite, X: p.X, Y: p.Y, Attributes: p.SpriteAttributes}
}
func setSceneObject(w *ActiveWorld, i int, o LogicalSprite) {
	if i < 10 {
		w.Combat.Display[i] = o
		if i < 2 {
			p := &w.Combat.Players[i]
			p.X = o.X
			p.setY(o.Y)
			p.DisplaySprite = o.Code
			p.DisplayAttribute = o.Attributes
		}
		return
	}
	p := &w.Enemies.Slots[i-10]
	p.Sprite = o.Code
	p.X = o.X
	p.Y = o.Y
	p.SpriteAttributes = o.Attributes
}
func sceneBackground(id uint8) []BackgroundWrite {
	out := make([]BackgroundWrite, len(campaignSceneBackgrounds[id]))
	for i, p := range campaignSceneBackgrounds[id] {
		out[i] = p
		out[i].Values = append([]uint8(nil), p.Values...)
	}
	return out
}
func sceneTextPackets(id uint8, blank, shift bool) ([]GraphicsTextPacket, error) {
	src, ok := campaignSceneTexts[id]
	if !ok {
		return nil, errors.New("missing scene text template")
	}
	out := make([]GraphicsTextPacket, len(src))
	for i, p := range src {
		out[i] = p
		out[i].Values = append([]uint8(nil), p.Values...)
		if blank {
			clear(out[i].Values)
		}
		if shift {
			out[i].Address += 0x400
		}
	}
	return out, nil
}
func sceneIntroLabels(w *ActiveWorld) error {
	q := &w.Presentation.Queue
	appendPackets := func(ps []GraphicsTextPacket) error {
		for _, p := range ps {
			if err := q.AppendText(p); err != nil {
				return err
			}
		}
		return nil
	}
	stage, _ := sceneTextPackets(12, false, false)
	stage[0].Values[len(stage[0].Values)-3] = w.Combat.World.Level + 2
	if err := appendPackets(stage); err != nil {
		return err
	}
	title, _ := sceneTextPackets(w.Combat.World.Level+17, false, false)
	if err := appendPackets(title); err != nil {
		return err
	}
	for i, score := range []uint16{w.Scores.HighScore, w.Scores.Hundreds[0], w.Scores.Hundreds[1]} {
		if i == 2 && w.Combat.World.PlayerMode == 0 {
			break
		}
		packets, _ := sceneTextPackets(uint8(9+i), false, false)
		v := packets[0].Values
		pos := len(v) - 3
		remaining := score
		for {
			v[pos] = uint8(remaining%10) + 1
			remaining /= 10
			if remaining == 0 {
				break
			}
			pos--
		}
		if score == 0 {
			v[len(v)-3] = 0
			v[len(v)-1] = 1
		} else {
			v[len(v)-2] = 1
			v[len(v)-1] = 1
		}
		if err := appendPackets(packets); err != nil {
			return err
		}
	}
	for i := 0; i <= int(w.Combat.World.PlayerMode); i++ {
		packets, _ := sceneTextPackets(uint8(7+i), false, false)
		p := w.Combat.Players[i]
		lives := (p.GameOver ^ 1) + p.Lives
		empty := lives == 0
		if lives != 0 {
			if int8(lives) < 0 {
				lives = 0
			}
			if lives > 99 {
				lives = 99
			}
			v := packets[0].Values
			if lives != 0 {
				v[len(v)-1] = lives%10 + 1
				if lives >= 10 {
					v[len(v)-2] = lives/10 + 1
				}
			}
		}
		if err := appendPackets(packets); err != nil {
			return err
		}
		if empty {
			over, _ := sceneTextPackets(uint8(15+i), false, false)
			if err := appendPackets(over); err != nil {
				return err
			}
		}
	}
	return nil
}
