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
	Players  int             `json:"players"`
	Epoch    string          `json:"epoch"`
	Options  engine.Options  `json:"options"`
	Snapshot json.RawMessage `json:"snapshot"`
	Actor    int             `json:"actor"`
	Action   engine.Action   `json:"action"`
	Ticks    int             `json:"ticks"`
}
type response struct {
	OK     bool          `json:"ok"`
	Error  string        `json:"error,omitempty"`
	State  *engine.State `json:"state,omitempty"`
	Digest string        `json:"digest,omitempty"`
}

func (b *Bridge) JSON(input string) string {
	out := response{}
	state, err := b.call(input)
	if err != nil {
		out.Error = err.Error()
	} else {
		out.OK = true
		out.State = state
		if state != nil {
			raw, _ := json.Marshal(state)
			sum := sha256.Sum256(raw)
			out.Digest = hex.EncodeToString(sum[:])
		}
	}
	raw, _ := json.Marshal(out)
	return string(raw)
}
func (b *Bridge) call(input string) (*engine.State, error) {
	if len(input) > 4<<20 {
		return nil, errors.New("运行时请求过大")
	}
	var q request
	d := json.NewDecoder(bytes.NewBufferString(input))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return nil, errors.New("无效运行时请求")
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return nil, errors.New("请求包含多余内容")
	}
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
		if json.Unmarshal(q.Snapshot, &s) != nil {
			return nil, errors.New("无效恢复快照")
		}
		if err := engine.Validate(s); err != nil {
			return nil, err
		}
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
		if err := engine.Apply(&s, q.Actor, q.Action); err != nil {
			return nil, err
		}
	case "step":
		if !exists {
			return nil, errors.New("运行时实例不存在")
		}
		if q.Ticks < 0 || q.Ticks > 2000 {
			return nil, errors.New("预测步数超出范围")
		}
		s = engine.Clone(s)
		if err := engine.Step(&s, q.Ticks); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("未知运行时操作")
	}
	b.sessions[q.ID] = s
	return &s, nil
}
