package contra

// Failures before implementation: graphics writes applied as collision later;
// attr-column timing differs from tile-column timing; streaming replaces an
// entire collision screen; midpoint lookahead uses the wrong map buffer; source
// map IDs or graphics addresses drift; unsupported alternate graphics silently
// use the initial tiles; a failed decode leaves half of a scroll committed.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type scrollSnapshot struct {
	State                  []uint8
	Maps, Collision, Queue string
}
type scrollOracle struct {
	Fields  []struct{ Name, Address string }
	Records []struct {
		Frame         int
		Before, After scrollSnapshot
	}
}

func loadScrollOracle(t *testing.T) scrollOracle {
	t.Helper()
	f, err := os.Open("../../../docs/research/nes-contra/evidence/world-scroll-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var d scrollOracle
	if err = json.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if len(d.Fields) != 22 || len(d.Records) != 600 {
		t.Fatal("incomplete original scrolling oracle")
	}
	return d
}
func scrollFromOriginal(t *testing.T, d scrollOracle, s scrollSnapshot) (JungleScroll, OutdoorWorld, CollisionWindow, uint8) {
	t.Helper()
	v := map[string]uint8{}
	for i, f := range d.Fields {
		v[f.Name] = s.State[i]
	}
	state := JungleScroll{AlternateGraphicsScreen: v["alternateGraphics"], Column: v["column"], TransitionRemaining: v["loadCountdown"],
		TileAddress: uint16(v["tileLo"]) | uint16(v["tileHi"])<<8, AttributeAddress: uint16(v["attributeLo"]) | uint16(v["attributeHi"])<<8, MapOffset: v["mapOffset"], TankScroll: v["tankScroll"]}
	raw, err := hex.DecodeString(s.Maps)
	if err != nil || len(raw) != 128 {
		t.Fatal("invalid map window")
	}
	copy(state.Supertiles[:], raw)
	raw, err = hex.DecodeString(s.Collision)
	if err != nil || len(raw) != 128 {
		t.Fatal("invalid collision window")
	}
	collision := CollisionWindow(raw)
	w := OutdoorWorld{Level: v["level"], SceneType: v["sceneType"], ScrollType: v["scrollType"], Screen: v["screen"], ScreenScroll: v["screenScroll"], FrameScroll: v["frameScroll"], AutoScroll: [2]uint8{v["autoA"], v["autoB"]}, BossScrollComplete: v["bossScrollComplete"], Scroll: CollisionScroll{X: v["scrollX"], Y: v["scrollY"], NameTable: v["ppuControl"]}}
	return state, w, collision, v["enemyOffset"]
}
func encodeOriginalWrites(t *testing.T, writes []BackgroundWrite) []uint8 {
	t.Helper()
	var encoded []uint8
	for _, w := range writes {
		switch w.Step {
		case 32:
			encoded = append(encoded, 2, uint8(len(w.Values)), 1, uint8(w.Address>>8), uint8(w.Address))
			encoded = append(encoded, w.Values...)
		case 8:
			encoded = append(encoded, 1, 1, uint8(len(w.Values)))
			for i, v := range w.Values {
				a := w.Address + uint16(i)*8
				encoded = append(encoded, uint8(a>>8), uint8(a), v)
			}
		default:
			t.Fatal("unexpected graphics update stride", w.Step)
		}
	}
	return encoded
}
func TestOriginalJungleScrolling(t *testing.T) {
	d := loadScrollOracle(t)
	columns, attributes := 0, 0
	for _, r := range d.Records {
		s, w, c, enemy := scrollFromOriginal(t, d, r.Before)
		wantS, wantW, wantC, wantEnemy := scrollFromOriginal(t, d, r.After)
		result, err := s.Advance(&w, &c)
		if err != nil {
			t.Fatalf("original frame %d: %v", r.Frame, err)
		}
		if result.ResetEnemyReadOffset {
			enemy = 0
		}
		if s != wantS || w != wantW || c != wantC || enemy != wantEnemy {
			t.Fatalf("original frame %d state differs\nstate %+v / %+v\nworld %+v / %+v\ncollision %x / %x", r.Frame, s, wantS, w, wantW, c, wantC)
		}
		before, _ := hex.DecodeString(r.Before.Queue)
		want, _ := hex.DecodeString(r.After.Queue)
		actual := append(before, encodeOriginalWrites(t, result.Writes)...)
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("original frame %d graphics queue got %x want %x", r.Frame, actual, want)
		}
		for _, write := range result.Writes {
			if write.Step == 32 {
				columns++
			} else {
				attributes++
			}
		}
	}
	if columns != 29 || attributes != 15 {
		t.Fatal("original stream coverage changed", columns, attributes)
	}
}
func TestJungleScrollScreenBoundaryAndRollback(t *testing.T) {
	d := loadScrollOracle(t)
	s, w, c, _ := scrollFromOriginal(t, d, d.Records[len(d.Records)-1].Before)
	w.ScreenScroll, w.Scroll.X, w.FrameScroll = 255, 255, 1
	s.Column = 15
	s.TileAddress = 0x200f
	s.AttributeAddress = 0x23c0
	s.MapOffset = 0
	result, err := s.Advance(&w, &c)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ResetEnemyReadOffset || w.Screen != 1 || w.ScreenScroll != 0 || w.Scroll.X != 0 || w.Scroll.NameTable&1 != 1 {
		t.Fatal("screen rollover", w, result)
	}
	w.Screen, w.ScreenScroll, w.Scroll.X, w.FrameScroll = 10, 255, 255, 1
	beforeS, beforeW, beforeC := s, w, c
	if _, err = s.Advance(&w, &c); err == nil {
		t.Fatal("unrecovered alternate graphics transition silently accepted")
	}
	if s != beforeS || w != beforeW || c != beforeC {
		t.Fatal("failed transition left partial world changes")
	}
}
