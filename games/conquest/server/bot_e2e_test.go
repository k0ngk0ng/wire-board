package conquest

// Failure scenarios fixed before policy implementation: hidden enemy orders or
// RNG affect the choice; the choice mutates the match; already submitted or
// paused seats spam actions; impossible sea costs; perpetually recruiting with
// no advance; cycling through owned provinces; isolated islands never captured;
// all bots stop before a real sole-survivor victory; replay produces another result.
// Full games exercise the public Module contract and normal starting resources.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type botReplayRecord struct {
	Kind    string          `json:"kind"`
	Context gameapi.Context `json:"context"`
	Action  json.RawMessage `json:"action,omitempty"`
	Match   *gameapi.Match  `json:"match,omitempty"`
}

func TestConquestBotFullGamesE2E(t *testing.T) {
	for _, mapID := range []string{"world", "europe"} {
		for players := 2; players <= 8; players++ {
			for _, fog := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d/normal/fog-%t", mapID, players, fog), func(t *testing.T) {
					runBotGame(t, mapID, players, "normal", fog)
				})
			}
		}
	}
}

func TestConquestBotDifficultyGamesE2E(t *testing.T) {
	for _, difficulty := range []string{"easy", "hard"} {
		for _, mapID := range []string{"world", "europe"} {
			for players := 2; players <= 8; players++ {
				for _, fog := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%d/%s/fog-%t", mapID, players, difficulty, fog), func(t *testing.T) {
						runBotGame(t, mapID, players, difficulty, fog)
					})
				}
			}
		}
	}
}

