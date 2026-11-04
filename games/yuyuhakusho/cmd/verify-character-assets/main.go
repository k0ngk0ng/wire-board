// verify-character-assets checks independent indexed presentation data against
// pinned source spans, original palette calls and private original-frame video.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
)

const originalROMSHA = "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06"

type assetTileSource struct {
	Address uint32
	SHA     string `json:"sha256"`
}
type assetAssignment struct {
	Slot          uint16
	Tile          uint32
	SourceAddress uint32 `json:"source_address"`
}
type assetPiece struct {
	CenterX     int16  `json:"center_x"`
	CenterY     int16  `json:"center_y"`
	WidthTiles  uint8  `json:"width_tiles"`
	HeightTiles uint8  `json:"height_tiles"`
	FirstSlot   uint16 `json:"first_slot"`
	FlipX       bool   `json:"flip_x"`
	FlipY       bool   `json:"flip_y"`
	PaletteSlot int8   `json:"palette_slot"`
}
type assetFrame struct {
	CharacterID        int `json:"character_id"`
	Parameter          uint16
	SourceSHA          string `json:"source_sha256"`
	Assignments        []assetAssignment
	Pieces             []assetPiece
	RequiredPriorSlots []uint16 `json:"required_prior_slots"`
}
type assetPalette struct {
	CharacterID   int    `json:"character_id"`
	SourceAddress uint32 `json:"source_address"`
	Variants      [4][10]uint16
}
type characterAssets struct {
	Schema      int
	ROMSHA      string            `json:"rom_sha256"`
	SpriteSHA   string            `json:"sprite_fixture_sha256"`
	TileFormat  string            `json:"tile_format"`
	TileBytes   string            `json:"tile_bytes"`
	TileSources []assetTileSource `json:"tile_sources"`
	Palettes    []assetPalette
	Shared      struct {
		SourceAddress uint32 `json:"source_address"`
		Variants      [8][10]uint16
	} `json:"shared_palettes"`
	Frames []assetFrame
	RawSHA string `json:"-"`
}
type assetPaletteCall struct {
	CharacterID           int `json:"character_id"`
	Selector, Slot        int
	SourcePointer         uint32 `json:"source_pointer"`
	Before, After, Return string
}
type assetPixelCase struct {
	CharacterID    int `json:"character_id"`
	Selector, Slot int
	SourcePointer  uint32 `json:"source_pointer"`
	StateFrame     int    `json:"state_frame"`
	VideoFrame     int    `json:"video_frame"`
	Actor          string
	VRAMTiles      string   `json:"vram_tiles"`
	CRAMCompact    []uint16 `json:"cram_compact"`
	WorldPalette   string   `json:"world_palette"`
	VDPRegisters   []uint8  `json:"vdp_registers"`
	Width, Height  int
	PixelFormat    int `json:"pixel_format"`
	Video          string
}
type characterAssetOracle struct {
	Schema      int
	AssetSHA    string `json:"asset_sha256"`
	Bindings    []assetPaletteCall
	Pixels      []assetPixelCase
	SourceTiles []struct {
		Address uint32
		Bytes   string
	} `json:"source_tiles"`
}
type sourceSpriteFrame struct {
	CharacterID int `json:"character_id"`
	Parameter   uint16
	TilesBase   uint32 `json:"tiles_base"`
	Payload     string
	SourceSHA   string `json:"source_sha256"`
}
type sourceSpriteFixture struct {
	Schema  int
	ROMSHA  string `json:"rom_sha256"`
	Library []sourceSpriteFrame
}
type assetReport struct {
	Pass                      bool `json:"pass"`
	Characters, Frames, Tiles int
	PaletteCalls              int `json:"palette_calls"`
	PixelCases                int `json:"pixel_cases"`
	Pixels, Mismatches        int
	PriorDependentFrames      int `json:"prior_dependent_frames"`
	Scope                     string
}

func assetDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func assetBytes(encoded string, size int) ([]byte, error) {
	b, err := hex.DecodeString(encoded)
	if err != nil || size >= 0 && len(b) != size {
		return nil, errors.New("invalid indexed-asset bytes")
	}
	return b, nil
}
func readAssetCompressed(path string, into any) (string, string, error) {
	zipped, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return "", "", err
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		return "", "", err
	}
	if err = json.Unmarshal(raw, into); err != nil {
		return "", "", err
	}
	return assetDigest(raw), assetDigest(zipped), nil
}
func loadCharacterInputs(directory string) (characterAssets, characterAssetOracle, sourceSpriteFixture, error) {
	var a characterAssets
	var o characterAssetOracle
	var s sourceSpriteFixture
	asha, azip, err := readAssetCompressed(filepath.Join(directory, "character-assets.json.gz"), &a)
	if err != nil {
		return a, o, s, err
	}
	a.RawSHA = asha
	osha, ozip, err := readAssetCompressed(filepath.Join(directory, "character-assets-oracle.json.gz"), &o)
	if err != nil {
		return a, o, s, err
	}
	_, szip, err := readAssetCompressed(filepath.Join(directory, "sprite-oracle.json.gz"), &s)
	if err != nil {
		return a, o, s, err
	}
	mb, err := os.ReadFile(filepath.Join(directory, "character-assets-manifest.json"))
	if err != nil {
		return a, o, s, err
	}
	var m struct {
		Pass      bool
		Baseline  bool   `json:"frames_match_original_core"`
		AssetSHA  string `json:"asset_sha256"`
		AssetZip  string `json:"asset_gzip_sha256"`
		OracleSHA string `json:"oracle_sha256"`
		OracleZip string `json:"oracle_gzip_sha256"`
		Resume    int    `json:"resume_control_frames_match_original"`
	}
	if err = json.Unmarshal(mb, &m); err != nil {
		return a, o, s, err
	}
	if !m.Pass || !m.Baseline || m.Resume != 10 || m.AssetSHA != asha || m.AssetZip != azip || m.OracleSHA != osha || m.OracleZip != ozip || a.SpriteSHA != szip {
		return a, o, s, errors.New("unbound original asset evidence")
	}
	return a, o, s, nil
}

func resolveFrameSlots(frame assetFrame, previous map[uint16]uint32, tileCount int) (map[uint16]uint32, error) {
	for slot, tile := range previous {
		if slot >= 512 || int(tile) >= tileCount {
			return nil, errors.New("invalid prior indexed tile")
		}
	}
	next := make(map[uint16]uint32, len(previous)+len(frame.Assignments))
	for slot, tile := range previous {
		next[slot] = tile
	}
	for _, a := range frame.Assignments {
		if a.Slot >= 512 || int(a.Tile) >= tileCount {
			return nil, errors.New("invalid frame tile assignment")
		}
		next[a.Slot] = a.Tile
	}
	for _, piece := range frame.Pieces {
		if piece.WidthTiles < 1 || piece.WidthTiles > 4 || piece.HeightTiles < 1 || piece.HeightTiles > 4 || piece.PaletteSlot < -1 || piece.PaletteSlot > 3 {
			return nil, errors.New("invalid indexed sprite piece")
		}
		for slot := int(piece.FirstSlot); slot < int(piece.FirstSlot)+int(piece.WidthTiles)*int(piece.HeightTiles); slot++ {
			if _, ok := next[uint16(slot)]; !ok {
				return nil, errors.New("prior actor tile content is unavailable")
			}
		}
	}
	return next, nil
}

func paletteTail(a characterAssets, character, selector int) ([10]uint16, uint32, error) {
	if character < 0 || character >= 11 || selector < 0 || selector >= 12 {
		return [10]uint16{}, 0, errors.New("unavailable character/palette selector")
	}
	if selector >= 4 {
		return a.Shared.Variants[selector-4], a.Shared.SourceAddress + uint32(selector-4)*32 + 12, nil
	}
	for _, p := range a.Palettes {
		if p.CharacterID == character {
			return p.Variants[selector], p.SourceAddress + uint32(selector)*32 + 12, nil
		}
	}
	return [10]uint16{}, 0, errors.New("character palette is unavailable")
}

