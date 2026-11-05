// Package pcmplayer advances the game's initialized PCM player in typed byte
// stages. It implements no CPU, firmware, ROM map, arbitrary program counter or
// RAM image. External command transactions and bus suspension are separate.
package pcmplayer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcm"
)

type Mode uint8

const (
	Single Mode = 1
	Dual   Mode = 2
)

type Kind string

const (
	DAC    Kind = "dac"
	Bank   Kind = "bank"
	Dummy  Kind = "dummy"
	Pan    Kind = "pan"
	Enable Kind = "enable"
)

// Sample contains immutable, content-identified DPCM bytes. Rate is the native
// single-player delay count; zero means 256 iterations, not no delay.
type Sample struct {
	ID      string
	Encoded []byte
	Rate    uint8
}

// Config selects the explicitly initialized reference profile. Dual begins
// after descriptor transfer with two empty prefetch buffers. Single begins
// after the original 128-byte preload. Origin is in reference master cycles;
// this package does not determine the original environment's start time.
type Config struct {
	Mode       Mode
	Samples    [2]*Sample
	Origin     uint64
	RestorePan uint8
}

// Event.Cycle is the original handler boundary; StartCycle identifies the
// semantic memory operation's beginning for reference frame admission. A Bank
// or Dummy read has already contributed the fixed reference 45-cycle wait to
// the player's subsequent timeline. No 68k/VDP suspension is included.
type Event struct {
	Cycle, StartCycle uint64
	Kind              Kind
	Value             uint8
	Slot              uint8
	Offset            uint32
}

type DualVoice struct {
	Consumed, Fetched uint32
	Accumulator       uint8
	Enabled, Active   bool
}
type DualState struct {
	PrefetchSlot uint8
	Voices       [2]DualVoice
}
type SingleState struct {
	Consumed, Fetched uint32
	Accumulator       uint8
	DummyCursor       uint32
}

// State has one presentation-independent owner. The two variants are exclusive.
// Source buffers are immutable resources; produced low-byte pointers are derived
// from Fetched and length, so no ring/RAM copy or shadow cursor is retained.
type State struct {
	Identity string
	Time     uint64
	Single   *SingleState
	Dual     *DualState
}

type Player struct {
	config Config
	state  State
	bound  uint64
	err    error
}