func runBotGame(t *testing.T, mapID string, players int, difficulty string, fog bool) {
	t.Helper()
	m := Module()
	opts, err := m.Configure(players, json.RawMessage(fmt.Sprintf(`{"mapId":%q,"difficulty":%q,"turnSeconds":120,"fog":%t}`, mapID, difficulty, fog)))
	if err != nil {
		t.Fatal(err)
	}
	ctx := gameapi.Context{Actor: 0, Players: players, Version: 1, RequestedVersion: 1, Now: time.Unix(1792700000+int64(players), 0), Automated: make([]bool, players)}
	for i := range ctx.Automated {
		ctx.Automated[i] = true
	}
	match, err := m.Start(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	artifactBase := os.Getenv("CONQUEST_BOT_ARTIFACT_DIR")
	if artifactBase == "" {
		artifactBase = filepath.Join(".local", "conquest", "rules", "bots")
	}
	if !filepath.IsAbs(artifactBase) {
		artifactBase = filepath.Join(root, artifactBase)
	}
	artifactBase = filepath.Clean(artifactBase)
	relative, err := filepath.Rel(root, artifactBase)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatal("bot artifacts must remain inside this worktree")
	}
	sourceSHA256 := botSourceDigests(t, root)
	out := filepath.Join(artifactBase, mapID, fmt.Sprint(players), difficulty, fmt.Sprintf("fog-%t", fog))
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	replay, err := os.Create(filepath.Join(out, "replay.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(replay)
	encoder := json.NewEncoder(compressed)
	if err := encoder.Encode(botReplayRecord{Kind: "start", Context: ctx, Action: opts, Match: &match}); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	steps, rounds := 0, 0
	defer func() {
		_ = encoder.Encode(botReplayRecord{Kind: "final", Context: ctx, Match: &match})
		_ = compressed.Close()
		_ = replay.Close()
		raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "map": mapID, "players": players, "difficulty": difficulty, "fog": fog, "steps": steps, "rounds": rounds, "actions": counts, "result": match.Result, "sourceSHA256": sourceSHA256, "scope": "normal initial state; public Module policy/action/timer callbacks, no resource boosts or forced winner"}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, "summary.json"), raw, 0644); err != nil {
			t.Error(err)
		}
	}()
	for steps = 0; steps < 200000 && match.Result == nil; steps++ {
		if err := m.ValidateMatch(match, players); err != nil {
			t.Fatal("invalid match", steps, err)
		}
		var public struct {
			Round int    `json:"round"`
			Phase string `json:"phase"`
		}
		if err := json.Unmarshal(match.Data, &public); err != nil {
			t.Fatal(err)
		}
		rounds = public.Round
		if rounds > 1200 {
			t.Fatalf("bots failed to unify %s after %d rounds", mapID, rounds)
		}
		if match.SystemDeadline > 0 {
			ctx.Actor = -1
			ctx.Now = time.UnixMilli(match.SystemDeadline)
			before, _ := json.Marshal(match)
			match, err = m.OnTimer(ctx, match)
			if err != nil {
				t.Fatal("timer", steps, err)
			}
			if bytes.Equal(before, func() []byte { b, _ := json.Marshal(match); return b }()) {
				t.Fatal("timer made no progress", steps)
			}
			if err := encoder.Encode(botReplayRecord{Kind: "timer", Context: ctx}); err != nil {
				t.Fatal(err)
			}
		} else {
			acted := false
			for _, actor := range match.Actors {
				ctx.Actor = actor
				snapshot, _ := json.Marshal(match)
				action, e := m.BotAction(ctx, match)
				if e != nil {
					t.Fatal("policy", steps, actor, e)
				}
				unchanged, _ := json.Marshal(match)
				if !bytes.Equal(snapshot, unchanged) {
					t.Fatal("policy mutated match", steps)
				}
				if len(action) == 0 {
					continue
				}
				var named struct {
					Type string `json:"type"`
				}
				_ = json.Unmarshal(action, &named)
				if named.Type == "resign" {
					t.Fatal("bot cannot fake completion by resigning")
				}
				counts[named.Type]++
				next, e := m.Apply(ctx, match, action)
				if e != nil {
					t.Fatalf("policy proposed illegal action at step %d seat %d: %s: %v", steps, actor, action, e)
				}
				match = next
				acted = true
				if err := encoder.Encode(botReplayRecord{Kind: "action", Context: ctx, Action: action}); err != nil {
					t.Fatal(err)
				}
				break
			}
			if !acted {
				t.Fatalf("bots stalled without pending system event at round %d", rounds)
			}
		}
		ctx.Version++
		ctx.RequestedVersion = ctx.Version
	}
	if match.Result == nil {
		t.Fatalf("no natural completion in %d steps", steps)
	}
	if len(match.Result.Winners) != 1 {
		t.Fatalf("free-for-all must have one winner: %+v", match.Result)
	}
	var final struct {
		Players []struct {
			Eliminated bool `json:"eliminated"`
		} `json:"players"`
		Provinces []struct {
			Owner int `json:"owner"`
		} `json:"provinces"`
	}
	if err := json.Unmarshal(match.Data, &final); err != nil {
		t.Fatal(err)
	}
	if len(final.Provinces) == 0 {
		t.Fatal("missing provinces")
	}
	for seat, p := range final.Players {
		if seat != match.Result.Winners[0] && !p.Eliminated {
			t.Fatal("declared winner while a rival survives")
		}
	}
	for _, p := range final.Provinces {
		if p.Owner >= 0 && p.Owner != match.Result.Winners[0] {
			t.Fatal("rival still owns land after final victory")
		}
	}
}

func TestConquestBotHiddenPlansE2E(t *testing.T) {
	m := Module()
	opts, err := m.Configure(2, json.RawMessage(`{"mapId":"europe","difficulty":"normal","turnSeconds":120}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx := gameapi.Context{Actor: 0, Players: 2, Version: 1, RequestedVersion: 1, Now: time.Unix(1792700020, 0)}
	match, err := m.Start(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	before, err := m.BotAction(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor = 1
	rival, err := m.BotAction(ctx, match)
	if err != nil || len(rival) == 0 {
		t.Fatal("rival policy", err)
	}
	match, err = m.Apply(ctx, match, rival)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor = 0
	after, err := m.BotAction(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("private rival plan changed decision:\n%s\n%s", before, after)
	}
}

// Each result binds the exact rule/policy/test files and both immutable maps.
// The independent run directory lets final acceptance preserve earlier results.
func botSourceDigests(t *testing.T, root string) map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "games", "conquest", "server", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	mapSources, err := filepath.Glob(filepath.Join(root, "games", "conquest", "maps", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, mapSources...)
	paths = append(paths, filepath.Join(root, "games", "conquest", "maps", "world.json"), filepath.Join(root, "games", "conquest", "maps", "europe.json"))
	result := map[string]string{}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		result[filepath.ToSlash(relative)] = fmt.Sprintf("%x", sha256.Sum256(body))
	}
	return result
}
