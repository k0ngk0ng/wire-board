package engine

import "testing"

// Failures established before correction: split each long message at three
// tiles; interpret messages as two player labels; overwrite reply with blanks;
// omit the original index6 gap; write the other team's panel or name row.
func TestOriginalDisplayTacticMessageLayout(t *testing.T) {
	var rows []struct {
		Slot, Main, Reply        uint8
		UpperUpload, LowerUpload struct {
			Address uint16
			Length  uint8
			Glyphs  [9]uint8
		}
		CacheBefore, CacheAfter [4]uint8
		CacheWrites             [][]any
	}
	readPropAttackFixture(t, "testdata/display-tactic-layout.json.gz", &rows)
	if len(rows) != 78 {
		t.Fatal("missing original message/reply combinations", len(rows))
	}
	for _, r := range rows {
		d, e := NewOriginalDisplay(DefaultOriginalMatchConfiguration())
		if e != nil {
			t.Fatal(e)
		}
		before := d
		side := r.Slot >> 1
		var tactics [4]uint8
		tactics[side*2], tactics[side*2+1] = r.Main, r.Reply
		m := MatchState{}
		m.Clock.HUDTurn = r.Slot
		out, e := ObserveDisplayHUD(d, m, m, tactics)
		if e != nil {
			t.Fatal(e)
		}
		p := out.HUD.Panels[side]
		if r.UpperUpload.Length != 9 || r.LowerUpload.Length != 9 || p.Lines[1] != r.UpperUpload.Glyphs || p.Lines[2] != r.LowerUpload.Glyphs {
			t.Fatalf("slot%d main%d reply%d: got upper%v lower%v source upper%v lower%v", r.Slot, r.Main, r.Reply, p.Lines[1], p.Lines[2], r.UpperUpload.Glyphs, r.LowerUpload.Glyphs)
		}
		if p.Lines[1][6] != 0 || p.Lines[2][6] != 0 {
			t.Fatal("original separator lost")
		}
		if p.Lines[0] != before.HUD.Panels[side].Lines[0] || p.HeaderGlyphs != before.HUD.Panels[side].HeaderGlyphs || p.EdgeGlyphs != before.HUD.Panels[side].EdgeGlyphs || out.HUD.Panels[side^1] != before.HUD.Panels[side^1] {
			t.Fatal("message update changed names/other panel")
		}
		if r.CacheBefore != r.CacheAfter || len(r.CacheWrites) != 0 {
			t.Fatal("unexpected original cache writes")
		}
	}
}
