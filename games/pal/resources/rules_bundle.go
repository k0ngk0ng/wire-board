package resources

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
)

type FrameMetric struct {
	Offset int
	Width  int
	Height int
}
type RuleBundle struct {
	Files        map[string][]byte
	SourceHashes map[string]string
}

// SpriteMetrics reads the original frame addresses and dimensions, including
// SDLPAL's documented wrapped offset for the Bloody-Mouth Bug sprite.
func SpriteMetrics(data []byte) ([]FrameMetric, error) {
	if len(data) < 2 {
		return nil, errors.New("truncated sprite frame table")
	}
	words := int(binary.LittleEndian.Uint16(data))
	if words < 1 || words*2 > len(data) {
		return nil, errors.New("invalid sprite frame table size")
	}
	frames := make([]FrameMetric, words-1)
	for i := range frames {
		offset := int(binary.LittleEndian.Uint16(data[i*2:])) * 2
		if offset == 0x18444 {
			offset &= 0xffff
		}
		if offset < words*2 || offset+4 > len(data) {
			return nil, fmt.Errorf("sprite frame %d exceeds data", i)
		}
		start := offset
		if binary.LittleEndian.Uint32(data[offset:]) == 2 {
			offset += 4
		}
		if offset+4 > len(data) {
			return nil, errors.New("truncated sprite dimensions")
		}
		w, h := int(binary.LittleEndian.Uint16(data[offset:])), int(binary.LittleEndian.Uint16(data[offset+2:]))
		if w > 4096 || h > 4096 {
			return nil, errors.New("sprite dimensions exceed limit")
		}
		frames[i] = FrameMetric{Offset: start, Width: w, Height: h}
	}
	return frames, nil
}

// SpriteAddressableMetrics also retains the valid final address accepted by
// PAL_SpriteGetFrame. Some original enemy atlases use that slot as a real
// animation frame although PAL_SpriteGetNumFrames excludes it. End sentinels
// and invalid trailing RLE remain outside the addressable visual frame list.
func SpriteAddressableMetrics(data []byte) ([]FrameMetric, error) {
	frames, err := SpriteMetrics(data)
	if err != nil {
		return nil, err
	}
	words := len(frames) + 1
	offset := int(binary.LittleEndian.Uint16(data[(words-1)*2:])) * 2
	if offset == 0x18444 {
		offset &= 0xffff
	}
	if offset < words*2 || offset+4 > len(data) {
		return frames, nil
	}
	bitmap, err := DecodeRLE(data[offset:])
	if err != nil || bitmap.Width == 0 || bitmap.Height == 0 {
		return frames, nil
	}
	return append(frames, FrameMetric{Offset: offset, Width: bitmap.Width, Height: bitmap.Height}), nil
}

func headlessRLE(w, h int) []byte {
	data := []byte{2, 0, 0, 0}
	data = binary.LittleEndian.AppendUint16(data, uint16(w))
	data = binary.LittleEndian.AppendUint16(data, uint16(h))
	for y := 0; y < h; y++ {
		for left := w; left > 0; {
			n := min(left, 127)
			data = append(data, byte(128+n))
			left -= n
		}
	}
	return data
}

func headlessSprite(frames []FrameMetric, nominal int) ([]byte, error) {
	if nominal < 0 || nominal >= 65535 || len(frames) < nominal || len(frames) > nominal+1 {
		return nil, errors.New("sprite frame count exceeds format")
	}
	data := make([]byte, (nominal+1)*2)
	for i, frame := range frames {
		if len(data)/2 > 65535 {
			return nil, errors.New("sprite metrics exceed address format")
		}
		binary.LittleEndian.PutUint16(data[i*2:], uint16(len(data)/2))
		data = append(data, headlessRLE(frame.Width, frame.Height)...)
		if len(data)%2 != 0 {
			data = append(data, 0)
		}
	}
	if len(data)/2 > 65535 {
		return nil, errors.New("sprite metric end exceeds format")
	}
	if len(frames) == nominal {
		binary.LittleEndian.PutUint16(data[nominal*2:], uint16(len(data)/2))
		// Safe empty storage for the reference's tolerated end-sentinel read.
		data = append(data, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	return data, nil
}

func packArchive(chunks [][]byte) ([]byte, error) {
	data := make([]byte, (len(chunks)+1)*4)
	for i, chunk := range chunks {
		if uint64(len(data))+uint64(len(chunk)) > 0xffffffff {
			return nil, errors.New("rule archive exceeds format")
		}
		binary.LittleEndian.PutUint32(data[i*4:], uint32(len(data)))
		data = append(data, chunk...)
	}
	binary.LittleEndian.PutUint32(data[len(chunks)*4:], uint32(len(data)))
	return data, nil
}

func encodeRuleChunk(data []byte) ([]byte, error) {
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	out := binary.LittleEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, 'W', 'B', 'Z', '1')
	out = binary.LittleEndian.AppendUint32(out, uint32(compressed.Len()))
	out = append(out, compressed.Bytes()...)
	return out, nil
}

// DecodeRuleChunk reads the bounded, zlib-compressed authority-only metrics.
// Original YJ2 data remains available to the separate visual resource pipeline.
func DecodeRuleChunk(data []byte, limit int) ([]byte, error) {
	if len(data) < 12 || !bytes.Equal(data[4:8], []byte("WBZ1")) {
		return nil, errors.New("invalid rule chunk header")
	}
	length, size := uint64(binary.LittleEndian.Uint32(data)), uint64(binary.LittleEndian.Uint32(data[8:]))
	if limit < 0 || length > uint64(limit) || size != uint64(len(data)-12) {
		return nil, errors.New("invalid rule chunk size")
	}
	input := bytes.NewReader(data[12:])
	reader, err := zlib.NewReader(input)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, int64(length)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(decoded)) != length || input.Len() != 0 {
		return nil, errors.New("rule chunk decoded size mismatch")
	}
	return decoded, nil
}

