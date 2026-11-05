package presentation

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

var arenaProfileCases = []struct {
	Arena              uint16
	Behavior, Resource uint32
	Name, SHA          string
	Tiles              int
}{
	{0, 0x20918, 0x21720, "arena-assets-default", "8b3626b608d90ac990176370f665b42499dc404546f955aafd0beeee5cd35542", 406},
	{2, 0x20ae0, 0x2175c, "arena-assets-2", "baf59b615e155f791e409b60e87cbe44ea43ce997230d426675c5cf6302709f5", 480},
	{4, 0x20cb2, 0x21798, "arena-assets-4", "a8e9906e78ff660650413ce94adc9b1c425f384638956ed304877bf69d12133a", 311},
	{6, 0x20e28, 0x217cc, "arena-assets-6", "7fa944acfdc4f9f0af3c6c131cf388fbd70bf7760da9e559b14c16174e304139", 462},
	{8, 0x20fb4, 0x21800, "arena-assets-8", "dc4b52569a636e5e6c4103399ae668527ec98363513bbc547c749bc5303fb737", 478},
	{10, 0x21126, 0x21834, "arena-assets-a", "fc95e4147eba1f158ef79a5e31374123e9f0d0359dd3041e7e0261f5f99743da", 455},
	{12, 0x212f8, 0x21870, "arena-assets-c", "2e1b28e6dfd4e975c4ace5ae9f350af242907e34437d9c07611633eb8fe4cb51", 420},
	{14, 0x21412, 0x218a4, "arena-assets-e", "a5fa5b72544edd249393e8eba4dedec4fc3b63771494e00d2ff09c07c79f41f6", 463},
}

func TestArenaProfilesCatalogsAndSelection(t *testing.T) {
	for _, item := range arenaProfileCases {
		t.Run(fmt.Sprintf("%x", item.Arena), func(t *testing.T) {
			raw, compressed := arenaReadGzip(t, item.Name)
			if hash := sha256.Sum256(compressed); hex.EncodeToString(hash[:]) != item.SHA {
				t.Fatal("unbound selected arena assets")
			}
			before := append([]byte(nil), raw...)
			catalog, err := DecodeArenaCatalogForArena(raw, item.Arena)
			if err != nil {
				t.Fatal(err)
			}
			if got := catalog.Identity(); got != (ArenaIdentity{ArenaOffset: item.Arena, SceneBehavior: item.Behavior, ResourceList: item.Resource}) {
				t.Fatalf("wrong selected identity: %+v", got)
			}
			if len(catalog.Planes[0].Tiles) != item.Tiles || len(catalog.Planes[1].Tiles) != item.Tiles || &catalog.Planes[0].Tiles[0] != &catalog.Planes[1].Tiles[0] {
				t.Fatal("immutable tile bank was replaced or duplicated")
			}
			for _, other := range arenaProfileCases {
				if other.Arena == item.Arena {
					continue
				}
				if got, err := DecodeArenaCatalogForArena(raw, other.Arena); err == nil || !reflect.DeepEqual(got, ArenaCatalog{}) {
					t.Fatal("wrong selection accepted", other.Arena)
				}
			}
			for _, bad := range []uint16{1, 3, 15, 16, 0xfffe, 0xffff} {
				if got, err := DecodeArenaCatalogForArena(raw, bad); err == nil || !reflect.DeepEqual(got, ArenaCatalog{}) {
					t.Fatal("unsupported selection accepted", bad)
				}
			}
			legacy, legacyErr := DecodeArenaCatalog(raw)
			if item.Arena == 0 {
				if legacyErr != nil || !reflect.DeepEqual(legacy, catalog) {
					t.Fatal("legacy default changed")
				}
			} else if legacyErr == nil || !reflect.DeepEqual(legacy, ArenaCatalog{}) {
				t.Fatal("legacy default entry accepted other arena")
			}
			if !bytes.Equal(raw, before) {
				t.Fatal("decoder changed source bytes")
			}
		})
	}
}

