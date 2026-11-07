package contra

// Failure scenarios precede implementation: a wipe immediately fails despite a
// continue; a continue is consumed twice; advance resurrects an out teammate or
// loses weapons/score; continue resets high score or level; boss/all-out order
// changes; low-byte FF sentinel or original delay carry is simplified; Start and
// Select priority changes; pause runs gameplay; restored duplicate ticks/terminal
// snapshots emit reset/points again; end sequence is replaced by a boss boolean.

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type campaignObserved struct {
	GameRoutine, LevelRoutine, EndSequence, Level, Loops, Continues, Boss                                                 uint8
	DelayLow, DelayHigh, GameOverDelay, Paused, Mode, PauseBlocked, Pressed, Choice, AliveAtExit, ActiveFlags, CheatLives uint8
	HighScore                                                                                                             uint16
	Score, NextLife                                                                                                       [2]uint16
	Players                                                                                                               [2]CampaignPlayer
}

func (o campaignObserved) state() (Campaign, OutdoorWorld, CampaignInput) {
	return Campaign{Continues: o.Continues, DelayLow: o.DelayLow, DelayHigh: o.DelayHigh, GameOverDelay: o.GameOverDelay, Choice: o.Choice, EndSequence: o.EndSequence, Loops: o.Loops, AliveAtExit: o.AliveAtExit, Paused: o.Paused != 0}, OutdoorWorld{GameRoutine: o.GameRoutine, LevelRoutine: o.LevelRoutine, Level: o.Level, BossDefeated: o.Boss, PlayerMode: o.Mode}, CampaignInput{Pressed: Buttons(o.Pressed), PauseBlocked: o.PauseBlocked != 0, CheatLives: o.CheatLives != 0, Players: o.Players}
}

func (o *campaignObserved) apply(c Campaign, w OutdoorWorld, p CampaignPlan) {
	o.GameRoutine, o.LevelRoutine, o.Level, o.Boss = w.GameRoutine, w.LevelRoutine, w.Level, w.BossDefeated
	o.EndSequence, o.Loops, o.Continues = c.EndSequence, c.Loops, c.Continues
	o.DelayLow, o.DelayHigh, o.GameOverDelay = c.DelayLow, c.DelayHigh, c.GameOverDelay
	o.Choice, o.AliveAtExit = c.Choice, c.AliveAtExit
	o.Paused = 0
	if c.Paused {
		o.Paused = 1
	}
	for _, command := range p.Commands {
		if command.Reset == nil {
			continue
		}
		r := command.Reset
		o.Players = r.Players
		if r.ClearArcadeScores {
			o.Score = [2]uint16{}
			o.NextLife = r.NextLife
			o.ActiveFlags = [2]uint8{1, 7}[r.PlayerMode]
		}
	}
}

