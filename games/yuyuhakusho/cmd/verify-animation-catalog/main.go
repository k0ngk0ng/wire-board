// Command verify-animation-catalog compares bounded immutable rule data with
// isolated original-program outputs. It does not emulate or execute the ROM.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type catalogCharacter struct {
	ID           int    `json:"character_id"`
	AnimationSet uint32 `json:"animation_set"`
	ShapeSet     uint32 `json:"shape_set"`
}

type catalogFrame struct {
	Key         engine.AnimationFrameKey   `json:"key"`
	Data        engine.AnimationRecordData `json:"data"`
	CharacterID int                        `json:"character_id"`
}

type semanticCatalog struct {
	Schema     int                `json:"schema"`
	ROMSHA     string             `json:"rom_sha256"`
	Characters []catalogCharacter `json:"characters"`
	Frames     []catalogFrame     `json:"frames"`
	RawSHA     string             `json:"-"`
}

type originalSnapshot struct {
	Actor        engine.AnimationActorState `json:"actor"`
	ActorBytes   string                     `json:"actor_bytes"`
	Buffers      string                     `json:"buffers"`
	Offsets      [4]uint16                  `json:"offsets"`
	PendingSound uint8                      `json:"pending_sound"`
	SoundQueue   [4]uint8                   `json:"sound_queue"`
}

type originalCall struct {
	Key         engine.AnimationFrameKey `json:"key"`
	CharacterID int                      `json:"character_id"`
	Variant     int                      `json:"variant"`
	Before      originalSnapshot         `json:"before"`
	After       originalSnapshot         `json:"after"`
	Exit        string                   `json:"exit"`
}

type originalCalls struct {
	Schema     int            `json:"schema"`
	CatalogSHA string         `json:"catalog_sha256"`
	Calls      []originalCall `json:"calls"`
	RawSHA     string         `json:"-"`
}

type verificationReport struct {
	Pass             bool   `json:"pass"`
	Characters       int    `json:"characters"`
	Frames           int    `json:"frames"`
	OriginalCPUCalls int    `json:"original_cpu_calls"`
	VerifiedFrames   int    `json:"verified_frames"`
	CatalogSHA       string `json:"catalog_sha256"`
	OracleSHA        string `json:"oracle_sha256"`
	Comparison       string `json:"comparison"`
	Scope            string `json:"scope"`
}

func digest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func readCompressed(path string, into any) (string, string, error) {
	zipped, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return "", "", err
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return "", "", err
	}
	return digest(raw), digest(zipped), nil
}

func loadInputs(catalogPath, oraclePath string) (semanticCatalog, originalCalls, error) {
	var catalog semanticCatalog
	var oracle originalCalls
	var catalogZipSHA, oracleZipSHA string
	var err error
	catalog.RawSHA, catalogZipSHA, err = readCompressed(catalogPath, &catalog)
	if err != nil {
		return catalog, oracle, err
	}
	oracle.RawSHA, oracleZipSHA, err = readCompressed(oraclePath, &oracle)
	if err != nil {
		return catalog, oracle, err
	}
	manifestBytes, err := os.ReadFile(filepath.Join(filepath.Dir(catalogPath), "animation-catalog-manifest.json"))
	if err != nil {
		return catalog, oracle, err
	}
	var manifest struct {
		ExtractionPass bool   `json:"extraction_pass"`
		CPUCoverage    bool   `json:"complete_cpu_coverage"`
		AllFrames      bool   `json:"all_catalog_frames_checked"`
		Characters     int    `json:"characters"`
		Frames         int    `json:"frames"`
		Calls          int    `json:"original_cpu_calls"`
		CatalogSHA     string `json:"catalog_sha256"`
		CatalogZipSHA  string `json:"catalog_gzip_sha256"`
		OracleSHA      string `json:"oracle_sha256"`
		OracleZipSHA   string `json:"oracle_gzip_sha256"`
	}
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		return catalog, oracle, err
	}
	if !manifest.ExtractionPass || !manifest.CPUCoverage || !manifest.AllFrames || manifest.Characters != len(catalog.Characters) || manifest.Frames != len(catalog.Frames) || manifest.Calls != len(oracle.Calls) || manifest.CatalogSHA != catalog.RawSHA || manifest.CatalogZipSHA != catalogZipSHA || manifest.OracleSHA != oracle.RawSHA || manifest.OracleZipSHA != oracleZipSHA {
		return catalog, oracle, errors.New("catalog/CPU inputs do not match the extraction manifest")
	}
	return catalog, oracle, nil
}

