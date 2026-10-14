package gametest_test

// Opt-in, explicitly labelled starting positions for real-browser feedback
// acceptance. These wrappers are compiled only into tests, not the application.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games"
	snooker "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
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
			case "colour":
				s.Target = "colour"
				s.Scores = [2]int{1, 0}
				s.Break = 1
				s.BestBreak[0] = 1
				s.Balls[0].Pos = snooker.Vec{X: snooker.TableWidth / 2, Y: .6}
				s.Balls[19].Pos = snooker.Vec{X: snooker.TableWidth / 2, Y: .2}
				s.Logs = append(s.Logs, "浏览器定点：彩球进袋及复位反馈，非自然完整比赛")
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
			m.Data, _ = json.Marshal(s)
			m.Actors = []int{0}
			m.Attention = []int{0}
			return m, nil
		}
	}
	return modules
}
