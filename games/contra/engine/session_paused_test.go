package contra

// Before the host presentation-only timer: paused display/audio may advance,
// but no pads, entropy, physics, life/score or campaign decision is consumed.
// Only an authorized resume request lets the next normal frame resume.
import (
	"encoding/json"
	"testing"
)

func TestSessionPurePausedPresentationAndResume(t *testing.T) {
	r := levelResources(t, "stage-1.json")
	s, e := NewSession(42, 2, "paused-host")
	if e != nil {
		t.Fatal(e)
	}
	for n := uint64(1); n <= 60; n++ {
		var pads [2]Buttons
		if n == 2 || n == 60 {
			pads[0] = ButtonStart
		}
		if _, e = s.StepPads(n, pads, r); e != nil {
			t.Fatal(n, e)
		}
	}
	if !s.World.Campaign.Paused {
		t.Fatal("did not enter original pause")
	}
	before := *s
	players := s.World.Active.Combat.Players
	displayFrame := s.World.Active.Combat.World.Frame
	for i := 0; i < 10; i++ {
		if _, e = s.PresentPaused(r); e != nil {
			t.Fatal(e)
		}
	}
	if s.Random != before.Random || s.Controls != before.Controls || s.World.Campaign != before.World.Campaign || s.World.Active.Combat.Weapons != before.World.Active.Combat.Weapons || s.World.Active.Enemies.Generator != before.World.Active.Enemies.Generator || s.World.Active.Enemies.Slots != before.World.Active.Enemies.Slots || s.World.Active.Combat.Terrain != before.World.Active.Combat.Terrain || s.World.Active.Scores != before.World.Active.Scores || s.World.Active.Random != before.World.Active.Random {
		t.Fatal("paused timer changed gameplay")
	}
	for i, p := range s.World.Active.Combat.Players {
		p.DisplaySprite = players[i].DisplaySprite
		p.DisplayAttribute = players[i].DisplayAttribute
		if p != players[i] {
			t.Fatal("paused timer moved player")
		}
	}
	if s.PresentationTicks != 10 || s.Audio.Tick != 70 || s.World.Active.Combat.World.Frame != displayFrame+10 || s.Revision != 70 {
		t.Fatal("separate paused clocks differ")
	}
	if e = s.Validate(); e != nil {
		t.Fatal(e)
	}
	if e = s.RequestResume(); e != nil {
		t.Fatal(e)
	}
	revision := s.Revision
	if e = s.RequestResume(); e != nil || s.Revision != revision {
		t.Fatal("resume request repeated")
	}
	raw, _ := json.Marshal(s)
	if _, e = s.PresentPaused(r); e == nil {
		t.Fatal("presentation timer consumed resume")
	}
	after, _ := json.Marshal(s)
	if string(raw) != string(after) {
		t.Fatal("rejected timer changed state")
	}
	var restored Session
	if e = json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	if _, e = s.StepPads(61, [2]Buttons{}, r); e != nil {
		t.Fatal(e)
	}
	if _, e = restored.StepPads(61, [2]Buttons{}, r); e != nil {
		t.Fatal(e)
	}
	if s.World.Campaign.Paused || s.ResumeRequested || s.Random.Frames != 61 || s.Audio.Tick != 71 || s.Revision != revision+1 {
		t.Fatal("normal frame did not resume")
	}
	x, _ := json.Marshal(s)
	y, _ := json.Marshal(restored)
	if string(x) != string(y) {
		t.Fatal("pending resume restore diverged")
	}
}
