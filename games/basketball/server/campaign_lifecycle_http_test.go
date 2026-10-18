package basketball

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Real HTTP/SQLite lifecycle, starting at a controlled, naturally completed Go
// match boundary. No result, clock, AI or inventory is fabricated by this test.
// The start wrapper is explicit: this is not a full natural HTTP campaign.
func TestCampaignLifecycleHTTP(t *testing.T) {
	_, finished := campaignFinishedHost(t, false)
	var checkpoint State
	if e := json.Unmarshal(finished.Data, &checkpoint); e != nil {
		t.Fatal(e)
	}
	m := Module()
	originalStart := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		initial, e := originalStart(ctx, raw)
		if e != nil {
			return initial, e
		}
		s, e := decode(initial)
		if e != nil {
			return initial, e
		}
		s.Runtime = checkpoint.Runtime
		s.Options = checkpoint.Options
		c := *checkpoint.Campaign
		c.DecisionAt = s.ServerAt + 3000
		s.Campaign = &c
		return s.match()
	}
	h := gametest.NewHTTP(t, 1, m, "basketball")
	h.Start(map[string]any{"mode": "solo", "campaign": map[string]any{"password": "020010", "opponent": 1}})
	a := h.People[0]
	view := func(c *gametest.Client) View {
		raw, _ := json.Marshal(gametest.Data(c.Room()))
		var v View
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	v := view(a)
	epoch := v.Epoch
	if a.Room()["status"] != "playing" || v.Campaign.Stage != "reward" || v.Campaign.Settlement.Reward != 0 || v.CanInput || !v.Campaign.CanAct {
		t.Fatal("intermediate victory prematurely finished room")
	}
	send := func(kind string, fields map[string]any) map[string]any {
		v := view(a)
		p := map[string]any{"type": kind, "revision": v.Campaign.Revision}
		for k, val := range fields {
			p[k] = val
		}
		return map[string]any{"type": "campaign", "epoch": v.Epoch, "campaign": p}
	}
	want := *v.Campaign.Settlement
	h.Restart()
	v = view(a)
	if v.Epoch != epoch || !reflect.DeepEqual(*v.Campaign.Settlement, want) {
		t.Fatal("restored reward changed")
	}
	h.Spectator.Action(send("continue", nil), 400)
	a.Action(send("continue", nil), 200)
	a.Action(send("equip", map[string]any{"character": 1, "inventoryIndex": 0}), 200)
	a.Action(send("opponent", map[string]any{"opponent": 4}), 200)
	prepared := view(a)
	h.Restart()
	v = view(a)
	if !reflect.DeepEqual(v.Campaign.Preparation, prepared.Campaign.Preparation) {
		t.Fatal("next preparation lost on restart")
	}
	h.Record("real result boundary remains active; reward and accepted next-round loadout survive independent SQLite restarts")
	// The platform owns autoplay, while recovered AI still owns basketball.
	// Let the old menu deadline expire while a human is still choosing. A new
	// handoff must publish its own countdown before confirming that choice.
	time.Sleep(3200 * time.Millisecond)
	a.Command("autoplay", map[string]any{"enabled": true}, 200)
	for deadline := time.Now().Add(time.Second); ; {
		v = view(a)
		if v.Epoch != epoch {
			t.Fatal("autoplay skipped the announced preparation countdown")
		}
		if v.Inputs[0].Automated {
			if v.Campaign.AutomaticAt-v.ServerAt < 2000 {
				t.Fatal("handoff did not provide a fresh visible deadline")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("autoplay metadata did not synchronize")
		}
		time.Sleep(20 * time.Millisecond)
	}
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
		v = view(a)
		if v.Epoch != epoch && v.Runtime.Playing != nil {
			if v.Campaign.Stage != "playing" || v.Campaign.Round != 2 || v.Runtime.Config.Campaign.Period != 4 || v.Runtime.Config.Campaign.Equipment[1][0] != 0 || v.Runtime.Config.Match.Court != 4 || !v.Inputs[0].Automated {
				t.Fatal("autoplay did not preserve next-round preparation")
			}
			a.Command("autoplay", map[string]any{"enabled": false}, 200)
			v = view(a)
			if v.Inputs[0].Automated {
				t.Fatal("reclaim failed in next round")
			}
			h.Record("platform autoplay confirms prepared next match; new epoch resets transport and human reclaims original controller")
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("automatic menu confirmation did not enter the next round")
}
