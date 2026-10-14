package gametest_test

// Opt-in, explicitly labelled starting positions for real-browser feedback
// acceptance. These wrappers are compiled only into tests, not the application.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games"
	snooker "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
	"time"
)

func snookerBrowserModules() []gameapi.Module {
	modules := games.Modules()
	kind := os.Getenv("WIRE_BOARD_SNOOKER_CASE")
	if kind == "" {
		return modules
	}
	for i := range modules {
		if modules[i].ID != "snooker" {
			continue
		}
		start := modules[i].Start
		modules[i].Start = func(ctx gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
			m, e := start(ctx, o)
			if e != nil {
				return m, e
			}
			var s snooker.State
			_ = json.Unmarshal(m.Data, &s)
			s.Phase = "aim"
			s.Turn = 0
			s.TurnIndex = 0
			s.InHand = false
			switch kind {
			case "colour", "colour_bot":
				s.Target = "colour"
				s.Scores = [2]int{1, 0}
				s.Break = 1
				s.BestBreak[0] = 1
				s.Balls[0].Pos = snooker.Vec{X: snooker.TableWidth / 2, Y: .6}
				s.Balls[19].Pos = snooker.Vec{X: snooker.TableWidth / 2, Y: .2}
				s.Logs = append(s.Logs, "浏览器定点：彩球进袋及复位反馈，非自然完整比赛")
				if kind == "colour_bot" {
					s.Turn = 1
					s.TurnIndex = 1
					s.Scores = [2]int{0, 1}
					s.BestBreak[0] = 0
					s.BestBreak[1] = 1
				}
			case "black_foul":
				s.Target = "clear"
				s.Clear = 7
				s.Scores = [2]int{1, 127}
				for j := range s.Balls {
					s.Balls[j].Down = s.Balls[j].ID != 0 && s.Balls[j].ID != 21
				}
				s.Balls[0].Pos = snooker.Vec{X: snooker.TableWidth / 2, Y: .6}
				s.Balls[21].Pos = snooker.Vec{X: snooker.TableWidth / 2, Y: .2}
				s.Logs = append(s.Logs, "浏览器定点：只剩黑球，验证犯规结束原因")
			case "concede":
				s.Scores = [2]int{200, 0}
				s.Phase = "concede"
				s.Concede[1] = true
				s.OfferTeam = 1
				s.OfferTurn = 0
				s.Logs = append(s.Logs, "浏览器定点：B 队已提出认输本局")
			default:
				panic("unknown snooker browser case")
			}
			s.Due = make([]int64, s.Players)
			s.Due[s.Turn] = ctx.Now.Add(120 * time.Second).UnixMilli()
			m.Deadline = s.Due[s.Turn]
			m.TimeoutActors = []int{s.Turn}
			m.Data, _ = json.Marshal(s)
			m.Actors = []int{s.Turn}
			m.Attention = []int{s.Turn}
			return m, nil
		}
	}
	return modules
}
