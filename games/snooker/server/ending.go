package snooker

import "fmt"

// Explanations are derived from the recorded authoritative stroke, so legacy
// finished matches can display the same reason without rewriting saved logs.
func (s *State) finishExplanation(reason string) string {
	switch reason {
	case "black":
		detail := "黑球终局："
		if f := s.Flight; f != nil && f.Resolved {
			r := Judge(f.Context, f.Strike, f.Motion)
			if r.Foul > 0 {
				detail += fmt.Sprintf("玩家 %d 本杆犯规（%s），对方获得 %d 分。", f.Seat+1, joinReasons(r.Reasons), r.Foul)
			} else if r.Points > 0 {
				detail += fmt.Sprintf("玩家 %d 本杆黑球进球，获得 %d 分。", f.Seat+1, r.Points)
			}
		}
		return detail + "台面只剩黑球时，首次得分或犯规即结束本局；平分则重摆黑球决胜。"
	case "claimed":
		return "宣告胜利：台面只剩黑球，当前一方领先超过 7 分并主动宣告本局胜利。"
	case "conceded":
		return "认输本局：一方提出认输本局，对方接受后结束本局。双打需要本队两人共同确认。"
	case "three_misses":
		return "三次解球未中判负：在可完整击中目标球、已警告并连续原位重打的情况下，第三次仍未先击中目标球，本局判负。"
	case "resigned":
		return "弃赛：一方确认退出整场比赛，对方赢得本场。双打需要本队两人共同确认。"
	default:
		return ""
	}
}

func (s *State) endExplanation() string {
	if s.Phase != "finished" && s.Phase != "frame_end" {
		return ""
	}
	reason := s.FinishReason
	if reason == "" && len(s.Results) > 0 {
		reason = s.Results[len(s.Results)-1].Reason
	}
	return s.finishExplanation(reason)
}
