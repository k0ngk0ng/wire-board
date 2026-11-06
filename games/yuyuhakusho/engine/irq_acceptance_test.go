package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
	"io"
	"math"
	"os"
	"reflect"
	"testing"
)

type irqAcceptanceRow struct {
	Mask, Level uint8
	Origin      uint64
	Accepted    bool
	EntryCycle  uint64 `json:"entry_cycle"`
	MaskAfter   uint8  `json:"mask_after"`
}
type irqArenaRow struct {
	Stage                     uint16
	Pan                       int16
	Used                      uint16
	Origin, Refresh, Deadline uint64
	BeforeIRQ                 uint64 `json:"before_irq"`
	EntryCycle                uint64 `json:"entry_cycle"`
	RefreshAfter              uint64 `json:"refresh_after"`
}

func irqAcceptanceFixture(t *testing.T) ([]irqAcceptanceRow, []irqArenaRow) {
	t.Helper()
	p := "../verification/world-irq-acceptance/"
	packed, err := os.ReadFile(p + "native-irq-acceptance.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(p + "capture.json")
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Pass       bool
		FixtureSHA string `json:"fixture_sha256"`
	}
	if err := json.Unmarshal(receipt, &report); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(packed)
	if !report.Pass || hex.EncodeToString(digest[:]) != report.FixtureSHA {
		t.Fatal("unbound IRQ fixture")
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Version int
		Cases   []irqAcceptanceRow
		Arena   []irqArenaRow `json:"arena_cases"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Version != 1 || len(data.Cases) != 1680 || len(data.Arena) != 32 {
		t.Fatal("IRQ acceptance scope missing")
	}
	return data.Cases, data.Arena
}
func TestWorldIRQAcceptanceOriginalNormalBoundary(t *testing.T) {
	rows, _ := irqAcceptanceFixture(t)
	accepted := 0
	for _, row := range rows {
		got, err := AcceptWorldIRQ(row.Origin, row.Level, row.Mask)
		if err != nil || got.Accepted != row.Accepted || got.Cycle != row.EntryCycle || got.Mask != row.MaskAfter {
			t.Fatal("original IRQ boundary differs", row, got, err)
		}
		if got.Accepted {
			accepted++
		}
	}
	if accepted != 700 {
		t.Fatal("missing blocked and accepted coverage", accepted)
	}
}
func TestWorldIRQAcceptanceAtActualArenaDeadline(t *testing.T) {
	_, rows := irqAcceptanceFixture(t)
	library, _ := arenaWorkFixture(t)
	stop := errors.New("accepted external IRQ")
	for _, row := range rows {
		video, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, videoclock.State{Counter: 9, Reload: 9, HorizontalEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := video.BeginFrame(); err != nil {
			t.Fatal(err)
		}
		if _, err := video.VerticalEdge(); err != nil {
			t.Fatal(err)
		}
		deadline := uint64(0)
		for i := 0; i < 261; i++ {
			signal, err := video.NextLine()
			if err != nil {
				t.Fatal(err)
			}
			if signal.Horizontal {
				deadline = uint64(signal.Cycle)
				break
			}
		}
		if deadline != row.Deadline {
			t.Fatal("VDP first HINT differs", deadline, row.Deadline)
		}
		world := ChuWorldState{Objects: WorldObjects{0x8180: {ObjectID: 0x8180, Header: MatchObjectHeader{Behavior: 0x20918, Size: 64, Stage: row.Stage}}}, Clock: WorldClockState{PanOffset: uint16(row.Pan)}, Camera: DefaultArenaCamera{X: 864, Y: 16, Used: row.Used}, SoundQueue: [4]uint8{4}}
		for i := 0; i < 4; i++ {
			world.Camera.SamplesX[i] = 900 << 16
			world.Camera.SamplesY[i] = 320 << 16
		}
		cycle, refresh := row.Origin, row.Refresh
		calls, entries := 0, 0
		got, events, err := StepChuWorldObjectsWithHooks(world, ChuWorldView{}, ChuWorldLibrary{Arena: library}, ChuWorldHooks{Work: func(op WorldWorkOperation, owners WorldSoundOwners) error {
			calls++
			if owners.Queue == nil || op.ObjectID != 0x8180 {
				t.Fatal("missing actual world work owner")
			}
			if cycle >= deadline {
				entries++
				if cycle != row.BeforeIRQ || refresh != row.RefreshAfter {
					t.Fatal("wrong source work boundary", row, cycle, refresh, op)
				}
				entered, err := AcceptWorldIRQ(cycle, 4, 0)
				if err != nil || !entered.Accepted || entered.Cycle != row.EntryCycle || entered.Mask != 4 {
					t.Fatal("native arena IRQ entry differs", row, entered, err)
				}
				return stop
			}
			next, after, err := op.Work.Advance(cycle, refresh)
			if err != nil {
				return err
			}
			cycle, refresh = next, after
			return nil
		}})
		if calls < 2 || entries != 1 || !errors.Is(err, stop) || events != nil || !reflect.DeepEqual(got, world) {
			t.Fatal("interior IRQ stop leaked world", row, calls, entries, err)
		}
	}
}
func TestWorldIRQAcceptanceRejectsInvalidAndOverflow(t *testing.T) {
	for _, input := range []struct{ Level, Mask uint8 }{{1, 0}, {2, 0}, {3, 0}, {5, 0}, {7, 0}, {4, 8}, {6, 255}} {
		if _, err := AcceptWorldIRQ(0, input.Level, input.Mask); err == nil {
			t.Fatal("invalid battle IRQ accepted", input)
		}
	}
	if _, err := AcceptWorldIRQ(math.MaxUint64-1, 4, 0); err == nil {
		t.Fatal("IRQ clock overflow accepted")
	}
	got, err := AcceptWorldIRQ(math.MaxUint64, 4, 7)
	if err != nil || got.Accepted || got.Cycle != math.MaxUint64 || got.Mask != 7 {
		t.Fatal("blocked IRQ changed state", got, err)
	}
}
