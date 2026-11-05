package stream

import "encoding/binary"

func u16(value uint16) []byte { return binary.BigEndian.AppendUint16(nil, value) }
func u32(value uint32) []byte { return binary.BigEndian.AppendUint32(nil, value) }
func u64(value uint64) []byte { return binary.BigEndian.AppendUint64(nil, value) }

func join(parts ...[]byte) []byte {
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	out := make([]byte, 0, size)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// Config bounds guarantee that every generated box is far below uint32 and
// that moof-relative data offsets remain positive signed32bit quantities.
func box(kind string, parts ...[]byte) []byte {
	body := join(parts...)
	out := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(out, uint32(8+len(body)))
	copy(out[4:8], kind)
	return append(out, body...)
}

func fullBox(kind string, version byte, flags uint32, parts ...[]byte) []byte {
	fields := []byte{version, byte(flags >> 16), byte(flags >> 8), byte(flags)}
	return box(kind, join(fields, join(parts...)))
}

func identityMatrix() []byte {
	var out []byte
	for _, v := range []uint32{0x10000, 0, 0, 0, 0x10000, 0, 0, 0, 0x40000000} {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	return out
}

func makeInitSegment(config Config) []byte {
	ftyp := box("ftyp", []byte("iso6"), u32(1), []byte("iso6isommp41"))
	mvhd := fullBox("mvhd", 0, 0, u32(0), u32(0), u32(config.Format.SampleRate), u32(0), u32(0x10000), u16(0x100), make([]byte, 10), identityMatrix(), make([]byte, 24), u32(2))
	tkhd := fullBox("tkhd", 0, 7, u32(0), u32(0), u32(1), u32(0), u32(0), make([]byte, 8), u16(0), u16(0), u16(0x100), u16(0), identityMatrix(), u32(0), u32(0))
	mdhd := fullBox("mdhd", 0, 0, u32(0), u32(0), u32(config.Format.SampleRate), u32(0), u16(0x55c4), u16(0))
	hdlr := fullBox("hdlr", 0, 0, u32(0), []byte("soun"), make([]byte, 12), []byte("SoundHandler\x00"))
	dref := fullBox("dref", 0, 0, u32(1), fullBox("url ", 0, 1))
	dinf := box("dinf", dref)
	dfla := fullBox("dfLa", 0, 0, []byte{0x80, 0, 0, 34}, streamInfo(config))
	entry := box("fLaC", make([]byte, 6), u16(1), make([]byte, 8), u16(uint16(config.Format.Channels)), u16(uint16(config.Format.BitsPerSample)), u16(0), u16(0), u32(config.Format.SampleRate<<16), dfla)
	stbl := box("stbl", fullBox("stsd", 0, 0, u32(1), entry), fullBox("stts", 0, 0, u32(0)), fullBox("stsc", 0, 0, u32(0)), fullBox("stsz", 0, 0, u32(0), u32(0)), fullBox("stco", 0, 0, u32(0)))
	minf := box("minf", fullBox("smhd", 0, 0, u16(0), u16(0)), dinf, stbl)
	trak := box("trak", tkhd, box("mdia", mdhd, hdlr, minf))
	trex := fullBox("trex", 0, 0, u32(1), u32(1), u32(0), u32(0), u32(0x02000000))
	return join(ftyp, box("moov", mvhd, trak, box("mvex", trex)))
}

type encodedPacket struct {
	frames uint32
	data   []byte
}

func encodeSegment(pcm []int16, config Config, first uint64, sequence uint32) Segment {
	frames := uint32(len(pcm) / 2)
	packets := make([]encodedPacket, 0, (frames+config.BlockFrames-1)/config.BlockFrames)
	for at := uint32(0); at < frames; {
		count := config.BlockFrames
		if count > frames-at {
			count = frames - at
		}
		data := encodeFLAC(pcm[int(at)*2:int(at+count)*2], first+uint64(at))
		packets = append(packets, encodedPacket{count, data})
		at += count
	}
	// Every trun includes its own duration and size. No redundant block or
	// packet counter persists beyond this serialization window.
	run := join(u32(uint32(len(packets))), u32(0))
	for _, packet := range packets {
		run = append(run, u32(packet.frames)...)
		run = append(run, u32(uint32(len(packet.data)))...)
	}
	mfhd := fullBox("mfhd", 0, 0, u32(sequence))
	tfhd := fullBox("tfhd", 0, 0x020000, u32(1))
	tfdt := fullBox("tfdt", 1, 0, u64(first))
	trun := fullBox("trun", 0, 0x301, run)
	moof := box("moof", mfhd, box("traf", tfhd, tfdt, trun))
	binary.BigEndian.PutUint32(run[4:8], uint32(len(moof)+8))
	// Rebuild the small run header after its relative offset is known; all
	// sample payload stays in its one temporary packet collection.
	trun = fullBox("trun", 0, 0x301, run)
	moof = box("moof", mfhd, box("traf", tfhd, tfdt, trun))
	var payload []byte
	for _, packet := range packets {
		payload = append(payload, packet.data...)
	}
	return Segment{Sequence: sequence, StartSample: first, SampleFrames: frames, Data: join(moof, box("mdat", payload))}
}