func TestArenaProfilesCatalogFailures(t *testing.T) {
	type mutation struct {
		name  string
		apply func(map[string]any)
	}
	mutations := []mutation{
		{"schema", func(v map[string]any) { v["schema"] = 2 }},
		{"ROM", func(v map[string]any) { v["rom_sha256"] = "wrong" }},
		{"arena", func(v map[string]any) { v["arena_offset"] = 16 }},
		{"behavior", func(v map[string]any) { v["scene_behavior"] = 0x2091a }},
		{"binding-arena", func(v map[string]any) { v["resource_binding"].(map[string]any)["arena_offset"] = 16 }},
		{"binding-behavior", func(v map[string]any) { v["resource_binding"].(map[string]any)["scene_behavior"] = 0x2091a }},
		{"binding-list", func(v map[string]any) { v["resource_binding"].(map[string]any)["resource_list"] = 0x21722 }},
		{"missing-binding", func(v map[string]any) { delete(v, "resource_binding") }},
		{"no-tiles", func(v map[string]any) { v["packed_4bpp_tiles"] = []string{} }},
		{"bad-hex", func(v map[string]any) { v["packed_4bpp_tiles"].([]any)[0] = strings.Repeat("gg", 32) }},
		{"short-tile", func(v map[string]any) { v["packed_4bpp_tiles"].([]any)[0] = strings.Repeat("00", 31) }},
		{"long-tile", func(v map[string]any) { v["packed_4bpp_tiles"].([]any)[0] = strings.Repeat("00", 33) }},
		{"short-palette", func(v map[string]any) { v["initial_scene_palette_0"] = make([]uint16, 15) }},
		{"long-palette", func(v map[string]any) { v["initial_scene_palette_0"] = make([]uint16, 17) }},
		{"bad-color", func(v map[string]any) { v["initial_scene_palette_0"].([]any)[15] = 512 }},
		{"missing-plane", func(v map[string]any) { v["planes"] = v["planes"].([]any)[:1] }},
		{"swapped-planes", func(v map[string]any) { p := v["planes"].([]any); p[0], p[1] = p[1], p[0] }},
		{"wrong-columns", func(v map[string]any) { v["planes"].([]any)[0].(map[string]any)["columns"] = 32 }},
		{"wrong-rows", func(v map[string]any) { v["planes"].([]any)[1].(map[string]any)["rows"] = 64 }},
		{"short-cells", func(v map[string]any) {
			p := v["planes"].([]any)[0].(map[string]any)
			p["cells"] = p["cells"].([]any)[:2047]
		}},
		{"long-cells", func(v map[string]any) {
			p := v["planes"].([]any)[1].(map[string]any)
			p["cells"] = append(p["cells"].([]any), map[string]any{"tile": 0, "flags": 0})
		}},
		{"tile-range", func(v map[string]any) {
			v["planes"].([]any)[0].(map[string]any)["cells"].([]any)[0].(map[string]any)["tile"] = 65535
		}},
		{"tile-flags", func(v map[string]any) {
			v["planes"].([]any)[1].(map[string]any)["cells"].([]any)[0].(map[string]any)["flags"] = 1
		}},
	}
	for _, item := range arenaProfileCases {
		raw, _ := arenaReadGzip(t, item.Name)
		for _, mutate := range mutations {
			t.Run(fmt.Sprintf("%x/%s", item.Arena, mutate.name), func(t *testing.T) {
				var source map[string]any
				if err := json.Unmarshal(raw, &source); err != nil {
					t.Fatal(err)
				}
				mutate.apply(source)
				bad, err := json.Marshal(source)
				if err != nil {
					t.Fatal(err)
				}
				if got, err := DecodeArenaCatalogForArena(bad, item.Arena); err == nil || !reflect.DeepEqual(got, ArenaCatalog{}) {
					t.Fatal("malformed catalog accepted")
				}
			})
		}
		for _, bad := range [][]byte{nil, []byte("{"), append(append([]byte(nil), raw...), 'x')} {
			if got, err := DecodeArenaCatalogForArena(bad, item.Arena); err == nil || !reflect.DeepEqual(got, ArenaCatalog{}) {
				t.Fatal("malformed JSON accepted")
			}
		}
	}
}

