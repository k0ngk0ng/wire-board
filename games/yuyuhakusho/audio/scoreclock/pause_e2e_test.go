package scoreclock_test

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type pauseNativeFields struct {
	value  uint8
	writes []psgAccess
}

func (p *pauseNativeFields) ReadPause(uint64) (uint8, error) { return p.value, nil }
func (p *pauseNativeFields) WritePause(at uint64, v uint8) error {
	p.value = v
	p.writes = append(p.writes, psgAccess{At: at, Value: v})
	return nil
}

// This isolation consumes observed native external responses. It proves the
// fixed caller's IO timing, not independently synthesized PCM/busy feedback.
type pauseNativeIO struct {
	rows []access
	next int
	psg  []psgAccess
}

func (p *pauseNativeIO) take(kind string, at uint64) (access, error) {
	if p.next >= len(p.rows) {
		return access{}, fmt.Errorf("extra %s at%d", kind, at)
	}
	want := p.rows[p.next]
	if want.Kind != kind || want.At != at {
		return access{}, fmt.Errorf("access%d %s at%d, native %+v", p.next, kind, at, want)
	}
	p.next++
	return want, nil
}
func (p *pauseNativeIO) RequestBus(at uint64, on bool) (uint64, error) {
	v, e := p.take("request", at)
	if e != nil {
		return 0, e
	}
	if v.Asserted != on {
		return 0, errors.New("native BUSREQ level differs")
	}
	return v.After, nil
}
func (p *pauseNativeIO) BusGranted(at uint64) (bool, error) {
	v, e := p.take("ack", at)
	return v.Granted, e
}
func (p *pauseNativeIO) Mailbox(at uint64) (uint8, error) {
	v, e := p.take("mailbox", at)
	return v.Value, e
}
func (p *pauseNativeIO) ReadStatus(at uint64, port uint8) (uint8, error) {
	v, e := p.take("status", at)
	if e == nil && v.Port != port {
		e = errors.New("native status port differs")
	}
	return v.Value, e
}
func (p *pauseNativeIO) WritePort(at uint64, port, value uint8) error {
	v, e := p.take("write", at)
	if e == nil && (v.Port != port || v.Value != value) {
		e = fmt.Errorf("native effective TL differs, got port%d value%d, native %+v", port, value, v)
	}
	return e
}
func (p *pauseNativeIO) WritePSG(at uint64, value uint8) error {
	p.psg = append(p.psg, psgAccess{At: at, Value: value})
	return nil
}

