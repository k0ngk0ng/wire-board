package gametest_test

// Test-only midgame fixtures: own hand 888899, opponent's pair 5 on table.
// All 108 physical cards are conserved; subsequent actions use production rules.
import (
	"encoding/json"
	ddz "github.com/k0ngk0ng/wire-board/games/doudizhu/server"
	gd "github.com/k0ngk0ng/wire-board/games/guandan/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"os"
)

func pokerHintModules(modules []gameapi.Module) []gameapi.Module {
	for i := range modules {
		kind := modules[i].ID
		if kind != "doudizhu" && kind != "guandan" {
			continue
		}
		start := modules[i].Start
		modules[i].Start = func(ctx gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
			m, e := start(ctx, o)
			if e != nil {
				return m, e
			}
			counts := []int{27, 27, 27, 27}
			if kind == "doudizhu" {
				counts = []int{33, 25, 25, 25}
			}
			own := []int{6, 19, 32, 45, 7, 20}
			if kind == "guandan" && os.Getenv("WIRE_BOARD_POKER_MANUAL") == "1" {
				own = []int{7, 20, 46, 33, 6, 19, 26, 80, 9}
			}
			hands, _ := fillFeedbackHands([][]int{own, {}, {}, {3, 16, 4, 17}}, counts, 0)
			bottom := append([]int{}, hands[0][25:]...)
			played := append(append([]int{}, hands[0][len(own):]...), hands[3][:2]...)
			hands[0] = hands[0][:len(own)]
			hands[3] = hands[3][2:]
			due := []int64{ctx.Now.UnixMilli() + 120000, 0, 0, 0}
			if kind == "doudizhu" {
				var s ddz.State
				_ = json.Unmarshal(m.Data, &s)
				s.Hands = hands
				s.Bottom = bottom
				s.Played = played
				s.Phase = "play"
				s.Turn = 0
				s.Landlord = 0
				s.Bid = 3
				s.BidStart = 0
				s.BidSteps = 1
				s.Bids = []int{3, 0, 0, 0}
				s.BidDone = []bool{true, false, false, false}
				s.Marker = 6
				s.MarkerSeat = 0
				s.TouLiao = true
				s.Multiplier = 1
				s.Due = due
				s.PlayCounts = []int{1, 0, 0, 1}
				s.LastPlay = &ddz.Play{Seat: 3, Cards: []int{3, 16}, Combination: ddz.Combination{Kind: "pair", High: 5, Count: 2, Units: 1}}
				s.Table = []ddz.Play{*s.LastPlay}
				s.Logs = append(s.Logs, "浏览器定点手牌夹具：使用生产操作验证提示与手动配牌")
				m.Data, _ = json.Marshal(s)
			} else {
				var s gd.State
				_ = json.Unmarshal(m.Data, &s)
				s.Hands = hands
				s.Played = played
				s.Phase = "play"
				s.Turn = 0
				s.Level = 2
				s.Due = due
				s.LastPlay = &gd.Play{Seat: 3, Cards: []int{3, 16}, Combination: gd.Combination{Key: "pair:5:2", Kind: "pair", High: 5, Count: 2, Wild: []gd.Replacement{}}}
				s.Table = []gd.Play{*s.LastPlay}
				s.Logs = append(s.Logs, "浏览器定点手牌夹具：使用生产操作验证提示与手动配牌")
				m.Data, _ = json.Marshal(s)
			}
			m.Actors = []int{0}
			m.TimeoutActors = []int{0}
			m.Attention = []int{0}
			m.Deadline = due[0]
			return m, nil
		}
	}
	return modules
}
