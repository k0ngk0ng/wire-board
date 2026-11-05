package stream

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"testing"
)

func testPCM(frames int) []int16 {
	pcm := make([]int16, frames*2)
	seed := uint32(0xdeadbeef)
	for i := range pcm {
		seed = seed*1664525 + 1013904223
		pcm[i] = int16(seed >> 16)
	}
	if len(pcm) >= 6 {
		copy(pcm, []int16{-32768, 32767, 0, -1, 1, 0})
	}
	return pcm
}

func testEncode(t *testing.T, config Config, pcm []int16, pattern []int) ([]byte, []Segment, Status) {
	t.Helper()
	e, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	init := e.InitSegment()
	if err := referenceInit(init, config.BlockFrames); err != nil {
		t.Fatal(err)
	}
	var segments []Segment
	for frame, step := 0, 0; frame < len(pcm)/2; step++ {
		n := pattern[step%len(pattern)]
		if n > len(pcm)/2-frame {
			n = len(pcm)/2 - frame
		}
		out, err := e.AppendPCM(pcm[frame*2 : (frame+n)*2])
		if err != nil {
			t.Fatal(err)
		}
		segments = append(segments, out...)
		frame += n
		status := e.Status()
		if status.AcceptedSamples != uint64(frame) || status.BufferedFrames >= config.FragmentFrames || status.AcceptedSamples != status.EmittedSamples+uint64(status.BufferedFrames) {
			t.Fatal("clock/pending state has another owner", status)
		}
	}
	out, err := e.Finish()
	if err != nil {
		t.Fatal(err)
	}
	segments = append(segments, out...)
	before := e.Status()
	again, err := e.Finish()
	if err != nil || len(again) != 0 || e.Status() != before {
		t.Fatal("Finish is not idempotent", err)
	}
	if !before.Closed || before.BufferedFrames != 0 || before.AcceptedSamples != uint64(len(pcm)/2) || before.EmittedSamples != uint64(len(pcm)/2) {
		t.Fatal("final cursor lost samples", before)
	}
	if _, err := e.AppendPCM(nil); !errors.Is(err, ErrClosed) {
		t.Fatal("closed encoder accepted append", err)
	}
	return init, segments, before
}

func testDecode(t *testing.T, segments []Segment) []int16 {
	t.Helper()
	var result []int16
	cursor := uint64(0)
	for i, segment := range segments {
		if segment.Sequence != uint32(i+1) || segment.StartSample != cursor {
			t.Fatal("noncontiguous fragment metadata", i, segment.Sequence, segment.StartSample, cursor)
		}
		pcm, err := referenceSegment(segment)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, pcm...)
		cursor += uint64(len(pcm) / 2)
	}
	return result
}

func testWire(init []byte, segments []Segment) []byte {
	wire := append([]byte(nil), init...)
	for _, segment := range segments {
		wire = append(wire, segment.Data...)
	}
	return wire
}

func TestEncoderFormatAndConfigErrors(t *testing.T) {
	for _, name := range []string{"rate", "channels", "depth", "block-zero", "block-short", "block-oversize", "fragment-zero", "fragment-short", "fragment-misaligned", "input-zero", "fragment-input", "input-huge"} {
		t.Run(name, func(t *testing.T) {
			c := DefaultConfig()
			switch name {
			case "rate":
				c.Format.SampleRate = 48000
			case "channels":
				c.Format.Channels = 1
			case "depth":
				c.Format.BitsPerSample = 24
			case "block-zero":
				c.BlockFrames = 0
			case "block-short":
				c.BlockFrames = 15
			case "block-oversize":
				c.BlockFrames = 65536
			case "fragment-zero":
				c.FragmentFrames = 0
			case "fragment-short":
				c.FragmentFrames = 512
			case "fragment-misaligned":
				c.FragmentFrames = 4095
			case "input-zero":
				c.MaxInputFrames = 0
			case "fragment-input":
				c.MaxInputFrames = c.FragmentFrames - 1
			case "input-huge":
				c.MaxInputFrames = math.MaxUint32
			}
			e, err := New(c)
			if err == nil || e != nil {
				t.Fatal("invalid config returned a partial encoder", name)
			}
		})
	}
}

func TestEncoderPartitionInvariantAndShortTails(t *testing.T) {
	for _, frames := range []int{0, 1, 7, 15, 16, 17, 1023, 1024, 1025, 4095, 4096, 4097, 8193} {
		pcm := testPCM(frames)
		c := DefaultConfig()
		init, baseline, _ := testEncode(t, c, pcm, []int{65536})
		if !reflect.DeepEqual(testDecode(t, baseline), pcm) && len(pcm) != 0 {
			t.Fatal("reference roundtrip changed samples", frames)
		}
		for _, pattern := range [][]int{{1}, {17, 257, 3, 997, 64}} {
			otherInit, other, _ := testEncode(t, c, pcm, pattern)
			if !bytes.Equal(testWire(init, baseline), testWire(otherInit, other)) {
				t.Fatal("input partition changed encoded bytes", frames, pattern)
			}
		}
	}
}

