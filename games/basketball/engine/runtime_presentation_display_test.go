package engine

// Failure inventory before implementation: stale result screen throughout reward;
// lost retained court rows/attributes; result/reward writes erase whole tables;
// submit becomes visible before DMA; JSON restore drops OAM/surface; double
// application of an upload; canonical IRQ samples frozen NMI credits; invented
// exact-source timing; source-outcome/RNG mutation; legacy restores animate.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRuntimePresentationDisplaySourceSeed(t *testing.T) {
	var rows []struct {
		Court   uint8
		Columns []CourtColumnCommit
		Patches []CourtTilePatch
		Digits  [2][3]uint8
		Surface OriginalRasterSurface
	}
	readPropAttackFixture(t, "testdata/postresult-runtime-seed.json.gz", &rows)
	if len(rows) != 8 {
		t.Fatal("missing court seed domains")
	}
	for _, row := range rows {
		s := runtimePresentationSourceShell(t, false)
		scene := s.Transition.Scene
		scene.Transition.Court = row.Court
		scene.Transition.Match.DisplayedScores = row.Digits
		d := s.Display
		d.CourtColumns = row.Columns
		d.CourtPatches = row.Patches
		got, e := seedRuntimePostResultSurface(scene, d)
		if e != nil || got != row.Surface {
			t.Fatalf("court%d retained result surface: %v", row.Court, e)
		}
	}
}

func TestRuntimePresentationDisplayLifecycle(t *testing.T) {
	for _, ending := range []bool{false, true} {
		s := runtimePresentationSourceShell(t, ending)
		expected, e := SettleCampaignRuntime(s)
		if e != nil {
			t.Fatal(e)
		}
		v, e := NewOriginalPostResultVideoAfterResult(s.Transition.Scene, s.Display, *s.Config.Campaign, s.LogicFrame)
		if e != nil {
			t.Fatal(e)
		}
		s.Presentation = &RuntimePresentationState{PendingResult: expected, Video: &v}
		s.Transition = nil
		s.Carry = nil
		s.FramesUntilLogic = 0
		initial := s
		var confirmFrames []uint64
		seenChanged, seenDMA := false, false
		for tick := 0; tick < 15000 && !RuntimeFinished(s); tick++ {
			before, _ := json.Marshal(s)
			held := [4]Buttons{}
			if RuntimePresentationNeedsConfirm(s) {
				held[0] = ButtonStart
				confirmFrames = append(confirmFrames, s.VideoFrames+1)
				if ending {
					writeRuntimePresentationArtifact(t, "ending-needs-confirm.json", s)
				}
			}
			next, e := StepVideo(s, held)
			if e != nil {
				t.Fatalf("ending%v frame%d: %v", ending, tick, e)
			}
			if after, _ := json.Marshal(s); string(before) != string(after) {
				t.Fatal("mutated prior snapshot")
			}
			render, e := RuntimePostResultRender(next.State)
			if e != nil || render == nil {
				t.Fatal("missing render", tick, e)
			}
			if !reflect.DeepEqual(render, next.State.Display.Visible.PostResult) {
				t.Fatal("persisted render differs", tick)
			}
			if render.Plan.TimingMode != OriginalRasterCanonical || render.Plan.TimingSource == "" {
				t.Fatal("unidentified timing")
			}
			if tick%71 == 0 || RuntimeFinished(next.State) {
				if _, e = ProjectOriginalPostResultRasterRows(render.Surface, render.Plan); e != nil {
					t.Fatal("projection", tick, e)
				}
				var restored RuntimeState
				if e = json.Unmarshal(before, &restored); e != nil {
					t.Fatal(e)
				}
				replay, e := StepVideo(restored, held)
				if e != nil || !reflect.DeepEqual(next, replay) {
					t.Fatal("display restore", tick, e)
				}
			}
			if next.State.Presentation.Display.HasVisible {
				seenDMA = true
			}
			if s.Display.Visible.PostResult != nil && render.Surface != s.Display.Visible.PostResult.Surface {
				seenChanged = true
			}
			s = next.State
		}
		if ending {
			writeRuntimePresentationArtifact(t, "ending.json", map[string]any{"initial": initial, "final": s, "confirmFrames": confirmFrames, "scope": "controlled original result-confirmation boundary; not earned14-win campaign"})
		}
		if !RuntimeFinished(s) || !seenChanged || !seenDMA {
			t.Fatal("no complete displayed lifecycle", ending)
		}
	}
}

func TestRuntimePresentationCanonicalUsesLiveIRQ(t *testing.T) {
	nmi := OriginalPostResultRasterSource{Scene: 7, Phase: 136, MachineControl: 128, Control: 136, Mask: 24, BackgroundCHR: [2]uint8{20, 22}}
	live := nmi
	live.Credits = [4]uint8{7, 13, 19, 23}
	got, e := buildRuntimePostResultPlan(nmi, live, false)
	if e != nil {
		t.Fatal(e)
	}
	var scrolls []uint16
	for _, effect := range got.Effects {
		if effect.Dot >= 0 && effect.Kind == OriginalRasterScrollX {
			scrolls = append(scrolls, effect.Value)
		}
	}
	if !reflect.DeepEqual(scrolls, []uint16{7, 13, 19, 23, 0}) {
		t.Fatal("IRQ froze NMI offsets", scrolls)
	}
}

func TestRuntimePresentationDisplayRejectsCorruptRestore(t *testing.T) {
	s := runtimePresentationSourceShell(t, false)
	expected, _ := SettleCampaignRuntime(s)
	v, e := NewOriginalPostResultVideoAfterResult(s.Transition.Scene, s.Display, *s.Config.Campaign, s.LogicFrame)
	if e != nil {
		t.Fatal(e)
	}
	s.Presentation = &RuntimePresentationState{PendingResult: expected, Video: &v}
	s.Transition = nil
	s.Carry = nil
	s.FramesUntilLogic = 0
	first, e := StepVideo(s, [4]Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	s = first.State
	for _, kind := range []string{"missing-display", "bad-palette", "bad-mirroring", "bad-chr", "forged-render"} {
		raw, _ := json.Marshal(s)
		var bad RuntimeState
		if e = json.Unmarshal(raw, &bad); e != nil {
			t.Fatal(e)
		}
		switch kind {
		case "missing-display":
			bad.Presentation.Display = nil
		case "bad-palette":
			bad.Presentation.Display.Surface.Palette[1] = 64
		case "bad-mirroring":
			bad.Presentation.Display.Surface.Mirroring = 2
		case "bad-chr":
			bad.Presentation.Display.VisibleFieldCHR[0] = 128
		case "forged-render":
			bad.Display.Visible.PostResult.OAM[0] ^= 1
		}
		if e = ValidateRuntimeState(bad); e == nil {
			t.Fatal("accepted corrupt presentation display", kind)
		}
	}
}

func writeRuntimePresentationArtifact(t *testing.T, name string, value any) {
	t.Helper()
	dir := os.Getenv("BASKETBALL_PRESENTATION_ARTIFACT_DIR")
	if dir == "" {
		return
	}
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, name), append(raw, '\n'), 0644); e != nil {
		t.Fatal(e)
	}
}