func TestPauseClockExistingNaturalMainIO(t *testing.T) {
	raw, err := os.ReadFile("../../verification/score-clock-pause/source/natural-pause-io.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	body, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Version int
		Cases   []struct {
			Frame      int
			Route      string
			Entry      scoreclock.Clock
			Pause      uint8
			PauseWrite struct {
				At    uint64
				Value uint8
			} `json:"pause_write"`
			Body   scoreclock.Clock
			Levels [6][4]uint8
			Trace  []access
		}
	}
	if err = json.Unmarshal(body, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 2 {
		t.Fatal("native pause fixture incomplete")
	}
	count := 0
	for _, row := range f.Cases {
		t.Run(row.Route, func(t *testing.T) {
			field := &pauseNativeFields{value: row.Pause}
			gate, err := scoreclock.EnterTick(row.Entry, field, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if gate.Clock != row.Body || len(field.writes) != 1 || field.writes[0] != (psgAccess{At: row.PauseWrite.At, Value: row.PauseWrite.Value}) {
				t.Fatal("native gate/body boundary differs", gate, field.writes)
			}
			e := &pauseNativeIO{rows: row.Trace}
			levels := rootcontrol.PauseLevels{Levels: row.Levels}
			captures := 0
			if row.Route == "start" {
				got, err := scoreclock.StartPause(gate.Clock, e, e, func(at uint64) (rootcontrol.PauseLevels, error) {
					captures++
					if at < gate.Clock.Cycle {
						return rootcontrol.PauseLevels{}, errors.New("capture precedes gate")
					}
					return levels, nil
				}, scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				if got.Levels != levels || captures != 1 || got.Clock.Cycle <= row.Trace[len(row.Trace)-1].At {
					t.Fatal("pause payload/final fixed tail incomplete")
				}
				if len(e.psg) != 4 {
					t.Fatal("PSG mute incomplete")
				}
				for i, value := range []uint8{0x9f, 0xbf, 0xdf, 0xff} {
					if e.psg[i].Value != value || e.psg[i].At <= row.Trace[len(row.Trace)-1].At {
						t.Fatal("PSG mute order differs")
					}
				}
			} else {
				got, err := scoreclock.ResumePause(gate.Clock, levels, e, e, scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				if got.Cycle <= row.Trace[len(row.Trace)-1].At || len(e.psg) != 0 {
					t.Fatal("resume duplicated PSG or missed return")
				}
			}
			if e.next != len(row.Trace) {
				t.Fatal("native Main IO left unconsumed", e.next, len(row.Trace))
			}
			count += e.next
		})
	}
	if count != 499 {
		t.Fatal("native pause IO coverage differs", count)
	}
}

type pauseMixedOwner struct {
	r                 *mixed.Runtime
	psgWrites, writes int
}

func (p *pauseMixedOwner) RequestBus(at uint64, on bool) (uint64, error) {
	return p.r.RequestBus(at, on)
}
func (p *pauseMixedOwner) BusGranted(at uint64) (bool, error) { return p.r.BusGranted(at) }
func (p *pauseMixedOwner) Mailbox(at uint64) (uint8, error)   { return p.r.Mailbox(at) }
func (p *pauseMixedOwner) ReadStatus(at uint64, port uint8) (uint8, error) {
	return p.r.ReadStatus(at, port)
}
func (p *pauseMixedOwner) WritePort(at uint64, port, value uint8) error {
	p.writes++
	return p.r.WritePort(at, port, value)
}
func (p *pauseMixedOwner) WritePSG(at uint64, value uint8) error {
	p.psgWrites++
	return p.r.WritePSG(at, value)
}
func newPauseMixed(t *testing.T) (*pauseMixedOwner, mixed.Config, rootcontrol.PauseLevels) {
	t.Helper()
	z, err := gzip.NewReader(bytes.NewReader(busNative))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	body, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Resource struct {
			ID, EncodedHex string
			Descriptor     pcmcommands.Descriptor
		}
	}
	if err = json.Unmarshal(body, &f); err != nil {
		t.Fatal(err)
	}
	payload, err := hex.DecodeString(f.Resource.EncodedHex)
	if err != nil {
		t.Fatal(err)
	}
	cfg := mixed.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}, PCM: pcmcommands.Config{IncludeYMAddresses: true, Resources: pcmcommands.Resources{Identity: f.Resource.ID, Length: func(id string) (uint32, bool) { return uint32(len(payload)), id == f.Resource.ID }, ByteAt: func(id string, at uint32) (byte, bool) {
		if id != f.Resource.ID || at >= uint32(len(payload)) {
			return 0, false
		}
		return payload[at], true
	}}, Descriptors: []pcmcommands.Descriptor{f.Resource.Descriptor}}}
	initial := chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()}
	for channel := range initial.YM.Channels {
		for op := range initial.YM.Channels[channel].Operators {
			initial.YM.Channels[channel].Operators[op].Level = uint8(3 + channel*9 + op)
		}
	}
	initial.PSG.Level = [4]uint16{2800, 2224, 1766, 1403}
	levels, err := rootcontrol.CapturePauseLevels(initial.YM)
	if err != nil {
		t.Fatal(err)
	}
	r, err := mixed.New(cfg, initial)
	if err != nil {
		t.Fatal(err)
	}
	return &pauseMixedOwner{r: r}, cfg, levels
}