func New(config Config) (*Player, error) {
	if config.Mode != Single && config.Mode != Dual {
		return nil, errors.New("unknown PCM player profile")
	}
	if config.RestorePan&^uint8(0xc0) != 0 {
		return nil, errors.New("unsupported restored pan")
	}
	p := &Player{config: config}
	count := 0
	maxLen := uint64(0)
	digest := sha256.New()
	digest.Write([]byte{byte(config.Mode), config.RestorePan})
	var origin [8]byte
	binary.BigEndian.PutUint64(origin[:], config.Origin)
	digest.Write(origin[:])
	for i, s := range config.Samples {
		if s == nil {
			digest.Write([]byte{0})
			continue
		}
		count++
		if config.Mode == Single && i != 0 {
			return nil, errors.New("single player accepts only slot zero")
		}
		minimum, maximum := 69, 32768
		if config.Mode == Single {
			minimum, maximum = 129, 7788
		}
		if len(s.Encoded) < minimum || len(s.Encoded) > maximum {
			return nil, errors.New("sample outside initialized profile length")
		}
		h := sha256.Sum256(s.Encoded)
		id := hex.EncodeToString(h[:])
		if s.ID != id {
			return nil, errors.New("sample content ID differs")
		}
		copied := *s
		copied.Encoded = append([]byte(nil), s.Encoded...)
		p.config.Samples[i] = &copied
		digest.Write([]byte{1, s.Rate})
		digest.Write(h[:])
		if uint64(len(s.Encoded)) > maxLen {
			maxLen = uint64(len(s.Encoded))
		}
	}
	if count == 0 || config.Mode == Single && (count != 1 || config.Samples[0] == nil) {
		return nil, errors.New("missing PCM resource")
	}
	// Explicit upper bound for the supported byte/rate profile, including startup
	// slack and both DAC delays. It is not an observed-timestamp fit.
	p.bound = (maxLen+256)*120000 + 100000
	if config.Origin > math.MaxUint64-p.bound {
		return nil, errors.New("PCM clock range overflows")
	}
	p.state = State{Identity: hex.EncodeToString(digest.Sum(nil)), Time: config.Origin}
	if config.Mode == Single {
		p.state.Single = &SingleState{Fetched: 128, Accumulator: 128}
	} else {
		p.state.Dual = &DualState{PrefetchSlot: 1}
		for i := range p.state.Dual.Voices {
			p.state.Dual.Voices[i] = DualVoice{Accumulator: 128, Active: p.config.Samples[i] != nil}
		}
	}
	return p, nil
}
func clone(s State) State {
	if s.Single != nil {
		v := *s.Single
		s.Single = &v
	}
	if s.Dual != nil {
		v := *s.Dual
		s.Dual = &v
	}
	return s
}
func (p *Player) Snapshot() State { return clone(p.state) }
func Restore(config Config, state State) (*Player, error) {
	p, err := New(config)
	if err != nil {
		return nil, err
	}
	if state.Identity != p.state.Identity {
		return nil, errors.New("PCM state resource identity differs")
	}
	if err = p.validate(state); err != nil {
		return nil, err
	}
	p.state = clone(state)
	return p, nil
}
func (p *Player) validate(s State) error {
	if s.Time < p.config.Origin || s.Time-p.config.Origin > p.bound {
		return errors.New("PCM state clock outside profile")
	}
	if p.config.Mode == Single {
		if s.Single == nil || s.Dual != nil {
			return errors.New("PCM state variant differs")
		}
		v := s.Single
		n := uint32(len(p.config.Samples[0].Encoded))
		maximum := (n + 1) &^ uint32(1)
		if v.Fetched < 128 || v.Fetched > maximum || v.Fetched%2 != 0 || v.Consumed > n || v.Consumed > v.Fetched || v.DummyCursor > 10*v.Consumed+1 {
			return errors.New("invalid single PCM cursor")
		}
		return nil
	}
	if s.Dual == nil || s.Single != nil || s.Dual.PrefetchSlot > 1 {
		return errors.New("PCM state variant or phase differs")
	}
	for i, v := range s.Dual.Voices {
		sample := p.config.Samples[i]
		if sample == nil {
			if v != (DualVoice{Accumulator: 128}) {
				return errors.New("inactive PCM resource has state")
			}
			continue
		}
		n := uint32(len(sample.Encoded))
		maximum := (n + 3) &^ uint32(3)
		produced := v.Fetched
		if produced > n {
			produced = n
		}
		if v.Fetched > maximum || v.Fetched%4 != 0 || v.Consumed > produced || v.Enabled && (!v.Active || v.Consumed >= n) || !v.Active && (v.Consumed != n || v.Enabled) {
			return errors.New("invalid dual PCM cursor")
		}
	}
	return nil
}
func (p *Player) Stopped() bool {
	if p.state.Single != nil {
		return p.state.Single.Consumed == uint32(len(p.config.Samples[0].Encoded))
	}
	return !p.state.Dual.Voices[0].Active && !p.state.Dual.Voices[1].Active
}
func (p *Player) add(t uint64) {
	delta := t * 15
	if p.state.Time > math.MaxUint64-delta {
		p.err = errors.New("PCM clock overflow")
		return
	}
	p.state.Time += delta
}
func (p *Player) emit(out *[]Event, kind Kind, value, slot uint8, offset uint32, lead uint64) {
	if p.state.Time < lead*15 {
		p.err = errors.New("PCM operation clock underflow")
		return
	}
	*out = append(*out, Event{Cycle: p.state.Time, StartCycle: p.state.Time - lead*15, Kind: kind, Value: value, Slot: slot, Offset: offset})
}
func (p *Player) stop(out *[]Event) {
	p.add(106)
	p.emit(out, Pan, p.config.RestorePan, 0, 0, 7)
	p.add(77)
	p.emit(out, Enable, 0, 0, 0, 7)
	p.add(49)
}

