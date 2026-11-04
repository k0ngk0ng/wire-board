package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

type soundQueueSnapshot struct {
	SoundPriorityState
	Bytes string
}

func decodeSoundPriorityState(t *testing.T, raw soundQueueSnapshot) SoundPriorityState {
	t.Helper()
	b, err := hex.DecodeString(raw.Bytes)
	if err != nil || len(b) != 256 {
		t.Fatal("invalid original sound RAM", err)
	}
	if b[0] != raw.CurrentPriority || b[9] != raw.SelectedSound || !bytes.Equal(b[10:14], raw.Queue[:]) {
		t.Fatal("semantic sound input does not match original bytes")
	}
	return raw.SoundPriorityState
}

func encodeSoundPriorityState(t *testing.T, state SoundPriorityState, previous soundQueueSnapshot) soundQueueSnapshot {
	t.Helper()
	b, err := hex.DecodeString(previous.Bytes)
	if err != nil || len(b) != 256 {
		t.Fatal("invalid original sound RAM", err)
	}
	b[0], b[9] = state.CurrentPriority, state.SelectedSound
	copy(b[10:14], state.Queue[:])
	return soundQueueSnapshot{SoundPriorityState: state, Bytes: hex.EncodeToString(b)}
}

func readSoundQueueFixture(t *testing.T, name string, target any) (SoundPriorityTable, int) {
	t.Helper()
	var table SoundPriorityTable
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
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
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass     bool
		Records  int
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
		DataSHA  string `json:"priority_data_sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original sound queue fixture")
	}
	var f struct {
		Priorities []uint8
		Calls      json.RawMessage
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	h = sha256.Sum256(f.Priorities)
	if len(f.Priorities) != len(table) || hex.EncodeToString(h[:]) != m.DataSHA {
		t.Fatal("unverified priority data")
	}
	copy(table[:], f.Priorities)
	if err := json.Unmarshal(f.Calls, target); err != nil {
		t.Fatal(err)
	}
	return table, m.Records
}

type soundQueueOriginalCall struct {
	Name          string
	Before, After soundQueueSnapshot
}

// Original full consumer fixtures and these tests precede sound_queue.go.
func TestSoundQueueAgainstOriginal(t *testing.T) {
	for _, name := range []string{"sound-queue-oracle", "sound-queue-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []soundQueueOriginalCall
			table, count := readSoundQueueFixture(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original consumer calls")
			}
			originalTable := table
			for i, row := range rows {
				input := decodeSoundPriorityState(t, row.Before)
				got := ConsumeSoundQueue(input, table)
				if got != decodeSoundPriorityState(t, row.After) || encodeSoundPriorityState(t, got, row.Before) != row.After {
					t.Fatalf("sound consumer %d %s: got %+v want %+v", i, row.Name, got, row.After.SoundPriorityState)
				}
			}
			if table != originalTable {
				t.Fatal("consumer changed priority data")
			}
			t.Logf("complete original sound consumers=%d", len(rows))
		})
	}
}

func TestSoundQueueSingleInitialState(t *testing.T) {
	var streams []struct {
		Name    string
		Initial soundQueueSnapshot
		Calls   []soundQueueOriginalCall
	}
	table, count := readSoundQueueFixture(t, "sound-queue-sequences", &streams)
	if count != 16 || len(streams) != count {
		t.Fatal("missing original sound consumer streams")
	}
	steps := 0
	for _, stream := range streams {
		state := decodeSoundPriorityState(t, stream.Initial)
		encoded := stream.Initial
		for i, step := range stream.Calls {
			state = ConsumeSoundQueue(state, table)
			encoded = encodeSoundPriorityState(t, state, encoded)
			if state != decodeSoundPriorityState(t, step.After) || encoded != step.After {
				t.Fatalf("sound stream %s step %d differs", stream.Name, i)
			}
			steps++
		}
	}
	if steps != 128 {
		t.Fatal("missing continuous sound steps")
	}
}
