package contra

// Failure cases precede implementation: script input must replace both players'
// old edges; airborne exit waits must not complete; timers wrap as bytes; odd
// frames alone decrement the shared escape budget; out players stay out; true
// script progress precedes routine 10; palette fade shares the existing owner;
// rejected state never partially changes world, players, input or campaign.
import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type campaignSceneRecord struct {
	Kind          string
	Frame         int
	Before, After string
	A, P          uint8
	Sounds        []int
}

func campaignSceneRecords(t *testing.T, kind string) []campaignSceneRecord {
	t.Helper()
	f, e := os.Open("../../../docs/research/nes-contra/evidence/campaign-scene-" + kind + "-original.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var d struct{ Records []campaignSceneRecord }
	if e = json.NewDecoder(z).Decode(&d); e != nil {
		t.Fatal(e)
	}
	return d.Records
}
func campaignSceneRAM(t *testing.T, raw string) []byte {
	t.Helper()
	b, e := hex.DecodeString(raw)
	if e != nil || len(b) != 2048 {
		t.Fatal("invalid RAM", e)
	}
	return b
}
func campaignExitFromRAM(b []byte) (CampaignSceneState, Campaign, ActiveWorld, [2]PlayerInput) {
	s := CampaignSceneState{ExitDelay: b[0x190], ExitBudget: b[0x191], ExitFadeSteps: b[0xfb], ExitActors: [2]uint8{b[0x192], b[0x193]}}
	c := Campaign{EndSequence: b[0x2d]}
	w := ActiveWorld{}
	w.Combat.World = OutdoorWorld{GameRoutine: b[0x18], LevelRoutine: b[0x2c], Level: b[0x30], BossDefeated: b[0x3b], Frame: b[0x1a]}
	var in [2]PlayerInput
	for i := 0; i < 2; i++ {
		pos := int32(b[0x94+i]) | int32(b[0x31a+i])<<8 | int32(b[0xba+i])<<16
		if pos&0x800000 != 0 {
			pos -= 0x1000000
		}
		w.Combat.Players[i] = Player{Motion: PlayerMotion{Position: pos, Jump: b[0xa0+i], Velocity: int16(uint16(b[0xc4+i]) | uint16(b[0xc6+i])<<8)}, X: b[0x334+i], GameOver: b[0x38+i], State: b[0x90+i], SkipLedge: b[0xda+i], DisplaySprite: b[0x300+i], DisplayAttribute: b[0x34e+i]}
		w.Combat.Display[i] = LogicalSprite{Code: b[0x300+i], X: b[0x334+i], Y: b[0x31a+i], Attributes: b[0x34e+i]}
		in[i] = PlayerInput{Held: Buttons(b[0xf1+i]), Pressed: Buttons(b[0xf5+i])}
	}
	w.Presentation.Palette.FadeTimer = b[0x74]
	w.Presentation.Palette.RequestedColors = b[0x36]
	copy(w.Presentation.Palette.Desired[:], b[0x7c0:0x7e0])
	return s, c, w, in
}
func TestCampaignSceneOriginalExit(t *testing.T) {
	count := 0
	records := append(campaignSceneRecords(t, "exit"), campaignSceneRecords(t, "exit-cases")...)
	for _, r := range records {
		if r.Kind != "exit" {
			continue
		}
		before, after := campaignSceneRAM(t, r.Before), campaignSceneRAM(t, r.After)
		s, c, w, in := campaignExitFromRAM(before)
		wantS, wantC, wantW, wantIn := campaignExitFromRAM(after)
		if e := s.StepEndSequence(&c, &w, &in); e != nil {
			t.Fatalf("frame %d: %v", r.Frame, e)
		}
		if !reflect.DeepEqual(s, wantS) || c.EndSequence != wantC.EndSequence || !reflect.DeepEqual(w, wantW) || in != wantIn {
			t.Fatalf("frame %d sequence %d\nscene %v want %v\ncampaign %v want %v\nworld %v want %v\ninput %v want %v", r.Frame, before[0x2d], s, wantS, c, wantC, w, wantW, in, wantIn)
		}
		count++
	}
	if count != 1410 {
		t.Fatalf("incomplete original exit: %d", count)
	}
}
func TestCampaignSceneRejectsWithoutMutation(t *testing.T) {
	s := CampaignSceneState{ExitDelay: 23}
	c := Campaign{EndSequence: 4}
	w := ActiveWorld{}
	w.Combat.World = OutdoorWorld{GameRoutine: 5, LevelRoutine: 9}
	in := [2]PlayerInput{{Held: ButtonRight}}
	bs, bc, bw, bi := s, c, w, in
	if e := s.StepEndSequence(&c, &w, &in); e == nil {
		t.Fatal("accepted unsupported script phase")
	}
	if s != bs || c != bc || !reflect.DeepEqual(w, bw) || in != bi {
		t.Fatal("partial mutation")
	}
}

// Entrance failures: premature ready after first nametable; dropped map-half
// load at column 32; queue grouping/collision changed before display commit;
// failure halfway through a column or map load must leave all owners intact.
func campaignEntranceFromRAM(b []byte) ActiveWorld {
	w := ActiveWorld{}
	w.Combat.World = OutdoorWorld{GameRoutine: b[0x18], LevelRoutine: b[0x2c], Level: b[0x30], SceneType: b[0x40], ScrollType: b[0x41], Screen: b[0x64], ScreenScroll: b[0x65]}
	w.Terrain = TerrainScrollState{Axis: b[0x60], TransitionRemaining: b[0x61], TileAddress: uint16(b[0x62]) | uint16(b[0x63])<<8, AttributeAddress: uint16(b[0x66]) | uint16(b[0x67])<<8, MapOffset: b[0x69]}
	copy(w.Terrain.Supertiles[:], b[0x600:0x680])
	copy(w.Combat.Terrain[:], b[0x680:0x700])
	copy(w.Header.CollisionLimits[:], b[0x49:0x4c])
	copy(w.Header.CycleIndices[:], b[0x4c:0x50])
	copy(w.Header.PaletteIndices[:], b[0x50:0x58])
	w.Presentation.Queue.Used = b[0x21]
	return w
}
func campaignSceneResources(t *testing.T, stages ...int) *LevelResources {
	t.Helper()
	stage := 1
	if len(stages) > 0 {
		stage = stages[0]
	}
	raw, e := os.ReadFile(fmt.Sprintf("leveldata/stage-%d.json", stage))
	if e != nil {
		t.Fatal(e)
	}
	r, e := DecodeLevelResources(raw)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestCampaignSceneOriginalEntrance(t *testing.T) {
	res := campaignSceneResources(t)
	count := 0
	for _, r := range campaignSceneRecords(t, "natural") {
		if r.Kind != "entrance" {
			continue
		}
		b, a := campaignSceneRAM(t, r.Before), campaignSceneRAM(t, r.After)
		w, want := campaignEntranceFromRAM(b), campaignEntranceFromRAM(a)
		done, e := StepCampaignEntrance(&w, res)
		if e != nil {
			t.Fatalf("frame%d: %v", r.Frame, e)
		}
		if done != (r.P&2 != 0) || w.Combat != want.Combat || w.Terrain != want.Terrain || w.Header != want.Header {
			t.Fatalf("frame%d done%v/%v terrain%+v want%+v collision%x want%x", r.Frame, done, r.P&2 != 0, w.Terrain, want.Terrain, w.Combat.Terrain, want.Combat.Terrain)
		}
		raw := append([]uint8(nil), b[0x700:0x700+int(b[0x21])]...)
		for _, p := range w.Presentation.Queue.Packets {
			v, e := p.Encode()
			if e != nil {
				t.Fatal(e)
			}
			raw = append(raw, v...)
		}
		if !reflect.DeepEqual(raw, a[0x700:0x700+int(a[0x21])]) || w.Presentation.Queue.Used != a[0x21] {
			t.Fatalf("frame%d queue differs", r.Frame)
		}
		count++
	}
	if count != 48 {
		t.Fatalf("entrance must last 48 original columns, got%d", count)
	}
}
func TestCampaignSceneAllOriginalEntrancesContinuous(t *testing.T) {
	count := 0
	for stage := 1; stage <= 8; stage++ {
		kind := "natural"
		if stage > 1 {
			kind += fmt.Sprintf("-stage-%d", stage)
		}
		res := campaignSceneResources(t, stage)
		var w ActiveWorld
		started := false
		for _, r := range campaignSceneRecords(t, kind) {
			if r.Kind != "entrance" {
				continue
			}
			if !started {
				w = campaignEntranceFromRAM(campaignSceneRAM(t, r.Before))
				started = true
			}
			a := campaignSceneRAM(t, r.After)
			want := campaignEntranceFromRAM(a)
			done, e := StepCampaignEntrance(&w, res)
			if e != nil {
				t.Fatalf("stage%d frame%d: %v", stage, r.Frame, e)
			}
			if done != (r.P&2 != 0) || w.Terrain != want.Terrain || w.Combat != want.Combat {
				t.Fatalf("stage%d frame%d continuous entrance differs", stage, r.Frame)
			}
			w.Presentation.Queue.Reset()
			count++
		}
	}
	if count != 334 {
		t.Fatalf("incomplete all entrances: %d", count)
	}
}

func TestCampaignSceneEntranceFailureIsAtomic(t *testing.T) {
	var w ActiveWorld
	for _, r := range campaignSceneRecords(t, "natural") {
		if r.Kind == "entrance" {
			w = campaignEntranceFromRAM(campaignSceneRAM(t, r.Before))
			break
		}
	}
	w.Presentation.Queue.Used = 190
	before := w
	if _, e := StepCampaignEntrance(&w, campaignSceneResources(t)); e == nil {
		t.Fatal("overflow accepted")
	}
	if !reflect.DeepEqual(w, before) {
		t.Fatal("partial entrance write")
	}
}

func TestCampaignSceneExitFadeDoesNotOverwritePaletteFade(t *testing.T) {
	s := CampaignSceneState{ExitDelay: 1}
	c := Campaign{EndSequence: 2}
	w := ActiveWorld{}
	w.Combat.World = OutdoorWorld{GameRoutine: 5, LevelRoutine: 9, Level: 7}
	w.Presentation.Palette.FadeTimer = 7
	var in [2]PlayerInput
	if e := s.StepEndSequence(&c, &w, &in); e != nil {
		t.Fatal(e)
	}
	if w.Presentation.Palette.FadeTimer != 7 {
		t.Fatalf("FB exit fade overwrote 74 palette fade: %d", w.Presentation.Palette.FadeTimer)
	}
	w.Combat.World.Frame = 16
	if e := s.StepEndSequence(&c, &w, &in); e != nil {
		t.Fatal(e)
	}
	if w.Presentation.Palette.FadeTimer != 7 {
		t.Fatal("final-level fade decremented palette loader's counter")
	}
}
