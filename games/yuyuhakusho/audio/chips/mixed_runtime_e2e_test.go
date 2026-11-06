package chips_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func mixedPlan(c pcmIntegrationSourceCase, index int) pcmc.FramePlan {
	plan := pcmc.FramePlan{}
	for _, in := range c.Inputs {
		if in.Begin/pcmc.FrameCycles == uint64(index) {
			plan.Inputs = append(plan.Inputs, in)
		}
	}
	for _, dma := range c.DMA {
		if dma.Frame == uint64(index) {
			plan.DMA = append(plan.DMA, dma.Interval)
		}
	}
	return plan
}

func TestMixedRuntimeOriginalFullPCMChipMatrix(t *testing.T) {
	sources, native := pcmIntegrationCases(t)
	frames, events, stereo := 0, 0, 0
	for _, c := range sources {
		t.Run(c.Name, func(t *testing.T) {
			want := native[c.Name]
			cfg := mixed.Config{PCM: pcmIntegrationConfig(t, c), Chips: want.Config}
			r, err := mixed.New(cfg, want.Initial)
			if err != nil {
				t.Fatal(err)
			}
			var transactions []pcmc.Transaction
			for index, observed := range c.Frames {
				raw, err := json.Marshal(r.Snapshot())
				if err != nil {
					t.Fatal(err)
				}
				var saved mixed.State
				if err = json.Unmarshal(raw, &saved); err != nil {
					t.Fatal(err)
				}
				r, err = mixed.Restore(cfg, saved)
				if err != nil {
					t.Fatal(err)
				}
				got, err := r.AdvanceFrame(mixedPlan(c, index))
				if err != nil {
					t.Fatalf("frame%d: %v", index, err)
				}
				if !reflect.DeepEqual(pcmIntegrationNormalize(got.Protocol.Ledger), pcmIntegrationNormalize(observed.Events)) || got.Protocol.Status != observed.Status {
					t.Fatalf("frame%d physical ports/status differ", index)
				}
				equalPCM(t, got.PCM, want.Frames[index].PCM)
				chip, err := chips.RestoreInteractive(cfg.Chips, r.Snapshot().Chips)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := chip.CommittedState()
				if err != nil || !reflect.DeepEqual(actual, want.Frames[index].State) {
					t.Fatalf("frame%d complete YM/PSG/FIR state differs: %v", index, err)
				}
				if len(r.Snapshot().PCM.PendingLedger) != 0 {
					t.Fatal("consumed ledger persisted")
				}
				transactions = append(transactions, got.Protocol.Transactions...)
				frames++
				events += len(got.Protocol.Ledger)
				stereo += len(got.PCM) / 2
			}
			if !reflect.DeepEqual(transactions, c.Transactions) {
				t.Fatal("hardware transactions differ")
			}
		})
	}
	if frames != 1274 || events != 519256 || stereo != 937609 {
		t.Fatalf("incomplete matrix: frames%d ports%d stereo%d", frames, events, stereo)
	}
	t.Logf("original cases23 frames%d physical-events%d stereo-frames%d; JSON restore before every frame", frames, events, stereo)
}

func TestMixedRuntimeRejectsMismatchedAndAliasedCheckpoints(t *testing.T) {
	sources, native := pcmIntegrationCases(t)
	c := sources[0]
	want := native[c.Name]
	cfg := mixed.Config{PCM: pcmIntegrationConfig(t, c), Chips: want.Config}
	r, err := mixed.New(cfg, want.Initial)
	if err != nil {
		t.Fatal(err)
	}
	semantic := cfg
	semantic.PCM.IncludeYMAddresses = false
	if _, err = mixed.New(semantic, want.Initial); err == nil {
		t.Fatal("semantic-only PCM output accepted")
	}
	before := r.Snapshot()
	for _, change := range []struct {
		name  string
		apply func(*mixed.State)
	}{
		{"schema", func(s *mixed.State) { s.Version++ }},
		{"chip-profile", func(s *mixed.State) { s.ChipConfig.MasterOrigin++ }},
		{"frame", func(s *mixed.State) { s.PCM.Frame++ }},
		{"pending", func(s *mixed.State) { s.PCM.PendingLedger = []pcmc.Event{{Kind: pcmc.Dummy}} }},
		{"catalog", func(s *mixed.State) { s.PCM.Identity = "other" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			s := r.Snapshot()
			change.apply(&s)
			if _, e := mixed.Restore(cfg, s); e == nil {
				t.Fatal("invalid combined checkpoint accepted")
			}
		})
	}
	copy := r.Snapshot()
	copy.Chips.FIR[0][0]++
	if !reflect.DeepEqual(before, r.Snapshot()) {
		t.Fatal("snapshot FIR aliases runtime")
	}
	copy = r.Snapshot()
	restored, err := mixed.Restore(cfg, copy)
	if err != nil {
		t.Fatal(err)
	}
	copy.Chips.FIR[0][0]++
	if !reflect.DeepEqual(before, restored.Snapshot()) {
		t.Fatal("restore borrows mutable FIR")
	}
	if err = r.WritePort(cfg.PCM.Origin+100, 4, 0); err == nil {
		t.Fatal("invalid YM port accepted")
	}
	if !reflect.DeepEqual(before, r.Snapshot()) {
		t.Fatal("rejected chip access changed combined state")
	}
}

