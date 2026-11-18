package pal

// Failures before changing magic scanning: a retained cursor skips an earlier
// healing/offensive spell because the original list clamps; Home confirms old
// glyphs before redraw; a saved selected spell is needlessly moved; unbounded
// or ownerless Home intent is accepted in persisted memory.
import "testing"

func TestBattleBotMagicScanRewindsRetainedCursor(t *testing.T) {
	for _, healing := range []bool{true, false} {
		name, wanted := "offense", "御剑术"
		if healing {
			name, wanted = "healing", "气疗术"
		}
		t.Run(name, func(t *testing.T) {
			v := offensiveView()
			v.Battle.Menu = 1
			m := &battleBotMemory{Roles: []battleBotRole{{Role: 0, Healing: healing, Offensive: !healing}}}
			if healing {
				v.Party[0].HP = 30
			}
			for _, r := range "天罡战气" {
				v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
			}
			a, ok := battleBotAction(v, 0, m)
			if !ok || a.Keys != 65536 {
				t.Fatal("retained magic cursor did not rewind before scanning", a)
			}
			rememberBattleBotInput(m, v, a.Keys)
			if !validBattleBotMemory(m) || m.Roles[0].Pending != 65536 {
				t.Fatal("original magic Home cannot survive a save", m)
			}
			for i := 0; i < 2; i++ {
				if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 0 {
					t.Fatal("magic rewind accepted stale glyphs", a)
				}
				rememberBattleBotInput(m, v, 0)
			}
			v.Overlay = nil
			for _, r := range wanted {
				v.Overlay = append(v.Overlay, engineGlyph(int(r), 250))
			}
			resetBattleBotInput(m)
			if a, ok = battleBotAction(v, 0, m); !ok || a.Keys != 2 {
				t.Fatal("saved visible spell selection was not preserved", a)
			}
		})
	}
	if validBattleBotMemory(&battleBotMemory{Roles: []battleBotRole{{Role: 0, Pending: 65536}}}) {
		t.Fatal("ownerless magic Home accepted")
	}
}
