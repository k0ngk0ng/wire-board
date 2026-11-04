package presentation

import (
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"
)

func TestDefaultArenaRowsAgainstAllOriginalRows(t *testing.T) {
	raw, _ := arenaReadGzip(t, "arena-assets-reference")
	var source struct {
		Frames []struct {
			arenaOriginalFrame
			RAM string
		}
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Frames) != 20 {
		t.Fatal("incomplete row evidence")
	}
	for _, f := range source.Frames {
		ram := dataBytes(t, f.RAM, 65536)
		var palette [64]uint16
		for i := range palette {
			palette[i] = binary.BigEndian.Uint16(ram[0xe000+i*2:])
		}
		if palette[0] != binary.BigEndian.Uint16(ram[0xf140:]) {
			t.Fatal("unbound backdrop upload source")
		}
		want := arenaObservedFrame(t, f.arenaOriginalFrame).Rows
		var horizontal [224][2]uint16
		var vertical [224][2]uint16
		for y, row := range want {
			horizontal[y] = row.Horizontal
			vertical[y] = row.Vertical
		}
		rows, err := DefaultArenaRows(CompactPalette(palette), horizontal, vertical)
		if err != nil {
			t.Fatal(err)
		}
		for y := range rows {
			if !reflect.DeepEqual(rows[y], want[y]) {
				t.Fatalf("frame%d row%d got%+v want%+v", f.Frame, y, rows[y], want[y])
			}
		}
	}
	t.Log("4,480 original rows: exact palette, CRAM0 bands, window and visibility profile")
}

func TestDefaultArenaRowsRejectsInvalidPalette(t *testing.T) {
	var palette [64]uint16
	palette[63] = 512
	if rows, err := DefaultArenaRows(palette, [224][2]uint16{}, [224][2]uint16{}); err == nil || rows != [224]ArenaRow{} {
		t.Fatal("invalid compact palette accepted")
	}
}