func TestMixedRuntimeFrameFailureIsAtomic(t *testing.T) {
	sources, native := pcmIntegrationCases(t)
	for _, c := range sources {
		if len(c.Frames[0].Events) < 3 {
			continue
		}
		want := native[c.Name]
		cfg := mixed.Config{PCM: pcmIntegrationConfig(t, c), Chips: want.Config}
		r, err := mixed.New(cfg, want.Initial)
		if err != nil {
			t.Fatal(err)
		}
		s := r.Snapshot()
		s.Chips.InputCount = chips.MaxPendingEvents - 1
		r, err = mixed.Restore(cfg, s)
		if err != nil {
			t.Fatal(err)
		}
		before := r.Snapshot()
		got, err := r.AdvanceFrame(mixedPlan(c, 0))
		if err == nil {
			t.Fatal("chip capacity failure missing")
		}
		if len(got.PCM) != 0 || len(got.Protocol.Ledger) != 0 || !reflect.DeepEqual(before, r.Snapshot()) {
			t.Fatal("partial combined frame committed")
		}
		return
	}
	t.Fatal("no original capacity failure candidate")
}

func TestPCMDeviceForkBorrowsResourcesAndOwnsPendingOutput(t *testing.T) {
	sources, _ := pcmIntegrationCases(t)
	cfg := pcmIntegrationConfig(t, sources[0])
	reads := 0
	read := cfg.Resources.ByteAt
	length := cfg.Resources.Length
	cfg.Resources.ByteAt = func(id string, at uint32) (byte, bool) { reads++; return read(id, at) }
	cfg.Resources.Length = func(id string) (uint32, bool) { reads++; return length(id) }
	d, err := pcmc.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources[0].Inputs) == 0 || sources[0].Inputs[0].Kind != pcmc.Ordinary || sources[0].Inputs[0].Begin != 0 || sources[0].Inputs[0].End != 0 {
		t.Fatal("original initial ordinary publication missing")
	}
	if _, err = d.RequestBus(0, true); err != nil {
		t.Fatal(err)
	}
	if err = d.PublishAccepted(sources[0].Inputs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = d.RequestBus(0, false); err != nil {
		t.Fatal(err)
	}
	if _, err = d.RunUntil(50000); err != nil {
		t.Fatal(err)
	}
	before := d.Snapshot()
	count := reads
	branch := d.Fork()
	if reads != count || !reflect.DeepEqual(before, branch.Snapshot()) {
		t.Fatal("Fork reauthenticated resources or changed state")
	}
	ledger := branch.DrainLedger()
	if len(ledger) == 0 {
		t.Fatal("native resource produced no pending physical output")
	}
	for i := range ledger {
		if ledger[i].Bus != nil {
			ledger[i].Bus.M68KDelay++
			break
		}
	}
	if !reflect.DeepEqual(before, d.Snapshot()) {
		t.Fatal("branch drained or mutated original pending ledger")
	}
	if _, err = branch.RequestBus(50000, true); err != nil {
		t.Fatal(err)
	}
	if d.Status().Held {
		t.Fatal("fork acquired original hardware bus")
	}
}

type mixedAccess struct {
	Kind              string
	At, After         uint64
	Port, Value       uint8
	Asserted, Granted bool
}
type mixedObservedBus struct {
	r     *mixed.Runtime
	trace []mixedAccess
}

