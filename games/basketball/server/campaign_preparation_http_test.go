package basketball_test

import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Controlled halftime boundary, real authentication, module actions, timer and
// SQLite. Start alone is wrapped to install a naturally generated engine half;
// this does not claim an available campaign creation UI or a full HTTP match.
func TestCampaignPreparationHTTPPersistence(t *testing.T) {
	_, fixture := game.ExportCampaignHostHalftime(t)
	var saved game.State
	if err := json.Unmarshal(fixture.Data, &saved); err != nil {
		t.Fatal(err)
	}
	module := game.Module()
	originalStart := module.Start
	module.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		m, e := originalStart(ctx, raw)
		if e != nil {
			return m, e
		}
		s, e := game.ExportDecode(m)
		if e != nil {
			return m, e
		}
		s.Runtime = saved.Runtime
		s.Options = saved.Options
		return game.ExportStateMatch(s)
	}
	h := gametest.NewHTTP(t, 1, module, "basketball")
	h.Start(map[string]any{"mode": "solo", "court": "court-0"})
	a := h.People[0]
	read := func(c *gametest.Client) game.View {
		raw, e := json.Marshal(gametest.Data(c.Room()))
		if e != nil {
			t.Fatal(e)
		}
		var v game.View
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	v := read(a)
	if v.Preparation == nil || !v.CanPrepare {
		t.Fatal("missing campaign halftime")
	}
	epoch := v.Epoch
	edit := func(kind string, revision uint64, fields map[string]any) map[string]any {
		p := map[string]any{"type": kind, "period": 3, "revision": revision}
		for k, v := range fields {
			p[k] = v
		}
		return map[string]any{"type": "campaign-preparation", "epoch": epoch, "preparation": p}
	}
	remove := edit("unequip", 0, map[string]any{"character": 0, "slot": 0})
	h.Spectator.Action(remove, 400)
	a.Action(remove, 200)
	a.Action(remove, 400)
	a.Action(edit("equip", 1, map[string]any{"character": 1, "inventoryIndex": 0}), 200)
	accepted := read(a)
	if accepted.Preparation.Progress.Equipment[1][0] != 1 || accepted.Preparation.Revision != 2 {
		t.Fatal("transfer not visible")
	}
	observer := read(h.Spectator)
	if observer.CanPrepare || !reflect.DeepEqual(observer.Preparation, accepted.Preparation) {
		t.Fatal("observer preparation differs")
	}
	h.Record("controlled original halftime: authenticated transfer accepted, stale edit and spectator rejected")
	h.Restart()
	restored := read(a)
	if restored.Epoch != epoch || !reflect.DeepEqual(restored.Preparation, accepted.Preparation) {
		t.Fatal("SQLite restore lost accepted equipment")
	}
	a.Action(edit("teammates", 2, map[string]any{"teammates": []int{1, 0}}), 200)
	a.Action(edit("tactic", 3, map[string]any{"tactic": 2}), 200)
	v = read(a)
	a.Action(map[string]any{"type": "buttons", "epoch": epoch, "sequence": v.Inputs[0].Sequence + 1, "buttons": engine.ButtonStart}, 200)
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
		v = read(a)
		if v.Runtime.Playing != nil {
			if v.Preparation != nil || v.CanPrepare || v.Runtime.Config.Match.Characters != v.Options.Characters || v.Options.Characters[0] != 1 || v.Runtime.Config.Campaign.Equipment[1][0] != 1 || v.Runtime.Config.HumanTactics[0] != 2 {
				t.Fatal("Start did not commit preparation")
			}
			h.Record("actual Start and real-clock timer commit equipment, roster and tactic into second half")
			t.Logf("HTTP/SQLite preparation preserved; second-half frame=%d scores=%v", v.Runtime.VideoFrames, v.Scores)
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("second half did not start")
}
