package dsp

// StepKernel returns a value copy of the fixed reference profile's numerical
// step kernel. Its source and LGPL2.1-or-later notice are in KERNEL_SOURCE.md.
// The DAC renderer and the full-chip renderer share this one immutable table.
func StepKernel() [33][8]int16 { return stepKernel }
