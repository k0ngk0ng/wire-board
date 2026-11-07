package contra

// Display failure inventory before code: C910 immediate clears versus queued
// text, wrong high-score/P2/zero digits, exhausted continues still showing an
// offer, cursor selection updated twice, Start+Select drawing instead of hiding,
// restarting resources twice, erasing cursor attributes or player Y high bytes,
// and duplicate CampaignPlan sounds. Reject invalid modes without partial state.
import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestCampaignResetOriginalGameOverDisplay(t *testing.T) {
	meta := loadFrameOracle(t)
	count := 0
	for _, r := range campaignResetRecords(t) {
		if r.Kind != "transition" || campaignSceneRAM(t, r.Before)[0x3b] != 0 {
			continue
		}
		t.Run(fmt.Sprintf("case%d", r.Case), func(t *testing.T) {
			b, a := campaignSceneRAM(t, r.Before), campaignSceneRAM(t, r.After)
			c, w, s := campaignResetFromRAM(t, meta, r.Before)
			_, want, wantScene := campaignResetFromRAM(t, meta, r.After)
			plan, e := c.BeginTick(uint64(r.Frame), &w.Combat.World, campaignResetInput(&w, b))
			if e != nil {
				t.Fatal(e)
			}
			for _, cmd := range plan.Commands {
				if cmd.Reset != nil {
					if e = ApplyCampaignRuntimeReset(&w, &s, cmd.Reset); e != nil {
						t.Fatal(e)
					}
				}
			}
			out, e := PrepareCampaignGameOver(&w, w.Combat.World.LevelRoutine == 6)
			if e != nil {
				t.Fatal(e)
			}
			for _, control := range out.Controls {
				switch control.Kind {
				case "ppu-control":
					w.Combat.World.Scroll.NameTable = control.Value
					if control.Value != a[0xff] {
						t.Fatal("control differs")
					}
				case "display-busy":
					if control.Value != a[0x20] {
						t.Fatal("busy differs")
					}
				case "display-disable":
					if control.Value != 0 {
						t.Fatal("display remains enabled during immediate copy")
					}
				default:
					t.Fatal("unknown display effect", control)
				}
			}
			queue, e := w.Presentation.Queue.Encode()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(queue, a[0x700:0x700+int(a[0x21])]) {
				t.Fatalf("queue got%x want%x", queue, a[0x700:0x700+int(a[0x21])])
			}
			campaignResetQueue(t, &want, r.After)
			if !reflect.DeepEqual(w, want) || s != wantScene {
				t.Fatalf("gameover case%d world got%+v want%+v scene%+v/%+v", r.Case, w, want, s, wantScene)
			}
			if len(out.Sounds) != 0 || len(plan.Sounds) != 1 || len(r.Sounds) != 1 || int(plan.Sounds[0]) != r.Sounds[0] {
				t.Fatal("sound duplicated or lost", out.Sounds, plan.Sounds, r.Sounds)
			}
			ppu, wantPPU := campaignResetPPU(t, r.PPUBefore), campaignResetPPU(t, r.PPUAfter)
			for _, write := range out.ImmediateBackground {
				for i, v := range write.Values {
					at := (int(write.Address) - 0x2000 + i*int(write.Step)) & 0x7ff
					ppu[at] = v
					ppu[at+0x800] = v
				}
			}
			if !bytes.Equal(ppu, wantPPU) {
				t.Fatal("immediate gameover background differs")
			}
			count++
		})
	}
	if count != 8 {
		t.Fatal("incomplete game-over display", count)
	}
}

func TestCampaignResetOriginalContinueDisplay(t *testing.T) {
	meta := loadFrameOracle(t)
	count := 0
	for _, r := range campaignResetRecords(t) {
		if r.Kind != "continueMenu" {
			continue
		}
		t.Run(fmt.Sprintf("case%d", r.Case), func(t *testing.T) {
			b := campaignSceneRAM(t, r.Before)
			c, w, s := campaignResetFromRAM(t, meta, r.Before)
			wc, want, wantScene := campaignResetFromRAM(t, meta, r.After)
			input := campaignResetInput(&w, b)
			plan, e := c.BeginTick(uint64(r.Frame), &w.Combat.World, input)
			if e != nil {
				t.Fatal(e)
			}
			for _, cmd := range plan.Commands {
				if cmd.Reset != nil {
					if e = ApplyCampaignRuntimeReset(&w, &s, cmd.Reset); e != nil {
						t.Fatal(e)
					}
				}
			}
			out, e := StepCampaignContinueDisplay(&w, &c, input.Pressed)
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(w, want) || s != wantScene || c.Choice != wc.Choice {
				t.Fatalf("continue case%d world got%+v want%+v scene%+v/%+v choice%d/%d", r.Case, w, want, s, wantScene, c.Choice, wc.Choice)
			}
			if !reflect.DeepEqual(out, CampaignSceneResult{}) {
				t.Fatal("cursor invented display/audio work", out)
			}
			sounds := []int{}
			for _, cmd := range plan.Commands {
				if cmd.Kind == "stopAudio" {
					sounds = append(sounds, -1)
				}
			}
			if !reflect.DeepEqual(sounds, append([]int{}, r.Sounds...)) {
				t.Fatal("menu audio differs", sounds, r.Sounds)
			}
			count++
		})
	}
	if count != 32 {
		t.Fatal("incomplete continue display", count)
	}
}

func TestCampaignResetDisplayFailuresAtomic(t *testing.T) {
	for _, name := range []string{"nil-world", "wrong-game", "wrong-phase", "wrong-mode", "mismatched-offer", "nil-campaign", "bad-choice", "cursor-phase", "bad-start"} {
		t.Run(name, func(t *testing.T) {
			w := ActiveWorld{}
			w.Combat.World = OutdoorWorld{GameRoutine: 5, LevelRoutine: 6}
			w.Presentation.Queue.Mode = GraphicsText
			c := NewCampaign()
			wp, cp := &w, &c
			offer := true
			pressed := Buttons(0)
			cursor := false
			switch name {
			case "nil-world":
				wp = nil
			case "wrong-game":
				w.Combat.World.GameRoutine = 3
			case "wrong-phase":
				w.Combat.World.LevelRoutine = 4
			case "wrong-mode":
				w.Presentation.Queue.Mode = GraphicsRectangular
			case "mismatched-offer":
				offer = false
			case "nil-campaign":
				cp = nil
				cursor = true
			case "bad-choice":
				c.Choice = 2
				cursor = true
			case "cursor-phase":
				w.Combat.World.LevelRoutine = 4
				cursor = true
			case "bad-start":
				pressed = ButtonStart
				cursor = true
			}
			bw, bc := w, c
			var out CampaignSceneResult
			var e error
			if cursor {
				out, e = StepCampaignContinueDisplay(wp, cp, pressed)
			} else {
				out, e = PrepareCampaignGameOver(wp, offer)
			}
			if e == nil {
				t.Fatal("invalid display context accepted")
			}
			if !reflect.DeepEqual(w, bw) || c != bc || !reflect.DeepEqual(out, CampaignSceneResult{}) {
				t.Fatal("display failure partially mutated state")
			}
		})
	}
}
