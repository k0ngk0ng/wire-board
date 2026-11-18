package pokepath_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games"
)

func TestPokepathProductionRegistration(t *testing.T) {
	found := 0
	for _, module := range games.Modules() {
		if module.ID != "pokepath" {
			continue
		}
		found++
		if err := module.Validate(); err != nil {
			t.Fatal(err)
		}
		if module.Name != "宝可梦：巡径守卫" || module.MinPlayers != 1 || module.MaxPlayers != 4 {
			t.Fatalf("unexpected public metadata: %+v", module)
		}
		if module.BotAction == nil || module.Reclaim == nil || !module.SupportsSave {
			t.Fatal("cooperative game must support bots, reclaim and platform saves")
		}
	}
	if found != 1 {
		t.Fatalf("expected one production pokepath module, got %d", found)
	}
}
