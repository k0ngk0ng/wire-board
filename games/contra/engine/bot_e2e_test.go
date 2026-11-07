package contra

// Standard campaign acceptance is declared before strategy code: only NewSession
// and legal held pads progress a standard match. The explicit user-authorized
// infinite-reserve test mode records every inventory grant and never counts as
// standard-lives acceptance. No ROM snapshots, stage skips, invulnerability or
// outcome injection. Attempts retain pads, policy state and outcome records.
import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func botStoredSessionsEqual(a, b *Session) (bool, error) {
	left, err := json.Marshal(a)
	if err != nil {
		return false, err
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(left, right), nil
}

func botCampaignSources(t *testing.T) map[string]string {
	t.Helper()
	names := []string{"testdata/bot-visual-catalog.json.gz", "bot_e2e_test.go", "bot_campaign_config_test.go", "bot_test_resources_test.go", "bot_test_lives_test.go"}
	for _, pattern := range []string{"*.go", "*.json", "leveldata/*.json"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range matches {
			if !strings.HasSuffix(name, "_test.go") {
				names = append(names, name)
			}
		}
	}
	out := make(map[string]string, len(names))
	for _, name := range names {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	return out
}

func TestBotStandardCampaignE2E(t *testing.T) {
	config, err := readBotCampaignConfig(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Enabled {
		t.Skip("long campaign is opt-in: set CONTRA_BOT_MODE=standard-terminal, assisted-clear or diagnostic")
	}
	sourcesBefore := botCampaignSources(t)
	infiniteLives, seats, frames, target := config.InfiniteLives, config.Seats, config.Frames, config.Target
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	folder := config.Evidence
	if folder == "" {
		folder = filepath.Join(root, ".local/contra-bot/runs")
		if e = os.MkdirAll(folder, 0755); e != nil {
			t.Fatal(e)
		}
		folder, e = os.MkdirTemp(folder, "attempt-")
		if e != nil {
			t.Fatal(e)
		}
	}
	rel, e := filepath.Rel(root, folder)
	if e != nil || rel == ".." || len(rel) >= 3 && rel[:3] == "../" {
		t.Fatal("bot evidence outside workspace")
	}
	var resources [8]*LevelResources
	for i := range resources {
		resources[i] = levelResources(t, fmt.Sprintf("stage-%d.json", i+1))
	}
	catalog := botBuiltinTestCatalog(t)
	bundle, e := BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := NewBuiltinBotVision(catalog, bundle)
	if e != nil {
		t.Fatal(e)
	}
	var prefix []byte
	if path := config.Prefix; path != "" {
		file, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		z, e := gzip.NewReader(file)
		if e != nil {
			file.Close()
			t.Fatal(e)
		}
		prefix, e = io.ReadAll(z)
		z.Close()
		file.Close()
		if e != nil || len(prefix)%2 != 0 {
			t.Fatal("invalid legal-pad prefix", e)
		}
	}
	for _, players := range seats {
		t.Run(fmt.Sprintf("players%d", players), func(t *testing.T) {
			out := filepath.Join(folder, fmt.Sprintf("players%d", players))
			if e = os.MkdirAll(out, 0755); e != nil {
				t.Fatal(e)
			}
			padFile, e := os.OpenFile(filepath.Join(out, "pads.bin.gz"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if e != nil {
				t.Fatal(e)
			}
			padStream := gzip.NewWriter(padFile)
			traceFile, e := os.Create(filepath.Join(out, "trace.jsonl.gz"))
			if e != nil {
				t.Fatal(e)
			}
			traceStream := gzip.NewWriter(traceFile)
			trace := json.NewEncoder(traceStream)
			s, e := NewSession(42, players, "bot-standard")
			if e != nil {
				t.Fatal(e)
			}
			var lifeRefills []botTestLifeRefill
			if infiniteLives {
				lifeRefills = replenishBotTestReserves(s, players)
			}
			bots := [2]Bot{}
			finished := false
			failure := ""
			lastPhase := uint8(255)
			lastLevel := uint8(255)
			lastLives := [2]uint8{255, 255}
			restores := 0
			for n := 1; n <= frames; n++ {
				var pads [2]Buttons
				// Only the submitted public picture and HUD reach the classifier.
				// Raw state below is diagnostic output, never a strategy input.
				hud := BotPublicHUD{Seats: players, Paused: BotPublicBool{Known: true, Value: s.World.Campaign.Paused}}
				for actor := 0; actor < players; actor++ {
					hud.Players[actor] = BotPublicPlayer{Occupied: true, Lives: BotPublicNumber{Known: true, Value: s.PresentedLives[actor]}}
				}
				visual, err := NewBotVisualFrame(s.World.Active.Presentation.Snapshot(), hud, catalog, 0, uint64(n))
				if err != nil {
					failure = err.Error()
					break
				}
				observed, err := ObserveBotFrame(visual, vision)
				if err != nil {
					failure = err.Error()
					break
				}
				var checkpoint struct {
					Session *Session
					Bots    [2]Bot
				}
				if n%997 == 0 {
					raw, err := json.Marshal(struct {
						Session *Session
						Bots    [2]Bot
					}{s, bots})
					if err == nil {
						err = json.Unmarshal(raw, &checkpoint)
					}
					if err != nil {
						failure = err.Error()
						break
					}
					equal, err := botStoredSessionsEqual(s, checkpoint.Session)
					if err != nil {
						failure = err.Error()
						break
					}
					if !equal || !reflect.DeepEqual(bots, checkpoint.Bots) {
						failure = "saved session or policy memory differs before decision"
						break
					}
				}
				if n <= len(prefix)/2 {
					pads = [2]Buttons{Buttons(prefix[(n-1)*2]), Buttons(prefix[(n-1)*2+1])}
				} else {
					for actor := 0; actor < players; actor++ {
						obs := observed
						obs.Actor = actor
						obs.Enemies = append([]BotEnemy(nil), observed.Enemies...)
						obs.PlayerCandidates = append([]BotPlayerCandidate(nil), observed.PlayerCandidates...)
						pads[actor] = bots[actor].Decide(obs)
						if checkpoint.Session != nil {
							obs.Enemies = append([]BotEnemy(nil), observed.Enemies...)
							obs.PlayerCandidates = append([]BotPlayerCandidate(nil), observed.PlayerCandidates...)
							if checkpoint.Bots[actor].Decide(obs) != pads[actor] {
								failure = "restored policy chose different input"
								break
							}
						}
					}
				}
				if failure != "" {
					break
				}
				restored := checkpoint.Session
				if restored != nil && !reflect.DeepEqual(bots, checkpoint.Bots) {
					failure = "restored policy memory diverged after decision"
					break
				}
				level := s.World.Active.Combat.World.Level
				_, e = s.StepPads(uint64(n), pads, resources[min(7, int(level))])
				if e != nil {
					failure = e.Error()
					break
				}
				var frameRefills []botTestLifeRefill
				if infiniteLives {
					frameRefills = replenishBotTestReserves(s, players)
					lifeRefills = append(lifeRefills, frameRefills...)
				}
				if _, err := padStream.Write([]byte{byte(pads[0]), byte(pads[1])}); err != nil {
					t.Fatal(err)
				}
				if restored != nil {
					_, e = restored.StepPads(uint64(n), pads, resources[min(7, int(level))])
					if e == nil && infiniteLives {
						replenishBotTestReserves(restored, players)
					}
					equal := false
					if e == nil {
						equal, e = botStoredSessionsEqual(s, restored)
					}
					if e != nil || !equal {
						failure = "restored session diverged"
						break
					}
					restores++
				}
				world := s.World.Active.Combat.World
				lives := [2]uint8{s.World.Active.Combat.Players[0].Lives, s.World.Active.Combat.Players[1].Lives}
				if n%6 == 0 || world.LevelRoutine != lastPhase || world.Level != lastLevel || lives != lastLives || len(frameRefills) != 0 {
					// Geometry only; dense terrain is omitted from each checkpoint to keep
					// the complete input replay compact. The final checkpoint retains World.
					if err := trace.Encode(map[string]any{"frame": n, "observation_tick": observed.Tick, "observation_precedes_step": true, "level": world.Level, "screen": world.Screen, "scroll": world.ScreenScroll, "routine": world.LevelRoutine, "lives": lives, "pads": pads, "players": observed.Players, "enemies": observed.Enemies, "continues": s.World.Campaign.Continues}); err != nil {
						t.Fatal(err)
					}
				}
				lastPhase, lastLevel, lastLives = world.LevelRoutine, world.Level, lives
				if n%600 == 0 {
					if err := padStream.Flush(); err != nil {
						t.Fatal(err)
					}
					if err := traceStream.Flush(); err != nil {
						t.Fatal(err)
					}
					progress, err := json.Marshal(map[string]any{"frame": n, "level": world.Level, "screen": world.Screen, "routine": world.LevelRoutine, "infinite_lives": infiniteLives, "reserve_refills": len(lifeRefills)})
					if err != nil {
						t.Fatal(err)
					}
					temporary := filepath.Join(out, "progress.tmp")
					if err = os.WriteFile(temporary, progress, 0644); err != nil {
						t.Fatal(err)
					}
					if err = os.Rename(temporary, filepath.Join(out, "progress.json")); err != nil {
						t.Fatal(err)
					}
					t.Logf("progress frame=%d level=%d screen=%d reserve_refills=%d", n, world.Level, world.Screen, len(lifeRefills))
				}
				if s.World.Campaign.Outcome != CampaignOngoing {
					if _, err := s.FlushTerminal(); err != nil {
						failure = err.Error()
						break
					}
					finished = config.TerminalComplete(s.World.Campaign.Outcome, s.Finalized)
					break
				}
				if config.Mode == "diagnostic" && target < 8 && int(world.Level) >= target {
					finished = true
					break
				}
			}
			for _, stream := range []io.Closer{padStream, padFile, traceStream, traceFile} {
				if err := stream.Close(); err != nil {
					t.Fatal(err)
				}
			}
			final, err := json.MarshalIndent(s, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(out, "final-session.json"), append(final, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			finalBots, err := json.MarshalIndent(bots, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(out, "final-bots.json"), append(finalBots, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			sourcesAfter := botCampaignSources(t)
			sourceChanged := !reflect.DeepEqual(sourcesBefore, sourcesAfter)
			if sourceChanged {
				finished = false
				failure = "sources changed during campaign acceptance"
			}
			report := map[string]any{"passed": finished, "complete_campaign": finished && config.Mode == "standard-terminal" && s.Finalized && s.World.Campaign.Outcome == CampaignCleared, "target_level": target, "seed": 42, "players": players, "frames": s.Random.Frames, "level": s.World.Active.Combat.World.Level, "screen": s.World.Active.Combat.World.Screen, "routine": s.World.Active.Combat.World.LevelRoutine, "outcome": s.World.Campaign.Outcome, "continues": s.World.Campaign.Continues, "restore_checkpoints": restores, "error": failure, "standard_reserve_lives": 2, "standard_continues": 3, "no_state_injection": true}
			report["mode"], report["acceptance"] = config.Mode, config.Acceptance()
			report["requested_players"], report["frame_budget"] = config.Seats, config.Frames
			report["standard_terminal_complete"] = finished && config.Mode == "standard-terminal" && s.Finalized
			report["policy_start_frame"] = len(prefix)/2 + 1
			if config.Mode == "standard-terminal" {
				report["scope"] = "standard reserves and continues; fresh legal-policy run to natural cleared or failed terminal; defeat is not eight-stage victory"
			}
			if config.Mode == "diagnostic" {
				report["scope"] = "explicit diagnostic only; inspect prefix, target and reserve grants; not CI campaign acceptance"
			}
			report["source_sha256"] = sourcesBefore
			report["source_changed_during_run"] = sourceChanged
			if sourceChanged {
				report["source_after_sha256"] = sourcesAfter
			}
			report["prefix_frames"] = len(prefix) / 2
			report["finalized"] = s.Finalized
			report["infinite_lives"] = infiniteLives
			if infiniteLives {
				report["complete_campaign"] = false
				report["assisted_campaign_cleared"] = finished && target == 8 && s.Finalized && s.World.Campaign.Outcome == CampaignCleared
				report["assisted_target_reached"] = finished
				report["no_state_injection"] = false
				report["original_reserve_lives"] = 2
				delete(report, "standard_reserve_lives")
				report["test_reserve_target"] = 10
				report["refill_after_visible_loss"] = true
				report["reserve_refills"] = len(lifeRefills)
				report["scope"] = "user-authorized test-only reserve replenishment; damage/death/respawn/weapon loss remain active; inspect mode/acceptance; not standard-lives acceptance"
				raw, err := json.MarshalIndent(lifeRefills, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(filepath.Join(out, "test-life-refills.json"), append(raw, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
			}
			report["restore_scope"] = "Stored Session meaning and both policy memories before Decide; recomputed inputs, post-decision memory and subsequent stored Session must match"
			report["observation"] = "submitted final RGBA and actual public HUD only; immutable global visual knowledge; shared classification"
			if len(prefix) != 0 {
				report["complete_campaign"] = false
				report["scope"] = "recorded legal-pad prefix; inspect infinite_lives for additional reserve assistance; not full-policy acceptance"
			}
			encoded, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(out, "result.json"), append(encoded, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			t.Logf("bot evidence %s: %s", out, encoded)
			if !finished {
				t.Fatalf("bot challenge incomplete (infinite_lives=%v): level%d screen%d phase%d outcome%q error%q", infiniteLives, s.World.Active.Combat.World.Level, s.World.Active.Combat.World.Screen, s.World.Active.Combat.World.LevelRoutine, s.World.Campaign.Outcome, failure)
			}
		})
	}
}
