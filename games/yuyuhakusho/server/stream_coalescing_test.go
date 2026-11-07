package yuyuhakusho

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
)

// Actual delayed four-browser evidence: UI last presented874; incoming881 and
// 883 were coalesced by React; a six-frame overlap on883 started876 and lost875.
// Cover one bounded prediction window, retaining the full recovery ring apart.
func TestStreamInputOverlapSurvivesBatchedFourPlayerViews(t *testing.T) {
	module, ctx, match := moduleStartTest(t, 4, Options{Characters: []uint8{3, 1, 0, 10}}, make([]bool, 4))
	epoch := moduleViewTest(t, module, match, 0).Epoch
	for seat := 0; seat < 4; seat++ {
		ctx.Actor = seat
		match = moduleActionTest(t, module, ctx, match, map[string]any{"type": "loaded", "epoch": epoch})
	}
	var sequences [4]int
	for tick := 1; tick <= 80; tick++ {
		seat := tick % 4
		sequences[seat]++
		ctx.Actor = seat
		ctx.Now = time.UnixMilli(int64(1000 + tick*17))
		match = moduleActionTest(t, module, ctx, match, map[string]any{"type": "input", "epoch": epoch, "sequence": sequences[seat], "buttons": engine.Buttons((tick % 5) << 4)})
		match = moduleTickTest(t, module, ctx, match, ctx.Now.UnixMilli())
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	for skipped := uint64(9); skipped <= 20; skipped++ {
		presented := state.Frame - skipped
		view, _ := streamDataTest(t, module, match, 3, fmt.Sprintf("%s:%d", epoch, state.Frame-2))
		frames, err := syncwire.Decode(view.Replay)
		if err != nil || view.Resync || len(frames) == 0 || frames[0].Frame > presented+1 {
			t.Fatalf("coalesced UI lost frame %d: window %d/%d (%v)", presented+1, view.Replay.FirstFrame, view.Replay.FrameCount, err)
		}
		if len(frames) > 22 {
			t.Fatal("sent more than bounded overlap", len(frames))
		}
	}
}
