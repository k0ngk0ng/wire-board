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

type laneMotionSnapshot struct {
	Body       FighterBodyState
	ActorBytes string `json:"actor_bytes"`
	Pool       struct {
		Cursor uint16
		Bytes  string
	}
	Occupied uint8
}
type laneMotionCase struct {
	Before, After laneMotionSnapshot
	Slots         [2]int
	Landed        bool
}

// Original fixtures and these tests precede StepLaneAction. Unknown actor and
// pool bytes stay in the test adapter and participate in full byte comparisons.
func readLaneMotion(t *testing.T, name string, target any) {
	t.Helper()
	zdata, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zdata))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass bool
		SHA  string `json:"fixture_sha256"`
		Gzip string `json:"gzip_sha256"`
	}
	if err = json.Unmarshal(mraw, &m); err != nil {
		t.Fatal(err)
	}
	d, zipped := sha256.Sum256(raw), sha256.Sum256(zdata)
	if !m.Pass || hex.EncodeToString(d[:]) != m.SHA || hex.EncodeToString(zipped[:]) != m.Gzip {
		t.Fatal("unverified lane-motion fixture")
	}
	if err = json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}

func compareLaneMotion(t *testing.T, body FighterBodyState, pool EffectPool, occupied uint8, actorRaw, poolRaw []byte, want laneMotionSnapshot) ([]byte, []byte) {
	t.Helper()
	expectedPool, expectedPoolRaw := decodeEffectPool(t, want.Pool.Cursor, want.Pool.Bytes)
	expectedActorRaw, err := hex.DecodeString(want.ActorBytes)
	if err != nil {
		t.Fatal(err)
	}
	actorBytes, poolBytes := encodeJumpBody(body, actorRaw), encodeEffectPool(pool, poolRaw)
	if body != want.Body || pool != expectedPool || occupied != want.Occupied || !bytes.Equal(actorBytes, expectedActorRaw) || !bytes.Equal(poolBytes, expectedPoolRaw) {
		t.Fatalf("lane motion differs: body=%+v want=%+v occupied=%x/%x", body, want.Body, occupied, want.Occupied)
	}
	return actorBytes, poolBytes
}

func TestLaneMotionOriginalCalls(t *testing.T) {
	for _, name := range []string{"lane-motion-oracle", "lane-motion-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []laneMotionCase
			readLaneMotion(t, name, &rows)
			if len(rows) == 0 {
				t.Fatal("empty phase trace")
			}
			for i, r := range rows {
				pool, poolBytes := decodeEffectPool(t, r.Before.Pool.Cursor, r.Before.Pool.Bytes)
				actorBytes, err := hex.DecodeString(r.Before.ActorBytes)
				if err != nil {
					t.Fatal(err)
				}
				body, pool, occupied, slots, landed, err := StepLaneAction(r.Before.Body, pool, r.Before.Occupied)
				if err != nil || slots != r.Slots || landed != r.Landed {
					t.Fatalf("call %d result differs: %v %v/%v %v/%v", i, err, slots, r.Slots, landed, r.Landed)
				}
				compareLaneMotion(t, body, pool, occupied, actorBytes, poolBytes, r.After)
			}
		})
	}
}

func TestLaneMotionSingleInitialState(t *testing.T) {
	var sequences []struct {
		Initial laneMotionCase
		Steps   []laneMotionCase
	}
	readLaneMotion(t, "lane-motion-sequences", &sequences)
	if len(sequences) != 12 {
		t.Fatal("missing complete lane sequences")
	}
	for i, sequence := range sequences {
		initial := sequence.Initial
		body := initial.Before.Body
		pool, poolBytes := decodeEffectPool(t, initial.Before.Pool.Cursor, initial.Before.Pool.Bytes)
		actorBytes, err := hex.DecodeString(initial.Before.ActorBytes)
		if err != nil {
			t.Fatal(err)
		}
		body, pool, occupied, slots, err := BeginLaneSwitch(body, pool, initial.Before.Occupied)
		if err != nil || slots != initial.Slots {
			t.Fatal("constructor mismatch", i, err)
		}
		actorBytes, poolBytes = compareLaneMotion(t, body, pool, occupied, actorBytes, poolBytes, initial.After)
		for j, step := range sequence.Steps {
			// Go owns the previous outputs. Never reload step.Before into it.
			var landed bool
			body, pool, occupied, slots, landed, err = StepLaneAction(body, pool, occupied)
			if err != nil || slots != step.Slots || landed != step.Landed {
				t.Fatalf("sequence %d step %d: %v", i, j, err)
			}
			actorBytes, poolBytes = compareLaneMotion(t, body, pool, occupied, actorBytes, poolBytes, step.After)
		}
		if len(sequence.Steps) == 0 || !sequence.Steps[len(sequence.Steps)-1].Landed || body.ActionKind != 0 {
			t.Fatal("sequence did not finish landing", i)
		}
	}
}

func TestLaneMotionRejectsWrongAction(t *testing.T) {
	body := FighterBodyState{X: 123}
	pool := EffectPool{Cursor: 0}
	got, p, occupied, slots, landed, err := StepLaneAction(body, pool, 0x80)
	if err == nil || got != body || p != pool || occupied != 0x80 || slots != [2]int{-1, -1} || landed {
		t.Fatal("invalid lane mode mutated state")
	}
}
