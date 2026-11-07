package contra

// Interlude failures are listed before implementation: header-address aliases
// mistaken for gameplay scroll/collision; text FF/F0 parsed as executable code;
// loading missing palette/CHR groups; dimming original 74/FB incorrectly;
// delayed NMI display guard ignored; cinematic particles sent to combat AI;
// clearing lives/score when only transient scene storage is cleared; final-stage
// or Start exit skipping required audio/queue effects; partial rejected writes.
import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
)

func campaignInterludeFromRAM(t *testing.T, b []byte) (CampaignSceneState, Campaign, ActiveWorld, [2]PlayerInput, CampaignSceneEnvironment) {
	_, c, w, in := campaignIntroFromRAM(b)
	s := CampaignSceneState{Interlude: CampaignInterludeState{FadeDelay: b[0x41], FadePhase: b[0x42], Actor: b[0x43], AnimationPhase: b[0x44], AnimationDelay: b[0x45], TextIndex: b[0x46], TextDelay: b[0x47], TextOrigin: uint16(b[0x48]) | uint16(b[0x49])<<8, TextAddress: uint16(b[0x4a]) | uint16(b[0x4b])<<8, MouthIndex: b[0x418]}}
	if b[0x23] != 0 {
		w.Presentation.Queue.Mode = GraphicsRectangular
	}
	copy(w.Header.PaletteIndices[:], b[0x50:0x58])
	w.Presentation.Palette.FadeTimer = b[0x74]
	copy(w.Presentation.Patterns.Desired[:], b[0x7f0:0x7f8])
	w.GenerationBias = b[6]
	copy(w.Terrain.Supertiles[:], b[0x600:0x680])
	copy(w.Combat.Terrain[:], b[0x680:0x700])
	w.Combat.Weapons = originalWeaponPool(t, hex.EncodeToString(b[0x368:0x4b8]))
	w.Combat.Weapons.Projectiles[0].Timer = 0 // 418 is cinematic MouthIndex while routine10 owns the scene.
	for i := 0; i < 16; i++ {
		p := &w.Enemies.Slots[i]
		p.Routine = b[0x4b8+i]
		p.FractionY = b[0x4c8+i]
		p.FractionX = b[0x4d8+i]
		p.VelocityY = int16(uint16(b[0x4f8+i]) | uint16(b[0x4e8+i])<<8)
		p.VelocityX = int16(uint16(b[0x518+i]) | uint16(b[0x508+i])<<8)
		p.Kind = b[0x528+i]
		p.AnimationDelay = b[0x538+i]
		p.VariableA = b[0x548+i]
		p.AttackDelay = b[0x558+i]
		p.Frame = b[0x568+i]
		p.HP = b[0x578+i]
		p.ScoreCollision = b[0x588+i]
		p.StateWidth = b[0x598+i]
		p.Attributes = b[0x5a8+i]
		for j := 0; j < 4; j++ {
			p.Data[j] = b[0x5b8+16*j+i]
		}
	}
	return s, c, w, in, CampaignSceneEnvironment{Busy: b[0x20], DisplayMask: b[0xfe]}
}
func TestCampaignSceneOriginalInterlude(t *testing.T) {
	n := 0
	records := campaignSceneRecords(t, "exit")
	for stage := 1; stage <= 8; stage++ {
		kind := "interlude"
		if stage > 1 {
			kind += fmt.Sprintf("-stage-%d", stage)
		}
		records = append(records, campaignSceneRecords(t, kind)...)
	}
	for _, r := range records {
		if r.Kind != "interlude" {
			continue
		}
		b, a := campaignSceneRAM(t, r.Before), campaignSceneRAM(t, r.After)
		s, c, w, in, env := campaignInterludeFromRAM(t, b)
		ws, wc, ww, _, we := campaignInterludeFromRAM(t, a)
		out, e := s.StepInterlude(&c, &w, in, env)
		if e != nil {
			t.Fatalf("frame%d phase%d: %v", r.Frame, c.EndSequence, e)
		}
		for _, control := range out.Controls {
			switch control.Kind {
			case "ppu-control":
				w.Combat.World.Scroll.NameTable = control.Value
			case "display-mask":
				env.DisplayMask = control.Value
			}
		}
		if s != ws || c != wc || w.Combat != ww.Combat || w.Terrain.Supertiles != ww.Terrain.Supertiles || !reflect.DeepEqual(w.Enemies, ww.Enemies) || w.Header != ww.Header || w.Presentation.Palette != ww.Presentation.Palette || w.Presentation.Patterns != ww.Presentation.Patterns || w.GenerationBias != ww.GenerationBias || env.DisplayMask != we.DisplayMask {
			t.Fatalf("frame%d phase%d scene%+v want%+v campaign%+v want%+v combat=%v map=%v enemies=%v header=%v palette=%v chr=%v bias%d/%d mask%d/%d", r.Frame, b[0x2d], s, ws, c, wc, w.Combat == ww.Combat, w.Terrain.Supertiles == ww.Terrain.Supertiles, reflect.DeepEqual(w.Enemies, ww.Enemies), w.Header == ww.Header, w.Presentation.Palette == ww.Presentation.Palette, w.Presentation.Patterns == ww.Presentation.Patterns, w.GenerationBias, ww.GenerationBias, env.DisplayMask, we.DisplayMask)
		}
		raw := append([]uint8(nil), b[0x700:0x700+int(b[0x21])]...)
		if len(out.ImmediateBackground) > 0 {
			raw = nil
		}
		for _, p := range w.Presentation.Queue.TextPackets {
			v, e := p.Encode()
			if e != nil {
				t.Fatal(e)
			}
			raw = append(raw, v...)
		}
		if len(raw)+int(a[0x21]) != 0 && !reflect.DeepEqual(raw, a[0x700:0x700+int(a[0x21])]) {
			t.Fatalf("frame%d text queue got%x want%x", r.Frame, raw, a[0x700:0x700+int(a[0x21])])
		}
		if w.Presentation.Queue.Used != a[0x21] {
			t.Fatal("queue cursor differs")
		}
		var sounds []int
		for _, command := range out.Sounds {
			if command.Kind == "reset" {
				sounds = append(sounds, -1)
			} else {
				sounds = append(sounds, int(command.ID))
			}
		}
		if !reflect.DeepEqual(append([]int{}, sounds...), append([]int{}, r.Sounds...)) {
			t.Fatalf("frame%d sounds%v want%v", r.Frame, sounds, r.Sounds)
		}
		n++
	}
	if n != 6623 {
		t.Fatalf("incomplete interlude:%d", n)
	}
}

func TestCampaignSceneInterludeQueueFailureIsAtomic(t *testing.T) {
	var s CampaignSceneState
	var c Campaign
	var w ActiveWorld
	var in [2]PlayerInput
	var env CampaignSceneEnvironment
	for _, r := range campaignSceneRecords(t, "interlude-stage-2") {
		if r.Kind != "interlude" {
			continue
		}
		b := campaignSceneRAM(t, r.Before)
		if b[0x2d] == 1 {
			s, c, w, in, env = campaignInterludeFromRAM(t, b)
			break
		}
	}
	w.Presentation.Queue = GraphicsQueue{Mode: GraphicsText}
	if e := w.Presentation.Queue.AppendText(GraphicsTextPacket{Kind: 1, Address: 0x2000, Values: make([]uint8, 184)}); e != nil {
		t.Fatal(e)
	}
	bs, bc, bw := s, c, w
	bw.Presentation.Queue = w.Presentation.Queue.Clone()
	if _, e := s.StepInterlude(&c, &w, in, env); e == nil {
		t.Fatal("accepted overflowing actor recolor")
	}
	if s != bs || c != bc || !reflect.DeepEqual(w, bw) {
		t.Fatal("failed cinematic output partially changed live owners")
	}
}
