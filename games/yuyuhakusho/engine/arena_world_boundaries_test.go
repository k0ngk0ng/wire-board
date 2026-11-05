package engine

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

// Existing complete-CPU callback captures additionally exercise the world
// boundary with actual dirty/gated/pan inputs. They are not new rule oracles.
func arenaWorldFromSceneTest(t *testing.T, scene ArenaSceneState, view DefaultArenaView, palette ArenaPaletteState) ChuWorldState {
	t.Helper()
	state := ChuWorldState{Objects: WorldObjects{scene.Object.ObjectID: scene.Object}, Camera: scene.Camera,
		ScrollMode: scene.Mode, BackdropColor: scene.BackdropColor, Vertical: scene.Vertical,
		ScrollSources: [2]int32{scene.SourceA, scene.SourceB}, Palette: palette, Stream: make([]byte, 1024),
		Clock: WorldClockState{PauseFlags: view.PauseFlags, SlowFlag: view.SlowFlag, ScreenShake: uint16(view.Shake), PanOffset: uint16(view.Pan), LogicFrame: uint16(view.FrameCounter)}}
	for i := range state.Stream {
		state.Stream[i] = 0xa5
	}
	if err := state.storePanRows(scene.PanA, scene.PanB); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 256; i++ {
		state.Cells[i], state.Cells[256+i], state.Cells[512+i] = uint16(view.HUDPrefix[i]), uint16(scene.RowsA[i]), uint16(scene.RowsB[i])
	}
	return state
}

func arenaWorldSceneTest(state ChuWorldState) ArenaSceneState {
	scene := ArenaSceneState{Object: state.Objects[0x8180], Camera: state.Camera, Mode: state.ScrollMode, BackdropColor: state.BackdropColor,
		Vertical: state.Vertical, PanA: state.PanRows(0), PanB: state.PanRows(1), SourceA: state.ScrollSources[0], SourceB: state.ScrollSources[1]}
	for i := range scene.RowsA {
		scene.RowsA[i], scene.RowsB[i] = int16(state.Cells[256+i]), int16(state.Cells[512+i])
	}
	return scene
}

func arenaWorldCheckSceneTest(t *testing.T, name string, state ChuWorldState, want ArenaSceneState, library ChuWorldLibrary) (ChuWorldState, []ChuWorldEvent) {
	t.Helper()
	got, events, err := StepChuWorldObjects(state, ChuWorldView{}, library)
	if err != nil || !reflect.DeepEqual(arenaWorldSceneTest(got), want) {
		t.Fatalf("world scene merge differs from actual CPU boundary %s: %v\ngot=%+v\nwant=%+v", name, err, arenaWorldSceneTest(got), want)
	}
	for _, span := range [][2]int{{448, 512}, {960, 1024}} {
		if !reflect.DeepEqual(got.Stream[span[0]:span[1]], state.Stream[span[0]:span[1]]) {
			t.Fatal("world feedback overwrote Stream's original gap bytes", name)
		}
	}
	for i := 0; i < 256; i++ {
		if got.Cells[i] != state.Cells[i] {
			t.Fatal("world arena overwrote its read-only HUD prefix", name)
		}
	}
	return got, events
}

func TestArenaWorldCPUFeedbackBoundaries268(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range []uint16{2, 6, 8} {
		count, feedback, gates := 0, 0, 0
		walkArena268(t, fmt.Sprintf("arena%x-boundaries", arena), func(_ ArenaSceneLibrary, row arena268Call) {
			before, view := decodeArena268(t, row.Before)
			want, _ := decodeArena268(t, row.After)
			state := arenaWorldFromSceneTest(t, before, view, ArenaPaletteState{})
			if row.ReferenceOnly != "" {
				got, events, err := StepChuWorldObjects(state, ChuWorldView{}, library)
				if err == nil || events != nil || !reflect.DeepEqual(got, state) {
					t.Fatal("declared unrepresented CPU feedback did not fail atomically", row.Name)
				}
				count++
				return
			}
			arenaWorldCheckSceneTest(t, row.Name, state, want, library)
			count++
			if view.Pan != 0 {
				feedback++
			}
			if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
				gates++
			}
		}, nil)
		if count == 0 || feedback == 0 || gates == 0 {
			t.Fatal("incomplete actual CPU world-boundary coverage", arena, count, feedback, gates)
		}
		t.Logf("arena=%x CPU boundaries=%d pan=%d gated=%d", arena, count, feedback, gates)
	}
}