func TestEncoderBlockAndFragmentVariants(t *testing.T) {
	pcm := testPCM(65536)
	for _, pair := range [][2]uint32{{16, 64}, {256, 1024}, {1024, 4096}, {4096, 8192}, {65535, 65535}} {
		c := DefaultConfig()
		c.BlockFrames, c.FragmentFrames = pair[0], pair[1]
		_, segments, _ := testEncode(t, c, pcm, []int{65536})
		if !reflect.DeepEqual(testDecode(t, segments), pcm) {
			t.Fatal("block/fragment configuration changed PCM", pair)
		}
	}
}

func TestEncoderOwnershipAndAtomicErrors(t *testing.T) {
	c := DefaultConfig()
	e, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	firstInit := e.InitSegment()
	originalInit := append([]byte(nil), firstInit...)
	firstInit[0] ^= 1
	if !bytes.Equal(e.InitSegment(), originalInit) {
		t.Fatal("Init aliases caller storage")
	}
	input := []int16{123, -456, 32767, -32768}
	expected := append([]int16(nil), input...)
	if output, err := e.AppendPCM(input); err != nil || len(output) != 0 {
		t.Fatal(err)
	}
	input[0] = 999
	before := e.Status()
	pending := append([]int16(nil), e.pending...)
	for _, bad := range [][]int16{{1}, make([]int16, int(c.MaxInputFrames)*2+2)} {
		if output, err := e.AppendPCM(bad); err == nil || len(output) != 0 || e.Status() != before || !reflect.DeepEqual(e.pending, pending) {
			t.Fatal("invalid append partially committed", err)
		}
	}
	out, err := e.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(testDecode(t, out), expected) {
		t.Fatal("pending PCM aliases the caller")
	}
	if !bytes.Equal(e.InitSegment(), originalInit) {
		t.Fatal("Finish rewrote a previously emitted init")
	}
	// Previously returned fragments cannot be retained as encoder state.
	full := testPCM(int(c.FragmentFrames))
	a, _ := New(c)
	b, _ := New(c)
	one, _ := a.AppendPCM(full)
	_, _ = b.AppendPCM(full)
	one[0].Data[0] ^= 1
	two, err := a.AppendPCM(full)
	three, otherErr := b.AppendPCM(full)
	if err != nil || otherErr != nil || !bytes.Equal(two[0].Data, three[0].Data) {
		t.Fatal("returned fragment aliases future output")
	}
}

func TestEncoderSampleAndSequenceBoundariesAreAtomic(t *testing.T) {
	c := DefaultConfig()
	c.BlockFrames, c.FragmentFrames = 16, 16
	for _, start := range []uint64{127, 128, 2047, 2048, 65535, 65536, (1 << 21) - 1, 1 << 21, (1 << 26) - 1, 1 << 26, (1 << 31) - 1, 1 << 31, (1 << 36) - 16} {
		e, err := New(c)
		if err != nil {
			t.Fatal(err)
		}
		e.sampleCursor = start
		segments, err := e.AppendPCM(testPCM(16))
		if err != nil || len(segments) != 1 {
			t.Fatal(start, err)
		}
		if segments[0].StartSample != start {
			t.Fatal("sample cursor was truncated", start)
		}
		pcm, err := referenceSegment(segments[0])
		if err != nil || !reflect.DeepEqual(pcm, testPCM(16)) {
			t.Fatal(start, err)
		}
	}
	e, _ := New(c)
	e.sampleCursor = 1<<36 - 16
	if _, err := e.AppendPCM(testPCM(16)); err != nil {
		t.Fatal(err)
	}
	before := e.Status()
	if output, err := e.AppendPCM(testPCM(1)); !errors.Is(err, ErrSampleNumber) || len(output) != 0 || e.Status() != before {
		t.Fatal("sample overflow not atomic", err)
	}
	e, _ = New(c)
	e.nextSequence = math.MaxUint32
	segments, err := e.AppendPCM(testPCM(16))
	if err != nil || len(segments) != 1 || segments[0].Sequence != math.MaxUint32 || e.Status().NextSequence != uint64(math.MaxUint32)+1 {
		t.Fatal("last valid fragment sequence lost", err)
	}
	before = e.Status()
	if output, err := e.AppendPCM(testPCM(1)); !errors.Is(err, ErrSequenceNumber) || len(output) != 0 || e.Status() != before {
		t.Fatal("future short tail sequence overflow not rejected early", err)
	}
	if output, err := e.Finish(); err != nil || len(output) != 0 {
		t.Fatal("empty finish needs no new sequence", err)
	}
}

func TestReferenceCRCAndCorruptionChecks(t *testing.T) {
	// Independently observed RFC-format header in the accepted FFmpeg carrier.
	if referenceCRC8([]byte{0xff, 0xf8, 0xc9, 0x98, 0}) != 0x74 {
		t.Fatal("reference CRC8 disagrees with known frame header")
	}
	c := DefaultConfig()
	_, segments, _ := testEncode(t, c, testPCM(1), []int{1})
	original := segments[0]
	for _, offset := range []int{0, len(original.Data) - 1} {
		changed := original
		changed.Data = append([]byte(nil), original.Data...)
		changed.Data[offset] ^= 1
		if _, err := referenceSegment(changed); err == nil {
			t.Fatal("corrupted box/frame accepted", offset)
		}
	}
	changed := original
	changed.StartSample++
	if _, err := referenceSegment(changed); err == nil {
		t.Fatal("sidecar timestamp mismatch accepted")
	}
}