func TestArenaProfilesAuxiliaryIdentityAndTiles(t *testing.T) {
	raw, compressed := arenaReadGzip(t, "arena-a-aux-assets")
	if h := sha256.Sum256(compressed); hex.EncodeToString(h[:]) != "3eb8fd42cf1e675c91140f406833af7fde38968109749e0702a31f1471aa75b4" {
		t.Fatal("unbound auxiliary resource")
	}
	got, err := DecodeArenaAAuxCatalog(raw)
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Tiles []string `json:"packed_4bpp_tiles"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Tiles) != 48 || len(got.Tiles) != 48 {
		t.Fatal("auxiliary bank must preserve all48 source tiles")
	}
	for i, encoded := range source.Tiles {
		if hex.EncodeToString(got.Tiles[i][:]) != encoded {
			t.Fatal("auxiliary source ordering changed", i)
		}
	}
	for _, mutate := range []func(map[string]any){
		func(v map[string]any) { v["schema"] = 2 }, func(v map[string]any) { v["rom_sha256"] = "wrong" },
		func(v map[string]any) { v["arena_offset"] = 0 }, func(v map[string]any) { v["scene_behavior"] = 0x20918 },
		func(v map[string]any) { v["callback"] = 0x2125a }, func(v map[string]any) { v["resource_source"] = 297056 },
		func(v map[string]any) { v["packed_4bpp_tiles"] = v["packed_4bpp_tiles"].([]any)[:47] },
		func(v map[string]any) {
			v["packed_4bpp_tiles"] = append(v["packed_4bpp_tiles"].([]any), strings.Repeat("00", 32))
		},
		func(v map[string]any) { v["packed_4bpp_tiles"].([]any)[47] = "00" },
		func(v map[string]any) { v["packed_4bpp_tiles"].([]any)[0] = strings.Repeat("zz", 32) },
	} {
		var bad map[string]any
		if err := json.Unmarshal(raw, &bad); err != nil {
			t.Fatal(err)
		}
		mutate(bad)
		encoded, _ := json.Marshal(bad)
		if got, err := DecodeArenaAAuxCatalog(encoded); err == nil || !reflect.DeepEqual(got, ArenaAAuxCatalog{}) {
			t.Fatal("malformed auxiliary resource accepted")
		}
	}
	for _, bad := range [][]byte{nil, []byte("{"), append(append([]byte(nil), raw...), 'x')} {
		if got, err := DecodeArenaAAuxCatalog(bad); err == nil || !reflect.DeepEqual(got, ArenaAAuxCatalog{}) {
			t.Fatal("malformed auxiliary JSON accepted")
		}
	}
}

func TestArenaProfilesHUDResourceBindingAndAtomicity(t *testing.T) {
	catalog := loadHUD(t)
	state, err := NewHUD(catalog)
	if err != nil {
		t.Fatal(err)
	}
	base := []engine.ChuWorldEvent{{Kind: "start", Start: engine.MatchStartEvent{Kind: "reset"}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "resource", Source: 0x21720}}, {Kind: "start", Start: engine.MatchStartEvent{Kind: "words", Offset: 0, Words: []uint16{0xe00d}}}, {Kind: "hud", HUD: engine.FighterBarTransfer{Pattern: 1, TargetTile: 16}}}
	want, err := ApplyHUDWorldEvents(state, base, nil, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range arenaProfileCases {
		events := append([]engine.ChuWorldEvent(nil), base...)
		events[1].Start.Source = item.Resource
		got, err := ApplyHUDWorldEventsForArena(state, events, nil, catalog, item.Arena)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("selected HUD changed common artwork", item.Arena, err)
		}
		if item.Arena != 0 {
			if _, err := ApplyHUDWorldEvents(state, events, nil, catalog); err == nil {
				t.Fatal("legacy HUD accepted another arena")
			}
		}
		for _, other := range arenaProfileCases {
			if other.Arena == item.Arena {
				continue
			}
			bad := append([]engine.ChuWorldEvent(nil), base...)
			bad[1].Start.Source = other.Resource
			before, _ := json.Marshal(want)
			if got, err := ApplyHUDWorldEventsForArena(want, bad, nil, catalog, item.Arena); err == nil || !reflect.DeepEqual(got, want) {
				t.Fatal("wrong HUD resource was not atomic", item.Arena, other.Arena)
			}
			after, _ := json.Marshal(want)
			if !bytes.Equal(before, after) {
				t.Fatal("HUD source state mutated")
			}
		}
	}
	for _, arena := range []uint16{1, 16, 65535} {
		if got, err := ApplyHUDWorldEventsForArena(want, nil, nil, catalog, arena); err == nil || !reflect.DeepEqual(got, want) {
			t.Fatal("unsupported HUD selection accepted")
		}
	}
	badCatalog := hudRead(t, "hud-assets", "catalog_sha256")
	var bad map[string]any
	if err := json.Unmarshal(badCatalog, &bad); err != nil {
		t.Fatal(err)
	}
	bad["resource_list"] = 0x2175c
	encoded, _ := json.Marshal(bad)
	if _, err := DecodeHUDCatalog(encoded); err == nil {
		t.Fatal("shared HUD original asset identity weakened")
	}
}

func TestArenaProfilesRowsAndValidation(t *testing.T) {
	var palette [64]uint16
	for i := range palette {
		palette[i] = uint16(i * 7)
	}
	var horizontal, vertical [224][2]uint16
	for y := range horizontal {
		horizontal[y] = [2]uint16{uint16(y * 3), uint16(65535 - y)}
		vertical[y] = [2]uint16{uint16(y + 40), uint16(y * 2)}
	}
	for _, item := range arenaProfileCases {
		for _, backdrop := range []uint8{0, 1, 63} {
			rows, err := ArenaRowsForArena(item.Arena, backdrop, palette, horizontal, vertical)
			if err != nil {
				t.Fatal(err)
			}
			for y, row := range rows {
				if row.Horizontal != horizontal[y] || row.Vertical != vertical[y] || row.Backdrop != backdrop || row.Visible != (y >= 14 && y < 208) || row.Window != (y < 48) || row.Limited {
					t.Fatal("incorrect observed row profile", item.Arena, y)
				}
				want := palette
				if y < 16 || y >= 208 {
					want[0] = 0
				}
				if row.Palette != want {
					t.Fatal("wrong row current color/backdrop bands", item.Arena, y)
				}
			}
			if item.Arena == 0 && backdrop == 0 {
				legacy, err := DefaultArenaRows(palette, horizontal, vertical)
				if err != nil || legacy != rows {
					t.Fatal("default row behavior changed")
				}
			}
		}
	}
	for _, item := range arenaProfileCases {
		if rows, err := ArenaRowsForArena(item.Arena, 64, palette, horizontal, vertical); err == nil || rows != [224]ArenaRow{} {
			t.Fatal("invalid backdrop accepted")
		}
	}
	for _, arena := range []uint16{1, 16, 65535} {
		if rows, err := ArenaRowsForArena(arena, 0, palette, horizontal, vertical); err == nil || rows != [224]ArenaRow{} {
			t.Fatal("unsupported row profile accepted")
		}
	}
	palette[63] = 512
	if rows, err := ArenaRowsForArena(2, 0, palette, horizontal, vertical); err == nil || rows != [224]ArenaRow{} {
		t.Fatal("invalid current color accepted")
	}
}

func TestArenaProfilesOriginalBackgroundPixels(t *testing.T) {
	compressed, err := os.ReadFile("testdata/additional-arena-presentation-reference.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/additional-arena-presentation-reference-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass           bool
		SHA            string `json:"fixture_sha256"`
		Gzip           string `json:"gzip_sha256"`
		Frames, Pixels int
	}
	if err := json.Unmarshal(mb, &manifest); err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(z)
	z.Close()
	if err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !manifest.Pass || manifest.SHA != hex.EncodeToString(h[:]) || manifest.Gzip != hex.EncodeToString(zh[:]) || manifest.Frames != 140 || manifest.Pixels != 9535530 {
		t.Fatal("unbound compact original fixture")
	}
	var fixture struct {
		Frames []struct {
			Arena                uint16
			Frame                int
			Palette              [64]uint16
			Backdrop             uint8
			Horizontal, Vertical [224][2]uint16
			Window               struct {
				Tiles []string
				Cells []ArenaCell
			}
			Mask              string
			SHA               string `json:"pixels_sha256"`
			Pixels            int
			RowFlags          [224]uint8 `json:"row_flags"`
			VisiblePaletteSHA string     `json:"visible_palette_sha256"`
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Frames) != 140 {
		t.Fatal("incomplete compact frames")
	}
	catalogs := map[uint16]ArenaCatalog{}
	for _, item := range arenaProfileCases {
		raw, _ := arenaReadGzip(t, item.Name)
		c, err := DecodeArenaCatalogForArena(raw, item.Arena)
		if err != nil {
			t.Fatal(err)
		}
		catalogs[item.Arena] = c
	}
	counts := map[uint16]int{}
	pixels := 0
	for _, f := range fixture.Frames {
		catalog, ok := catalogs[f.Arena]
		if !ok || f.Arena == 0 || f.Frame < 2700 || f.Frame > 3550 {
			t.Fatal("unbound compact sample identity")
		}
		counts[f.Arena]++
		rows, err := ArenaRowsForArena(f.Arena, f.Backdrop, f.Palette, f.Horizontal, f.Vertical)
		if err != nil {
			t.Fatal(err)
		}
		paletteBytes := bytes.Buffer{}
		for y, row := range rows {
			var flags uint8
			if row.Visible {
				flags |= 1
			}
			if row.Window {
				flags |= 2
			}
			if row.Limited {
				flags |= 4
			}
			if flags != f.RowFlags[y] {
				t.Fatal("original display flags differ", f.Arena, f.Frame, y)
			}
			if row.Visible {
				for _, color := range row.Palette {
					if err := binary.Write(&paletteBytes, binary.BigEndian, color); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		if h := sha256.Sum256(paletteBytes.Bytes()); hex.EncodeToString(h[:]) != f.VisiblePaletteSHA {
			t.Fatal("visible palette differs", f.Arena, f.Frame)
		}
		window := IndexedPlane{Columns: 64, Rows: 32, Cells: f.Window.Cells}
		for _, encoded := range f.Window.Tiles {
			b, err := hex.DecodeString(encoded)
			if err != nil || len(b) != 32 {
				t.Fatal("invalid reference window")
			}
			var tile Tile
			copy(tile[:], b)
			window.Tiles = append(window.Tiles, tile)
		}
		raster, err := RasterizeArena(catalog, ArenaFrame{Width: 320, Height: 224, Rows: rows[:], Window: &window})
		if err != nil {
			t.Fatal(err)
		}
		mask, err := hex.DecodeString(f.Mask)
		if err != nil || len(mask) != (320*224+7)/8 {
			t.Fatal("invalid original sprite exclusion")
		}
		compared := 0
		visible := bytes.Buffer{}
		for i, color := range raster.Colors {
			if mask[i/8]&(1<<uint(i&7)) != 0 {
				continue
			}
			if err := binary.Write(&visible, binary.LittleEndian, color); err != nil {
				t.Fatal(err)
			}
			compared++
		}
		if h := sha256.Sum256(visible.Bytes()); hex.EncodeToString(h[:]) != f.SHA || compared != f.Pixels {
			t.Fatal("original unmasked pixels differ", f.Arena, f.Frame, compared)
		}
		pixels += compared
	}
	for _, item := range arenaProfileCases[1:] {
		if counts[item.Arena] != 20 {
			t.Fatal("incomplete original sample group", item.Arena, counts[item.Arena])
		}
	}
	if pixels != 9535530 {
		t.Fatal("incomplete original pixel coverage", pixels)
	}
	t.Log("140 original frames; 31,360 observed rows; 9,535,530 unmasked background pixels match; ID4 blank unused9..11 phases remain outside palette-equality claim")
}