func TestPauseClockActualMixedCaptureRestore(t *testing.T) {
	owner, cfg, before := newPauseMixed(t)
	captures := 0
	got, err := scoreclock.StartPause(scoreclock.Clock{Cycle: 1000, Refresh: 1100}, owner, owner, func(uint64) (rootcontrol.PauseLevels, error) {
		captures++
		if !owner.r.Status().Held {
			return rootcontrol.PauseLevels{}, errors.New("capture lacks actual bus hold")
		}
		return rootcontrol.CapturePauseLevels(owner.r.Snapshot().Chips.YM)
	}, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if captures != 1 || got.Levels != before || owner.psgWrites != 4 {
		t.Fatal("actual capture or PSG mute differs")
	}
	muted := owner.r.Snapshot()
	for _, channel := range muted.Chips.YM.Channels {
		for _, op := range channel.Operators {
			if op.Level != 127 {
				t.Fatal("actual FM not muted")
			}
		}
	}
	if muted.Chips.PSG.Level != ([4]uint16{}) {
		t.Fatal("actual PSG not muted")
	}
	if owner.r.Status().Held {
		t.Fatal("leaf invented a later world/IRQ frame hold")
	}
	checkpoint, err := json.Marshal(struct {
		Mixed  mixed.State
		Levels rootcontrol.PauseLevels
	}{owner.r.Snapshot(), got.Levels})
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Mixed  mixed.State
		Levels rootcontrol.PauseLevels
	}
	if err = json.Unmarshal(checkpoint, &saved); err != nil {
		t.Fatal(err)
	}
	owner.r, err = mixed.Restore(cfg, saved.Mixed)
	if err != nil {
		t.Fatal(err)
	}
	end, err := scoreclock.ResumePause(got.Clock, saved.Levels, owner, owner, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	after, err := rootcontrol.CapturePauseLevels(owner.r.Snapshot().Chips.YM)
	if err != nil || after != before || owner.psgWrites != 4 || owner.writes != 96 {
		t.Fatal("actual resume levels/port effects differ", err, owner.writes)
	}
	if saved.Levels != before || end.Cycle <= got.Clock.Cycle {
		t.Fatal("resume mutated payload or moved clock backwards")
	}
	if _, err = owner.r.RunUntil(chips.FrameCycles); err != nil {
		t.Fatal(err)
	}
	frame, err := owner.r.FrameBoundary()
	if err != nil || len(frame.PCM)/2 < 735 || len(frame.PCM)/2 > 736 {
		t.Fatal("actual mixed output did not flush", err)
	}
}

func TestPauseClockBudgetIsSharedAndDetached(t *testing.T) {
	original, cfg, _ := newPauseMixed(t)
	before := original.r.Snapshot()
	branch, err := mixed.Restore(cfg, before)
	if err != nil {
		t.Fatal(err)
	}
	work := &pauseMixedOwner{r: branch}
	got, err := scoreclock.StartPause(scoreclock.Clock{Cycle: 1000, Refresh: 1100}, work, work, func(uint64) (rootcontrol.PauseLevels, error) {
		return rootcontrol.CapturePauseLevels(work.r.Snapshot().Chips.YM)
	}, scoreclock.Limits{MaxOperations: 96})
	if !errors.Is(err, scoreclock.ErrBudget) || got != (scoreclock.PauseResult{}) || work.writes == 0 || work.writes > 8 {
		t.Fatal("nested services reset the shared budget", err, work.writes)
	}
	if !reflect.DeepEqual(before, original.r.Snapshot()) {
		t.Fatal("failed detached branch changed canonical owner")
	}
}

func TestPauseClockRejectsInvalidAndPropagatesCaptureError(t *testing.T) {
	for _, row := range []struct {
		clock  scoreclock.Clock
		limits scoreclock.Limits
	}{{scoreclock.Clock{Cycle: math.MaxUint64}, scoreclock.Limits{}}, {scoreclock.Clock{}, scoreclock.Limits{MaxOperations: 65537}}} {
		owner, _, _ := newPauseMixed(t)
		before := owner.r.Snapshot()
		got, err := scoreclock.StartPause(row.clock, owner, owner, func(uint64) (rootcontrol.PauseLevels, error) { return rootcontrol.PauseLevels{}, nil }, row.limits)
		if err == nil || got != (scoreclock.PauseResult{}) || owner.writes != 0 || !reflect.DeepEqual(before, owner.r.Snapshot()) {
			t.Fatal("invalid pause input changed external owners")
		}
	}
	owner, _, levels := newPauseMixed(t)
	before := owner.r.Snapshot()
	levels.Levels[0][0] = 128
	if _, err := scoreclock.ResumePause(scoreclock.Clock{}, levels, owner, owner, scoreclock.Limits{}); err == nil || !reflect.DeepEqual(before, owner.r.Snapshot()) {
		t.Fatal("invalid saved level accepted")
	}
	if _, err := scoreclock.StartPause(scoreclock.Clock{}, owner, owner, nil, scoreclock.Limits{}); err == nil || !reflect.DeepEqual(before, owner.r.Snapshot()) {
		t.Fatal("missing capture accepted")
	}
	failure := errors.New("declared capture failure")
	got, err := scoreclock.StartPause(scoreclock.Clock{}, owner, owner, func(uint64) (rootcontrol.PauseLevels, error) { return rootcontrol.PauseLevels{}, failure }, scoreclock.Limits{})
	if !errors.Is(err, failure) || got != (scoreclock.PauseResult{}) || owner.writes != 0 {
		t.Fatal("capture error swallowed or returned partial result", err)
	}
}
