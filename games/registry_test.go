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
