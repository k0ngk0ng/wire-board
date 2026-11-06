package chips_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmassets"
	pcmc "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

var bridgeOnce sync.Once
var bridgeLibrary *resourcebridge.Library
var bridgeLoadError error

func bridgeUnpack(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	body, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func bridgeLoad(t *testing.T) *resourcebridge.Library {
	t.Helper()
	bridgeOnce.Do(func() {
		bridgeLibrary, bridgeLoadError = resourcebridge.Decode(bridgeUnpack(t, "../../data/audio-score.json.gz"), bridgeUnpack(t, "../../data/audio-pcm-payloads.json.gz"), bridgeUnpack(t, "../../data/audio-score-parser-work.json.gz"))
	})
	if bridgeLoadError != nil {
		t.Fatal(bridgeLoadError)
	}
	return bridgeLibrary
}

type bridgeExpectedRow struct {
	Domain     string
	Code       uint8
	Descriptor pcmassets.Descriptor
	Bytes      int `json:"encoded_bytes"`
}

func bridgeExpected(t *testing.T) []bridgeExpectedRow {
	t.Helper()
	raw, err := os.ReadFile("../pcmassets/testdata/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct{ Rows []bridgeExpectedRow }
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Rows) != 129 {
		t.Fatal("original descriptor scope differs")
	}
	return f.Rows
}

func TestResourceBridgeActualCatalogOwnsAllOriginalBytes(t *testing.T) {
	library := bridgeLoad(t)
	cfg, err := library.PCMConfig(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Descriptors) != 129 || !cfg.IncludeYMAddresses || cfg.Resources.Identity != library.Identity() {
		t.Fatal("unbound physical config")
	}
	byID := map[string]pcmc.Descriptor{}
	for i, d := range cfg.Descriptors {
		if i > 0 && cfg.Descriptors[i-1].ID >= d.ID {
			t.Fatal("descriptor enumeration not canonical")
		}
		byID[d.ID] = d
	}
	for _, row := range bridgeExpected(t) {
		d, ok := byID[row.Descriptor.ID]
		protocol := pcmc.OrdinaryProtocol
		if row.Domain == "music" {
			protocol = pcmc.MusicProtocol
		}
		if !ok || d != (pcmc.Descriptor{ID: row.Descriptor.ID, PayloadID: row.Descriptor.SampleID, NativeRate: row.Descriptor.NativeRate, Flags: row.Descriptor.Flags, RoutingOrPan: row.Descriptor.RoutingOrPan, Protocol: protocol}) {
			t.Fatal("original descriptor differs", row.Domain, row.Code)
		}
		n, ok := cfg.Resources.Length(d.PayloadID)
		if !ok || int(n) != row.Bytes {
			t.Fatal("original payload length differs")
		}
		h := sha256.New()
		for at := uint32(0); at < n; at++ {
			b, ok := cfg.Resources.ByteAt(d.PayloadID, at)
			if !ok {
				t.Fatal("source byte unavailable")
			}
			h.Write([]byte{b})
		}
		if hex.EncodeToString(h.Sum(nil)) != d.PayloadID {
			t.Fatal("original encoded content differs")
		}
		if _, ok = cfg.Resources.ByteAt(d.PayloadID, n); ok {
			t.Fatal("payload outside its own content")
		}
	}
	if _, ok := cfg.Resources.Length("missing"); ok {
		t.Fatal("unknown payload length invented")
	}
	cfg.Descriptors[0].ID = "changed"
	fresh, err := library.PCMConfig(0, 0)
	if err != nil || fresh.Descriptors[0].ID == "changed" {
		t.Fatal("config aliases immutable descriptors")
	}
	if library.Catalog() == nil || library.Parser() == nil {
		t.Fatal("score/parser dependency missing")
	}
	driver, err := score.New(library.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = driver.AdvanceObserved(score.ObservedInput{}, score.ObservedHooks{Parser: library.Parser(), Operation: func(score.Operation) error { return nil }}); err != nil {
		t.Fatal("parser not bound to same score", err)
	}
	pcm, err := pcmassets.Decode(bytes.NewReader(bridgeUnpack(t, "../../data/audio-pcm-payloads.json.gz")), bridgeUnpack(t, "../../data/audio-score.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	all := pcm.Descriptors()
	if len(all) != 129 {
		t.Fatal("enumeration lost descriptors")
	}
	id := all[0].ID
	all[0].ID = "changed"
	if _, ok := pcm.Descriptor(id); !ok || pcm.Descriptors()[0].ID != id {
		t.Fatal("enumeration exposes internal map")
	}
	var absent *pcmassets.Catalog
	if absent.Descriptors() != nil {
		t.Fatal("nil catalog enumerated data")
	}
}

func TestResourceBridgeProductionResourcesOriginalFullChip(t *testing.T) {
	library := bridgeLoad(t)
	sources, native := pcmIntegrationCases(t)
	frames, events, stereo := 0, 0, 0
	for _, c := range sources {
		t.Run(c.Name, func(t *testing.T) {
			cfg, err := library.PCMConfig(0, 0)
			if err != nil {
				t.Fatal(err)
			}
			ids := map[string]string{}
			for _, old := range c.Descriptors {
				for _, d := range cfg.Descriptors {
					if old.PayloadID == d.PayloadID && old.NativeRate == d.NativeRate && old.Flags == d.Flags && old.RoutingOrPan == d.RoutingOrPan && old.Protocol == d.Protocol {
						ids[old.ID] = d.ID
						break
					}
				}
				if ids[old.ID] == "" {
					t.Fatal("original descriptor absent from production catalog", old.ID)
				}
			}
			want := native[c.Name]
			config := mixed.Config{PCM: cfg, Chips: want.Config}
			r, err := mixed.New(config, want.Initial)
			if err != nil {
				t.Fatal(err)
			}
			var transactions []pcmc.Transaction
			for index, observed := range c.Frames {
				if index == len(c.Frames)/2 {
					raw, _ := json.Marshal(r.Snapshot())
					var saved mixed.State
					if err = json.Unmarshal(raw, &saved); err != nil {
						t.Fatal(err)
					}
					r, err = mixed.Restore(config, saved)
					if err != nil {
						t.Fatal(err)
					}
				}
				plan := mixedPlan(c, index)
				for i := range plan.Inputs {
					if id := plan.Inputs[i].DescriptorID; id != "" {
						plan.Inputs[i].DescriptorID = ids[id]
					}
				}
				got, err := r.AdvanceFrame(plan)
				if err != nil {
					t.Fatalf("frame%d %v", index, err)
				}
				if !reflect.DeepEqual(pcmIntegrationNormalize(got.Protocol.Ledger), pcmIntegrationNormalize(observed.Events)) || got.Protocol.Status != observed.Status {
					t.Fatal("production config physical output differs", index)
				}
				equalPCM(t, got.PCM, want.Frames[index].PCM)
				chip, err := chips.RestoreInteractive(want.Config, r.Snapshot().Chips)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := chip.CommittedState()
				if err != nil || !reflect.DeepEqual(actual, want.Frames[index].State) {
					t.Fatal("production config full YM/PSG/FIR differs", index, err)
				}
				transactions = append(transactions, got.Protocol.Transactions...)
				frames++
				events += len(got.Protocol.Ledger)
				stereo += len(got.PCM) / 2
			}
			if !reflect.DeepEqual(transactions, c.Transactions) {
				t.Fatal("production config transactions differ")
			}
		})
	}
	if frames != 1274 || events != 519256 || stereo != 937609 {
		t.Fatal("production catalog matrix incomplete", frames, events, stereo)
	}
}

func TestResourceBridgeOriginalOrdinaryArbitration(t *testing.T) {
	library := bridgeLoad(t)
	actual := map[uint8]pcmassets.Descriptor{}
	for _, row := range bridgeExpected(t) {
		if row.Domain == "ordinary" {
			actual[row.Code] = row.Descriptor
		}
	}
	var f struct {
		Schema int
		Calls  []struct {
			Code     uint8
			Command  engine.PCMArbitrationCommand
			View     engine.PCMArbitrationView
			Expected engine.PCMArbitrationDecision
		}
	}
	pcmIntegrationJSON(t, "../../engine/testdata/pcm-arbitration-private.json.gz", &f)
	if f.Schema != 1 || len(f.Calls) != 13036 {
		t.Fatal("native arbitration scope differs")
	}
	count := 0
	var destinations [4]int
	for _, row := range f.Calls {
		d, ok := actual[row.Code]
		v := row.View
		if !ok || d.Flags != row.Command.Flags || d.RoutingOrPan != row.Command.RoutingOrPan || v.ChoiceBits > 1 || v.DuckMode != 0 || v.ModeFlags&0x20 != 0 {
			continue
		}
		status := pcmc.Status{ModeFlags: v.ModeFlags, SlotFlags: v.SlotFlags, SlotBusy: v.SlotBusy}
		got, err := library.PrepareOrdinary(row.Code, status, v.ChoiceBits)
		if err != nil {
			t.Fatal(err)
		}
		if got.Accepted != (row.Expected.Destination != engine.PCMReject) || got.ChoiceBits != row.Expected.ChoiceBits {
			t.Fatal("native priority/choice differs", row.Code)
		}
		if got.Accepted {
			want := pcmc.Input{Kind: pcmc.Ordinary, DescriptorID: d.ID, Destination: pcmc.Destination(row.Expected.Destination - 1), StartDual: row.Expected.StartDual}
			if got.Input != want {
				t.Fatal("native accepted publication differs", row.Code, got.Input, want)
			}
		} else if got.Input != (pcmc.Input{}) {
			t.Fatal("rejected request generated publication")
		}
		count++
		destinations[row.Expected.Destination]++
	}
	if count != 1350 || destinations != ([4]int{512, 113, 346, 379}) {
		t.Fatal("closed original arbitration scope incomplete", count, destinations)
	}
}

func TestResourceBridgeOriginalAcceptedMusicIntents(t *testing.T) {
	library := bridgeLoad(t)
	var f struct {
		Version int
		Index   struct{ Path, SHA256 string }
		Sources []struct{ Path, SHA256 string }
		Counts  map[string]int
		Rows    []struct{ Intent score.Intent }
	}
	pcmIntegrationJSON(t, "../../verification/audio-resource-bridge/native-pcm-intents.json.gz", &f)
	if f.Version != 1 || len(f.Sources) != 42 || len(f.Rows) != 9232 || f.Counts["dac"] != 9230 || f.Counts["dac-pan"] != 2 {
		t.Fatal("original music intent scope differs")
	}
	for _, source := range append(f.Sources, f.Index) {
		raw, err := os.ReadFile("../../../../" + source.Path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != source.SHA256 {
			t.Fatal("original music source changed", source.Path)
		}
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, row := range f.Rows {
		original := row.Intent
		beforePan := *original.Pan
		got, err := library.PrepareMusic(original)
		if err != nil {
			t.Fatal(err)
		}
		pan := uint8(0)
		if original.Pan.Left {
			pan |= 128
		}
		if original.Pan.Right {
			pan |= 64
		}
		want := pcmc.Input{Kind: pcmc.Music, DescriptorID: original.DescriptorID, Pan: pan}
		if original.Kind == "dac-pan" {
			want = pcmc.Input{Kind: pcmc.SetRestorePan, Pan: pan}
		} else {
			seen[original.DescriptorID] = true
		}
		if got != want || !reflect.DeepEqual(original, row.Intent) || *original.Pan != beforePan {
			t.Fatal("original accepted music publication differs")
		}
		counts[original.Kind]++
	}
	if len(seen) != 6 || counts["dac"] != 9230 || counts["dac-pan"] != 2 {
		t.Fatal("six actual music dependencies not closed")
	}
}

func TestResourceBridgeRejectsUnknownAndCrossProtocolInputs(t *testing.T) {
	library := bridgeLoad(t)
	before := library.Identity()
	stereo := score.Stereo{Left: true, Right: true}
	for _, code := range []uint8{0, 1, 4, 0x7f, 0x82, 0xff} {
		if _, err := library.PrepareOrdinary(code, pcmc.Status{}, 0); err == nil {
			t.Fatal("nonordinary/fight-excluded request accepted", code)
		}
	}
	if _, err := library.PrepareOrdinary(6, pcmc.Status{}, 2); err == nil {
		t.Fatal("private choice profile accepted")
	}
	if _, err := library.PrepareOrdinary(6, pcmc.Status{ModeFlags: 0x20}, 0); err == nil {
		t.Fatal("private duck profile accepted")
	}
	for _, intent := range []score.Intent{{Kind: "ym"}, {Kind: "dac", DescriptorID: "missing", Pan: &stereo}, {Kind: "dac", DescriptorID: "descriptor-ac63deae129cdee66afd", Pan: &stereo}, {Kind: "dac-pan"}, {Kind: "dac-pan", DescriptorID: "missing", Pan: &stereo}, {Kind: "dac-pan", Pan: &stereo, Value: 1}, {Kind: "dac-pan", Pan: &stereo, Order: -1}} {
		if _, err := library.PrepareMusic(intent); err == nil {
			t.Fatal("invalid music intent accepted", intent)
		}
	}
	for _, marker := range []uint8{0, 1, 127, 128, 255} {
		for flags := 0; flags < 256; flags++ {
			status := pcmc.Status{StartMarker: marker, ModeFlags: uint8(flags)}
			want := score.DACFeedback{Blocked: marker&128 != 0, Priority: uint8(flags) & 192, PanRedirect: marker != 0}
			if got := resourcebridge.Feedback(status); got != want {
				t.Fatal("feedback stages conflated", marker, flags, got, want)
			}
		}
	}
	if library.Identity() != before {
		t.Fatal("rejected input mutated resource binding")
	}
	var absent *resourcebridge.Library
	if _, err := absent.PCMConfig(0, 0); err == nil {
		t.Fatal("nil config accepted")
	}
	if _, err := absent.PrepareOrdinary(6, pcmc.Status{}, 0); err == nil {
		t.Fatal("nil ordinary catalog accepted")
	}
	if _, err := absent.PrepareMusic(score.Intent{}); err == nil {
		t.Fatal("nil music catalog accepted")
	}
}

func TestResourceBridgeResourceDecodeIsAtomic(t *testing.T) {
	scoreJSON := bridgeUnpack(t, "../../data/audio-score.json.gz")
	pcmJSON := bridgeUnpack(t, "../../data/audio-pcm-payloads.json.gz")
	parserJSON := bridgeUnpack(t, "../../data/audio-score-parser-work.json.gz")
	for _, row := range []struct {
		name    string
		s, p, m []byte
	}{{"missing-score", nil, pcmJSON, parserJSON}, {"broken-pcm", scoreJSON, []byte("{}"), parserJSON}, {"broken-parser", scoreJSON, pcmJSON, []byte("{}")}, {"different-score-source", append(append([]byte(nil), scoreJSON...), ' '), pcmJSON, parserJSON}} {
		t.Run(row.name, func(t *testing.T) {
			got, err := resourcebridge.Decode(row.s, row.p, row.m)
			if err == nil || got != nil {
				t.Fatal("invalid resource set returned partial library")
			}
		})
	}
}
