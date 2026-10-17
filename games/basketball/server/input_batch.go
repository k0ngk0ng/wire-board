package basketball

import (
	"fmt"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Relative timing preserves short and turbo edges while the browser waits for
// an HTTP acknowledgement. The client never chooses an absolute game time.
type inputEvent struct {
	Sequence   int64           `json:"sequence"`
	Buttons    engine.Buttons  `json:"buttons"`
	OffsetMS   int64           `json:"offsetMS"`
	Release    bool            `json:"release"`
	CancelMask *engine.Buttons `json:"cancelMask,omitempty"`
}

// prepareBatch is transactional, including when used to accept a stale room
// version. Copy the queue before appending; validation must not alter s.
func prepareBatch(ctx gameapi.Context, s State, a action) (State, error) {
	now := ctx.Now.UnixMilli()
	if a.Seat == nil || *a.Seat != ctx.Actor || a.BasedOn == nil || *a.BasedOn < 0 || *a.BasedOn > now || *a.BasedOn > s.ServerAt || now-*a.BasedOn > 1000 {
		return s, fmt.Errorf("输入页面已过期或席位已改变")
	}
	if a.Buttons != 0 || len(a.Events) == 0 || len(a.Events) > 64 {
		return s, fmt.Errorf("无效输入批次")
	}
	var previousSequence, previousOffset int64
	for _, event := range a.Events {
		if event.Sequence <= previousSequence || event.Sequence > maxSequence || event.OffsetMS < previousOffset || event.OffsetMS > 500 || event.Release && event.Buttons != 0 {
			return s, fmt.Errorf("无效输入时间线")
		}
		if event.CancelMask != nil && (!event.Release || *event.CancelMask == 0) {
			return s, fmt.Errorf("无效取消按键范围")
		}
		previousSequence, previousOffset = event.Sequence, event.OffsetMS
	}
	if previousSequence != a.Sequence {
		return s, fmt.Errorf("输入批次序号不一致")
	}
	i := s.Inputs[s.actorForSeat(ctx.Actor)]
	if a.Sequence <= i.Sequence {
		return s, nil
	}
	i.Changes = append([]InputChange(nil), i.Changes...)
	base := now
	if base < s.ServerAt {
		base = s.ServerAt
	}
	if n := len(i.Changes); n > 0 && base < i.Changes[n-1].At {
		base = i.Changes[n-1].At
	}
	first := true
	var clearMask engine.Buttons
	for _, event := range a.Events {
		if event.Sequence <= i.Sequence {
			continue
		}
		if first {
			base -= event.OffsetMS
			first = false
		}
		if event.Release {
			// Cancellation takes effect immediately even when an earlier batch
			// scheduled future edges. A subsequent gesture starts a new timeline.
			mask := engine.Buttons(255)
			if event.CancelMask != nil {
				mask = *event.CancelMask
			}
			clearMask |= mask
			if mask == 255 {
				clearTransport(&i)
				base = now - event.OffsetMS
			} else {
				// A lost touch may coexist with a held keyboard direction.
				// Preserve its timeline and do not manufacture another edge.
				i.Buttons &^= mask
				i.Pending &^= mask
				i.AppliedButtons &^= mask
				changes := make([]InputChange, 0, len(i.Changes))
				held := i.AppliedButtons
				for _, change := range i.Changes {
					change.Buttons &^= mask
					if change.Buttons != held {
						changes = append(changes, change)
					}
					held = change.Buttons
				}
				i.Changes = changes
			}
		} else {
			at := base + event.OffsetMS
			if at > now+500 {
				return s, fmt.Errorf("输入时间线过长")
			}
			if event.Buttons != i.Buttons {
				if len(i.Changes) >= 64 {
					return s, fmt.Errorf("输入过于频繁，请稍后重试")
				}
				i.Changes = append(i.Changes, InputChange{At: at, Buttons: event.Buttons})
			}
			i.Pending |= event.Buttons &^ i.Buttons
			if event.Buttons != 0 || i.Buttons != 0 {
				i.IdleAt = now + idleMS
			}
			i.Buttons = event.Buttons
		}
		i.Sequence = event.Sequence
	}
	if clearMask != 0 {
		runtime, err := engine.ClearRuntimeButtons(s.Runtime, uint8(s.actorForSeat(ctx.Actor)), clearMask)
		if err != nil {
			return s, err
		}
		s.Runtime = runtime
	}
	i.LastPacketAt = now
	s.Inputs[s.actorForSeat(ctx.Actor)] = i
	return s, nil
}
