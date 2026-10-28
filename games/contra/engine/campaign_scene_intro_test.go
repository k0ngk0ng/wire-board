package contra

// Before implementation: preserve the shared text queue's exact admission cost;
// do not convert original immediate backgrounds into delayed rectangles; do not
// replace scroll/reveal/beacon phases with a timer; preserve title/score/lives;
// apply palette cycling before phase-specific replacement; reject unavailable
// resources without changing any live scene owner.
import (
	"fmt"
	"reflect"
	"testing"
)

func campaignIntroFromRAM(b []byte) (CampaignSceneState, Campaign, ActiveWorld, [2]PlayerInput) {
	s := CampaignSceneState{IntroScroll: b[0x70], Reveal: b[0x7e]}
	c := Campaign{EndSequence: b[0x2d], DelayLow: b[0x2a]}
	w := ActiveWorld{}
	w.Combat.World = OutdoorWorld{GameRoutine: b[0x18], LevelRoutine: b[0x2c], Level: b[0x30], Frame: b[0x1a], PlayerMode: b[0x22], Scroll: CollisionScroll{X: b[0xfd], Y: b[0xfc], NameTable: b[0xff]}}
	var in [2]PlayerInput
	for i := 0; i < 26; i++ {
		o := LogicalSprite{Code: b[0x300+i], X: b[0x334+i], Y: b[0x31a+i], Attributes: b[0x34e+i]}
		if i < 10 {
			w.Combat.Display[i] = o
		} else {
			p := &w.Enemies.Slots[i-10]
			p.Sprite = o.Code
			p.X = o.X
			p.Y = o.Y
			p.SpriteAttributes = o.Attributes
		}
	}
	for i := 0; i < 2; i++ {
		w.Combat.Players[i] = Player{X: b[0x334+i], Motion: PlayerMotion{Position: int32(b[0x31a+i]) << 8}, DisplaySprite: b[0x300+i], DisplayAttribute: b[0x34e+i], Lives: b[0x32+i], GameOver: b[0x38+i]}
		in[i] = PlayerInput{Held: Buttons(b[0xf1+i]), Pressed: Buttons(b[0xf5+i])}
	}
	w.Presentation.Queue.Mode = GraphicsText
	w.Presentation.Queue.Used = b[0x21]
	copy(w.Presentation.Palette.Desired[:], b[0x7c0:0x7e0])
	w.Presentation.Palette.RequestedColors = b[0x36]
	w.Scores.HighScore = uint16(b[0x7e0]) | uint16(b[0x7e1])<<8
	for i := 0; i < 2; i++ {
		w.Scores.Hundreds[i] = uint16(b[0x7e2+2*i]) | uint16(b[0x7e3+2*i])<<8
	}
	return s, c, w, in
}
func TestCampaignSceneOriginalIntro(t *testing.T) {
	n := 0
	records := append(campaignSceneRecords(t, "natural"), campaignSceneRecords(t, "intro-cases")...)
	for stage := 2; stage <= 8; stage++ {
		records = append(records, campaignSceneRecords(t, fmt.Sprintf("natural-stage-%d", stage))...)
	}
	for _, r := range records {
		if r.Kind != "intro" {
			continue
		}
		b, a := campaignSceneRAM(t, r.Before), campaignSceneRAM(t, r.After)
		s, c, w, in := campaignIntroFromRAM(b)
		ws, wc, ww, _ := campaignIntroFromRAM(a)
		out, e := s.StepIntro(&c, &w, in, b[0x1c] != 0)
		if e != nil {
			t.Fatalf("frame%d phase%d: %v", r.Frame, c.EndSequence, e)
		}
		for _, control := range out.Controls {
			if control.Kind == "ppu-control" {
				w.Combat.World.Scroll.NameTable = control.Value
			}
		}
		if s != ws || c != wc || w.Combat != ww.Combat || !reflect.DeepEqual(w.Enemies, ww.Enemies) || w.Presentation.Palette != ww.Presentation.Palette {
			t.Fatalf("frame%d phase%d scene%+v want%+v campaign%+v want%+v; world/player/objects=%v enemies=%v palette=%v", r.Frame, b[0x2d], s, ws, c, wc, w.Combat == ww.Combat, reflect.DeepEqual(w.Enemies, ww.Enemies), w.Presentation.Palette == ww.Presentation.Palette)
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
		if !reflect.DeepEqual(raw, a[0x700:0x700+int(a[0x21])]) && len(raw)+int(a[0x21]) != 0 {
			t.Fatalf("frame%d phase%d text queue got%x want%x", r.Frame, b[0x2d], raw, a[0x700:0x700+int(a[0x21])])
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
		if r.Sounds != nil && !reflect.DeepEqual(append([]int{}, sounds...), append([]int{}, r.Sounds...)) {
			t.Fatalf("frame%d sounds%v want%v", r.Frame, sounds, r.Sounds)
		}
		n++
	}
	if n != 5225 {
		t.Fatalf("incomplete intro: %d", n)
	}
}

func TestCampaignSceneIntroQueueFailureIsAtomic(t *testing.T) {
	s := CampaignSceneState{Reveal: 255}
	c := Campaign{EndSequence: 7}
	w := ActiveWorld{}
	w.Combat.World = OutdoorWorld{GameRoutine: 5, LevelRoutine: 1}
	w.Presentation.Queue.Mode = GraphicsText
	if e := w.Presentation.Queue.AppendText(GraphicsTextPacket{Kind: 1, Address: 0x2000, Values: make([]uint8, 156)}); e != nil {
		t.Fatal(e)
	}
	bs, bc, bw := s, c, w
	bw.Presentation.Queue = w.Presentation.Queue.Clone()
	if _, e := s.StepIntro(&c, &w, [2]PlayerInput{}, false); e == nil {
		t.Fatal("accepted overflowing second reveal packet")
	}
	if s != bs || c != bc || !reflect.DeepEqual(w, bw) {
		t.Fatal("failed second packet partially changed intro")
	}
}
