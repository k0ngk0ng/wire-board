package gameapi

// Failure cases: a save-only pause hook registers without save support; optional
// hooks change ordinary modules; realtime saved resume remains mandatory.
import "testing"

func TestSaveControlCapability(t *testing.T) {
	m := pausedClockModule()
	m.OnPausedTimer = nil
	m.SaveControl = func(_ Context, g Match, _ bool) (Match, error) { return g, nil }
	if m.Validate() == nil {
		t.Fatal("save control registered without save support")
	}
	m.SupportsSave = true
	if m.Validate() == nil {
		t.Fatal("realtime save registered without resume")
	}
	m.ResumeSaved = func(_ Context, g Match) (Match, error) { return g, nil }
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	m.SaveControl, m.ResumeSaved, m.SupportsSave = nil, nil, false
	if err := m.Validate(); err != nil {
		t.Fatal("ordinary module changed", err)
	}
}
