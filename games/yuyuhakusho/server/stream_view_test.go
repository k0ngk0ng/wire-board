package yuyuhakusho

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func streamDataTest(t *testing.T, module gameapi.Module, match gameapi.Match, seat int, cursor string) (View, string) {
	t.Helper()
	if module.StreamView == nil {
		t.Fatal("missing per-connection input projection")
	}
	before := append([]byte(nil), match.Data...)
	result, err := module.StreamView(match, seat, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, match.Data) {
		t.Fatal("stream projection modified persistent state")
	}
	view, ok := result.Data.(View)
	if !ok {
		t.Fatal("stream changed existing view type")
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"world"`, `"presentation"`, `"checkpoint"`, `"sprite_slots"`, `"Objects"`, `"Stream"`, `"hud"`} {
		if bytes.Contains(raw, []byte(key)) {
			t.Fatal("stream contains checkpoint/render history", key)
		}
	}
	return view, result.Cursor
}

func TestStreamDeltaCursorsKeepRecoveryRingAndMetadata(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprintf("seats-%d", n), func(t *testing.T) {
			characters := []uint8{3, 9, 7, 1}[:n]
			module, context, match := moduleStartTest(t, n, Options{Characters: characters}, make([]bool, n))
			epoch := moduleViewTest(t, module, match, 0).Epoch
			for seat := 0; seat < n; seat++ {
				context.Actor = seat
				match = moduleActionTest(t, module, context, match, map[string]any{"type": "loaded", "epoch": epoch})
			}
			initial, cursor := streamDataTest(t, module, match, 0, "")
			if !initial.Resync || initial.Replay.FrameCount != 0 || cursor != epoch+":0" {
				t.Fatal("new connection did not explicitly recover", initial, cursor)
			}
			cursorBefore := cursor
			for tick := 1; tick <= 160; tick++ {
				context.Actor = 0
				context.Now = time.UnixMilli(int64(1000 + tick*17))
				match = moduleActionTest(t, module, context, match, map[string]any{"type": "input", "epoch": epoch, "sequence": tick, "buttons": engine.Buttons(1 << uint(tick%12))})
				match = moduleTickTest(t, module, context, match, context.Now.UnixMilli())
				view, next := streamDataTest(t, module, match, 0, cursor)
				if view.Resync || view.Replay.FrameCount > 22 || view.Frame < initial.Frame {
					t.Fatal("regular stream repeats full recovery ring", view.Replay, view.Resync)
				}
				raw, _ := json.Marshal(view)
				if len(raw) > 2048 {
					t.Fatal("incremental game view exceeded frozen budget", len(raw))
				}
				public, err := module.View(match, 0)
				if err != nil {
					t.Fatal(err)
				}
				original := public.(View)
				window, err := syncwire.Decode(view.Replay)
				if err != nil {
					t.Fatal(err)
				}
				full, err := syncwire.Decode(original.Replay)
				if err != nil {
					t.Fatal(err)
				}
				if len(window) > 0 && !reflect.DeepEqual(window, full[len(full)-len(window):]) {
					t.Fatal("delta differs from retained authoritative inputs")
				}
				view.Replay = original.Replay
				if !reflect.DeepEqual(view, original) {
					t.Fatal("stream changed public metadata or digests")
				}
				cursor = next
			}
			public, _ := module.View(match, 0)
			if public.(View).Replay.FrameCount != 120 {
				t.Fatal("incremental send trimmed persistent recovery ring")
			}
			stable, same := streamDataTest(t, module, match, 0, cursor)
			if stable.Resync || stable.Replay != (syncwire.InputWindow{Version: 1}) || same != cursor {
				t.Fatal("same-frame stream replays redundant old inputs")
			}
			late, _ := streamDataTest(t, module, match, n-1, cursorBefore)
			if !late.Resync || late.Replay.FrameCount != 0 || late.LocalSeat != n-1 {
				t.Fatal("lagged connection did not recover independently")
			}
			again, _ := streamDataTest(t, module, match, 0, cursor)
			if again.Resync || again.Replay.FrameCount != 0 {
				t.Fatal("lagged viewer changed fast connection cursor")
			}
		})
	}
}

func TestStreamCursorGapCanonicalAndUint64Boundaries(t *testing.T) {
	module, context, match := moduleStartTest(t, 2, Options{Characters: []uint8{3, 9}}, []bool{true, true})
	for tick := 1; tick <= 50; tick++ {
		match = moduleTickTest(t, module, context, match, int64(1000+tick*50))
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the exact 16-bit world clock relationship while exercising a
	// network counter which JavaScript cannot represent as an integer number.
	offset := uint64(1) << 54
	state.Frame += offset
	state.World.Clock.Frame = uint16(state.Frame)
	for i := range state.History {
		state.History[i].Frame += offset
	}
	match, err = state.match()
	if err != nil {
		t.Fatal(err)
	}
	first := state.History[0].Frame
	for _, cursor := range []string{"", "bad", strings.Repeat("f", 24) + ":1", state.Epoch + ":01", state.Epoch + ":+1", state.Epoch + ":18446744073709551616", state.Epoch + ":" + strconv.FormatUint(state.Frame+1, 10), state.Epoch + ":" + strconv.FormatUint(first-2, 10)} {
		view, _ := streamDataTest(t, module, match, -1, cursor)
		if !view.Resync || view.Replay.FrameCount != 0 {
			t.Fatal("invalid/gapped cursor guessed missing inputs", cursor)
		}
	}
	view, cursor := streamDataTest(t, module, match, -1, state.Epoch+":"+strconv.FormatUint(first-1, 10))
	if view.Resync || view.Replay.FrameCount != 120 || cursor != state.Epoch+":"+strconv.FormatUint(state.Frame, 10) {
		t.Fatal("exact window boundary or uint64 cursor lost", view.Replay, cursor)
	}
	if _, err = module.StreamView(match, 2, cursor); err == nil {
		t.Fatal("unauthorized viewer accepted")
	}
}
