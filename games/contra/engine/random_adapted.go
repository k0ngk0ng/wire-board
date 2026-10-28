package contra

import "errors"

const (
	FrameRandomVersion uint8  = 1
	splitMixIncrement  uint64 = 0x9e3779b97f4a7c15
)

// AdaptedRandom is the explicitly permitted online-platform sequence adapter.
// SplitMix64 entropy injection and its once-per-authoritative-frame schedule
// are NOT the FC algorithm. Original ADC arithmetic and enemy decisions remain
// in RandomState and the game rules. No recorded oracle output is used here.
//
// The server creates Seed once when creating a match. Restore, reconnect and
// manual takeover preserve this entire state. Decimal strings keep uint64 values
// exact through JavaScript JSON transport and Go/Wasm checkpoint round trips.
type AdaptedRandom struct {
	Version   uint8       `json:"version"`
	Seed      uint64      `json:"seed,string"`
	Entropy   uint64      `json:"entropy,string"`
	Frames    uint64      `json:"frames,string"`
	Work      RandomState `json:"work"`
	IdleCarry bool        `json:"idleCarry"`
}

func NewAdaptedRandom(serverSeed uint64) AdaptedRandom {
	return AdaptedRandom{Version: FrameRandomVersion, Seed: serverSeed, Entropy: serverSeed}
}

func (s AdaptedRandom) Validate() error {
	if s.Version != FrameRandomVersion {
		return errors.New("unsupported Contra frame random adapter version")
	}
	if s.Entropy != s.Seed+s.Frames*splitMixIncrement {
		return errors.New("inconsistent Contra frame random adapter snapshot")
	}
	return nil
}

// AdvanceFrame runs exactly once before the rule phases of the next
// authoritative simulation frame. Input packets, rendered frames, network
// retries, server catch-up batching and snapshot reads must not invoke it.
// Repeated or skipped frame numbers fail without consuming entropy.
//
// XOR is an adaptation-only entropy injection: unlike a fixed idle-loop count,
// it does not trap the original ADC accumulator at counter 0/255 or mod-255
// cycles. The following ADC uses the caller's original byte counter. Neither
// this schedule nor its distribution is claimed to reproduce original hardware.
func (s *AdaptedRandom) AdvanceFrame(frame uint64, counter uint8) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Frames == ^uint64(0) || frame != s.Frames+1 {
		return errors.New("Contra frame random requires the next authoritative frame")
	}
	next := *s
	next.Entropy += splitMixIncrement
	// Standard SplitMix64 output permutation, with unsigned modulo-2^64 math.
	word := next.Entropy
	word = (word ^ (word >> 30)) * 0xbf58476d1ce4e5b9
	word = (word ^ (word >> 27)) * 0x94d049bb133111eb
	word ^= word >> 31
	next.Work.Value ^= uint8(word >> 56)
	next.IdleCarry = next.Work.Accumulate(counter, next.IdleCarry)
	next.Frames = frame
	*s = next
	return nil
}
