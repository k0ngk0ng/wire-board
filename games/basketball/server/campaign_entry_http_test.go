package basketball_test

// Before preparation implementation: reject unearned items/masks/reward tables;
// preserve source inventory order when moving equipment; keep equipment tied
// to four roster slots; prevent partner edits and unauthorized team edits;
// persist selections through actual SQLite and use them at Start, not defaults.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/basketball/campaign"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestCampaignEntryPreparationHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 1, game.Module(), "basketball")
	a := h.People[0]
	a.Request("POST", "/api/admin/games/basketball", map[string]bool{"enabled": true}, 200)
	a.Request("POST", "/api/rooms", map[string]any{"kind": "basketball", "capacity": 1, "name": "战役准备", "options": map[string]any{"mode": "solo", "campaign": map[string]any{"password": "020010", "opponent": 4}}}, 201)
	read := func() game.Options {
		raw, _ := json.Marshal(a.Room()["options"])
		var o game.Options
		if e := json.Unmarshal(raw, &o); e != nil {
			t.Fatal(e)
		}
		return o
	}
	o := read()
	if o.Campaign.Preparation.Inventory[0] != 1 || o.Court != "court-0" {
		t.Fatal("password preview missing")
	}
	a.Command("prepare", map[string]any{"action": map[string]any{"type": "equip", "character": 2, "inventoryIndex": 0}}, 200)
	a.Command("prepare", map[string]any{"action": map[string]any{"type": "character", "actor": 0, "character": 2}}, 200)
	a.Command("prepare", map[string]any{"action": map[string]any{"type": "tactic", "tactic": 2}}, 200)
	a.Command("prepare", map[string]any{"action": map[string]any{"type": "opponent", "opponent": 5}}, 200)
	o = read()
	if o.Campaign.Preparation.Equipment[2][0] != 1 || o.Campaign.Preparation.Inventory[0] != campaign.Empty || o.Characters[0] != 2 || o.Teams[1] != 5 || o.Campaign.Tactic != 2 {
		t.Fatal("preparation choices not retained", o)
	}
	h.Restart()
	restored := read()
	if !reflect.DeepEqual(o, restored) {
		t.Fatal("restart changed accepted preparation")
	}
	a.Command("ready", nil, 200)
	a.Command("start", nil, 200)
	v := snapshotView(t, a)
	if v.Runtime.Config.Match.Court != 0 || v.Runtime.Config.Match.Teams != ([2]uint8{0, 5}) || v.Runtime.Config.Match.Characters[0] != 2 || v.Runtime.Config.Match.Equipment[2][0] != 1 || v.Runtime.Config.HumanTactics[0] != 2 || v.Runtime.Config.Campaign.Period != 2 {
		t.Fatal("campaign Start lost waiting-room selections")
	}
	h.Record("password entry, original owned equipment and visible roster choices survive SQLite and initialize actual campaign")
}

func TestCampaignEntryRejectsForgedPreparation(t *testing.T) {
	raw, e := game.Module().Configure(1, json.RawMessage(`{"mode":"solo","campaign":{"password":"020010","opponent":1}}`))
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"inventory", "equipment", "reward", "wins", "mode"} {
		var o game.Options
		json.Unmarshal(raw, &o)
		switch kind {
		case "inventory":
			o.Campaign.Preparation.Inventory[0] = 31
		case "equipment":
			o.Campaign.Preparation.Equipment[2][0] = 1
		case "reward":
			o.Campaign.Preparation.RewardSet = 3
		case "wins":
			o.Campaign.Preparation.WinBits = [2]uint8{0, 0}
			o.Campaign.Preparation.Period = 0
		case "mode":
			o.Campaign.Preparation.Mode = 1
		}
		bad, _ := json.Marshal(o)
		if _, e = game.Module().Configure(1, bad); e == nil {
			t.Fatal("forged preparation accepted", kind)
		}
	}
}

// Before explicit entry intent: waiting-room options are saved by a button,
// not a native form submission. An empty password must not silently become a
// new campaign merely because the browser's required field was bypassed.
func TestCampaignExplicitEntryIntent(t *testing.T) {
	for _, raw := range []string{
		`{"mode":"solo","campaign":{"entry":"new"}}`,
		`{"mode":"solo","campaign":{"entry":"password","password":"020010"}}`,
	} {
		if _, err := game.Module().Configure(1, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{
		`{"mode":"solo","campaign":{"entry":"password","password":""}}`,
		`{"mode":"solo","campaign":{"entry":"new","password":"020010"}}`,
		`{"mode":"solo","campaign":{"entry":"resume"}}`,
	} {
		if _, err := game.Module().Configure(1, json.RawMessage(raw)); err == nil {
			t.Fatal("ambiguous campaign entry accepted", raw)
		}
	}
}
