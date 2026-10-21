package engine

import "testing"

// Failure inventory: reversing before DMA changes the currently visible frame;
// failing to reverse the whole64-slot working buffer changes later draws;
// running underScene7 mode80 or with a detached hoop violatesF268..F274.
func TestOriginalPostResultVideoOAMReverseEffect(t *testing.T) {
	chain := postResultVideoRows(t)[0]
	base := WaitAfterOriginalPostResultDispatch(postResultVideoRAM(chain.Initial, chain.Progress))
	for _, c := range []struct {
		mode, flags, detached uint8
		want                  bool
	}{{192, 128, 0, true}, {192, 0, 0, false}, {192, 128, 1, false}, {128, 128, 0, false}} {
		s := base
		s.Context.Ending.MachineControl = c.mode
		s.Scene.Transition.DisplayFlags = c.flags
		s.Scene.DetachedCount = c.detached
		got, err := StepOriginalPostResultVideo(s, [4]ControllerFrame{})
		if err != nil {
			t.Fatal(err)
		}
		nmi, reverse := -1, -1
		for i, e := range got.Effects {
			if e.Kind == "normal-nmi" {
				nmi = i
			}
			if e.Kind == "oam-reverse" {
				if reverse >= 0 {
					t.Fatal("duplicate reversal")
				}
				reverse = i
			}
		}
		if nmi < 0 {
			t.Fatal("source normalNMI absent")
		}
		if (reverse >= 0) != c.want {
			t.Fatalf("mode%d flags%d detached%d reverse=%d want%t", c.mode, c.flags, c.detached, reverse, c.want)
		}
		if c.want && reverse != nmi+1 {
			t.Fatal("F002 must follow DMA/NMI before resumed mainline")
		}
	}
}
