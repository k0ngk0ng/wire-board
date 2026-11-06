package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrMusicSequence = errors.New("invalid music publication sequence")

type MusicBus interface {
	Bus
	MusicDescriptor(id string) (pcmcommands.Descriptor, bool)
	PanRequest(at uint64) (uint8, error)
	MusicFlags(at uint64) (uint8, error)
	PublishAcceptedAt(at uint64, input pcmcommands.Input) error
}

type MusicRequest struct {
	DescriptorID string
	Pan          score.Pan
}

type MusicResult struct {
	Clock     Clock
	Feedback  score.DACFeedback
	Published bool
}

// MusicPublication is one temporary transaction on detached owners. It reads
// live feedback before the score's accepted intent, then publishes at that
// intent's phase. All metadata and payload state remain PCM-owned.
type MusicPublication struct {
	s        *service
	bus      MusicBus
	input    pcmcommands.Input
	feedback score.DACFeedback
	applied  bool
	done     bool
	failure  error
}

func beginMusic(clock Clock, request MusicRequest, bus MusicBus, limits Limits, commit bool) (*MusicPublication, error) {
	if bus == nil || request.DescriptorID == "" || request.Pan.AMS > 3 || request.Pan.FMS > 7 {
		return nil, ErrMusicSequence
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return nil, err
	}
	return beginMusicOnService(s, request, bus, commit)
}

func beginMusicOnService(s *service, request MusicRequest, bus MusicBus, commit bool) (*MusicPublication, error) {
	var err error
	descriptor, ok := bus.MusicDescriptor(request.DescriptorID)
	if !ok || descriptor.ID != request.DescriptorID || descriptor.PayloadID == "" || descriptor.Protocol != pcmcommands.MusicProtocol || descriptor.NativeRate == 0 || descriptor.NativeRate >= 128 || descriptor.RoutingOrPan != 0 {
		return nil, ErrMusicSequence
	}
	s.bus = bus
	t := &MusicPublication{s: s, bus: bus}
	if commit {
		if err = s.noteCommit(NoteCommitWork{Family: "dac", DACSample: true}); err != nil {
			return nil, err
		}
	}
	if err = s.spend(14); err != nil {
		return nil, err
	} // save interrupt mask
	if err = s.spend(20); err != nil {
		return nil, err
	}
	if err = s.panAcquire(); err != nil {
		return nil, err
	} // same BUSACK-only acquisition, no busy mailbox
	var marker uint8
	if err = s.cost(16, func() error {
		if err := s.add(7); err != nil {
			return err
		}
		var err error
		marker, err = bus.PanRequest(s.clock.Cycle)
		return err
	}); err != nil {
		return nil, err
	}
	t.feedback.Blocked = marker&128 != 0
	if err = s.branch(t.feedback.Blocked, false); err != nil {
		return nil, err
	}
	if t.feedback.Blocked {
		return t, nil
	}
	var flags uint8
	if err = s.cost(16, func() error {
		if err := s.add(7); err != nil {
			return err
		}
		var err error
		flags, err = bus.MusicFlags(s.clock.Cycle)
		return err
	}); err != nil {
		return nil, err
	}
	t.feedback.Priority = flags & 192
	if err = s.spend(8); err != nil {
		return nil, err
	}
	if err = s.spend(12); err != nil {
		return nil, err
	} // selected immutable descriptor flags
	if err = s.spend(8); err != nil {
		return nil, err
	}
	if err = s.spend(4); err != nil {
		return nil, err
	}
	rejected := descriptor.Flags&192 < t.feedback.Priority
	if err = s.branch(rejected, true); err != nil {
		return nil, err
	}
	if !rejected {
		pan := request.Pan.AMS<<4 | request.Pan.FMS
		if request.Pan.Left {
			pan |= 128
		}
		if request.Pan.Right {
			pan |= 64
		}
		t.input = pcmcommands.Input{Kind: pcmcommands.Music, DescriptorID: descriptor.ID, Pan: pan}
	}
	return t, nil
}

// BeginMusicPublication continues after a previously completed DAC commit
// prefix. Begin stops before any accepted metadata effect; blocked requests
// never read priority. The caller discards all detached owners on failure.
func BeginMusicPublication(clock Clock, request MusicRequest, bus MusicBus, limits Limits) (*MusicPublication, error) {
	return beginMusic(clock, request, bus, limits, false)
}

// BeginMusicNotePublication starts with the actual DAC note commit and shares
// one budget/clock with arbitration, publication and return. Both entry points
// use the same canonical bodies, never a second timing or publication model.
func BeginMusicNotePublication(clock Clock, request MusicRequest, bus MusicBus, limits Limits) (*MusicPublication, error) {
	return beginMusic(clock, request, bus, limits, true)
}

func (t *MusicPublication) Clock() Clock                { return t.s.clock }
func (t *MusicPublication) Feedback() score.DACFeedback { return t.feedback }
func (t *MusicPublication) ExpectedInput() (pcmcommands.Input, bool) {
	return t.input, t.input.Kind == pcmcommands.Music
}
func (t *MusicPublication) fail(err error) error { t.failure = err; return err }

func (t *MusicPublication) Apply(expected pcmcommands.Input) error {
	if t == nil {
		return ErrMusicSequence
	}
	if t.done || t.failure != nil || t.applied || t.input.Kind != pcmcommands.Music || expected != t.input {
		return t.fail(ErrMusicSequence)
	}
	input := expected
	// Both CPUs are excluded from observing intermediate metadata: the player
	// is held and main interrupts remain masked. Price marker+six fields at
	// their original boundaries, then publish the canonical group at pan.
	for field := 0; field < 7; field++ {
		if err := t.s.cost(20, func() error {
			if err := t.s.add(7); err != nil {
				return err
			}
			if field == 0 {
				input.Begin = t.s.clock.Cycle
			}
			return nil
		}); err != nil {
			return t.fail(err)
		}
	}
	if err := t.s.cost(24, func() error {
		if err := t.s.add(7); err != nil {
			return err
		}
		input.End = t.s.clock.Cycle
		return t.bus.PublishAcceptedAt(t.s.clock.Cycle, input)
	}); err != nil {
		return t.fail(err)
	}
	t.applied = true
	return nil
}

func (t *MusicPublication) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrMusicSequence
	}
	if t.done || t.failure != nil || t.input.Kind == pcmcommands.Music && !t.applied {
		return Clock{}, t.fail(ErrMusicSequence)
	}
	if err := t.s.request(20, false); err != nil {
		return Clock{}, t.fail(err)
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	} // restore interrupt mask
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}

func ServeMusicNotePublication(clock Clock, request MusicRequest, bus MusicBus, limits Limits) (MusicResult, error) {
	t, err := BeginMusicNotePublication(clock, request, bus, limits)
	if err != nil {
		return MusicResult{}, err
	}
	input, accepted := t.ExpectedInput()
	if accepted {
		if err = t.Apply(input); err != nil {
			return MusicResult{}, err
		}
	}
	end, err := t.Finish()
	if err != nil {
		return MusicResult{}, err
	}
	return MusicResult{Clock: end, Feedback: t.Feedback(), Published: accepted}, nil
}
