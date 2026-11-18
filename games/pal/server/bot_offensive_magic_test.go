package pal

// Failure cases before removing global classic Force from bot decisions:
// one role's shortcut silently commits the whole party, preventing independent
// healing/cooperation; an unobserved or unavailable spell is cast; redraw lag
// confirms stale text; an empty/clamped menu loops; friendly magic is mistaken
// for attack; save/reclaim loses intent or confirms a human's old selection.
import (
	"encoding/json"
	"strings"
	"testing"
)

func offensiveView() View {
	return View{Phase: "battle", Party: []PartyMember{{Role: 0, Controller: 0, HP: 100, MaxHP: 100, MP: 100, MaxMP: 100}}, Battle: &BattleView{State: 1}}
}

func TestBattleBotOffenseUsesPerRoleOriginalMenu(t *testing.T) {
	v := offensiveView()
	m := &battleBotMemory{}
	a, ok := battleBotAction(v, 0, m)
	if !ok || a.Keys != 8 {
		t.Fatal("bot still uses global classic Force instead of one role's menu", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Selected = 1
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("one role's magic icon not confirmed", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Menu = 1
	for _, r := range "御剑术" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
	}
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("visible affordable offensive spell not selected", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Menu = 0
	v.Battle.State = 2
	v.Battle.Action = 3
	resetBattleBotInput(m)
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("saved original enemy target lost offensive intent", a)
	}
	clearBattleBotOwner(m, v, 0)
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 1 {
		t.Fatal("takeover confirms human magic", a)
	}
}

func TestBattleBotOffensiveScanAndMissingMagicAreBounded(t *testing.T) {
	v := offensiveView()
	m := &battleBotMemory{}
	rememberBattleBotInput(m, v, 8)
	v.Battle.Selected = 1
	for i := 0; i < 4; i++ {
		rememberBattleBotInput(m, v, 2)
	}
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 16 {
		t.Fatal("empty original offensive menu loops at its icon", a)
	}
	v = offensiveView()
	m = &battleBotMemory{}
	rememberBattleBotInput(m, v, 8)
	v.Battle.Menu = 1
	for _, r := range "天罡战气" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
	}
	a, ok := battleBotAction(v, 0, m)
	if !ok || a.Keys != 65536 {
		t.Fatal("unrelated buff is cast as an offensive spell", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	for i := 0; i < 2; i++ {
		rememberBattleBotInput(m, v, 0)
	}
	a, ok = battleBotAction(v, 0, m)
	if !ok || a.Keys != 32 {
		t.Fatal("rewound offensive scan did not inspect the next original spell", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	for i := 0; i < 2; i++ {
		a, ok = battleBotAction(v, 0, m)
		if !ok || a.Keys != 0 {
			t.Fatal("offensive scan confirms stale cursor", a)
		}
		rememberBattleBotInput(m, v, 0)
	}
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 1 {
		t.Fatal("clamped offensive scan did not cancel", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Menu = 0
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("missing magic does not fall back to ordinary attack", a)
	}
	b, _ := json.Marshal(m)
	if !strings.Contains(string(b), `"offenseFailed":true`) {
		t.Fatal("missing offensive menu was not remembered", string(b))
	}
}

func TestBattleBotOffensiveMemoryBounds(t *testing.T) {
	for _, payload := range []string{`{"role":0,"offenseAttempts":-1}`, `{"role":0,"offenseAttempts":5}`, `{"role":0,"offenseCheckedMP":-1}`, `{"role":0,"offenseCheckedMP":65536}`} {
		var m battleBotMemory
		if err := json.Unmarshal([]byte(`{"roles":[`+payload+`]}`), &m); err != nil {
			t.Fatal(err)
		}
		if validBattleBotMemory(&m) {
			t.Fatal("invalid offensive policy memory accepted", payload)
		}
	}
}
