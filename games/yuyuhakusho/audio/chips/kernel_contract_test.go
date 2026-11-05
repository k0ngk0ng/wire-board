package chips_test

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
	"testing"
)

func TestSharedStepKernelIsValueCopy(t *testing.T) {
	before := dsp.StepKernel()
	changed := dsp.StepKernel()
	changed[0][0]++
	if changed == before || dsp.StepKernel() != before {
		t.Fatal("shared immutable numerical profile was mutated")
	}
	if before[0][0] != 43 || before[32][7] != 5861 {
		t.Fatal("unexpected fixed reference kernel")
	}
}