func decodeOriginalAssetFrame(f sourceSpriteFrame) ([]assetAssignment, []assetPiece, []uint16, error) {
	b, err := assetBytes(f.Payload, -1)
	if err != nil || assetDigest(b) != f.SourceSHA {
		return nil, nil, nil, errors.New("unbound original graphic bytes")
	}
	position := 0
	assignments := make([]assetAssignment, 0)
	assigned := map[uint16]bool{}
	for i := 0; i < 4; i++ {
		if position+4 > len(b) {
			return nil, nil, nil, errors.New("short original transfer descriptor")
		}
		d := binary.BigEndian.Uint32(b[position:])
		position += 4
		for j := uint32(0); j < uint32(uint8(d)); j++ {
			slot := uint16(uint8(d>>8)) + uint16(j)
			source := f.TilesBase + ((d>>16)&0x7fff)*32 + j*32
			assignments = append(assignments, assetAssignment{Slot: slot, SourceAddress: source})
			assigned[slot] = true
		}
		if d&0x80000000 != 0 {
			break
		}
	}
	pieces := make([]assetPiece, 0)
	missing := map[uint16]bool{}
	ended := false
	for i := 0; i < 80; i++ {
		if position+4 > len(b) {
			break
		}
		v := binary.BigEndian.Uint32(b[position:])
		position += 4
		size := uint8(v>>8) & 15
		flags := (uint16(v) & 0xf000) >> 1
		piece := assetPiece{CenterX: int16((v>>24)&127) - 64, CenterY: int16((v>>16)&127) - 64, WidthTiles: (size >> 2) + 1, HeightTiles: (size & 3) + 1, FirstSlot: uint16(uint8(v)), FlipX: flags&0x800 != 0, FlipY: flags&0x1000 != 0, PaletteSlot: -1}
		if flags&0x6000 != 0 {
			piece.PaletteSlot = int8(flags>>13) & 3
		}
		pieces = append(pieces, piece)
		for j := uint16(0); j < uint16(piece.WidthTiles)*uint16(piece.HeightTiles); j++ {
			slot := piece.FirstSlot + j
			if !assigned[slot] {
				missing[slot] = true
			}
		}
		if v&0x80000000 != 0 {
			ended = true
			break
		}
	}
	if !ended || position != len(b) {
		return nil, nil, nil, errors.New("invalid original graphic termination")
	}
	prior := make([]uint16, 0, len(missing))
	for slot := range missing {
		prior = append(prior, slot)
	}
	sort.Slice(prior, func(i, j int) bool { return prior[i] < prior[j] })
	return assignments, pieces, prior, nil
}

