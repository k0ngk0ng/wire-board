package stream

// This verifier is independent of the production writer. It accepts the small
// RFC9639/BMFF subset promised by this package; ffmpeg and real MSE provide the
// separate general-decoder acceptance.
import (
	"encoding/binary"
	"errors"
	"fmt"
)

type referenceBox struct {
	kind      string
	raw, body []byte
}

func referenceBoxes(raw []byte) ([]referenceBox, error) {
	var out []referenceBox
	for len(raw) > 0 {
		if len(raw) < 8 {
			return nil, errors.New("short box header")
		}
		n := uint64(binary.BigEndian.Uint32(raw))
		if n < 8 || n > uint64(len(raw)) {
			return nil, errors.New("invalid box size")
		}
		out = append(out, referenceBox{string(raw[4:8]), raw[:n], raw[8:n]})
		raw = raw[n:]
	}
	return out, nil
}

func referenceFind(raw []byte, kind string) (referenceBox, error) {
	boxes, err := referenceBoxes(raw)
	if err != nil {
		return referenceBox{}, err
	}
	var found []referenceBox
	for _, box := range boxes {
		if box.kind == kind {
			found = append(found, box)
		}
	}
	if len(found) != 1 {
		return referenceBox{}, fmt.Errorf("wanted exactly one %s", kind)
	}
	return found[0], nil
}

func referenceCRC8(raw []byte) uint8 {
	var value uint8
	for _, b := range raw {
		value ^= b
		for bit := 0; bit < 8; bit++ {
			if value&0x80 != 0 {
				value = value<<1 ^ 0x07
			} else {
				value <<= 1
			}
		}
	}
	return value
}

func referenceCRC16(raw []byte) uint16 {
	var value uint16
	for _, b := range raw {
		value ^= uint16(b) << 8
		for bit := 0; bit < 8; bit++ {
			if value&0x8000 != 0 {
				value = value<<1 ^ 0x8005
			} else {
				value <<= 1
			}
		}
	}
	return value
}

func referenceNumber(raw []byte) (uint64, int, error) {
	if len(raw) == 0 {
		return 0, 0, errors.New("missing coded number")
	}
	if raw[0] < 128 {
		return uint64(raw[0]), 1, nil
	}
	count := 0
	for mask := byte(0x80); raw[0]&mask != 0 && mask != 0; mask >>= 1 {
		count++
	}
	if count < 2 || count > 7 || len(raw) < count {
		return 0, 0, errors.New("bad extended UTF8 number")
	}
	value := uint64(raw[0] & byte(0x7f>>count))
	for i := 1; i < count; i++ {
		if raw[i]&0xc0 != 0x80 {
			return 0, 0, errors.New("bad UTF8 continuation")
		}
		value = value<<6 | uint64(raw[i]&63)
	}
	minimum := []uint64{0, 0, 128, 2048, 65536, 1 << 21, 1 << 26, 1 << 31}[count]
	if value < minimum || value >= 1<<36 {
		return 0, 0, errors.New("overlong or oversized coded number")
	}
	return value, count, nil
}

func referenceFrame(raw []byte) ([]int16, uint64, error) {
	if len(raw) < 16 || raw[0] != 255 || raw[1] != 0xf9 || raw[2] != 0x70 || raw[3] != 0x18 {
		return nil, 0, errors.New("unexpected FLAC frame header")
	}
	first, width, err := referenceNumber(raw[4:])
	if err != nil {
		return nil, 0, err
	}
	at := 4 + width
	if at+3 > len(raw) {
		return nil, 0, errors.New("missing FLAC block size")
	}
	frames := int(binary.BigEndian.Uint16(raw[at:])) + 1
	at += 2
	if frames > 65535 || referenceCRC8(raw[:at]) != raw[at] {
		return nil, 0, errors.New("FLAC header CRC/size mismatch")
	}
	at++
	if len(raw) != at+2+frames*4+2 || referenceCRC16(raw[:len(raw)-2]) != binary.BigEndian.Uint16(raw[len(raw)-2:]) {
		return nil, 0, errors.New("FLAC frame CRC/length mismatch")
	}
	pcm := make([]int16, frames*2)
	for channel := 0; channel < 2; channel++ {
		if raw[at] != 2 {
			return nil, 0, errors.New("not independent16bit verbatim")
		}
		at++
		for i := 0; i < frames; i++ {
			pcm[i*2+channel] = int16(binary.BigEndian.Uint16(raw[at:]))
			at += 2
		}
	}
	return pcm, first, nil
}

