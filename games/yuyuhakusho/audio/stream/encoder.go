// Package stream encodes bounded PCM input as lossless FLAC/fMP4 segments.
// It contains no synthesizer, media player, ROM data or external tool runner.
package stream

import "errors"

const MIMEType = "audio/mp4; codecs=\"fLaC\""

const (
	maxInputFrames = 1 << 20
	maxSampleEnd   = uint64(1) << 36
	maxSequence    = uint64(1)<<32 - 1
)

var (
	ErrFormat         = errors.New("unsupported PCM format: need44100Hz stereo signed16")
	ErrConfig         = errors.New("invalid FLAC stream block/fragment configuration")
	ErrPCMAlignment   = errors.New("PCM input is not complete stereo sample frames")
	ErrCapacity       = errors.New("PCM append exceeds the configured input capacity")
	ErrClosed         = errors.New("PCM stream encoder is closed")
	ErrSampleNumber   = errors.New("FLAC sample number exceeds the36bit range")
	ErrSequenceNumber = errors.New("MP4 fragment sequence exceeds uint32")
)

type Format struct {
	SampleRate    uint32
	Channels      uint8
	BitsPerSample uint8
}

type Config struct {
	Format         Format
	BlockFrames    uint32
	FragmentFrames uint32
	MaxInputFrames uint32
}

type Segment struct {
	Sequence     uint32
	StartSample  uint64
	SampleFrames uint32
	Data         []byte
}

// Status is a copied interpretation, not another persisted set of counters.
// A sample here is one interchannel frame containing both left and right.
type Status struct {
	EmittedSamples  uint64
	BufferedFrames  uint32
	AcceptedSamples uint64
	NextSequence    uint64
	Closed          bool
}

// Encoder is owned by one serialized caller. Only pending PCM, its output
// sample cursor, the next fragment sequence and closed are mutable owners.
// FLAC block numbers and input totals are not separately retained.
type Encoder struct {
	config       Config
	pending      []int16
	sampleCursor uint64
	nextSequence uint64
	closed       bool
}

func DefaultConfig() Config {
	return Config{Format: Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}, BlockFrames: 1024, FragmentFrames: 4096, MaxInputFrames: 65536}
}

func New(config Config) (*Encoder, error) {
	if config.Format != (Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}) {
		return nil, ErrFormat
	}
	if config.BlockFrames < 16 || config.BlockFrames > 65535 || config.FragmentFrames == 0 || config.FragmentFrames%config.BlockFrames != 0 || config.MaxInputFrames == 0 || config.MaxInputFrames > maxInputFrames || config.FragmentFrames > config.MaxInputFrames {
		return nil, ErrConfig
	}
	return &Encoder{config: config, nextSequence: 1}, nil
}

func (e *Encoder) Status() Status {
	buffered := uint32(len(e.pending) / 2)
	return Status{EmittedSamples: e.sampleCursor, BufferedFrames: buffered, AcceptedSamples: e.sampleCursor + uint64(buffered), NextSequence: e.nextSequence, Closed: e.closed}
}

// InitSegment returns fresh caller-owned bytes. The streaming metadata does
// not backpatch duration or MD5 when more input or Finish arrives.
func (e *Encoder) InitSegment() []byte { return makeInitSegment(e.config) }

func (e *Encoder) preflight(frames uint64) error {
	buffered := uint64(len(e.pending) / 2)
	if e.sampleCursor > maxSampleEnd || buffered > maxSampleEnd-e.sampleCursor || frames > maxSampleEnd-e.sampleCursor-buffered {
		return ErrSampleNumber
	}
	total := buffered + frames
	// Include a possible future short tail: data accepted now must remain
	// encodable by Finish even when this append emits no complete fragment.
	needed := (total + uint64(e.config.FragmentFrames) - 1) / uint64(e.config.FragmentFrames)
	if needed > 0 && (e.nextSequence == 0 || e.nextSequence > maxSequence || needed-1 > maxSequence-e.nextSequence) {
		return ErrSequenceNumber
	}
	return nil
}

func (e *Encoder) AppendPCM(pcm []int16) ([]Segment, error) {
	if e.closed {
		return nil, ErrClosed
	}
	if len(pcm)%2 != 0 {
		return nil, ErrPCMAlignment
	}
	frames := uint64(len(pcm) / 2)
	if frames > uint64(e.config.MaxInputFrames) {
		return nil, ErrCapacity
	}
	if frames == 0 {
		return nil, nil
	}
	if err := e.preflight(frames); err != nil {
		return nil, err
	}
	// This temporary input window is never persisted. Validate every possible
	// error before encoding, then commit the unique pending owner and cursors.
	window := make([]int16, len(e.pending)+len(pcm))
	copy(window, e.pending)
	copy(window[len(e.pending):], pcm)
	fragmentSamples := int(e.config.FragmentFrames) * 2
	count := len(window) / fragmentSamples
	out := make([]Segment, 0, count)
	cursor, sequence := e.sampleCursor, e.nextSequence
	for index := 0; index < count; index++ {
		part := window[index*fragmentSamples : (index+1)*fragmentSamples]
		out = append(out, encodeSegment(part, e.config, cursor, uint32(sequence)))
		cursor += uint64(e.config.FragmentFrames)
		sequence++
	}
	tail := append([]int16(nil), window[count*fragmentSamples:]...)
	e.pending, e.sampleCursor, e.nextSequence = tail, cursor, sequence
	return out, nil
}

func (e *Encoder) Finish() ([]Segment, error) {
	if e.closed {
		return nil, nil
	}
	if len(e.pending) == 0 {
		e.closed = true
		return nil, nil
	}
	if err := e.preflight(0); err != nil {
		return nil, err
	}
	frames := uint32(len(e.pending) / 2)
	out := encodeSegment(e.pending, e.config, e.sampleCursor, uint32(e.nextSequence))
	e.sampleCursor += uint64(frames)
	e.nextSequence++
	e.pending = nil
	e.closed = true
	return []Segment{out}, nil
}
