// Command verify-yuyu-arena-aux verifies the complete production presentation
// path for the six arena-A raw submissions against compact original video.
// It is an offline verification harness, not a game or reference-core runtime.
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
	"io"
	"os"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
)

const romSHA = "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06"

type pixelFrame struct {
	Frame            int                   `json:"frame"`
	FirstCount       uint16                `json:"first_count"`
	Draws            []engine.ArenaASprite `json:"draws"`
	NonAuxMask       string                `json:"non_aux_mask"`
	AuxVisibleMask   string                `json:"aux_visible_mask"`
	Expected         string                `json:"expected_rgb565"`
	ExpectedSHA      string                `json:"expected_sha256"`
	ComparedPixels   int                   `json:"compared_pixels"`
	AuxVisiblePixels int                   `json:"aux_visible_pixels"`
	AuxDistinct      int                   `json:"aux_distinct_from_background"`
	ExcludedPixels   int                   `json:"non_aux_excluded_pixels"`
	SATIndices       []uint16              `json:"sat_tail_indices"`
}

type rowFrame struct {
	Arena                uint16
	Frame                int
	Palette              [64]uint16
	Backdrop             uint8
	Horizontal, Vertical [224][2]uint16
	Window               struct {
		Tiles []string
		Cells []presentation.ArenaCell
	}
}

type frameResult struct {
	Frame           int    `json:"frame"`
	FirstCount      uint16 `json:"first_count"`
	ComparedPixels  int    `json:"compared_pixels"`
	ExcludedPixels  int    `json:"non_aux_excluded_pixels"`
	AuxVisible      int    `json:"aux_visible_pixels"`
	AuxDistinct     int    `json:"aux_distinct_from_background"`
	PixelMismatches int    `json:"pixel_mismatches"`
	OwnerMismatches int    `json:"aux_visibility_mismatches"`
}

type result struct {
	Pass            bool              `json:"pass"`
	Mode            string            `json:"mode"`
	Error           string            `json:"error,omitempty"`
	Frames          []frameResult     `json:"frames"`
	ComparedPixels  int               `json:"compared_pixels"`
	ExcludedPixels  int               `json:"non_aux_excluded_pixels"`
	AuxVisible      int               `json:"aux_visible_pixels"`
	AuxDistinct     int               `json:"aux_distinct_from_background"`
	PixelMismatches int               `json:"pixel_mismatches"`
	OwnerMismatches int               `json:"aux_visibility_mismatches"`
	Digests         map[string]string `json:"source_gzip_sha256"`
}

