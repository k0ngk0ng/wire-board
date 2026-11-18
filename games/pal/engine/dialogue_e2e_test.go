package engine_test

// Before Go original-dialogue projection: raw message yields remain the shared
// read boundary; shadow glyphs/portraits disappear; UI/page memory does not
// survive fresh-instance restore; a timer or zero acknowledgement skips a page;
// timed dream text gains a manual confirmation or is rendered as control codes.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestNativeOriginalDialoguePages(t *testing.T) {
	r := nativeRuntime(t)
	data, e := os.ReadFile(filepath.Join("..", "resources", "testdata", "inn-dialogue-pages.json"))
	if e != nil {
		t.Fatal(e)
	}
	var expected struct {
		Messages uint32 `json:"messages"`
		Pages    []struct {
			Scene int      `json:"scene"`
			IDs   []int    `json:"ids"`
			Text  []string `json:"text"`
			Lines int      `json:"lines"`
		} `json:"pages"`
	}
	if e = json.Unmarshal(data, &expected); e != nil {
		t.Fatal(e)
	}
	s, e := r.StartCampaign(context.Background(), 12345)
	if e != nil {
		t.Fatal(e)
	}
	wait := uint32(s.Event.Value)
	pages := 0
	timedDream := false
	for i := 0; i < 4096; i++ {
		if s.View.Scene == 1 && s.View.Messages == 1 {
			for _, op := range s.View.Overlay {
				if op.Kind == 0 && op.ID == 0x674e && op.X == 80 && op.Y == 40 && op.Color == 0x4f {
					timedDream = true
				}
			}
			if s.Event.Kind == engine.Dialogue {
				t.Fatal("original timed dream line requires an extra confirmation")
			}
		}
		if s.Event.Kind == engine.Dialogue {
			if pages >= len(expected.Pages) {
				t.Fatal("extra original page")
			}
			page := expected.Pages[pages]
			v, e := r.View(s)
			if e != nil {
				t.Fatal(e)
			}
			if !s.View.DialogWaiting || s.View.Scene != page.Scene || s.View.DialogLine != page.Lines || !reflect.DeepEqual(s.View.DialogPage, page.IDs) || v.Text != strings.Join(page.Text, "\n") {
				t.Fatal("page differs from original C reference", pages, s.View.DialogPage, page.IDs, v.Text)
			}
			if pages == 0 {
				if !reflect.DeepEqual(page.IDs, []int{1317, 1318, 1319}) {
					t.Fatal("original first inn page changed")
				}
				unchanged, e := r.Step(context.Background(), s, 0)
				if e != nil || !reflect.DeepEqual(unchanged.View, s.View) || unchanged.Event != s.Event {
					t.Fatal("zero acknowledgement skipped shared page", e)
				}
				encoded, e := r.Encode(s)
				if e != nil {
					t.Fatal(e)
				}
				fresh := nativeRuntime(t)
				restored, e := fresh.Decode(encoded)
				if e != nil {
					t.Fatal(e)
				}
				a, e := r.Step(context.Background(), s, 1)
				if e != nil {
					t.Fatal(e)
				}
				b, e := fresh.Step(context.Background(), restored, 1)
				if e != nil || !reflect.DeepEqual(a, b) {
					t.Fatal("fresh original-page continuation changed", e)
				}
			}
			pages++
			s, e = r.Step(context.Background(), s, 1)
			if e != nil {
				t.Fatal(e)
			}
			wait = uint32(s.Event.Value)
			continue
		}
		if s.View.Scene == 2 && s.View.Mode == engine.FieldMode && !s.View.Entering && s.Event.Kind == engine.Frame {
			break
		}
		advanced, e := r.AdvanceTimed(context.Background(), s, engine.Input{Direction: 4}, wait, 250)
		if e != nil {
			t.Fatal(e)
		}
		s, wait = advanced.Snapshot, advanced.RemainingMS
	}
	if !timedDream || pages != 23 || s.View.Messages != expected.Messages || s.View.Messages != 72 || s.View.Scene != 2 || s.View.Mode != engine.FieldMode {
		t.Fatal("original dialogue campaign did not complete", timedDream, pages, s.View.Messages)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "scope": "real original GameMain and text/UI C bodies; 72 messages, 23 original pages, timed dream, title/portrait observations, no-ack freeze and compressed independent-runtime continuation", "pages": pages, "messages": s.View.Messages}, "", "  ")
	if e = os.WriteFile(filepath.Join("..", "..", "..", ".local", "pal", "verification", "native-dialogue-go.json"), report, 0600); e != nil {
		t.Fatal(e)
	}
}
