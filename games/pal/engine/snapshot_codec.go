package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const snapshotEnvelopeMagic = "PALMEM01"
const maxSnapshotMetadata = 8 * 1024 * 1024
const snapshotZstdMagic = "\x28\xb5\x2f\xfd"

var snapshotEncoders sync.Pool
var snapshotDecoders sync.Pool

func snapshotEncoder(output io.Writer) (*zstd.Encoder, error) {
	if cached := snapshotEncoders.Get(); cached != nil {
		writer := cached.(*zstd.Encoder)
		writer.Reset(output)
		return writer, nil
	}
	return zstd.NewWriter(output, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithEncoderConcurrency(2), zstd.WithWindowSize(1024*1024), zstd.WithEncoderCRC(true))
}

func snapshotDecoder(input io.Reader) (*zstd.Decoder, error) {
	if cached := snapshotDecoders.Get(); cached != nil {
		reader := cached.(*zstd.Decoder)
		if err := reader.Reset(input); err != nil {
			reader.Close()
			return nil, err
		}
		return reader, nil
	}
	return zstd.NewReader(input, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(maxMemory+maxSnapshotMetadata+16))
}

type snapshotBuffer struct{ bytes.Buffer }

func (b *snapshotBuffer) Write(data []byte) (int, error) {
	if len(data) > maxEncoded-b.Len() {
		return 0, errors.New("native PAL save exceeds compressed limit")
	}
	return b.Buffer.Write(data)
}

func (r *Runtime) Encode(s Snapshot) ([]byte, error) {
	if err := r.validateMetadata(s); err != nil {
		return nil, err
	}
	verified := make(chan bool, 1)
	go func() {
		verified <- snapshotIdentity(s.Memory, s.StackPointer) == s.MemoryID
	}()
	encoded, err := encodeValidated(s)
	if !<-verified {
		return nil, errors.New("corrupt native PAL snapshot memory")
	}
	return encoded, err
}

