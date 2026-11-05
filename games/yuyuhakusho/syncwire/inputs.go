// Package syncwire transfers bounded physical-input history, not game saves.
// The same12-bit records are consumed by native Go and the browser's Wasm.
package syncwire

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strconv"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

const Version uint8 = 1
const MaxFrames = 120
const runBytes = 10

// InputFrame supplies the exact held state and host-derived automation for
// one independent uint64 tick. The tick application, not this codec, computes
// edges or applies control changes. World.Controllers is never changed here.
type InputFrame struct {
	Frame     uint64            `json:"frame,string"`
	Held      [4]engine.Buttons `json:"held"`
	Automated uint8             `json:"automated"`
}

// InputWindow is a canonical versioned RLE packet. FirstFrame is a decimal JSON
// string so JavaScript never rounds frame numbers larger than2^53. Empty is
// exactly{version:1,firstFrame:"0",frameCount:0,data:""}.
type InputWindow struct {
	Version    uint8  `json:"version"`
	FirstFrame uint64 `json:"firstFrame,string"`
	FrameCount uint16 `json:"frameCount"`
	Data       string `json:"data"`
}

func validateFrame(frame InputFrame) error {
	if frame.Frame == 0 {
		return errors.New("input frame must be nonzero")
	}
	if frame.Automated&0xf0 != 0 {
		return errors.New("input automation contains unknown seats")
	}
	for _, held := range frame.Held {
		if err := engine.ValidateControllerButtons(held); err != nil {
			return err
		}
	}
	return nil
}
func validateFrames(frames []InputFrame) error {
	if len(frames) > MaxFrames {
		return errors.New("input history exceeds120 frames")
	}
	for index, frame := range frames {
		if err := validateFrame(frame); err != nil {
			return err
		}
		if index > 0 && (frames[index-1].Frame == ^uint64(0) || frame.Frame != frames[index-1].Frame+1) {
			return errors.New("input frames are not consecutive")
		}
	}
	return nil
}

// Encode accepts only a complete consecutive input window and does not retain
// caller memory. Every run is length byte, automation byte, four big-endian
// uint16 held masks. No JavaScript bitwise packing is required.
func Encode(frames []InputFrame) (InputWindow, error) {
	if err := validateFrames(frames); err != nil {
		return InputWindow{}, err
	}
	if len(frames) == 0 {
		return InputWindow{Version: Version}, nil
	}
	raw := make([]byte, 0, len(frames)*runBytes)
	for index := 0; index < len(frames); {
		first := frames[index]
		count := 1
		for index+count < len(frames) && frames[index+count].Held == first.Held && frames[index+count].Automated == first.Automated {
			count++
		}
		raw = append(raw, uint8(count), first.Automated)
		for _, held := range first.Held {
			raw = binary.BigEndian.AppendUint16(raw, uint16(held))
		}
		index += count
	}
	return InputWindow{Version: Version, FirstFrame: frames[0].Frame, FrameCount: uint16(len(frames)), Data: base64.StdEncoding.EncodeToString(raw)}, nil
}

// Decode validates the entire packet before returning any frames. Alternative
// base64 encodings, adjacent equal runs, zero runs and trailing data are errors.
func Decode(window InputWindow) ([]InputFrame, error) {
	if window.Version != Version {
		return nil, errors.New("unsupported input window version")
	}
	if window.FrameCount == 0 {
		if window.FirstFrame != 0 || window.Data != "" {
			return nil, errors.New("noncanonical empty input window")
		}
		return []InputFrame{}, nil
	}
	if window.FrameCount > MaxFrames || window.FirstFrame == 0 || window.FirstFrame > ^uint64(0)-(uint64(window.FrameCount)-1) || len(window.Data) > base64.StdEncoding.EncodedLen(MaxFrames*runBytes) {
		return nil, errors.New("invalid input window bounds")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(window.Data)
	if err != nil || len(raw) == 0 || len(raw)%runBytes != 0 || base64.StdEncoding.EncodeToString(raw) != window.Data {
		return nil, errors.New("invalid or noncanonical input window encoding")
	}
	out := make([]InputFrame, 0, int(window.FrameCount))
	for at := 0; at < len(raw); at += runBytes {
		run := raw[at : at+runBytes]
		count := int(run[0])
		if count == 0 || count > MaxFrames || count > int(window.FrameCount)-len(out) {
			return nil, errors.New("invalid input run length")
		}
		if at > 0 && bytes.Equal(raw[at-runBytes+1:at], run[1:]) {
			return nil, errors.New("adjacent equal input runs are noncanonical")
		}
		frame := InputFrame{Frame: window.FirstFrame + uint64(len(out)), Automated: run[1]}
		for seat := range frame.Held {
			frame.Held[seat] = engine.Buttons(binary.BigEndian.Uint16(run[2+seat*2 : 4+seat*2]))
		}
		if err := validateFrame(frame); err != nil {
			return nil, err
		}
		for step := 0; step < count; step++ {
			frame.Frame = window.FirstFrame + uint64(len(out))
			out = append(out, frame)
		}
	}
	if len(out) != int(window.FrameCount) {
		return nil, errors.New("input run count differs from window")
	}
	return out, nil
}

// DecodeJSON enforces exact field presence, decimal-string frames and one JSON
// object. It complements Decode: ordinary struct unmarshal cannot reject a
// repeated key or distinguish a missing zero-valued field from a supplied one.
func DecodeJSON(raw []byte) ([]InputFrame, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return nil, errors.New("invalid input window JSON size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("input window must be an object")
	}
	fields := make(map[string]json.RawMessage, 4)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok || name != "version" && name != "firstFrame" && name != "frameCount" && name != "data" || fields[name] != nil {
			return nil, errors.New("unknown or repeated input window JSON field")
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, errors.New("null input window JSON field")
		}
		fields[name] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("invalid input window JSON object")
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, errors.New("input window contains trailing JSON")
	}
	if len(fields) != 4 {
		return nil, errors.New("input window JSON field missing")
	}
	var decimal string
	if err = json.Unmarshal(fields["firstFrame"], &decimal); err != nil {
		return nil, errors.New("input frame must be a decimal string")
	}
	frame, err := strconv.ParseUint(decimal, 10, 64)
	if err != nil || strconv.FormatUint(frame, 10) != decimal {
		return nil, errors.New("noncanonical input frame decimal")
	}
	var window InputWindow
	if err = json.Unmarshal(raw, &window); err != nil {
		return nil, err
	}
	return Decode(window)
}

// Push copies a valid retained history, appends exactly one consecutive frame
// and drops only the oldest entries beyond120. Neither the old slice nor its
// backing array is modified or retained in the result.
func Push(history []InputFrame, next InputFrame) ([]InputFrame, error) {
	if err := validateFrames(history); err != nil {
		return nil, err
	}
	if err := validateFrame(next); err != nil {
		return nil, err
	}
	if len(history) > 0 && (history[len(history)-1].Frame == ^uint64(0) || next.Frame != history[len(history)-1].Frame+1) {
		return nil, errors.New("new input is not the next history frame")
	}
	size := len(history) + 1
	drop := 0
	if size > MaxFrames {
		size = MaxFrames
		drop = 1
	}
	out := make([]InputFrame, size)
	copy(out, history[drop:])
	out[len(out)-1] = next
	return out, nil
}
