package cs16

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestColdHistoryCodecBoundsCorruptionAndDeterminism(t *testing.T) {
	raw := bytes.Repeat([]byte("history-value-"), 4000)
	packed, err := compressHistory(raw)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := expandHistory(packed, len(raw))
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatal("history compression changed bytes", err)
	}
	corrupt := bytes.Clone(packed)
	corrupt[len(corrupt)-5] ^= 1
	for _, candidate := range []struct {
		body []byte
		size int
	}{{packed[:len(packed)-1], len(raw)}, {corrupt, len(raw)}, {append(bytes.Clone(packed), 0), len(raw)}, {packed, len(raw) - 1}, {packed, len(raw) + 1}, {packed, 0}, {packed, 2097153}} {
		if _, err := expandHistory(candidate.body, candidate.size); err == nil {
			t.Fatal("corrupt, ambiguous or oversized history accepted")
		}
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 5 {
				actual, err := compressHistory(raw)
				if err != nil || !bytes.Equal(actual, packed) {
					t.Error("concurrent compression changed bytes", err)
				}
			}
		}()
	}
	workers.Wait()
}

func TestColdHistoryKeepsAllFramesAndOrdinaryInputCheckpoint(t *testing.T) {
	ctx, match := cursorFixture(t)
	ctx.Now = ctx.Now.Add(time.Second)
	match, err := onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Schema        int    `json:"schema"`
		HistoryPacked []byte `json:"historyPacked"`
		HistoryBytes  int    `json:"historyBytes"`
		World         struct {
			History json.RawMessage `json:"history"`
		} `json:"world"`
	}
	if json.Unmarshal(match.Data, &stored) != nil || stored.Schema != 2 || len(stored.HistoryPacked) == 0 || string(stored.World.History) != "null" {
		t.Fatal("history is not a separate cold checkpoint")
	}
	state := expandedMatchState(t, match)
	if len(state.World.History) != engine.MaxHistory {
		t.Fatal("history window was shortened")
	}
	raw, _ := json.Marshal(inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[ctx.Actor].Life, ViewFrame: state.World.Frame}})
	updated, err := apply(ctx, match, raw)
	if err != nil {
		t.Fatal(err)
	}
	var next struct {
		HistoryPacked []byte `json:"historyPacked"`
	}
	if json.Unmarshal(updated.Data, &next) != nil || !bytes.Equal(next.HistoryPacked, stored.HistoryPacked) {
		t.Fatal("ordinary input changed cold history")
	}
	legacy, _ := json.Marshal(legacyMatchState{Schema: 1, Epoch: state.Epoch, BaseAt: state.BaseAt, ObservedAt: state.ObservedAt, RulesSHA256: state.RulesSHA256, World: state.World})
	if len(match.Data)*2 >= len(legacy) {
		t.Fatal("cold history did not remove the repeated bulk payload")
	}
}
