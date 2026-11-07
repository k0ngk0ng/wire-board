// Package runtimebridge exposes the same independent engine to native replay
// and browser prediction. Each owner has isolated, explicitly released state.
package runtimebridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
)

type Bridge struct{ sessions map[string]engine.State }

func New() *Bridge { return &Bridge{sessions: map[string]engine.State{}} }

type request struct {
	Op       string          `json:"op"`
	ID       string          `json:"id"`
	Source   string          `json:"source,omitempty"`
	Players  int             `json:"players"`
	Epoch    string          `json:"epoch"`
	Options  engine.Options  `json:"options"`
	Snapshot json.RawMessage `json:"snapshot"`
	Actor    int             `json:"actor"`
	Action   engine.Action   `json:"action"`
	Ticks    int             `json:"ticks"`
	Compact  bool            `json:"compact,omitempty"`
	AtTick   *int64          `json:"atTick,omitempty"`
}
type response struct {
	OK     bool                 `json:"ok"`
	Error  string               `json:"error,omitempty"`
	State  *engine.State        `json:"state,omitempty"`
	Digest string               `json:"digest,omitempty"`
	View   *engine.Presentation `json:"view,omitempty"`
}

func (b *Bridge) JSON(input string) string {
	out := response{}
	q, err := decodeRequest(input)
	var state *engine.State
	if err == nil {
		state, err = b.call(q)
	}
	if err != nil {
		out.Error = err.Error()
	} else {
		out.OK = true
		if state != nil && q.Compact {
			view := engine.Present(*state)
			out.View = &view
		} else if state != nil {
			out.State = state
			raw, _ := json.Marshal(state)
			sum := sha256.Sum256(raw)
			out.Digest = hex.EncodeToString(sum[:])
		}
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}
func decodeRequest(input string) (request, error) {
	var q request
	if len(input) > 4<<20 {
		return q, errors.New("运行时请求过大")
	}
	d := json.NewDecoder(bytes.NewBufferString(input))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return q, errors.New("无效运行时请求")
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return q, errors.New("请求包含多余内容")
	}
	return q, nil
}
func (b *Bridge) call(q request) (*engine.State, error) {
	if q.ID == "" || len(q.ID) > 96 {
		return nil, errors.New("无效运行时实例")
	}
	if q.Op == "drop" {
		delete(b.sessions, q.ID)
		return nil, nil
	}
	s, exists := b.sessions[q.ID]
	if !exists && len(b.sessions) >= 32 {
		return nil, errors.New("运行时实例已满")
	}
	switch q.Op {
	case "new":
		next, err := engine.New(q.Options, q.Players, q.Epoch)
		if err != nil {
			return nil, err
		}
		s = next
	case "load":
		// This is a replacement checkpoint, never a merge into reused slices.
		// Optional zero motion/state fields must erase their old values.
		s = engine.State{}
		if json.Unmarshal(q.Snapshot, &s) != nil {
			return nil, errors.New("无效恢复快照")
		}
		if err := engine.Validate(s); err != nil {
			return nil, err
		}
	case "fork":
		original, ok := b.sessions[q.Source]
		if !ok {
			return nil, errors.New("复制的规则实例不存在")
		}
		s = engine.Clone(original)
	case "inspect":
		if !exists {
			return nil, errors.New("运行时实例不存在")
		}
		return &s, nil
	case "apply":
		if !exists {
			return nil, errors.New("运行时实例不存在")
		}
		s = engine.Clone(s)
		// A local intent belongs after the last displayed fractional frame. Its
		// own object is previewed now without simulating the future world tick.
		at := s.Tick
		if q.AtTick != nil {
			if *q.AtTick < at || *q.AtTick > at+1 {
				return nil, errors.New("无效本地操作帧")
			}
			s.Tick = *q.AtTick
		}
		if err := engine.Apply(&s, q.Actor, q.Action); err != nil {
			return nil, err
		}
		s.Tick = at
	case "step":
		if !exists {
			return nil, errors.New("运行时实例不存在")
		}
		if q.Ticks < 0 || q.Ticks > 2000 {
			return nil, errors.New("预测步数超出范围")
		}
		s = engine.Clone(s)
		var err error
		if s.Projection {
			err = engine.StepProjection(&s, q.Ticks)
		} else {
			err = engine.Step(&s, q.Ticks)
		}
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("未知运行时操作")
	}
	b.sessions[q.ID] = s
	return &s, nil
}