func buildLibrary(catalog semanticCatalog) (engine.AnimationLibrary, error) {
	if catalog.Schema != 1 || catalog.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || len(catalog.Characters) != 11 || len(catalog.Frames) == 0 {
		return nil, errors.New("unknown or empty catalog identity")
	}
	for i, character := range catalog.Characters {
		if character.ID != i || character.AnimationSet == 0 || character.ShapeSet == 0 {
			return nil, errors.New("invalid character binding")
		}
	}
	library := make(engine.AnimationLibrary, len(catalog.Frames))
	for _, frame := range catalog.Frames {
		if frame.CharacterID < 0 || frame.CharacterID >= len(catalog.Characters) || frame.Key.Pose > 190 {
			return nil, errors.New("invalid frame character/pose")
		}
		character := catalog.Characters[frame.CharacterID]
		if character.AnimationSet != frame.Key.AnimationSet || character.ShapeSet != frame.Key.ShapeSet {
			return nil, errors.New("frame belongs to a different character set")
		}
		if _, exists := library[frame.Key]; exists {
			return nil, errors.New("duplicate immutable animation key")
		}
		library[frame.Key] = frame.Data
	}
	return library, nil
}

func decodeSnapshot(s originalSnapshot) (engine.AnimationSubmissionState, []byte, []byte, error) {
	state := engine.AnimationSubmissionState{Actor: s.Actor, PendingSound: s.PendingSound, SoundQueue: s.SoundQueue}
	a, err := hex.DecodeString(s.ActorBytes)
	if err != nil || len(a) != 256 {
		return state, nil, nil, errors.New("invalid full actor bytes")
	}
	b, err := hex.DecodeString(s.Buffers)
	if err != nil || len(b) != 1024 {
		return state, nil, nil, errors.New("invalid full shape bytes")
	}
	w := func(offset int) uint16 { return binary.BigEndian.Uint16(a[offset:]) }
	l := func(offset int) uint32 { return binary.BigEndian.Uint32(a[offset:]) }
	actor := engine.AnimationActorState{
		ObjectID: s.Actor.ObjectID, AnimationSet: l(0x50), ShapeSet: l(0x8c), Pose: a[0x5f],
		Clock:  engine.AnimationClock{FrameIndex: a[0x59], Duration: a[0x5a], Ticks: a[0x5b], Flags: a[0x5c], Completed: a[0x60]},
		Facing: a[0x61], ActionFlags: a[0x6d], EnabledStages: a[0x6c], Attributes: w(0x24),
		GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)), WorldX: int32(l(0x40)), WorldY: int32(l(0x44)),
		GraphicParameter: w(0x3c), Auxiliary94: l(0x94), Auxiliary64: a[0x64], Value65: a[0x65], Value66: a[0x66], Value67: a[0x67], Value68: a[0x68],
	}
	if actor != s.Actor {
		return state, nil, nil, errors.New("semantic actor disagrees with original bytes")
	}
	for group := range state.Buffers {
		state.Buffers[group].Offset = s.Offsets[group]
		for index := range state.Buffers[group].Records {
			raw := b[group*256+index*16 : group*256+(index+1)*16]
			state.Buffers[group].Records[index] = engine.AnimationShapeRecord{X: int16(binary.BigEndian.Uint16(raw)), Y: int16(binary.BigEndian.Uint16(raw[4:])), Width: raw[8], Height: raw[9], Owner: binary.BigEndian.Uint16(raw[12:]), Preserved: [4]uint16{binary.BigEndian.Uint16(raw[2:]), binary.BigEndian.Uint16(raw[6:]), binary.BigEndian.Uint16(raw[10:]), binary.BigEndian.Uint16(raw[14:])}}
		}
	}
	return state, a, b, nil
}

func encodeSnapshot(state engine.AnimationSubmissionState, originalActor, originalBuffers []byte) ([]byte, []byte) {
	a, b := append([]byte(nil), originalActor...), append([]byte(nil), originalBuffers...)
	s := state.Actor
	a[0x5a], a[0x5c], a[0x6c] = s.Clock.Duration, s.Clock.Flags, s.EnabledStages
	binary.BigEndian.PutUint16(a[0x24:], s.Attributes)
	binary.BigEndian.PutUint16(a[0x28:], uint16(s.GraphicOffsetX))
	binary.BigEndian.PutUint16(a[0x2a:], uint16(s.GraphicOffsetY))
	binary.BigEndian.PutUint16(a[0x3c:], s.GraphicParameter)
	binary.BigEndian.PutUint32(a[0x94:], s.Auxiliary94)
	a[0x64], a[0x65], a[0x66], a[0x67], a[0x68] = s.Auxiliary64, s.Value65, s.Value66, s.Value67, s.Value68
	for group, buffer := range state.Buffers {
		for index, r := range buffer.Records {
			raw := b[group*256+index*16 : group*256+(index+1)*16]
			binary.BigEndian.PutUint16(raw, uint16(r.X))
			binary.BigEndian.PutUint16(raw[4:], uint16(r.Y))
			raw[8], raw[9] = r.Width, r.Height
			binary.BigEndian.PutUint16(raw[12:], r.Owner)
			for i, offset := range []int{2, 6, 10, 14} {
				binary.BigEndian.PutUint16(raw[offset:], r.Preserved[i])
			}
		}
	}
	return a, b
}

