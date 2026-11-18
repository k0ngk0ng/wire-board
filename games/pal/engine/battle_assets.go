package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"math/bits"

	"github.com/tetratelabs/wazero/api"
)

func observeBattleAssets(ctx context.Context, module api.Module) ([][2]int, error) {
	if module.ExportedFunction("wb_battle_assets") == nil {
		return nil, nil
	}
	address, err := call(ctx, module, "wb_battle_assets")
	if err != nil {
		return nil, err
	}
	raw, ok := module.Memory().Read(uint32(address), 64)
	if !ok {
		return nil, errors.New("native battle assets exceed memory")
	}
	var assets [][2]int
	for group, source := range [...]int{6, 8} {
		for word := range 8 {
			mask := binary.LittleEndian.Uint32(raw[(group*8+word)*4:])
			for mask != 0 {
				bit := bits.TrailingZeros32(mask)
				assets = append(assets, [2]int{source, word*32 + bit})
				mask &= mask - 1
			}
		}
	}
	return assets, nil
}

func validateBattleAssets(assets [][2]int, available bool) error {
	if len(assets) == 0 {
		return nil
	}
	if !available || len(assets) > 512 {
		return errors.New("invalid saved battle asset plan")
	}
	previous := -1
	for _, asset := range assets {
		source, id := asset[0], asset[1]
		key := source*256 + id
		if (source != 6 && source != 8) || id < 0 || id >= 256 || key <= previous {
			return errors.New("invalid saved battle asset identity")
		}
		previous = key
	}
	return nil
}
