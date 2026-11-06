package scoreclock_test

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

var musicConfigOnce sync.Once
var musicConfigValue mixed.Config

func musicConfig(t *testing.T) mixed.Config {
	t.Helper()
	musicConfigOnce.Do(func() {
		library, err := resourcebridge.DecodeWithInstrumentWork(
			parserClockUnpack(t, "../../data/audio-score.json.gz"),
			parserClockUnpack(t, "../../data/audio-pcm-payloads.json.gz"),
			parserClockUnpack(t, "../../data/audio-score-parser-work.json.gz"),
			parserClockUnpack(t, "../../data/audio-score-instrument-work.json.gz"))
		if err != nil {
			t.Fatal(err)
		}
		pcm, err := library.PCMConfig(0, 64)
		if err != nil {
			t.Fatal(err)
		}
		musicConfigValue = mixed.Config{PCM: pcm, Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	})
	return musicConfigValue
}

type musicIO struct {
	*panIO
	publications               []pcmcommands.Input
	failFlags, failPublication bool
}

func (e *musicIO) MusicDescriptor(id string) (pcmcommands.Descriptor, bool) {
	return e.r.MusicDescriptor(id)
}
func (e *musicIO) MusicFlags(at uint64) (uint8, error) {
	if e.failFlags {
		return 0, errors.New("actual flags read failed")
	}
	value, err := e.r.MusicFlags(at)
	e.trace = append(e.trace, access{Kind: "music-flags", At: at, Value: value})
	return value, err
}
func (e *musicIO) PublishAcceptedAt(at uint64, input pcmcommands.Input) error {
	if e.failPublication {
		return errors.New("actual publication failed")
	}
	err := e.r.PublishAcceptedAt(at, input)
	if err == nil {
		e.publications = append(e.publications, input)
		e.trace = append(e.trace, access{Kind: "publish", At: at, Value: input.Pan})
	}
	return err
}

func newMusicIO(t *testing.T, marker, flags uint8) (*musicIO, mixed.Config) {
	t.Helper()
	cfg := musicConfig(t)
	r, err := mixed.New(cfg, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.RequestBus(0, true); err != nil {
		t.Fatal(err)
	}
	state := r.Snapshot()
	state.PCM.Global = pcmcommands.Globals{StartMarker: marker, ModeFlags: flags, Busy: 9,
		RestorePan: 64, MusicPan: 64, RateRequest: 17, LatestFlags: 5}
	r, err = mixed.Restore(cfg, state)
	if err != nil {
		t.Fatal(err)
	}
	return &musicIO{panIO: &panIO{instrumentIO: &instrumentIO{r: r}}}, cfg
}

func musicDescriptor(t *testing.T, cfg mixed.Config) pcmcommands.Descriptor {
	t.Helper()
	for _, d := range cfg.PCM.Descriptors {
		if d.Protocol == pcmcommands.MusicProtocol {
			return d
		}
	}
	t.Fatal("formal music descriptor absent")
	return pcmcommands.Descriptor{}
}

func TestMusicPublicationOriginalCommitAndActualResources(t *testing.T) {
	var f struct {
		Version              int
		DescriptorProvenance map[string]struct{ Bytes string } `json:"descriptor_provenance"`
		Cases                []struct {
			Input struct {
				ID                 string
				Descriptor         pcmcommands.Descriptor
				Marker, Flags, Pan uint8
				Origin, Refresh    uint64
			}
			Events []instrumentNativeEvent
			End    scoreclock.Clock
			Fields map[string]uint8
			Equal  bool `json:"native_observer_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-music/native-music-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 288 || len(f.DescriptorProvenance) != 6 {
		t.Fatal("original music matrix incomplete")
	}
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("observer changed native result")
			}
			input := row.Input
			e, _ := newMusicIO(t, input.Marker, input.Flags)
			d, ok := e.r.MusicDescriptor(input.Descriptor.ID)
			if !ok || d != input.Descriptor {
				t.Fatal("actual production resource differs from source descriptor", d, input.Descriptor)
			}
			before := e.r.Snapshot()
			req := scoreclock.MusicRequest{DescriptorID: d.ID, Pan: panValue(input.Pan)}
			tx, err := scoreclock.BeginMusicNotePublication(scoreclock.Clock{Cycle: input.Origin, Refresh: input.Refresh}, req, e, scoreclock.Limits{})
			if err != nil {
				t.Fatal(err)
			}
			blocked := input.Marker&128 != 0
			feedback := score.DACFeedback{Blocked: blocked}
			if !blocked {
				feedback.Priority = input.Flags & 192
			}
			if tx.Feedback() != feedback || len(e.publications) != 0 || !reflect.DeepEqual(before.PCM.Global, e.r.Snapshot().PCM.Global) {
				t.Fatal("stage feedback differs or preemptively published music")
			}
			expected, accepted := tx.ExpectedInput()
			wantAccepted := !blocked && d.Flags&192 >= feedback.Priority
			if accepted != wantAccepted {
				t.Fatal("original arbitration differs")
			}
			var stores []instrumentNativeEvent
			for _, event := range row.Events {
				if event.Kind == 20 {
					stores = append(stores, event)
				}
			}
			want := instrumentAccesses(t, row.Events)
			for _, event := range row.Events {
				if event.Kind == 17 {
					want = append(want, access{Kind: "pan-request", At: event.After, Value: uint8(event.Value)})
				}
				if event.Kind == 19 {
					want = append(want, access{Kind: "music-flags", At: event.After, Value: uint8(event.Value)})
				}
			}
			if accepted {
				wantInput := pcmcommands.Input{Kind: pcmcommands.Music, DescriptorID: d.ID, Pan: input.Pan}
				if expected != wantInput || len(stores) != 8 {
					t.Fatal("accepted music effect or source store count differs", expected, wantInput, len(stores))
				}
				raw, err := hex.DecodeString(f.DescriptorProvenance[d.ID].Bytes)
				if err != nil || len(raw) != 8 {
					t.Fatal("source descriptor binding absent", err)
				}
				addresses := []uint32{0x1ffd, 0x1fe8, 0x1fe9, 0x1fe6, 0x1fe7, 0x1ffe, 0x1ffb, 0x1ff9}
				values := append(append([]byte{1}, raw[:6]...), input.Pan)
				for i, store := range stores {
					if store.Address&0xffff != addresses[i] || uint8(store.Value) != values[i] {
						t.Fatal("original held metadata group differs from bound resource", i)
					}
				}
				if err = tx.Apply(expected); err != nil {
					t.Fatal(err)
				}
				wantInput.Begin, wantInput.End = stores[0].After, stores[7].After
				if !reflect.DeepEqual(e.publications, []pcmcommands.Input{wantInput}) {
					t.Fatal("atomic publication lost original first/last store boundaries", e.publications, wantInput)
				}
				want = append(want, access{Kind: "publish", At: stores[7].After, Value: input.Pan})
			} else if len(stores) != 0 {
				t.Fatal("rejected original music unexpectedly wrote metadata")
			}
			end, err := tx.Finish()
			if err != nil || end != row.End {
				t.Fatal("complete DAC commit final clock differs", end, row.End, err)
			}
			sort.SliceStable(want, func(i, j int) bool { return want[i].At < want[j].At })
			if !reflect.DeepEqual(e.trace, want) {
				t.Fatal("actual mixed arbitration/bus/publication trace differs", e.trace, want)
			}
			state := e.r.Snapshot()
			g := state.PCM.Global
			fields := map[string]uint8{"8189": g.StartMarker, "8188": g.ModeFlags, "8184": g.RestorePan,
				"8185": g.MusicPan, "8190": g.RateRequest, "8187": g.LatestFlags, "7978": g.Busy}
			if !reflect.DeepEqual(fields, row.Fields) || state.PCM.Held != nil || !reflect.DeepEqual(state.Chips, before.Chips) {
				t.Fatal("canonical PCM metadata or held/chip state differs", fields, row.Fields)
			}
			if accepted && g.SingleDescriptor != d.ID || !accepted && g.SingleDescriptor != before.PCM.Global.SingleDescriptor {
				t.Fatal("sample identity changed outside accepted publication")
			}
		})
	}
}

func TestMusicPublicationStagedAndContinuationShareCosts(t *testing.T) {
	for _, pair := range [][2]uint8{{0, 0}, {1, 33}, {0, 64}, {128, 0}} {
		left, cfg := newMusicIO(t, pair[0], pair[1])
		right, _ := newMusicIO(t, pair[0], pair[1])
		d := musicDescriptor(t, cfg)
		req := scoreclock.MusicRequest{DescriptorID: d.ID, Pan: panValue(181)}
		a, err := scoreclock.ServeMusicNotePublication(scoreclock.Clock{}, req, left, scoreclock.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		prefix, err := scoreclock.PriceNoteCommit(scoreclock.Clock{}, scoreclock.NoteCommitWork{Family: "dac", DACSample: true}, scoreclock.Limits{})
		if err != nil || !prefix.NeedsDACPublication {
			t.Fatal(err)
		}
		b, err := scoreclock.BeginMusicPublication(prefix.Clock, req, right, scoreclock.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if input, ok := b.ExpectedInput(); ok {
			if err = b.Apply(input); err != nil {
				t.Fatal(err)
			}
		}
		end, err := b.Finish()
		if err != nil || a.Clock != end || a.Feedback != b.Feedback() || !reflect.DeepEqual(left.trace, right.trace) || !reflect.DeepEqual(left.r.Snapshot(), right.r.Snapshot()) {
			t.Fatal("note prefix and music continuation diverged", err)
		}
	}
	owner, cfg := newMusicIO(t, 0, 0)
	req := scoreclock.MusicRequest{DescriptorID: musicDescriptor(t, cfg).ID, Pan: panValue(181)}
	if result, err := scoreclock.ServeMusicNotePublication(scoreclock.Clock{}, req, owner, scoreclock.Limits{MaxOperations: 26}); !errors.Is(err, scoreclock.ErrBudget) || result != (scoreclock.MusicResult{}) {
		t.Fatal("DAC prefix and publication reset their operation budget", result, err)
	}
}

func TestMusicPublicationActualOwnershipFrameAndMetadata(t *testing.T) {
	e, cfg := newMusicIO(t, 0, 0)
	d := musicDescriptor(t, cfg)
	input := pcmcommands.Input{Kind: pcmcommands.Music, DescriptorID: d.ID, Pan: 181}
	before := e.r.Snapshot()
	copy, ok := e.r.MusicDescriptor(d.ID)
	copy.Flags++
	again, found := e.r.MusicDescriptor(d.ID)
	if !ok || !found || again != d {
		t.Fatal("caller changed immutable descriptor")
	}
	if _, found := e.r.MusicDescriptor("unknown"); found {
		t.Fatal("unknown music descriptor exposed")
	}
	for _, descriptor := range cfg.PCM.Descriptors {
		if descriptor.Protocol == pcmcommands.OrdinaryProtocol {
			if _, found := e.r.MusicDescriptor(descriptor.ID); found {
				t.Fatal("ordinary resource exposed as music descriptor")
			}
			break
		}
	}
	for _, bad := range []struct {
		at    uint64
		input pcmcommands.Input
	}{
		{at: pcmcommands.FrameCycles + 1, input: input},
		{at: 1, input: input},
		{input: pcmcommands.Input{Kind: pcmcommands.Music, Begin: 1, DescriptorID: d.ID}},
		{input: pcmcommands.Input{Kind: pcmcommands.Hold}},
	} {
		if err := e.r.PublishAcceptedAt(bad.at, bad.input); err == nil || !reflect.DeepEqual(before, e.r.Snapshot()) {
			t.Fatal("invalid timed publication committed state", bad, err)
		}
	}
	if _, err := e.r.MusicFlags(pcmcommands.FrameCycles + 1); err == nil {
		t.Fatal("flags read outside frame")
	}
	if _, err := e.r.RequestBus(0, false); err != nil {
		t.Fatal(err)
	}
	unheld := e.r.Snapshot()
	if _, err := e.r.MusicFlags(0); err == nil {
		t.Fatal("unheld flags read")
	}
	if err := e.r.PublishAcceptedAt(0, input); err == nil || !reflect.DeepEqual(unheld, e.r.Snapshot()) {
		t.Fatal("unheld publication committed", err)
	}
	device, err := pcmcommands.Restore(cfg.PCM, before.PCM)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := device.MusicFlags(0); err != nil || got != 0 {
		t.Fatal(got, err)
	}
	if got, ok := device.MusicDescriptor(d.ID); !ok || got != d {
		t.Fatal("direct immutable descriptor differs")
	}
	if err = device.PublishAcceptedAt(0, input); err != nil || device.Snapshot().Global.SingleDescriptor != d.ID || device.Snapshot().Global.MusicPan != 181 {
		t.Fatal("direct canonical publication failed", err)
	}
	for _, scenario := range []string{"scheduled", "metadata-overlap"} {
		state := before
		hold := *before.PCM.Held
		state.PCM.Held = &hold
		if scenario == "scheduled" {
			state.PCM.Held.Open = false
		} else {
			state.PCM.Phase = "s-init"
			state.PCM.Part = 0
		}
		r, err := mixed.Restore(cfg, state)
		if err != nil {
			t.Fatal("invalid failure setup", scenario, err)
		}
		snapshot := r.Snapshot()
		if err = r.PublishAcceptedAt(0, input); err == nil || !reflect.DeepEqual(snapshot, r.Snapshot()) {
			t.Fatal("publication bypassed existing boundary guard", scenario, err)
		}
	}
}

func TestMusicPublicationValidationAndPoisonedBranches(t *testing.T) {
	e, cfg := newMusicIO(t, 0, 0)
	req := scoreclock.MusicRequest{DescriptorID: musicDescriptor(t, cfg).ID, Pan: panValue(181)}
	before := e.r.Snapshot()
	for _, bad := range []scoreclock.MusicRequest{{}, {DescriptorID: "unknown"}, {DescriptorID: req.DescriptorID, Pan: score.Pan{AMS: 4}}, {DescriptorID: req.DescriptorID, Pan: score.Pan{FMS: 8}}} {
		if tx, err := scoreclock.BeginMusicNotePublication(scoreclock.Clock{}, bad, e, scoreclock.Limits{}); err == nil || tx != nil || len(e.trace) != 0 {
			t.Fatal("invalid request touched bus")
		}
	}
	if _, err := scoreclock.BeginMusicNotePublication(scoreclock.Clock{}, req, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("missing bus accepted")
	}
	if _, err := scoreclock.BeginMusicNotePublication(scoreclock.Clock{Cycle: math.MaxUint64}, req, e, scoreclock.Limits{}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := scoreclock.BeginMusicNotePublication(scoreclock.Clock{}, req, e, scoreclock.Limits{MaxOperations: 65537}); err == nil {
		t.Fatal("oversized limit accepted")
	}
	for _, scenario := range []string{"early-finish", "wrong-input", "duplicate", "flags-error", "publication-error", "grant-budget"} {
		t.Run(scenario, func(t *testing.T) {
			r, err := mixed.Restore(cfg, before)
			if err != nil {
				t.Fatal(err)
			}
			x := &musicIO{panIO: &panIO{instrumentIO: &instrumentIO{r: r}}}
			x.failFlags, x.failPublication, x.denyGrant = scenario == "flags-error", scenario == "publication-error", scenario == "grant-budget"
			tx, err := scoreclock.BeginMusicNotePublication(scoreclock.Clock{}, req, x, scoreclock.Limits{MaxOperations: 64})
			if scenario == "flags-error" || scenario == "grant-budget" {
				if err == nil || tx != nil {
					t.Fatal("failed feedback/acquisition accepted")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				input, ok := tx.ExpectedInput()
				if !ok {
					t.Fatal("accepted intent absent")
				}
				switch scenario {
				case "early-finish":
					if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrMusicSequence) {
						t.Fatal("premature Finish accepted")
					}
				case "wrong-input":
					input.Pan++
				case "duplicate":
					if err = tx.Apply(input); err != nil {
						t.Fatal(err)
					}
				}
				if err = tx.Apply(input); err == nil {
					t.Fatal("wrong/repeated/failed effect accepted")
				}
				if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrMusicSequence) {
					t.Fatal("poisoned publication continued")
				}
			}
			if !reflect.DeepEqual(before, e.r.Snapshot()) {
				t.Fatal("detached failure changed original owner")
			}
		})
	}
}
