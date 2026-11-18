package engine

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// These two previously verified campaign builds predate compiled provenance.
// Unknown legacy binaries remain usable alone, but cannot join a catalog.
//
//go:embed legacy-runtimes.json
var legacyRuntimeJSON []byte

type buildStamp struct {
	Format       int    `json:"format"`
	Profile      string `json:"profile"`
	Purpose      string `json:"purpose"`
	SourceCommit string `json:"sourceCommit"`
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func validateBuildStamp(s buildStamp) error {
	profile := s.Profile == "original-98"
	if strings.HasPrefix(s.Profile, "development-health:") {
		text := strings.TrimPrefix(s.Profile, "development-health:")
		hp, err := strconv.Atoi(text)
		profile = err == nil && hp >= 1000 && hp <= 9999 && strconv.Itoa(hp) == text
	}
	if s.Format != 1 || !profile || buildPurpose(s.Purpose) == 0 || !commitPattern.MatchString(s.SourceCommit) {
		return errors.New("invalid native PAL build provenance")
	}
	return nil
}

func buildPurpose(name string) uint32 {
	switch name {
	case "campaign":
		return 1
	case "font-reference":
		return 2
	case "rng-fixture":
		return 3
	case "battle-fixture":
		return 4
	case "ending-fixture":
		return 5
	case "game-over-fixture":
		return 6
	}
	return 0
}

func readULEB(data []byte, pos *int) (uint32, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		if *pos >= len(data) {
			return 0, io.ErrUnexpectedEOF
		}
		b := data[*pos]
		*pos++
		if i == 4 && b > 15 {
			return 0, errors.New("invalid native Wasm section length")
		}
		value |= uint32(b&127) << uint(i*7)
		if b&128 == 0 {
			return value, nil
		}
	}
	return 0, errors.New("invalid native Wasm section length")
}

func readBuildStamp(program []byte, id string) (*buildStamp, bool, error) {
	if len(program) < 8 || !bytes.Equal(program[:8], []byte{'\x00', 'a', 's', 'm', 1, 0, 0, 0}) {
		return nil, false, errors.New("invalid native Wasm header")
	}
	var found *buildStamp
	for pos := 8; pos < len(program); {
		kind := program[pos]
		pos++
		size, err := readULEB(program, &pos)
		if err != nil {
			return nil, false, err
		}
		if uint64(pos)+uint64(size) > uint64(len(program)) {
			return nil, false, io.ErrUnexpectedEOF
		}
		end := pos + int(size)
		if kind == 0 {
			section := program[:end]
			nameSize, err := readULEB(section, &pos)
			if err != nil {
				return nil, false, err
			}
			if uint64(pos)+uint64(nameSize) > uint64(end) {
				return nil, false, io.ErrUnexpectedEOF
			}
			name := string(program[pos : pos+int(nameSize)])
			pos += int(nameSize)
			if name == "wire-board.pal.build" {
				if found != nil || end-pos > 4096 {
					return nil, false, errors.New("duplicate or oversized native PAL build stamp")
				}
				var s buildStamp
				decoder := json.NewDecoder(bytes.NewReader(program[pos:end]))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&s); err != nil {
					return nil, false, err
				}
				if decoder.Decode(new(any)) != io.EOF {
					return nil, false, errors.New("excess native PAL build stamp data")
				}
				if err := validateBuildStamp(s); err != nil {
					return nil, false, err
				}
				found = &s
			}
		}
		pos = end
	}
	if found != nil {
		return found, true, nil
	}
	var legacy map[string]buildStamp
	if err := json.Unmarshal(legacyRuntimeJSON, &legacy); err != nil {
		return nil, false, err
	}
	if s, ok := legacy[id]; ok {
		if err := validateBuildStamp(s); err != nil {
			return nil, false, err
		}
		return &s, false, nil
	}
	return nil, false, nil
}

func (r *Runtime) verifyCompiledProvenance(ctx context.Context) error {
	if r.program.ExportedFunctions()["wb_build_info"] == nil {
		return errors.New("native PAL stamped core lacks compiled provenance ABI")
	}
	module, err := r.instance(ctx)
	if err != nil {
		return err
	}
	defer module.Close(context.Background())
	address, err := call(ctx, module, "wb_build_info")
	if err != nil {
		return err
	}
	info, ok := module.Memory().Read(uint32(address), 16)
	if !ok {
		return errors.New("native PAL compiled provenance exceeds memory")
	}
	hp := 0
	if strings.HasPrefix(r.provenance.Profile, "development-health:") {
		hp, _ = strconv.Atoi(strings.TrimPrefix(r.provenance.Profile, "development-health:"))
	}
	if binary.LittleEndian.Uint32(info) != 1 || binary.LittleEndian.Uint32(info[4:]) != buildPurpose(r.provenance.Purpose) || binary.LittleEndian.Uint32(info[8:]) != uint32(hp) || binary.LittleEndian.Uint32(info[12:]) != 0 {
		return errors.New("native PAL embedded profile disagrees with compiled provenance")
	}
	return nil
}
