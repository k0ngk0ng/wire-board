// Command verify-chu-session runs fresh independent matches without a ROM,
// emulator, reference snapshot or expected intermediate states.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type verification struct {
	Pass                 bool   `json:"pass"`
	Completed            bool   `json:"completed"`
	ReferenceStateLoaded bool   `json:"reference_state_loaded"`
	Environment          string `json:"environment"`
	Seed                 uint32 `json:"seed"`
	Players              int    `json:"players"`
	Team                 bool   `json:"team"`
	HumanIdle            bool   `json:"human_idle"`
	Characters           []int  `json:"characters,omitempty"`
	Ticks                int    `json:"ticks"`
	Callbacks            int    `json:"callbacks"`
	FirstDamage          int    `json:"first_damage_tick"`
	FirstKO              int    `json:"first_ko_tick"`
	Retirement           int    `json:"retirement_tick"`
	Result               int    `json:"result_tick"`
	Clear                int    `json:"clear_tick"`
	Winner               int16  `json:"winner_code"`
	AutoTakeovers        int    `json:"human_auto_takeovers"`
	RulesSHA             string `json:"rules_sha256"`
	FinalSHA             string `json:"final_state_sha256"`
	TraceSHA             string `json:"trace_sha256"`
}

func digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func run() error {
	players := flag.Int("players", 2, "number of active original slots,2..4")
	selection := flag.String("characters", "", "comma-separated source IDs,0..10; defaults to all9")
	team := flag.Bool("team", false, "use original four-player1/1/4/4 teams")
	human := flag.Bool("human-idle", false, "keep slot0 explicitly human with neutral input")
	seed := flag.Uint64("seed", 0x12345678, "shared32-bit initial rule seed")
	max := flag.Int("max-ticks", 20000, "maximum complete logical ticks")
	path := flag.String("rules", "games/yuyuhakusho/data/chu-world-rules.json.gz", "semantic rules only")
	flag.Parse()
	if *players < 2 || *players > 4 || *seed > 0xffffffff || *max < 1 {
		return fmt.Errorf("invalid players/seed/tick limit")
	}
	characters := make([]uint8, *players)
	for i := range characters {
		characters[i] = 9
	}
	if *selection != "" {
		parts := strings.Split(*selection, ",")
		if len(parts) != *players {
			return fmt.Errorf("one character ID is required for each player")
		}
		for i, part := range parts {
			id, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
			if err != nil || id > 10 {
				return fmt.Errorf("unsupported character ID %q", part)
			}
			characters[i] = uint8(id)
		}
	}
	file, err := os.Open(*path)
	if err != nil {
		return err
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		return err
	}
	library, err := engine.DecodeChuWorldRules(raw)
	if err != nil {
		return err
	}
	options := engine.ChuSessionOptions{Seed: uint32(*seed), Difficulty: 6, TeamBattle: *team}
	slots := []uint8{0, 3}
	if *players == 3 {
		slots = []uint8{0, 1, 3}
	} else if *players == 4 {
		slots = []uint8{0, 1, 2, 3}
	}
	for i, slot := range slots {
		options.Players = append(options.Players, engine.ChuSessionPlayer{Slot: slot, CharacterID: characters[i], Color: uint8(i), Computer: !(*human && slot == 0)})
	}
	state, _, err := engine.NewChuSession(options, library)
	if err != nil {
		return err
	}
	state, err = engine.CompleteChuSessionResource(state, engine.DefaultChuSessionResource())
	if err != nil {
		return err
	}
	report := verification{Seed: uint32(*seed), Players: *players, Team: *team, HumanIdle: *human, Environment: runtime.GOOS + "/" + runtime.GOARCH, RulesSHA: digest(raw)}
	if *selection != "" {
		for _, character := range characters {
			report.Characters = append(report.Characters, int(character))
		}
	}
	trace := sha256.New()
	for tick := 1; tick <= *max && !state.MatchComplete; tick++ {
		var events []engine.ChuWorldEvent
		state, events, _, err = engine.StepChuSessionFrame(state, [8]engine.Buttons{}, library)
		if err != nil {
			return fmt.Errorf("logical tick%d: %w", tick, err)
		}
		report.Ticks = tick
		for _, event := range events {
			if event.Kind == "callback" {
				report.Callbacks++
			}
		}
		for _, slot := range slots {
			actor := state.Objects[0x8400+uint16(slot)*0x400]
			if actor.Header.Behavior != 0x12668 || actor.Header.Stage != 2 {
				continue
			}
			if actor.Body.Health < 223 && report.FirstDamage == 0 {
				report.FirstDamage = tick
			}
			if actor.Body.Health < 0 && report.FirstKO == 0 {
				report.FirstKO = tick
			}
			if actor.Body.Health == -1024 && report.Retirement == 0 {
				report.Retirement = tick
			}
			if *human && slot == 0 && actor.InputChannel == 255 {
				report.AutoTakeovers++
			}
		}
		if state.Match.WinnerCode != 0 && report.Result == 0 {
			report.Result = tick
		}
		if state.MatchComplete {
			report.Clear = tick
		}
		encoded, marshalErr := json.Marshal(struct {
			Tick   int
			State  engine.ChuWorldState
			Events []engine.ChuWorldEvent
		}{tick, state, events})
		if marshalErr != nil {
			return marshalErr
		}
		trace.Write(encoded)
	}
	report.Completed, report.Winner = state.MatchComplete, state.Match.WinnerCode
	final, err := json.Marshal(state)
	if err != nil {
		return err
	}
	report.FinalSHA, report.TraceSHA = digest(final), hex.EncodeToString(trace.Sum(nil))
	report.Pass = report.Completed && report.FirstDamage > 0 && report.FirstKO >= report.FirstDamage && report.Retirement >= report.FirstKO && report.Result >= report.Retirement && report.Clear >= report.Result && report.AutoTakeovers == 0
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if !report.Pass {
		return fmt.Errorf("fresh legal-input match did not complete verified lifecycle within%d ticks", *max)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
