package engine

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/tetratelabs/wazero/api"
)

type EndingState struct {
	Clip      int  `json:"clip"`
	Frame     int  `json:"frame"`
	Frames    int  `json:"frames"`
	Rate      int  `json:"rate"`
	ElapsedMS int  `json:"elapsedMS"`
	Serial    int  `json:"serial"`
	Finished  bool `json:"finished"`
}

func validateEnding(v *EndingState, event Event) error {
	if v == nil {
		return nil
	}
	frames := map[int]int{4: 200, 5: 2294, 6: 2196}
	if v.Finished {
		if v.Clip != 0 || v.Serial != 3 || event.Kind != SceneComplete || v.Rate != 15 || v.Frame < 0 || v.Frame >= 2196 || v.Frames != 2196 || v.ElapsedMS != v.Frame*1000/15 {
			return errors.New("invalid completed native PAL ending")
		}
		return nil
	}
	if v.Clip < 4 || v.Clip > 6 || v.Frames != frames[v.Clip] || v.Frame < 0 || v.Frame >= v.Frames || v.Rate != 15 || v.Serial != v.Clip-3 || v.ElapsedMS != v.Frame*1000/15 || event.Kind != Delay || event.Value < 0 || event.Value > 500 {
		return errors.New("invalid native PAL ending position")
	}
	return nil
}

func observeEnding(ctx context.Context, module api.Module, event Event) (*EndingState, error) {
	if module.ExportedFunction("wb_ending_info") == nil {
		return nil, nil
	}
	address, err := call(ctx, module, "wb_ending_info")
	if err != nil {
		return nil, err
	}
	data, ok := module.Memory().Read(uint32(address), 36)
	if !ok {
		return nil, errors.New("native PAL ending exceeds memory")
	}
	get := func(i int) int { return int(binary.LittleEndian.Uint32(data[i*4:])) }
	if get(0) != 1 || get(4) != 15 || get(5) != 1 || get(8) > 1 {
		return nil, errors.New("native PAL ending format is invalid")
	}
	if get(1) == 0 && get(8) == 0 {
		return nil, nil
	}
	v := &EndingState{Clip: get(1), Frame: get(2), Frames: get(3), Rate: get(4), ElapsedMS: get(6), Serial: get(7), Finished: get(8) == 1}
	return v, validateEnding(v, event)
}
