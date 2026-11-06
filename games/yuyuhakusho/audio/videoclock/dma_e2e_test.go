package videoclock_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
)

func dmaFixture(t *testing.T, name string, dst any) {
	t.Helper()
	raw, err := os.ReadFile("../../verification/world-dma/" + name)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err := json.NewDecoder(z).Decode(dst); err != nil {
		t.Fatal(err)
	}
}
func dmaTarget(t *testing.T, name string) videoclock.DMATarget {
	t.Helper()
	switch name {
	case "VRAM":
		return videoclock.DMAVRAM
	case "CRAM":
		return videoclock.DMACRAM
	case "VSRAM":
		return videoclock.DMAVSRAM
	}
	t.Fatal("unknown native DMA target", name)
	return 0
}
func TestWorldDMAActualNativeSegmentEffects(t *testing.T) {
	var fixture struct {
		Version int
		Cases   []struct {
			Input struct {
				Target                string
				Remaining, At         uint32
				Line                  uint16
				VBlank, AuxWaiting    bool
				MainBefore, AuxBefore uint32
			}
			End, Remaining, MainAfter, AuxAfter uint32
			AuxWaitingAfter                     bool
			NativeMode                          uint8
		}
	}
	dmaFixture(t, "native-dma-segments.json.gz", &fixture)
	if fixture.Version != 1 || len(fixture.Cases) != 2592 {
		t.Fatal("native DMA segment matrix missing")
	}
	backwardAssignments := 0
	for index, row := range fixture.Cases {
		state := videoclock.State{Phase: videoclock.Scanlines, Line: row.Input.Line}
		if state.Line == 0 {
			state.Phase = videoclock.PreVertical
		}
		c, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, state)
		if err != nil {
			t.Fatal(err)
		}
		before := c.Snapshot()
		got, err := c.AdvanceForcedBlankDMA(dmaTarget(t, row.Input.Target), row.Input.Remaining, row.Input.At, row.Input.AuxWaiting)
		if err != nil {
			t.Fatal(index, err)
		}
		main, aux := got.End, row.Input.AuxBefore
		if got.MoveAux {
			aux = got.End
		}
		if got.End != row.End || got.Remaining != row.Remaining || got.Mode != row.NativeMode || main != row.MainAfter || aux != row.AuxAfter || got.AuxWaiting != row.AuxWaitingAfter || c.Snapshot() != before {
			t.Fatal("actual DMA segment differs", index, got, row)
		}
		if main < row.Input.MainBefore {
			backwardAssignments++
		}
	}
	if backwardAssignments == 0 {
		t.Fatal("reference clock overwrite edge not exercised")
	}
}

type dmaWorldEvent map[string]uint64

func worldVideo(e dmaWorldEvent) videoclock.State {
	phase := videoclock.Scanlines
	if e["line"] == 0 {
		phase = videoclock.PreVertical
		if e["cycle"] >= 788 {
			phase = videoclock.PostVertical
		}
	}
	return videoclock.State{Phase: phase, Line: uint16(e["line"] / 3420), Counter: uint8(e["counter"]), Reload: uint8(e["reload"]), HorizontalEnabled: e["reg0"]&16 != 0, VerticalEnabled: e["reg1"]&32 != 0, DisplayEnabled: e["reg1"]&64 != 0, HorizontalPending: e["hpending"] != 0, VerticalPending: e["vpending"] != 0}
}

