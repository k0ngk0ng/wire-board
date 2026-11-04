package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

type fighterPreparationSnapshot struct {
	Input      FighterPreparationState
	Random     AttackValueState
	ActorBytes string `json:"actor_bytes"`
}

type fighterPreparationCall struct {
	Kind          string
	Name          string
	Before, After fighterPreparationSnapshot
	ExternalInput struct {
		Energy          int32
		Held            uint8
		Pressed         uint8
		ExtendedPressed uint8 `json:"extended_pressed"`
		Facing          uint8
	} `json:"external_input"`
	ExternalScale uint16 `json:"external_scale"`
}

func encodeFighterPreparation(t *testing.T, state fighterPreparationSnapshot, previous string) string {
	t.Helper()
	b, err := hex.DecodeString(previous)
	if err != nil || len(b) != 256 {
		t.Fatal("invalid original preparation actor", err)
	}
	p := state.Input
	binary.BigEndian.PutUint32(b[0xb8:], uint32(p.Energy))
	b[0x71], b[0x72], b[0xc1], b[0x61], b[0xc7], b[0x6b] = p.Held, p.Pressed, p.ExtendedPressed, p.Facing, p.ChargeTicks, p.Request
	binary.BigEndian.PutUint16(b[0xc4:], state.Random.Scale)
	b[0x65] = state.Random.Value
	return hex.EncodeToString(b)
}

func readFighterPreparationFixture(t *testing.T, name string, target any) int {
	t.Helper()
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
	manifestRaw, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Pass     bool
		Records  int
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		GzipSHA  string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !manifest.Pass || !manifest.Baseline || manifest.Records == 0 || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zh[:]) != manifest.GzipSHA {
		t.Fatal("unverified original preparation fixture")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return manifest.Records
}

func runFighterPreparation(t *testing.T, kind string, input fighterPreparationSnapshot) fighterPreparationSnapshot {
	t.Helper()
	switch kind {
	case "input":
		input.Input = PrepareFighterInput(input.Input)
	case "random":
		input.Random = RandomizeAttackValue(input.Random)
	default:
		t.Fatal("unknown original preparation kind", kind)
	}
	return input
}

// Original full-call fixtures and these tests precede fighter_preparation.go.
func TestFighterPreparationOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"fighter-preparation-oracle", "fighter-preparation-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []fighterPreparationCall
			count := readFighterPreparationFixture(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing complete preparation calls")
			}
			kinds := map[string]int{}
			for i, row := range rows {
				got := runFighterPreparation(t, row.Kind, row.Before)
				got.ActorBytes = encodeFighterPreparation(t, got, row.Before.ActorBytes)
				if got != row.After {
					t.Fatalf("preparation %d %s/%s differs: got %+v want %+v", i, row.Kind, row.Name, got, row.After)
				}
				kinds[row.Kind]++
			}
			if kinds["input"] == 0 || kinds["random"] == 0 {
				t.Fatal("missing one original preparation routine", kinds)
			}
			if name == "fighter-preparation-boundaries" && (kinds["input"] != 9074 || kinds["random"] != 540) {
				t.Fatal("missing declared isolated preparation matrix", kinds)
			}
			t.Logf("complete original calls=%d kinds=%v", count, kinds)
		})
	}
}

func TestFighterPreparationSingleInitialState(t *testing.T) {
	var streams []struct {
		Kind    string
		Initial fighterPreparationSnapshot
		Calls   []fighterPreparationCall
	}
	count := readFighterPreparationFixture(t, "fighter-preparation-sequences", &streams)
	if len(streams) != count || count != 7 {
		t.Fatal("missing original preparation streams")
	}
	steps := 0
	for i, stream := range streams {
		state := stream.Initial
		for j, call := range stream.Calls {
			if stream.Kind == "input" {
				p := call.ExternalInput
				state.Input.Energy, state.Input.Held, state.Input.Pressed = p.Energy, p.Held, p.Pressed
				state.Input.ExtendedPressed, state.Input.Facing = p.ExtendedPressed, p.Facing
			} else if stream.Kind == "random" {
				state.Random.Scale = call.ExternalScale
			} else {
				t.Fatal("unknown continuous preparation kind")
			}
			state = runFighterPreparation(t, stream.Kind, state)
			state.ActorBytes = encodeFighterPreparation(t, state, state.ActorBytes)
			if state != call.After {
				t.Fatalf("preparation stream %d %s step %d differs", i, stream.Kind, j)
			}
			steps++
		}
	}
	if steps != 896 {
		t.Fatal("missing continuous preparation steps", steps)
	}
	t.Logf("single-initial-state preparation streams=%d steps=%d", count, steps)
}
