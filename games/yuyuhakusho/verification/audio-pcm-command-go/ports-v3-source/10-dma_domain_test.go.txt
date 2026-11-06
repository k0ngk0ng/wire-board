package pcmcommands_test

import (
	"crypto/sha256"
	"encoding/hex"
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"reflect"
	"testing"
)

func TestUnscheduledDMAWaitDomainIsRejectedAtomically(t *testing.T) {
	digest := sha256.Sum256([]byte{0})
	id := hex.EncodeToString(digest[:])
	cfg := pcmc.Config{Resources: pcmc.Resources{Identity: "private-dma-rejection", Length: func(s string) (uint32, bool) { return 1, s == id }, ByteAt: func(s string, i uint32) (byte, bool) { return 0, s == id && i == 0 }}, Descriptors: []pcmc.Descriptor{{ID: "tiny", PayloadID: id, NativeRate: 20, Flags: 224, RoutingOrPan: 192, Protocol: pcmc.OrdinaryProtocol}}}
	d, e := pcmc.New(cfg)
	if e != nil {
		t.Fatal(e)
	}
	before := d.Snapshot()
	_, e = d.AdvanceFrame(pcmc.FramePlan{Inputs: []pcmc.Input{{Kind: pcmc.Ordinary, DescriptorID: "tiny"}}, DMA: []pcmc.DMAInterval{{Begin: 0, End: 100000, Mode: 0, Remaining: 1}}})
	if e == nil || !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("unproven DMA wait scheduling accepted or partially committed")
	}
}