func verifyCharacterAssets(a characterAssets, o characterAssetOracle, s sourceSpriteFixture) (assetReport, error) {
	r := assetReport{Characters: len(a.Palettes), Frames: len(a.Frames), Tiles: len(a.TileSources), PaletteCalls: len(o.Bindings), PixelCases: len(o.Pixels), Scope: "Indexed tiles/palette bindings and declared-private idle frame pixels; not full game rendering"}
	if a.Schema != 1 || o.Schema != 1 || s.Schema != 1 || a.ROMSHA != originalROMSHA || s.ROMSHA != originalROMSHA || o.AssetSHA != a.RawSHA || a.TileFormat != "md-4bpp-row-major-high-nibble-first" || len(a.Palettes) != 11 || len(a.Frames) != 1133 || len(s.Library) != 1133 || len(o.Bindings) != 528 || len(o.Pixels) != 132 {
		return r, errors.New("incomplete indexed asset identity or coverage")
	}
	bank, err := assetBytes(a.TileBytes, len(a.TileSources)*32)
	if err != nil {
		return r, err
	}
	sources := make(map[uint32][]byte, len(o.SourceTiles))
	for _, src := range o.SourceTiles {
		b, err := assetBytes(src.Bytes, 32)
		if err != nil {
			return r, err
		}
		if _, ok := sources[src.Address]; ok {
			return r, errors.New("duplicate ROM source tile")
		}
		sources[src.Address] = b
	}
	seenBank := make(map[string]bool, len(a.TileSources))
	for i, src := range a.TileSources {
		tile := bank[i*32 : (i+1)*32]
		if src.SHA != assetDigest(tile) || !bytes.Equal(tile, sources[src.Address]) || seenBank[string(tile)] {
			return r, errors.New("tilebank differs from bound ROM tile or is not deduplicated")
		}
		seenBank[string(tile)] = true
	}
	type frameKey = struct {
		character int
		parameter uint16
	}
	original := make(map[frameKey]sourceSpriteFrame, len(s.Library))
	for _, f := range s.Library {
		k := frameKey{f.CharacterID, f.Parameter}
		if _, ok := original[k]; ok {
			return r, errors.New("duplicate original graphic key")
		}
		original[k] = f
	}
	frames := make(map[frameKey]assetFrame, len(a.Frames))
	usedSources := map[uint32]bool{}
	for _, f := range a.Frames {
		k := frameKey{f.CharacterID, f.Parameter}
		source, ok := original[k]
		if !ok || f.CharacterID < 0 || f.CharacterID >= 11 || f.SourceSHA != source.SourceSHA {
			return r, errors.New("unbound indexed graphic key")
		}
		if _, ok := frames[k]; ok {
			return r, errors.New("duplicate indexed frame")
		}
		assignments, pieces, prior, err := decodeOriginalAssetFrame(source)
		if err != nil {
			return r, err
		}
		if len(assignments) != len(f.Assignments) || !reflect.DeepEqual(pieces, f.Pieces) || !reflect.DeepEqual(prior, f.RequiredPriorSlots) {
			return r, errors.New("indexed frame geometry or prior dependency differs from original")
		}
		for i, assignment := range f.Assignments {
			expected := assignments[i]
			if assignment.Slot != expected.Slot || assignment.SourceAddress != expected.SourceAddress || int(assignment.Tile) >= len(a.TileSources) || !bytes.Equal(bank[int(assignment.Tile)*32:(int(assignment.Tile)+1)*32], sources[expected.SourceAddress]) {
				return r, errors.New("indexed assignment differs from original tile source")
			}
			usedSources[expected.SourceAddress] = true
		}
		if len(prior) != 0 {
			r.PriorDependentFrames++
		} else if _, err := resolveFrameSlots(f, nil, len(a.TileSources)); err != nil {
			return r, err
		}
		frames[k] = f
	}
	if len(usedSources) != len(sources) || r.PriorDependentFrames != 7 {
		return r, errors.New("unreferenced ROM tiles or missing prior-content frames")
	}
	var paletteIDs [11]bool
	for _, p := range a.Palettes {
		if p.CharacterID < 0 || p.CharacterID >= 11 || paletteIDs[p.CharacterID] {
			return r, errors.New("invalid character palette binding")
		}
		paletteIDs[p.CharacterID] = true
	}
	seenBindings := map[[3]int]bool{}
	for _, call := range o.Bindings {
		key := [3]int{call.CharacterID, call.Selector, call.Slot}
		if seenBindings[key] || call.Slot < 0 || call.Slot >= 4 || call.Return != "0x3d8" {
			return r, errors.New("invalid or duplicate palette CPU call")
		}
		seenBindings[key] = true
		palette, pointer, err := paletteTail(a, call.CharacterID, call.Selector)
		if err != nil || pointer != call.SourcePointer {
			return r, errors.New("palette source selector differs from original")
		}
		before, err := assetBytes(call.Before, 128)
		if err != nil {
			return r, err
		}
		after, err := assetBytes(call.After, 128)
		if err != nil {
			return r, err
		}
		for i, word := range palette {
			binary.BigEndian.PutUint16(before[call.Slot*32+12+i*2:], word)
		}
		if !bytes.Equal(before, after) {
			return r, errors.New("palette submission modified wrong colors or common prefix")
		}
	}
	seenPixels := map[[2]int]bool{}
	for i, pixel := range o.Pixels {
		key := [2]int{pixel.CharacterID, pixel.Selector}
		if seenPixels[key] {
			return r, errors.New("duplicate original pixel case")
		}
		seenPixels[key] = true
		count, mismatches, _, err := verifyAssetPixelCase(a, pixel, frames, bank)
		if err != nil {
			return r, fmt.Errorf("asset pixel case%d character%d selector%d: %w", i, pixel.CharacterID, pixel.Selector, err)
		}
		r.Pixels += count
		r.Mismatches += mismatches
	}
	if r.Mismatches != 0 {
		return r, fmt.Errorf("independent indexed sprites have%d differing original pixels", r.Mismatches)
	}
	r.Pass = true
	return r, nil
}