func verifyCall(library engine.AnimationLibrary, call originalCall) error {
	if call.Exit != "0x1edaa" && call.Exit != "0x21970" && call.Exit != "0x21982" && call.Exit != "0x21994" && call.Exit != "0x219a6" && call.Exit != "0x219aa" {
		return errors.New("unconfirmed original return")
	}
	input, actorBefore, buffersBefore, err := decodeSnapshot(call.Before)
	if err != nil {
		return err
	}
	want, actorAfter, buffersAfter, err := decodeSnapshot(call.After)
	if err != nil {
		return err
	}
	key := engine.AnimationFrameKey{AnimationSet: input.Actor.AnimationSet, ShapeSet: input.Actor.ShapeSet, Pose: input.Actor.Pose, Index: input.Actor.Clock.FrameIndex}
	if key != call.Key {
		return errors.New("CPU input does not select the declared immutable frame")
	}
	got, err := engine.SubmitAnimationRecord(input, library)
	if err != nil {
		return err
	}
	actor, buffers := encodeSnapshot(got, actorBefore, buffersBefore)
	if got != want || !bytes.Equal(actor, actorAfter) || !bytes.Equal(buffers, buffersAfter) {
		return errors.New("full actor/buffer/offset/pending-sound/queue comparison differs")
	}
	return nil
}

func verifyCatalog(catalog semanticCatalog, oracle originalCalls) (verificationReport, error) {
	report := verificationReport{Characters: len(catalog.Characters), Frames: len(catalog.Frames), OriginalCPUCalls: len(oracle.Calls), CatalogSHA: catalog.RawSHA, OracleSHA: oracle.RawSHA, Comparison: "Full 256-byte actor, 1,024-byte shape buffers, all four offsets, pending sound and four queue slots; no masks or expected-value injection", Scope: "Isolated original-CPU animation submission calls; not complete character gameplay, graphics or matches"}
	if oracle.Schema != 1 || catalog.RawSHA == "" || oracle.CatalogSHA != catalog.RawSHA || len(oracle.Calls) != 2*len(catalog.Frames) {
		return report, errors.New("missing or unbound complete CPU coverage")
	}
	library, err := buildLibrary(catalog)
	if err != nil {
		return report, err
	}
	before := make(engine.AnimationLibrary, len(library))
	for key, data := range library {
		for i, group := range data.Groups {
			data.Groups[i] = append([]engine.AnimationShapeDefinition{}, group...)
		}
		before[key] = data
	}
	coverage := make(map[engine.AnimationFrameKey]uint8, len(library))
	for i, call := range oracle.Calls {
		if call.Variant < 0 || call.Variant > 1 || call.CharacterID < 0 || call.CharacterID >= len(catalog.Characters) {
			return report, fmt.Errorf("call %d has invalid scenario or character", i)
		}
		character := catalog.Characters[call.CharacterID]
		if character.AnimationSet != call.Key.AnimationSet || character.ShapeSet != call.Key.ShapeSet {
			return report, fmt.Errorf("call %d has wrong character binding", i)
		}
		bit := uint8(1 << call.Variant)
		if coverage[call.Key]&bit != 0 {
			return report, fmt.Errorf("call %d duplicates frame scenario", i)
		}
		if err := verifyCall(library, call); err != nil {
			return report, fmt.Errorf("call %d character %d pose %d index %d variant %d: %w", i, call.CharacterID, call.Key.Pose, call.Key.Index, call.Variant, err)
		}
		coverage[call.Key] |= bit
	}
	for key := range library {
		if coverage[key] != 3 {
			return report, errors.New("frame missing a CPU scenario")
		}
		report.VerifiedFrames++
	}
	if !reflect.DeepEqual(library, before) {
		return report, errors.New("animation submission mutated immutable catalog")
	}
	report.Pass = true
	return report, nil
}

func main() {
	catalogPath := flag.String("catalog", "games/yuyuhakusho/data/animation-catalog.json.gz", "bounded semantic catalog")
	oraclePath := flag.String("oracle", "games/yuyuhakusho/data/animation-catalog-oracle.json.gz", "isolated original CPU outputs")
	flag.Parse()
	catalog, oracle, err := loadInputs(*catalogPath, *oraclePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	report, err := verifyCatalog(catalog, oracle)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
