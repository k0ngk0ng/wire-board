package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type pcmOriginalCall struct {
	Name     string
	Code     uint8
	Command  PCMArbitrationCommand
	View     PCMArbitrationView
	Expected PCMArbitrationDecision
	RAMBytes int             `json:"ram_bytes"`
	Unknown  []int           `json:"unknown_writes"`
	RawDelta json.RawMessage `json:"delta"`
	ReadPC   string          `json:"read_pc"`
}

func pcmArbitrationFixture(t *testing.T, name string) ([]pcmOriginalCall, map[string]json.RawMessage) {
	t.Helper()
	zipped, err := os.ReadFile("testdata/pcm-arbitration-" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("testdata/pcm-arbitration-" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass bool
		SHA  string `json:"fixture_json_sha256"`
		Gzip string `json:"fixture_gzip_sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	packed := sha256.Sum256(zipped)
	if !manifest.Pass || hex.EncodeToString(digest[:]) != manifest.SHA || hex.EncodeToString(packed[:]) != manifest.Gzip {
		t.Fatal("unverified original PCM decisions")
	}
	var body struct {
		Schema int
		Calls  []pcmOriginalCall
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Schema != 1 || len(body.Calls) == 0 {
		t.Fatal("missing original PCM boundary calls")
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	return body.Calls, fields
}

func TestPCMArbitrationOriginalBoundaries(t *testing.T) {
	calls, fields := pcmArbitrationFixture(t, "private")
	if len(calls) != 13036 {
		t.Fatal("lost original boundaries", len(calls))
	}
	var all, boundary, continuous int
	_ = json.Unmarshal(fields["all_descriptor_calls"], &all)
	_ = json.Unmarshal(fields["boundary_calls"], &boundary)
	_ = json.Unmarshal(fields["continuous_calls"], &continuous)
	if all != 492 || boundary != 12288 || continuous != 256 {
		t.Fatal("lost descriptor/edge/continuous scope", all, boundary, continuous)
	}
	seen := make(map[uint8]int)
	destinations := make(map[PCMDestination]int)
	for _, call := range calls {
		input, command := call.View, call.Command
		got := SelectPCMCommand(command, input)
		if got != call.Expected || input != call.View || command != call.Command {
			t.Fatalf("%s code%x got%+v want%+v", call.Name, call.Code, got, call.Expected)
		}
		if call.RAMBytes != 65536 || len(call.Unknown) != 0 || (call.ReadPC != "0x37da4" && call.ReadPC != "0x37e9e") {
			t.Fatal("unclosed source boundary", call.Name)
		}
		var deltas [][]json.RawMessage
		if err := json.Unmarshal(call.RawDelta, &deltas); err != nil {
			t.Fatal(err)
		}
		for _, row := range deltas {
			if len(row) != 3 {
				t.Fatal("invalid source delta")
			}
			var address int
			var before, after string
			_ = json.Unmarshal(row[0], &address)
			_ = json.Unmarshal(row[1], &before)
			_ = json.Unmarshal(row[2], &after)
			old, err := hex.DecodeString(before)
			if err != nil {
				t.Fatal(err)
			}
			updated, err := hex.DecodeString(after)
			if err != nil || len(old) != len(updated) {
				t.Fatal("invalid source write range")
			}
			for i := range old {
				at := address + i
				if at != 0xf82b && at != 0xf82c && (at < 0xfee0 || at >= 0xff00) {
					t.Fatal("source wrote outside declared semantic/stack owner", call.Name, at)
				}
			}
		}
		if strings.HasPrefix(call.Name, "all-") {
			seen[call.Code]++
		}
		destinations[got.Destination]++
	}
	if len(seen) != 123 || seen[0xf0] != 4 {
		t.Fatal("missing complete PCM/F0 descriptors")
	}
	for code := 6; code <= 127; code++ {
		if seen[uint8(code)] != 4 {
			t.Fatal("missing descriptor", code)
		}
	}
	for _, destination := range []PCMDestination{PCMReject, PCMSingle, PCMSlotA, PCMSlotB} {
		if destinations[destination] == 0 {
			t.Fatal("missing actual branch", destination)
		}
	}
}

func TestPCMArbitrationObservedPairCommands(t *testing.T) {
	calls, _ := pcmArbitrationFixture(t, "sampled")
	if len(calls) != 33 {
		t.Fatal("lost real pair dispatches", len(calls))
	}
	for _, call := range calls {
		if got := SelectPCMCommand(call.Command, call.View); got != call.Expected {
			t.Fatal(call.Name, got, call.Expected)
		}
	}
}

func TestPCMArbitrationSingleInitialDecisionStreams(t *testing.T) {
	calls, _ := pcmArbitrationFixture(t, "private")
	streams := make(map[string][]pcmOriginalCall)
	for _, call := range calls {
		if strings.HasPrefix(call.Name, "stream-") {
			parts := strings.Split(call.Name, "-")
			streams[parts[1]] = append(streams[parts[1]], call)
		}
	}
	if len(streams) != 8 {
		t.Fatal("lost decision streams")
	}
	for name, stream := range streams {
		if len(stream) != 32 {
			t.Fatal(name, "lost stream calls")
		}
		choice, duck := stream[0].View.ChoiceBits, stream[0].View.DuckMode
		for i, call := range stream {
			if !strings.HasSuffix(call.Name, "-"+strconv.Itoa(i)) {
				t.Fatal("stream reordering")
			}
			view := call.View
			view.ChoiceBits, view.DuckMode = choice, duck
			got := SelectPCMCommand(call.Command, view)
			if got != call.Expected {
				t.Fatal(name, i, got, call.Expected)
			}
			choice, duck = got.ChoiceBits, got.DuckMode
		}
	}
}

func TestPCMArbitrationTemporaryViewsAndRejectedIntent(t *testing.T) {
	view := PCMArbitrationView{ModeFlags: 0xc1, SlotFlags: [2]uint8{0xc1, 0xc1}, SlotBusy: [2]uint8{0xff, 0x80}, ChoiceBits: 0xaa, DuckMode: 0xff}
	command := PCMArbitrationCommand{Flags: 1, RoutingOrPan: 0x80}
	before := view
	got := SelectPCMCommand(command, view)
	if got != (PCMArbitrationDecision{Destination: PCMReject, ChoiceBits: 0xaa, DuckMode: 0xff}) || !reflect.DeepEqual(view, before) {
		t.Fatal("rejected intent changed canonical view", got, view)
	}
	typ := reflect.TypeFor[PCMArbitrationDecision]()
	for _, field := range []string{"Queue", "AudioPause", "ModeFlags", "SlotFlags", "SlotBusy", "Registers", "RAM", "Program"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatal("decision owns input/hardware state", field)
		}
	}
}