func paletteCompact(word uint16) uint16 { return (word&0xe00)>>3 | (word&0xe0)>>2 | (word&0xe)>>1 }
func assetColor565(compact uint16, limited bool) uint16 {
	if limited {
		compact &= 0x49
	}
	r, g, b := (compact&7)*2, ((compact>>3)&7)*2, ((compact>>6)&7)*2
	return r<<12 | (r>>3)<<11 | g<<7 | (g>>2)<<5 | b<<1 | b>>3
}

// Generic key shape is shared only within this verifier; no ROM addresses are
// interpreted by the renderer. All pixels select compact tile IDs and colors.
func verifyAssetPixelCase(a characterAssets, p assetPixelCase, frames map[struct {
	character int
	parameter uint16
}]assetFrame, bank []byte) (int, int, *image.RGBA, error) {
	if p.StateFrame <= 0 || p.VideoFrame != p.StateFrame+1 || p.Width != 320 || p.Height != 224 || p.PixelFormat != 2 || p.Slot != 1 || len(p.CRAMCompact) != 64 || len(p.VDPRegisters) != 32 || p.VDPRegisters[12]&0x0e != 0 {
		return 0, 0, nil, errors.New("unbound pixel timing or display mode")
	}
	actor, err := assetBytes(p.Actor, 256)
	if err != nil {
		return 0, 0, nil, err
	}
	if int(actor[0x58]) != p.CharacterID {
		return 0, 0, nil, errors.New("pixel source actor differs from character")
	}
	word := func(off int) uint16 { return binary.BigEndian.Uint16(actor[off:]) }
	attrs := word(0x24)
	if attrs&0x8000 == 0 || int(attrs>>13&3) != p.Slot {
		return 0, 0, nil, errors.New("unsupported low-priority or wrong-slot reference")
	}
	f, ok := frames[struct {
		character int
		parameter uint16
	}{p.CharacterID, word(0x3c)}]
	if !ok {
		return 0, 0, nil, errors.New("pixel graphic key is unavailable")
	}
	palette, pointer, err := paletteTail(a, p.CharacterID, p.Selector)
	if err != nil || pointer != p.SourcePointer {
		return 0, 0, nil, errors.New("pixel palette source mismatch")
	}
	colors := append([]uint16(nil), p.CRAMCompact...)
	for _, v := range colors {
		if v > 511 {
			return 0, 0, nil, errors.New("invalid compact CRAM color")
		}
	}
	for i, v := range palette {
		c := paletteCompact(v)
		if colors[p.Slot*16+6+i] != c {
			return 0, 0, nil, errors.New("original CRAM does not contain selected asset colors")
		}
		colors[p.Slot*16+6+i] = c
	}
	world, err := assetBytes(p.WorldPalette, 128)
	if err != nil {
		return 0, 0, nil, err
	}
	for i := 0; i < 6; i++ {
		if paletteCompact(binary.BigEndian.Uint16(world[p.Slot*32+i*2:])) != colors[p.Slot*16+i] {
			return 0, 0, nil, errors.New("world common palette prefix differs from CRAM")
		}
	}
	vram, err := assetBytes(p.VRAMTiles, 8192)
	if err != nil {
		return 0, 0, nil, err
	}
	for _, assignment := range f.Assignments {
		start := int(assignment.Slot) * 32
		if start+32 > len(vram) || !bytes.Equal(vram[start:start+32], bank[int(assignment.Tile)*32:(int(assignment.Tile)+1)*32]) {
			return 0, 0, nil, errors.New("asset tilebank differs from actual original VRAM")
		}
	}
	slots, err := resolveFrameSlots(f, nil, len(a.TileSources))
	if err != nil {
		return 0, 0, nil, err
	}
	video, err := assetBytes(p.Video, p.Width*p.Height*2)
	if err != nil {
		return 0, 0, nil, err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 320, 224))
	occupied := make([]bool, 320*224)
	count, mismatch := 0, 0
	x, y := word(8)+word(0x28), word(12)+word(0x2a)
	scale := int32(int16(word(0x3e)))
	for _, piece := range f.Pieces {
		dx, dy := piece.CenterX, piece.CenterY
		if attrs&0x800 != 0 {
			dx = -dx
		}
		if attrs&0x1000 != 0 {
			dy = -dy
		}
		dx = int16(int32(dx) * scale >> 8)
		dy = int16(int32(dy) * scale >> 8)
		width, height := int(piece.WidthTiles)*8, int(piece.HeightTiles)*8
		sx := int((x+uint16(dx)-uint16(width/2))&511) - 128
		sy := int((y+uint16(dy)-uint16(height/2))&1023) - 128
		flipX, flipY := attrs&0x800 != 0, attrs&0x1000 != 0
		flipX = flipX != piece.FlipX
		flipY = flipY != piece.FlipY
		pal := int(attrs>>13) & 3
		if piece.PaletteSlot >= 0 {
			pal = int(piece.PaletteSlot)
		}
		for py := 0; py < height; py++ {
			yy := sy + py
			if yy < 0 || yy >= 224 {
				continue
			}
			for px := 0; px < width; px++ {
				xx := sx + px
				if xx < 0 || xx >= 320 || occupied[yy*320+xx] {
					continue
				}
				u, v := px, py
				if flipX {
					u = width - 1 - u
				}
				if flipY {
					v = height - 1 - v
				}
				slot := piece.FirstSlot + uint16((u/8)*int(piece.HeightTiles)+v/8)
				tile := slots[slot]
				value := bank[int(tile)*32+(v%8)*4+(u%8)/2]
				if u&1 == 0 {
					value >>= 4
				}
				value &= 15
				if value == 0 {
					continue
				}
				occupied[yy*320+xx] = true
				wanted := assetColor565(colors[pal*16+int(value)], p.VDPRegisters[0]&4 == 0)
				actual := binary.LittleEndian.Uint16(video[(yy*320+xx)*2:])
				count++
				if wanted != actual {
					mismatch++
				}
				canvas.SetRGBA(xx, yy, color.RGBA{uint8(uint32(wanted>>11) * 255 / 31), uint8(uint32((wanted>>5)&63) * 255 / 63), uint8(uint32(wanted&31) * 255 / 31), 255})
			}
		}
	}
	if count < 100 {
		return count, mismatch, nil, errors.New("empty or too-small character pixel reference")
	}
	return count, mismatch, canvas, nil
}

