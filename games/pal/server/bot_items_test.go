package pal

// Failure cases before battle supplies: low-mana injured roles ignore earned
// medicines; unavailable/quest items are consumed; a retained inventory cursor
// at the end never searches earlier supplies; redraw lag confirms stale text;
// a clamped/no-supply list loops; the wrong original role is targeted after
// reorder; saved selection/reservation is lost; reclaim confirms a human item;
// malformed item intent or impossible persisted input bounds are accepted.
import (
	"encoding/json"
	"testing"
)

func itemView() View {
	return View{Phase: "battle", Party: []PartyMember{{Role: 0, Controller: 0, HP: 60, MaxHP: 100, MP: 10, MaxMP: 100}, {Role: 2, Controller: 0, HP: 30, MaxHP: 100, MP: 50, MaxMP: 100}}, Battle: &BattleView{State: 1, Current: 0}}
}
func itemMemory() *battleBotMemory { return &battleBotMemory{Roles: []battleBotRole{{Role: 0}}} }

func TestBattleBotOriginalSuppliesAndSavedTarget(t *testing.T) {
	v := itemView()
	m := itemMemory()
	a, ok := battleBotAction(v, 0, m)
	if !ok || a.Keys != 2048 {
		t.Fatal("urgent low-mana party did not open original supplies", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Menu = 2
	for _, r := range "蜂王蜜" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
	}
	for _, r := range "蜜蜂所酿最好的蜜。HPMP+150" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 31))
	}
	a, ok = battleBotAction(v, 0, m)
	if !ok || a.Keys != 65536 {
		t.Fatal("supply scan did not return to the original inventory start", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	for i := 0; i < 2; i++ {
		a, ok = battleBotAction(v, 0, m)
		if !ok || a.Keys != 0 {
			t.Fatal("supply scan accepted stale glyphs", a)
		}
		rememberBattleBotInput(m, v, 0)
	}
	a, ok = battleBotAction(v, 0, m)
	if !ok || a.Keys != 2 {
		t.Fatal("visible enabled recovery supply was not selected", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Menu = 0
	v.Battle.State = 3
	v.Battle.Action = 7
	v.Battle.Target = 0
	resetBattleBotInput(m)
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 32 {
		t.Fatal("saved medicine target ignored the original reordered role", a)
	}
	v.Battle.Target = 1
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("original medicine target was not confirmed", a)
	}
	clearBattleBotOwner(m, v, 0)
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 1 {
		t.Fatal("reclaim confirmed a human's old medicine target", a)
	}
}

func TestBattleBotNoSuppliesCancelAndRemember(t *testing.T) {
	v := itemView()
	v.Party[0].HP = 100
	v.Party[1].HP = 100
	m := itemMemory()
	rememberBattleBotInput(m, v, 2048)
	v.Battle.Menu = 2
	for _, r := range "石钥匙" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
	}
	a, _ := battleBotAction(v, 0, m)
	rememberBattleBotInput(m, v, a.Keys)
	for i := 0; i < 2; i++ {
		rememberBattleBotInput(m, v, 0)
	}
	a, ok := battleBotAction(v, 0, m)
	if !ok || a.Keys != 32 {
		t.Fatal("quest item selected as medicine", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	for i := 0; i < 2; i++ {
		rememberBattleBotInput(m, v, 0)
	}
	a, ok = battleBotAction(v, 0, m)
	if !ok || a.Keys != 1 {
		t.Fatal("no-supply clamped cursor did not cancel", a)
	}
	rememberBattleBotInput(m, v, a.Keys)
	v.Battle.Menu = 0
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
		t.Fatal("empty supply list was repeatedly opened", a)
	}
	v = itemView()
	v.Party[0].MP = 100
	m = itemMemory()
	if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 8 {
		t.Fatal("available ordinary healing was displaced by inventory scanning", a)
	}
}

func TestBattleBotSupplyMemoryBounds(t *testing.T) {
	for _, payload := range []string{`{"role":0,"itemKind":3}`, `{"role":0,"itemKind":-1}`, `{"role":0,"itemRole":6}`, `{"role":0,"noItems":4}`, `{"role":0,"itemAttempts":5}`} {
		var m battleBotMemory
		if err := json.Unmarshal([]byte(`{"roles":[`+payload+`]}`), &m); err != nil {
			t.Fatal(err)
		}
		if validBattleBotMemory(&m) {
			t.Fatal("invalid supply intent accepted", payload)
		}
	}
}

// Cursor tracking intentionally includes gray selected items. That is not
// proof that the original menu will accept the item (including reservations
// made by an earlier role). Only the enabled highlight can authorize use.
func TestBattleBotDoesNotConfirmAnInactiveSupply(t *testing.T) {
	v := itemView()
	m := itemMemory()
	rememberBattleBotInput(m, v, 2048)
	v.Battle.Menu = 2
	for _, r := range "蜂王蜜" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 28))
	}
	for _, r := range "HPMP+150" {
		v.Overlay = append(v.Overlay, engineGlyph(int(r), 31))
	}
	a, _ := battleBotAction(v, 0, m)
	rememberBattleBotInput(m, v, a.Keys)
	for i := 0; i < 2; i++ {
		rememberBattleBotInput(m, v, 0)
	}
	if a, ok := battleBotAction(v, 0, m); !ok || a.Keys != 32 {
		t.Fatal("gray reserved/unavailable supply was repeatedly confirmed", a)
	}
}