func (e *mixedObservedBus) RequestBus(at uint64, on bool) (uint64, error) {
	v, err := e.r.RequestBus(at, on)
	e.trace = append(e.trace, mixedAccess{Kind: "request", At: at, After: v, Asserted: on})
	return v, err
}
func (e *mixedObservedBus) BusGranted(at uint64) (bool, error) {
	v, err := e.r.BusGranted(at)
	e.trace = append(e.trace, mixedAccess{Kind: "ack", At: at, Granted: v})
	return v, err
}
func (e *mixedObservedBus) Mailbox(at uint64) (uint8, error) {
	v, err := e.r.Mailbox(at)
	e.trace = append(e.trace, mixedAccess{Kind: "mailbox", At: at, Value: v})
	return v, err
}
func (e *mixedObservedBus) ReadStatus(at uint64, p uint8) (uint8, error) {
	v, err := e.r.ReadStatus(at, p)
	e.trace = append(e.trace, mixedAccess{Kind: "status", At: at, Port: p, Value: v})
	return v, err
}
func (e *mixedObservedBus) WritePort(at uint64, p, v uint8) error {
	err := e.r.WritePort(at, p, v)
	e.trace = append(e.trace, mixedAccess{Kind: "write", At: at, Port: p, Value: v})
	return err
}

func TestMixedRuntimeOriginalBusService(t *testing.T) {
	var f struct {
		Version  int
		Resource struct {
			ID, EncodedHex string
			Descriptor     pcmc.Descriptor
		}
		Cases []struct {
			Input struct {
				ID                    string
				Origin                uint64
				Bank, Register, Value uint8
			}
			Before struct {
				Frontier, M68KDelay uint64
				Busy                uint8
			}
			Main []mixedAccess
			End  struct {
				Clock    scoreclock.Clock
				Frontier uint64
				Status   pcmc.Status
			}
		}
	}
	pcmIntegrationJSON(t, "../scoreclock/testdata/bus-native-services.json.gz", &f)
	if f.Version != 1 || len(f.Cases) != 4 {
		t.Fatal("original bus matrix incomplete")
	}
	payload, err := hex.DecodeString(f.Resource.EncodedHex)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != f.Resource.ID {
		t.Fatal("resource differs")
	}
	pcfg := pcmc.Config{IncludeYMAddresses: true, Resources: pcmc.Resources{Identity: f.Resource.ID, Length: func(id string) (uint32, bool) { return uint32(len(payload)), id == f.Resource.ID }, ByteAt: func(id string, at uint32) (byte, bool) {
		if id != f.Resource.ID || at >= uint32(len(payload)) {
			return 0, false
		}
		return payload[at], true
	}}, Descriptors: []pcmc.Descriptor{f.Resource.Descriptor}}
	main, busy := 0, 0
	for _, c := range f.Cases {
		t.Run(c.Input.ID, func(t *testing.T) {
			cfg := mixed.Config{PCM: pcfg, Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
			r, err := mixed.New(cfg, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.RequestBus(0, true); err != nil {
				t.Fatal(err)
			}
			if err = r.PublishAccepted(pcmc.Input{Kind: pcmc.Ordinary, DescriptorID: f.Resource.Descriptor.ID}); err != nil {
				t.Fatal(err)
			}
			if _, err = r.RequestBus(0, false); err != nil {
				t.Fatal(err)
			}
			initial, err := r.RunUntil(c.Input.Origin)
			if err != nil {
				t.Fatal(err)
			}
			if initial.Frontier != c.Before.Frontier || initial.M68KDelay != c.Before.M68KDelay || r.Status().Busy != c.Before.Busy {
				t.Fatal("pre-service state differs")
			}
			// Restore an actual in-flight PCM/chip/FIR state before the real service.
			r, err = mixed.Restore(cfg, r.Snapshot())
			if err != nil {
				t.Fatal(err)
			}
			e := &mixedObservedBus{r: r}
			got, err := scoreclock.ServeYM(scoreclock.Clock{Cycle: c.Input.Origin, Refresh: c.Input.Origin}, scoreclock.Request{Bank: c.Input.Bank, Register: c.Input.Register, Value: c.Input.Value}, e, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Clock != c.End.Clock || r.Snapshot().PCM.Time != c.End.Frontier || r.Status() != c.End.Status || !reflect.DeepEqual(e.trace, c.Main) {
				t.Fatal("original Main access/end state differs")
			}
			for _, v := range e.trace {
				if v.Kind == "mailbox" && v.Value != 0 {
					busy++
				}
			}
			main += len(e.trace)
		})
	}
	if main != 71 || busy != 7 {
		t.Fatalf("incomplete real service Main%d busy%d", main, busy)
	}
}
