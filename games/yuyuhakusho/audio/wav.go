package audio

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

func mediaSHA(raw []byte) string { value := sha256.Sum256(raw); return hex.EncodeToString(value[:]) }

// ValidateWAV authenticates the complete original WAV and PCM against a catalog
// whose own digest the caller has authenticated. It preserves all leading and
// trailing samples. Supported files are RIFF/WAVE with one16-byte PCM fmt chunk
// and one aligned stereo data chunk; ancillary chunks require valid RIFF bounds.
func (c Catalog) ValidateWAV(code uint8, raw []byte) error {
	clip, ok := c.Lookup(code)
	if !ok {
		return errors.New("unknown or excluded finite audio code")
	}
	if mediaSHA(raw) != clip.WAVSHA256 {
		return errors.New("finite WAV file SHA differs")
	}
	if len(raw) < 12 || !bytes.Equal(raw[:4], []byte("RIFF")) || !bytes.Equal(raw[8:12], []byte("WAVE")) || uint64(binary.LittleEndian.Uint32(raw[4:8]))+8 != uint64(len(raw)) {
		return errors.New("invalid finite RIFF/WAVE header or length")
	}
	var pcm []byte
	foundFormat, foundData := false, false
	for position := uint64(12); position < uint64(len(raw)); {
		if uint64(len(raw))-position < 8 {
			return errors.New("truncated finite WAV chunk header")
		}
		header := raw[int(position):int(position+8)]
		size := uint64(binary.LittleEndian.Uint32(header[4:]))
		start := position + 8
		end := start + size
		if end > uint64(len(raw)) || end+(size&1) > uint64(len(raw)) {
			return errors.New("finite WAV chunk outside RIFF bounds")
		}
		chunk := raw[int(start):int(end)]
		switch string(header[:4]) {
		case "fmt ":
			if foundFormat || len(chunk) != 16 {
				return errors.New("missing, duplicate or unsupported finite WAV format")
			}
			foundFormat = true
			if binary.LittleEndian.Uint16(chunk) != 1 || binary.LittleEndian.Uint16(chunk[2:]) != 2 || binary.LittleEndian.Uint32(chunk[4:]) != 44100 || binary.LittleEndian.Uint32(chunk[8:]) != 176400 || binary.LittleEndian.Uint16(chunk[12:]) != 4 || binary.LittleEndian.Uint16(chunk[14:]) != 16 {
				return errors.New("finite WAV is not44100 stereo signed16 little-endian PCM")
			}
		case "data":
			if foundData || size == 0 || size%4 != 0 {
				return errors.New("invalid or duplicate finite WAV data")
			}
			foundData = true
			pcm = chunk
		}
		position = end + (size & 1)
	}
	if !foundFormat || !foundData || uint64(len(pcm))/4 != clip.StereoFrames {
		return errors.New("finite WAV format/data or stereo-frame count differs")
	}
	if mediaSHA(pcm) != clip.PCMSHA256 {
		return errors.New("finite PCM SHA differs")
	}
	for frame := uint64(0); frame < clip.StereoFrames; frame++ {
		i := int(frame * 4)
		if pcm[i] != 0 || pcm[i+1] != 0 || pcm[i+2] != 0 || pcm[i+3] != 0 {
			if frame != clip.FirstNonzeroStereoFrame {
				return errors.New("finite WAV onset differs; original leading delay must remain intact")
			}
			return nil
		}
	}
	return errors.New("finite WAV is unexpectedly silent")
}
