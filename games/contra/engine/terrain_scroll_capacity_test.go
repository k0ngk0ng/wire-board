package contra

// Added before tightening scroll storage: a 33-byte real tile column must fit
// exactly at the 192-byte graphics/palette boundary; one more byte must roll
// back, and an already-overlapping cursor must fail even on a no-draw frame.
import (
	"reflect"
	"testing"
)

func TestTerrainScrollDoesNotOverlapPaletteStorage(t *testing.T) {
	d := terrainScrollOriginal(t, "natural")
	resources := terrainScrollResources(t, "stage-1.json")
	var chosen terrainScrollRecord
	found := false
	for _, capture := range d.Captures {
		if capture.Stage != 1 {
			continue
		}
		for _, row := range capture.Records {
			s, w, _, _, _, _ := terrainScrollState(t, d, row.Before)
			if w.FrameScroll == 1 && w.ScreenScroll&7 == 7 && w.ScreenScroll < 248 && s.Axis < 31 && s.TankScroll == 0 && w.AutoScroll == [2]uint8{} {
				chosen = row
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no actual single-column source phase")
	}
	for _, used := range []uint8{159, 160, 193} {
		s, w, c, h, clear, _ := terrainScrollState(t, d, chosen.Before)
		if used == 193 {
			w.FrameScroll = 0
		}
		q := GraphicsQueue{Used: used}
		beforeS, beforeW, beforeC, beforeH, beforeClear, beforeQ := s, w, c, h, clear, q
		result, err := s.AdvanceQueued(TerrainScrollContext{World: &w, Terrain: &c, Header: &h, ScreenCleared: &clear, Queue: &q, Resources: resources})
		if used == 159 {
			if err != nil || q.Used != 192 || len(q.Packets) != 1 || q.Packets[0].EncodedSize() != 33 {
				t.Fatal("legal exact physical capacity rejected", err, q)
			}
		} else {
			if err == nil {
				t.Fatalf("cursor %d crossed graphics/palette storage", used)
			}
			if s != beforeS || w != beforeW || c != beforeC || h != beforeH || clear != beforeClear || !reflect.DeepEqual(q, beforeQ) || len(result.Effects) != 0 {
				t.Fatal("physical-capacity failure left partial phase")
			}
		}
	}
}