// Next commits one bounded semantic byte stage, including its two DAC halves
// or terminal high-half/restore sequence. The returned events are ordered and
// may span a reference frame; caller admission uses StartCycle. This API does
// not permit command insertion inside that byte stage.
func (p *Player) Next() ([]Event, error) {
	if p.Stopped() {
		return nil, errors.New("PCM player is stopped")
	}
	n := *p
	n.state = clone(p.state)
	n.err = nil
	out := make([]Event, 0, 6)
	if n.config.Mode == Single {
		n.single(&out)
	} else {
		n.dual(&out)
	}
	if n.err != nil {
		return nil, n.err
	}
	if len(out) > 6 {
		return nil, errors.New("PCM step event bound exceeded")
	}
	if n.state.Time < n.config.Origin || n.state.Time-n.config.Origin > n.bound {
		return nil, errors.New("PCM duration bound exceeded")
	}
	p.state = n.state
	return out, nil
}
func (p *Player) dual(out *[]Event) {
	d := p.state.Dual
	phase := d.PrefetchSlot
	var values [2][2]uint8
	p.add(45)
	for i := range d.Voices {
		v := &d.Voices[i]
		enabled := v.Enabled
		values[i] = [2]uint8{128, 128}
		if enabled {
			sample := p.config.Samples[i]
			if sample == nil || v.Consumed >= uint32(len(sample.Encoded)) {
				p.err = errors.New("dual decode cursor outside resource")
				return
			}
			next, pair := pcm.DecodeByte(pcm.DecoderState{Accumulator: v.Accumulator}, sample.Encoded[v.Consumed])
			v.Accumulator = next.Accumulator
			values[i] = pair
			v.Consumed++
			produced := v.Fetched
			if produced > uint32(len(sample.Encoded)) {
				produced = uint32(len(sample.Encoded))
			}
			if uint8(i) != phase && uint8(v.Consumed) == uint8(produced) {
				v.Enabled = false
			}
		}
		var cost uint64
		if phase == 0 {
			cost = [2]uint64{137, 168}[i]
		} else {
			cost = [2]uint64{171, 134}[i]
		}
		if !enabled {
			cost += 5
		}
		p.add(cost)
	}
	p.add(52)
	p.emit(out, DAC, pcm.MixDual(values[0][0], values[1][0]), 0, 0, 7)
	p.add(41)
	current := &d.Voices[phase]
	other := &d.Voices[1-phase]
	sample := p.config.Samples[phase]
	length := uint32(0)
	if sample != nil {
		length = uint32(len(sample.Encoded))
	}
	if current.Fetched < length {
		p.add(28)
		produced := current.Fetched
		gap := uint8(current.Consumed - produced)
		if gap == 0 || gap >= 192 {
			p.add(44 + 143)
			for i := 0; i < 4; i++ {
				p.add(16)
				p.emit(out, Bank, 0, phase, current.Fetched, 16)
				p.add(3)
				current.Fetched++
			}
			p.add(10)
			if current.Fetched > length {
				p.add(56)
			} else {
				p.add(57)
			}
		} else {
			current.Enabled = true
			p.add(17 + 27 + 60 + 214)
		}
	} else {
		p.add(109)
		if !current.Active {
			p.add(71 + 27 + 139)
		} else {
			p.add(28)
			produced := current.Fetched
			if produced > length {
				produced = length
			}
			if uint8(current.Consumed) != uint8(produced) {
				p.add(57 + 139)
			} else {
				current.Active = false
				current.Enabled = false
				p.add(43 + 27)
				if !other.Active {
					p.stop(out)
					return
				}
				p.add(139)
			}
		}
	}
	p.add(52)
	p.emit(out, DAC, pcm.MixDual(values[0][1], values[1][1]), 0, 0, 7)
	p.add(38)
	d.PrefetchSlot = 1 - phase
}
func (p *Player) single(out *[]Event) {
	s := p.state.Single
	sample := p.config.Samples[0]
	length := uint32(len(sample.Encoded))
	if s.Consumed >= length {
		p.err = errors.New("single decode cursor outside resource")
		return
	}
	tail := s.Fetched >= length
	next, pair := pcm.DecodeByte(pcm.DecoderState{Accumulator: s.Accumulator}, sample.Encoded[s.Consumed])
	s.Accumulator = next.Accumulator
	s.Consumed++
	rate := uint64(sample.Rate)
	if rate == 0 {
		rate = 256
	}
	delay := 13*rate - 5
	p.add(110)
	p.emit(out, DAC, pair[0], 0, 0, 13)
	if tail {
		p.add(183 + delay)
		s.DummyCursor += 9
		p.emit(out, DAC, pair[1], 0, 0, 13)
		p.add(51 + delay)
		s.DummyCursor++
		if s.Consumed == length {
			p.stop(out)
		} else {
			p.add(27)
		}
		return
	}
	gap := uint8(s.Consumed - s.Fetched)
	if gap == 0 || gap&128 != 0 {
		p.add(79 + delay + 10)
		for i := 0; i < 2; i++ {
			p.add(16)
			p.emit(out, Bank, 0, 0, s.Fetched, 16)
			p.add(3)
			s.Fetched++
		}
		p.add(11 + 51)
	} else {
		p.add(74 + delay)
		p.emit(out, Dummy, 0, 0, s.DummyCursor, 19)
		p.add(3 + 40 + 8 + 10 + 51)
		s.DummyCursor += 4
	}
	p.emit(out, DAC, pair[1], 0, 0, 13)
	p.add(78 + delay)
	if s.Fetched > length {
		s.DummyCursor++
	}
}

// String is a concise diagnostic; source/ROM addresses are not player state.
func (p *Player) String() string {
	return fmt.Sprintf("PCM profile=%d time=%d stopped=%t", p.config.Mode, p.state.Time, p.Stopped())
}
