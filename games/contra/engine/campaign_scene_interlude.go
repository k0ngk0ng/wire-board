package contra

import "errors"

// Interlude-specific values occupy the cartridge's retired header storage.
// These have distinct cinematic meanings: FadeDelay is not gameplay ScrollType,
// and TextAddress is not a collision limit. Palette/CHR/Queue remain shared.
type CampaignInterludeState struct {
	FadeDelay, FadePhase, Actor, AnimationPhase, AnimationDelay uint8
	TextIndex, TextDelay, MouthIndex                            uint8
	TextOrigin, TextAddress                                     uint16
}
type CampaignSceneEnvironment struct{ Busy, DisplayMask uint8 }

func (s *CampaignSceneState) StepInterlude(c *Campaign, w *ActiveWorld, inputs [2]PlayerInput, env CampaignSceneEnvironment) (CampaignSceneResult, error) {
	if s == nil || c == nil || w == nil || w.Combat.World.GameRoutine != 5 || w.Combat.World.LevelRoutine != 10 || w.Combat.World.Level >= 8 || c.EndSequence > 23 {
		return CampaignSceneResult{}, errors.New("invalid campaign interlude")
	}
	ns, nc, nw := *s, *c, *w
	nw.Presentation.Queue = w.Presentation.Queue.Clone()
	out, err := ns.stepInterlude(&nc, &nw, inputs, env)
	if err != nil {
		return CampaignSceneResult{}, err
	}
	*s, *c, *w = ns, nc, nw
	return out, nil
}
func (s *CampaignSceneState) stepInterlude(c *Campaign, w *ActiveWorld, inputs [2]PlayerInput, env CampaignSceneEnvironment) (CampaignSceneResult, error) {
	var out CampaignSceneResult
	control := func(kind string, value uint8) { out.Controls = append(out.Controls, CampaignSceneControl{kind, value}) }
	sound := func(id uint8) { out.Sounds = append(out.Sounds, WorldSoundCommand{Kind: "sound", ID: id}) }
	stop := func() { out.Sounds = append(out.Sounds, WorldSoundCommand{Kind: "reset"}) }
	done := func() {
		w.Presentation.Queue.Mode = GraphicsText
		stop()
		w.Combat.World.LevelRoutine = 5
		c.EndSequence = 0
	}
	clearBackground := func(id uint8) {
		out.ImmediateBackground = append(out.ImmediateBackground, sceneBackground(id)...)
		w.Presentation.Queue.Reset()
		w.Combat.World.Scroll.X = 0
		w.Combat.World.Scroll.Y = 0
		control("display-disable", 0)
		control("ppu-control", 0xb0)
		control("display-busy", 5)
	}
	if inputs[0].Held&ButtonStart != 0 {
		control("display-disable", 0)
		control("display-busy", 5)
		done()
		return out, nil
	}
	control("graphics-mode", 0)
	w.Presentation.Palette.FadeTimer = 0
	if w.Presentation.Queue.Used != 0 && w.Presentation.Queue.Mode != GraphicsText {
		return out, errors.New("interlude cannot reinterpret queued rectangles")
	}
	w.Presentation.Queue.Mode = GraphicsText
	p := &s.Interlude
	level := w.Combat.World.Level
	group := uint8(0)
	if level >= 5 {
		group = 1
	}
	loadPalette := func() error {
		resources := &LevelResources{colors: campaignSceneColors}
		result, err := w.Presentation.Palette.Load(32, group, w.Header, resources)
		if err == nil {
			w.applyPaletteBias(result)
		}
		return err
	}
	advance := func(delay uint8) { c.DelayLow = delay; c.EndSequence++ }
	if c.EndSequence >= 13 {
		if err := advanceInterludeParticles(w); err != nil {
			return out, err
		}
	}
	switch c.EndSequence {
	case 0:
		if level == 7 {
			done()
			return out, nil
		}
		clearBackground(0)
		stop()
		*p = CampaignInterludeState{}
		w.Header.CollisionLimits = [3]uint8{}
		w.Header.CycleIndices = [4]uint8{}
		w.Combat.Weapons = WeaponSystem{}
		for i := 0; i < 26; i++ {
			setSceneObject(w, i, LogicalSprite{})
		}
		w.Enemies.Slots = [16]Enemy{}
		p.Actor = level & 1
		if w.Combat.Players[0].GameOver|w.Combat.Players[1].GameOver != 0 {
			p.Actor = 0
			if w.Combat.Players[0].GameOver != 0 {
				p.Actor = 1
			}
		}
		clearBackground(campaignSceneInterludeBackground[group])
		copy(w.Header.PaletteIndices[:], campaignSceneInterludePalette[:])
		w.Presentation.Patterns.Desired = campaignScenePresets[campaignSceneInterludeCHR[group]]
		for i, o := range campaignSceneObjects[group*2+p.Actor] {
			setSceneObject(w, i+1, o)
		}
		advance(255)
	case 1:
		if p.Actor != 0 {
			w.Header.PaletteIndices[6] = 0x81
			w.Header.PaletteIndices[2] = 0x82
			ps, err := sceneTextPackets(128+group, false, false)
			if err != nil {
				return out, err
			}
			for _, packet := range ps {
				if err = w.Presentation.Queue.AppendText(packet); err != nil {
					return out, err
				}
			}
		}
		if err := loadPalette(); err != nil {
			return out, err
		}
		o := sceneObject(w, 0)
		o.Y = campaignSceneInterludeMarkerY[group]
		o.X = 5
		o.Code = 0x4f
		o.Attributes = 0x20
		setSceneObject(w, 0, o)
		c.EndSequence++
	case 2:
		if env.Busy != 0 {
			break
		}
		control("raster-mask", env.DisplayMask&0xef)
		control("raster-control", w.Combat.World.Scroll.NameTable|1)
		o := sceneObject(w, 0)
		o.Y++
		setSceneObject(w, 0, o)
		if o.Y >= 0xba {
			advance(0x20)
		}
	case 3:
		c.DelayLow--
		if c.DelayLow == 0 {
			c.EndSequence = 4
			if group != 0 {
				c.EndSequence = 13
			}
		}
	case 4:
		o := sceneObject(w, 1)
		o.Code = 0x5b
		setSceneObject(w, 1, o)
		p.MouthIndex = 0
		advance(8)
	case 5:
		c.DelayLow--
		if c.DelayLow != 0 {
			break
		}
		if p.MouthIndex == 0 {
			sound(0x13)
		}
		c.DelayLow = 4
		if int(p.MouthIndex) >= len(campaignSceneMouthFrames) {
			return out, errors.New("invalid interlude mouth phase")
		}
		o := sceneObject(w, 1)
		o.Code = campaignSceneMouthFrames[p.MouthIndex]
		setSceneObject(w, 1, o)
		p.MouthIndex++
		if p.MouthIndex >= 3 {
			c.EndSequence++
		}
	case 6:
		c.EndSequence++
	case 7, 18:
		p.TextOrigin = 0x2111
		p.TextAddress = 0x2111
		p.TextIndex = 0
		p.TextDelay = 0x21
		c.EndSequence++
	case 8, 19:
		if c.EndSequence == 8 {
			o := sceneObject(w, 2)
			o.Code = 0
			if p.TextDelay < 7 && w.Combat.World.Frame&8 == 0 {
				o.Code = 0x67
			}
			setSceneObject(w, 2, o)
		}
		ready, err := p.advanceText(w, &out)
		if err != nil {
			return out, err
		}
		if ready {
			advance(0x60)
		}
	case 9, 20:
		c.DelayLow--
		if c.DelayLow == 0 {
			c.EndSequence++
		}
	case 10, 21:
		p.FadePhase = 5
		p.FadeDelay = 1
		c.EndSequence++
	case 11, 22:
		p.FadeDelay--
		if p.FadeDelay != 0 {
			break
		}
		p.FadeDelay = 8
		if p.FadePhase > 5 {
			return out, errors.New("invalid interlude fade phase")
		}
		index := int(p.FadePhase) * 2
		mask := campaignSceneShadeSteps[index]
		if mask == 255 {
			advance(0x40)
			break
		}
		if err := loadPalette(); err != nil {
			return out, err
		}
		p.FadePhase--
		control("display-mask", env.DisplayMask&0x1f|mask)
		cap := campaignSceneShadeSteps[index+1]
		for i, color := range w.Presentation.Palette.Desired {
			if cap == 15 {
				w.Presentation.Palette.Desired[i] = 15
			} else if color != 15 && color&0xf0 >= cap {
				w.Presentation.Palette.Desired[i] = color&15 | cap
			}
		}
	case 12, 23:
		c.DelayLow--
		if c.DelayLow == 0 {
			clearBackground(0)
			control("display-mask", 0x1e)
			done()
		}
	case 13:
		advance(1)
	case 14:
		c.DelayLow--
		if c.DelayLow == 0 {
			sound(0x20)
			advance(0x60)
		}
	case 15:
		if w.Combat.World.Frame&1 == 0 {
			break
		}
		if c.DelayLow == 0 {
			o := sceneObject(w, 1)
			o.Code = 0
			setSceneObject(w, 1, o)
			c.EndSequence++
			break
		}
		c.DelayLow--
		at := int(c.DelayLow & 2)
		w.Presentation.Patterns.Desired[5] = campaignSceneParticleCHR[at]
		w.Presentation.Patterns.Desired[6] = campaignSceneParticleCHR[at+1]
		phase := (c.DelayLow >> 1) & 1
		if p.Actor > 1 {
			return out, errors.New("invalid cinematic actor")
		}
		o := sceneObject(w, 3)
		o.Code = campaignSceneHeadFrames[p.Actor*2+phase]
		setSceneObject(w, 3, o)
		o = sceneObject(w, 1)
		o.Code = 0
		if c.DelayLow&3 != 0 {
			o.Code = campaignSceneBlinkFrames[phase]
		}
		setSceneObject(w, 1, o)
		if c.DelayLow&3 == 2 {
			spawnInterludeParticle(w)
		}
		p.AnimationDelay++
		if p.AnimationDelay >= 24 {
			p.AnimationDelay = 0
			p.AnimationPhase++
		}
		frame := p.AnimationPhase
		if frame >= 4 {
			frame = 4
		}
		o = sceneObject(w, 2)
		if at == 0 {
			o.Code = campaignSceneArm0[frame]
		} else {
			o.Code = campaignSceneArm1[frame]
		}
		setSceneObject(w, 2, o)
	case 16:
		advance(0x30)
	case 17:
		c.DelayLow--
		if c.DelayLow == 0 {
			c.EndSequence++
		}
	}
	return out, nil
}
func (p *CampaignInterludeState) advanceText(w *ActiveWorld, out *CampaignSceneResult) (bool, error) {
	p.TextDelay--
	if p.TextDelay != 0 {
		return false, nil
	}
	p.TextDelay = 6
	text, ok := campaignSceneLetters[w.Combat.World.Level]
	if !ok || int(p.TextIndex) >= len(text) {
		return false, errors.New("invalid cinematic text position")
	}
	marker := text[p.TextIndex]
	if marker == 255 {
		return true, nil
	}
	index := p.TextIndex
	if marker == 0xf0 {
		index++
	}
	if int(index) >= len(text) {
		return false, errors.New("incomplete cinematic line marker")
	}
	value := text[index]
	if err := w.Presentation.Queue.AppendText(GraphicsTextPacket{Kind: 1, Address: p.TextAddress, Values: []uint8{value}}); err != nil {
		return false, err
	}
	if value != 0 {
		out.Sounds = append(out.Sounds, WorldSoundCommand{Kind: "sound", ID: 0x10})
	}
	p.TextIndex = index + 1
	p.TextAddress = p.TextAddress&0xff00 | uint16(uint8(p.TextAddress)+1)
	if marker == 0xf0 {
		p.TextOrigin += 0x40
		p.TextAddress = p.TextOrigin
		p.TextDelay = 0x10
	}
	return false, nil
}
func advanceInterludeParticles(w *ActiveWorld) error {
	for i := 15; i >= 0; i-- {
		e := &w.Enemies.Slots[i]
		if e.Routine == 0 {
			continue
		}
		if e.Routine != 1 {
			return errors.New("unknown cinematic particle routine")
		}
		e.VelocityY += 16
		y := uint16(e.FractionY) + uint16(uint8(e.VelocityY))
		e.FractionY = uint8(y)
		e.Y += uint8(e.VelocityY>>8) + uint8(y>>8)
		if e.Y >= 0xc8 {
			e.Routine = 0
			e.Sprite = 0
			continue
		}
		x := uint16(e.FractionX) + uint16(uint8(e.VelocityX))
		e.FractionX = uint8(x)
		e.X += uint8(e.VelocityX>>8) + uint8(x>>8)
		if e.X < 8 {
			e.Routine = 0
			e.Sprite = 0
		}
	}
	return nil
}
func spawnInterludeParticle(w *ActiveWorld) {
	for i := 15; i >= 0; i-- {
		e := &w.Enemies.Slots[i]
		if e.Routine != 0 {
			continue
		}
		e.Routine = 1
		e.VelocityY = -256
		e.VelocityX = -192
		e.FractionY = 0
		e.FractionX = 0
		e.Y = 80
		e.X = 64
		e.Sprite = 0xdd
		e.SpriteAttributes = 0
		return
	}
}