func TestCampaignOriginalDecisionBoundaries(t *testing.T) {
	f, err := os.Open("../../../docs/research/nes-contra/evidence/campaign-boundaries-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Records []struct {
			Kind              string
			Case, Frame, Exit int
			Before, After     campaignObserved
			Sounds            []uint8
			Effects           []string
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, row := range oracle.Records {
		c, w, input := row.Before.state()
		var plan CampaignPlan
		switch row.Kind {
		case "beginPlaying", "transition", "continueMenu", "exhaustedMenu":
			plan, err = c.BeginTick(1, &w, input)
		case "afterCombat":
			c.Pending, c.OriginRoutine = CampaignCombat, w.LevelRoutine
			plan, err = c.AfterCombat(&w, input.Players)
		case "clearWait":
			c.Pending, c.OriginRoutine = CampaignEnemies, 8
			plan, err = c.AfterEnemies(&w, input.Players)
		case "gameOverWait":
			c.Pending, c.OriginRoutine = CampaignEnemies, 11
			plan, err = c.AfterEnemies(&w, input.Players)
		case "pause":
			plan = c.updatePause(input)
		case "delay":
			c.advanceDelay()
		case "resetCounters":
			plan = c.continueResources(&w, input)
		case "initialize":
			c.Pending = CampaignInitializeLevel
			plan, err = c.AfterPresentation(&w, true)
		case "prepareBattle":
			c.Pending = CampaignPrepareBattle
			plan, err = c.AfterPresentation(&w, true)
		default:
			t.Fatal("unhandled native campaign boundary", row.Kind)
		}
		if err != nil {
			t.Fatalf("%s case %d: %v", row.Kind, row.Case, err)
		}
		got := row.Before
		got.apply(c, w, plan)
		want := row.After
		// PPU-ready is a presentation effect outside the independent rules.
		got.PauseBlocked = want.PauseBlocked
		if got != want || !reflect.DeepEqual(append([]uint8{}, plan.Sounds...), append([]uint8{}, row.Sounds...)) {
			t.Fatalf("%s case %d frame %d\nbefore=%+v\ngot=%+v\nwant=%+v\nsounds %v want %v", row.Kind, row.Case, row.Frame, row.Before, got, want, plan.Sounds, row.Sounds)
		}
		if row.Kind == "beginPlaying" && (plan.Work == CampaignCombat) != (row.Exit == 0xccdc) {
			t.Fatal("original pause/boss work ordering", row.Case, plan.Work, row.Exit)
		}
		if row.Kind == "afterCombat" && (plan.Work == CampaignEnemies) != (row.Exit == 0xcce8) {
			t.Fatal("original wipe work ordering", row.Case, plan.Work, row.Exit)
		}
		wantStops, wantResets := 0, 0
		for _, effect := range row.Effects {
			if effect == "stopAudio" {
				wantStops++
			}
			if effect == "clearFrom64" {
				wantResets++
			}
		}
		gotStops, gotResets := 0, 0
		for _, command := range plan.Commands {
			if command.Kind == "stopAudio" {
				gotStops++
			}
			if command.Reset != nil && command.Reset.ClearStageRuntime {
				gotResets++
			}
		}
		if gotStops != wantStops || gotResets != wantResets {
			t.Fatalf("%s case %d effects stop/reset=%d/%d want=%d/%d", row.Kind, row.Case, gotStops, gotResets, wantStops, wantResets)
		}
		counts[row.Kind]++
	}
	if len(oracle.Records) != 199 || counts["transition"] != 20 || counts["continueMenu"] != 16 || counts["clearWait"] != 36 {
		t.Fatal("campaign oracle coverage changed", counts)
	}
	t.Logf("199 original boundaries match: %v", counts)
}

func TestCampaignInvalidStateAndCheckpointAtomicity(t *testing.T) {
	c := NewCampaign()
	w := OutdoorWorld{GameRoutine: 5, LevelRoutine: 6, PlayerMode: 1}
	c.Continues = 255
	before := c
	worldBefore := w
	if _, err := c.BeginTick(1, &w, CampaignInput{Pressed: ButtonStart}); err == nil {
		t.Fatal("invalid continue offer accepted")
	}
	if c != before || w != worldBefore {
		t.Fatal("invalid input changed campaign before rollback", c, w)
	}
	c = NewCampaign()
	c.Outcome = "unknown"
	if _, err := c.BeginTick(1, &w, CampaignInput{}); err == nil {
		t.Fatal("unknown terminal value accepted")
	}
	c = NewCampaign()
	c.Outcome = CampaignCleared
	w = OutdoorWorld{GameRoutine: 7, Level: 8}
	if err := c.Validate(&w, true); err == nil {
		t.Fatal("Japanese sound-test routine accepted as a cleared campaign")
	}
	c = NewCampaign()
	c.Pending = CampaignEnemies
	c.OriginRoutine = 8
	w.Level = 8
	before = c
	worldBefore = w
	if _, err := c.AfterEnemies(&w, [2]CampaignPlayer{{State: 1}, {State: 1}}); err == nil {
		t.Fatal("invalid clear level accepted")
	}
	if c != before || w != worldBefore {
		t.Fatal("failed work changed campaign state")
	}
	c = NewCampaign()
	w = OutdoorWorld{GameRoutine: 5, LevelRoutine: 4}
	if err := c.Validate(&w, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.BeginTick(1, &w, CampaignInput{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(&w, true); err == nil {
		t.Fatal("unfinished tick can be saved")
	}
	if _, err := c.AfterCombat(&w, [2]CampaignPlayer{{State: 1}, {GameOver: 1}}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AfterEnemies(&w, [2]CampaignPlayer{{State: 1}, {GameOver: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(&w, true); err != nil {
		t.Fatal("complete tick rejected", err)
	}
}

func TestCampaignThreeContinuesAndTerminalRestore(t *testing.T) {
	c := NewCampaign()
	w := OutdoorWorld{GameRoutine: 5, LevelRoutine: 4, Level: 3, PlayerMode: 1}
	input := CampaignInput{Players: [2]CampaignPlayer{{GameOver: 1}, {GameOver: 1}}}
	serial := uint64(1)
	for failure := 0; failure < 4; failure++ {
		plan, err := c.BeginTick(serial, &w, input)
		if err != nil || plan.Work != CampaignCombat {
			t.Fatal(plan, err)
		}
		plan, err = c.AfterCombat(&w, input.Players)
		if err != nil || w.LevelRoutine != 11 || c.GameOverDelay != 96 || c.Outcome != "" {
			t.Fatal("wipe skipped original delay/offer", c, w, plan, err)
		}
		for range 96 {
			serial++
			plan, err = c.BeginTick(serial, &w, input)
			if err != nil || plan.Work != CampaignEnemies {
				t.Fatal(plan, err)
			}
			plan, err = c.AfterEnemies(&w, input.Players)
			if err != nil {
				t.Fatal(err)
			}
		}
		if w.LevelRoutine != 5 {
			t.Fatal("wrong game-over delay", w, c)
		}
		serial++
		plan, err = c.BeginTick(serial, &w, input)
		if err != nil {
			t.Fatal(err)
		}
		if failure < 3 {
			if w.LevelRoutine != 6 || c.Outcome != "" || c.RemainingContinues(w.LevelRoutine) != 3-failure {
				t.Fatal("wrong remaining continue offer", w, c)
			}
			// A completed tick survives save/restore; same-tick delivery is inert.
			raw, _ := json.Marshal(c)
			var restored Campaign
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			c = restored
			duplicate, err := c.BeginTick(serial, &w, input)
			if err != nil || !duplicate.Duplicate || len(duplicate.Commands) != 0 {
				t.Fatal("duplicate continue offer", duplicate, err)
			}
			serial++
			input.Pressed = ButtonStart
			plan, err = c.BeginTick(serial, &w, input)
			input.Pressed = 0
			if err != nil || w.Level != 3 || w.LevelRoutine != 0 || c.Outcome != "" {
				t.Fatal("continue reset campaign", w, c, err)
			}
			found := false
			for _, command := range plan.Commands {
				if command.Reset != nil && command.Reset.Kind == CampaignResetContinue {
					found = true
					if command.Reset.Players[0].Lives != 2 || command.Reset.Players[1].Lives != 2 || !command.Reset.ClearArcadeScores {
						t.Fatal(command)
					}
				}
			}
			if !found {
				t.Fatal("missing resource reset")
			}
			w.LevelRoutine = 4 // World initialization/presentation is outside this reducer test.
			serial++
		} else if c.Outcome != CampaignFailed || w.LevelRoutine != 7 {
			t.Fatal("exhausted challenge still running", w, c)
		}
	}
	before, _ := json.Marshal(c)
	plan, err := c.BeginTick(serial+1, &w, input)
	after, _ := json.Marshal(c)
	if err != nil || len(plan.Commands) != 0 || string(before) != string(after) {
		t.Fatal("terminal replay changed state", plan, err)
	}
}

func TestCampaignBossWipePriorityPauseAndClearOnce(t *testing.T) {
	c := NewCampaign()
	w := OutdoorWorld{GameRoutine: 5, LevelRoutine: 4, Level: 7, BossDefeated: 1, PlayerMode: 1}
	input := CampaignInput{Players: [2]CampaignPlayer{{GameOver: 1}, {GameOver: 1}}}
	plan, err := c.BeginTick(1, &w, input)
	if err != nil || w.LevelRoutine != 8 || plan.Work != CampaignNoWork || c.Outcome != "" {
		t.Fatal("boss prematurely settled", w, c, err)
	}
	plan, err = c.BeginTick(2, &w, input)
	if err != nil || plan.Work != CampaignCombat {
		t.Fatal(plan, err)
	}
	_, err = c.AfterCombat(&w, input.Players)
	if err != nil || w.LevelRoutine != 11 || c.Outcome != "" {
		t.Fatal("boss overrode full wipe", w, c, err)
	}
	// A real final transition is the win boundary, not a boss flag alone.
	c = NewCampaign()
	w = OutdoorWorld{GameRoutine: 5, LevelRoutine: 5, Level: 7, BossDefeated: 1, PlayerMode: 1}
	input.Players = [2]CampaignPlayer{{State: 1, Lives: 2, Weapon: 17}, {GameOver: 1, Weapon: 4}}
	plan, err = c.BeginTick(10, &w, input)
	if err != nil || c.Outcome != CampaignCleared || w.Level != 8 || w.GameRoutine != 6 || c.Loops != 1 {
		t.Fatal("final transition", w, c, err)
	}
	wins := 0
	for _, command := range plan.Commands {
		if command.Kind == "cleared" {
			wins++
		}
	}
	if wins != 1 {
		t.Fatal("missing or duplicate clear command", plan)
	}
	raw, _ := json.Marshal(c)
	var restored Campaign
	_ = json.Unmarshal(raw, &restored)
	plan, err = restored.BeginTick(11, &w, input)
	if err != nil || len(plan.Commands) != 0 || plan.Work != CampaignNoWork {
		t.Fatal("restored victory began another loop", plan, err)
	}
	c = NewCampaign()
	w = OutdoorWorld{GameRoutine: 5, LevelRoutine: 4}
	input = CampaignInput{Pressed: ButtonStart}
	plan, err = c.BeginTick(1, &w, input)
	if err != nil || !c.Paused || plan.Work != CampaignNoWork {
		t.Fatal("pause ran gameplay", c, plan, err)
	}
	plan, err = c.BeginTick(1, &w, input)
	if err != nil || !plan.Duplicate || !c.Paused {
		t.Fatal("retry toggled pause", c, plan, err)
	}
	plan, err = c.BeginTick(2, &w, input)
	if err != nil || c.Paused || plan.Work != CampaignCombat {
		t.Fatal("resume did not run same frame", c, plan, err)
	}
}
