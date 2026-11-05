package stream

import "encoding/binary"

// These tables are immutable after package initialization and never belong to
// an encoder's mutable playback state.
var flacHeaderCRC = func() (table [256]byte) {
	for index := range table {
		v := byte(index)
		for bit := 0; bit < 8; bit++ {
			if v&0x80 != 0 {
				v = v<<1 ^ 0x07
			} else {
				v <<= 1
			}
		}
		table[index] = v
	}
	return
}()

var flacFrameCRC = func() (table [256]uint16) {
	for index := range table {
		v := uint16(index) << 8
		for bit := 0; bit < 8; bit++ {
			if v&0x8000 != 0 {
				v = v<<1 ^ 0x8005
			} else {
				v <<= 1
			}
		}
		table[index] = v
	}
	return
}()

func checksumHeader(raw []byte) byte {
	var crc byte
	for _, value := range raw {
		crc = flacHeaderCRC[crc^value]
	}
	return crc
}

func checksumFrame(raw []byte) uint16 {
	var crc uint16
	for _, value := range raw {
		crc = crc<<8 ^ flacFrameCRC[byte(crc>>8)^value]
	}
	return crc
}

// FLAC's number coding extends UTF8 to36bits. Unicode rune helpers would
// reject or truncate the upper part of the permitted sample index range.
func sampleNumber(value uint64) []byte {
	if value < 128 {
		return []byte{byte(value)}
	}
	limits := [6]uint64{1 << 11, 1 << 16, 1 << 21, 1 << 26, 1 << 31, 1 << 36}
	length := 2
	for _, limit := range limits {
		if value < limit {
			break
		}
		length++
	}
	out := make([]byte, length)
	for index := length - 1; index > 0; index-- {
		out[index] = 0x80 | byte(value&63)
		value >>= 6
	}
	out[0] = byte(0xff)<<uint(8-length) | byte(value)
	return out
}

func encodeFLAC(pcm []int16, first uint64) []byte {
	frames := len(pcm) / 2
	header := []byte{0xff, 0xf9, 0x70, 0x18} // variable blocks, block hint16bit, STREAMINFO rate, independent stereo16.
	header = append(header, sampleNumber(first)...)
	header = binary.BigEndian.AppendUint16(header, uint16(frames-1))
	header = append(header, checksumHeader(header))
	out := make([]byte, 0, len(header)+2+frames*4+2)
	out = append(out, header...)
	for channel := 0; channel < 2; channel++ {
		out = append(out, 0x02) // verbatim, no wasted bits.
		for sample := channel; sample < len(pcm); sample += 2 {
			out = binary.BigEndian.AppendUint16(out, uint16(pcm[sample]))
		}
	}
	return binary.BigEndian.AppendUint16(out, checksumFrame(out))
}

func streamInfo(config Config) []byte {
	out := make([]byte, 34)
	maximum := config.BlockFrames
	if maximum < 17 {
		maximum = 17
	}
	binary.BigEndian.PutUint16(out, 16)
	binary.BigEndian.PutUint16(out[2:], uint16(maximum))
	// Frame byte bounds, total sample count and MD5 remain unknown. The
	// declared16..max range is valid for variable blocks; only the final
	// block may contain fewer than16 samples (RFC9639 section8.2).
	packed := uint64(config.Format.SampleRate)<<44 | uint64(config.Format.Channels-1)<<41 | uint64(config.Format.BitsPerSample-1)<<36
	binary.BigEndian.PutUint64(out[10:], packed)
	return out
}