func digest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func boundFixture(path string) ([]byte, string, error) {
	compressed, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	manifest, err := os.ReadFile(path[:len(path)-len(".json.gz")] + "-manifest.json")
	if err != nil {
		return nil, "", err
	}
	var binding struct {
		Pass bool
		SHA  string `json:"fixture_sha256"`
		Gzip string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(manifest, &binding); err != nil {
		return nil, "", err
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, "", err
	}
	raw, err := io.ReadAll(z)
	closeErr := z.Close()
	if err != nil {
		return nil, "", err
	}
	if closeErr != nil {
		return nil, "", closeErr
	}
	if !binding.Pass || binding.SHA != digest(raw) || binding.Gzip != digest(compressed) {
		return nil, "", errors.New("unbound original semantic fixture")
	}
	return raw, binding.Gzip, nil
}

func asset(path, sha string) ([]byte, error) {
	compressed, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if digest(compressed) != sha {
		return nil, errors.New("unbound original indexed asset")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(z)
	closeErr := z.Close()
	if err != nil {
		return nil, err
	}
	return raw, closeErr
}

func window(frame rowFrame) (presentation.IndexedPlane, error) {
	p := presentation.IndexedPlane{Columns: 64, Rows: 32, Cells: frame.Window.Cells}
	for _, encoded := range frame.Window.Tiles {
		raw, err := hex.DecodeString(encoded)
		if err != nil || len(raw) != 32 {
			return p, errors.New("invalid original indexed window adapter")
		}
		var tile presentation.Tile
		copy(tile[:], raw)
		p.Tiles = append(p.Tiles, tile)
	}
	return p, nil
}

func bit(mask []byte, index int) bool { return mask[index/8]&(1<<uint(index&7)) != 0 }

func verifyFrame(f pixelFrame, rows rowFrame, arena presentation.ArenaCatalog, aux presentation.ArenaAAuxCatalog, mode string) (frameResult, error) {
	r := frameResult{Frame: f.Frame, FirstCount: f.FirstCount}
	if len(f.Draws) != 6 || len(f.SATIndices) != 6 || f.FirstCount < 2 || f.FirstCount > 74 || f.ComparedPixels <= 0 || f.AuxVisiblePixels <= 0 || f.AuxDistinct <= 0 {
		return r, errors.New("incomplete six-raw original sample")
	}
	for i, index := range f.SATIndices {
		if index != f.FirstCount+uint16(i) {
			return r, errors.New("original raw count sequence differs from SAT tail")
		}
	}
	nonAux, err := hex.DecodeString(f.NonAuxMask)
	if err != nil || len(nonAux) != 8960 {
		return r, errors.New("invalid non-A pixel exclusion mask")
	}
	auxMask, err := hex.DecodeString(f.AuxVisibleMask)
	if err != nil || len(auxMask) != 8960 {
		return r, errors.New("invalid visible auxiliary mask")
	}
	expected, err := hex.DecodeString(f.Expected)
	if err != nil || len(expected) != 2*f.ComparedPixels || digest(expected) != f.ExpectedSHA {
		return r, errors.New("unbound original video pixel sequence")
	}
	if mode == "mask-all" {
		for i := range nonAux {
			nonAux[i] = 255
		}
	}
	covered, auxCount := 0, 0
	for i := 0; i < 320*224; i++ {
		if bit(nonAux, i) {
			if bit(auxMask, i) {
				return r, errors.New("exclusion mask covers an original-visible auxiliary pixel")
			}
		} else {
			covered++
		}
		if bit(auxMask, i) {
			auxCount++
		}
	}
	if covered != f.ComparedPixels || auxCount != f.AuxVisiblePixels || 320*224-covered != f.ExcludedPixels {
		return r, errors.New("original pixel mask coverage changed")
	}
	profile, err := presentation.ArenaRowsForArena(10, rows.Backdrop, rows.Palette, rows.Horizontal, rows.Vertical)
	if err != nil {
		return r, err
	}
	w, err := window(rows)
	if err != nil {
		return r, err
	}
	bg, err := presentation.RasterizeArena(arena, presentation.ArenaFrame{Width: 320, Height: 224, Rows: profile[:], Window: &w})
	if err != nil {
		return r, err
	}
	state := presentation.BeginFrame(presentation.TileSlots{}, f.FirstCount)
	if mode != "no-aux" {
		for _, raw := range f.Draws {
			if mode == "bad-x" {
				raw.X++
			}
			if mode == "bad-tile" {
				raw.Tile = 0x5d0 + ((raw.Tile - 0x5d0 + 16) % 48)
			}
			state, err = presentation.ComposeArenaASprite(state, raw)
			if err != nil {
				return r, err
			}
		}
		if len(state.Parts) != 6 || state.Count != f.FirstCount+6 {
			return r, errors.New("production raw order or sprite budget changed")
		}
	}
	var sprites presentation.Raster
	if mode == "wrong-bank" {
		sprites, err = presentation.RasterizeSprites(state, presentation.Catalog{}, rows.Palette, 320, 224, false)
	} else {
		sprites, err = presentation.RasterizeSpritesForArena(state, presentation.Catalog{}, &aux, rows.Palette, 320, 224, false)
	}
	if err != nil {
		return r, err
	}
	combined, err := presentation.CompositeArenaSprites(bg, sprites)
	if err != nil {
		return r, err
	}
	if mode == "bad-expected" {
		for i, j := 0, 0; i < 320*224; i++ {
			if bit(nonAux, i) {
				continue
			}
			if bit(auxMask, i) {
				expected[j*2] ^= 1
				break
			}
			j++
		}
	}
	index := 0
	for i, color := range combined.Colors {
		if bit(nonAux, i) {
			r.ExcludedPixels++
			continue
		}
		want := binary.LittleEndian.Uint16(expected[index*2:])
		index++
		r.ComparedPixels++
		if color != want {
			r.PixelMismatches++
		}
		owned := combined.Owners[i] == 0x9e00
		if owned != bit(auxMask, i) {
			r.OwnerMismatches++
		}
		if owned {
			r.AuxVisible++
			if color != bg.Colors[i] {
				r.AuxDistinct++
			}
		}
	}
	return r, nil
}

func verify(fixturePath, rowsPath, arenaPath, auxPath, mode string) (result, error) {
	r := result{Mode: mode, Digests: map[string]string{}}
	switch mode {
	case "restored", "no-aux", "wrong-bank", "bad-x", "bad-tile", "mask-all", "bad-expected":
	default:
		return r, errors.New("unknown verification mode")
	}
	raw, fixtureSHA, err := boundFixture(fixturePath)
	if err != nil {
		return r, err
	}
	r.Digests[fixturePath] = fixtureSHA
	var original struct {
		Schema  int
		ROM     string `json:"rom_sha256"`
		Arena   uint16
		RowsSHA string `json:"rows_gzip_sha256"`
		Frames  []pixelFrame
	}
	if err := json.Unmarshal(raw, &original); err != nil {
		return r, err
	}
	raw, rowsSHA, err := boundFixture(rowsPath)
	if err != nil {
		return r, err
	}
	r.Digests[rowsPath] = rowsSHA
	var sourceRows struct {
		Schema int
		ROM    string `json:"rom_sha256"`
		Frames []rowFrame
	}
	if err := json.Unmarshal(raw, &sourceRows); err != nil {
		return r, err
	}
	if original.Schema != 1 || original.ROM != romSHA || original.Arena != 10 || len(original.Frames) != 20 || original.RowsSHA != rowsSHA || sourceRows.Schema != 1 || sourceRows.ROM != romSHA {
		return r, errors.New("wrong original arena/row identity")
	}
	const arenaSHA = "fc95e4147eba1f158ef79a5e31374123e9f0d0359dd3041e7e0261f5f99743da"
	raw, err = asset(arenaPath, arenaSHA)
	if err != nil {
		return r, err
	}
	r.Digests[arenaPath] = arenaSHA
	arena, err := presentation.DecodeArenaCatalogForArena(raw, 10)
	if err != nil {
		return r, err
	}
	const auxSHA = "3eb8fd42cf1e675c91140f406833af7fde38968109749e0702a31f1471aa75b4"
	raw, err = asset(auxPath, auxSHA)
	if err != nil {
		return r, err
	}
	r.Digests[auxPath] = auxSHA
	aux, err := presentation.DecodeArenaAAuxCatalog(raw)
	if err != nil {
		return r, err
	}
	rowsByFrame := map[int]rowFrame{}
	for _, f := range sourceRows.Frames {
		if f.Arena == 10 {
			if _, exists := rowsByFrame[f.Frame]; exists {
				return r, errors.New("duplicate original row sample")
			}
			rowsByFrame[f.Frame] = f
		}
	}
	seen := map[int]bool{}
	for _, f := range original.Frames {
		rows, ok := rowsByFrame[f.Frame]
		if !ok || seen[f.Frame] {
			return r, errors.New("missing or duplicate original frame")
		}
		seen[f.Frame] = true
		frame, err := verifyFrame(f, rows, arena, aux, mode)
		if err != nil {
			return r, fmt.Errorf("frame %d: %w", f.Frame, err)
		}
		r.Frames = append(r.Frames, frame)
		r.ComparedPixels += frame.ComparedPixels
		r.ExcludedPixels += frame.ExcludedPixels
		r.AuxVisible += frame.AuxVisible
		r.AuxDistinct += frame.AuxDistinct
		r.PixelMismatches += frame.PixelMismatches
		r.OwnerMismatches += frame.OwnerMismatches
		if mode == "restored" && (frame.AuxVisible != f.AuxVisiblePixels || frame.AuxDistinct != f.AuxDistinct) {
			return r, errors.New("production auxiliary contribution differs from original")
		}
	}
	r.Pass = len(r.Frames) == 20 && r.ComparedPixels == 1368854 && r.ExcludedPixels == 64746 && r.AuxVisible == 29063 && r.AuxDistinct == 28224 && r.PixelMismatches == 0 && r.OwnerMismatches == 0
	return r, nil
}

func main() {
	fixture := flag.String("fixture", "games/yuyuhakusho/presentation/testdata/arena-a-aux-pixel-reference.json.gz", "compact original pixel oracle")
	rows := flag.String("rows-fixture", "games/yuyuhakusho/presentation/testdata/additional-arena-presentation-reference.json.gz", "shared semantic original display inputs")
	arena := flag.String("arena-assets", "games/yuyuhakusho/data/arena-assets-a.json.gz", "selected strict indexed background")
	aux := flag.String("aux-assets", "games/yuyuhakusho/data/arena-a-aux-assets.json.gz", "selected strict auxiliary local bank")
	mode := flag.String("mode", "restored", "restored or deliberate negative verification mode")
	reportPath := flag.String("report", "", "optional JSON report path")
	flag.Parse()
	r, err := verify(*fixture, *rows, *arena, *aux, *mode)
	if err != nil {
		r.Pass = false
		r.Error = err.Error()
	}
	encoded, marshalErr := json.MarshalIndent(r, "", "  ")
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, marshalErr)
		os.Exit(1)
	}
	encoded = append(encoded, '\n')
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, encoded, 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	os.Stdout.Write(encoded)
	if !r.Pass {
		os.Exit(1)
	}
}
