package snooker_test

// Full natural matches using the actual HTTP API and actual system deadlines.
// Explicit opt-in keeps this real-time acceptance out of duplicate rule shards;
// main's verified-build job runs it once and uploads the trace artifacts.
import (
	"encoding/json"
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"os"
	"testing"
	"time"
)

func publicState(t *testing.T, room map[string]any) game.State {
	t.Helper()
	raw, _ := json.Marshal(gametest.Data(room))
	var s game.State
	if e := json.Unmarshal(raw, &s); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestSnookerCompleteNaturalMatchesHTTP(t *testing.T) {
	if os.Getenv("WIRE_BOARD_SNOOKER_HTTP_FULL") != "1" {
		t.Skip("main verified build / explicit real-time acceptance")
	}
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, game.Module(), "snooker")
			h.AssertUnpublished()
			options := map[string]any{"bestOf": 1}
			if n == 4 {
				options["teams"] = []int{0, 0, 1, 1}
			}
			h.Start(options)
			records := []any{map[string]any{"scope": "natural rack, actual HTTP identities, real system deadlines (not accelerated)", "initial": gametest.Data(h.People[0].Room())}}
			shots := 0
			for steps := 0; steps < 850; steps++ {
				room := h.People[0].Room()
				s := publicState(t, room)
				if room["status"] == "finished" {
					records = append(records, map[string]any{"final": gametest.Data(room), "shots": shots})
					break
				}
				actor := h.People[s.Turn]
				submit := func(a map[string]any) { a["window"] = s.Window; actor.Action(a, 200) }
				switch s.Phase {
				case "start_choice":
					submit(map[string]any{"type": "break_choice", "team": 0})
				case "aim":
					cueDown := false
					for _, b := range s.Balls {
						if b.ID == 0 && b.Down {
							cueDown = true
						}
					}
					if cueDown {
						placed := false
						for _, p := range []game.Vec{{X: game.BaulkX - .15, Y: game.TableHeight / 2}, {X: game.BaulkX - .08, Y: game.TableHeight/2 - .15}, {X: game.BaulkX - .08, Y: game.TableHeight/2 + .15}, {X: game.BaulkX - .25, Y: game.TableHeight / 2}} {
							valid := true
							for _, b := range s.Balls {
								if b.ID != 0 && !b.Down && distance(b.Pos, p) <= 2*game.BallRadius+.00001 {
									valid = false
								}
							}
							if valid {
								submit(map[string]any{"type": "place", "x": p.X, "y": p.Y})
								placed = true
								break
							}
						}
						if !placed {
							t.Fatal("no usable D position")
						}
						s = publicState(t, actor.Room())
					}
					shot := plannedShot(t, s)
					submit(map[string]any{"type": "shoot", "angle": shot.Angle, "power": shot.Power, "nominate": shot.Nominate})
					if shots == 20 {
						h.Restart()
					}
					deadline := time.Now().Add(15 * time.Second)
					var after map[string]any
					for time.Now().Before(deadline) {
						after = actor.Room()
						if gametest.Data(after)["phase"] != "rolling" {
							break
						}
						time.Sleep(100 * time.Millisecond)
					}
					if gametest.Data(after)["phase"] == "rolling" {
						t.Fatal("real system deadline did not settle")
					}
					d := gametest.Data(after)
					records = append(records, map[string]any{"beforeBalls": s.Balls, "turn": s.Turn, "shot": shot, "scores": d["scores"], "phase": d["phase"], "flight": d["flight"].(map[string]any)["id"]})
					shots++
					if shots%10 == 0 {
						t.Logf("%d players: %d strokes, scores %v", n, shots, d["scores"])
						h.Record(fmt.Sprintf("natural real-time HTTP stroke %d", shots))
					}
				case "foul":
					submit(map[string]any{"type": "foul_choice", "choice": "accept"})
				case "black_choice":
					submit(map[string]any{"type": "black_choice", "team": 0})
				case "black_player":
					submit(map[string]any{"type": "black_player", "seat": s.Turn})
				default:
					t.Fatal("unexpected HTTP match phase", s.Phase)
				}
			}
			final := h.People[0].Room()
			s := publicState(t, final)
			if final["status"] != "finished" || len(s.Results) != 1 || s.Results[0].Reason != "black" {
				t.Fatal("natural HTTP match did not finish on black", s.Phase, s.Results)
			}
			for _, b := range s.Balls {
				if b.ID > 0 && b.ID < 21 && !b.Down {
					t.Fatal("finished before clearing objects")
				}
			}
			winners := final["game"].(map[string]any)["winners"].([]any)
			for _, p := range winners {
				if s.Teams[int(p.(float64))] != s.Results[0].Winner {
					t.Fatal("wrong winner team")
				}
			}
			h.Record("complete natural HTTP match, actual clocks, all object balls cleared")
			trace(t, records)
			h.Restart()
			restored := publicState(t, h.People[0].Room())
			if restored.Scores != s.Scores || restored.FramesWon != s.FramesWon {
				t.Fatal("finished HTTP match changed after restart")
			}
		})
	}
}
