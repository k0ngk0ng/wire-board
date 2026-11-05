package yuyuhakusho

import (
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/syncwire"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const streamRedundancy = 6

func streamView(match gameapi.Match, seat int, cursor string) (gameapi.StreamViewResult, error) {
	state, err := decode(match)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	epoch, number, split := strings.Cut(cursor, ":")
	after, parseErr := strconv.ParseUint(number, 10, 64)
	valid := split && parseErr == nil && epoch == state.Epoch && strconv.FormatUint(after, 10) == number && after <= state.Frame
	if valid && after < state.Frame {
		valid = len(state.History) > 0 && after+1 >= state.History[0].Frame
	}
	var history []syncwire.InputFrame
	if valid && after < state.Frame {
		first := 0
		for first < len(state.History) && state.History[first].Frame <= after {
			first++
		}
		first = max(0, first-streamRedundancy)
		history = state.History[first:]
	}
	value, err := projectState(state, seat, history)
	if err != nil {
		return gameapi.StreamViewResult{}, err
	}
	view := value.(View)
	view.Resync = !valid
	return gameapi.StreamViewResult{Data: view, Cursor: state.Epoch + ":" + strconv.FormatUint(state.Frame, 10)}, nil
}
