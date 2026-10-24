package conquest

// Failure cases were recorded before regional-map implementation. The public
// module must preserve explicit existing maps while defaulting new rooms to world.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func TestConquestRegionalMapsAndDefault(t *testing.T) {
	m := Module()
	o, e := m.Configure(2, nil)
	if e != nil {
		t.Fatal(e)
	}
	var opts Options
	if e = json.Unmarshal(o, &opts); e != nil {
		t.Fatal(e)
	}
	if opts.MapID != "world" {
		t.Fatalf("new rooms default to %s, want world", opts.MapID)
	}
	for _, id := range []string{"world", "europe", "usa", "china"} {
		for n := 2; n <= 8; n++ {
			o, e := m.Configure(n, raw(map[string]any{"mapId": id}))
			if e != nil {
				t.Fatal(id, n, e)
			}
			c := gameapi.Context{Actor: 0, Players: n, Now: time.Unix(1792800000, 0)}
			match, e := m.Start(c, o)
			if e != nil {
				t.Fatal(id, n, e)
			}
			v, e := m.View(match, 0)
			if e != nil {
				t.Fatal(e)
			}
			view := v.(View)
			if view.MapID != id || view.MapSHA256 == "" || len(view.Players) != n {
				t.Fatal("wrong map/view", id, n)
			}
			restored, e := m.Restore(c, match)
			if e != nil {
				t.Fatal(e)
			}
			if string(restored.Data) != string(match.Data) {
				t.Fatal("restore changed map", id, n)
			}
		}
	}
}