func main() {
	directory := flag.String("data", "games/yuyuhakusho/data", "verified asset/evidence directory")
	preview := flag.String("preview", "", "optional local contact-sheet PNG")
	flag.Parse()
	a, o, s, err := loadCharacterInputs(*directory)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report, err := verifyCharacterAssets(a, o, s)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *preview != "" {
		if err := writeAssetPreview(a, o, *preview); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(report)
}

func writeAssetPreview(a characterAssets, o characterAssetOracle, path string) error {
	bank, err := assetBytes(a.TileBytes, len(a.TileSources)*32)
	if err != nil {
		return err
	}
	frames := map[struct {
		character int
		parameter uint16
	}]assetFrame{}
	for _, f := range a.Frames {
		frames[struct {
			character int
			parameter uint16
		}{f.CharacterID, f.Parameter}] = f
	}
	const cellW, cellH = 112, 128
	sheet := image.NewRGBA(image.Rect(0, 0, cellW*12, cellH*11))
	for _, p := range o.Pixels {
		_, _, sprite, err := verifyAssetPixelCase(a, p, frames, bank)
		if err != nil {
			return err
		}
		bounds := image.Rectangle{Min: image.Pt(320, 224), Max: image.Pt(0, 0)}
		for y := 0; y < 224; y++ {
			for x := 0; x < 320; x++ {
				if sprite.RGBAAt(x, y).A != 0 {
					if x < bounds.Min.X {
						bounds.Min.X = x
					}
					if y < bounds.Min.Y {
						bounds.Min.Y = y
					}
					if x+1 > bounds.Max.X {
						bounds.Max.X = x + 1
					}
					if y+1 > bounds.Max.Y {
						bounds.Max.Y = y + 1
					}
				}
			}
		}
		ox := p.Selector*cellW + (cellW-bounds.Dx())/2
		oy := p.CharacterID*cellH + cellH - bounds.Dy() - 8
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				sheet.SetRGBA(ox+x-bounds.Min.X, oy+y-bounds.Min.Y, sprite.RGBAAt(x, y))
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, sheet)
}