func referenceInit(raw []byte, block uint32) error {
	ftyp, err := referenceFind(raw, "ftyp")
	if err != nil {
		return err
	}
	if len(ftyp.body) < 8 || string(ftyp.body[:4]) != "iso6" {
		return errors.New("unexpected movie brand")
	}
	moov, err := referenceFind(raw, "moov")
	if err != nil {
		return err
	}
	mvex, err := referenceFind(moov.body, "mvex")
	if err != nil {
		return err
	}
	if _, err = referenceFind(mvex.body, "trex"); err != nil {
		return err
	}
	trak, err := referenceFind(moov.body, "trak")
	if err != nil {
		return err
	}
	mdia, err := referenceFind(trak.body, "mdia")
	if err != nil {
		return err
	}
	mdhd, err := referenceFind(mdia.body, "mdhd")
	if err != nil {
		return err
	}
	if len(mdhd.body) < 20 || mdhd.body[0] != 0 || binary.BigEndian.Uint32(mdhd.body[12:]) != 44100 {
		return errors.New("wrong media timebase")
	}
	minf, err := referenceFind(mdia.body, "minf")
	if err != nil {
		return err
	}
	stbl, err := referenceFind(minf.body, "stbl")
	if err != nil {
		return err
	}
	for _, name := range []string{"stts", "stsc", "stco"} {
		b, e := referenceFind(stbl.body, name)
		if e != nil || len(b.body) != 8 || binary.BigEndian.Uint32(b.body[4:]) != 0 {
			return errors.New("nonempty initial sample table")
		}
	}
	stsd, err := referenceFind(stbl.body, "stsd")
	if err != nil {
		return err
	}
	if len(stsd.body) < 8 || binary.BigEndian.Uint32(stsd.body[4:]) != 1 {
		return errors.New("wrong sample entry count")
	}
	flac, err := referenceFind(stsd.body[8:], "fLaC")
	if err != nil {
		return err
	}
	if len(flac.body) < 28 || binary.BigEndian.Uint16(flac.body[16:]) != 2 || binary.BigEndian.Uint16(flac.body[18:]) != 16 || binary.BigEndian.Uint32(flac.body[24:]) != 44100<<16 {
		return errors.New("wrong FLAC sample entry")
	}
	dfla, err := referenceFind(flac.body[28:], "dfLa")
	if err != nil {
		return err
	}
	if len(dfla.body) != 42 || binary.BigEndian.Uint32(dfla.body[:4]) != 0 || string(dfla.body[4:8]) != string([]byte{128, 0, 0, 34}) {
		return errors.New("wrong FLAC metadata block")
	}
	info := dfla.body[8:]
	maximum := block
	if maximum < 17 {
		maximum = 17
	}
	if binary.BigEndian.Uint16(info) != 16 || uint32(binary.BigEndian.Uint16(info[2:])) != maximum {
		return errors.New("wrong variable block bounds")
	}
	for _, b := range info[4:10] {
		if b != 0 {
			return errors.New("frame bytes must be unknown")
		}
	}
	packed := binary.BigEndian.Uint64(info[10:18])
	if packed>>44 != 44100 || packed>>41&7 != 1 || packed>>36&31 != 15 || packed&((1<<36)-1) != 0 {
		return errors.New("wrong STREAMINFO format/length")
	}
	for _, b := range info[18:] {
		if b != 0 {
			return errors.New("stream MD5 must be unknown")
		}
	}
	return nil
}

func referenceSegment(segment Segment) ([]int16, error) {
	moof, err := referenceFind(segment.Data, "moof")
	if err != nil {
		return nil, err
	}
	mdat, err := referenceFind(segment.Data, "mdat")
	if err != nil {
		return nil, err
	}
	mfhd, err := referenceFind(moof.body, "mfhd")
	if err != nil {
		return nil, err
	}
	if len(mfhd.body) != 8 || binary.BigEndian.Uint32(mfhd.body[4:]) != segment.Sequence {
		return nil, errors.New("wrong sequence")
	}
	traf, err := referenceFind(moof.body, "traf")
	if err != nil {
		return nil, err
	}
	tfhd, err := referenceFind(traf.body, "tfhd")
	if err != nil {
		return nil, err
	}
	if len(tfhd.body) != 8 || binary.BigEndian.Uint32(tfhd.body) != 0x020000 || binary.BigEndian.Uint32(tfhd.body[4:]) != 1 {
		return nil, errors.New("wrong fragment-relative track header")
	}
	tfdt, err := referenceFind(traf.body, "tfdt")
	if err != nil {
		return nil, err
	}
	if len(tfdt.body) != 12 || tfdt.body[0] != 1 || binary.BigEndian.Uint64(tfdt.body[4:]) != segment.StartSample {
		return nil, errors.New("wrong64bit decode time")
	}
	trun, err := referenceFind(traf.body, "trun")
	if err != nil {
		return nil, err
	}
	if len(trun.body) < 12 || binary.BigEndian.Uint32(trun.body) != 0x301 {
		return nil, errors.New("wrong explicit run fields")
	}
	count := uint64(binary.BigEndian.Uint32(trun.body[4:]))
	if count == 0 || uint64(len(trun.body)) != 12+count*8 || int32(binary.BigEndian.Uint32(trun.body[8:])) != int32(len(moof.raw)+8) {
		return nil, errors.New("bad packet count/data offset")
	}
	var output []int16
	position := 0
	for i := uint64(0); i < count; i++ {
		at := 12 + i*8
		n, size := binary.BigEndian.Uint32(trun.body[at:]), binary.BigEndian.Uint32(trun.body[at+4:])
		if uint64(position)+uint64(size) > uint64(len(mdat.body)) {
			return nil, errors.New("packet outside mdat")
		}
		pcm, first, e := referenceFrame(mdat.body[position : position+int(size)])
		if e != nil {
			return nil, e
		}
		if n != uint32(len(pcm)/2) || first != segment.StartSample+uint64(len(output)/2) {
			return nil, errors.New("packet clock is not contiguous")
		}
		output = append(output, pcm...)
		position += int(size)
	}
	if position != len(mdat.body) || uint32(len(output)/2) != segment.SampleFrames {
		return nil, errors.New("extra/dropped media data")
	}
	return output, nil
}
