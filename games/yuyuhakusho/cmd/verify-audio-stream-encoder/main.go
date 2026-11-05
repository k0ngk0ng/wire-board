// Command verify-audio-stream-encoder writes pure-Go PCM/FLAC/fMP4 acceptance
// fixtures. It does not load a ROM, synthesize game audio or launch ffmpeg.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/stream"
)

type fragment struct {
	Index        int    `json:"index"`
	Sequence     uint32 `json:"sequence"`
	File         string `json:"file"`
	SHA          string `json:"sha256"`
	StartSample  uint64 `json:"start_sample"`
	SampleFrames uint32 `json:"sample_frames"`
	Packets      uint32 `json:"packets"`
	Bytes        int    `json:"bytes"`
}

type manifest struct {
	Schema         int        `json:"schema"`
	Pass           bool       `json:"pass"`
	Scope          string     `json:"scope"`
	Environment    string     `json:"environment"`
	MIME           string     `json:"mime"`
	SampleRate     uint32     `json:"sample_rate"`
	Channels       uint8      `json:"channels"`
	BitsPerSample  uint8      `json:"bits_per_sample"`
	StereoFrames   uint64     `json:"stereo_frames"`
	SampleFrames   uint64     `json:"sample_frames"`
	SourceSHA      string     `json:"source_pcm_sha256"`
	InitFile       string     `json:"init_file"`
	InitSHA        string     `json:"init_sha256"`
	BlockFrames    uint32     `json:"block_frames"`
	FragmentFrames uint32     `json:"fragment_frames"`
	Closed         bool       `json:"closed"`
	Fragments      []fragment `json:"fragments"`
	Dependencies   []string   `json:"encoder_runtime_external_dependencies"`
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func outputInsideWorktree(path string) (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("output must remain in the current worktree")
	}
	ancestor := full
	for {
		_, statErr := os.Stat(ancestor)
		if statErr == nil {
			break
		}
		if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", statErr
		}
		ancestor = parent
	}
	real, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("output symlink leaves the current worktree")
	}
	return full, nil
}

func run() error {
	input := flag.String("pcm", "", "source stereo signed16 little-endian PCM")
	output := flag.String("output", ".local/md-yuyuhakusho/go-audio-stream", "fixture output inside worktree")
	block := flag.Uint64("block", 1024, "FLAC block sample frames")
	fragmentSize := flag.Uint64("fragment", 4096, "MSE fragment sample frames")
	maximum := flag.Uint64("max-input", 65536, "maximum sample frames in oneAppend")
	chunks := flag.String("chunks", "65536", "comma-separated input partition sizes in sample frames")
	flag.Parse()
	if *input == "" || *block > 1<<32-1 || *fragmentSize > 1<<32-1 || *maximum > 1<<32-1 {
		return errors.New("invalid PCM path or unsigned32bit configuration")
	}
	config := stream.DefaultConfig()
	config.BlockFrames = uint32(*block)
	config.FragmentFrames = uint32(*fragmentSize)
	config.MaxInputFrames = uint32(*maximum)
	encoder, err := stream.New(config)
	if err != nil {
		return err
	}
	var pattern []int
	for _, token := range strings.Split(*chunks, ",") {
		n, parseErr := strconv.ParseUint(token, 10, 32)
		if parseErr != nil || n == 0 || n > uint64(config.MaxInputFrames) {
			return errors.New("invalid input partition")
		}
		pattern = append(pattern, int(n))
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	if len(raw)%4 != 0 {
		return errors.New("source PCM does not contain complete stereo sample frames")
	}
	pcm := make([]int16, len(raw)/2)
	for index := range pcm {
		pcm[index] = int16(binary.LittleEndian.Uint16(raw[index*2:]))
	}
	directory, err := outputInsideWorktree(*output)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	init := encoder.InitSegment()
	if err = os.WriteFile(filepath.Join(directory, "init.mp4"), init, 0644); err != nil {
		return err
	}
	report := manifest{Schema: 1, Scope: "pure-go-pcm-flac-mp4", Environment: runtime.GOOS + "/" + runtime.GOARCH, MIME: stream.MIMEType,
		SampleRate: 44100, Channels: 2, BitsPerSample: 16, StereoFrames: uint64(len(pcm) / 2), SampleFrames: uint64(len(pcm) / 2),
		SourceSHA: digest(raw), InitFile: "init.mp4", InitSHA: digest(init), BlockFrames: config.BlockFrames, FragmentFrames: config.FragmentFrames,
		Dependencies: []string{}, Fragments: []fragment{}}
	store := func(segments []stream.Segment) error {
		for _, segment := range segments {
			index := len(report.Fragments)
			name := fmt.Sprintf("fragment-%05d.m4s", index)
			if err := os.WriteFile(filepath.Join(directory, name), segment.Data, 0644); err != nil {
				return err
			}
			report.Fragments = append(report.Fragments, fragment{Index: index, Sequence: segment.Sequence, File: name, SHA: digest(segment.Data), StartSample: segment.StartSample, SampleFrames: segment.SampleFrames, Packets: (segment.SampleFrames + config.BlockFrames - 1) / config.BlockFrames, Bytes: len(segment.Data)})
		}
		return nil
	}
	for at, part := 0, 0; at < len(pcm)/2; part++ {
		n := pattern[part%len(pattern)]
		if n > len(pcm)/2-at {
			n = len(pcm)/2 - at
		}
		segments, appendErr := encoder.AppendPCM(pcm[at*2 : (at+n)*2])
		if appendErr != nil {
			return appendErr
		}
		if err = store(segments); err != nil {
			return err
		}
		at += n
	}
	segments, err := encoder.Finish()
	if err != nil {
		return err
	}
	if err = store(segments); err != nil {
		return err
	}
	status := encoder.Status()
	report.Closed = status.Closed
	report.Pass = status.Closed && status.BufferedFrames == 0 && status.EmittedSamples == report.StereoFrames && status.AcceptedSamples == report.StereoFrames
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err = os.WriteFile(filepath.Join(directory, "manifest.json"), encoded, 0644); err != nil {
		return err
	}
	fmt.Print(string(encoded))
	if !report.Pass {
		return errors.New("encoder clock did not reach the source end")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
