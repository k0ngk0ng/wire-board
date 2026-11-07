package games

import "testing"

// A registered game without both hooks cannot fill seats or hand control back.
func TestAllIndependentGamesSupportComputers(t *testing.T) {
	for _, m := range Modules() {
		if m.BotAction == nil || m.Reclaim == nil {
			t.Errorf("%s must support computer seats and autoplay", m.ID)
		}
	}
}

// The platform registry attaches the current per-game controller definition.
func TestYuYuRegistrationRetainsMDController(t *testing.T) {
	for _, module := range Modules() {
		if module.ID == "yuyuhakusho" {
			if module.Controls == nil || module.Controls.Layout != "md-6" || module.Controls.LegacyProfile != "md-6" {
				t.Fatal("registered YuYu controller missing", module.Controls)
			}
			return
		}
	}
	t.Fatal("YuYu registration missing")
}
