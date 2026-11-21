package games

import "testing"

func TestCS16RegistrationSupportsTenPlayers(t *testing.T) {
	for _, module := range Modules() {
		if module.ID != "cs16" {
			continue
		}
		if module.MinPlayers != 2 || module.MaxPlayers < 10 || module.BotAction == nil || module.Reclaim == nil || module.Controls == nil || module.Controls.Layout != "game" {
			t.Fatal("CS 1.6 registration incomplete", module.MaxPlayers)
		}
		return
	}
	t.Fatal("CS 1.6 registration missing")
}
