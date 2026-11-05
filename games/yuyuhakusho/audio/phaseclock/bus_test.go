package phaseclock

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/binary"
	"io"
	"math"
	"testing"
)

//go:embed testdata/native-bus.bin.gz
var nativeBus []byte

func TestOriginalBankTransactions(t *testing.T) {
	r, err := gzip.NewReader(bytes.NewReader(nativeBus))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var header [12]byte
	if _, err = io.ReadFull(r, header[:]); err != nil {
		t.Fatal(err)
	}
	if string(header[:8]) != "YYHBUS1\n" {
		t.Fatal("invalid native corpus magic")
	}
	count := binary.BigEndian.Uint32(header[8:])
	if count != 851037 {
		t.Fatal("incomplete original corpus", count)
	}
	var row [20]byte
	var dmaCases, phaseCounterexamples int
	for i := uint32(0); i < count; i++ {
		if _, err = io.ReadFull(r, row[:]); err != nil {
			t.Fatal(i, err)
		}
		cycle := binary.BigEndian.Uint32(row[0:4])
		dma := DMA{End: binary.BigEndian.Uint32(row[4:8]), Mode: row[8], Remaining: binary.BigEndian.Uint32(row[9:13])}
		effect, err := ResolveBankAccess(cycle, dma)
		if err != nil {
			t.Fatal(i, err)
		}
		resume := binary.BigEndian.Uint32(row[13:17])
		flags := row[18]
		if effect.SetWaitFlag {
			flags |= 4
		}
		if effect.At != resume-45 || effect.Z80After != resume || effect.M68KDelay != uint32(row[17]) || flags != row[19] {
			t.Fatalf("native row %d: cycle=%d dma=%+v effect=%+v resume=%d main=%d flags=%d/%d", i, cycle, dma, effect, resume, row[17], flags, row[19])
		}
		if resume-cycle > 45 {
			dmaCases++
		}
		// Advancing one reference-frame origin changes residue modulo seven.
		wrong, err := ResolveBankAccess(cycle+896040, dma)
		if err == nil && wrong.M68KDelay != effect.M68KDelay {
			phaseCounterexamples++
		}
	}
	if n, err := r.Read(row[:1]); n != 0 || err != io.EOF {
		t.Fatal("trailing or invalid gzip data", n, err)
	}
	if dmaCases != 357 || phaseCounterexamples == 0 {
		t.Fatal("missing DMA or frame-origin counterexamples", dmaCases, phaseCounterexamples)
	}
	t.Logf("%d observed transactions; %d DMA stalls; %d absolute-phase counterexamples", count, dmaCases, phaseCounterexamples)
}

func TestReferenceBankBoundaries(t *testing.T) {
	for _, test := range []struct {
		cycle uint32
		dma   DMA
		want  Effect
	}{
		{0, DMA{}, Effect{At: 0, Z80After: 45, M68KDelay: 70}},
		{5, DMA{}, Effect{At: 5, Z80After: 50, M68KDelay: 77}},
		{6, DMA{}, Effect{At: 6, Z80After: 51, M68KDelay: 77}},
		{0, DMA{End: 5, Mode: 0, Remaining: 1}, Effect{At: 5, Z80After: 50, M68KDelay: 77, SetWaitFlag: true}},
		{0, DMA{End: 5, Mode: 1}, Effect{At: 5, Z80After: 50, M68KDelay: 77}},
		{0, DMA{End: 5, Mode: 2, Remaining: 1}, Effect{At: 0, Z80After: 45, M68KDelay: 70}},
		{0, DMA{End: 5, Mode: 3, Remaining: 1}, Effect{At: 0, Z80After: 45, M68KDelay: 70}},
		{5, DMA{End: 5, Mode: 0, Remaining: 1}, Effect{At: 5, Z80After: 50, M68KDelay: 77}},
		{math.MaxUint32 - 45, DMA{}, Effect{At: math.MaxUint32 - 45, Z80After: math.MaxUint32, M68KDelay: 70}},
	} {
		got, err := ResolveBankAccess(test.cycle, test.dma)
		if err != nil || got != test.want {
			t.Fatalf("cycle %d dma %+v: got %+v/%v want %+v", test.cycle, test.dma, got, err, test.want)
		}
	}
}

func TestReferenceBankRejectsInvalidWithoutEffect(t *testing.T) {
	for _, test := range []struct {
		cycle uint32
		dma   DMA
	}{
		{0, DMA{Mode: 4}}, {0, DMA{Mode: 255}},
		{math.MaxUint32 - 44, DMA{}}, {math.MaxUint32, DMA{}},
		{0, DMA{End: math.MaxUint32, Mode: 0, Remaining: 1}},
	} {
		got, err := ResolveBankAccess(test.cycle, test.dma)
		if err == nil || got != (Effect{}) {
			t.Fatal("invalid bank effect accepted", test, got, err)
		}
	}
}
