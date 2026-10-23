package conquest_test

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/games"
)

// Before registration: a correct isolated module can still be absent from the
// production composition root, accept an unknown map, or omit bot support.
func TestConquestProductionRegistration(t *testing.T) {
	count := 0
	for _, m := range games.Modules() {
		if m.ID != "conquest" {
			continue
		}
		count++
		if m.Name != "征服时代" || m.MinPlayers != 2 || m.MaxPlayers != 8 || m.BotAction == nil || m.Reclaim == nil {
			t.Fatal("incomplete conquest production registration")
		}
		for _, id := range []string{"world", "europe"} {
			if _, e := m.Configure(4, json.RawMessage(`{"mapId":"`+id+`"}`)); e != nil {
				t.Fatal(id, e)
			}
		}
		if _, e := m.Configure(4, json.RawMessage(`{"mapId":"missing"}`)); e == nil {
			t.Fatal("unknown map accepted")
		}
	}
	if count != 1 {
		t.Fatalf("want one production conquest module, found %d", count)
	}
}