// The source oracle is a real interrupted wrapper invocation. Each DMA starts
// from its observed entry, but its complete transfer duration/line progression
// is independently calculated from the typed length/target. No original CPU or
// observed DMA end is passed to the production calculator.
func TestWorldDMAOriginalWrapperTransfersAndCrossFrameProgress(t *testing.T) {
	var fixture struct {
		Version int
		Fields  []string
		Cases   []struct {
			Frame     int
			EndFrame  int `json:"end_frame"`
			Events    [][]uint64
			Transfers []uint16
		}
	}
	dmaFixture(t, "native-world-dma.json.gz", &fixture)
	if fixture.Version != 1 || len(fixture.Cases) != 19 {
		t.Fatal("real World wrapper oracle missing")
	}
	transfers, crosses, crossTransfers := 0, 0, 0
	for caseIndex, row := range fixture.Cases {
		t.Run(fmt.Sprint(caseIndex), func(t *testing.T) {
			if row.EndFrame != row.Frame {
				crosses++
			}
			var cpu []dmaWorldEvent
			for _, values := range row.Events {
				e := dmaWorldEvent{}
				for i, name := range fixture.Fields {
					e[name] = values[i]
				}
				if e["kind"] == 1 || e["kind"] == 10 {
					cpu = append(cpu, e)
				}
			}
			item := 0
			for i, e := range cpu {
				if e["pc"] != 0x688 {
					continue
				}
				if i+1 >= len(cpu) || cpu[i+1]["pc"] != 0x68a || (item+1)*8 > len(row.Transfers) {
					t.Fatal("missing original DMA resume", item)
				}
				next := cpu[i+1]
				words := row.Transfers[item*8 : (item+1)*8]
				item++
				length := uint32(words[0]&255) | uint32(words[1]&255)<<8
				if length == 0 {
					length = 65536
				}
				code := (words[6]>>14)&3 | (words[7]>>2)&0x3c
				var target videoclock.DMATarget
				switch code & 15 {
				case 1:
					target = videoclock.DMAVRAM
				case 3:
					target = videoclock.DMACRAM
				case 5:
					target = videoclock.DMAVSRAM
				default:
					t.Fatal("unexpected real DMA destination", code)
				}
				c, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, worldVideo(e))
				if err != nil {
					t.Fatal(err)
				}
				frame := uint64(0)
				at := uint32(e["cycle"])
				if int32(at) >= int32(e["refresh"]) {
					at += 14
				}
				got, err := c.AdvanceForcedBlankDMA(target, length, at, e["bus_state"]&4 != 0)
				if err != nil {
					t.Fatal(err)
				}
				main := uint64(got.End) + 12*7
				advanceVideo := func(until uint64) {
					for {
						if c.Snapshot().Phase == videoclock.PreVertical {
							if until < frame*896040+788 {
								return
							}
							if _, err := c.VerticalEdge(); err != nil {
								t.Fatal(err)
							}
							continue
						}
						if until < frame*896040+uint64(c.LineCycle())+3420 {
							return
						}
						if _, err := c.NextLine(); err != nil {
							t.Fatal(err)
						}
						if c.Snapshot().Phase == videoclock.BetweenFrames {
							frame++
							if _, err := c.BeginFrame(); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				for got.Remaining != 0 {
					// The next VDP batch overwrites the reference main cursor. The
					// previous instruction's tail is not added again after every line.
					advanceVideo(main)
					got, err = c.AdvanceForcedBlankDMA(target, got.Remaining, c.LineCycle(), got.AuxWaiting)
					if err != nil {
						t.Fatal(err)
					}
					main = frame*896040 + uint64(got.End)
				}
				advanceVideo(main)
				wantFrame := next["frame_index"] - e["frame_index"]
				if frame != wantFrame || main != wantFrame*896040+next["cycle"] || got.End != uint32(next["dma_end"]) || c.Snapshot() != worldVideo(next) {
					t.Fatal("actual World DMA/resume differs", item, "main", main, next, "DMA", got, "video", c.Snapshot(), worldVideo(next))
				}
				transfers++
				if frame != 0 {
					crossTransfers++
				}
			}
			if item != len(row.Transfers)/8 && cpu[len(cpu)-1]["pc"] == 0x3a2e {
				t.Fatal("not all real DMA items consumed", item)
			}
		})
	}
	if transfers != 89 || crosses != 6 || crossTransfers != 4 {
		t.Fatal("real World DMA coverage differs", transfers, crosses, crossTransfers)
	}
}

func TestWorldDMARejectsUnsupportedInputsBeforeChangingVideo(t *testing.T) {
	for _, kind := range []string{"target", "zero", "length", "display", "phase", "early", "late"} {
		t.Run(kind, func(t *testing.T) {
			state := videoclock.State{Phase: videoclock.Scanlines, Line: 40}
			target, length, at := videoclock.DMAVRAM, uint32(1), uint32(40*3420)
			switch kind {
			case "target":
				target = 0
			case "zero":
				length = 0
			case "length":
				length = 65537
			case "display":
				state.DisplayEnabled = true
			case "phase":
				state.Phase, state.Line = videoclock.BetweenFrames, 0
			case "early":
				at--
			case "late":
				at += 3420 + 14
			}
			c, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, state)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.AdvanceForcedBlankDMA(target, length, at, false); err == nil || c.Snapshot() != state {
				t.Fatal("unsupported DMA changed owner", err)
			}
		})
	}
}
