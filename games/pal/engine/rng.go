package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"slices"
	"sort"

	"github.com/tetratelabs/wazero/api"
)

// RasterBase retains the scene that existed before a delta movie enabled the
// immutable surface graph. It contains the same public draw IDs as field play.
type RasterBase struct {
	SceneReady bool     `json:"sceneReady"`
	ClearColor int      `json:"clearColor"`
	Map        int      `json:"map"`
	CameraX    int      `json:"cameraX"`
	CameraY    int      `json:"cameraY"`
	Draws      []DrawOp `json:"draws"`
	Overlay    []UIOp   `json:"overlay"`
}

func cloneRasterBase(base *RasterBase) *RasterBase {
	if base == nil {
		return nil
	}
	v := *base
	v.Draws = slices.Clone(base.Draws)
	v.Overlay = slices.Clone(base.Overlay)
	return &v
}

func validateRasterBase(base *RasterBase) error {
	if base == nil || base.ClearColor < 0 || base.ClearColor > 255 || base.Map < 0 || base.Map > 65535 || len(base.Draws) > 8192 || len(base.Overlay) > 8192 {
		return errors.New("invalid PAL raster scene base")
	}
	if err := validateSceneDraws(base.Draws); err != nil {
		return err
	}
	for _, op := range base.Draws {
		if op.Kind != 2 && (op.A < 0 || op.A > 65535 || op.B < 0 || op.B > 65535) {
			return errors.New("invalid PAL raster scene draw")
		}
	}
	for _, op := range base.Overlay {
		if op.Kind < 0 || op.Kind > 1 || op.ID < 0 || op.ID > 65535 || op.Color < 0 || op.Color > 255 || op.Frame < 0 || op.Frame > 4096 || op.Kind == 1 && (op.Source < 1 || op.Source > 9) {
			return errors.New("invalid PAL raster scene overlay")
		}
	}
	return nil
}

func observeRasterBase(ctx context.Context, module api.Module, slot uint64) (*RasterBase, error) {
	p, err := call(ctx, module, "wb_raster_seed_info", slot)
	if err != nil {
		return nil, err
	}
	header, ok := module.Memory().Read(uint32(p), 32)
	if !ok {
		return nil, errors.New("invalid native raster scene header")
	}
	h := func(i int) int { return int(int32(binary.LittleEndian.Uint32(header[i*4:]))) }
	if h(0) != 1 || h(6) < 0 || h(6) > 8192 || h(7) < 0 || h(7) > 8192 {
		return nil, errors.New("invalid native raster scene counts")
	}
	base := &RasterBase{SceneReady: h(1) != 0, ClearColor: h(2), Map: h(3), CameraX: h(4), CameraY: h(5), Draws: make([]DrawOp, h(6)), Overlay: make([]UIOp, h(7))}
	data, ok := module.Memory().Read(uint32(p)+32, uint32((h(6)+h(7))*32))
	if !ok {
		return nil, errors.New("native raster scene exceeds memory")
	}
	for i := range base.Draws {
		p := data[i*32:]
		n := func(j int) int { return int(int32(binary.LittleEndian.Uint32(p[j*4:]))) }
		base.Draws[i] = DrawOp{n(0), n(1), n(2), n(3), n(4), n(5), n(6), n(7)}
	}
	for i := range base.Overlay {
		p := data[(h(6)+i)*32:]
		n := func(j int) int { return int(int32(binary.LittleEndian.Uint32(p[j*4:]))) }
		base.Overlay[i] = UIOp{n(0), n(1), n(2), n(3), n(4), n(5), n(6), n(7)}
	}
	sort.SliceStable(base.Draws, func(i, j int) bool { return base.Draws[i].SortY < base.Draws[j].SortY })
	return base, validateRasterBase(base)
}