func TestArenaWorldCPUFeedbackBoundaries4CE(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	for _, arena := range []uint16{4, 12, 14} {
		path := fmt.Sprintf("testdata/arenas-4-c-e-%x-boundaries.jsonl.gz", arena)
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			t.Fatal(err)
		}
		d := json.NewDecoder(z)
		count, feedback, gates, oddFrames := 0, 0, 0, 0
		for {
			var row arena4CECall
			if err := d.Decode(&row); err == io.EOF {
				break
			} else if err != nil {
				t.Fatal(err)
			}
			if row.Kind != "arena" {
				continue
			}
			before, view, palette := decodeArena4CE(t, row.Before)
			want, _, wantPalette := decodeArena4CE(t, row.After)
			got, _ := arenaWorldCheckSceneTest(t, row.Name, arenaWorldFromSceneTest(t, before, view, palette), want, library)
			if got.Palette != wantPalette {
				t.Fatal("world arena palette merge differs from actual CPU", arena, row.Name)
			}
			count++
			if view.Pan != 0 {
				feedback++
			}
			if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
				gates++
			}
			if view.FrameCounter&1 != 0 {
				oddFrames++
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if count == 0 || feedback == 0 || gates == 0 || arena == 12 && oddFrames == 0 {
			t.Fatal("incomplete actual CPU world-boundary coverage", arena, count, feedback, gates, oddFrames)
		}
		t.Logf("arena=%x CPU boundaries=%d pan=%d gated=%d odd-frame=%d", arena, count, feedback, gates, oddFrames)
	}
}

func TestArenaWorldCPUFeedbackBoundariesA(t *testing.T) {
	library := arenaWorldLibraryTest(t)
	var rows []arenaAOriginalCall
	arenaAReadFixture(t, "arena-a-boundaries", &rows)
	count, feedback, gates := 0, 0, 0
	for _, row := range rows {
		if row.Kind != "arena" {
			continue
		}
		before, child, view := decodeArenaATest(t, row.Before)
		want, wantChild, _ := decodeArenaATest(t, row.After)
		state := arenaWorldFromSceneTest(t, before, view, ArenaPaletteState{})
		state.Objects[child.ObjectID] = child
		// Parent callback fixtures stop before the same-pass child executes.
		// Keep the child slot inert for this parent-only boundary comparison.
		state.Objects[child.ObjectID] = WorldObject{ObjectID: child.ObjectID}
		if before.Object.Header.Stage == 0 && view.PauseFlags&128 == 0 && view.SlowFlag == 0 {
			// Initialization creates the child, so this boundary is verified by
			// the fresh-session same-pass E2E and its six descriptor assertions.
			continue
		}
		got, _ := arenaWorldCheckSceneTest(t, row.Name, state, want, library)
		_ = wantChild
		_ = got
		count++
		if view.Pan != 0 {
			feedback++
		}
		if view.PauseFlags&128 != 0 || view.SlowFlag != 0 {
			gates++
		}
	}
	if count == 0 || feedback == 0 || gates == 0 {
		t.Fatal("incomplete actual CPU arena-A parent world-boundary coverage", count, feedback, gates)
	}
	// The child's dirty and gated CPU returns have six ordered descriptors.
	for _, row := range rows {
		if row.Kind != "child" {
			continue
		}
		parent, child, view := decodeArenaATest(t, row.Before)
		_, wantChild, _ := decodeArenaATest(t, row.After)
		state := arenaWorldFromSceneTest(t, parent, view, ArenaPaletteState{})
		state.Objects = WorldObjects{child.ObjectID: child}
		// Parent stays present as data without running its own callback.
		parent.Object.Header.Behavior = 0
		state.Objects[parent.Object.ObjectID] = parent.Object
		got, events, err := StepChuWorldObjects(state, ChuWorldView{}, library)
		if err != nil || got.Objects[child.ObjectID] != wantChild {
			t.Fatal("arena A child canonical merge differs from actual CPU", row.Name, err)
		}
		var want []ArenaASprite
		for _, draw := range row.Draws {
			b, err := hex.DecodeString(draw.Descriptor)
			if err != nil || len(b) != 10 {
				t.Fatal("invalid original raw sprite descriptor")
			}
			want = append(want, ArenaASprite{Y: binary.BigEndian.Uint16(b), SizeLink: binary.BigEndian.Uint16(b[2:]), Tile: binary.BigEndian.Uint16(b[4:]), X: binary.BigEndian.Uint16(b[6:])})
		}
		compareArenaWorldSpritesTest(t, arenaWorldOriginalPassTest{RawArenaSprites: want}, events)
	}
	t.Logf("arena=a CPU parent boundaries=%d pan=%d gated=%d", count, feedback, gates)
}