func encodeValidated(s Snapshot) ([]byte, error) {
	metadata := s
	metadata.Memory = nil
	raw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxSnapshotMetadata {
		return nil, errors.New("native PAL snapshot metadata exceeds limit")
	}
	var header [16]byte
	copy(header[:], snapshotEnvelopeMagic)
	binary.LittleEndian.PutUint32(header[8:], uint32(len(raw)))
	binary.LittleEndian.PutUint32(header[12:], uint32(len(s.Memory)))
	var compressed snapshotBuffer
	compressed.Grow(min(len(s.Memory)/8, 1024*1024))
	writer, err := snapshotEncoder(&compressed)
	if err != nil {
		return nil, err
	}
	defer func() { writer.Reset(io.Discard); snapshotEncoders.Put(writer) }()
	for _, data := range [][]byte{header[:], raw, s.Memory} {
		if _, err = writer.Write(data); err != nil {
			writer.Close()
			return nil, err
		}
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func decodeSnapshotMetadata(raw []byte) (Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var s Snapshot
	if err := decoder.Decode(&s); err != nil {
		return Snapshot{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Snapshot{}, errors.New("native PAL save contains trailing JSON")
	}
	return s, nil
}

func (r *Runtime) Decode(encoded []byte) (Snapshot, error) {
	s, err := decodeSnapshot(encoded)
	if err != nil {
		return Snapshot{}, err
	}
	if err = r.validateMetadata(s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func decodeSnapshot(encoded []byte) (Snapshot, error) {
	if len(encoded) > maxEncoded {
		return Snapshot{}, errors.New("native PAL compressed save exceeds limit")
	}
	input := bytes.NewReader(encoded)
	var reader io.Reader
	if len(encoded) >= 4 && string(encoded[:4]) == snapshotZstdMagic {
		decoder, err := snapshotDecoder(input)
		if err != nil {
			return Snapshot{}, err
		}
		defer func() {
			if decoder.Reset(nil) != nil {
				decoder.Close()
			} else {
				snapshotDecoders.Put(decoder)
			}
		}()
		reader = decoder
	} else {
		decoder, err := gzip.NewReader(input)
		if err != nil {
			return Snapshot{}, err
		}
		defer decoder.Close()
		decoder.Multistream(false)
		reader = decoder
	}
	var err error
	var prefix [8]byte
	n, prefixErr := io.ReadFull(reader, prefix[:])
	if prefixErr != nil && prefixErr != io.EOF && prefixErr != io.ErrUnexpectedEOF {
		return Snapshot{}, prefixErr
	}
	var s Snapshot
	verified := false
	if n == len(prefix) && string(prefix[:]) == snapshotEnvelopeMagic {
		var lengths [8]byte
		if _, err = io.ReadFull(reader, lengths[:]); err != nil {
			return Snapshot{}, err
		}
		metadataSize := binary.LittleEndian.Uint32(lengths[:4])
		memorySize := binary.LittleEndian.Uint32(lengths[4:])
		if metadataSize == 0 || metadataSize > maxSnapshotMetadata || memorySize < 65536 || memorySize > maxMemory || memorySize%65536 != 0 {
			return Snapshot{}, errors.New("invalid native PAL binary snapshot lengths")
		}
		raw := make([]byte, metadataSize)
		if _, err = io.ReadFull(reader, raw); err != nil {
			return Snapshot{}, err
		}
		if s, err = decodeSnapshotMetadata(raw); err != nil {
			return Snapshot{}, err
		}
		if len(s.Memory) != 0 {
			return Snapshot{}, errors.New("native PAL binary metadata contains duplicate memory")
		}
		s.Memory = make([]byte, memorySize)
		if err = readSnapshotMemory(reader, &s); err != nil {
			return Snapshot{}, err
		}
		verified = true
		var extra [1]byte
		if n, err = reader.Read(extra[:]); n != 0 || err != io.EOF {
			if err != nil {
				return Snapshot{}, err
			}
			return Snapshot{}, errors.New("native PAL save contains excess data")
		}
	} else {
		raw, err := io.ReadAll(io.LimitReader(io.MultiReader(bytes.NewReader(prefix[:n]), reader), maxJSON+1))
		if err != nil {
			return Snapshot{}, err
		}
		if len(raw) > maxJSON {
			return Snapshot{}, errors.New("native PAL save contains excess data")
		}
		if s, err = decodeSnapshotMetadata(raw); err != nil {
			return Snapshot{}, err
		}
	}
	if input.Len() != 0 {
		return Snapshot{}, errors.New("native PAL save contains excess data")
	}
	if !verified && snapshotIdentity(s.Memory, s.StackPointer) != s.MemoryID {
		return Snapshot{}, errors.New("corrupt native PAL snapshot memory")
	}
	return s, nil
}

func readSnapshotMemory(reader io.Reader, state *Snapshot) error {
	blocks := make(chan []byte, 2)
	finished := make(chan string, 1)
	go func() {
		hash := sha256.New()
		for block := range blocks {
			hash.Write(block)
		}
		var pointer [4]byte
		binary.LittleEndian.PutUint32(pointer[:], state.StackPointer)
		hash.Write(pointer[:])
		finished <- hex.EncodeToString(hash.Sum(nil))
	}()
	var failure error
	for offset := 0; offset < len(state.Memory); {
		end := min(len(state.Memory), offset+256*1024)
		count, err := io.ReadFull(reader, state.Memory[offset:end])
		if count > 0 {
			blocks <- state.Memory[offset : offset+count]
		}
		offset += count
		if err != nil {
			failure = err
			break
		}
	}
	close(blocks)
	digest := <-finished
	if failure != nil {
		return failure
	}
	if digest != state.MemoryID {
		return errors.New("corrupt native PAL snapshot memory")
	}
	return nil
}
