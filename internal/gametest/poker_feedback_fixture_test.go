package gametest_test

// Labelled UI milestones only; the normal browser fixture still deals naturally.
// No private debug routes or test branches are compiled into the application.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games"
	ddz "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	sj "github.com/k0ngk0ng/wire-board/games/shengji/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
)

func fillFeedbackHands(h [][]int, counts []int, bottom int) ([][]int, []int) {
	used := map[int]bool{}
	for _, hand := range h {
		for _, id := range hand {
			if used[id] {
				panic("duplicate fixture card")
			}
			used[id] = true
		}
	}
	rest := []int{}
	for id := 0; id < 108; id++ {
		if !used[id] {
			rest = append(rest, id)
		}
	}
	for p := range h {
		n := counts[p] - len(h[p])
		h[p] = append(h[p], rest[:n]...)
		rest = rest[n:]
	}
	if len(rest) != bottom {
		panic("fixture conservation")
	}
	return h, rest
}
func pokerBrowserModules() []gameapi.Module {
	modules := games.Modules()
	if os.Getenv("WIRE_BOARD_POKER_FEEDBACK") != "1" {
		return modules
	}
	for i := range modules {
		start := modules[i].Start
		switch modules[i].ID {
		case "doudizhu":
			modules[i].Start = func(ctx gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
				m, e := start(ctx, o)
				if e != nil {
					return m, e
				}
				var s ddz.State
				_ = json.Unmarshal(m.Data, &s)
				s.Hands, _ = fillFeedbackHands([][]int{{1, 14, 27, 6, 19, 12, 13, 40, 2, 3, 5, 18, 31, 44, 52, 53, 106, 107}, {}, {}, {}}, []int{33, 25, 25, 25}, 0)
				s.Bottom = append([]int{}, s.Hands[0][25:]...)
				s.Phase = "play"
				s.Turn = 0
				s.Landlord = 0
				s.Bid = 3
				s.BidStart = 0
				s.BidSteps = 1
				s.Bids = []int{3, 0, 0, 0}
				s.BidDone = []bool{true, false, false, false}
				s.Marker = 0
				s.MarkerSeat = 0
				s.TouLiao = true
				s.OpenHand = false
				s.Multiplier = 1
				s.Reports = [][]int{}
				s.BombCounts = make([]int, 4)
				s.Due = []int64{ctx.Now.UnixMilli() + 120000, 0, 0, 0}
				s.Logs = append(s.Logs, "浏览器定点夹具：地主已选择正常出牌；用于三带二、A2345、炸弹和王炸反馈")
				m.Data, _ = json.Marshal(s)
				m.Actors = []int{0}
				m.TimeoutActors = []int{0}
				m.Attention = []int{0}
				m.Deadline = s.Due[0]
				return m, nil
			}
		case "shengji":
			modules[i].Start = func(ctx gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
				m, e := start(ctx, o)
				if e != nil {
					return m, e
				}
				var s sj.State
				_ = json.Unmarshal(m.Data, &s)
				s.Hands, s.Bottom = fillFeedbackHands([][]int{{12, 10, 64, 9, 52, 53, 106, 107, 0, 13, 26, 39, 54}, {11}, {}, {}}, []int{25, 25, 25, 25}, 8)
				s.Phase = "play"
				s.Turn = 0
				s.Dealer = 0
				s.StartSeat = 0
				s.Level = 2
				s.Trump = 3
				s.Declaration = nil
				s.Revealed = []sj.Reveal{}
				s.Trick = []sj.Play{}
				s.Lead = nil
				s.LastTrick = nil
				s.Penalty = 0
				s.ChallengeSeat = -1
				s.Due = []int64{ctx.Now.UnixMilli() + 120000, 0, 0, 0}
				s.Logs = append(s.Logs, "浏览器定点夹具：扣底完成，用于主牌排序及不成立甩牌的拒绝反馈")
				m.Data, _ = json.Marshal(s)
				m.Actors = []int{0}
				m.TimeoutActors = []int{0}
				m.Attention = []int{0}
				m.Deadline = s.Due[0]
				return m, nil
			}
		}
	}
	return modules
}
