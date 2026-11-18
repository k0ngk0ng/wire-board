package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"slices"

	"github.com/tetratelabs/wazero/api"
)

// FieldState is a bounded public projection of original scene.c movement.
// Actors are [kind, role/event, sprite, worldX, worldY, frame, walkFrames,
// layer]. Cells answer original collisions and touch-trigger boundaries in an
// isometric radius; no event scripts, RNG, memory or rule resources are exposed.
type FieldState struct {
	Frame     uint32    `json:"frame"`
	Anchor    [2]int    `json:"anchor"`
	Camera    [2]int    `json:"camera"`
	Offset    [2]int    `json:"offset"`
	Layer     int       `json:"layer"`
	Direction int       `json:"direction"`
	Step      int       `json:"step"`
	Party     int       `json:"party"`
	Followers int       `json:"followers"`
	Radius    int       `json:"radius"`
	Trail     [5][3]int `json:"trail"`
	Checks    [5][2]int `json:"checks"`
	Cells     []byte    `json:"cells"`
	Actors    [][8]int  `json:"actors"`
}

func cloneField(source *FieldState) *FieldState {
	if source == nil {
		return nil
	}
	copy := *source
	copy.Cells = slices.Clone(source.Cells)
	copy.Actors = slices.Clone(source.Actors)
	return &copy
}

func observeField(ctx context.Context, module api.Module) (*FieldState, error) {
	address, err := call(ctx, module, "wb_field_info")
	if err != nil {
		return nil, err
	}
	head, ok := module.Memory().Read(uint32(address), 64)
	if !ok {
		return nil, errors.New("native field observation exceeds memory")
	}
	get := func(data []byte, i int) int { return int(int32(binary.LittleEndian.Uint32(data[i*4:]))) }
	if get(head, 0) == 0 {
		return nil, nil
	}
	count := get(head, 13)
	if get(head, 0) != 1 || get(head, 14) != 8 || get(head, 15) != 334 || count < 1 || count > 517 {
		return nil, errors.New("invalid native field observation header")
	}
	data, ok := module.Memory().Read(uint32(address), uint32((365+count*8)*4))
	if !ok {
		return nil, errors.New("native field actors exceed memory")
	}
	f := &FieldState{Frame: uint32(get(data, 1)), Anchor: [2]int{get(data, 2), get(data, 3)}, Camera: [2]int{get(data, 4), get(data, 5)}, Offset: [2]int{get(data, 6), get(data, 7)}, Layer: get(data, 8), Direction: get(data, 9), Step: get(data, 10), Party: get(data, 11), Followers: get(data, 12), Radius: 8, Cells: make([]byte, 334), Actors: make([][8]int, count)}
	for i := range f.Trail {
		for j := range f.Trail[i] {
			f.Trail[i][j] = get(data, 16+i*3+j)
		}
		f.Checks[i] = [2]int{f.Trail[i][0], f.Trail[i][1]}
	}
	for i := range f.Cells {
		value := get(data, 31+i)
		if value < 0 || value > 3 {
			return nil, errors.New("invalid native field collision flags")
		}
		f.Cells[i] = byte(value)
	}
	for i := range f.Actors {
		for j := range f.Actors[i] {
			f.Actors[i][j] = get(data, 365+i*8+j)
		}
	}
	return f, nil
}

func validateField(v StateView) error {
	f := v.Field
	if f == nil {
		return nil
	}
	bad := func() error { return errors.New("invalid native field prediction state") }
	if v.Mode != FieldMode || v.Entering || v.Battle.Active || !v.SceneReady || f.Radius != 8 || len(f.Cells) != 334 || f.Party < 1 || f.Party > 3 || f.Party != len(v.Party) || f.Followers < 0 || f.Followers > 2 || f.Party+f.Followers > 5 || len(f.Actors) < f.Party+f.Followers || len(f.Actors) > 517 || f.Step < 0 || f.Step > 3 || f.Direction < 0 || f.Direction > 3 || f.Camera != [2]int{v.CameraX, v.CameraY} || f.Layer < 0 || f.Layer > 65535 {
		return bad()
	}
	for i := 0; i < 2; i++ {
		if f.Offset[i] < -32768 || f.Offset[i] > 32767 || f.Anchor[i] != f.Camera[i]+f.Offset[i] {
			return bad()
		}
	}
	for i, cell := range f.Cells {
		if cell > 3 || (i >= 289 && cell > 1) {
			return bad()
		}
	}
	for i, trail := range f.Trail {
		if trail[0] < 0 || trail[0] > 65535 || trail[1] < 0 || trail[1] > 65535 || trail[2] < 0 || trail[2] > 3 || f.Checks[i] != [2]int{trail[0], trail[1]} {
			return bad()
		}
	}
	for i, actor := range f.Actors {
		kind := 1
		if i < f.Party {
			kind = 0
			if actor[1] != v.Party[i] {
				return bad()
			}
		} else if i < f.Party+f.Followers {
			kind = 2
		}
		if actor[0] != kind || actor[1] < 0 || actor[1] > 65535 || actor[2] < 0 || actor[2] > 65535 || actor[3] < -65536 || actor[3] > 131070 || actor[4] < -65536 || actor[4] > 131070 || actor[5] < 0 || actor[5] > 4096 || actor[6] < 0 || actor[6] > 4096 || actor[7] < -524288 || actor[7] > 524288 {
			return bad()
		}
	}
	if f.Actors[0][3] != v.X || f.Actors[0][4] != v.Y {
		return bad()
	}
	return nil
}