// BuildRuleBundle separates authority data from client visuals. Map collision
// and height bits, animation counts and dimensions are original. Pixel data is
// replaced by transparent metrics used only by the headless rule core.
func BuildRuleBundle(files fs.FS) (RuleBundle, error) {
	b := RuleBundle{Files: map[string][]byte{}, SourceHashes: map[string]string{}}
	archives := map[string]*Archive{}
	for _, name := range []string{"sss.mkf", "data.mkf", "map.mkf", "gop.mkf", "mgo.mkf", "abc.mkf", "f.mkf", "fire.mkf", "fbp.mkf", "ball.mkf", "rgm.mkf", "pat.mkf", "rng.mkf"} {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return b, err
		}
		sum := sha256.Sum256(data)
		b.SourceHashes[name] = hex.EncodeToString(sum[:])
		a, err := OpenArchive(data)
		if err != nil {
			return b, fmt.Errorf("%s: %w", name, err)
		}
		archives[name] = a
		if name == "sss.mkf" {
			b.Files[name] = bytes.Clone(data)
		}
	}
	// Original RNG playback reads nested frame offsets itself. Retain those
	// offsets and empty stop records; only rendering payloads are replaced by
	// a valid empty delta command. Client archives keep the original commands.
	if err := transformRNGArchive(archives["rng.mkf"], &b); err != nil {
		return b, err
	}
	transform := func(name string, fn func(int, []byte) ([]byte, error)) error {
		a := archives[name]
		chunks := make([][]byte, a.Count())
		for i := range chunks {
			input, _ := a.Chunk(i)
			if len(input) == 0 {
				continue
			}
			var err error
			chunks[i], err = fn(i, input)
			if err != nil {
				return fmt.Errorf("%s chunk %d: %w", name, i, err)
			}
		}
		data, err := packArchive(chunks)
		if err != nil {
			return err
		}
		b.Files[name] = data
		return nil
	}
	if err := transform("map.mkf", func(_ int, input []byte) ([]byte, error) {
		data, err := DecompressYJ2(input, 65536)
		if err != nil {
			return nil, err
		}
		if len(data) != 65536 {
			return nil, errors.New("original map size differs")
		}
		for i := 0; i < len(data); i += 4 {
			binary.LittleEndian.PutUint32(data[i:], binary.LittleEndian.Uint32(data[i:])&0x0f002f00)
		}
		return encodeRuleChunk(data)
	}); err != nil {
		return b, err
	}
	for _, name := range []string{"mgo.mkf", "abc.mkf", "f.mkf", "fire.mkf"} {
		if err := transform(name, func(_ int, input []byte) ([]byte, error) {
			data, err := DecompressYJ2(input, 8*1024*1024)
			if err != nil {
				return nil, err
			}
			frames, err := SpriteAddressableMetrics(data)
			if err != nil {
				return nil, err
			}
			metadata, err := headlessSprite(frames, int(binary.LittleEndian.Uint16(data))-1)
			if err != nil {
				return nil, err
			}
			return encodeRuleChunk(metadata)
		}); err != nil {
			return b, err
		}
	}
	if err := transform("gop.mkf", func(_ int, _ []byte) ([]byte, error) {
		return headlessSprite([]FrameMetric{{Width: 32, Height: 16}}, 1)
	}); err != nil {
		return b, err
	}
	if err := transform("fbp.mkf", func(_ int, input []byte) ([]byte, error) {
		data, err := DecompressYJ2(input, 64000)
		if err != nil {
			return nil, err
		}
		if len(data) != 64000 {
			return nil, errors.New("original background dimensions differ")
		}
		return encodeRuleChunk(make([]byte, len(data)))
	}); err != nil {
		return b, err
	}
	for _, name := range []string{"ball.mkf", "rgm.mkf"} {
		if err := transform(name, func(_ int, input []byte) ([]byte, error) {
			offset := 0
			if len(input) >= 4 && binary.LittleEndian.Uint32(input) == 2 {
				offset = 4
			}
			if len(input) < offset+4 {
				return nil, errors.New("truncated original bitmap")
			}
			w, h := int(binary.LittleEndian.Uint16(input[offset:])), int(binary.LittleEndian.Uint16(input[offset+2:]))
			if w > 4096 || h > 4096 {
				return nil, errors.New("bitmap metrics exceed limit")
			}
			return headlessRLE(w, h), nil
		}); err != nil {
			return b, err
		}
	}
	if err := transform("pat.mkf", func(_ int, input []byte) ([]byte, error) { return make([]byte, len(input)), nil }); err != nil {
		return b, err
	}
	if err := transform("data.mkf", func(i int, input []byte) ([]byte, error) {
		if i != 9 && i != 10 && i != 12 {
			return input, nil
		}
		frames, err := SpriteAddressableMetrics(input)
		if err != nil {
			return nil, err
		}
		data, err := headlessSprite(frames, int(binary.LittleEndian.Uint16(input))-1)
		if err != nil {
			return nil, err
		}
		if i == 12 {
			if len(data) > 282 {
				return nil, errors.New("dialog metrics exceed original buffer")
			}
			data = append(data, make([]byte, 282-len(data))...)
		}
		return data, nil
	}); err != nil {
		return b, err
	}
	return b, nil
}
