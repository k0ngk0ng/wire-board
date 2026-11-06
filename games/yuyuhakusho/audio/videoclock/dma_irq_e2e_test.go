package videoclock_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// This observes the existing semantic exception helper at the actual mask
// restore inside0654. It is not an implementation of the vertical handler.
func TestWorldDMAOriginalMaskRestoreVerticalAcceptance(t *testing.T) {
	var fixture struct {
		Fields []string
		Cases  []struct{ Events [][]uint64 }
	}
	dmaFixture(t, "native-world-dma.json.gz", &fixture)
	accepted := 0
	for caseIndex, row := range fixture.Cases {
		var cpu []dmaWorldEvent
		for _, values := range row.Events {
			e := dmaWorldEvent{}
			for i, key := range fixture.Fields {
				e[key] = values[i]
			}
			if e["kind"] == 1 || e["kind"] == 10 {
				cpu = append(cpu, e)
			}
		}
		for i, e := range cpu {
			if i+1 == len(cpu) || e["pc"] != 0x68e || cpu[i+1]["pc"] != 0x2e5e {
				continue
			}
			next := cpu[i+1]
			at := e["cycle"]
			if int32(at) >= int32(e["refresh"]) {
				at += 14
			}
			irq, err := engine.AcceptWorldIRQ(at, uint8(e["pin"]), 0)
			if err != nil || !irq.Accepted || irq.Mask != 6 || e["mask"] != 7 || irq.Cycle+16*7 != next["cycle"] || next["mask"] != 6 || next["vpending"] != 0 || next["pin"] != 0 {
				t.Fatal("real mask-restore IRQ differs", caseIndex, err, irq, e, next)
			}
			accepted++
		}
	}
	if accepted != 6 {
		t.Fatal("missing cross-frame mask restore cases", accepted)
	}
}
