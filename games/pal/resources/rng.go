// SPDX-License-Identifier: GPL-3.0-only
// RNG delta commands follow SDLPAL rngplay.c.
// Copyright (c) 2009-2011 Wei Mingzhi and 2011-2026 SDLPAL development team.
// See LICENSE.SDLPAL and ../native/reference/rngplay.c.
package resources

import (
	"bytes"
	"errors"
)

// ApplyRNGFrame applies the original 320x200 indexed delta stream. Commands
// leave skipped pixels intact, including opaque palette index zero. Validate
// before writing so corrupt data never partially changes a caller's surface.
func ApplyRNGFrame(data, pixels []byte) error {
	if len(pixels) != 64000 || len(data) > 65000 {
		return errors.New("invalid RNG surface or frame size")
	}
	data = bytes.Clone(data)
	if err := applyRNG(data, nil); err != nil {
		return err
	}
	return applyRNG(data, pixels)
}

func applyRNG(data, pixels []byte) error {
	ptr, dst := 0, 0
	read := func(n int) ([]byte, error) {
		if n < 0 || n > len(data)-ptr {
			return nil, errors.New("truncated RNG command")
		}
		b := data[ptr : ptr+n]
		ptr += n
		return b, nil
	}
	for ptr < len(data) {
		op := data[ptr]
		ptr++
		if op == 0 || op == 0x13 {
			return nil
		}
		if op == 2 {
			dst += 2
			continue
		}
		literal, repeat, skip := op >= 6 && op <= 12, op >= 13 && op <= 18, op == 3 || op == 4
		if !literal && !repeat && !skip {
			continue
		} // Original switch ignores unused opcodes.
		count := int(op) - 5
		if repeat {
			count = int(op) - 11
		}
		if skip || op == 11 || op == 12 || op == 17 || op == 18 {
			n := 1
			if op == 4 || op == 12 || op == 18 {
				n = 2
			}
			b, err := read(n)
			if err != nil {
				return err
			}
			count = int(b[0]) + 1
			if n == 2 {
				count += int(b[1]) << 8
			}
		}
		if skip {
			dst += count * 2
			continue
		}
		if count < 1 || dst < 0 || count > 32000 || dst > 64000-count*2 {
			return errors.New("RNG write exceeds surface")
		}
		n := count * 2
		if repeat {
			n = 2
		}
		b, err := read(n)
		if err != nil {
			return err
		}
		if pixels != nil {
			if repeat {
				for i := 0; i < count; i++ {
					copy(pixels[dst+i*2:dst+i*2+2], b)
				}
			} else {
				copy(pixels[dst:dst+n], b)
			}
		}
		dst += count * 2
	}
	return nil
}

// DecodeRNGClip preserves every original frame slot. A nil slot is an empty
// record at which PAL_RNGReadFrame stops; it must not become a drawn frame.
func DecodeRNGClip(data []byte) ([][]byte, error) {
	a, err := OpenArchive(data)
	if err != nil {
		return nil, err
	}
	if a.Count() > 4096 {
		return nil, errors.New("RNG frame count exceeds limit")
	}
	frames := make([][]byte, a.Count())
	for i := range frames {
		chunk, err := a.Chunk(i)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			continue
		}
		frames[i], err = DecompressYJ2(chunk, 65000)
		if err != nil {
			return nil, err
		}
		if err := applyRNG(frames[i], nil); err != nil {
			return nil, err
		}
	}
	return frames, nil
}

func transformRNGArchive(a *Archive, bundle *RuleBundle) error {
	clips := make([][]byte, a.Count())
	marker, err := encodeRuleChunk([]byte{0})
	if err != nil {
		return err
	}
	for clip := range clips {
		chunk, err := a.Chunk(clip)
		if err != nil {
			return err
		}
		if len(chunk) == 0 {
			continue
		}
		frames, err := DecodeRNGClip(chunk)
		if err != nil {
			return err
		}
		for i, frame := range frames {
			if len(frame) > 0 {
				frames[i] = marker
			}
		}
		clips[clip], err = packArchive(frames)
		if err != nil {
			return err
		}
	}
	bundle.Files["rng.mkf"], err = packArchive(clips)
	return err
}
