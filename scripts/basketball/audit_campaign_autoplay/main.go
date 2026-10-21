// Exploratory public-module campaign audit with virtual host time. It never
// edits game state, seeds, scores, clocks, roster or equipment, and is not a
// real-time HTTP or final fourteen-win campaign acceptance.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func main() {
	output := flag.String("output", ".local/basketball/campaign-autoplay-audit", "workspace output directory")
	initial := flag.String("initial", "", "replay retained initial public module Match, without reseeding")
	resume := flag.String("resume", "", "continue an actual retained Match and host time, counting only newly completed matches")
	matches := flag.Int("matches", 20, "maximum actual completed matches before bounded stop")
	flag.Parse()
	if *initial != "" && *resume != "" {
		panic("choose either initial replay or continuation")
	}
	if *matches < 1 || *matches > 100 {
		panic("matches must be1..100")
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		panic(err)
	}
	write := func(name string, v any) {
		b, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(*output, name), append(b, '\n'), 0600); e != nil {
			panic(e)
		}
	}
	f, e := os.Create(filepath.Join(*output, "events.jsonl"))
	if e != nil {
		panic(e)
	}
	defer f.Close()
	events := json.NewEncoder(f)
	module := game.Module()
	ctx := gameapi.Context{Players: 1, Actor: 0, Automated: []bool{true}, Now: time.UnixMilli(1000000)}
	var match gameapi.Match
	source := *initial
	if *resume != "" {
		source = *resume
	}
	var originSHA string
	if source != "" {
		raw, e := os.ReadFile(source)
		if e != nil {
			panic(e)
		}
		if e = json.Unmarshal(raw, &match); e != nil {
			panic(e)
		}
		originSHA = fmt.Sprintf("%x", sha256.Sum256(raw))
		// This is the exact retained envelope, not a modified seed/runtime.
		if e = match.Validate(1); e != nil {
			panic(e)
		}
	} else {
		options, e := module.Configure(1, json.RawMessage(`{"mode":"solo","campaign":{"opponent":1}}`))
		if e != nil {
			panic(e)
		}
		match, e = module.Start(ctx, options)
		if e != nil {
			panic(e)
		}
	}
	write("initial.json", match)
	var originState game.State
	if e = json.Unmarshal(match.Data, &originState); e != nil {
		panic(e)
	}
	if source != "" {
		ctx.Now = time.UnixMilli(originState.ServerAt)
	}
	executable, e := os.Executable()
	if e != nil {
		panic(e)
	}
	binary, e := os.ReadFile(executable)
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(filepath.Join(*output, "audit-driver"), binary, 0700); e != nil {
		panic(e)
	}
	write("origin.json", map[string]any{"source": source, "sourceSHA256": originSHA, "continuation": *resume != "", "serverAt": originState.ServerAt, "round": originState.Campaign.Round, "video": originState.Runtime.VideoFrames, "driverSHA256": fmt.Sprintf("%x", sha256.Sum256(binary)), "scope": "preserved Match plus original host timestamp; no seed or world field edits"})
	began := time.Now()
	lastStage := ""
	rounds := 0
	wins, losses := 0, 0
	lastSettled := uint64(0)
	if *resume != "" && originState.Runtime.Outcome != nil && originState.Runtime.Outcome.Confirmed {
		lastSettled = originState.Campaign.Round
	}
	ctx.Actor = -1
	var state game.State
	for tick := 0; tick < *matches*9000; tick++ {
		// Optional fields disappear at a new round; decode into a fresh target
		// so the observer cannot retain the preceding Outcome pointer.
		state = game.State{}
		if e = json.Unmarshal(match.Data, &state); e != nil {
			panic(e)
		}
		if state.Campaign == nil {
			panic("campaign state absent")
		}
		key := fmt.Sprintf("%d/%s", state.Campaign.Round, state.Campaign.Stage)
		if key != lastStage {
			progress := state.Runtime.Config.Campaign
			row := map[string]any{"hostMillis": ctx.Now.UnixMilli(), "round": state.Campaign.Round, "stage": state.Campaign.Stage, "video": state.Runtime.VideoFrames, "period": progress.Period, "wins": progress.WinBits, "seed": state.Runtime.Config.Match.Seed, "court": state.Runtime.Config.Match.Court, "teams": state.Runtime.Config.Match.Teams, "characters": state.Runtime.Config.Match.Characters, "outcome": state.Runtime.Outcome}
			if e = events.Encode(row); e != nil {
				panic(e)
			}
			fmt.Printf("round%d stage%s video%d period%d wins%v court%d\n", state.Campaign.Round, state.Campaign.Stage, state.Runtime.VideoFrames, progress.Period, progress.WinBits, state.Runtime.Config.Match.Court)
			lastStage = key
		}
		if state.Runtime.Outcome != nil && state.Runtime.Outcome.Confirmed && state.Campaign.Round != lastSettled {
			lastSettled = state.Campaign.Round
			rounds++
			if state.Runtime.Outcome.WinningTeam == 0 {
				wins++
			} else {
				losses++
			}
		}
		if match.Result != nil || state.Campaign.Stage == "complete" || rounds >= *matches {
			status := "bounded-stop"
			if state.Campaign.Stage == "complete" {
				status = "campaign-complete"
			}
			write("final.json", match)
			write("result.json", map[string]any{"status": status, "scope": "exploratory public module, virtual100ms host ticks; existing automatic menu policy; no forced outcomes/reseeding; postresult lifecycle under active integration, not final acceptance", "matches": rounds, "wins": wins, "losses": losses, "elapsedSeconds": time.Since(began).Seconds(), "hostMillis": ctx.Now.UnixMilli(), "stage": state.Campaign.Stage})
			return
		}
		ctx.Now = ctx.Now.Add(100 * time.Millisecond)
		next, err := module.OnTimer(ctx, match)
		if err != nil {
			write("first-failure-match.json", match)
			failure := map[string]any{"error": err.Error(), "context": ctx, "round": state.Campaign.Round, "video": state.Runtime.VideoFrames}
			var runtimeErr *game.RuntimeAdvanceError
			if errors.As(err, &runtimeErr) {
				write("first-failure-runtime.json", runtimeErr.Before)
				failure["held"] = runtimeErr.Held
			}
			write("result.json", map[string]any{"status": "failed", "failure": failure, "matches": rounds, "wins": wins, "losses": losses, "scope": "exploratory public module with virtual100ms host ticks; no failure retries", "elapsedSeconds": time.Since(began).Seconds()})
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		match = next
	}
	write("final.json", match)
	write("result.json", map[string]any{"status": "tick-bound", "matches": rounds, "wins": wins, "losses": losses, "scope": "exploratory public module; bounded audit did not complete campaign"})
}
