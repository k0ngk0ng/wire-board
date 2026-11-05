package runtimebridge

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/presentation"
)

func TestArenaSceneFrameRejectsTruncatedSources(t *testing.T) {
	for _, source := range []int32{0xe600, 0x7000e600, -0x11a00, -0x19ff, -0x15be, 2147483646} {
		w := engine.ChuWorldState{ScrollSources: [2]int32{source, -0x1800}, Stream: make([]byte, 1024)}
		if _, err := sceneFrame(w, [64]uint16{}, &presentation.IndexedPlane{}); err == nil {
			t.Errorf("accepted unowned source %d through a truncated address", source)
		}
	}
	for _, source := range []int32{-0x1a00, -0x1800, -0x15c0} {
		w := engine.ChuWorldState{ScrollSources: [2]int32{source, source}, Stream: make([]byte, 1024)}
		if _, err := sceneFrame(w, [64]uint16{}, &presentation.IndexedPlane{}); err != nil {
			t.Errorf("owned complete row view %d rejected: %v", source, err)
		}
	}
}
